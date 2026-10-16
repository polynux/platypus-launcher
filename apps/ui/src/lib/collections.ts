import type { Collection, Game, SmartFilter } from '../types';
import { hasBadgeValue } from './compat';

/** Collections (issue #45): smart filters are evaluated here, on the library
 *  already loaded; the core only stores them. Membership of a manual collection
 *  is its game ids. Nothing in this module calls the core. */

/** Whether a game has been played: a last-played time or some playtime. */
export function hasPlayed(g: Game): boolean {
  return (g.last_played_at ?? 0) > 0 || (g.playtime_minutes ?? 0) > 0;
}

/** Whether a game matches every set field of a smart filter. `members` gives a
 *  manual collection's game ids (for the filter's collection facet); `nowSec` is
 *  the clock in unix seconds. */
export function matchesSmart(g: Game, f: SmartFilter, members: (collectionId: string) => ReadonlySet<string>, nowSec: number): boolean {
  if (f.store && g.store !== f.store) return false;
  if (f.installed != null && g.is_installed !== f.installed) return false;
  if (f.played != null && hasPlayed(g) !== f.played) return false;
  if (f.played_within_days && (g.last_played_at ?? 0) < nowSec - f.played_within_days * 86400) return false;
  if (f.min_playtime_minutes && (g.playtime_minutes ?? 0) < f.min_playtime_minutes) return false;
  if (f.badge_kind && !hasBadgeValue(g, f.badge_kind, f.badge_value ?? '')) return false;
  // A game with no known year matches no year range: it can't be shown to be in it.
  if (f.year_from || f.year_to) {
    const y = g.release_year ?? 0;
    if (!y || (f.year_from && y < f.year_from) || (f.year_to && y > f.year_to)) return false;
  }
  if (f.status && (f.status === 'none' ? g.status : g.status !== f.status)) return false;
  if (f.collection && !members(f.collection).has(g.id)) return false;
  return true;
}

/** The test a collection applies to a library game, or null when the collection
 *  isn't in the list. Each manual collection's ids are read once per call. */
export function collectionPredicate(collections: Collection[], id: string, nowSec: number): ((g: Game) => boolean) | null {
  const c = collections.find((x) => x.id === id);
  if (!c) return null;
  const memo = new Map<string, Set<string>>();
  const members = (cid: string): ReadonlySet<string> => {
    let s = memo.get(cid);
    if (!s) {
      s = new Set(collections.find((x) => x.id === cid)?.games ?? []);
      memo.set(cid, s);
    }
    return s;
  };
  if (c.smart) {
    const f = c.smart;
    return (g) => matchesSmart(g, f, members, nowSec);
  }
  const set = members(id);
  return (g) => set.has(g.id);
}

/** The ids a manual collection holds that aren't in the library now: they stay
 *  listed as missing (a game can leave the library and come back). */
export function missingFrom(c: Collection, games: Game[]): string[] {
  if (c.smart) return [];
  const have = new Set(games.map((g) => g.id));
  return (c.games ?? []).filter((id) => !have.has(id));
}

/** The names of the games a manual collection holds that aren't in the library:
 *  each one's last known title, its id only when no title was ever remembered. */
export function missingNames(c: Collection, games: Game[]): string[] {
  return missingFrom(c, games).map((id) => c.titles?.[id] || id);
}

/** The titles the core should remember: each library game a manual collection
 *  holds whose remembered title is out of date, by game id. Empty once every
 *  title is current, which is what stops the sync. */
export function staleTitles(collections: Collection[], games: Game[]): Record<string, string> {
  const byId = new Map(games.map((g) => [g.id, g]));
  const out: Record<string, string> = {};
  for (const c of collections) {
    if (c.smart) continue;
    for (const id of c.games ?? []) {
      const title = byId.get(id)?.title;
      if (title && c.titles?.[id] !== title) out[id] = title;
    }
  }
  return out;
}

/** How many library games each collection holds, by id. */
export function collectionCounts(collections: Collection[], games: Game[], nowSec: number): Record<string, number> {
  const out: Record<string, number> = {};
  for (const c of collections) {
    const test = collectionPredicate(collections, c.id, nowSec);
    out[c.id] = test ? games.filter(test).length : 0;
  }
  return out;
}

/** The order after moving one collection up or down by one place, or null when
 *  it is already at that end. */
export function moveCollection(ids: string[], id: string, delta: -1 | 1): string[] | null {
  const i = ids.indexOf(id);
  const j = i + delta;
  if (i < 0 || j < 0 || j >= ids.length) return null;
  const out = [...ids];
  [out[i], out[j]] = [out[j], out[i]];
  return out;
}
