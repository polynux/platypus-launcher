import { describe, it, expect } from 'vitest';
import type { Collection, Game } from '../types';
import { collectionCounts, collectionPredicate, matchesSmart, missingFrom, missingNames, moveCollection, staleTitles } from './collections';

const NOW = 1_800_000_000;
const DAY = 86400;

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, features: [], ...o }) as Game;

const none = () => new Set<string>();

describe('matchesSmart', () => {
  it('matches installed and played facts', () => {
    const played = mk('a', { is_installed: true, playtime_minutes: 30 });
    const idle = mk('b', { is_installed: false });
    expect(matchesSmart(played, { installed: true }, none, NOW)).toBe(true);
    expect(matchesSmart(idle, { installed: true }, none, NOW)).toBe(false);
    expect(matchesSmart(played, { played: true }, none, NOW)).toBe(true);
    expect(matchesSmart(idle, { played: false }, none, NOW)).toBe(true);
  });

  it('keeps games played within N days by their last-played time', () => {
    const recent = mk('r', { last_played_at: NOW - 3 * DAY });
    const old = mk('o', { last_played_at: NOW - 60 * DAY });
    expect(matchesSmart(recent, { played_within_days: 30 }, none, NOW)).toBe(true);
    expect(matchesSmart(old, { played_within_days: 30 }, none, NOW)).toBe(false);
    expect(matchesSmart(mk('n'), { played_within_days: 30 }, none, NOW)).toBe(false);
  });

  it('compares the store, the playtime, and the ProtonDB and Deck facts', () => {
    const g = mk('x', { store: 'gog', playtime_minutes: 120, fix_info: { tier: 'gold', deck: 'verified' } as Game['fix_info'] });
    expect(matchesSmart(g, { store: 'gog' }, none, NOW)).toBe(true);
    expect(matchesSmart(g, { store: 'epic' }, none, NOW)).toBe(false);
    expect(matchesSmart(g, { min_playtime_minutes: 120 }, none, NOW)).toBe(true);
    expect(matchesSmart(g, { min_playtime_minutes: 121 }, none, NOW)).toBe(false);
    expect(matchesSmart(g, { tier: 'gold' }, none, NOW)).toBe(true);
    expect(matchesSmart(g, { tier: 'none' }, none, NOW)).toBe(false);
    expect(matchesSmart(g, { deck: 'verified' }, none, NOW)).toBe(true);
    expect(matchesSmart(g, { deck: 'playable' }, none, NOW)).toBe(false);
  });

  it('treats a native build as its own tier, like the library filter does', () => {
    expect(matchesSmart(mk('n', { native: true }), { tier: 'native' }, none, NOW)).toBe(true);
    expect(matchesSmart(mk('p'), { tier: 'none' }, none, NOW)).toBe(true);
  });

  it('matches year ranges, and a game with no known year fails any range', () => {
    const g = mk('y', { release_year: 2004 });
    expect(matchesSmart(g, { year_from: 2000, year_to: 2010 }, none, NOW)).toBe(true);
    expect(matchesSmart(g, { year_from: 2005 }, none, NOW)).toBe(false);
    expect(matchesSmart(mk('u'), { year_to: 2010 }, none, NOW)).toBe(false);
  });

  it('matches the status, with none meaning no status at all', () => {
    expect(matchesSmart(mk('s', { status: 'playing' }), { status: 'playing' }, none, NOW)).toBe(true);
    expect(matchesSmart(mk('s', { status: 'playing' }), { status: 'none' }, none, NOW)).toBe(false);
    expect(matchesSmart(mk('t'), { status: 'none' }, none, NOW)).toBe(true);
  });

  it('tests membership of a manual collection through its members', () => {
    const members = (id: string) => (id === 'couch' ? new Set(['a']) : new Set<string>());
    expect(matchesSmart(mk('a'), { collection: 'couch' }, members, NOW)).toBe(true);
    expect(matchesSmart(mk('b'), { collection: 'couch' }, members, NOW)).toBe(false);
  });
});

describe('collectionPredicate', () => {
  const list: Collection[] = [
    { id: 'couch', name: 'Couch', games: ['a', 'gone'] },
    { id: 'installed', name: 'Installed', builtin: true, smart: { installed: true } },
    { id: 'co-installed', name: 'Couch and installed', smart: { collection: 'couch', installed: true } },
  ];
  const games = [mk('a'), mk('b', { is_installed: false }), mk('c')];

  it('is membership for a manual collection and the filter for a smart one', () => {
    expect(games.filter(collectionPredicate(list, 'couch', NOW)!).map((g) => g.id)).toEqual(['a']);
    expect(games.filter(collectionPredicate(list, 'installed', NOW)!).map((g) => g.id)).toEqual(['a', 'c']);
  });

  it('applies a smart collection that filters on a manual one', () => {
    expect(games.filter(collectionPredicate(list, 'co-installed', NOW)!).map((g) => g.id)).toEqual(['a']);
  });

  it('is null for a collection that is not in the list', () => {
    expect(collectionPredicate(list, 'missing', NOW)).toBeNull();
  });

  it('counts library games per collection', () => {
    expect(collectionCounts(list, games, NOW)).toEqual({ couch: 1, installed: 2, 'co-installed': 1 });
  });
});

describe('missingFrom', () => {
  it('lists the manual ids that are no longer in the library', () => {
    const c: Collection = { id: 'couch', name: 'Couch', games: ['a', 'gone', 'b'] };
    expect(missingFrom(c, [mk('a'), mk('b')])).toEqual(['gone']);
  });

  it('is empty for a smart collection', () => {
    expect(missingFrom({ id: 's', name: 'S', smart: { installed: true } }, [])).toEqual([]);
  });
});

describe('staleTitles', () => {
  it('asks to remember only the titles of library games that are out of date', () => {
    const list: Collection[] = [
      { id: 'couch', name: 'Couch', games: ['a', 'b', 'gone'], titles: { a: 'Old A', gone: 'Gone' } },
      { id: 'installed', name: 'Installed', smart: { installed: true } },
    ];
    expect(staleTitles(list, [mk('a', { title: 'Alpha' }), mk('b', { title: '' })])).toEqual({ a: 'Alpha' });
  });

  it('is empty once every title is current', () => {
    const list: Collection[] = [{ id: 'couch', name: 'Couch', games: ['a'], titles: { a: 'Alpha' } }];
    expect(staleTitles(list, [mk('a', { title: 'Alpha' })])).toEqual({});
  });
});

describe('missingNames', () => {
  it('gives each missing game its last known title, or its id', () => {
    const c: Collection = { id: 'couch', name: 'Couch', games: ['a', 'gone', 'lost'], titles: { gone: 'Gone Game' } };
    expect(missingNames(c, [mk('a')])).toEqual(['Gone Game', 'lost']);
  });
});

describe('moveCollection', () => {
  it('swaps a collection with its neighbour, and stops at the ends', () => {
    expect(moveCollection(['a', 'b', 'c'], 'b', -1)).toEqual(['b', 'a', 'c']);
    expect(moveCollection(['a', 'b', 'c'], 'b', 1)).toEqual(['a', 'c', 'b']);
    expect(moveCollection(['a', 'b', 'c'], 'a', -1)).toBeNull();
    expect(moveCollection(['a', 'b', 'c'], 'c', 1)).toBeNull();
    expect(moveCollection(['a', 'b'], 'zz', 1)).toBeNull();
  });
});
