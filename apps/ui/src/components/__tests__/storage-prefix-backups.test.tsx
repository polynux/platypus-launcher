// Issue #163: the prefix backups a reset left show up in Storage, in the
// classic presentation too (the Atlas one is covered with its panels).
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { DiskUsage, Game } from '../../types';

const api = vi.hoisted(() => ({ getDiskUsage: vi.fn(), getGameFolders: vi.fn(), listJobs: vi.fn(), prefixBackups: vi.fn(), deletePrefixBackup: vi.fn() }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...args: unknown[]) => (api as Record<string, (...a: unknown[]) => unknown>)[k](...args) }),
  onDiskUsage: () => () => {},
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
  api.getDiskUsage.mockReturnValue(Effect.succeed(usage));
  api.getGameFolders.mockReturnValue(Effect.succeed([]));
  api.listJobs.mockReturnValue(Effect.succeed([]));
  api.prefixBackups.mockReturnValue(Effect.succeed([
    { name: 'inside.bak-1700000000', path: '/prefixes/inside.bak-1700000000', bytes: 3e9, at: 1700000000 },
  ]));
  api.deletePrefixBackup.mockReturnValue(Effect.succeed(3e9));
});

describe('prefix backups in the classic Storage page', () => {
  it('lists what a reset left and deletes one', async () => {
    const onNotify = vi.fn();
    render(<StorageView games={[game]} onNotify={onNotify} onRemove={() => {}} onUninstall={() => {}} />);
    expect(await screen.findByText('Prefix backups')).toBeInTheDocument();
    expect(screen.getByText('inside.bak-1700000000')).toBeInTheDocument();
    expect(screen.getByText('3.0 GB')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete inside.bak-1700000000' }));
    await waitFor(() => expect(api.deletePrefixBackup).toHaveBeenCalledWith('/prefixes/inside.bak-1700000000'));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Deleted inside.bak-1700000000 and freed 3.0 GB.', 'ok'));
    expect(screen.queryByText('inside.bak-1700000000')).toBeNull();
  });
});
