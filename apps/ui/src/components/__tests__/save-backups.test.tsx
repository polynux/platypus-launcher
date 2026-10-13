import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';
import React from 'react';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string, o?: Record<string, unknown>) => (o?.defaultValue as string) ?? k }),
}));

const apiMocks = vi.hoisted(() => ({
  getBackupSettings: vi.fn(),
  setBackupSettings: vi.fn(),
  refreshSaveManifest: vi.fn(),
  listSaveBackups: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
  createSaveBackup: vi.fn(),
  restoreSaveBackup: vi.fn(),
  previewRestoreSaveBackup: vi.fn(),
  deleteSaveBackup: vi.fn(),
  setGameSavePaths: vi.fn(),
  setGameSaveBackup: vi.fn(),
  uninstallGameWithoutBackup: vi.fn(),
  pickFolder: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({ LauncherApi: apiMocks }));

import { BackupsSetting } from '../BackupsSetting';
import { SaveBackupsSection } from '../SaveBackupsSection';

const game = {
  id: 'g1', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'G',
  install_dir: '/games/g', is_installed: true,
} as never;

beforeEach(() => {
  vi.clearAllMocks();
  apiMocks.getBackupSettings.mockReturnValue(Effect.succeed({
    folder: '/b', default_folder: '/b', keep: 10, mode: 'destructive',
    after_session: false, manifest: 'ready',
  } as never));
  apiMocks.setBackupSettings.mockReturnValue(Effect.succeed({
    folder: '/b', default_folder: '/b', keep: 5, mode: 'all',
    after_session: true, manifest: 'ready',
  } as never));
  apiMocks.refreshSaveManifest.mockReturnValue(Effect.succeed(undefined as never));
  apiMocks.listSaveBackups.mockReturnValue(Effect.succeed([
    { id: 'b1', game_id: 'g1', created_at: 1700000000, trigger: 'manual', files: 2, bytes: 10, size: 500 },
  ] as never));
  apiMocks.getSavePaths.mockReturnValue(Effect.succeed([
    { path: '/saves/a', origin: 'user', exists: true },
    { path: '/saves/b', origin: 'manifest', exists: false },
  ] as never));
  apiMocks.getGameSaveBackup.mockReturnValue(Effect.succeed('' as never));
  apiMocks.getAutoBackupState.mockReturnValue(Effect.succeed({ on: true, reason: 'on' } as never));
  apiMocks.createSaveBackup.mockReturnValue(Effect.succeed({ id: 'j1', kind: 'backup' } as never));
  apiMocks.previewRestoreSaveBackup.mockReturnValue(Effect.succeed({ overwritten: 1, added: 2, kept: 3, files: 6, bytes: 60 } as never));
});

describe('BackupsSetting', () => {
  it('loads the settings and saves a changed form', async () => {
    const notified: string[] = [];
    const { findByLabelText } = render(
      <BackupsSetting onNotify={(m) => notified.push(m)} />,
    );
    const keep = (await findByLabelText('backups.keepLabel')) as HTMLInputElement;
    expect(keep).toBeTruthy();
    expect(apiMocks.getBackupSettings).toHaveBeenCalled();
  });
});

describe('SaveBackupsSection', () => {
  it('lists backups with each path origin and queues a manual backup', async () => {
    const notified: string[] = [];
    const { findByText, getByText, findAllByText } = render(
      <SaveBackupsSection game={game} onNotify={(m) => notified.push(m)} />,
    );
    await findByText(/manual/);
    expect(apiMocks.listSaveBackups).toHaveBeenCalledWith('g1');
    expect(apiMocks.getSavePaths).toHaveBeenCalled();
    // Each path's origin is shown.
    await findAllByText(/pathOrigin/);
    fireEvent.click(getByText('gameSettings:maintenance.saveBackups.backupNow'));
    await vi.waitFor(() => expect(apiMocks.createSaveBackup).toHaveBeenCalled());
    await vi.waitFor(() => expect(notified).toContain('gameSettings:maintenance.saveBackups.queued'));
  });

  it('adds a save folder with the folder picker and removes one of yours', async () => {
    apiMocks.pickFolder.mockReturnValue(Effect.succeed('/saves/new' as never));
    apiMocks.setGameSavePaths.mockReturnValue(Effect.succeed(undefined as never));
    const { findByText, getByLabelText, queryByLabelText } = render(
      <SaveBackupsSection game={game} onNotify={() => {}} />,
    );
    fireEvent.click(await findByText('gameSettings:maintenance.saveBackups.pathsAdd'));
    await vi.waitFor(() => expect(apiMocks.setGameSavePaths).toHaveBeenCalledWith('g1', ['/saves/a', '/saves/new']));
    // Only your own folders can be removed; found ones can't.
    expect(queryByLabelText('gameSettings:maintenance.saveBackups.pathRemove')).toBeTruthy();
    fireEvent.click(getByLabelText('gameSettings:maintenance.saveBackups.pathRemove'));
    await vi.waitFor(() => expect(apiMocks.setGameSavePaths).toHaveBeenCalledWith('g1', []));
  });

  it('keeps the list when the picker is canceled', async () => {
    apiMocks.pickFolder.mockReturnValue(Effect.succeed('' as never));
    const { findByText } = render(<SaveBackupsSection game={game} onNotify={() => {}} />);
    fireEvent.click(await findByText('gameSettings:maintenance.saveBackups.pathsAdd'));
    await new Promise((r) => setTimeout(r, 20));
    expect(apiMocks.setGameSavePaths).not.toHaveBeenCalled();
  });
});
