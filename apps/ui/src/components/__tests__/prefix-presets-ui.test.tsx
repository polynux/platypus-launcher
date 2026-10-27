import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, GameSettings, PrefixPreset } from '../../types';

// Wine prefix presets in the settings (issue #52): the grouped picker, the
// preview before applying, applying with a settings refresh, and saving and
// deleting the user's own presets.

const api = vi.hoisted(() => ({
  getLauncherSettings: vi.fn(),
  prefixUsage: vi.fn(),
  previewCommand: vi.fn(),
  detectGameExecutable: vi.fn(),
  isGameExecutable: vi.fn(),
  pickFolder: vi.fn(),
  pickGameExecutable: vi.fn(),
  updateCustomGameExecutable: vi.fn(),
  makeGameExecutable: vi.fn(),
  listPrefixPresets: vi.fn(),
  savePrefixPreset: vi.fn(),
  deletePrefixPreset: vi.fn(),
  applyPrefixPreset: vi.fn(),
  getGameSettings: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
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
  onLibraryPartial: () => () => {},
}));

import { SettingsModal } from '../SettingsModal';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom',
  title: 'Sideloaded Gem', cover_url: '', banner_url: '',
  install_dir: '/games/sideload', executable: '/games/sideload/gem.exe',
  is_installed: true, features: null,
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

/** Fixture settings: only the keys the test cares about. */
const settings = (o: Record<string, unknown>): GameSettings => o as unknown as GameSettings;

const presets: PrefixPreset[] = [
  { id: 'builtin:old-dx9', name: 'Old DirectX 9 game', builtin: true, dll_overrides: { xaudio2_7: 'n,b', ddraw: 'n,b' }, winetricks: ['d3dx9', 'corefonts'] },
  { id: 'builtin:vc-fonts', name: 'Visual C++ + fonts baseline', builtin: true, winetricks: ['vcrun2019', 'corefonts'] },
  { id: 'user-1', name: 'My preset', builtin: false, env: { FOO: 'preset', BAR: 'preset' }, dll_overrides: { ddraw: 'n,b' }, winetricks: ['corefonts'], windows_version: 'win8' },
];

beforeEach(() => {
  vi.clearAllMocks();
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/pfx' });
  api.prefixUsage.mockResolvedValue([]);
  api.previewCommand.mockResolvedValue('wine game.exe');
  api.detectGameExecutable.mockResolvedValue({ native: false, kind: 'windows-exe' });
  api.isGameExecutable.mockResolvedValue(true);
  api.pickFolder.mockResolvedValue('');
  api.pickGameExecutable.mockResolvedValue('');
  api.updateCustomGameExecutable.mockResolvedValue(game());
  api.makeGameExecutable.mockResolvedValue(game());
  api.listPrefixPresets.mockResolvedValue(presets);
  api.applyPrefixPreset.mockResolvedValue({ env: {}, overrides: '', skipped: [], verbs: [], windows_version: 'win10' });
  api.savePrefixPreset.mockImplementation(async (p: PrefixPreset) => ({ ...p, id: 'user-2', builtin: false }));
  api.deletePrefixPreset.mockResolvedValue(undefined);
  api.getGameSettings.mockResolvedValue({});
});

const open = (g: Game) => render(<SettingsModal game={g} runners={[]} onClose={() => {}} onSave={async () => {}} onCopied={() => {}} />);
const openPresets = () =>
  fireEvent.pointerDown(screen.getByRole('combobox', { name: 'Apply a preset…' }), { button: 0, ctrlKey: false, pointerType: 'mouse' });

describe('The preset picker', () => {
  it('lists the built-ins and the user presets in their own groups', async () => {
    open(game());
    openPresets();
    const list = await screen.findByRole('listbox');
    expect(within(list).getByText('built-in')).toBeTruthy();
    expect(within(list).getByText('my presets')).toBeTruthy();
    expect(within(list).getByRole('option', { name: 'Old DirectX 9 game' })).toBeTruthy();
    expect(within(list).getByRole('option', { name: 'Visual C++ + fonts baseline' })).toBeTruthy();
    expect(within(list).getByRole('option', { name: 'My preset' })).toBeTruthy();
  });

  it('previews the additions, the skipped keys, the overrides and what is stored', async () => {
    open(game({ settings: settings({ runner_id: 'auto', env_vars: { BAR: 'user', WINEDLLOVERRIDES: 'dxgi=n,b' } }) }));
    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'My preset' }));

    expect(await screen.findByText('FOO=preset')).toBeTruthy(); // added
    expect(screen.getByText('keeps yours: BAR')).toBeTruthy(); // skipped: the user's value wins
    expect(screen.getByText('WINEDLLOVERRIDES=dxgi=n,b;ddraw=n,b')).toBeTruthy(); // composed, the user's entry kept
    expect(screen.getByText('Stored for later, not run yet')).toBeTruthy();
    expect(screen.getByText('winetricks: corefonts')).toBeTruthy();
    expect(screen.getByText('windows version: win8')).toBeTruthy();
  });
});

describe('Applying a preset', () => {
  it('calls the API and refreshes what the form shows', async () => {
    api.getGameSettings.mockResolvedValue({
      'custom-1': { env_vars: { FOO: 'preset' }, winetricks: ['corefonts'], prefix_preset: 'user-1' },
    });
    open(game());
    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'My preset' }));
    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    await waitFor(() => expect(api.applyPrefixPreset).toHaveBeenCalledWith('custom-1', 'user-1'));
    // The settings shown come back from core: FOO is now one of the rows.
    await waitFor(() => {
      const names = screen.getAllByLabelText('Variable name') as HTMLInputElement[];
      expect(names.map((n) => n.value)).toContain('FOO');
    });
    expect(await screen.findByText('already applied')).toBeTruthy();
  });

  it('says already applied for the preset the game records, without applying', async () => {
    open(game({ settings_extra: { prefix_preset: 'user-1' } }));
    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'My preset' }));
    expect(await screen.findByText('already applied')).toBeTruthy();
    expect(api.applyPrefixPreset).not.toHaveBeenCalled();
  });
});

describe('Saving and deleting presets', () => {
  it('saves the current settings as a user preset', async () => {
    open(game({
      settings: settings({ runner_id: 'auto', env_vars: { BAR: 'user', WINEDLLOVERRIDES: 'dxgi=n,b' } }),
      settings_extra: { winetricks: ['corefonts'], windows_version: 'win7' },
    }));
    fireEvent.change(screen.getByLabelText('Preset name'), { target: { value: 'Mine' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save current as preset' }));

    await waitFor(() => expect(api.savePrefixPreset).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Mine',
      env: { BAR: 'user' },
      dll_overrides: { dxgi: 'n,b' },
      winetricks: ['corefonts'],
      windows_version: 'win7',
    })));
    await waitFor(() => expect(screen.getByLabelText('Preset name')).toHaveValue(''));
  });

  it('deletes a user preset; a built-in offers no delete control', async () => {
    open(game());
    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'Old DirectX 9 game' }));
    expect(screen.queryByRole('button', { name: /Delete preset/ })).toBeNull();

    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'My preset' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete preset My preset' }));

    await waitFor(() => expect(api.deletePrefixPreset).toHaveBeenCalledWith('user-1'));
    await waitFor(() => expect(api.listPrefixPresets).toHaveBeenCalledTimes(2)); // the list reloaded
    expect(screen.queryByRole('button', { name: /Delete preset/ })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Apply preset' })).toBeNull();
  });
});

describe('Games presets never apply to', () => {
  it('hides the whole presets area for a native game', () => {
    open(game({ settings: settings({ env_vars: {}, run_mode: 'native' }) }));
    expect(screen.queryByRole('combobox', { name: 'Apply a preset…' })).toBeNull();
    expect(screen.queryByLabelText('Preset name')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Save current as preset' })).toBeNull();
    expect(api.listPrefixPresets).not.toHaveBeenCalled(); // core refuses them: never even asked
  });
});

describe('Unsaved edits hold Apply back', () => {
  it('disables Apply with a save-first hint until the form is saved', async () => {
    open(game());
    openPresets();
    fireEvent.click(await screen.findByRole('option', { name: 'My preset' }));
    const apply = screen.getByRole('button', { name: 'Apply preset' });
    expect(apply).toBeEnabled();
    expect(screen.queryByText('Save your changes first.')).toBeNull();

    // An unsaved edit: the core would merge into the saved settings, so the
    // refresh after applying would replace what the user is typing.
    fireEvent.change(screen.getByLabelText('wine prefix'), { target: { value: '/mnt/pfx' } });
    expect(apply).toBeDisabled();
    expect(screen.getByText('Save your changes first.')).toBeTruthy();

    // Saving the form clears the way.
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Apply preset' })).toBeEnabled());
    expect(screen.queryByText('Save your changes first.')).toBeNull();
    expect(api.applyPrefixPreset).not.toHaveBeenCalled();
  });
});
