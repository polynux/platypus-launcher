import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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

const mk = (id: string, install_dir: string): Game => ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir, executable: `${install_dir}/run`, is_installed: true, outside_folder: id === 'outside', size_bytes: 3e9, settings: {}, features: [] }) as Game;
const usage: DiskUsage = {
  drives: [{ id: '/', mount: '/', folders: ['/home/games'], fs_type: '', device_kind: '', total_bytes: 100e9, free_bytes: 30e9, games_bytes: 10e9, other_bytes: 60e9 }],
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
  it('uses outside_folder and only lists canceled installs with downloaded bytes', async () => {
    const canceled = { id: 'left', kind: 'install', title: 'Canceled', state: 'canceled', bytes: 12e9, leftover: '' } as never;
    const moved = { id: 'move', kind: 'move', title: 'Moved', state: 'finished', bytes: 12e9, leftover: '/old' } as never;
    render(<StorageView games={[mk('inside', '/home/games/Inside'), mk('outside', '/home/Downloads/Outside'), { ...mk('store-folder', '/steam/library'), outside_folder: false } as Game]} jobs={[canceled, moved]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    expect(await screen.findByText('Default')).toBeInTheDocument();
    expect(screen.getByText('Outside your game folders')).toBeInTheDocument();
    expect(screen.getByText('/home/Downloads/Outside, 3.0 GB')).toBeInTheDocument();
    expect(screen.getByText('Canceled, canceled install')).toBeInTheDocument();
    expect(screen.queryByText('store-folder')).not.toBeInTheDocument();
    expect(screen.getByText('Moved: /old')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Clean 12.0 GB' }));
    await waitFor(() => expect(api.discardJob).toHaveBeenCalledWith('left'));
  });

  it('uses the filesystem label as the drive name and shows the device under it', async () => {
    api.getDiskUsage.mockReturnValue(Effect.succeed({
      ...usage,
      drives: [{ ...usage.drives[0], label: 'JEUX', device_kind: 'ssd', fs_type: 'btrfs' }],
    }));
    render(<StorageView games={[mk('inside', '/home/games/Inside')]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    expect(await screen.findByText('JEUX')).toBeInTheDocument();
    expect(screen.getByText('/home/games, SSD, btrfs')).toBeInTheDocument();
  });

  it('keeps what the user unticked after a clean', async () => {
    const job = (id: string, title: string) => ({ id, kind: 'install', title, state: 'canceled', bytes: 1e9, leftover: '' }) as never;
    const props = { games: [mk('inside', '/home/games/Inside')], onNotify: () => {}, onRemove: () => {}, onUninstall: () => {} };
    const { rerender } = render(<StorageView {...props} jobs={[job('a', 'Alpha'), job('b', 'Beta')]} />);
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Beta' }));
    fireEvent.click(screen.getByRole('button', { name: 'Clean 1.0 GB' }));
    await waitFor(() => expect(api.discardJob).toHaveBeenCalledWith('a'));
    expect(api.discardJob).not.toHaveBeenCalledWith('b');
    // Alpha is cleaned; Beta stays unticked.
    rerender(<StorageView {...props} jobs={[job('b', 'Beta')]} />);
    expect(screen.getByRole('checkbox', { name: 'Beta' })).not.toBeChecked();
  });

  it('does not match sibling paths by mount prefix', async () => {
    const driveUsage: DiskUsage = {
      ...usage,
      drives: [
        { ...usage.drives[0], id: '/home/games', mount: '/home/games', folders: ['/home/games'] },
        { ...usage.drives[0], id: '/home/games-old', mount: '/home/games-old', folders: ['/home/games-old'] },
      ],
      games: [{ ...usage.games[0], game_id: 'outside', title: 'Outside', install_dir: '/home/games-old/Outside' }],
    };
    api.getDiskUsage.mockReturnValue(Effect.succeed(driveUsage));
    render(<StorageView games={[mk('outside', '/home/games-old/Outside')]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    expect(within(await screen.findByRole('region', { name: '/home/games-old' })).getAllByText('Outside')).toHaveLength(2);
    expect(within(screen.getByRole('region', { name: '/home/games' })).queryByText('Outside')).toBeNull();
  });

  it('renders one move dialog for an outside-folder game', async () => {
    const outside = { ...mk('outside', '/home/Downloads/Outside'), outside_folder: true } as Game;
    render(<StorageView games={[outside]} onNotify={() => {}} onRemove={() => {}} onUninstall={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: /Move to games/i }));
    expect(screen.getAllByRole('dialog')).toHaveLength(1);
  });
});
