import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';

const mock = vi.hoisted(() => ({
  api: {
    getDiskUsage: vi.fn(),
    refreshDiskUsage: vi.fn(),
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

function usage(over: Record<string, unknown> = {}) {
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
    games: [] as Record<string, unknown>[],
    scanning: false,
    generated_at: 0,
    ...over,
  };
}

function game(over: Record<string, unknown> = {}) {
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

const wrap = (ui: React.ReactElement) => render(<TooltipProvider>{ui}</TooltipProvider>);

beforeEach(() => {
  mock.subs.length = 0;
  Object.values(mock.api).forEach((f) => f.mockReset());
});

describe('StorageView', () => {
  it('asks for the snapshot and shows the drive bar proportions with figures', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage()));
    const { container } = wrap(<StorageView />);
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
            game({ game_id: 's', title: 'Small', install_dir: '/mnt/a/Games/Small', size_bytes: 2 * GB, prefix_bytes: 0 }),
            game({ game_id: 'b', title: 'Big', install_dir: '/mnt/a/Games/Big', size_bytes: 9 * GB, prefix_bytes: 2 * GB, from_plugin: true, measured: false }),
            game({ game_id: 'm', title: 'Mid', install_dir: '/mnt/a/Games/Mid', size_bytes: 5 * GB, prefix_bytes: 1 * GB }),
          ],
        }),
      ),
    );
    wrap(<StorageView />);
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
    const { container } = wrap(<StorageView />);
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
    wrap(<StorageView />);
    fireEvent.click(await screen.findByRole('button', { name: /Rescan all sizes/ }));
    expect(mock.api.refreshDiskUsage).toHaveBeenCalled();
    expect(await screen.findByRole('button', { name: /Measuring/ })).toBeDisabled();
  });

  it('shows the empty state when there is nothing to measure', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage({ drives: [] })));
    wrap(<StorageView />);
    expect(await screen.findByText('no games to measure')).toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).toBeNull();
  });

  it('re-renders in French without a reload', async () => {
    mock.api.getDiskUsage.mockReturnValue(Effect.succeed(usage()));
    wrap(<StorageView />);
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
    wrap(<StorageView />);
    await screen.findByRole('status');
    act(() => {
      mock.subs.forEach((cb) => cb(usage()));
    });
    expect(await screen.findByRole('progressbar')).toBeInTheDocument();
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
