import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, Job, MoveCheck, SessionEvent, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  verifyGame: vi.fn(),
  repairGame: vi.fn(),
  pickMoveFolder: vi.fn(),
  checkMove: vi.fn(),
  moveGame: vi.fn(),
  getGameDetails: vi.fn(),
  getGameVersions: vi.fn(),
  getInstallSize: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';
import { VerifyDialog } from '../VerifyDialog';
import { MoveDialog } from '../MoveDialog';
import { formatBytes } from '../../lib/games';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: ['install', 'verify', 'move'],
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const job = (o: Partial<Job> = {}): Job => ({
  id: 'j1', kind: 'verify', source: 'echo', game_id: 'p.echo.tiny', store_game_id: 'tiny', title: 'Echo Tiny', state: 'queued',
  bytes: 0, total: 0, speed: 0, percent: -1, attempts: 0, created_at: 0, updated_at: 0, ...o,
}) as Job;

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;

const base = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
});

/** Opens the game settings from the page's gear, then a tab by its name. */
const openTab = (name: RegExp | string) => {
  fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
  fireEvent.click(screen.getByRole('tab', { name }));
};

describe('Verify and move in the game settings', () => {
  it('offers Verify files only when the plugin offers verify', () => {
    const { unmount } = render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    expect(screen.queryByRole('button', { name: /Verify files/ })).toBeNull(); // not on the page itself
    openTab('Maintenance');
    expect(screen.getByRole('button', { name: /Verify files/ })).toBeTruthy();
    unmount();
    render(<GamePage game={game({ features: null })} plugin={plugin(['install'])} {...base} />);
    openTab('Maintenance');
    expect(screen.queryByRole('button', { name: /Verify files/ })).toBeNull();
  });

  it('offers Move for an installed plugin game, whatever the plugin offers', () => {
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    openTab('Maintenance');
    expect(screen.getByRole('button', { name: /Move…/ })).toBeTruthy();
  });

  it('opens the verify dialog above the settings, no wider than a dialog should be', async () => {
    api.verifyGame.mockResolvedValue(job({ id: 'check-9' }));
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: /Verify files/ }));
    const dialog = await screen.findByRole('dialog', { name: 'Verify Echo Tiny' });
    expect(dialog.className).toMatch(/max-w-\[560px\]/);
    expect(api.verifyGame).toHaveBeenCalledOnce();
  });

  it('asks before uninstalling, names the game, folder and space, and closes the settings', () => {
    const onJob = vi.fn();
    render(<GamePage game={game({ size_bytes: 1_200_000_000 })} plugin={plugin(['install', 'verify'])} {...base} onJob={onJob} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    expect(onJob).not.toHaveBeenCalled();
    expect(screen.getByText(/Delete Echo Tiny — 1\.2 GB in \/games\/echo\/tiny\?/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Delete game files' }));
    expect(onJob).toHaveBeenCalledWith('uninstall');
    expect(screen.queryByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeNull();
  });

  it('keeps the confirmation through a blur, and Cancel or Escape backs out of it', () => {
    const onJob = vi.fn();
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} onJob={onJob} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    fireEvent.blur(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByRole('button', { name: 'Delete game files' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByRole('button', { name: 'Uninstall' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.getByRole('button', { name: 'Uninstall' })).toBeTruthy();
    expect(onJob).not.toHaveBeenCalled();
  });

  it('shows the running job instead of the actions', () => {
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} job={job({ kind: 'update', state: 'running', percent: 40 })} />);
    openTab('Maintenance');
    expect(screen.getByRole('progressbar')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Verify files/ })).toBeNull();
  });
});

describe('Remove from library for sideloaded games', () => {
  const custom = (o: Partial<Game> = {}): Game => game({
    id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom',
    title: 'Sideloaded Gem', install_dir: '/games/sideload', executable: '/games/sideload/gem',
    features: null, ...o,
  });

  it('shows only Remove from library, no Verify/Move/Uninstall', () => {
    render(<GamePage game={custom()} {...base} onRemove={() => {}} />);
    openTab('Maintenance');
    expect(screen.getByRole('button', { name: 'Remove from library' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Verify files/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /Move/ })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Uninstall' })).toBeNull();
    expect(screen.getByText(/Removes Sideloaded Gem from Platypus\. Its files stay in \/games\/sideload\./)).toBeTruthy();
  });

  it('asks to confirm; Cancel or Escape backs out without removing', () => {
    const onRemove = vi.fn();
    render(<GamePage game={custom()} {...base} onRemove={onRemove} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    expect(onRemove).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.getByRole('button', { name: 'Remove from library' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.getByRole('button', { name: 'Remove from library' })).toBeTruthy();
    expect(onRemove).not.toHaveBeenCalled();
  });

  it('confirming calls onRemove once and closes the settings', () => {
    const onRemove = vi.fn();
    render(<GamePage game={custom()} {...base} onRemove={onRemove} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    expect(onRemove).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog', { name: 'Settings for Sideloaded Gem' })).toBeNull();
  });

  it('asks on the confirm row, unlike the first row which explains', () => {
    render(<GamePage game={custom()} {...base} onRemove={() => {}} />);
    openTab('Maintenance');
    expect(screen.getByText(/Removes Sideloaded Gem from Platypus\. Its files stay in \/games\/sideload\./)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    expect(screen.getByText(/Remove Sideloaded Gem from Platypus\? Its files stay in \/games\/sideload\./)).toBeTruthy();
    expect(screen.queryByText(/Removes Sideloaded Gem from Platypus\. Its files stay in \/games\/sideload\./)).toBeNull();
  });

  it('disables Remove while the custom game is running, with no confirm step', () => {
    const onRemove = vi.fn();
    const session = { game_id: 'custom-1', state: 'running' } as SessionEvent;
    const { unmount } = render(<GamePage {...base} game={custom()} session={session} onRemove={onRemove} />);
    openTab('Maintenance');
    expect(screen.getByText(/Stop Sideloaded Gem before removing it from the library\./)).toBeTruthy();
    const remove = screen.getByRole<HTMLButtonElement>('button', { name: 'Remove from library' });
    expect(remove.disabled).toBe(true);
    fireEvent.click(remove);
    expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull();
    expect(onRemove).not.toHaveBeenCalled();
    unmount();

    // Launching counts as running too.
    render(<GamePage {...base} game={custom()} launching onRemove={onRemove} />);
    openTab('Maintenance');
    expect(screen.getByText(/Stop Sideloaded Gem before removing it from the library\./)).toBeTruthy();
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Remove from library' }).disabled).toBe(true);
    expect(onRemove).not.toHaveBeenCalled();
  });

  it('closes the confirm row if the game starts while it is open', () => {
    const onRemove = vi.fn();
    const { rerender } = render(<GamePage {...base} game={custom()} onRemove={onRemove} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeTruthy();
    rerender(<GamePage {...base} game={custom()} launching onRemove={onRemove} />);
    expect(screen.queryByRole('button', { name: 'Cancel' })).toBeNull();
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Remove from library' }).disabled).toBe(true);
    expect(onRemove).not.toHaveBeenCalled();
  });
});

describe('VerifyDialog', () => {
  it('starts a check when it opens and offers a repair after damage is found', async () => {
    api.verifyGame.mockResolvedValue(job({ id: 'check-1' }));
    const { rerender } = render(<VerifyDialog game={game()} jobs={[job({ id: 'check-1' })]} onClose={() => {}} />);
    expect(api.verifyGame).toHaveBeenCalledOnce();
    expect(await screen.findByText(/Checking the files/)).toBeTruthy();

    rerender(<VerifyDialog game={game()} jobs={[job({ id: 'check-1', state: 'done', verify: { checked: 12, damaged: 3, missing: 0, repaired: 0 } })]} onClose={() => {}} />);
    expect(await screen.findByText('3 files damaged — Repair?')).toBeTruthy();

    api.repairGame.mockResolvedValue(job({ id: 'repair-1', repair: true }));
    fireEvent.click(screen.getByRole('button', { name: 'Repair' }));
    await waitFor(() => expect(api.repairGame).toHaveBeenCalledOnce());
    rerender(<VerifyDialog game={game()} jobs={[
      job({ id: 'check-1', state: 'done', verify: { checked: 12, damaged: 3, missing: 0, repaired: 0 } }),
      job({ id: 'repair-1', repair: true, state: 'done', verify: { checked: 12, damaged: 0, missing: 0, repaired: 3 } }),
    ]} onClose={() => {}} />);
    expect(await screen.findByText('Repaired 3 files.')).toBeTruthy();
  });

  it('says so when every file checks out, and offers no repair', async () => {
    api.verifyGame.mockResolvedValue(job({ id: 'check-2' }));
    render(<VerifyDialog game={game()} jobs={[job({ id: 'check-2', state: 'done', verify: { checked: 12, damaged: 0, missing: 0, repaired: 0 } })]} onClose={() => {}} />);
    expect(await screen.findByText('All 12 files check out.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Repair' })).toBeNull();
  });

  it('shows why a check could not start', async () => {
    api.verifyGame.mockRejectedValue(new Error('Echo Tiny is not installed'));
    render(<VerifyDialog game={game()} jobs={[]} onClose={() => {}} />);
    expect(await screen.findByText('Echo Tiny is not installed')).toBeTruthy();
  });
});

describe('MoveDialog', () => {
  const check: MoveCheck = { to: '/mnt/disk/tiny', need_bytes: 3_000_000, free_bytes: 90_000_000_000, plugin_moves: false };

  it('shows where the game goes and the room there, then queues the move', async () => {
    api.pickMoveFolder.mockResolvedValue('/mnt/disk');
    api.checkMove.mockResolvedValue(check);
    api.moveGame.mockResolvedValue(job({ id: 'move-1', kind: 'move', destination: '/mnt/disk/tiny' }));
    const onQueued = vi.fn();
    render(<MoveDialog game={game()} startFolder="/games/echo" onClose={() => {}} onQueued={onQueued} />);

    fireEvent.click(screen.getByRole('button', { name: 'Choose folder…' }));
    expect(await screen.findByText('/mnt/disk/tiny')).toBeTruthy();
    expect(screen.getByText(formatBytes(3_000_000))).toBeTruthy();
    expect(screen.getByText(formatBytes(90_000_000_000))).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Move' }));
    await waitFor(() => expect(onQueued).toHaveBeenCalledOnce());
    expect(api.moveGame).toHaveBeenCalledWith(expect.objectContaining({ id: 'p.echo.tiny' }), '/mnt/disk');
  });

  it('shows the refusal and keeps Move off', async () => {
    api.pickMoveFolder.mockResolvedValue('/usr');
    api.checkMove.mockRejectedValue(new Error('refusing to move into system folder /usr/tiny'));
    render(<MoveDialog game={game()} startFolder="/games/echo" onClose={() => {}} onQueued={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Choose folder…' }));
    expect(await screen.findByRole('alert')).toBeTruthy();
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Move' }).disabled).toBe(true);
    expect(api.moveGame).not.toHaveBeenCalled();
  });

  it('does nothing when the folder picker is cancelled', async () => {
    api.pickMoveFolder.mockResolvedValue('');
    render(<MoveDialog game={game()} startFolder="/games/echo" onClose={() => {}} onQueued={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Choose folder…' }));
    await waitFor(() => expect(api.pickMoveFolder).toHaveBeenCalledWith('/games/echo'));
    expect(api.checkMove).not.toHaveBeenCalled();
  });
});
