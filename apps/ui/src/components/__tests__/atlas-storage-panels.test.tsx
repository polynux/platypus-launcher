import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { DiskUsage, Game } from '../../types';

const api = vi.hoisted(() => ({ getDiskUsage: vi.fn(), getGameFolders: vi.fn(), listJobs: vi.fn(), discardJob: vi.fn() }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...args: unknown[]) => (api as Record<string, (...a: unknown[]) => unknown>)[k](...args) }),
  onDiskUsage: () => () => {},
}));
vi.mock('../../lib/theme', async (importOriginal) => ({
  ...await importOriginal<typeof import('../../lib/theme')>(),
  useVariant: (axis: string) => axis === 'shell.navigation' ? 'sourcebar' : 'page',
}));

import { StorageView } from '../StorageView';

const mk = (id: string, install_dir: string): Game => ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir, executable: `${install_dir}/run`, is_installed: true, size_bytes: 3e9, settings: {}, features: [] }) as Game;
const usage: DiskUsage = {
  drives: [{ id: '/', mount: '/', folders: ['/home/games'], total_bytes: 100e9, free_bytes: 30e9, games_bytes: 10e9, other_bytes: 60e9 }],
  games: [{ game_id: 'inside', title: 'Inside', source: 'steam', store: 'steam', install_dir: '/home/games/Inside', size_bytes: 10e9, prefix_bytes: 0, from_plugin: false, measured: true }],
  scanning: false, generated_at: 1,
};

beforeEach(() => {
  api.getDiskUsage.mockReturnValue(Effect.succeed(usage));
  api.getGameFolders.mockReturnValue(Effect.succeed([{ path: '/home/games', is_default: true, exists: true, free_bytes: 30e9 }]));
  api.listJobs.mockReturnValue(Effect.succeed([{ id: 'left', kind: 'install', source: 'steam', game_id: 'old', store_game_id: 'old', title: 'Canceled', state: 'canceled', bytes: 12e9, total: 0, speed: 0, percent: 0, attempts: 1, created_at: 1, updated_at: 1, leftover: '/tmp/left' }]));
  api.discardJob.mockReturnValue(Effect.succeed(undefined as never));
});

describe('Atlas Storage panels', () => {
  it('marks the default drive, lists games outside folders, and cleans selected leftovers', async () => {
    render(<StorageView games={[mk('inside', '/home/games/Inside'), mk('outside', '/home/Downloads/Outside')]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    expect(await screen.findByText('Default')).toBeInTheDocument();
    expect(screen.getByText('Outside your game folders')).toBeInTheDocument();
    expect(screen.getByText('/home/Downloads/Outside, 3.0 GB')).toBeInTheDocument();
    expect(screen.getByText('Canceled, canceled install')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Clean 12.0 GB' }));
    await waitFor(() => expect(api.discardJob).toHaveBeenCalledWith('left'));
  });
});
