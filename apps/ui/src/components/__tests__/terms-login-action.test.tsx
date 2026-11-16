import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, InstallLocation, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  getGameTerms: vi.fn(async () => []), // no pending terms: the install goes through (protocol 1.11)
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

vi.mock('../../lib/notify', () => ({ notify: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import { InstallDialog } from '../InstallDialog';
import { notify } from '../../lib/notify';
import { setStoreLoginOpener } from '../../lib/storeLogin';

const GB = 1e9;
const game = { id: 'p.echo.licensed', source: 'echo', store: 'echo', store_app_id: 'licensed', title: 'Echo Licensed', is_installed: false, installable: true, features: ['install'] } as unknown as Game;
const plugin = { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['install'], declared: ['install'] } as unknown as StorePlugin;
const places: InstallLocation[] = [
  { folder: '/home/u/Games/echo', kind: 'default', free_bytes: 120 * GB, exists: true },
];
const loginError = () => {
  // The core's terms.needsLogin refusal, with the translatable payload.
  const err = new Error('Log in to the store first: it can\'t list the terms of Echo Licensed until you do.');
  const shaped: Error & { data?: unknown } = err;
  shaped.data = { code: 'terms.needsLogin', params: { title: 'Echo Licensed' } };
  return err;
};

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getInstallSize.mockResolvedValue(30 * GB);
  api.getInstallLocations.mockResolvedValue(places);
  vi.mocked(notify.error).mockReset();
  setStoreLoginOpener(null);
  api.getGameTerms.mockResolvedValue([]);
});

describe('InstallDialog terms login', () => {
  it('offers "Log in" when the store needs a login for its terms, opening its login flow', async () => {
    // The real sequence: the dialog reads the terms first, and the read
    // itself carries terms.needsLogin (protocol 1.11, "Terms").
    api.getGameTerms.mockRejectedValue(loginError());
    const open = vi.fn();
    setStoreLoginOpener(open);
    const onNotify = vi.fn();
    render(<InstallDialog game={game} plugin={plugin} onClose={() => {}} onChanged={() => {}} onNotify={onNotify} />);
    await screen.findByRole('radiogroup', { name: 'Install location' });
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    await waitFor(() => expect(notify.error).toHaveBeenCalled());
    // Nothing is queued, and the plain error toast stays off: the action
    // toast explains and offers the login.
    expect(api.installGameTo).not.toHaveBeenCalled();
    expect(onNotify).not.toHaveBeenCalled();
    const [msg, options] = vi.mocked(notify.error).mock.calls.at(-1) as [string, { action: { label: string; onClick: () => void } }];
    expect(msg).toMatch(/Log in to the store first/);
    expect(options.action.label).toBe('Log in');
    options.action.onClick();
    expect(open).toHaveBeenCalledWith('echo');
  });

  it('keeps the plain error toast for other refusals', async () => {
    api.installGameTo.mockRejectedValue(new Error('disk is full'));
    const onNotify = vi.fn();
    render(<InstallDialog game={game} plugin={plugin} onClose={() => {}} onChanged={() => {}} onNotify={onNotify} />);
    await screen.findByRole('radiogroup', { name: 'Install location' });
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('disk is full', 'err'));
    expect(notify.error).not.toHaveBeenCalled();
  });
});
