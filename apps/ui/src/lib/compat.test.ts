import { Effect } from 'effect';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Compat, Game } from '../types';

const getGameCompat = vi.fn();
vi.mock('../effect/runtime', () => ({
  LauncherApi: { getGameCompat: (g: Game) => getGameCompat(g) },
}));

import {
  appIdSourceLabel,
  asOfDate,
  badgeTitle,
  cardCompat,
  clearCompatCache,
  deckTitle,
  gameCompat,
  setGameCompat,
  tierFilterValue,
} from './compat';

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'echo', store: 'epic', store_app_id: id, title: id, is_installed: true, settings: {}, ...o }) as Game;

const compat = (o: Partial<Compat> = {}): Compat => ({ native: false, ratings_online: true, protection_online: true, badges: [], ...o });

describe('card compatibility', () => {
  it('shows a native build over a tier, and a tier when there is no native build', () => {
    expect(cardCompat(mk('a', { native: true, compat_badges: [{ kind: 'rating', label: 'Gold', tone: 'gold', source: 'ProtonDB', checked_at: 0 }] }))).toEqual({ kind: 'native' });
    expect(cardCompat(mk('b', { compat_badges: [{ kind: 'rating', label: 'Gold', tone: 'gold', source: 'ProtonDB', checked_at: 0 }] }))).toEqual({ kind: 'tier', tier: 'gold', source: 'ProtonDB', checkedAt: 0 });
  });

  it('shows nothing for a game with no answer', () => {
    expect(cardCompat(mk('c'))).toBeNull();
  });

  it('filters by native, a tier, or none', () => {
    expect(tierFilterValue(mk('a', { native: true }))).toBe('native');
    expect(tierFilterValue(mk('b', { compat_badges: [{ kind: 'rating', label: 'Platinum', tone: 'platinum', source: 'ProtonDB', checked_at: 0 }] }))).toBe('platinum');
    expect(tierFilterValue(mk('c'))).toBe('none');
  });
});

describe('wording', () => {


  it('dates in UTC and names the source of a badge', () => {
    expect(asOfDate(1791589136)).toBe('2026-10-09');
    const t = badgeTitle({ kind: 'online', label: 'Online required', detail: 'The store says so.', source: 'Epic', checked_at: 1791589136 });
    expect(t).toBe('The store says so. Source: Epic, checked 2026-10-09.');
  });

  it('names where a Steam AppID came from', () => {
    expect(appIdSourceLabel('user')).toBe('set by you');
    expect(appIdSourceLabel('plugin')).toBe('from the store');
    expect(appIdSourceLabel('')).toBe('none');
  });

  it('words a Deck verdict with its caveats and the source', () => {
    expect(deckTitle('verified')).toBe('Steam Deck: Verified. Steam Deck compatibility, from Steam.');
    expect(deckTitle('playable', ["Some text is too small to read on the Deck."])).toBe(
      "Steam Deck: Playable. Some text is too small to read on the Deck. Steam Deck compatibility, from Steam.",
    );
    expect(deckTitle('unsupported', [])).toBe('Steam Deck: Unsupported. Steam Deck compatibility, from Steam.');
  });
});

describe('gameCompat cache', () => {
  beforeEach(() => {
    clearCompatCache();
    getGameCompat.mockReset();
  });

  it('asks the backend once and shares the answer', async () => {
    getGameCompat.mockReturnValue(Effect.succeed(compat({ native: true })));
    const g = mk('one');
    const [a, b] = await Promise.all([gameCompat(g), gameCompat(g)]);
    expect(a.native).toBe(true);
    expect(b).toBe(a);
    expect(getGameCompat).toHaveBeenCalledTimes(1);
  });

  it('keeps a failed lookup out of the cache, so the next look asks again', async () => {
    getGameCompat.mockReturnValueOnce(Effect.fail(new Error('down')));
    getGameCompat.mockReturnValueOnce(Effect.succeed(compat()));
    const g = mk('two');
    await expect(gameCompat(g)).rejects.toThrow('down');
    await gameCompat(g);
    expect(getGameCompat).toHaveBeenCalledTimes(2);
  });

  it('takes a saved answer as the new one', async () => {
    setGameCompat('three', compat({ steam_app_id: '730', steam_app_id_source: 'user' }));
    const c = await gameCompat(mk('three'));
    expect(c.steam_app_id).toBe('730');
    expect(getGameCompat).not.toHaveBeenCalled();
  });
});
