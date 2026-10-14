import { describe, it, expect } from 'vitest';
import { sortKey, currentArt, hasCustomization } from './customize';
import { mergeCustomization } from './games';
import { sortGames } from '../components/LibraryView';
import type { Game } from '../types';

const g = (o: Partial<Game>) => ({ id: 'x', title: 'T', cover_url: '', banner_url: '', ...o }) as unknown as Game;

describe('customization helpers', () => {
  it('sorts by the sort title when there is one', () => {
    expect(sortKey(g({ title: 'The Witness', sort_title: 'Witness' }))).toBe('witness');
    expect(sortKey(g({ title: 'The Witness' }))).toBe('the witness');
    const list = [g({ id: '1', title: 'The Witness', sort_title: 'Witness' }), g({ id: '2', title: 'Braid' }), g({ id: '3', title: 'Zelda' })];
    expect(sortGames(list, { key: 'title', dir: 'asc' }).map((x) => x.id)).toEqual(['2', '1', '3']);
  });

  it('reads the art a game shows now', () => {
    const x = g({ cover_url: 'c', hero_url: 'h', banner_url: 'b', logo_url: 'l' });
    expect(currentArt(x, 'cover')).toBe('c');
    expect(currentArt(x, 'banner')).toBe('h');
    expect(currentArt(x, 'logo')).toBe('l');
    expect(currentArt(x, 'icon')).toBe('');
  });

  it('knows an empty customization', () => {
    expect(hasCustomization(null)).toBe(false);
    expect(hasCustomization({ title: '', sort_title: '', art: {} })).toBe(false);
    expect(hasCustomization({ title: '', sort_title: '', art: { icon: 'i' } })).toBe(true);
  });
});

describe('mergeCustomization', () => {
  it('carries only the user\'s own fields of a fresh read into the games on screen', () => {
    const prev = [g({ id: 'a', title: 'A', status: 'playing', user_notes: 'mine', is_installed: false } as Partial<Game>), g({ id: 'b', title: 'B' })];
    const fresh = [g({ id: 'a', title: 'Mine', store_title: 'A', cover_url: '/platypus-art/x/cover.png', customized: ['title', 'cover'], status: 'finished', user_notes: 'new' } as Partial<Game>), g({ id: 'b', title: 'B' })];
    const next = mergeCustomization(prev, fresh);
    // Status and notes are the user's own (issue #51), like their title and art.
    expect(next[0]).toMatchObject({ id: 'a', title: 'Mine', cover_url: '/platypus-art/x/cover.png', status: 'finished', user_notes: 'new' });
    // Fields the user doesn't own stay as shown.
    expect(next[0].is_installed).toBe(false);
    expect(next[1]).toBe(prev[1]);
  });

  it('returns the same list when nothing changed', () => {
    const prev = [g({ id: 'a' })];
    expect(mergeCustomization(prev, [g({ id: 'a' })])).toBe(prev);
  });
});
