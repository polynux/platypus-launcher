import type { Game, GameSettings } from '../types';
import { formatBytes as formatBytesLocale, formatLastPlayed } from './format';
import i18n from './i18n';

/** A game "needs attention" when it has a curated fix recipe or a poor ProtonDB tier. */
export function needsAttention(g: Game): boolean {
  const f = g.fix_info;
  if (!f) return false;
  if (f.source !== 'protondb') return true;
  return f.tier === 'bronze' || f.tier === 'borked';
}

export function attentionReason(g: Game): string {
  const f = g.fix_info;
  if (!f) return '';
  if (f.source !== 'protondb') return f.description;
  return f.tier === 'borked'
    ? i18n.t('fixes:attention.borked')
    : i18n.t('fixes:attention.bronze');
}

/** Portrait images to try in order. Older Steam titles 404 on the portrait
 *  (library_600x900.jpg) and only have the landscape header: the card crops
 *  it (object-cover) instead of showing a title placeholder. */
export function coverImages(g: Game): string[] {
  return [...new Set([g.cover_url, g.banner_url, g.hero_url].filter(Boolean))];
}

/** Wide images to try in order. Steam's header.jpg is only 460px wide, so prefer
 *  the library hero art when it exists. */
export function heroImages(g: Game): string[] {
  const list: string[] = [];
  if (g.hero_url) list.push(g.hero_url);
  if (g.banner_url) list.push(g.banner_url);
  if (g.cover_url) list.push(g.cover_url);
  return list;
}

export function storeLabel(s: string): string {
  return { steam: 'Steam', gog: 'GOG', epic: 'Epic', ubisoft: 'Ubisoft', custom: 'Sideloaded' }[s] ?? s.charAt(0).toUpperCase() + s.slice(1);
}

/** 1.4 GB, 820 MB: decimal units, like file managers on Linux. The number
 *  follows the UI locale (see lib/format); English output is unchanged. */
export function formatBytes(n?: number): string {
  return formatBytesLocale(n);
}

/** Whether the game comes from a store plugin. */
export function isPluginGame(g: Game): boolean {
  return g.source !== 'steam' && g.source !== 'custom' && g.source !== '';
}

export function greeting(d = new Date()): string {
  const h = d.getHours();
  if (h < 5) return 'Up late';
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
}

/** Hours played, one decimal, or undefined when Steam has no record. */
export function hoursPlayed(g: Game): number | undefined {
  return g.playtime_minutes ? Math.round(g.playtime_minutes / 6) / 10 : undefined;
}

export function lastPlayedLabel(g: Game, now = Date.now()): string | undefined {
  return formatLastPlayed(g.last_played_at, now);
}

/** Most recently played first; never-played titles keep scan order at the end. */
export function byRecent(a: Game, b: Game): number {
  return (b.last_played_at ?? 0) - (a.last_played_at ?? 0);
}

/** Merge a fresh backend scan into the current list (FE-5). Edited settings
 *  are kept across rescans like before; `fix_info` is carried over too
 *  because the backend list never carries it (enrichment is frontend-side).
 *  Fresh backend `fix_info`, when present, wins. */
export function mergeRescannedGames(
  prev: Game[],
  next: Game[],
  saved: Record<string, GameSettings>,
): Game[] {
  const edited = new Map(prev.map((x) => [x.id, x.settings]));
  const fixById = new Map(prev.map((x) => [x.id, x.fix_info]));
  return next.map((x) => {
    const s = edited.get(x.id) ?? (saved[x.id] ? { ...x.settings, ...saved[x.id] } : undefined);
    const fix = x.fix_info ?? fixById.get(x.id);
    const withSettings = s ? { ...x, settings: s } : x;
    return fix && !withSettings.fix_info ? { ...withSettings, fix_info: fix } : withSettings;
  });
}

/** Take only the playtime fields from a fresh library read (protocol 1.7), so a
 *  playtime report updates the library without a rescan. Returns `prev` itself
 *  when nothing changed. */
export function mergePlaytime(prev: Game[], fresh: Game[]): Game[] {
  const byId = new Map(fresh.map((x) => [x.id, x]));
  let changed = false;
  const next = prev.map((g) => {
    const f = byId.get(g.id);
    if (!f) return g;
    const same =
      g.playtime_minutes === f.playtime_minutes &&
      g.playtime_local_minutes === f.playtime_local_minutes &&
      g.playtime_store_minutes === f.playtime_store_minutes;
    if (same) return g;
    changed = true;
    return {
      ...g,
      playtime_minutes: f.playtime_minutes,
      playtime_local_minutes: f.playtime_local_minutes,
      playtime_store_minutes: f.playtime_store_minutes,
    };
  });
  return changed ? next : prev;
}

/** Take only the family fields (who shares a borrowed game, its copies and the
 *  preferred copy) from a fresh library read, so a copy pick shows without a
 *  rescan. Returns `prev` itself when nothing changed. */
export function mergeCopies(prev: Game[], fresh: Game[]): Game[] {
  const byId = new Map(fresh.map((x) => [x.id, x]));
  let changed = false;
  const next = prev.map((g) => {
    const f = byId.get(g.id);
    if (!f) return g;
    const same =
      g.preferred_copy === f.preferred_copy &&
      JSON.stringify(g.copies ?? null) === JSON.stringify(f.copies ?? null) &&
      JSON.stringify(g.shared_by ?? null) === JSON.stringify(f.shared_by ?? null);
    if (same) return g;
    changed = true;
    return { ...g, copies: f.copies, preferred_copy: f.preferred_copy, shared_by: f.shared_by };
  });
  return changed ? next : prev;
}

const CUSTOM_FIELDS = [
  'title', 'store_title', 'sort_title', 'cover_url', 'banner_url', 'hero_url', 'logo_url', 'icon_url', 'background_url',
  'release_year',
] as const;

/** Take only what the user's customization sets (their title and art) from a
 *  fresh library read, so editing it shows without a rescan. Returns `prev`
 *  itself when nothing changed. */
export function mergeCustomization(prev: Game[], fresh: Game[]): Game[] {
  const byId = new Map(fresh.map((x) => [x.id, x]));
  let changed = false;
  const next = prev.map((g) => {
    const f = byId.get(g.id);
    if (!f) return g;
    const same = CUSTOM_FIELDS.every((k) => g[k] === f[k]) && JSON.stringify(g.customized ?? null) === JSON.stringify(f.customized ?? null);
    if (same) return g;
    changed = true;
    const merged: Game = { ...g, customized: f.customized };
    for (const k of CUSTOM_FIELDS) (merged as unknown as Record<string, unknown>)[k] = f[k];
    return merged;
  });
  return changed ? next : prev;
}
