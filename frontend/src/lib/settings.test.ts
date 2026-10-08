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
