import { describe, expect, it } from 'vitest';
import type { GameSettings, Tweak } from '../types';
import { setTweak, steamLaunchOptions, tweakApplied } from './settings';

const base = { runner_id: 'auto', env_vars: {}, use_gamemode: true, launch_arguments: '-novid' } as unknown as GameSettings;
const ntsync: Tweak = { id: 'ntsync', kind: 'recommended', title: 'ntsync', reason: '', env: { PROTON_USE_NTSYNC: '1' }, wrappers: [] } as unknown as Tweak;
const gamemode: Tweak = { id: 'gamemode', kind: 'recommended', title: 'gm', reason: '', env: {}, wrappers: ['gamemode'] } as unknown as Tweak;

describe('tweaks edit the env vars they name', () => {
  it('turns a tweak on and off without touching the other env vars', () => {
    const on = setTweak({ ...base, env_vars: { FOO: 'bar' } }, ntsync, true);
    expect(on.env_vars).toEqual({ FOO: 'bar', PROTON_USE_NTSYNC: '1' });
    expect(tweakApplied(on, ntsync)).toBe(true);
    const off = setTweak(on, ntsync, false);
    expect(off.env_vars).toEqual({ FOO: 'bar' });
    expect(tweakApplied(off, ntsync)).toBe(false);
  });

  it('switches the wrappers a tweak names', () => {
    expect(tweakApplied({ ...base, use_gamemode: false }, gamemode)).toBe(false);
    expect(setTweak(base, gamemode, false).use_gamemode).toBe(false);
  });
});

describe('the Steam launch options', () => {
  it('spell out the env and wrappers the backend reports, then %command%', () => {
    const text = steamLaunchOptions({ settings: base }, { env: { PROTON_LOG: '1', MANGOHUD_CONFIG: 'preset=2' }, wrappers: ['gamemode', 'mangohud'] });
    expect(text).toBe('MANGOHUD_CONFIG=preset=2 PROTON_LOG=1 gamemoderun mangohud %command% -novid');
  });
});
