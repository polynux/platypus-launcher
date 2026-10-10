import { describe, it, expect } from 'vitest';
import { mergeRescannedGames } from '../../lib/games';
import type { Game } from '../../types';

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'g1',
    source: 'steam',
    store: 'steam',
    store_app_id: '10',
    title: 'G',
    cover_url: '',
    banner_url: '',
    hero_url: '',
    features: [],
    install_dir: '/g',
    executable: '',
    is_installed: true,
    settings: {} as Game['settings'],
    ...over,
  };
}

describe('FE-5 fix_info survives rescan', () => {
  it('preserves fix_info when the backend list carries none', () => {
    const fix = { source: 'curated', tier: 'gold', description: 'fix it', winetricks: [], dll_overrides: {}, launch_args: '' };
    const prev = [game({ id: 'g1', fix_info: fix })];
    const next = [game({ id: 'g1', fix_info: undefined })];
    const merged = mergeRescannedGames(prev, next, {});
    expect(merged[0].fix_info).toEqual(fix);
  });

  it('prefers fresh backend fix_info when present', () => {
    const oldFix = { source: 'curated', tier: 'gold', description: 'old', winetricks: [], dll_overrides: {}, launch_args: '' };
    const newFix = { source: 'protondb', tier: 'silver', description: 'new', winetricks: [], dll_overrides: {}, launch_args: '' };
    const merged = mergeRescannedGames(
      [game({ id: 'g1', fix_info: oldFix })],
      [game({ id: 'g1', fix_info: newFix })],
      {}
    );
    expect(merged[0].fix_info).toEqual(newFix);
  });
});
