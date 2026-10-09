import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, GameOptions, OptionGroup, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  getGameOptions: vi.fn(),
  getInstallFreeSpace: vi.fn(),
  getInstallSize: vi.fn(),
  installGameWithOptions: vi.fn(),
  updateGameWithOptions: vi.fn(),
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
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: openUrl }));

import { InstallOptionsDialog } from '../InstallOptionsDialog';
import { formatBytes } from '../../lib/games';

const MiB = 1024 * 1024;

const groups: OptionGroup[] = [
  { id: 'language', label: 'Language', kind: 'select', default: 'en', values: [
    { id: 'en', label: 'English', owned: true },
    { id: 'fr', label: 'Français', owned: true },
  ] },
  { id: 'dlc', label: 'Add-ons', kind: 'multi', dlc: true, values: [
    { id: 'season', label: 'Season Pass', sizeBytes: 1 * MiB, owned: true },
    { id: 'deluxe', label: 'Deluxe Content', sizeBytes: 4 * MiB, owned: false, storeUrl: 'https://example.com/store/deluxe' },
    { id: 'soundtrack', label: 'Soundtrack', sizeBytes: 2 * MiB, owned: true, installed: true },
  ] },
  { id: 'subtitles', label: 'Subtitles', kind: 'bool', default: false },
];

const options = (o: Partial<GameOptions> = {}): GameOptions => ({
  groups,
  chosen: { language: 'en', dlc: ['soundtrack'], subtitles: false },
  ...o,
});

const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', is_installed: false, installable: true,
  ...o,
}) as Game;
const plugin = { id: 'echo', name: 'Echo', capabilities: ['install-options'], declared: ['install-options'] } as StorePlugin;

// base is the store's size of the game before any add-on; null means unknown.
function open(mode: 'install' | 'update' = 'install', g: Game = game(), o: GameOptions = options(), free = 100 * MiB, base: number | null = 0) {
  api.getGameOptions.mockResolvedValue(o);
  api.getInstallFreeSpace.mockResolvedValue(free);
  if (base == null) api.getInstallSize.mockRejectedValue(new Error('the store can not size it'));
  else api.getInstallSize.mockResolvedValue(base);
  const onClose = vi.fn(), onChanged = vi.fn(), onNotify = vi.fn();
  render(<InstallOptionsDialog game={g} plugin={plugin} mode={mode} onClose={onClose} onChanged={onChanged} onNotify={onNotify} />);
  return { onClose, onChanged, onNotify };
}

// The dialog is ready once the options have loaded.
const ready = () => screen.findByLabelText('Season Pass');

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  openUrl.mockReset();
  api.installGameWithOptions.mockResolvedValue({ id: 'j' });
  api.updateGameWithOptions.mockResolvedValue({ id: 'j' });
});

describe('InstallOptionsDialog', () => {
  it('shows a dropdown, a checkbox per add-on with its size, and a switch', async () => {
    open();
    await ready();
    expect(screen.getByLabelText('Language').textContent).toMatch(/English/);
    expect((screen.getByLabelText('Season Pass') as HTMLInputElement).checked).toBe(false);
    expect((screen.getByLabelText('Soundtrack') as HTMLInputElement).checked).toBe(true);
    expect(screen.getByText('installed')).toBeTruthy();
    expect(screen.getByRole('switch', { name: 'Subtitles' }).getAttribute('aria-checked')).toBe('false');
  });

  it('greys out DLC the user does not own, with the store link', async () => {
    open();
    const deluxe = await screen.findByLabelText('Deluxe Content') as HTMLInputElement;
    expect(deluxe.disabled).toBe(true);
    expect(deluxe.checked).toBe(false);
    expect(screen.getByText('not owned')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /Store/ }));
    expect(openUrl).toHaveBeenCalledWith('https://example.com/store/deluxe');
  });

  it('keeps the live total of the chosen add-ons and warns when the space is short', async () => {
    // Half a MiB free: nothing is short until the season pass (1 MiB) is picked.
    open('install', game(), options(), MiB / 2);
    await ready();
    expect(screen.getByTestId('added-size').textContent).toBe('—');
    expect(screen.queryByRole('alert')).toBeNull();
    fireEvent.click(screen.getByLabelText('Season Pass'));
    expect(screen.getByTestId('added-size').textContent).toBe('+' + formatBytes(1 * MiB));
    expect(screen.getByTestId('free-space').className).toMatch(/text-red/);
    expect(screen.getByRole('alert').textContent).toMatch(/Not enough space/);
    expect((screen.getByRole('button', { name: 'Install' }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByLabelText('Season Pass'));
    expect(screen.queryByRole('alert')).toBeNull();
    expect((screen.getByRole('button', { name: 'Install' }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('counts an add-on already on disk as nothing to download', async () => {
    // The soundtrack is installed and chosen: it adds no bytes.
    open('install', game(), options(), 100 * MiB, 10 * MiB);
    await ready();
    expect(screen.getByTestId('added-size').textContent).toBe('—');
    expect(screen.getByTestId('total-size').textContent).toBe(formatBytes(10 * MiB));
  });

  it('does not block an install when the base size is unknown', async () => {
    open('install', game(), options(), 0, null);
    await ready();
    expect(screen.getByTestId('total-size').textContent).toBe('unknown');
    fireEvent.click(screen.getByLabelText('Season Pass'));
    expect((screen.getByRole('button', { name: 'Install' }) as HTMLButtonElement).disabled).toBe(false);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('installs with the choices, unowned DLC never among them', async () => {
    const { onClose, onChanged, onNotify } = open();
    await ready();
    fireEvent.click(screen.getByLabelText('Season Pass'));
    fireEvent.click(screen.getByRole('switch', { name: 'Subtitles' }));
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    await waitFor(() => expect(api.installGameWithOptions).toHaveBeenCalledTimes(1));
    expect(api.installGameWithOptions.mock.calls[0][1]).toEqual({
      language: 'en', dlc: ['soundtrack', 'season'], subtitles: true,
    });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(onChanged).toHaveBeenCalled();
    expect(onNotify).toHaveBeenCalledWith('Install of Echo Tiny queued', 'busy');
  });

  it('changes an installed game through the update call', async () => {
    open('update', game({ is_installed: true }));
    await ready();
    expect(screen.getByRole('heading', { name: 'Add-ons & options' })).toBeTruthy();
    fireEvent.click(screen.getByLabelText('Season Pass'));
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    await waitFor(() => expect(api.updateGameWithOptions).toHaveBeenCalledTimes(1));
    expect(api.updateGameWithOptions.mock.calls[0][1]).toEqual({ language: 'en', dlc: ['soundtrack', 'season'], subtitles: false });
    expect(api.installGameWithOptions).not.toHaveBeenCalled();
  });

  it('keeps Apply off until an update changes something', async () => {
    open('update', game({ is_installed: true }));
    await ready();
    const apply = () => screen.getByRole('button', { name: 'Apply' }) as HTMLButtonElement;
    expect(apply().disabled).toBe(true);
    fireEvent.click(screen.getByRole('switch', { name: 'Subtitles' }));
    expect(apply().disabled).toBe(false);
    fireEvent.click(screen.getByRole('switch', { name: 'Subtitles' }));
    expect(apply().disabled).toBe(true);
  });

  it('asks once more before an update removes an add-on already on disk', async () => {
    open('update', game({ is_installed: true }));
    await ready();
    fireEvent.click(screen.getByLabelText('Soundtrack'));
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    expect(screen.getByRole('alert').textContent).toMatch(/removes Soundtrack from Echo Tiny/);
    expect(api.updateGameWithOptions).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
    await waitFor(() => expect(api.updateGameWithOptions).toHaveBeenCalledTimes(1));
    expect(api.updateGameWithOptions.mock.calls[0][1]).toEqual({ language: 'en', dlc: [], subtitles: false });
  });

  it('picks a language from the dropdown', async () => {
    open();
    await ready();
    fireEvent.pointerDown(screen.getByLabelText('Language'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(await screen.findByRole('option', { name: 'Français' }));
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    await waitFor(() => expect(api.installGameWithOptions).toHaveBeenCalled());
    expect(api.installGameWithOptions.mock.calls[0][1]).toMatchObject({ language: 'fr' });
  });

  it('says so when the store has nothing to choose', async () => {
    open('install', game(), options({ groups: [], chosen: {} }));
    expect(await screen.findByText(/nothing to choose/)).toBeTruthy();
  });

  it('keeps the dialog open and reports the error when the install is refused', async () => {
    api.installGameWithOptions.mockRejectedValue(new Error('Deluxe Content is not owned: buy it in the store first'));
    const { onClose, onNotify } = open();
    await ready();
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.stringMatching(/not owned/), 'err'));
    expect(onClose).not.toHaveBeenCalled();
  });
});
