import { describe, it, expect } from 'vitest';
import type { CopyDiff, Game, GameCopy } from '../types';
import { compareRows, copyDiffSummary, copyStatus, countdownLabel, diffFor, dlcLabel, memberInitial, pickableCopies, playedCopy, roleLabel, sharedWithMe } from './family';

describe('family helpers', () => {
  it('names only the roles the store can mean', () => {
    expect(roleLabel('adult')).toBe('adult');
    expect(roleLabel('child')).toBe('child');
    expect(roleLabel('owner')).toBe('');
    expect(roleLabel('')).toBe('');
  });

  it('shows the initial of the name, or of the id without one', () => {
    expect(memberInitial({ id: 'ana', name: 'ana' })).toBe('A');
    expect(memberInitial({ id: 'ben', name: '' })).toBe('B');
  });

  it('keeps the games a family member lends', () => {
    const games = [{ id: 'own' }, { id: 'lent', shared_by: { id: 'ana', name: 'Ana' } }] as Game[];
    expect(sharedWithMe(games).map((g) => g.id)).toEqual(['lent']);
  });

  it('counts down only when the plugin gave a time', () => {
    expect(countdownLabel(42)).toBe('42 s left');
    expect(countdownLabel(0)).toBe('');
  });
});

describe('family copies', () => {
  const copy = (owner_id: string, o: Partial<import('../types').GameCopy> = {}) =>
    ({ owner_id, owner_name: owner_id, available: true, ...o }) as import('../types').GameCopy;

  it('plays the copy the user picked, else the lender\'s', () => {
    expect(playedCopy({ shared_by: { id: 'ana', name: 'Ana' }, preferred_copy: 'ben' })).toBe('ben');
    expect(playedCopy({ shared_by: { id: 'ana', name: 'Ana' }, preferred_copy: '' })).toBe('ana');
    expect(playedCopy({ shared_by: null, preferred_copy: 'ben' })).toBeNull();
  });

  it('offers copies only for a borrowed game', () => {
    expect(pickableCopies({ shared_by: null, copies: [copy('ana')] })).toEqual([]);
    expect(pickableCopies({ shared_by: { id: 'ana', name: 'Ana' }, copies: null })).toEqual([]);
    expect(pickableCopies({ shared_by: { id: 'ana', name: 'Ana' }, copies: [copy('ana')] })).toHaveLength(1);
  });

  it('labels a copy that is unavailable or in use, and names its add-ons', () => {
    expect(copyStatus(copy('a', { available: false }))).toBe('unavailable');
    expect(copyStatus(copy('a', { in_use: true }))).toBe('in use');
    expect(copyStatus(copy('a'))).toBe('');
    expect(dlcLabel({ dlc: [{ id: 'x', name: 'Soundtrack', kind: 'dlc' }, { id: 'y', name: 'Track pack', kind: 'dlc' }] })).toBe('Soundtrack, Track pack');
    expect(dlcLabel({ dlc: [] })).toBe('no add-ons');
  });
});

describe('copy comparison', () => {
  const diff = (o: Partial<CopyDiff> = {}): CopyDiff => ({
    owner_id: 'ana', only_here: 0, only_baseline: 0, shared: 0,
    soundtracks_only_here: 0, soundtracks_only_baseline: 0, soundtracks_shared: 0, ...o,
  });

  it('says what a copy has more or fewer of, soundtracks apart', () => {
    expect(copyDiffSummary('Ben', 'Ana', diff({ only_here: 3 }))).toBe("Ben's copy has 3 more DLC than Ana's.");
    expect(copyDiffSummary('Ben', 'Ana', diff({ only_here: 3, soundtracks_only_here: 1 }))).toBe("Ben's copy has 3 more DLC and 1 more soundtrack than Ana's.");
    expect(copyDiffSummary('Ben', 'Ana', diff({ only_baseline: 2, soundtracks_only_baseline: 2 }))).toBe("Ben's copy has 2 fewer DLC and 2 fewer soundtracks than Ana's.");
    expect(copyDiffSummary('Ben', 'Ana', diff({ only_here: 1, only_baseline: 2 }))).toBe("Ben's copy has 1 more and 2 fewer DLC than Ana's.");
  });

  it('says so when the add-ons match, a shared soundtrack included', () => {
    expect(copyDiffSummary('Ben', 'Ana', diff({ shared: 4, soundtracks_shared: 1 }))).toBe("Ben's copy has the same add-ons as Ana's.");
  });

  it('keeps the add-on rows the table shows: differences only drops the shared ones, soundtracks wait for the toggle', () => {
    const copies: GameCopy[] = [
      { owner_id: 'ana', owner_name: 'Ana', available: true, dlc: [{ id: 'x', name: 'X', kind: 'dlc' }, { id: 'ost', name: 'OST', kind: 'soundtrack' }] },
      { owner_id: 'ben', owner_name: 'Ben', available: true, dlc: [{ id: 'x', name: 'X', kind: 'dlc' }, { id: 'y', name: 'Y', kind: 'other' }] },
    ];
    expect(compareRows(copies, { differencesOnly: true, soundtracks: false }).map((r) => r.id)).toEqual(['y']);
    expect(compareRows(copies, { differencesOnly: false, soundtracks: false }).map((r) => r.id)).toEqual(['x', 'y']);
    expect(compareRows(copies, { differencesOnly: true, soundtracks: true }).map((r) => r.id)).toEqual(['ost', 'y']);
    const row = compareRows(copies, { differencesOnly: false, soundtracks: true }).find((r) => r.id === 'y');
    expect(row).toMatchObject({ kind: 'other', owners: ['ben'] });
  });

  it('finds the comparison of one copy', () => {
    const cmp = { baseline: 'ben', diffs: [diff({ owner_id: 'ana', only_here: 1 })] };
    expect(diffFor(cmp, 'ana')?.only_here).toBe(1);
    expect(diffFor(cmp, 'ben')).toBeUndefined();
    expect(diffFor(null, 'ana')).toBeUndefined();
  });
});
