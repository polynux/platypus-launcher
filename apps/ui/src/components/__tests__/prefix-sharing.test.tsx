import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, Job, PrefixShare, StorePlugin } from '../../types';

// Shared Wine prefixes (issue #52): the prefix picker and the "Shared with"
// lines in the settings and the remove dialog.

const api = vi.hoisted(() => ({
  getLauncherSettings: vi.fn(),
  prefixUsage: vi.fn(),
  previewCommand: vi.fn(),
  detectGameExecutable: vi.fn(),
  isGameExecutable: vi.fn(),
  pickFolder: vi.fn(),
  pickGameExecutable: vi.fn(),
  updateCustomGameExecutable: vi.fn(),
  makeGameExecutable: vi.fn(),
  checkRemove: vi.fn(),
  removeGame: vi.fn(),
  getSavePaths: vi.fn(),
  getAutoBackupState: vi.fn(),
  listSaveBackups: vi.fn(),
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
  onLibraryPartial: () => () => {},
}));

import { SettingsModal } from '../SettingsModal';
import { GamePage } from '../GamePage';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom',
  title: 'Sideloaded Gem', cover_url: '', banner_url: '',
  install_dir: '/games/sideload', executable: '/games/sideload/gem.exe',
  is_installed: true, features: null,
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;

const job = (o: Partial<Job> = {}): Job =>
  ({ id: 'j1', kind: 'remove', source: 'custom', game_id: 'custom-1', store_game_id: 'custom', title: 'Sideloaded Gem', state: 'queued', ...o }) as Job;

const base = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/pfx' });
  api.prefixUsage.mockResolvedValue([]);
  api.previewCommand.mockResolvedValue('wine game.exe');
  api.detectGameExecutable.mockResolvedValue({ native: false, kind: 'windows-exe' });
  api.isGameExecutable.mockResolvedValue(true);
  api.pickFolder.mockResolvedValue('');
  api.pickGameExecutable.mockResolvedValue('');
  api.updateCustomGameExecutable.mockResolvedValue(game());
  api.makeGameExecutable.mockResolvedValue(game());
  api.checkRemove.mockResolvedValue({
    targets: [
      { kind: 'prefix', path: '/pfx/gem', bytes: 1_000_000, exists: true, allowed: true },
      { kind: 'files', path: '/games/sideload', bytes: 2_000_000, exists: true, allowed: true },
    ],
    saves: { backup_id: '', files: 0, bytes: 0, no_paths: false },
  });
  api.removeGame.mockResolvedValue(job({ id: 'remove-1' }));
  api.getSavePaths.mockResolvedValue([]);
  api.getAutoBackupState.mockResolvedValue({ on: true, reason: 'on' });
  api.listSaveBackups.mockResolvedValue([]);
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
});

/** Opens the game settings from the page's gear, then a tab by its name. */
const openTab = (name: RegExp | string) => {
  fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
  fireEvent.click(screen.getByRole('tab', { name }));
};

describe('The existing-prefix picker', () => {
  it('lists every prefix with its game count and saves the picked path', async () => {
    api.prefixUsage.mockResolvedValue([
      { path: '/pfx/shared', games: [{ id: 'custom-1', title: 'Sideloaded Gem' }, { id: 'custom-2', title: 'Other Gem' }] },
      { path: '/pfx/alone', games: [{ id: 'custom-9', title: 'Lonely Gem' }] },
    ] as PrefixShare[]);
    const onSave = vi.fn();
    render(<SettingsModal game={game()} runners={[]} onClose={() => {}} onSave={onSave} onCopied={() => {}} />);

    fireEvent.pointerDown(screen.getByRole('combobox', { name: 'use an existing prefix' }), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    expect(await screen.findByRole('option', { name: /\/pfx\/shared · 2 games/ })).toBeTruthy();
    expect(screen.getByRole('option', { name: /\/pfx\/alone · 1 game/ })).toBeTruthy();
    fireEvent.click(screen.getByRole('option', { name: /\/pfx\/shared · 2 games/ }));

    await waitFor(() => expect((screen.getByLabelText('wine prefix') as HTMLInputElement).value).toBe('/pfx/shared'));
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(onSave.mock.calls[0][0].wine_prefix).toBe('/pfx/shared');
  });
});

describe('The shared-prefix line in the settings', () => {
  it('names the sharers and warns only while the effective prefix is shared', async () => {
    api.prefixUsage.mockResolvedValue([
      { path: '/pfx/custom-1', games: [{ id: 'custom-1', title: 'Sideloaded Gem' }, { id: 'custom-2', title: 'Other Gem' }] },
    ] as PrefixShare[]);
    const view = render(<SettingsModal game={game()} runners={[]} onClose={() => {}} onSave={() => {}} onCopied={() => {}} />);
    expect(await screen.findByText('Shared with: Other Gem')).toBeTruthy();
    expect(screen.getByText('A broken shared prefix breaks every game in it.')).toBeTruthy();
    view.unmount();

    // The same prefix with no other game: nothing to share, nothing to warn.
    api.prefixUsage.mockResolvedValue([
      { path: '/pfx/custom-1', games: [{ id: 'custom-1', title: 'Sideloaded Gem' }] },
    ] as PrefixShare[]);
    render(<SettingsModal game={game()} runners={[]} onClose={() => {}} onSave={() => {}} onCopied={() => {}} />);
    // Wait for the usage answer to land: the picker lists its one prefix.
    fireEvent.pointerDown(screen.getByRole('combobox', { name: 'use an existing prefix' }), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    await screen.findByRole('option', { name: /1 game/ });
    expect(screen.queryByText(/Shared with/)).toBeNull();
    expect(screen.queryByText(/breaks every game/)).toBeNull();
  });
});

describe('The remove dialog on a shared prefix', () => {
  it('names every game still using the prefix and keeps its box blocked', async () => {
    api.checkRemove.mockResolvedValue({
      targets: [
        { kind: 'prefix', path: '/pfx/shared', bytes: 0, exists: true, allowed: false, reason: 'refusing to delete /pfx/shared: First still uses it', reason_code: 'remove.shared', reason_params: { path: '/pfx/shared', title: 'First' } },
        { kind: 'files', path: '/games/sideload', bytes: 2_000_000, exists: true, allowed: true },
      ],
      saves: { backup_id: '', files: 0, bytes: 0, no_paths: false },
    });
    api.prefixUsage.mockResolvedValue([
      { path: '/pfx/shared', games: [
        { id: 'custom-1', title: 'Sideloaded Gem' },
        { id: 'custom-2', title: 'First' },
        { id: 'custom-3', title: 'Second' },
      ] },
    ] as PrefixShare[]);
    const onRemove = vi.fn();
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} onRemove={onRemove} />);
    openTab('Maintenance');
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));

    expect(await screen.findByText('Shared with: First, Second')).toBeTruthy();
    const prefix = screen.getByRole<HTMLInputElement>('checkbox', { name: /Also delete the Wine prefix/ });
    expect(prefix.disabled).toBe(true); // blocked: a real click can't tick it
    expect(prefix.checked).toBe(false);
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    expect(onRemove).toHaveBeenCalledWith({ delete_prefix: false, delete_files: false, allow_no_saves: false });
    expect(api.prefixUsage).toHaveBeenCalled();
  });
});
