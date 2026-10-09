import { describe, it, expect } from 'vitest';
import type { Game } from '../types';
import { countdownLabel, memberInitial, roleLabel, sharedWithMe } from './family';

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
