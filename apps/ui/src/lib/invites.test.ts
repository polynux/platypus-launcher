import { describe, it, expect } from 'vitest';
import { friendInvite, inviteKey, joinDecision, splitConnectArgs } from './invites';
import type { Game } from '../types';

describe('splitConnectArgs', () => {
  it('splits the observed fragment shapes, honouring quotes and escapes', () => {
    expect(splitConnectArgs('+connect 1.2.3.4:27015')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(splitConnectArgs('+invite 30095787509687845 -launchsource2')).toEqual(['+invite', '30095787509687845', '-launchsource2']);
    expect(splitConnectArgs('+connect "1.2.3.4:27015"')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(splitConnectArgs('  +connect\t1.2.3.4:27015  ')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(splitConnectArgs('a\\ b c')).toEqual(['a b', 'c']);
    expect(splitConnectArgs('"a\\"b"')).toEqual(['a"b']);
    expect(splitConnectArgs('')).toEqual([]);
  });
  it('refuses an unclosed quote instead of guessing argv', () => {
    expect(splitConnectArgs('+connect "1.2.3.4:27015')).toBeNull();
  });
  it('caps at 16 tokens', () => {
    expect(splitConnectArgs(Array.from({ length: 20 }, (_, i) => `a${i}`).join(' '))).toHaveLength(16);
  });
});

const libGame = (over: Partial<Game> = {}): Game =>
  ({
    id: 'p.steam-standalone.620',
    title: 'Portal',
    source: 'steam-standalone',
    is_installed: true,
    client_launch: false,
    ...over,
  }) as Game;

describe('joinDecision', () => {
  it('launches an installed game with the split fragment', () => {
    const d = joinDecision({ kind: 'connect', connect: '+connect 1.2.3.4:27015' }, 'p.steam-standalone.620', [libGame()]);
    expect(d).toEqual({ kind: 'launch', game: libGame(), args: ['+connect', '1.2.3.4:27015'] });
  });
  it('launches a lobby join through a client-launched game', () => {
    const d = joinDecision({ kind: 'lobby', lobbyId: '109775241000000001' }, 'p.steam-standalone.620', [
      libGame({ client_launch: true }),
    ]);
    expect(d).toEqual({ kind: 'launch', game: libGame({ client_launch: true }), args: ['+connect_lobby', '109775241000000001'] });
  });
  it('opens the game page when it is not installed', () => {
    expect(
      joinDecision({ kind: 'connect', connect: '+connect 1.2.3.4:27015' }, 'p.steam-standalone.620', [
        libGame({ is_installed: false }),
      ]),
    ).toEqual({ kind: 'page', gameId: 'p.steam-standalone.620' });
  });
  it('disables a lobby join without a client launch, an unreadable fragment, and a missing game', () => {
    expect(joinDecision({ kind: 'lobby', lobbyId: '1' }, 'p.steam-standalone.620', [libGame()])).toEqual({
      kind: 'disabled',
      reason: 'client',
    });
    expect(joinDecision({ kind: 'connect', connect: '+connect "oops' }, 'p.steam-standalone.620', [libGame()])).toEqual({
      kind: 'disabled',
      reason: 'fragment',
    });
    expect(joinDecision({ kind: 'connect', connect: '+connect 1.2.3.4:27015' }, undefined, [libGame()])).toEqual({
      kind: 'disabled',
      reason: 'missing',
    });
  });
});

describe('friendInvite', () => {
  it('prefers the presence fragment and falls back to the lobby id', () => {
    expect(friendInvite({ id: '1', name: 'Ana', state: 'online', game: { title: 'G', joinable: true, connect: '+connect x' } })).toEqual({
      kind: 'connect',
      connect: '+connect x',
    });
    expect(friendInvite({ id: '1', name: 'Ana', state: 'online', game: { title: 'G', joinable: true, lobbyId: '7' } })).toEqual({
      kind: 'lobby',
      lobbyId: '7',
    });
    expect(friendInvite({ id: '1', name: 'Ana', state: 'online', game: { title: 'G' } })).toBeNull();
    expect(friendInvite({ id: '1', name: 'Ana', state: 'online' })).toBeNull();
  });
});

describe('inviteKey', () => {
  it('scopes a toast to the store, the friend and the time', () => {
    expect(inviteKey('steam', '1', 'connect', 9)).toBe('steam:1:connect:9');
  });
});
