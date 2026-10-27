import { afterEach, describe, expect, it } from 'vitest';
import i18n from './i18n';
import { lastPlayedAgo } from './games';
import type { Game } from '../types';

const game = { last_played_at: 1000 } as Game;
afterEach(async () => { await i18n.changeLanguage('en'); });

describe('lastPlayedAgo', () => {
  it('formats relative time in the active locale', async () => {
    expect(lastPlayedAgo(game, 1000 * 1000 + 5 * 60_000)).toBe('5 min ago');
    await i18n.changeLanguage('fr');
    expect(lastPlayedAgo(game, 1000 * 1000 + 5 * 60_000)).toBe('il y a 5 min');
  });
});
