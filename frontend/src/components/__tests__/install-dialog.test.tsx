import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, InstallLocation, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getInstallLocations: vi.fn(),
  checkInstallFolder: vi.fn(),
  pickInstallFolder: vi.fn(),
  installGameTo: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { InstallDialog } from '../InstallDialog';

const GB = 1e9;
const game = { id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', is_installed: false, installable: true, features: ['install'] } as unknown as Game;
const plugin = { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['install'], declared: ['install'] } as unknown as StorePlugin;
const places: InstallLocation[] = [
  { folder: '/home/u/Games/echo', kind: 'default', free_bytes: 120 * GB, exists: true },
  { folder: '/mnt/ssd/echo', kind: 'library', root: '/mnt/ssd', free_bytes: 8 * GB, exists: false },
  { folder: '/data/old', kind: 'recent', free_bytes: 500 * GB, exists: true },
];

function open() {
  const onClose = vi.fn(), onChanged = vi.fn(), onNotify = vi.fn();
  render(<InstallDialog game={game} plugin={plugin} onClose={onClose} onChanged={onChanged} onNotify={onNotify} />);
  return { onClose, onChanged, onNotify };
}
const installBtn = () => screen.getByRole('button', { name: 'Install' }) as HTMLButtonElement;

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getInstallSize.mockResolvedValue(30 * GB);
  api.getInstallLocations.mockResolvedValue(places);
  api.installGameTo.mockResolvedValue({ id: 'j' });
});

describe('InstallDialog ("Install to…")', () => {
  it('shows the size of the install and the places with their free space', async () => {
    open();
    expect((await screen.findByTestId('install-size')).textContent).toBe('30.0 GB');
    const group = await screen.findByRole('radiogroup', { name: 'Install location' });
    const radios = group.querySelectorAll('[role=radio]');
    expect(radios).toHaveLength(3);
    expect(radios[0].textContent).toMatch(/Default.*\/home\/u\/Games\/echo.*120 GB free/);
    expect(radios[1].textContent).toMatch(/Game folder.*\/mnt\/ssd\/echo.*8\.0 GB free/);
    expect(radios[2].textContent).toMatch(/Recent.*\/data\/old/);
    expect(radios[0].getAttribute('aria-checked')).toBe('true');
  });

  it('says so when the store cannot tell the size, and still installs', async () => {
    api.getInstallSize.mockRejectedValue(new Error('no size'));
    const { onClose, onChanged } = open();
    expect((await screen.findByTestId('install-size')).textContent).toBe('Size unknown');
    await screen.findByRole('radiogroup', { name: 'Install location' });
    expect(installBtn().disabled).toBe(false);
    fireEvent.click(installBtn());
    await waitFor(() => expect(api.installGameTo).toHaveBeenCalledWith(game, ''));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onChanged).toHaveBeenCalled();
  });

  it('installs into the place chosen, and blocks a place that is too small', async () => {
    open();
    await screen.findByText('30.0 GB');
    const group = await screen.findByRole('radiogroup', { name: 'Install location' });
    const radios = group.querySelectorAll('[role=radio]');
    fireEvent.click(radios[1]); // 8 GB free for a 30 GB game
    expect(screen.getByRole('alert').textContent).toMatch(/Not enough space/);
    expect(installBtn().disabled).toBe(true);
    fireEvent.click(radios[2]);
    expect(screen.queryByRole('alert')).toBeNull();
    fireEvent.click(installBtn());
    await waitFor(() => expect(api.installGameTo).toHaveBeenCalledWith(game, '/data/old'));
  });

  it('adds a folder picked by hand once core accepts it', async () => {
    api.pickInstallFolder.mockResolvedValue('/mnt/big');
    api.checkInstallFolder.mockResolvedValue({ folder: '/mnt/big', free_bytes: 900 * GB, exists: true });
    open();
    fireEvent.click(await screen.findByRole('button', { name: /Choose folder/ }));
    const chosen = await screen.findByRole('radio', { name: /Chosen.*\/mnt\/big/ });
    expect(chosen.getAttribute('aria-checked')).toBe('true');
    expect(chosen.textContent).toMatch(/900 GB free/);
    expect(api.pickInstallFolder).toHaveBeenCalledWith('/home/u/Games/echo');
    fireEvent.click(installBtn());
    await waitFor(() => expect(api.installGameTo).toHaveBeenCalledWith(game, '/mnt/big'));
  });

  it('keeps the choice and shows the reason when core refuses a folder', async () => {
    api.pickInstallFolder.mockResolvedValue('/usr/share');
    api.checkInstallFolder.mockRejectedValue(new Error('refusing to install into system folder /usr/share'));
    open();
    fireEvent.click(await screen.findByRole('button', { name: /Choose folder/ }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/system folder/);
    expect(screen.getAllByRole('radio')[0].getAttribute('aria-checked')).toBe('true');
    expect(screen.queryByRole('radio', { name: /usr\/share/ })).toBeNull();
  });

  it('does nothing when the picker is canceled', async () => {
    api.pickInstallFolder.mockResolvedValue('');
    open();
    fireEvent.click(await screen.findByRole('button', { name: /Choose folder/ }));
    await waitFor(() => expect(api.pickInstallFolder).toHaveBeenCalled());
    expect(api.checkInstallFolder).not.toHaveBeenCalled();
    expect(screen.getAllByRole('radio')).toHaveLength(3);
  });

  it('reports an install core refuses without closing', async () => {
    api.installGameTo.mockRejectedValue(new Error('you can\'t write to /mnt'));
    const { onNotify, onClose } = open();
    await screen.findByRole('radiogroup', { name: 'Install location' });
    fireEvent.click(installBtn());
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith("you can't write to /mnt", 'err'));
    expect(onClose).not.toHaveBeenCalled();
  });
});
