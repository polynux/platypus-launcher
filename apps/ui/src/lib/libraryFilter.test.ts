// Issue #157: one filterLibrary for list, grid and table.
import { describe, expect, it } from 'vitest';
import { filterLibrary } from './libraryFilter';
import { atlasFilteredGames } from '../components/AtlasLibraryViews';
import type { Game } from '../types';

const game = (over: Partial<Game>): Game =>
  ({
    id: 'g',
    title: 'Test Game',
    store: 'steam',
    source: 'steam',
    is_installed: true,
    settings: {},
    ...over,
  }) as Game;

describe('filterLibrary', () => {
  it('filters by ownership, store, query and extra filters in one predicate', () => {
    const games = [
      game({ id: 'a', title: 'Alpha', store: 'steam', is_installed: true }),
      game({ id: 'b', title: 'Beta', store: 'gog', is_installed: false }),
    ];
    // Ownership: installed only
    expect(
      filterLibrary(games, {
        ownership: 'installed',
        source: 'all',
        tier: 'all',
        status: 'all',
        lending: 'all',
        query: '',
      }).map((g) => g.id),
    ).toEqual(['a']);
    // Store + query
    expect(
      filterLibrary(games, {
        ownership: 'owned',
        source: 'gog',
        tier: 'all',
        status: 'all',
        lending: 'all',
        query: 'bet',
      }).map((g) => g.id),
    ).toEqual(['b']);
    // Multi-store selection replaces the single source
    expect(
      filterLibrary(games, {
        ownership: 'owned',
        source: 'all',
        storeSelection: ['steam'],
        tier: 'all',
        status: 'all',
        lending: 'all',
        query: '',
      }).map((g) => g.id),
    ).toEqual(['a']);
  });

  it('atlasFilteredGames is filterLibrary plus sort (no drift)', () => {
    const games = [
      game({ id: 'b', title: 'Beta', is_installed: true }),
      game({ id: 'a', title: 'Alpha', is_installed: true }),
    ];
    const base = {
      ownership: 'owned' as const,
      source: 'all' as const,
      tier: 'all' as const,
      status: 'all' as const,
      lending: 'all' as const,
      query: '',
    };
    const filtered = filterLibrary(games, base);
    const sorted = atlasFilteredGames(games, { ...base, sort: 'title', dir: 'asc' });
    expect(filtered.map((g) => g.id).sort()).toEqual(['a', 'b']);
    expect(sorted.map((g) => g.id)).toEqual(['a', 'b']);
  });
});
