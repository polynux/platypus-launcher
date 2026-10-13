import type { FriendState, SocialFriend, SocialStatus, SocialView } from '../types';
import i18n from './i18n';

export const SOCIAL_STATUSES: SocialStatus[] = ['online', 'away', 'busy', 'snooze', 'invisible'];

/** A status a person can pick, as the friends modal names it. Unknown values
 *  pass through unchanged (a store's own word). */
const STATUS_KEY: Record<SocialStatus, string> = {
  online: 'chat:status.online',
  away: 'chat:status.away',
  busy: 'chat:status.busy',
  snooze: 'chat:status.snooze',
  invisible: 'chat:status.invisible',
};

export function statusLabel(s: string): string {
  const key = (STATUS_KEY as Record<string, string>)[s];
  return key ? i18n.t(key) : s;
}

/** A friend's state, as the friends modal names it. Unknown values pass
 *  through unchanged. */
const FRIEND_STATE_KEY: Record<FriendState, string> = {
  online: 'chat:state.online',
  offline: 'chat:state.offline',
  away: 'chat:state.away',
  busy: 'chat:state.busy',
  snooze: 'chat:state.snooze',
  looking: 'chat:state.looking',
  invisible: 'chat:state.invisible',
};

export function friendStateLabel(s: string): string {
  const key = (FRIEND_STATE_KEY as Record<string, string>)[s];
  return key ? i18n.t(key) : s;
}

/** Dot colour for a status or friend state: green online, amber away-ish, grey offline. */
export function stateDot(state: string): string {
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
  const parts = [i18n.t('chat:summary.online', { count: online })];
  if (inGame > 0) parts.push(i18n.t('chat:summary.inGame', { count: inGame }));
  return { online, inGame, text: parts.join(' · ') };
}

/** The banner text that explains a store's connection, or null when it is online. */
export function connectionLine(conn: SocialView['connection']): string | null {
  let text: string | null = null;
  switch (conn.state) {
    case 'connecting':
      text = i18n.t('chat:connection.connecting');
      break;
    case 'reconnecting':
      text = i18n.t('chat:connection.reconnecting');
      break;
    case 'replaced':
      text = i18n.t('chat:connection.replaced');
      break;
    case 'offline':
      text = i18n.t('chat:connection.offline');
      break;
    case 'loggedOut':
      text = i18n.t('chat:connection.loggedOut');
      break;
    default:
      text = null;
  }
  if (!text) return null;
  return conn.message ? `${text} ${conn.message}` : text;
}

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

/** Replaces (or adds) one store's view. Every event carries the whole view;
 *  one older than the view held (a lower seq, arrived late) is ignored. */
export function upsertView(list: SocialView[], view: SocialView): SocialView[] {
  const i = list.findIndex((v) => v.plugin === view.plugin);
  if (i < 0) return [...list, view];
  if (list[i].seq > view.seq) return list;
  const next = list.slice();
  next[i] = view;
  return next;
}

/** Brings the list in line with a fresh snapshot: each store's view is upserted,
 *  and a store the snapshot leaves out (no longer ready) is dropped. */
export function reconcileViews(list: SocialView[], snapshot: SocialView[]): SocialView[] {
  const plugins = new Set(snapshot.map((v) => v.plugin));
  return snapshot.reduce((acc, view) => upsertView(acc, view), list.filter((v) => plugins.has(v.plugin)));
}
