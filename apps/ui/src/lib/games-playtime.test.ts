import { describe, it, expect } from 'vitest';
import { mergePlaytime } from './games';
import type { Game } from '../types';

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'p.echo.tiny',
    source: 'echo',
    store: 'echo',
    store_app_id: 'tiny',
    title: 'Echo Tiny',
    cover_url: '',
    banner_url: '',
    hero_url: '',
    features: [],
    install_dir: '',
    executable: '',
    is_installed: false,
    settings: {} as Game['settings'],
    ...over,
  };
}

describe('playtime merge', () => {
  it('takes only the playtime fields from a fresh library', () => {
    const prev = [game({ title: 'Kept', is_installed: true, playtime_minutes: 30, settings: { runner_id: 'x' } as Game['settings'] })];
    const fresh = [game({ title: 'Renamed', is_installed: false, playtime_minutes: 150, playtime_local_minutes: 30, playtime_store_minutes: 150 })];
    const [merged] = mergePlaytime(prev, fresh);
    expect(merged.title).toBe('Kept');
    expect(merged.is_installed).toBe(true);
    expect(merged.settings).toEqual({ runner_id: 'x' });
    expect(merged.playtime_minutes).toBe(150);
    expect(merged.playtime_store_minutes).toBe(150);
    expect(merged.playtime_local_minutes).toBe(30);
  });

  it('keeps the same list when no playtime changed', () => {
    const prev = [game({ playtime_minutes: 30 })];
    expect(mergePlaytime(prev, [game({ playtime_minutes: 30 })])).toBe(prev);
  });

  it('leaves games absent from the fresh list alone', () => {
    const prev = [game({ id: 'other', playtime_minutes: 5 })];
    expect(mergePlaytime(prev, [game({ playtime_minutes: 150 })])).toBe(prev);
  });
});
