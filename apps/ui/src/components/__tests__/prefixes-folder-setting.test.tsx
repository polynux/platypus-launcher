import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getLauncherSettings: vi.fn(),
  setPrefixesFolder: vi.fn(),
  pickFolder: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import { PrefixesFolderSetting } from '../LauncherSettingsView';

const saved = {
  games_folder: '/home/you/Games',
  default_games_folder: '/home/you/Games',
  cancel_cleanup_days: 7,
  ratings_online: true,
  prefixes_folder: '',
  default_prefixes_folder: '/home/you/.local/share/platypus/prefixes',
};
const ok = <T,>(v: T) => Effect.succeed(v);

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getLauncherSettings.mockReturnValue(ok(saved));
});

function open(onNotify: (msg: string, kind?: 'ok' | 'err') => void = vi.fn()) {
  render(<PrefixesFolderSetting onNotify={onNotify} />);
  return { onNotify };
}

describe('PrefixesFolderSetting (issue #37)', () => {
  it('shows the default folder when none is set, with no reset button', async () => {
    open();
    const field = await screen.findByLabelText('Default Wine prefix folder');
    expect((field as HTMLInputElement).placeholder).toBe('/home/you/.local/share/platypus/prefixes');
    expect(screen.getByText(/Prefixes are created in/)).toHaveTextContent('/home/you/.local/share/platypus/prefixes');
    expect(screen.queryByRole('button', { name: 'Reset to default' })).toBeNull();
  });

  it('saves a folder picked by hand', async () => {
    const onNotify = vi.fn();
    api.pickFolder.mockReturnValue(ok('/mnt/big/prefixes'));
    api.setPrefixesFolder.mockReturnValue(ok({ ...saved, prefixes_folder: '/mnt/big/prefixes' }));
    open(onNotify);
    await screen.findByLabelText('Default Wine prefix folder');
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalled());
    expect((screen.getByLabelText('Default Wine prefix folder') as HTMLInputElement).value).toBe('/mnt/big/prefixes');
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(api.setPrefixesFolder).toHaveBeenCalledWith('/mnt/big/prefixes'));
    expect(onNotify).toHaveBeenCalledWith('Wine prefix folder saved');
  });

  it('leaves the field alone when the picker is cancelled', async () => {
    api.pickFolder.mockReturnValue(ok(''));
    open();
    await screen.findByLabelText('Default Wine prefix folder');
    fireEvent.click(screen.getByRole('button', { name: /Browse/ }));
    await waitFor(() => expect(api.pickFolder).toHaveBeenCalled());
    expect(api.setPrefixesFolder).not.toHaveBeenCalled();
    expect((screen.getByLabelText('Default Wine prefix folder') as HTMLInputElement).value).toBe('');
  });

  it('resets a custom folder to the default', async () => {
    const onNotify = vi.fn();
    api.getLauncherSettings.mockReturnValue(ok({ ...saved, prefixes_folder: '/mnt/big/prefixes' }));
    api.setPrefixesFolder.mockReturnValue(ok(saved));
    open(onNotify);
    fireEvent.click(await screen.findByRole('button', { name: 'Reset to default' }));
    await waitFor(() => expect(api.setPrefixesFolder).toHaveBeenCalledWith(''));
    expect(onNotify).toHaveBeenCalledWith('Wine prefix folder reset to the default');
  });

  it('shows why a folder was refused', async () => {
    api.setPrefixesFolder.mockReturnValue(Effect.fail(new Error('a folder must be an absolute path')));
    open();
    await screen.findByLabelText('Default Wine prefix folder');
    fireEvent.change(screen.getByLabelText('Default Wine prefix folder'), { target: { value: 'relative/dir' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/absolute path/);
  });

  it('says changing the folder never moves existing prefixes, which stay pinned', async () => {
    open();
    await screen.findByLabelText('Default Wine prefix folder');
    expect(screen.getByText(/does not move or delete existing prefixes/)).toBeInTheDocument();
    expect(screen.getByText(/pins it in their settings/)).toBeInTheDocument();
  });
});
