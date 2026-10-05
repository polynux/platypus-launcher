import type { Game, GameSettings, HudPreset, SyncMode, DxLayer, Tweak, Upscaler } from '../types';

// Proton flags the modal exposes. Each one is a real Proton env var toggled to 1.
export const PROTON_FLAGS: { key: string; desc: string }[] = [
  { key: 'PROTON_ENABLE_NVAPI', desc: 'Expose NVAPI to the game (DLSS, Reflex on NVIDIA)' },
  { key: 'PROTON_HIDE_NVIDIA_GPU', desc: 'Report NVIDIA GPUs as AMD to work around vendor checks' },
  { key: 'PROTON_ENABLE_WAYLAND', desc: 'Use the native Wayland driver (Proton 10+/GE)' },
  { key: 'PROTON_NO_D3D11', desc: 'Disable D3D11 so the game falls back to another API' },
  { key: 'PROTON_NO_D3D12', desc: 'Disable D3D12 so the game falls back to D3D11' },
  { key: 'PROTON_FORCE_LARGE_ADDRESS_AWARE', desc: 'Let 32-bit games use more than 2 GB of RAM' },
  { key: 'PROTON_OLD_GL_STRING', desc: 'Shorten the GL extension string for old games' },
  { key: 'PROTON_LOG', desc: 'Write a debug log to $HOME/steam-<appid>.log' },
];

export const SYNC_MODES: { value: SyncMode; label: string; title: string }[] = [
  { value: 'fsync', label: 'fsync', title: 'Proton default (fsync when the kernel supports it)' },
  { value: 'esync', label: 'esync', title: 'PROTON_NO_FSYNC=1' },
  { value: 'ntsync', label: 'ntsync', title: 'PROTON_USE_NTSYNC=1 (needs the ntsync kernel module)' },
  { value: 'off', label: 'off', title: 'PROTON_NO_ESYNC=1 PROTON_NO_FSYNC=1' },
];

export const DX_LAYERS: { value: DxLayer; label: string; title: string }[] = [
  { value: 'dxvk', label: 'DXVK (bundled)', title: 'The DXVK build shipped with the selected runner' },
  { value: 'wined3d', label: 'WineD3D', title: 'PROTON_USE_WINED3D=1' },
];

export const UPSCALERS: { value: Upscaler; label: string; title: string }[] = [
  { value: 'native', label: 'native', title: 'No upscaling' },
  { value: 'fsr1', label: 'FSR 1', title: 'WINE_FULLSCREEN_FSR=1 (GE-Proton only)' },
];

export const HUD_PRESETS: { value: HudPreset; label: string; title: string }[] = [
  { value: 'off', label: 'off', title: 'MangoHud disabled' },
  { value: 'fps only', label: 'fps only', title: 'MANGOHUD_CONFIG=preset=1' },
  { value: 'minimal', label: 'minimal', title: 'MANGOHUD_CONFIG=preset=2' },
  { value: 'full', label: 'full', title: 'MANGOHUD_CONFIG=preset=4' },
];

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
    upscaler: s.upscaler ?? (on('WINE_FULLSCREEN_FSR') ? 'fsr1' : 'native'),
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

/** Steam launches through `steam -applaunch`, so Hydra's env never reaches the
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
