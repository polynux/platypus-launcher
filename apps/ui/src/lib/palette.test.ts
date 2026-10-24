import { describe, expect, it } from 'vitest';
import { UNFILTERED_GAME_LIMIT, filterPaletteItems, paletteScore, type PaletteItem } from './palette';

const item = (id: string, label: string, kind: PaletteItem['kind'] = 'place'): PaletteItem => ({
  id,
  group: kind === 'game' ? 'Games' : 'Go to',
  label,
  kind,
  run: () => {},
});

describe('paletteScore', () => {
  it('ranks an exact prefix above a substring above a subsequence', () => {
    expect(paletteScore('lib', 'Library')).toBeGreaterThan(paletteScore('brar', 'Library'));
    expect(paletteScore('brar', 'Library')).toBeGreaterThan(paletteScore('lbr', 'Library'));
  });

  it('ignores case and never matches an absent letter', () => {
    expect(paletteScore('LIB', 'Library')).toBe(paletteScore('lib', 'Library'));
    expect(paletteScore('xyz', 'Library')).toBe(0);
  });

  it('treats an empty query as a match for everything', () => {
    expect(paletteScore('', 'Library')).toBe(1);
  });
});

describe('filterPaletteItems', () => {
  it('keeps the built order with no query, and caps the games', () => {
    const games = Array.from({ length: UNFILTERED_GAME_LIMIT + 3 }, (_, i) => item(`g${i}`, `Game ${i}`, 'game'));
    const items = [item('p', 'Library'), ...games];
    const shown = filterPaletteItems(items, '');
    expect(shown[0].label).toBe('Library');
    expect(shown.filter((x) => x.kind === 'game')).toHaveLength(UNFILTERED_GAME_LIMIT);
  });

  it('drops rows that do not match and puts the best match first', () => {
    const items = [item('a', 'Achievements'), item('b', 'Activity'), item('c', 'Storage'), item('d', 'Act')];
    // Storage drops out; the two prefix matches keep their built order, and
    // the subsequence-only match sorts after them.
    expect(filterPaletteItems(items, 'act').map((x) => x.id)).toEqual(['b', 'd', 'a']);
  });

  it('returns nothing when nothing matches', () => {
    expect(filterPaletteItems([item('a', 'Library')], 'zzz')).toEqual([]);
  });
});
