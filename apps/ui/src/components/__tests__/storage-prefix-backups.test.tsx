// Issue #163: the prefix backups a reset left show up in Storage, in the
// classic presentation too (the Atlas one is covered with its panels).
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { DiskUsage, Game } from '../../types';

const api = vi.hoisted(() => ({ getDiskUsage: vi.fn(), getGameFolders: vi.fn(), listJobs: vi.fn(), prefixBackups: vi.fn(), deletePrefixBackup: vi.fn() }));
const libraryListeners = vi.hoisted(() => [] as ((id: string) => void)[]);
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...args: unknown[]) => (api as Record<string, (...a: unknown[]) => unknown>)[k](...args) }),
  onDiskUsage: () => () => {},
  onLibrary: (cb: (id: string) => void) => {
    libraryListeners.push(cb);
    return () => {};
  },
}));
vi.mock('../../lib/theme', async (importOriginal) => ({
  ...await importOriginal<typeof import('../../lib/theme')>(),
  // The classic themes: warm, cool and slate draw the layout blocks.
  useVariant: (axis: string) => (axis === 'shell.navigation' ? 'sidebar' : 'page'),
  useThemeLayout: () => null,
}));

import { StorageView } from '../StorageView';

const game = { id: 'inside', source: 'steam', store: 'steam', store_app_id: 'inside', title: 'Inside', cover_url: '', banner_url: '', hero_url: '', install_dir: '/home/games/Inside', executable: '/home/games/Inside/run', is_installed: true, size_bytes: 1e9, settings: {}, features: [] } as unknown as Game;
const usage: DiskUsage = {
  drives: [{ id: '/', mount: '/', folders: ['/home/games'], fs_type: '', device_kind: '', total_bytes: 100e9, free_bytes: 30e9, games_bytes: 10e9, other_bytes: 60e9 }],
  games: [{ game_id: 'inside', title: 'Inside', source: 'steam', store: 'steam', install_dir: '/home/games/Inside', size_bytes: 1e9, prefix_bytes: 0, from_plugin: false, measured: true }],
  scanning: false,
  generated_at: 1,
};

beforeEach(() => {
  vi.clearAllMocks();
  libraryListeners.length = 0;
  api.getDiskUsage.mockReturnValue(Effect.succeed(usage));
  api.getGameFolders.mockReturnValue(Effect.succeed([]));
  api.listJobs.mockReturnValue(Effect.succeed([]));
  api.prefixBackups.mockReturnValue(Effect.succeed([
    { name: 'inside.bak-1700000000', path: '/prefixes/inside.bak-1700000000', bytes: 3e9, at: 1700000000 },
  ]));
  api.deletePrefixBackup.mockReturnValue(Effect.succeed(3e9));
});

describe('prefix backups in the classic Storage page', () => {
  it('asks before deleting one, naming it and its size, and says a backup can hold saves', async () => {
    const onNotify = vi.fn();
    render(<StorageView games={[game]} onNotify={onNotify} onRemove={() => {}} onUninstall={() => {}} />);
    expect(await screen.findByText('Prefix backups')).toBeInTheDocument();
    expect(screen.getByText('inside.bak-1700000000')).toBeInTheDocument();
    expect(screen.getByText('3.0 GB')).toBeInTheDocument();
    expect(screen.getByText(/may still hold saves/)).toBeInTheDocument();
    // The trash only arms the confirm: nothing is deleted yet.
    fireEvent.click(screen.getByRole('button', { name: 'Delete inside.bak-1700000000' }));
    expect(screen.getByText('Delete this prefix backup?')).toBeInTheDocument();
    expect(screen.getByText('The old Wine prefix of Inside (inside.bak-1700000000, 3.0 GB) is deleted for good, with any saves it still holds.')).toBeInTheDocument();
    expect(api.deletePrefixBackup).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Delete for good' }));
    await waitFor(() => expect(api.deletePrefixBackup).toHaveBeenCalledWith('/prefixes/inside.bak-1700000000'));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Deleted inside.bak-1700000000 and freed 3.0 GB.', 'ok'));
    expect(screen.queryByText('inside.bak-1700000000')).toBeNull();
  });

  it('cancelling the confirm keeps the backup', async () => {
    render(<StorageView games={[game]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Delete inside.bak-1700000000' }));
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByText('Delete this prefix backup?')).toBeNull();
    expect(api.deletePrefixBackup).not.toHaveBeenCalled();
    expect(screen.getByText('inside.bak-1700000000')).toBeInTheDocument();
  });

  it('re-reads the backups when the library changes, so a reset shows up', async () => {
    render(<StorageView games={[game]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    await screen.findByText('inside.bak-1700000000');
    expect(api.prefixBackups).toHaveBeenCalledTimes(1);
    api.prefixBackups.mockReturnValue(Effect.succeed([
      { name: 'inside.bak-1700000000', path: '/prefixes/inside.bak-1700000000', bytes: 3e9, at: 1700000000 },
      { name: 'inside.bak-1700000001', path: '/prefixes/inside.bak-1700000001', bytes: 4e9, at: 1700000001 },
    ]));
    const before = api.prefixBackups.mock.calls.length;
    act(() => { libraryListeners.forEach((cb) => cb('inside')); });
    expect(await screen.findByText('inside.bak-1700000001')).toBeInTheDocument();
    expect(api.prefixBackups.mock.calls.length).toBeGreaterThan(before);
  });
});
