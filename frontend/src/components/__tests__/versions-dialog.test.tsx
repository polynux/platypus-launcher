import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, StorePlugin, VersionInfo } from '../../types';

const api = vi.hoisted(() => ({
  getGameVersions: vi.fn(),
  unlockVersionChannel: vi.fn(),
  pinGameVersion: vi.fn(),
  unpinGameVersion: vi.fn(),
  installGameVersion: vi.fn(),
  updateGameVersion: vi.fn(),
  updateGame: vi.fn(),
}));
const openUrl = vi.hoisted(() => vi.fn());

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    // Each call returns an Effect of whatever the test's mock resolves to.
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: openUrl }));

import { VersionsDialog } from '../VersionsDialog';

const info = (o: Partial<VersionInfo> = {}): VersionInfo => ({
  channels: [
    { id: 'latest', name: 'Latest', description: 'The current release.', requires: 'none', unlocked: false, buildId: 'b300' },
    { id: 'beta', name: 'Beta', description: 'Needs a password.', requires: 'password', unlocked: false, buildId: 'b310' },
  ],
  builds: [
    { id: 'b300', name: '3.0.0', source: 'store', available: true, channel: 'latest', current: true },
    { id: 'b200', name: '2.0.0', source: 'store', available: true },
    { id: 'b150', name: 'build b150', source: 'archive', available: true, publishedAt: '2026-02-03' },
    { id: 'b090', name: '0.9', source: 'store', available: false },
  ],
  manual: { label: 'Build id', hint: 'b and digits, like b200.' },
  links: [{ label: 'Depots on the web', url: 'https://example.com/depots' }],
  updateAvailable: false, pinned: null,
  ledger: [{ store: 'echo', game: 'p.echo.tiny', build: 'b300', versionName: '3.0.0', channel: 'latest', date: '2026-10-01T10:00:00Z' }],
  ...o,
});

const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', is_installed: true, installable: true, version: '3.0.0',
  ...o,
}) as Game;
const plugin = { id: 'echo', name: 'Echo', capabilities: ['versions'], declared: ['versions'] } as StorePlugin;

function open(g: Game = game(), v: VersionInfo = info()) {
  api.getGameVersions.mockResolvedValue(v);
  const onClose = vi.fn(), onChanged = vi.fn(), onNotify = vi.fn();
  render(<VersionsDialog game={g} plugin={plugin} onClose={onClose} onChanged={onChanged} onNotify={onNotify} />);
  return { onClose, onChanged, onNotify };
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  openUrl.mockReset();
  api.installGameVersion.mockResolvedValue({ id: 'j' });
  api.updateGameVersion.mockResolvedValue({ id: 'j' });
  api.pinGameVersion.mockResolvedValue(undefined);
  api.unpinGameVersion.mockResolvedValue(undefined);
  api.updateGame.mockResolvedValue({ id: 'j' });
});

describe('Version dialog', () => {
  it('shows the current version, channels and builds with where each build came from', async () => {
    open();
    expect(await screen.findByText('Installed')).toBeTruthy();
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getAllByText('3.0.0').length).toBeGreaterThan(0);
    expect(within(dialog).getByText(/Last install/)).toBeTruthy();
    const channels = screen.getByRole('radiogroup', { name: 'Channels' });
    expect(within(channels).getByRole('radio', { name: 'Latest' })).toBeTruthy();
    const builds = screen.getByRole('radiogroup', { name: 'Builds' });
    expect(within(builds).getAllByText('from the store')).toHaveLength(3);
    expect(within(builds).getByText('from your install history')).toBeTruthy();
    // Unavailable builds are listed but can't be picked.
    expect((within(builds).getByRole('radio', { name: '0.9' }) as HTMLButtonElement).disabled).toBe(true);
    expect(within(builds).getByText(/can’t be fetched right now/)).toBeTruthy();
    expect(screen.getByText('b and digits, like b200.')).toBeTruthy();
  });

  it('opens a link in the browser and nowhere else', async () => {
    open();
    fireEvent.click(await screen.findByRole('button', { name: /Depots on the web/ }));
    expect(openUrl).toHaveBeenCalledWith('https://example.com/depots');
  });

  it('warns before switching to another build, then updates the installed game to the target', async () => {
    const { onChanged, onClose } = open();
    fireEvent.click(await screen.findByRole('radio', { name: '2.0.0' }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));

    const alert = await screen.findByRole('alert');
    expect(alert.textContent).toMatch(/Cloud saves and online play may not match/);
    expect(api.updateGameVersion).not.toHaveBeenCalled(); // nothing is queued before the user confirms

    fireEvent.click(screen.getByRole('button', { name: 'Switch anyway' }));
    await waitFor(() => expect(api.updateGameVersion).toHaveBeenCalledWith(expect.objectContaining({ id: 'p.echo.tiny' }), { build: 'b200' }));
    expect(api.installGameVersion).not.toHaveBeenCalled();
    expect(api.pinGameVersion).not.toHaveBeenCalled();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onChanged).toHaveBeenCalled();
  });

  it('can go back from the warning without queuing anything', async () => {
    open();
    fireEvent.click(await screen.findByRole('radio', { name: '2.0.0' }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Back' }));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(api.updateGameVersion).not.toHaveBeenCalled();
  });

  it('installs a game that is not installed yet at the chosen target, and pins it on request', async () => {
    open(game({ is_installed: false }));
    fireEvent.click(await screen.findByRole('radio', { name: 'Latest' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Keep this version/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Install this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Install anyway' }));
    await waitFor(() => expect(api.installGameVersion).toHaveBeenCalledWith(expect.anything(), { channel: 'latest' }));
    await waitFor(() => expect(api.pinGameVersion).toHaveBeenCalledWith(expect.anything(), { kind: 'channel', id: 'latest' }));
    expect(api.updateGameVersion).not.toHaveBeenCalled();
  });

  it('sends typed text as the manual target and cannot pin it', async () => {
    open();
    fireEvent.change(await screen.findByLabelText('Build id'), { target: { value: ' b777 ' } });
    const pin = screen.getByRole('checkbox', { name: /Keep this version/ }) as HTMLInputElement;
    expect(pin.disabled).toBe(true);
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Switch anyway' }));
    await waitFor(() => expect(api.updateGameVersion).toHaveBeenCalledWith(expect.anything(), { manual: { input: 'b777' } }));
    expect(api.pinGameVersion).not.toHaveBeenCalled();
  });

  it('keeps the install button off until something is chosen, and for the installed version', async () => {
    open();
    const go = await screen.findByRole('button', { name: 'Switch to this version' }) as HTMLButtonElement;
    expect(go.disabled).toBe(true);
    fireEvent.click(screen.getByRole('radio', { name: '3.0.0' })); // already installed
    const same = screen.getByRole('button', { name: 'This is the installed version' }) as HTMLButtonElement;
    expect(same.disabled).toBe(true);
  });

  it('pins the installed version without reinstalling it', async () => {
    const { onChanged } = open();
    fireEvent.click(await screen.findByRole('radio', { name: '3.0.0' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Keep this version/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Pin this version' }));
    await waitFor(() => expect(api.pinGameVersion).toHaveBeenCalledWith(expect.anything(), { kind: 'build', id: 'b300' }));
    expect(api.updateGameVersion).not.toHaveBeenCalled();
    expect(onChanged).toHaveBeenCalled();
  });

  it('shows no unavailable-version state for a game with a single build', async () => {
    open(game(), info({ channels: [], builds: [], manual: null, links: [] }));
    expect(await screen.findByText(/lists only one version/)).toBeTruthy();
  });

  it('says why listing failed', async () => {
    api.getGameVersions.mockRejectedValue(new Error('Echo is down'));
    render(<VersionsDialog game={game()} plugin={plugin} onClose={() => {}} onChanged={() => {}} onNotify={() => {}} />);
    expect((await screen.findByRole('alert')).textContent).toMatch(/Echo is down/);
  });
});

describe('Channel passwords', () => {
  it('locks a password channel until the right password is sent, and forgets it right away', async () => {
    api.unlockVersionChannel.mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    const { onNotify } = open();
    const beta = await screen.findByRole('radio', { name: 'Beta' }) as HTMLButtonElement;
    expect(beta.disabled).toBe(true);

    const box = screen.getByLabelText('Password for Beta') as HTMLInputElement;
    expect(box.type).toBe('password');
    fireEvent.change(box, { target: { value: 'wrong' } });
    fireEvent.click(screen.getByRole('button', { name: 'Unlock' }));
    expect(await screen.findByText(/wasn’t accepted/)).toBeTruthy();
    expect(api.unlockVersionChannel).toHaveBeenCalledWith(expect.anything(), 'beta', 'wrong');
    expect((screen.getByLabelText('Password for Beta') as HTMLInputElement).value).toBe(''); // not kept
    expect(document.body.textContent).not.toContain('wrong');

    api.getGameVersions.mockResolvedValue(info({
      channels: [
        { id: 'latest', name: 'Latest', requires: 'none' },
        { id: 'beta', name: 'Beta', requires: 'password', unlocked: true },
      ],
    }));
    fireEvent.change(screen.getByLabelText('Password for Beta'), { target: { value: 'hydra-beta' } });
    fireEvent.click(screen.getByRole('button', { name: 'Unlock' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Beta unlocked'));
    await waitFor(() => expect((screen.getByRole('radio', { name: 'Beta' }) as HTMLButtonElement).disabled).toBe(false));
    expect(screen.queryByLabelText('Password for Beta')).toBeNull();
    expect(document.body.textContent).not.toContain('hydra-beta');
  });

  it('shows the plugin’s own message when unlocking fails', async () => {
    api.unlockVersionChannel.mockRejectedValue(new Error('This store has no channel passwords'));
    open();
    fireEvent.change(await screen.findByLabelText('Password for Beta'), { target: { value: 'x' } });
    fireEvent.click(screen.getByRole('button', { name: 'Unlock' }));
    expect(await screen.findByText(/no channel passwords/)).toBeTruthy();
  });
});

describe('Pinned game', () => {
  const pinned = () => info({ pinned: { kind: 'build', id: 'b200' }, updateAvailable: true });

  it('says what it is pinned to and returns to latest', async () => {
    const { onChanged, onNotify } = open(game(), pinned());
    expect((await screen.findByText(/Pinned to build 2\.0\.0/)).textContent).toBeTruthy();
    expect(screen.getByText(/update prompt stays off/)).toBeTruthy();
    api.getGameVersions.mockResolvedValue(info());
    fireEvent.click(screen.getByRole('button', { name: 'Return to latest' }));
    await waitFor(() => expect(api.unpinGameVersion).toHaveBeenCalledWith('p.echo.tiny'));
    expect(onChanged).toHaveBeenCalled();
    expect(onNotify).toHaveBeenCalledWith(expect.stringMatching(/latest/));
    await waitFor(() => expect(screen.queryByText(/Pinned to/)).toBeNull());
  });

  it('offers to update to the pin when the installed build differs', async () => {
    const { onClose } = open(game(), pinned());
    fireEvent.click(await screen.findByRole('button', { name: 'Update to pinned version' }));
    await waitFor(() => expect(api.updateGame).toHaveBeenCalledWith(expect.objectContaining({ id: 'p.echo.tiny' })));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it('does not offer an update when the pin is what is installed', async () => {
    open(game(), info({ pinned: { kind: 'build', id: 'b300' }, updateAvailable: false }));
    await screen.findByText(/Pinned to/);
    expect(screen.queryByRole('button', { name: 'Update to pinned version' })).toBeNull();
  });

  it('queues nothing when the pin cannot be set', async () => {
    api.pinGameVersion.mockRejectedValueOnce(new Error('the store refused the pin'));
    const { onNotify, onChanged } = open();
    fireEvent.click(await screen.findByRole('radio', { name: '2.0.0' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Keep this version/ }));
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Switch anyway' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('the store refused the pin', 'err'));
    expect(api.updateGameVersion).not.toHaveBeenCalled();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it('puts the previous pin back when the job cannot be queued', async () => {
    api.updateGameVersion.mockRejectedValueOnce(new Error('the store is busy'));
    const { onNotify } = open(game(), info({ pinned: { kind: 'build', id: 'b200' }, updateAvailable: true }));
    fireEvent.click(await screen.findByRole('radio', { name: '2.0.0' }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Keep this version/ })); // untick: the pin goes
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Switch anyway' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('the store is busy', 'err'));
    expect(api.unpinGameVersion).toHaveBeenCalledWith('p.echo.tiny');
    expect(api.pinGameVersion).toHaveBeenCalledWith(expect.anything(), { kind: 'build', id: 'b200' });
  });

  it('starts with the pin ticked and removes it when unticked', async () => {
    open(game(), pinned());
    fireEvent.click(await screen.findByRole('radio', { name: '2.0.0' }));
    const box = screen.getByRole('checkbox', { name: /Keep this version/ }) as HTMLInputElement;
    expect(box.checked).toBe(true);
    fireEvent.click(box);
    fireEvent.click(screen.getByRole('button', { name: 'Switch to this version' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Switch anyway' }));
    await waitFor(() => expect(api.unpinGameVersion).toHaveBeenCalledWith('p.echo.tiny'));
    expect(api.pinGameVersion).not.toHaveBeenCalled();
  });
});
