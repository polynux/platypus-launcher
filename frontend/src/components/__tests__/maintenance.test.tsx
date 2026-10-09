import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, Job, MoveCheck, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  verifyGame: vi.fn(),
  repairGame: vi.fn(),
  pickMoveFolder: vi.fn(),
  checkMove: vi.fn(),
  moveGame: vi.fn(),
  getGameDetails: vi.fn(),
  getGameVersions: vi.fn(),
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
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

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
  onChangeSettings() {}, onOpenSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
});

describe('Verify and move on the game page', () => {
  it('offers Verify files only when the plugin offers verify', () => {
    const { unmount } = render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    expect(screen.getByRole('button', { name: /Verify files/ })).toBeTruthy();
    unmount();
    render(<GamePage game={game({ features: null })} plugin={plugin(['install'])} {...base} />);
    expect(screen.queryByRole('button', { name: /Verify files/ })).toBeNull();
  });

  it('offers Move for an installed plugin game, whatever the plugin offers', () => {
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    expect(screen.getByRole('button', { name: /Move…/ })).toBeTruthy();
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
