import { describe, it, expect } from 'vitest';
import type { Game } from '../types';
import { copyStatus, countdownLabel, dlcLabel, memberInitial, pickableCopies, playedCopy, roleLabel, sharedWithMe } from './family';

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
    expect(dlcLabel({ dlc: [{ id: 'x', name: 'Soundtrack' }, { id: 'y', name: 'Track pack' }] })).toBe('Soundtrack, Track pack');
    expect(dlcLabel({ dlc: [] })).toBe('no add-ons');
  });
});
