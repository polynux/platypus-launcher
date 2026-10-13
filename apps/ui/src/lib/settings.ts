// i18n: converted
import type { TFunction } from 'i18next';
import type { Game, GameSettings, HudPreset, SyncMode, DxLayer, Tweak, Upscaler } from '../types';

// Proton flags the modal exposes. Each one is a real Proton env var toggled to 1.
// What each flag means lives in the gameSettings catalog (protonFlags.descs),
// keyed by the flag itself.
export const PROTON_FLAGS: { key: string }[] = [
  { key: 'PROTON_ENABLE_NVAPI' },
  { key: 'PROTON_HIDE_NVIDIA_GPU' },
  { key: 'PROTON_ENABLE_WAYLAND' },
  { key: 'PROTON_NO_D3D11' },
  { key: 'PROTON_NO_D3D12' },
  { key: 'PROTON_FORCE_LARGE_ADDRESS_AWARE' },
  { key: 'PROTON_OLD_GL_STRING' },
  { key: 'PROTON_LOG' },
];

/** What a Proton flag means, for its tooltip. */
export function protonFlagDesc(t: TFunction, key: string): string {
  switch (key) {
    case 'PROTON_ENABLE_NVAPI': return t('gameSettings:protonFlags.descs.PROTON_ENABLE_NVAPI');
    case 'PROTON_HIDE_NVIDIA_GPU': return t('gameSettings:protonFlags.descs.PROTON_HIDE_NVIDIA_GPU');
    case 'PROTON_ENABLE_WAYLAND': return t('gameSettings:protonFlags.descs.PROTON_ENABLE_WAYLAND');
    case 'PROTON_NO_D3D11': return t('gameSettings:protonFlags.descs.PROTON_NO_D3D11');
    case 'PROTON_NO_D3D12': return t('gameSettings:protonFlags.descs.PROTON_NO_D3D12');
    case 'PROTON_FORCE_LARGE_ADDRESS_AWARE': return t('gameSettings:protonFlags.descs.PROTON_FORCE_LARGE_ADDRESS_AWARE');
    case 'PROTON_OLD_GL_STRING': return t('gameSettings:protonFlags.descs.PROTON_OLD_GL_STRING');
    default: return t('gameSettings:protonFlags.descs.PROTON_LOG');
  }
}

export const SYNC_MODES: { value: SyncMode; label: string }[] = [
  { value: 'fsync', label: 'fsync' },
  { value: 'esync', label: 'esync' },
  { value: 'ntsync', label: 'ntsync' },
  { value: 'off', label: 'off' },
];

/** What a sync mode means, for its tooltip. */
export function syncModeTitle(t: TFunction, value: SyncMode): string {
  switch (value) {
    case 'fsync': return t('gameSettings:sync.titles.fsync');
    case 'esync': return t('gameSettings:sync.titles.esync');
    case 'ntsync': return t('gameSettings:sync.titles.ntsync');
    default: return t('gameSettings:sync.titles.off');
  }
}

export const DX_LAYERS: { value: DxLayer }[] = [
  { value: 'dxvk' },
  { value: 'wined3d' },
];

/** The DX layer choices, for the segmented control. */
export function dxLayerLabel(t: TFunction, value: DxLayer): string {
  return value === 'dxvk' ? t('gameSettings:graphics.dxLayers.dxvk') : t('gameSettings:graphics.dxLayers.wined3d');
}

/** What a DX layer choice means, for its tooltip. */
export function dxLayerTitle(t: TFunction, value: DxLayer): string {
  return value === 'dxvk' ? t('gameSettings:graphics.dxTitles.dxvk') : t('gameSettings:graphics.dxTitles.wined3d');
}

export const UPSCALERS: { value: Upscaler }[] = [
  { value: 'native' },
  { value: 'fsr1' },
];

/** The upscaler choices, for the segmented control. */
export function upscalerLabel(t: TFunction, value: Upscaler): string {
  return value === 'fsr1' ? t('gameSettings:graphics.upscalers.fsr1') : t('gameSettings:graphics.upscalers.native');
}

/** What an upscaler choice means, for its tooltip. */
export function upscalerTitle(t: TFunction, value: Upscaler): string {
  return value === 'fsr1' ? t('gameSettings:graphics.upscalerTitles.fsr1') : t('gameSettings:graphics.upscalerTitles.native');
}

export const HUD_PRESETS: { value: HudPreset }[] = [
  { value: 'off' },
  { value: 'fps only' },
  { value: 'minimal' },
  { value: 'full' },
];

/** The MangoHud choices, for the segmented control. */
export function hudLabel(t: TFunction, value: HudPreset): string {
  switch (value) {
    case 'fps only': return t('gameSettings:graphics.huds.fps only');
    case 'minimal': return t('gameSettings:graphics.huds.minimal');
    case 'full': return t('gameSettings:graphics.huds.full');
    default: return t('gameSettings:graphics.huds.off');
  }
}

/** What a MangoHud choice sets, for its tooltip. */
export function hudTitle(t: TFunction, value: HudPreset): string {
  switch (value) {
    case 'fps only': return t('gameSettings:graphics.hudTitles.fps only');
    case 'minimal': return t('gameSettings:graphics.hudTitles.minimal');
    case 'full': return t('gameSettings:graphics.hudTitles.full');
    default: return t('gameSettings:graphics.hudTitles.off');
  }
}

const HUD_CONFIG: Record<HudPreset, string | null> = {
  off: null,
  'fps only': 'preset=1',
  minimal: 'preset=2',
  full: 'preset=4',
};

export const WINETRICKS_VERBS = ['vcrun2022', 'vcrun2019', 'd3dx9', 'd3dcompiler_47', 'dotnet48', 'corefonts', 'xact', 'mfc140'];

// Every env var this module owns. They are stripped from user env_vars and
// re-derived from the structured fields, so toggles never drift from env.
const MANAGED_KEYS = new Set<string>([
  ...PROTON_FLAGS.map((f) => f.key),
  'PROTON_NO_ESYNC',
  'PROTON_NO_FSYNC',
  'PROTON_USE_NTSYNC',
  'PROTON_USE_WINED3D',
  'WINE_FULLSCREEN_FSR',
  'MANGOHUD_CONFIG',
  'DXVK_STATE_CACHE',
]);

/** Fill structured fields from legacy settings (env_vars, use_mangohud). */
export function normalizeSettings(s: GameSettings): GameSettings {
  const env = s.env_vars ?? {};
  const on = (k: string) => env[k] === '1';
  const sync: SyncMode =
    s.sync_mode ??
    (on('PROTON_USE_NTSYNC') ? 'ntsync' : on('PROTON_NO_ESYNC') ? 'off' : on('PROTON_NO_FSYNC') ? 'esync' : 'fsync');
  const hud: HudPreset =
    s.mangohud_preset ??
    (!s.use_mangohud
      ? 'off'
      : env.MANGOHUD_CONFIG === 'preset=1'
        ? 'fps only'
        : env.MANGOHUD_CONFIG === 'preset=2'
          ? 'minimal'
          : 'full');
  return {
    ...s,
    env_vars: Object.fromEntries(Object.entries(env).filter(([k]) => !MANAGED_KEYS.has(k))),
    proton_flags: s.proton_flags ?? PROTON_FLAGS.map((f) => f.key).filter(on),
    sync_mode: sync,
    dx_layer: s.dx_layer ?? (on('PROTON_USE_WINED3D') ? 'wined3d' : 'dxvk'),
    // Legacy 'integer' was never applied, so it is plain native.
    upscaler: s.upscaler === 'fsr1' ? 'fsr1' : s.upscaler ? 'native' : on('WINE_FULLSCREEN_FSR') ? 'fsr1' : 'native',
    mangohud_preset: hud,
    shader_cache: s.shader_cache ?? (env.DXVK_STATE_CACHE === '0' ? 'off' : 'on'),
    winetricks: s.winetricks ?? [],
  };
}

/** Env vars derived from the structured fields only. */
export function derivedEnv(s: GameSettings): Record<string, string> {
  const n = normalizeSettings(s);
  const env: Record<string, string> = {};
  for (const f of n.proton_flags ?? []) env[f] = '1';
  if (n.sync_mode === 'esync') env.PROTON_NO_FSYNC = '1';
  if (n.sync_mode === 'ntsync') env.PROTON_USE_NTSYNC = '1';
  if (n.sync_mode === 'off') {
    env.PROTON_NO_ESYNC = '1';
    env.PROTON_NO_FSYNC = '1';
  }
  if (n.dx_layer === 'wined3d') env.PROTON_USE_WINED3D = '1';
  if (n.upscaler === 'fsr1') env.WINE_FULLSCREEN_FSR = '1';
  const hud = HUD_CONFIG[n.mangohud_preset ?? 'off'];
  if (hud) env.MANGOHUD_CONFIG = hud;
  if (n.shader_cache === 'off') env.DXVK_STATE_CACHE = '0';
  return env;
}

/** The game as the Go backend should see it: structured fields folded into env_vars. */
export function toBackendGame(game: Game): Game {
  const n = normalizeSettings(game.settings);
  return {
    ...game,
    settings: {
      ...n,
      use_mangohud: n.mangohud_preset !== 'off',
      env_vars: { ...n.env_vars, ...derivedEnv(n) },
    },
  };
}

/** Steam launches through `steam -applaunch`, so Platypus's env never reaches the
 *  game. This is the equivalent string to paste into Steam → Properties → Launch options. */
export function steamLaunchOptions(game: Game): string {
  const g = toBackendGame(game);
  const env = Object.entries(g.settings.env_vars)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${/\s/.test(v) ? `"${v}"` : v}`);
  const wrappers = [g.settings.use_gamemode && 'gamemoderun', g.settings.use_mangohud && 'mangohud'].filter(Boolean);
  return [...env, ...wrappers, '%command%', g.settings.launch_arguments].filter(Boolean).join(' ');
}

export function runnerShort(runnerId: string, runners: { id: string; name: string }[]): string {
  if (!runnerId || runnerId === 'auto') return 'auto';
  const r = runners.find((x) => x.id === runnerId || x.name === runnerId);
  return (r?.name ?? runnerId).split(' ')[0];
}

/** The runner option label (issue #48): the umu entry is labelled in the UI
 *  language, every other runner keeps the name the core reported. */
export function runnerDisplayName(t: TFunction, r: { id: string; name: string }): string {
  if (r.id === 'umu') return t('gameSettings:runner.umuOption');
  return r.name;
}

/** Whether every env var and wrapper of a tweak is already in the settings. */
export function tweakApplied(s: GameSettings, t: Tweak): boolean {
  const env = toBackendGame({ settings: s } as Game).settings.env_vars;
  const n = normalizeSettings(s);
  const envOk = Object.entries(t.env ?? {}).every(([k, v]) => env[k] === v);
  const wrapOk = (t.wrappers ?? []).every((w) => (w === 'gamemode' ? n.use_gamemode : w === 'mangohud' ? n.mangohud_preset !== 'off' : false));
  return envOk && wrapOk;
}

/** Turn a tweak on or off. Env is folded flat, edited, then re-normalized so
 *  managed keys (PROTON_USE_NTSYNC → sync_mode, etc.) land in their fields. */
export function setTweak(s: GameSettings, t: Tweak, on: boolean): GameSettings {
  const flat = toBackendGame({ settings: s } as Game).settings;
  const env = { ...flat.env_vars };
  for (const [k, v] of Object.entries(t.env ?? {})) {
    if (on) env[k] = v;
    else delete env[k];
  }
  let next: GameSettings = {
    ...flat,
    env_vars: env,
    proton_flags: undefined,
    sync_mode: undefined,
    dx_layer: undefined,
    upscaler: undefined,
    mangohud_preset: undefined,
    shader_cache: undefined,
  };
  for (const w of t.wrappers ?? []) {
    if (w === 'gamemode') next = { ...next, use_gamemode: on };
    if (w === 'mangohud') next = { ...next, use_mangohud: on };
  }
  return normalizeSettings(next);
}

// ---------------------------------------------------------------------------
// Gamescope: toggles <-> the free-form `gamescope_args` string.
// The Go launcher appends -w/-h/-r, then strings.Fields(gamescope_args), so the
// structured controls are folded into that string. Flag spellings checked
// against `gamescope --help`.

export const GS_TOGGLES: { key: GsToggleKey; flag: string; aliases: string[] }[] = [
  { key: 'fullscreen', flag: '-f', aliases: ['--fullscreen'] },
  { key: 'borderless', flag: '-b', aliases: ['--borderless'] },
  { key: 'grabCursor', flag: '--force-grab-cursor', aliases: [] },
  { key: 'hdr', flag: '--hdr-enabled', aliases: [] },
  { key: 'vrr', flag: '--adaptive-sync', aliases: [] },
  { key: 'wayland', flag: '--expose-wayland', aliases: [] },
];

/** The gamescope switch labels, for the pipeline toggles. */
export function gsToggleLabel(t: TFunction, key: GsToggleKey): string {
  switch (key) {
    case 'fullscreen': return t('gameSettings:gsToggles.fullscreen');
    case 'borderless': return t('gameSettings:gsToggles.borderless');
    case 'grabCursor': return t('gameSettings:gsToggles.grabCursor');
    case 'hdr': return t('gameSettings:gsToggles.hdr');
    case 'vrr': return t('gameSettings:gsToggles.vrr');
    default: return t('gameSettings:gsToggles.wayland');
  }
}

/** What a gamescope switch sets, for its tooltip. */
export function gsToggleTitle(t: TFunction, key: GsToggleKey): string {
  switch (key) {
    case 'fullscreen': return t('gameSettings:gsToggleTitles.fullscreen');
    case 'borderless': return t('gameSettings:gsToggleTitles.borderless');
    case 'grabCursor': return t('gameSettings:gsToggleTitles.grabCursor');
    case 'hdr': return t('gameSettings:gsToggleTitles.hdr');
    case 'vrr': return t('gameSettings:gsToggleTitles.vrr');
    default: return t('gameSettings:gsToggleTitles.wayland');
  }
}

export type GsToggleKey = 'fullscreen' | 'borderless' | 'grabCursor' | 'hdr' | 'vrr' | 'wayland';
export type GsFilter = '' | 'fsr' | 'nis' | 'linear' | 'nearest' | 'pixel';

export const GS_FILTERS: { value: GsFilter; title: string }[] = [
  { value: '', title: 'No -F flag' },
  { value: 'linear', title: '-F linear' },
  { value: 'nearest', title: '-F nearest' },
  { value: 'fsr', title: '-F fsr (AMD FidelityFX Super Resolution)' },
  { value: 'nis', title: '-F nis (NVIDIA Image Scaling)' },
  { value: 'pixel', title: '-F pixel' },
];

/** The gamescope upscale filter labels, for the segmented control. The titles
 *  stay here: they spell the -F flag itself, the same in every language. */
export function gsFilterLabel(t: TFunction, value: GsFilter): string {
  switch (value) {
    case 'linear': return t('gameSettings:gsFilters.linear');
    case 'nearest': return t('gameSettings:gsFilters.nearest');
    case 'fsr': return t('gameSettings:gsFilters.fsr');
    case 'nis': return t('gameSettings:gsFilters.nis');
    case 'pixel': return t('gameSettings:gsFilters.pixel');
    default: return t('gameSettings:gsFilters.default');
  }
}

export interface GsArgs {
  toggles: Record<GsToggleKey, boolean>;
  filter: GsFilter;
  /** Tokens we do not model, kept in their original order. */
  extra: string;
}

const GS_FILTER_VALUES = new Set<string>(GS_FILTERS.map((f) => f.value).filter(Boolean));

export function parseGamescopeArgs(args: string): GsArgs {
  const toggles = Object.fromEntries(GS_TOGGLES.map((t) => [t.key, false])) as Record<GsToggleKey, boolean>;
  let filter: GsFilter = '';
  const extra: string[] = [];
  const tokens = (args ?? '').split(/\s+/).filter(Boolean);
  for (let i = 0; i < tokens.length; i++) {
    const tok = tokens[i];
    const t = GS_TOGGLES.find((x) => x.flag === tok || x.aliases.includes(tok));
    if (t) {
      toggles[t.key] = true;
    } else if ((tok === '-F' || tok === '--filter') && GS_FILTER_VALUES.has(tokens[i + 1] ?? '')) {
      filter = tokens[++i] as GsFilter;
    } else {
      extra.push(tok);
    }
  }
  return { toggles, filter, extra: extra.join(' ') };
}

export function composeGamescopeArgs(a: GsArgs): string {
  const parts: string[] = GS_TOGGLES.filter((t) => a.toggles[t.key]).map((t) => t.flag);
  if (a.filter) parts.push('-F', a.filter);
  parts.push(...a.extra.split(/\s+/).filter(Boolean));
  return parts.join(' ');
}

export const GS_RESOLUTIONS: { w: number; h: number }[] = [
  { w: 1280, h: 720 },
  { w: 1280, h: 800 },
  { w: 1920, h: 1080 },
  { w: 1920, h: 1200 },
  { w: 2560, h: 1440 },
  { w: 3840, h: 2160 },
];
export const GS_RATES = [60, 75, 90, 120, 144, 165, 240];

/** Select value for a saved resolution: '' (game default), 'WxH' for a known one, 'custom' otherwise. */
export function resolutionChoice(w: number, h: number, display?: { w: number; h: number }): string {
  if (!w && !h) return '';
  const known = [...(display ? [display] : []), ...GS_RESOLUTIONS];
  return known.some((r) => r.w === w && r.h === h) ? `${w}x${h}` : 'custom';
}

export function rateChoice(r: number): string {
  if (!r) return '';
  return GS_RATES.includes(r) ? String(r) : 'custom';
}

// ---------------------------------------------------------------------------
// Env rows: editable KEY/value list, invalid keys are flagged and never saved.

export interface EnvRow {
  id: number;
  key: string;
  value: string;
}

export const ENV_KEY_RE = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function envKeyError(rows: EnvRow[], index: number, t: TFunction): string | null {
  const key = rows[index].key;
  if (key === '') return null; // empty row: just ignored
  if (!ENV_KEY_RE.test(key)) return t('gameSettings:launch.badKey');
  if (rows.slice(0, index).some((r) => r.key === key)) return t('gameSettings:launch.duplicateKey');
  return null;
}

/** Valid rows only. A duplicate key keeps the last value (like a shell would). */
export function rowsToEnv(rows: EnvRow[]): Record<string, string> {
  const env: Record<string, string> = {};
  for (const r of rows) if (ENV_KEY_RE.test(r.key)) env[r.key] = r.value;
  return env;
}

export function envToRows(env: Record<string, string>): EnvRow[] {
  return Object.entries(env).map(([key, value], id) => ({ id, key, value }));
}
