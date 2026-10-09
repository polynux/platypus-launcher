import type { FriendState, SocialConnState, SocialFriend, SocialStatus, SocialView } from '../types';

export const SOCIAL_STATUSES: SocialStatus[] = ['online', 'away', 'busy', 'snooze', 'invisible'];

export const STATUS_LABEL: Record<SocialStatus, string> = {
  online: 'Online',
  away: 'Away',
  busy: 'Busy',
  snooze: 'Snooze',
  invisible: 'Invisible',
};

export const FRIEND_STATE_LABEL: Record<FriendState, string> = {
  online: 'Online',
  offline: 'Offline',
  away: 'Away',
  busy: 'Busy',
  snooze: 'Snooze',
  looking: 'Looking to play',
  invisible: 'Offline',
};

/** Dot colour for a status or friend state: green online, amber away-ish, grey offline. */
export function stateDot(state: FriendState | SocialStatus): string {
  switch (state) {
    case 'online':
    case 'looking':
      return 'bg-emerald-400';
    case 'away':
    case 'busy':
    case 'snooze':
      return 'bg-amber-300';
    default:
      return 'bg-white/25';
  }
}

export interface FriendGroups {
  inGame: SocialFriend[];
  online: SocialFriend[];
  offline: SocialFriend[];
}

/** Whether a friend shows as online at all (in a game counts as online). */
export function isOnline(f: SocialFriend): boolean {
  return f.state !== 'offline' && f.state !== 'invisible';
}

/** Splits friends into the three groups the modal shows, each sorted by name.
 *  `query` matches the friend's name or the game's title, case-insensitively. */
export function groupFriends(friends: SocialFriend[], query = ''): FriendGroups {
  const q = query.trim().toLowerCase();
  const byName = (a: SocialFriend, b: SocialFriend) => a.name.localeCompare(b.name);
  const groups: FriendGroups = { inGame: [], online: [], offline: [] };
  for (const f of friends) {
    if (q && !f.name.toLowerCase().includes(q) && !(f.game?.title ?? '').toLowerCase().includes(q)) continue;
    if (f.game) groups.inGame.push(f);
    else if (isOnline(f)) groups.online.push(f);
    else groups.offline.push(f);
  }
  groups.inGame.sort(byName);
  groups.online.sort(byName);
  groups.offline.sort(byName);
  return groups;
}

/** "3 online · 1 in game" across the enabled stores; empty when none is on. */
export function describeSummary(views: SocialView[]): { online: number; inGame: number; text: string } {
  let online = 0;
  let inGame = 0;
  for (const v of views) {
    if (!v.enabled) continue;
    for (const f of v.friends) {
      if (isOnline(f)) online++;
      if (f.game) inGame++;
    }
  }
  const parts = [`${online} online`, inGame > 0 && `${inGame} in game`].filter(Boolean);
  return { online, inGame, text: parts.join(' · ') };
}

/** The banner text that explains a store's connection, or null when it is online. */
export function connectionLine(conn: SocialView['connection']): string | null {
  const text = CONNECTION_TEXT[conn.state];
  if (!text) return null;
  return conn.message ? `${text} ${conn.message}` : text;
}

const CONNECTION_TEXT: Partial<Record<SocialConnState, string>> = {
  connecting: 'Connecting…',
  reconnecting: 'Reconnecting…',
  replaced: 'Another session replaced this one.',
  offline: 'Offline.',
  loggedOut: 'Signed out of the store.',
};

/** A status can be changed only while the store session is online. */
export function canSetStatus(v: SocialView): boolean {
  return v.enabled && v.running && v.connection.state === 'online' && v.self !== undefined;
}

/** Up to two letters for an avatar placeholder: "Ana Ruiz" → "AR". */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  const first = parts[0][0] ?? '?';
  const last = parts.length > 1 ? parts[parts.length - 1][0] : '';
  return (first + last).toUpperCase();
}

/** Replaces (or adds) one store's view. Every event carries the whole view. */
export function upsertView(list: SocialView[], view: SocialView): SocialView[] {
  const i = list.findIndex((v) => v.plugin === view.plugin);
  if (i < 0) return [...list, view];
  const next = list.slice();
  next[i] = view;
  return next;
}
