import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getGameFolders: vi.fn(),
  addGameFolder: vi.fn(),
  removeGameFolder: vi.fn(),
  makeDefaultGameFolder: vi.fn(),
  chooseGameFolder: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import { GameFoldersSettings } from '../GameFoldersSettings';

const def = { path: '/home/me/Games', is_default: true, exists: true, free_bytes: 200e9 };
const extra = { path: '/mnt/big/Games', is_default: false, exists: true, free_bytes: 900e9 };

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameFolders.mockReturnValue(Effect.succeed([def, extra]));
});

describe('GameFoldersSettings', () => {
  it('lists the folders and marks the default one, which cannot be removed', async () => {
    render(<GameFoldersSettings onNotify={() => {}} />);
    const list = await screen.findByRole('list', { name: 'Game folders' });
    expect(list).toHaveTextContent('/home/me/Games');
    expect(list).toHaveTextContent('default');
    expect(list).toHaveTextContent(/200 GB free(?! free)/);
    expect(screen.queryByRole('button', { name: 'Remove /home/me/Games' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Remove /mnt/big/Games' })).toBeInTheDocument();
  });

  it('adds a folder from the picker', async () => {
    const onNotify = vi.fn();
    api.chooseGameFolder.mockReturnValue(Effect.succeed('/mnt/new'));
    api.addGameFolder.mockReturnValue(Effect.succeed([def, extra, { ...extra, path: '/mnt/new' }]));
    render(<GameFoldersSettings onNotify={onNotify} />);
    await screen.findByRole('list');
    fireEvent.click(screen.getByRole('button', { name: /add folder/i }));
    await waitFor(() => expect(screen.getByRole('list')).toHaveTextContent('/mnt/new'));
    expect(api.addGameFolder).toHaveBeenCalledWith('/mnt/new');
    expect(onNotify).toHaveBeenCalledWith('Game folder added');
  });

  it('leaves the list alone when the picker is cancelled', async () => {
    api.chooseGameFolder.mockReturnValue(Effect.succeed(''));
    render(<GameFoldersSettings onNotify={() => {}} />);
    await screen.findByRole('list');
    fireEvent.click(screen.getByRole('button', { name: /add folder/i }));
    await waitFor(() => expect(api.chooseGameFolder).toHaveBeenCalled());
    expect(api.addGameFolder).not.toHaveBeenCalled();
  });

  it('shows why a folder was refused', async () => {
    api.chooseGameFolder.mockReturnValue(Effect.succeed('/etc'));
    api.addGameFolder.mockReturnValue(Effect.fail(new Error('cannot install into /etc: system folder')));
    render(<GameFoldersSettings onNotify={() => {}} />);
    await screen.findByRole('list');
    fireEvent.click(screen.getByRole('button', { name: /add folder/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/system folder/);
  });

  it('makes a library folder the default, and the old default stays in the list', async () => {
    const onNotify = vi.fn();
    const swapped = [{ ...extra, is_default: true }, { ...def, is_default: false }];
    api.makeDefaultGameFolder.mockReturnValue(Effect.succeed(swapped));
    render(<GameFoldersSettings onNotify={onNotify} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Make /mnt/big/Games the default folder' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Make /home/me/Games the default folder' })).toBeInTheDocument());
    expect(api.makeDefaultGameFolder).toHaveBeenCalledWith('/mnt/big/Games');
    expect(screen.queryByRole('button', { name: 'Remove /mnt/big/Games' })).toBeNull();
    expect(onNotify).toHaveBeenCalledWith('Default game folder changed');
  });

  it('removes a folder', async () => {
    api.removeGameFolder.mockReturnValue(Effect.succeed([def]));
    render(<GameFoldersSettings onNotify={() => {}} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Remove /mnt/big/Games' }));
    await waitFor(() => expect(screen.getByRole('list')).not.toHaveTextContent('/mnt/big'));
    expect(api.removeGameFolder).toHaveBeenCalledWith('/mnt/big/Games');
  });
});
