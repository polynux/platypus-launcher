// i18n: converted
import type { TFunction } from 'i18next';
import type { GameSettings, RunnerEnv, Tweak } from '../types';

// The runner's choices (Proton flags, sync, DX layer, MangoHud, gamescope) are
// typed game settings that the backend turns into env vars and arguments
// (runner.options, runner.env). This module only formats and edits what the
// user picks; it never builds an env var from them.

export const WINETRICKS_VERBS = ['vcrun2022', 'vcrun2019', 'd3dx9', 'd3dcompiler_47', 'dotnet48', 'corefonts', 'xact', 'mfc140'];

export function runnerShort(runnerId: string, runners: { id: string; name: string }[]): string {
  if (!runnerId || runnerId === 'auto') return 'auto';
  const r = runners.find((x) => x.id === runnerId || x.name === runnerId);
  return (r?.name ?? runnerId).split(' ')[0];
}

/** Whether a tweak is already on: its env vars are in the game's env and its
 *  wrappers are switched on. */
export function tweakApplied(s: GameSettings, t: Tweak): boolean {
  const env = s.env_vars ?? {};
  const envOk = Object.entries(t.env ?? {}).every(([k, v]) => env[k] === v);
  const mangohud = s.mangohud_preset ? s.mangohud_preset !== 'off' : !!s.use_mangohud;
  const wrapOk = (t.wrappers ?? []).every((w) => (w === 'gamemode' ? !!s.use_gamemode : w === 'mangohud' ? mangohud : false));
  return envOk && wrapOk;
}

/** Turn a tweak on or off: its env vars are set or removed, its wrappers switched. */
export function setTweak(s: GameSettings, t: Tweak, on: boolean): GameSettings {
  const env = { ...(s.env_vars ?? {}) };
  for (const [k, v] of Object.entries(t.env ?? {})) {
    if (on) env[k] = v;
    else delete env[k];
  }
  let next: GameSettings = { ...s, env_vars: env };
  for (const w of t.wrappers ?? []) {
    if (w === 'gamemode') next = { ...next, use_gamemode: on };
    if (w === 'mangohud') next = { ...next, mangohud_preset: on ? 'full' : 'off' };
  }
  return next;
}

/** The Steam launch options that set the game up the way Platypus would: the
 *  runner's env and wrappers (from the backend), then %command% and the
 *  launch arguments. Steam starts the game itself, so Platypus can't pass them. */
export function steamLaunchOptions(game: { settings: GameSettings }, runner: RunnerEnv): string {
  const env = Object.entries(runner.env ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${/\s/.test(v) ? `"${v}"` : v}`);
  const wrappers = (runner.wrappers ?? []).map((w) => (w === 'gamemode' ? 'gamemoderun' : w));
  return [...env, ...wrappers, '%command%', game.settings.launch_arguments].filter(Boolean).join(' ');
}

/** The runner option label (issue #48): the umu entry is labelled in the UI
 *  language, every other runner keeps the name the core reported. */
export function runnerDisplayName(t: TFunction, r: { id: string; name: string }): string {
  if (r.id === 'umu') return t('gameSettings:runner.umuOption');
  return r.name;
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
