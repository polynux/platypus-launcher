import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { DiskUsage, Game } from '../../types';

const mock = vi.hoisted(() => ({
  api: {
    getDiskUsage: vi.fn(),
    getGameFolders: vi.fn(),
    listJobs: vi.fn(),
    discardJob: vi.fn(),
    refreshDiskUsage: vi.fn(),
    openGameFolder: vi.fn(),
    scanGameSize: vi.fn(),
    checkRemove: vi.fn(),
    checkMove: vi.fn(),
    pickMoveFolder: vi.fn(),
    moveGame: vi.fn(),
  },
  subs: [] as ((d: unknown) => void)[],
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (mock.api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
  onDiskUsage: (cb: (d: unknown) => void) => {
    mock.subs.push(cb);
    return () => {};
  },
}));

import { StorageView } from '../StorageView';
import { Sidebar } from '../Sidebar';

const GB = 1e9;
const noop = () => {};

function usage(over: Partial<DiskUsage> = {}): DiskUsage {
  return {
    drives: [
      {
        id: '/mnt/a',
        mount: '/mnt/a',
        folders: ['/mnt/a/Games'],
        total_bytes: 100 * GB,
        free_bytes: 40 * GB,
        games_bytes: 30 * GB,
        other_bytes: 30 * GB,
      },
    ],
    games: [],
    scanning: false,
    generated_at: 0,
    ...over,
  };
}

function row(over: Partial<DiskUsage['games'][number]> = {}): DiskUsage['games'][number] {
  return {
    game_id: 'g',
    title: 'Alpha',
    source: 'custom',
    store: 'custom',
    install_dir: '/mnt/a/Games/Alpha',
    size_bytes: 5 * GB,
    prefix_bytes: 1 * GB,
    from_plugin: false,
    measured: true,
    ...over,
  };
}

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'custom', store: 'custom', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/mnt/a/Games/${id}`, executable: `/mnt/a/Games/${id}/run.sh`, is_installed: true, settings: {}, features: [], ...o }) as Game;

function viewProps(over: Partial<Parameters<typeof StorageView>[0]> = {}) {
  return { games: [], onNotify: noop, onRemove: noop, onUninstall: noop, ...over };
}

const wrap = (ui: React.ReactElement) => render(<TooltipProvider>{ui}</TooltipProvider>);
const view = (over: Partial<Parameters<typeof StorageView>[0]> = {}) => wrap(<StorageView {...viewProps(over)} />);
const emit = (d: DiskUsage) =>
  act(() => {
    mock.subs.forEach((cb) => cb(d));
  });

beforeEach(() => {
  mock.subs.length = 0;
  Object.values(mock.api).forEach((f) => f.mockReset());
  mock.api.getGameFolders.mockReturnValue(Effect.succeed([]));
  mock.api.listJobs.mockReturnValue(Effect.succeed([]));
  mock.api.discardJob.mockReturnValue(Effect.succeed(undefined as never));
});

describe('StorageView', () => {
  it('asks for the snapshot and shows the drive bar proportions with figures', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage()));
    const { container } = view();
    const bar = await screen.findByRole('progressbar');
    expect(mock.api.getDiskUsage).toHaveBeenCalled();
    expect(bar).toHaveAttribute('aria-valuenow', '30');
    // 30 GB of 100 GB are the games' share, 30 GB the rest: two segments.
    expect(bar.children[0]).toHaveStyle({ width: '30%' });
    expect(bar.children[1]).toHaveStyle({ width: '30%' });
    expect(screen.getByText('Platypus games')).toBeInTheDocument();
    expect(screen.getByText('Free')).toBeInTheDocument();
    expect(screen.getAllByText('30.0 GB')).toHaveLength(2);
    expect(screen.getByText('40.0 GB')).toBeInTheDocument();
    expect(container.querySelector('[aria-label="/mnt/a"]')).toBe(bar);
  });

  it('lists games biggest first with store, location and prefix figures', async () => {
    mock.api.getDiskUsage.mockReturnValue(
      Effect.succeed(
        usage({
          games: [
            row({ game_id: 's', title: 'Small', install_dir: '/mnt/a/Games/Small', size_bytes: 2 * GB, prefix_bytes: 0 }),
            row({ game_id: 'b', title: 'Big', install_dir: '/mnt/a/Games/Big', size_bytes: 9 * GB, prefix_bytes: 2 * GB, from_plugin: true, measured: false }),
            row({ game_id: 'm', title: 'Mid', install_dir: '/mnt/a/Games/Mid', size_bytes: 5 * GB, prefix_bytes: 1 * GB }),
          ],
        }),
      ),
    );
    view();
    const rows = await screen.findAllByRole('listitem');
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining('Big'),
      expect.stringContaining('Mid'),
      expect.stringContaining('Small'),
    ]);
    expect(screen.getByText('from store')).toBeInTheDocument();
    expect(screen.getAllByText('scanned')).toHaveLength(2);
    expect(screen.getAllByText(/Wine prefix/)).toHaveLength(2);
    expect(screen.getByText('/mnt/a/Games/Big')).toBeInTheDocument();
  });

  it('pulses placeholders and disables the rescan while measuring', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ scanning: true })));
    const { container } = view();
    expect(await screen.findByRole('status')).toBeInTheDocument();
    expect(container.querySelector('.animate-pulse')).toBeTruthy();
    const btn = screen.getByRole('button', { name: /Measuring/ });
    expect(btn).toBeDisabled();
    // The drive's own totals come straight from statfs: the bar stays, only
    // the games still wait for the pass.
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('rescans all sizes on the header button', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage()));
    mock.api.refreshDiskUsage.mockReturnValue(Effect.succeed(usage({ scanning: true })));
    view();
    fireEvent.click(await screen.findByRole('button', { name: /Rescan all sizes/ }));
    expect(mock.api.refreshDiskUsage).toHaveBeenCalled();
    expect(await screen.findByRole('button', { name: /Measuring/ })).toBeDisabled();
  });

  it('shows the empty state when there is nothing to measure', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ drives: [] })));
    view();
    expect(await screen.findByText('no games to measure')).toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('re-renders in French without a reload', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage()));
    view();
    await screen.findByRole('progressbar');
    expect(screen.getByText('Rescan all sizes')).toBeInTheDocument();

    await act(async () => {
      await i18n.changeLanguage('fr');
    });
    expect(screen.getByText('Réanalyser toutes les tailles')).toBeInTheDocument();
    expect(screen.getByText('Jeux Platypus')).toBeInTheDocument();

    await act(async () => {
      await i18n.changeLanguage('en');
    });
    expect(screen.getByText('Rescan all sizes')).toBeInTheDocument();
  });

  it('applies a finished pass pushed by the core', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ scanning: true })));
    view();
    await screen.findByRole('status');
    emit(usage());
    expect(await screen.findByRole('progressbar')).toBeInTheDocument();
  });

  it('keeps the newer snapshot when the first answer lands late', async () => {
    let answer!: (v: unknown) => void;
    mock.api.getDiskUsage.mockReturnValue(Effect.promise(() => new Promise((res) => (answer = res))));
    view();
    // An event arrives while the first call is still out…
    emit(usage({ generated_at: 200, games: [row({ game_id: 'new', title: 'Fresh' })] }));
    expect(await screen.findByText('Fresh')).toBeInTheDocument();
    // …and its older answer must not overwrite it.
    await act(async () => {
      answer(usage({ generated_at: 100, games: [row({ game_id: 'old', title: 'Stale' })] }));
    });
    expect(screen.getByText('Fresh')).toBeInTheDocument();
    expect(screen.queryByText('Stale')).toBeNull();
  });

  it('opens the folder of a game', async () => {
    const alpha = mk('g', { title: 'Alpha' });
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ games: [row({ game_id: 'g', title: 'Alpha' })] })));
    view({ games: [alpha] });
    fireEvent.click(await screen.findByRole('button', { name: 'Open the folder of Alpha' }));
    await waitFor(() => expect(mock.api.openGameFolder).toHaveBeenCalledWith(alpha));
  });

  it('scans one game and spins until the next snapshot', async () => {
    const alpha = mk('g', { title: 'Alpha' });
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ games: [row({ game_id: 'g', title: 'Alpha' })] })));
    // The scan stays pending until the test releases it: asserting the
    // spinner while the promise is unresolved makes the spinning state
    // deterministic instead of racing the mock's resolution.
    let resolveScan!: (v: unknown) => void;
    mock.api.scanGameSize.mockReturnValue(Effect.promise(() => new Promise((res) => (resolveScan = res))));
    view({ games: [alpha] });
    await screen.findByRole('button', { name: 'Scan Alpha on disk' });
    fireEvent.click(screen.getByRole('button', { name: 'Scan Alpha on disk' }));
    await waitFor(() => expect(mock.api.scanGameSize).toHaveBeenCalledWith(alpha));
    // The Scan request is still out: the row spins and stays mounted (the
    // core answers with the current snapshot marked scanning, games kept).
    // waitFor the appearance: under full-suite load React may flush the
    // click's render a tick after the mock records the call. The deferred
    // scan keeps this deterministic — the spinner cannot disappear again
    // until the test releases the scan and emits the next snapshot below,
    // so a missing spinner still fails loudly instead of being masked.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Scan Alpha on disk' }).querySelector('.animate-spin')).toBeTruthy(),
    );
    await act(async () => {
      resolveScan(usage({ generated_at: 1, scanning: true, games: [row({ game_id: 'g', title: 'Alpha' })] }));
    });
    expect(screen.getByRole('button', { name: 'Scan Alpha on disk' }).querySelector('.animate-spin')).toBeTruthy();
    emit(usage({ generated_at: 5, games: [row({ game_id: 'g', title: 'Alpha' })] }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Scan Alpha on disk' }).querySelector('.animate-spin')).toBeNull());
  });

  it('offers Scan, Move and Trash only where they apply', async () => {
    const lib = [
      mk('g', { title: 'Alpha' }), // sideloaded
      mk('p', { title: 'Beta', source: 'echo', store: 'gog', is_installed: true }), // store game
      mk('u', { title: 'Gamma', source: 'echo', store: 'gog', is_installed: false }), // not installed
    ];
    mock.api.getDiskUsage.mockReturnValue(
      Effect.succeed(
        usage({
          games: [
            row({ game_id: 'g', title: 'Alpha' }),
            row({ game_id: 'p', title: 'Beta', source: 'echo', store: 'gog', install_dir: '/mnt/a/Games/Beta' }),
            row({ game_id: 'u', title: 'Gamma', source: 'echo', store: 'gog', install_dir: '/mnt/a/Games/Gamma' }),
          ],
        }),
      ),
    );
    view({ games: lib });
    await screen.findByRole('list');
    // Scan and Move need an installed game; Move also needs the store plugin
    // core's planMove goes through.
    expect(screen.getByRole('button', { name: 'Scan Alpha on disk' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Scan Beta on disk' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Scan Gamma on disk' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Move Alpha' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Move Beta' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Move Gamma' })).toBeNull();
    // Trash: removal for the sideloaded game, uninstall for the installed
    // store game, nothing for the uninstalled one.
    expect(screen.getByRole('button', { name: 'Remove Alpha from the library' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Uninstall Beta' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Uninstall Gamma' })).toBeNull();
  });

  it('removes a sideloaded game through the #43 panel', async () => {
    const alpha = mk('g', { title: 'Alpha' });
    const onRemove = vi.fn();
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ games: [row({ game_id: 'g', title: 'Alpha' })] })));
    mock.api.checkRemove.mockReturnValue(
      Effect.succeed({ targets: [], saves: { off: true } } as never),
    );
    view({ games: [alpha], onRemove });
    fireEvent.click(await screen.findByRole('button', { name: 'Remove Alpha from the library' }));
    // The first row, with the check fetched as the panel opens. (The row's
    // MonoLabel heading shares the button's text, so wait for the button.)
    expect(await screen.findByRole('button', { name: 'Remove from library' })).toBeInTheDocument();
    await waitFor(() => expect(mock.api.checkRemove).toHaveBeenCalledWith('g', { delete_prefix: true, delete_files: true, allow_no_saves: false }));
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove from library' })[0]);
    expect(await screen.findByTestId('remove-confirm-panel')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove from library' }).at(-1)!);
    await waitFor(() =>
      expect(onRemove).toHaveBeenCalledWith(alpha, { delete_prefix: false, delete_files: false, allow_no_saves: false }),
    );
  });

  it('uninstalls an installed store game after an inline confirm', async () => {
    const beta = mk('p', { title: 'Beta', source: 'echo', store: 'gog' });
    const onUninstall = vi.fn();
    const onCancel = vi.fn();
    mock.api.getDiskUsage.mockReturnValue(
      Effect.succeed(usage({ games: [row({ game_id: 'p', title: 'Beta', source: 'echo', store: 'gog' })] })),
    );
    view({ games: [beta], onUninstall, onNotify: onCancel });
    fireEvent.click(await screen.findByRole('button', { name: 'Uninstall Beta' }));
    expect(await screen.findByText('Uninstall Beta? Its files leave the disk.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onUninstall).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall Beta' }));
    fireEvent.click(await screen.findByRole('button', { name: /^Uninstall$/ }));
    await waitFor(() => expect(onUninstall).toHaveBeenCalledWith(beta));
  });

  it('moves a store game through the Move dialog', async () => {
    const beta = mk('p', { title: 'Beta', source: 'echo', store: 'gog' });
    mock.api.getDiskUsage.mockReturnValue(
      Effect.succeed(usage({ games: [row({ game_id: 'p', title: 'Beta', source: 'echo', store: 'gog' })] })),
    );
    view({ games: [beta] });
    fireEvent.click(await screen.findByRole('button', { name: 'Move Beta' }));
    expect(await screen.findByText('Move Beta')).toBeInTheDocument();
  });
});

describe('sidebar Storage entry', () => {
  const base = {
    view: { name: 'storage' as const },
    onNavigate: () => {},
    search: '',
    onSearch: () => {},
    storeCounts: {},
    onPickSource: () => {},
    gameCount: 0,
    downloadCount: 0,
    authNeeded: false,
    fixCount: 0,
    runnerCount: 0,
    lastScan: null,
    sessions: [],
    onStop: () => {},
    friends: [],
    friendLabels: {},
    onOpenFriends: () => {},
    canRedeem: false,
    onRedeem: () => {},
  };

  it('sits in the Tools group and is marked current', () => {
    render(
      <TooltipProvider>
        <Sidebar {...base} />
      </TooltipProvider>,
    );
    const btn = screen.getByRole('button', { name: 'Storage' });
    expect(btn.getAttribute('aria-current')).toBe('page');
  });
});
