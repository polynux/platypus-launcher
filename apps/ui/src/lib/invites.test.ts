import { describe, it, expect } from 'vitest';
import { friendInvite, inviteKey, joinDecision, parseJoinFragment, splitConnectArgs } from './invites';
import type { Game } from '../types';

describe('splitConnectArgs', () => {
  it('splits on whitespace', () => {
    expect(splitConnectArgs('+connect 1.2.3.4:27015')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(splitConnectArgs('  +connect\t1.2.3.4:27015  ')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(splitConnectArgs('')).toEqual([]);
  });
  it('refuses quotes and backslashes instead of guessing argv', () => {
    expect(splitConnectArgs('+connect "1.2.3.4:27015"')).toBeNull();
    expect(splitConnectArgs('a\\ b')).toBeNull();
  });
});

describe('parseJoinFragment', () => {
  it('accepts the exact allowlist shapes', () => {
    expect(parseJoinFragment('connect', '+connect example.com')).toEqual(['+connect', 'example.com']);
    expect(parseJoinFragment('connect', '+connect 1.2.3.4:27015')).toEqual(['+connect', '1.2.3.4:27015']);
    expect(parseJoinFragment('connect', '+connect 1.2.3.4:27015 +password s3cret')).toEqual([
      '+connect',
      '1.2.3.4:27015',
      '+password',
      's3cret',
    ]);
    expect(parseJoinFragment('connect', '+connect [2001:db8::1]:27015')).toEqual(['+connect', '[2001:db8::1]:27015']);
    expect(parseJoinFragment('connect', '+invite 30095787509687845')).toEqual(['+invite', '30095787509687845']);
    expect(parseJoinFragment('connect', '+invite 30095787509687845 -launchsource2')).toEqual([
      '+invite',
      '30095787509687845',
      '-launchsource2',
    ]);
    expect(parseJoinFragment('lobby', undefined, '109775241000000001')).toEqual(['+connect_lobby', '109775241000000001']);
  });
  it('refuses everything else', () => {
    expect(parseJoinFragment('connect', '-insecure')).toBeNull();
    expect(parseJoinFragment('connect', '+exec x')).toBeNull();
    expect(parseJoinFragment('connect', '+connect a b')).toBeNull();
    expect(parseJoinFragment('connect', '+connect 1.2.3.4:27015 +exec x')).toBeNull();
    expect(parseJoinFragment('connect', '+connect 1.2.3.4:99999')).toBeNull();
    expect(parseJoinFragment('connect', '+connect ho_st!')).toBeNull();
    expect(parseJoinFragment('connect', '+connect ::1')).toBeNull();
    expect(parseJoinFragment('connect', '+connect h:1 +password -x')).toBeNull();
    expect(parseJoinFragment('lobby', undefined, `1${'0'.repeat(20)}`)).toBeNull();
    expect(parseJoinFragment('lobby', undefined, '0')).toBeNull();
    expect(parseJoinFragment('lobby')).toBeNull();
    expect(parseJoinFragment('Connect', '+connect 1.2.3.4:27015')).toBeNull();
    expect(parseJoinFragment('stream', '+connect 1.2.3.4:27015')).toBeNull();
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
  it('launches an installed game with the join args', () => {
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
    expect(joinDecision({ kind: 'connect', connect: '-insecure' }, 'p.steam-standalone.620', [libGame()])).toEqual({
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
  it('scopes a toast to the store, the friend and the app', () => {
    expect(inviteKey('steam', '1', '620')).toBe('steam:1:620');
  });
});
