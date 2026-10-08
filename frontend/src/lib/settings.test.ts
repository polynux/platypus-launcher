import { describe, it, expect } from 'vitest';
import { normalizeSettings } from './settings';
import type { GameSettings } from '../types';

describe('S2 upscaler normalization', () => {
  const base = { env_vars: {} } as unknown as GameSettings;
  it('maps legacy integer to native', () => {
    expect(normalizeSettings({ ...base, upscaler: 'integer' as never }).upscaler).toBe('native');
  });
  it('keeps fsr1 and native, derives fsr1 from env', () => {
    expect(normalizeSettings({ ...base, upscaler: 'fsr1' }).upscaler).toBe('fsr1');
    expect(normalizeSettings({ ...base, upscaler: 'native' }).upscaler).toBe('native');
    expect(normalizeSettings({ env_vars: { WINE_FULLSCREEN_FSR: '1' } } as unknown as GameSettings).upscaler).toBe('fsr1');
  });
});

import { parseGamescopeArgs, composeGamescopeArgs, resolutionChoice, rateChoice } from './settings';

describe('gamescope args', () => {
  it('parses known flags, filter and keeps unknown tokens in order', () => {
    const a = parseGamescopeArgs('-f --mangoapp --adaptive-sync -F fsr --rt --borderless');
    expect(a.toggles.fullscreen && a.toggles.vrr && a.toggles.borderless).toBe(true);
    expect(a.toggles.hdr).toBe(false);
    expect(a.filter).toBe('fsr');
    expect(a.extra).toBe('--mangoapp --rt');
  });
  it('round-trips (canonical order, extras preserved)', () => {
    const out = composeGamescopeArgs(parseGamescopeArgs('--mangoapp -b -f --rt --hdr-enabled'));
    expect(out).toBe('-f -b --hdr-enabled --mangoapp --rt');
    expect(composeGamescopeArgs(parseGamescopeArgs(out))).toBe(out);
  });
  it('accepts long aliases and keeps unsupported filter values as extras', () => {
    expect(composeGamescopeArgs(parseGamescopeArgs('--fullscreen --filter nis'))).toBe('-f -F nis');
    const a = parseGamescopeArgs('-F sgsr -F');
    expect(a.filter).toBe('');
    expect(a.extra).toBe('-F sgsr -F');
  });
  it('handles empty input', () => {
    expect(composeGamescopeArgs(parseGamescopeArgs(''))).toBe('');
  });
});

describe('gamescope resolution and rate choice', () => {
  const display = { w: 2560, h: 1440 };
  it('detects presets, display and unset', () => {
    expect(resolutionChoice(0, 0, display)).toBe('');
    expect(resolutionChoice(1920, 1080, display)).toBe('1920x1080');
    expect(resolutionChoice(3440, 1440, { w: 3440, h: 1440 })).toBe('3440x1440');
  });
  it('shows an unknown saved value as custom, including half-set pairs', () => {
    expect(resolutionChoice(3440, 1440, display)).toBe('custom');
    expect(resolutionChoice(1920, 0, display)).toBe('custom');
  });
  it('rate', () => {
    expect(rateChoice(0)).toBe('');
    expect(rateChoice(144)).toBe('144');
    expect(rateChoice(100)).toBe('custom');
  });
});

import { envKeyError, envToRows, rowsToEnv } from './settings';

describe('env rows', () => {
  const rows = (...kv: [string, string][]) => kv.map(([key, value], id) => ({ id, key, value }));
  it('round-trips valid env', () => {
    const env = { A: '1', _B2: 'x y' };
    expect(rowsToEnv(envToRows(env))).toEqual(env);
  });
  it('flags invalid keys and excludes them from the env', () => {
    const r = rows(['OK', '1'], ['1BAD', '2'], ['has space', '3'], ['A-B', '4'], ['', 'ignored']);
    expect(envKeyError(r, 0)).toBeNull();
    expect(envKeyError(r, 1)).toBeTruthy();
    expect(envKeyError(r, 2)).toBeTruthy();
    expect(envKeyError(r, 3)).toBeTruthy();
    expect(envKeyError(r, 4)).toBeNull();
    expect(rowsToEnv(r)).toEqual({ OK: '1' });
  });
  it('flags a duplicate on the later row and the last value wins', () => {
    const r = rows(['A', '1'], ['A', '2']);
    expect(envKeyError(r, 0)).toBeNull();
    expect(envKeyError(r, 1)).toMatch(/Duplicate/);
    expect(rowsToEnv(r)).toEqual({ A: '2' });
  });
});
