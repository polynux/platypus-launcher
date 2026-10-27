import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';
import type { AutoBackupState, Game, SavePath, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  verifyGame: vi.fn(),
  checkRemove: vi.fn(),
  removeGame: vi.fn(),
  getGameDetails: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
  listSaveBackups: vi.fn(),
  getGameSettingsFields: vi.fn(),
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
  onLibrary: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: ['install'],
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;

const base = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.checkRemove.mockResolvedValue({
    targets: [
      { kind: 'prefix', path: '/pfx/gem', bytes: 1_000_000, exists: true, allowed: true },
      { kind: 'files', path: '/games/sideload', bytes: 2_000_000, exists: true, allowed: true },
    ],
    saves: { backup_id: '', files: 0, bytes: 0, no_paths: false },
  });
  api.removeGame.mockResolvedValue({ id: 'remove-1' });
  api.getSavePaths.mockResolvedValue([]);
  api.getGameSaveBackup.mockResolvedValue('');
  api.getAutoBackupState.mockResolvedValue({ on: true, reason: 'on' });
  api.listSaveBackups.mockResolvedValue([]);
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
});

/** Opens the game settings from the page's gear, then a tab by its name. */
const openTab = (name: RegExp | string) => {
  fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
  fireEvent.click(screen.getByRole('tab', { name }));
};

/** Core calls that never answer: the first frame must already be complete. */
const pendAll = () => {
  const pending = () => new Promise<never>(() => {});
  api.listSaveBackups.mockReturnValue(pending());
  api.getSavePaths.mockReturnValue(pending());
  api.getGameSaveBackup.mockReturnValue(pending());
  api.getAutoBackupState.mockReturnValue(pending());
};

describe('Maintenance first frame (issue #56)', () => {
  it('renders its section structure before the core answers, never an empty frame', () => {
    pendAll();
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    openTab('Maintenance');
    // The Save backups section is all there: heading, action, list and path
    // placeholders with loading text in place, never null.
    expect(screen.getByText(/save backups/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /Back up now/ })).toBeTruthy();
    expect(screen.getByTestId('save-backups-loading')).toBeTruthy();
    expect(screen.getAllByText('Loading…').length).toBeGreaterThanOrEqual(3);
    // The uninstall row is static: no core call gates it.
    expect(screen.getByRole('button', { name: 'Uninstall' })).toBeTruthy();
    // The panel itself is non-empty on the very first render.
    const panel = screen.getByRole('tabpanel', { name: 'Maintenance' });
    expect(panel.textContent!.length).toBeGreaterThan(0);
  });

  it('shows loading, not the wrong line, in the uninstall confirm while the paths land', () => {
    pendAll();
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    expect(screen.getAllByText('Loading…').length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText(/Saves will be backed up first\./)).toBeNull();
    expect(screen.queryByText(/knows no save paths/)).toBeNull();
  });

  it('fills the confirm in without remounting, and fetches once per dialog open', async () => {
    let resolvePaths!: (v: SavePath[]) => void;
    let resolveAuto!: (v: AutoBackupState) => void;
    api.listSaveBackups.mockResolvedValue([]);
    api.getSavePaths.mockReturnValue(new Promise<SavePath[]>((r) => { resolvePaths = r; }));
    api.getGameSaveBackup.mockResolvedValue('');
    api.getAutoBackupState.mockReturnValue(new Promise<AutoBackupState>((r) => { resolveAuto = r; }));
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    const panel = screen.getByRole('tabpanel', { name: 'Maintenance' });
    expect(screen.getAllByText('Loading…').length).toBeGreaterThanOrEqual(1);
    resolvePaths([]);
    resolveAuto({ on: true, reason: 'on' } as AutoBackupState);
    // No save paths known, no cloud: the checkbox line lands in the same panel.
    expect(await screen.findByText(/knows no save paths/)).toBeTruthy();
    expect(screen.getByRole('tabpanel', { name: 'Maintenance' })).toBe(panel);
    // One fetch per dialog open, shared by the tab: the dialog-open prefetch
    // answered the section and the confirm, with no refetch on either.
    expect(api.listSaveBackups).toHaveBeenCalledTimes(1);
    expect(api.getSavePaths).toHaveBeenCalledTimes(1);
    expect(api.getGameSaveBackup).toHaveBeenCalledTimes(1);
    expect(api.getAutoBackupState).toHaveBeenCalledTimes(1);
  });

  it('renders the sideloaded first frame before the removal check lands', () => {
    pendAll();
    api.checkRemove.mockReturnValue(new Promise<never>(() => {}));
    render(<GamePage game={game({
      id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom',
      title: 'Sideloaded Gem', install_dir: '/games/sideload', executable: '/games/sideload/gem',
      features: null,
    })} {...base} onRemove={() => {}} />);
    openTab('Maintenance');
    expect(screen.getByText(/save backups/i)).toBeTruthy();
    expect(screen.getByTestId('save-backups-loading')).toBeTruthy();
    // The Remove row is static; only the confirm panel waits on the check.
    expect(screen.getByRole('button', { name: 'Remove from library' })).toBeTruthy();
  });

  it('shows loading in the plugin settings tab before the plugin answers', () => {
    api.getGameSettingsFields.mockReturnValue(new Promise<never>(() => {}));
    // features null: the game has all of the plugin's capabilities.
    render(<GamePage game={game({ features: null })} plugin={plugin(['install', 'settings'])} {...base} />);
    openTab('Plugin settings');
    expect(screen.getByText('Loading…')).toBeTruthy();
  });
});
