import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { cleanup, render, screen, fireEvent, act, waitFor } from '@testing-library/react';
import { renderHook } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';
import React from 'react';
import type { Game, StorePlugin, Tweak } from '../types';
import { atlasFieldCategory, atlasGroups, atlasSummary, atlasUnmappedFields } from './AtlasSettings';
import { compatTabCount } from './GameTabs';

expect.extend(matchers);
afterEach(() => { cleanup(); });

const t = ((key: string, opts?: Record<string, unknown>) => {
  if (key === 'gameSettings:summary.auto') return 'Automatic runner';
  if (key === 'gameSettings:summary.native') return 'Native Linux';
  if (key === 'atlas:summary.default') return 'Default';
  if (key === 'atlas:summary.bundled') return 'Bundled';
  if (key === 'atlas:summary.none') return 'None';
  if (key === 'atlas:summary.noneApplied') return 'None applied';
  if (key === 'atlas:summary.notUsed') return 'Not used';
  if (key === 'atlas:summary.notNeeded') return 'Not needed';
  if (key === 'atlas:summary.noArgs') return 'No arguments';
  if (key.startsWith('atlas:summary.applied')) return `${opts?.count} applied`;
  if (key.startsWith('atlas:summary.setCount')) return `${opts?.count} set`;
  if (key === 'gameSettings:summary.cloudStore') return 'Synced by the store';
  if (key === 'gameSettings:summary.cloudPlatypus') return 'Synced by Platypus';
  if (key === 'gameSettings:summary.pinned') return `Pinned to ${(opts as { version: string }).version}`;
  if (key === 'gameSettings:summary.update') return 'Update available';
  if (key === 'gameSettings:summary.latest') return 'Latest version';
  if (key === 'gameSettings:summary.options') return 'Add-ons and install options';
  if (key === 'gameSettings:summary.plugin') return 'Store plugin';
  if (key === 'gameSettings:summary.customization') return 'Art, title and sort';
  if (key === 'gameSettings:summary.maintenance') return 'Verify, move, backups';
  return key;
}) as unknown as (key: string, opts?: Record<string, unknown>) => string;

const testGame = (): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny',
  cover_url: '', banner_url: '', install_dir: '/games/echo/tiny', executable: '', is_installed: true,
  installable: true, features: null,
  compat_badges: [{ kind: 'rating', label: 'Gold', tone: 'gold' }] as unknown as Game['compat_badges'],
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' },
}) as Game;

describe('atlas drawer mapping', () => {
  it('maps every known runner field into exactly one drawer category, unknowns into Fixes', () => {
    const known = [
      ['runner_id', 'compat'],
      ['use_gamemode', 'performance'],
      ['use_mangohud', 'performance'],
      ['mangohud_preset', 'performance'],
      ['use_gamescope', 'display'],
      ['gamescope_resolution', 'display'],
      ['gamescope_rate', 'display'],
      ['gamescope_filter', 'display'],
      ['gamescope_fullscreen', 'display'],
      ['gamescope_borderless', 'display'],
      ['gamescope_grab_cursor', 'display'],
      ['gamescope_hdr', 'display'],
      ['gamescope_vrr', 'display'],
      ['gamescope_wayland', 'display'],
      ['gamescope_w', 'display'],
      ['gamescope_h', 'display'],
      ['gamescope_r', 'display'],
      ['upscaler', 'display'],
      ['dx_layer', 'layers'],
      ['sync_mode', 'layers'],
      ['shader_cache', 'layers'],
      ['proton_enable_nvapi', 'layers'],
      ['proton_hide_nvidia_gpu', 'layers'],
      ['proton_enable_wayland', 'layers'],
      ['proton_no_d3d11', 'layers'],
      ['proton_no_d3d12', 'layers'],
      ['proton_force_large_address_aware', 'layers'],
      ['proton_old_gl_string', 'layers'],
      ['proton_log', 'env'],
      ['wine_prefix', 'env'],
      ['env_vars', 'env'],
      ['launch_arguments', 'launch'],
      ['working_dir', 'launch'],
      ['gamescope_args', 'launch'],
      ['eac_runtime', 'fixes'],
      ['battleye_runtime', 'fixes'],
      ['winetricks', 'fixes'],
      ['some_future_runner_flag', 'fixes'],
    ] as const;
    for (const [field, cat] of known) expect(atlasFieldCategory(field)).toBe(cat);
  });

  it('groups Everyday, Advanced and Game in drawer order, Game only with its tabs', () => {
    const full = atlasGroups(['general', 'cloud', 'versions', 'options', 'plugin', 'tweaks', 'customization', 'maintenance'] as never);
    expect(full.map((g) => g.level)).toEqual(['everyday', 'advanced', 'game']);
    expect(full[0].cats).toEqual(['compat', 'performance', 'display', 'saves']);
    expect(full[1].cats).toEqual(['layers', 'fixes', 'env', 'launch']);
    expect(full[2].cats).toEqual(['versions', 'options', 'plugin', 'customization', 'maintenance']);
    const noGame = atlasGroups(['general'] as never);
    expect(noGame.map((g) => g.level)).toEqual(['everyday', 'advanced']);
  });

  it('summarises each category from real values (kit.jsx catSummary)', () => {
    const base = { game: testGame(), runners: [], tweaks: [] as Tweak[] };
    expect(atlasSummary(t, 'compat', base)).toBe('Automatic runner');
    expect(atlasSummary(t, 'performance', base)).toBe('Default');
    expect(atlasSummary(t, 'display', base)).toBe('Default');
    expect(atlasSummary(t, 'saves', base)).toBe('Synced by Platypus');
    expect(atlasSummary(t, 'layers', base)).toBe('Bundled');
    expect(atlasSummary(t, 'fixes', base)).toBe('None applied');
    expect(atlasSummary(t, 'env', base)).toBe('None');
    expect(atlasSummary(t, 'launch', base)).toBe('No arguments');
    const gamemode = { ...base, game: { ...testGame(), settings: { ...testGame().settings, use_gamemode: true } } as Game };
    expect(atlasSummary(t, 'performance', gamemode)).toContain('GameMode');
    const native = { ...base, game: { ...testGame(), native: true } as Game };
    expect(atlasSummary(t, 'compat', native)).toBe('Native Linux');
    expect(atlasSummary(t, 'layers', native)).toBe('Not used');
    expect(atlasSummary(t, 'fixes', native)).toBe('Not needed');
    const withArgs = { ...base, game: { ...testGame(), settings: { ...testGame().settings, launch_arguments: '-novid' } } as Game };
    expect(atlasSummary(t, 'launch', withArgs)).toBe('-novid');
  });

  it('B5: atlasUnmappedFields finds future runner flags, not known ones', () => {
    const known = [
      { id: 'g', fields: [{ id: 'runner_id' }, { id: 'use_gamemode' }, { id: 'dx_layer' }, { id: 'winetricks' }] },
    ] as never;
    expect(atlasUnmappedFields(known)).toEqual([]);
    const mixed = [{ id: 'g', fields: [{ id: 'runner_id' }, { id: 'some_future_runner_flag' }] }] as never;
    expect(atlasUnmappedFields(mixed)).toEqual(['some_future_runner_flag']);
  });

  it('B4: the Compatibility tab count includes the fix-info row', () => {
    const withFix = { fix_info: { description: 'Use the curated recipe', source: 'Platypus' } } as Game;
    expect(compatTabCount(withFix, 0, 0)).toBe(1);
    expect(compatTabCount(withFix, 2, 3)).toBe(6);
    const plain = {} as Game;
    expect(compatTabCount(plain, 2, 1)).toBe(3);
  });
});

// Save/Discard footer and drawer chrome, behind the accordion variant.
const state = vi.hoisted(() => ({
  variants: {
    'game-settings.presentation': 'sheet',
    'game-settings.tabs': 'accordion',
    'section.chrome': 'hairline',
    'dialog.presentation': 'dialog',
    'dialog.size': 'auto',
  } as Record<string, string>,
}));
const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getCloudInfo: vi.fn(),
  checkCloudSaves: vi.fn(),
  getGameSettingsFields: vi.fn(),
  getGameVersions: vi.fn(),
  getCustomization: vi.fn(),
  previewCommand: vi.fn(),
  getLauncherSettings: vi.fn(),
  getRunnerOptions: vi.fn(),
  getRunnerEnv: vi.fn(),
  listSaveBackups: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
}));
const draftCtl = vi.hoisted(() => ({
  dirty: false,
  discard: vi.fn(),
  markClean: vi.fn(),
  onSave: vi.fn(),
}));

vi.mock('../lib/theme', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/theme')>()),
  useVariant: (axis: string) => state.variants[axis],
}));
vi.mock('../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (tgt, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (tgt as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));
vi.mock('./SettingsModal', () => ({
  GeneralSettings: () => null,
  useGameSettingsDraft: (game: Game) => ({
    draft: game.settings,
    setDraft: () => {},
    envRows: [],
    setEnvRows: () => {},
    nextRowId: { current: 0 },
    current: game.settings,
    dirty: draftCtl.dirty,
    markClean: draftCtl.markClean,
    applySaved: () => {},
    discard: draftCtl.discard,
  }),
  CustomProgram: () => null,
  PrefixPresets: () => null,
}));
vi.mock('./CustomGameYear', () => ({ CustomGameYear: () => null }));
vi.mock('./LegalSection', () => ({ LegalSection: () => null }));
vi.mock('./maintenancePrefetch', () => ({
  MaintenancePrefetchProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMaintenancePrefetchSource: () => ({}),
  useMaintenancePrefetch: () => null,
}));

import { GameSettingsModal } from './GameSettingsModal';

describe('atlas drawer save model', () => {
  beforeEach(() => {
    Object.values(api).forEach((f) => f.mockReset());
    api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
    api.getInstallSize.mockResolvedValue(0);
    api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: null });
    api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
    api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
    api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
    api.getCustomization.mockResolvedValue({ title: '', sort_title: '', art: {} });
    api.previewCommand.mockResolvedValue('wine game.exe');
    api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/prefixes' });
    api.getRunnerOptions.mockResolvedValue([]);
    api.getRunnerEnv.mockResolvedValue({ env: {}, wrappers: [] });
    api.listSaveBackups.mockResolvedValue([]);
    api.getSavePaths.mockResolvedValue([]);
    api.getGameSaveBackup.mockResolvedValue('');
    api.getAutoBackupState.mockResolvedValue(null);
    draftCtl.dirty = false;
    draftCtl.discard.mockReset();
    draftCtl.markClean.mockReset();
    draftCtl.onSave.mockReset();
    draftCtl.onSave.mockResolvedValue(undefined);
  });

  const props = () => ({
    game: testGame(),
    plugin: { id: 'echo', name: 'Echo', status: 'ready', capabilities: [], declared: [], install_folder: '/games' } as unknown as StorePlugin,
    runners: [], tweaks: [] as Tweak[], jobs: [],
    onClose: () => {}, onSaveSettings: draftCtl.onSave, onChangeSettings: () => {}, onJob: () => {}, onJobAction: () => {},
    onNotify: () => {}, onChanged: () => {}, onCustomized: () => {}, onCopied: () => {},
  });

  it('shows a slim sticky footer only when dirty, with Discard and Save', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<GameSettingsModal {...props()} />);
    expect(screen.queryByText('Unsaved changes')).toBeNull();
    draftCtl.dirty = true;
    rerender(<GameSettingsModal {...props()} />);
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
    const bar = screen.getByText('Unsaved changes').parentElement!;
    expect(bar.className).toMatch(/sticky|border-t/);
    await user.click(screen.getByRole('button', { name: 'Discard' }));
    expect(draftCtl.discard).toHaveBeenCalledOnce();
    const saves = screen.getAllByRole('button', { name: 'Save' });
    // The footer Save is the primary one in the sticky bar; SteamAppId (when
    // present) uses a neutral Save.
    const footerSave = saves.find((b) => b.className.includes('bg-accent')) ?? saves[saves.length - 1];
    await user.click(footerSave);
    expect(draftCtl.onSave).toHaveBeenCalledOnce();
    expect(draftCtl.markClean).toHaveBeenCalledOnce();
  });

  it('uses /21 row markup: 600 labels, 0.93em muted descriptions, 190px sans controls', async () => {
    draftCtl.dirty = false;
    api.getRunnerOptions.mockResolvedValue([
      { id: 'runner', label: 'Runner', fields: [{ id: 'runner_id', kind: 'select', label: 'Runner', description: 'The Proton build.', scope: 'game', default: 'auto', options: [{ id: 'auto', label: 'Automatic' }] }] },
      { id: 'wrappers', label: 'Wrappers', fields: [{ id: 'use_gamemode', kind: 'bool', label: 'GameMode', description: 'Full speed.', scope: 'game', default: true }] },
    ]);
    render(<GameSettingsModal {...props()} />);
    // Compatibility is open by default: the runner row is a grid with the control right.
    const label = await screen.findByText('Compatibility layer');
    const row = label.closest('div')!;
    expect(row.parentElement!.className).toMatch(/grid.*1fr.*auto/);
    expect(screen.getByText('Compatibility layer').className).toMatch(/font-semibold/);
    // The select control is 190px and sans (not the ds mono).
    const trigger = screen.getByRole('combobox', { name: 'Runner' });
    expect(trigger.className).toMatch(/font-sans/);
    expect(trigger).toHaveStyle({ width: '190px' });
  });

  it('keeps the unsaved-changes close guard (Escape inert when dirty)', async () => {
    draftCtl.dirty = true;
    const closed = vi.fn();
    render(<GameSettingsModal {...props()} onClose={closed} />);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(closed).not.toHaveBeenCalled();
  });

  it('returns focus to the Tune button when the drawer closes after its opener is gone (issue #130)', async () => {
    const user = userEvent.setup();
    const tune = document.createElement('button');
    tune.setAttribute('data-tune-anchor', '');
    tune.textContent = 'Tune';
    document.body.appendChild(tune);
    const opener = document.createElement('button');
    opener.textContent = 'All settings';
    document.body.appendChild(opener);
    opener.focus();
    const Harness = () => {
      const [open, setOpen] = React.useState(true);
      return open ? <GameSettingsModal {...props()} onClose={() => setOpen(false)} /> : null;
    };
    render(<Harness />);
    // The flyout (and its All settings button) closes under the drawer.
    opener.remove();
    await user.click(screen.getByRole('button', { name: 'Close' }));
    await waitFor(() => expect(document.activeElement).toBe(tune));
    tune.remove();
  });

  it('B3: extra switch fields fall back to their default when the draft is missing', async () => {
    draftCtl.dirty = false;
    api.getRunnerOptions.mockResolvedValue([
      { id: 'runner', label: 'Runner', fields: [{ id: 'runner_id', kind: 'select', label: 'Runner', description: 'The build.', scope: 'game', default: 'auto', options: [{ id: 'auto', label: 'Automatic' }] }] },
      { id: 'future', label: 'Future', fields: [{ id: 'some_future_flag', kind: 'bool', label: 'Future flag', description: 'A new runner switch.', scope: 'game', default: true }] },
    ]);
    render(<GameSettingsModal {...props()} />);
    // Fixes holds unknown flags: open it and the switch reads the default (on).
    await userEvent.setup().click(await screen.findByRole('button', { name: /Fixes/ }));
    const sw = await screen.findByRole('switch', { name: /Future flag/ });
    expect(sw.getAttribute('aria-checked')).toBe('true');
  });

  it('B2: footer Save clears only once the save resolves (a failed save stays dirty)', async () => {
    const user = userEvent.setup();
    draftCtl.dirty = true;
    draftCtl.markClean.mockClear();
    draftCtl.onSave.mockReset();
    draftCtl.onSave.mockImplementationOnce(() => Promise.reject(new Error('no disk')));
    render(<GameSettingsModal {...props()} />);
    expect(await screen.findByText('Unsaved changes')).toBeInTheDocument();
    const saves = screen.getAllByRole('button', { name: 'Save' });
    const footerSave = saves.find((b) => b.className.includes('bg-accent')) ?? saves[saves.length - 1];
    await user.click(footerSave);
    await new Promise((r) => setTimeout(r, 20));
    expect(draftCtl.onSave).toHaveBeenCalledOnce();
    expect(draftCtl.markClean).not.toHaveBeenCalled();
  });

  it('B1: the slim Saves row skips its auto-check on an uninstalled game', async () => {
    const { CloudSlimRow } = await import('./GameTabs');
    api.getCloudInfo.mockReset();
    api.checkCloudSaves.mockReset();
    // No status yet, cloud on: an installed game checks once.
    api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: null });
    api.checkCloudSaves.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: null });
    const installed = { ...testGame(), is_installed: true };
    const { unmount } = render(<CloudSlimRow game={installed} plain />);
    await new Promise((r) => setTimeout(r, 30));
    expect(api.checkCloudSaves).toHaveBeenCalledTimes(1);
    unmount();
    cleanup();
    api.getCloudInfo.mockClear();
    api.checkCloudSaves.mockClear();
    api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: null });
    const uninstalled = { ...testGame(), id: 'p.echo.uninstalled', is_installed: false };
    render(<CloudSlimRow game={uninstalled} plain />);
    await new Promise((r) => setTimeout(r, 30));
    expect(api.checkCloudSaves).not.toHaveBeenCalled();
  });

  it('B2: markClean remembers the saved env (Discard restores it, not stale vars)', async () => {
    const actual = await vi.importActual<typeof import('./SettingsModal')>('./SettingsModal');
    const game = { ...testGame(), settings: { ...testGame().settings, env_vars: { OLD: '1' } } } as Game;
    const { result } = renderHook(() => actual.useGameSettingsDraft(game));
    // Edit the env rows to a new valid value, then mark the save.
    act(() => {
      result.current.setEnvRows([{ id: 0, key: 'NEW', value: '2' }]);
    });
    expect(result.current.current.env_vars).toEqual({ NEW: '2' });
    act(() => {
      result.current.markClean();
    });
    expect(result.current.dirty).toBe(false);
    // More edits, then Discard returns to the marked save (NEW), not OLD.
    act(() => {
      result.current.setEnvRows([{ id: 1, key: 'OTHER', value: '3' }]);
    });
    expect(result.current.dirty).toBe(true);
    act(() => {
      result.current.discard();
    });
    expect(result.current.current.env_vars).toEqual({ NEW: '2' });
    expect(result.current.dirty).toBe(false);
  });
});

describe('atlas maintenance parity (issue #132)', () => {
  it('carries verify, move and uninstall in the Maintenance category', async () => {
    const { AtlasCategoryBody } = await import('./AtlasSettings');
    const g = testGame();
    const shared = {
      draft: g.settings,
      setDraft: () => {},
      envRows: [],
      setEnvRows: () => {},
      nextRowId: { current: 0 },
      current: g.settings,
      dirty: false,
      markClean: () => {},
      applySaved: () => {},
      discard: () => {},
    };
    render(
      <AtlasCategoryBody
        cat="maintenance"
        game={g}
        plugin={{ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['verify', 'install'], declared: [], install_folder: '/games' } as unknown as StorePlugin}
        runners={[]}
        tweaks={[]}
        jobs={[]}
        groups={[]}
        shared={shared}
        onSaveSettings={async () => {}}
        onChangeSettings={() => {}}
        onJob={() => {}}
        onJobAction={() => {}}
        onNotify={() => {}}
        onChanged={() => {}}
        onCustomized={() => {}}
        onCopied={() => {}}
        onClose={() => {}}
        onOpenGeneral={() => {}}
      />,
    );
    expect(await screen.findByRole('button', { name: /Verify files/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Move/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Uninstall' })).toBeInTheDocument();
  });
});
