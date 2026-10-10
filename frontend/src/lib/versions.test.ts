import { describe, it, expect } from 'vitest';
import {
  VERSION_WARNING, choiceIsEmpty, isCurrent, pinLabel, pinOf, samePin, sourceLabel, targetOf,
} from './versions';
import type { VersionInfo } from '../types';

const info = (o: Partial<VersionInfo> = {}): VersionInfo => ({
  channels: [{ id: 'beta', name: 'Beta', requires: 'none', current: true }],
  builds: [{ id: 'b200', name: '2.0.0', source: 'store', available: true, current: false }, { id: 'b300', source: 'archive', available: true, current: true }],
  manual: null, links: [], updateAvailable: false, pinned: null, ledger: [], ...o,
});

describe('version targets', () => {
  it('names a channel or a build', () => {
    expect(targetOf({ kind: 'channel', id: 'beta' })).toEqual({ channel: 'beta' });
    expect(targetOf({ kind: 'build', id: 'b200' })).toEqual({ build: 'b200' });
  });

  it('sends typed text as it was typed, trimmed, in manual.input', () => {
    expect(targetOf({ kind: 'manual', text: '  481 7845\n482 9012 ' })).toEqual({ manual: { input: '481 7845\n482 9012' } });
  });

  it('never has a place for a password', () => {
    for (const c of [{ kind: 'channel', id: 'x' }, { kind: 'build', id: 'y' }, { kind: 'manual', text: 'z' }] as const) {
      expect(JSON.stringify(targetOf(c)).toLowerCase()).not.toContain('password');
    }
  });
});

describe('pins', () => {
  it('pins builds and channels but not typed text', () => {
    expect(pinOf({ kind: 'build', id: 'b1' })).toEqual({ kind: 'build', id: 'b1' });
    expect(pinOf({ kind: 'channel', id: 'beta' })).toEqual({ kind: 'channel', id: 'beta' });
    expect(pinOf({ kind: 'manual', text: 'x' })).toBeNull();
  });

  it('compares pins', () => {
    expect(samePin(null, undefined)).toBe(true);
    expect(samePin({ kind: 'build', id: 'a' }, { kind: 'build', id: 'a' })).toBe(true);
    expect(samePin({ kind: 'build', id: 'a' }, { kind: 'channel', id: 'a' })).toBe(false);
    expect(samePin({ kind: 'build', id: 'a' }, null)).toBe(false);
  });

  it('labels a pin with the listed name when there is one', () => {
    expect(pinLabel({ kind: 'build', id: 'b200' }, info())).toBe('build 2.0.0');
    expect(pinLabel({ kind: 'build', id: 'zzz' }, info())).toBe('build zzz');
    expect(pinLabel({ kind: 'channel', id: 'beta' }, info())).toBe('channel Beta');
    expect(pinLabel({ kind: 'channel', id: 'gone' })).toBe('channel gone');
  });
});

describe('choices', () => {
  it('knows when the choice is what is installed', () => {
    expect(isCurrent(info(), { kind: 'channel', id: 'beta' })).toBe(true);
    expect(isCurrent(info(), { kind: 'build', id: 'b300' })).toBe(true);
    expect(isCurrent(info(), { kind: 'build', id: 'b200' })).toBe(false);
    expect(isCurrent(info(), { kind: 'manual', text: 'b300' })).toBe(false);
    expect(isCurrent(info(), null)).toBe(false);
  });

  it('an empty typed value is no choice', () => {
    expect(choiceIsEmpty(null)).toBe(true);
    expect(choiceIsEmpty({ kind: 'manual', text: '  \n' })).toBe(true);
    expect(choiceIsEmpty({ kind: 'manual', text: 'b2' })).toBe(false);
    expect(choiceIsEmpty({ kind: 'build', id: 'b2' })).toBe(false);
  });

  it('says where a build came from', () => {
    expect(sourceLabel('store')).toBe('from the store');
    expect(sourceLabel('archive')).toBe('from your install history');
    expect(sourceLabel('manual')).toBe('typed by you');
  });

  it('warns about saves and online play', () => {
    expect(VERSION_WARNING).toMatch(/saves/);
    expect(VERSION_WARNING).toMatch(/online/);
  });
});
