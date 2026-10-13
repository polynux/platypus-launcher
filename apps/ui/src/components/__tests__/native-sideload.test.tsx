import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';
import type { Game, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  pickGameExecutable: vi.fn(),
  pickFolder: vi.fn(),
  detectGameExecutable: vi.fn(),
  isGameExecutable: vi.fn(),
  makeGameExecutable: vi.fn(),
  updateCustomGameExecutable: vi.fn(),
  previewCommand: vi.fn(),
  getLauncherSettings: vi.fn(),
  getGameDetails: vi.fn(),
  getGameVersions: vi.fn(),
  getCustomization: vi.fn(),
  getCloudInfo: vi.fn(),
  getGameSettingsFields: vi.fn(),
  getGameOptions: vi.fn(),
  getInstallSize: vi.fn(),
  checkRemove: vi.fn(),
  removeGame: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
  listSaveBackups: vi.fn(),
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
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { AddGameModal, type AddGameOptions } from '../AddGameModal';
import { GamePage } from '../GamePage';

const custom = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom', title: 'Native Gem',
  cover_url: '', banner_url: '', install_dir: '/games/gem', executable: '/games/gem/gem',
  is_installed: true, features: null, native: true,
  settings: { runner_id: 'auto', env_vars: {}, use_gamemode: true },
  ...o,
}) as Game;

const base = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.detectGameExecutable.mockResolvedValue({ native: true, kind: 'elf' });
  api.isGameExecutable.mockResolvedValue(true);
  api.previewCommand.mockResolvedValue('/games/gem/gem');
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' });
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getCustomization.mockResolvedValue({ title: '', sort_title: '', art: {} });
  api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: { state: 'synced', checkedAt: 1, counts: { same: 1, download: 0, upload: 0, conflict: 0 } } });
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.checkRemove.mockResolvedValue({
    targets: [{ kind: 'files', path: '/games/gem', bytes: 2_000_000, exists: true, allowed: true }],
    saves: { backup_id: '', files: 0, bytes: 0, no_paths: false },
  });
  api.getSavePaths.mockResolvedValue([]);
  api.getGameSaveBackup.mockResolvedValue('');
  api.getAutoBackupState.mockResolvedValue({ on: true, reason: 'on' });
  api.listSaveBackups.mockResolvedValue([]);
});

const openSettings = () => fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));

describe('AddGameModal run with (issue #47)', () => {
  function open(onAdd: (title: string, exe: string, opts: AddGameOptions) => Promise<void> | void = vi.fn()) {
    const onClose = vi.fn();
    render(<AddGameModal onClose={onClose} onAdd={onAdd} />);
    return { onClose, onAdd };
  }

  it('shows the detected value as the default and hides the prefix for native games', async () => {
    open();
    fireEvent.change(screen.getByLabelText('executable'), { target: { value: '/games/gem/gem' } });
    await waitFor(() => expect(api.detectGameExecutable).toHaveBeenCalledWith('/games/gem/gem'));
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    expect(await screen.findByText('Automatic (detected: native)')).toBeTruthy();
    expect(screen.queryByLabelText('wine prefix')).toBeNull();
  });

  it('overrides detection and passes the run mode on', async () => {
    const onAdd = vi.fn();
    open(onAdd);
    fireEvent.change(screen.getByPlaceholderText('My Game'), { target: { value: 'Gem' } });
    fireEvent.change(screen.getByLabelText('executable'), { target: { value: '/games/gem/gem' } });
    fireEvent.click(screen.getByRole('button', { name: /Advanced/ }));
    fireEvent.change(screen.getByLabelText('Run with'), { target: { value: 'wine' } });
    // Overridden to Wine: the prefix comes back.
    expect(screen.getByLabelText('wine prefix')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Add game' }));
    expect(onAdd).toHaveBeenCalledWith('Gem', '/games/gem/gem', expect.objectContaining({ runMode: 'wine' }));
  });
});

describe('Maintenance for native games (issue #47)', () => {
  it('offers no Wine prefix target', async () => {
    render(<GamePage game={custom()} plugin={undefined} {...base} onRemove={() => {}} />);
    openSettings();
    fireEvent.click(screen.getByRole('tab', { name: 'Maintenance' }));
    fireEvent.click(screen.getByRole('button', { name: 'Remove from library' }));
    await waitFor(() => expect(api.checkRemove).toHaveBeenCalledWith('custom-1', { delete_prefix: false, delete_files: true, allow_no_saves: false }));
    expect(screen.queryByRole('checkbox', { name: /Also delete the Wine prefix/ })).toBeNull();
    expect(screen.getByRole('checkbox', { name: /Also delete the game files/ })).toBeTruthy();
  });

  it('hides the prefix options in the game settings', async () => {
    render(<GamePage game={custom()} plugin={undefined} {...base} />);
    openSettings();
    expect(screen.queryByLabelText('wine prefix')).toBeNull();
    // The working directory still applies.
    expect(screen.getByLabelText('working directory')).toBeInTheDocument();
    // Overriding to Wine brings the prefix back.
    const user = userEvent.setup();
    await user.click(screen.getByLabelText('Run with'));
    await user.click(await screen.findByRole('option', { name: 'Wine / Proton' }));
    expect(await screen.findByLabelText('wine prefix')).toBeTruthy();
  });
});

describe('Make it executable (issue #47)', () => {
  it('offers to set the bit when it is missing', async () => {
    api.isGameExecutable.mockResolvedValue(false);
    api.makeGameExecutable.mockImplementation(async () => custom());
    render(<GamePage game={custom()} plugin={undefined} {...base} />);
    openSettings();
    const button = await screen.findByRole('button', { name: 'Make it executable' });
    fireEvent.click(button);
    await waitFor(() => expect(api.makeGameExecutable).toHaveBeenCalledWith('custom-1'));
  });

  it('stays quiet when the file is already executable', async () => {
    render(<GamePage game={custom()} plugin={undefined} {...base} />);
    openSettings();
    await waitFor(() => expect(api.isGameExecutable).toHaveBeenCalledWith('custom-1'));
    expect(screen.queryByRole('button', { name: 'Make it executable' })).toBeNull();
  });
});
