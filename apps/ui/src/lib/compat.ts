import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { Compat, CompatBadge, Game } from '../types';

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

/** What a ProtonDB tier means for the player, in ProtonDB's own terms. */
export const TIER_MEANING: Record<string, string> = {
  platinum: 'Works perfectly out of the box',
  gold: 'Works after small tweaks',
  silver: 'Works, with notable issues',
  bronze: 'Runs, with crashes or major issues',
  borked: "Doesn't run or isn't playable",
};

/** What a library card shows: a native Linux build, a ProtonDB tier, or nothing. */
export type CardCompat = { kind: 'native' } | { kind: 'tier'; tier: string; source: string; link?: string; checkedAt?: number } | null;

export function ratingBadge(g: Game) {
  return g.compat_badges?.find((b) => b.kind === 'rating');
}

/** The Steam Deck verdict and its associated notes, read from generic facts. */
export function steamDeckFact(badges?: CompatBadge[] | null) {
  const badge = badges?.find((b) => b.kind === 'verdict' && b.source === 'Steam Deck');
  if (!badge) return null;
  return {
    badge,
    rating: badge.tone || badge.label.toLowerCase(),
    notes: badges?.filter((b) => b.kind === 'note' && b.source === 'Steam Deck').map((b) => b.label) ?? [],
    checkedAt: badge.checked_at,
    link: badge.link,
  };
}

/** Native wins over a tier: a native game runs without Proton, so its tier is secondary. */
export function cardCompat(g: Game): CardCompat {
  if (g.native) return { kind: 'native' };
  const badge = ratingBadge(g);
  if (badge) return { kind: 'tier', tier: badge.tone || badge.label.toLowerCase(), source: badge.source, link: badge.link, checkedAt: badge.checked_at };
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

/** A saved badge filter compares the selected kind and its value (tone when set, else label). */
export function hasBadgeValue(g: Game, kind: string, value: string): boolean {
  if (kind === 'rating' && value === 'native') return !!g.native;
  const badges = g.compat_badges ?? [];
  if (kind === 'rating' && value === 'none') return !g.native && !badges.some((b) => b.kind === 'rating');
  return badges.some((b) => b.kind === kind && (!value || (b.tone || b.label.toLowerCase()) === value));
}

/** The tier filter's options, in the order the control shows them. */
export const TIER_FILTERS = ['all', 'native', 'platinum', 'gold', 'silver', 'bronze', 'borked', 'none'] as const;
export type TierFilter = (typeof TIER_FILTERS)[number];

/** What a Steam Deck verdict means for the player, in one word. */
export const DECK_LABEL: Record<string, string> = {
  verified: 'Verified',
  playable: 'Playable',
  unsupported: 'Unsupported',
};

/** The tooltip of a Steam Deck badge: the verdict, its caveats, and the source. */
export function deckTitle(rating: string, notes?: string[] | null): string {
  const head = `Steam Deck: ${DECK_LABEL[rating] ?? rating}.`;
  const body = (notes ?? []).filter(Boolean).join(' ');
  return `${head}${body ? ` ${body}` : ''} Steam Deck compatibility, from Steam.`;
}

/** YYYY-MM-DD in UTC for a unix time in seconds: the "as of" and "checked on" dates. */
export function asOfDate(unix: number): string {
  return new Date(unix * 1000).toISOString().slice(0, 10);
}

/** One line for a ProtonDB rating: the medal and how many reports, or why there is none. */
/** The tooltip of a DRM or anti-cheat badge: what it means, who said it, and when. */
export function badgeTitle(b: CompatBadge): string {
	const detail = (b.detail || b.label).replace(/[.。]+$/, '');
	return `${detail}. Source: ${b.source}${b.checked_at ? `, checked ${asOfDate(b.checked_at)}` : ''}.`;
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
const cache = new Map<string, { at: number; p: Promise<Compat>; badges: string }>();

/** The compat facts of a game, from the cache when they are recent. */
export function gameCompat(game: Game): Promise<Compat> {
  const badges = JSON.stringify(game.compat_badges ?? []);
  const hit = cache.get(game.id);
  if (hit && (!hit.badges || hit.badges === badges) && Date.now() - hit.at < CACHE_MS) return hit.p;
  // Starting inside a promise turns a failed start into a rejection the caller already handles.
  const p = Promise.resolve().then(() => Effect.runPromise(LauncherApi.getGameCompat(game)));
  cache.set(game.id, { at: Date.now(), p, badges });
  p.catch(() => cache.delete(game.id));
  return p;
}

/** Fired on window when a game's facts are replaced, so every view showing
 *  them (the game page and the settings modal, say) takes the new answer. */
export const COMPAT_CHANGED = 'platypus:compat-changed';

/** Replaces a game's cached facts, after the user changed its Steam AppID. */
export function setGameCompat(id: string, c: Compat): void {
  cache.set(id, { at: Date.now(), p: Promise.resolve(c), badges: cache.get(id)?.badges ?? '' });
  window.dispatchEvent(new CustomEvent<{ id: string; compat: Compat }>(COMPAT_CHANGED, { detail: { id, compat: c } }));
}

/** Test helper: forget every cached answer. */
export function clearCompatCache(): void {
  cache.clear();
}
