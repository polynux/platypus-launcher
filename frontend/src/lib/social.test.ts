import { describe, it, expect } from 'vitest';
import type { SocialFriend, SocialView } from '../types';
import { canSetStatus, connectionLine, describeSummary, groupFriends, initials, upsertView } from './social';

const f = (id: string, name: string, state: SocialFriend['state'], game?: SocialFriend['game']): SocialFriend => ({ id, name, state, game });

const view = (plugin: string, over: Partial<SocialView> = {}): SocialView => ({
  plugin,
  enabled: true,
  running: true,
  connection: { state: 'online' },
  self: { name: 'Me', state: 'online' },
  friends: [],
  ...over,
});

describe('groupFriends', () => {
  const friends = [
    f('1', 'Zoe', 'online'),
    f('2', 'Ana', 'away'),
    f('3', 'Bo', 'busy', { id: 'p.echo.tiny', title: 'Echo Tiny' }),
    f('4', 'Cy', 'offline'),
    f('5', 'Dee', 'invisible'),
    f('6', 'Eli', 'online', { title: 'Untitled Survival' }),
  ];

  it('puts a friend in a game first, then online (away and busy too), then offline', () => {
    const g = groupFriends(friends);
    expect(g.inGame.map((x) => x.name)).toEqual(['Bo', 'Eli']);
    expect(g.online.map((x) => x.name)).toEqual(['Ana', 'Zoe']);
    expect(g.offline.map((x) => x.name)).toEqual(['Cy', 'Dee']);
  });

  it('filters by name or by game title, case-insensitively', () => {
    expect(groupFriends(friends, 'ANA').online.map((x) => x.name)).toEqual(['Ana']);
    expect(groupFriends(friends, 'echo tiny').inGame.map((x) => x.name)).toEqual(['Bo']);
    const none = groupFriends(friends, 'nobody');
    expect(none.inGame.length + none.online.length + none.offline.length).toBe(0);
  });
});

describe('describeSummary', () => {
  it('counts online and in game across the enabled stores only', () => {
    const views = [
      view('echo', { friends: [f('1', 'A', 'online'), f('2', 'B', 'online', { title: 'X' }), f('3', 'C', 'offline')] }),
      view('other', { enabled: false, friends: [f('4', 'D', 'online')] }),
    ];
    expect(describeSummary(views)).toEqual({ online: 2, inGame: 1, text: '2 online · 1 in game' });
  });

  it('leaves out the in-game part when nobody plays', () => {
    expect(describeSummary([view('echo', { friends: [f('1', 'A', 'away')] })]).text).toBe('1 online');
  });

  it('is empty when no store is on', () => {
    expect(describeSummary([view('echo', { enabled: false })]).text).toBe('0 online');
  });
});

describe('connectionLine', () => {
  it('is quiet while online and explains every other state', () => {
    expect(connectionLine({ state: 'online' })).toBeNull();
    expect(connectionLine({ state: 'replaced' })).toBe('Another session replaced this one.');
    expect(connectionLine({ state: 'loggedOut' })).toBe('Signed out of the store.');
    expect(connectionLine({ state: 'reconnecting', message: 'the store plugin stopped' })).toBe(
      'Reconnecting… the store plugin stopped',
    );
  });
});

describe('canSetStatus', () => {
  it('needs an online, running, enabled store with a profile', () => {
    expect(canSetStatus(view('echo'))).toBe(true);
    expect(canSetStatus(view('echo', { self: undefined }))).toBe(false);
    expect(canSetStatus(view('echo', { connection: { state: 'reconnecting' } }))).toBe(false);
    expect(canSetStatus(view('echo', { running: false }))).toBe(false);
    expect(canSetStatus(view('echo', { enabled: false }))).toBe(false);
  });
});

describe('initials and upsertView', () => {
  it('takes the first letter of the first and last name', () => {
    expect(initials('Ana Ruiz')).toBe('AR');
    expect(initials('Plato')).toBe('P');
    expect(initials('  ')).toBe('?');
  });

  it('replaces the view of the same store and appends a new one', () => {
    const first = view('echo', { friends: [] });
    const list = upsertView([first], view('other'));
    expect(list.map((v) => v.plugin)).toEqual(['echo', 'other']);
    const replaced = upsertView(list, view('echo', { running: false }));
    expect(replaced).toHaveLength(2);
    expect(replaced[0].running).toBe(false);
    expect(list[0].running).toBe(true);
  });
});
