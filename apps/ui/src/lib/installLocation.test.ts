import { describe, it, expect } from 'vitest';
import { freeLabel, isShort, kindLabel, locationFor } from './installLocation';
import type { InstallLocation } from '../types';

describe('install location helpers', () => {
  it('says the room as the dialog shows it, and nothing for an unreadable disk', () => {
    expect(freeLabel(120e9)).toBe('120 GB free');
    expect(freeLabel(0)).toBe('0 B free');
    expect(freeLabel(-1)).toBe('');
    expect(freeLabel(undefined)).toBe('');
  });

  it('is short only when both sides are known and the game does not fit', () => {
    expect(isShort(30e9, 8e9)).toBe(true);
    expect(isShort(8e9, 8e9)).toBe(false);
    expect(isShort(null, 8e9)).toBe(false); // size unknown: the install checks again
    expect(isShort(30e9, -1)).toBe(false); // room unknown
    expect(isShort(0, 0)).toBe(false);
  });

  it('names each kind of place', () => {
    expect(kindLabel('default')).toBe('Default');
    expect(kindLabel('picked')).toBe('Chosen');
  });

  it('finds the place a value stands for', () => {
    const l = [
      { folder: '/a', kind: 'default' }, { folder: '/b', kind: 'recent' },
    ] as InstallLocation[];
    expect(locationFor(l, '')?.folder).toBe('/a');
    expect(locationFor(l, '/b')?.kind).toBe('recent');
    expect(locationFor(l, '/zzz')).toBeUndefined();
  });
});
