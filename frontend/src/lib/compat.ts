import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { Compat, CompatBadge, Game, ProtonRating } from '../types';

/** Compatibility facts for the library and the game page (internal/compat). */

/** The credit PCGamingWiki asks for under every fact read from it (CC BY-NC-SA 3.0). */
export const PCGW_ATTRIBUTION = 'Source: PCGamingWiki contributors, CC BY-NC-SA 3.0';

export const TIER_LABEL: Record<string, string> = {
  platinum: 'Platinum',
  gold: 'Gold',
  silver: 'Silver',
  bronze: 'Bronze',
  borked: 'Borked',
};

/** What a library card shows: a native Linux build, a ProtonDB tier, or nothing. */
export type CardCompat = { kind: 'native' } | { kind: 'tier'; tier: string; source: string } | null;

/** Native wins over a tier: a native game runs without Proton, so its tier is secondary. */
export function cardCompat(g: Game): CardCompat {
  if (g.native) return { kind: 'native' };
  const f = g.fix_info;
  if (f?.tier) return { kind: 'tier', tier: f.tier, source: f.source };
  return null;
}

/** The value the tier filter compares a game with: 'native', a tier, or 'none'. */
export function tierFilterValue(g: Game): string {
  const c = cardCompat(g);
  if (!c) return 'none';
  return c.kind === 'native' ? 'native' : c.tier;
}

/** The tier filter's options, in the order the control shows them. */
export const TIER_FILTERS = ['all', 'native', 'platinum', 'gold', 'silver', 'bronze', 'borked', 'none'] as const;
export type TierFilter = (typeof TIER_FILTERS)[number];

/** YYYY-MM-DD in UTC for a unix time in seconds: the "as of" and "checked on" dates. */
export function asOfDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

/** One line for a ProtonDB rating: the medal and how many reports, or why there is none. */
export function ratingLine(r: ProtonRating): string {
  if (r.no_reports) return 'No ProtonDB reports';
  if (!r.tier || r.few_reports) return `Few ProtonDB reports (${r.total})`;
  return `${TIER_LABEL[r.tier] ?? r.tier} · ${r.total} reports`;
}

/** The tooltip of a DRM or anti-cheat badge: what it means, who said it, and when. */
export function badgeTitle(b: CompatBadge): string {
  return `${b.detail} Source: ${b.source}, checked ${asOfDate(b.checked_at)}.`;
}

/** Where the Steam AppID of a rating came from, in words for the game page. */
export function appIdSourceLabel(source?: string): string {
  switch (source) {
    case 'user':
      return 'set by you';
    case 'plugin':
      return 'from the store';
    case 'steam':
      return 'Steam game';
    default:
      return 'none';
  }
}

// The compat answer of a game is fetched once and shared by every view that
// asks within a few minutes: the library, the game page and its settings.
const CACHE_MS = 5 * 60 * 1000;
const cache = new Map<string, { at: number; p: Promise<Compat> }>();

/** The compat facts of a game, from the cache when they are recent. */
export function gameCompat(game: Game): Promise<Compat> {
  const hit = cache.get(game.id);
  if (hit && Date.now() - hit.at < CACHE_MS) return hit.p;
  // Starting inside a promise turns a failed start into a rejection the caller already handles.
  const p = Promise.resolve().then(() => Effect.runPromise(LauncherApi.getGameCompat(game)));
  cache.set(game.id, { at: Date.now(), p });
  p.catch(() => cache.delete(game.id));
  return p;
}

/** Fired on window when a game's facts are replaced, so every view showing
 *  them (the game page and the settings modal, say) takes the new answer. */
export const COMPAT_CHANGED = 'hydra:compat-changed';

/** Replaces a game's cached facts, after the user changed its Steam AppID. */
export function setGameCompat(id: string, c: Compat): void {
  cache.set(id, { at: Date.now(), p: Promise.resolve(c) });
  window.dispatchEvent(new CustomEvent<{ id: string; compat: Compat }>(COMPAT_CHANGED, { detail: { id, compat: c } }));
}

/** Test helper: forget every cached answer. */
export function clearCompatCache(): void {
  cache.clear();
}
