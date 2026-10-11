import { describe, it, expect } from 'vitest';
import { coverImages } from './games';
import type { Game } from '../types';

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'p.steam-standalone.400',
    source: 'steam-standalone',
    store: 'steam',
    store_app_id: '400',
    title: 'Portal',
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

describe('coverImages', () => {
  it('tries the portrait first, then the landscape fallbacks', () => {
    const g = game({ cover_url: 'cover.jpg', banner_url: 'header.jpg', hero_url: 'hero.jpg' });
    expect(coverImages(g)).toEqual(['cover.jpg', 'header.jpg', 'hero.jpg']);
  });

  it('drops missing art and dedupes repeats', () => {
    expect(coverImages(game({ banner_url: 'header.jpg' }))).toEqual(['header.jpg']);
    expect(coverImages(game({ cover_url: 'same.jpg', banner_url: 'same.jpg' }))).toEqual(['same.jpg']);
    expect(coverImages(game())).toEqual([]);
  });
});
