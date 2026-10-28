import type { Game, GameSettings, Job, ProtonVersion } from '../types';
import { formatBytes as formatBytesLocale, formatLastPlayed } from './format';
import i18n from './i18n';
import { ratingBadge } from './compat';

export type AttentionIssue = { game: Game; kind: 'install' | 'outside' | 'runner' | 'update' | 'conflict' | 'prefix' | 'rating'; error?: string; runnerId?: string; failedJobKind?: 'install' | 'update' };

/** Issues the frontend can identify from its current library, job and runner snapshots. */
export function attentionIssues(games: Game[], jobs: Job[], runners: ProtonVersion[]): AttentionIssue[] {
  const latest = new Map<string, Job>();
  for (const job of jobs) {
    const prev = latest.get(job.game_id);
    if (!prev || job.updated_at > prev.updated_at) latest.set(job.game_id, job);
  }
  const out: AttentionIssue[] = [];
  for (const game of games) {
    const job = latest.get(game.id);
    if (job?.state === 'failed' && (job.kind === 'install' || job.kind === 'update')) {
      out.push({ game, kind: 'install', error: job.error ?? undefined, failedJobKind: job.kind });
    }
    if (game.outside_folder) out.push({ game, kind: 'outside' });
    const runnerId = game.settings.runner_id;
    const runner = runners.find((r) => r.id === runnerId || r.name === runnerId);
    if (!game.native && runnerId && runnerId !== 'auto' && !runner) {
      out.push({ game, kind: 'runner', runnerId });
    }
    if (game.update_available && game.is_installed) out.push({ game, kind: 'update' });
    // Save conflicts and damaged prefixes come from the core (issue #144):
    // the library carries them, the frontend never derives them. The rating
    // row is only for the rating signal, so a signal game shows once.
    if (game.has_save_conflict) out.push({ game, kind: 'conflict' });
    if (game.prefix_damaged) out.push({ game, kind: 'prefix' });
    if (hasRatingSignal(game)) out.push({ game, kind: 'rating' });
  }
  return out;
}

/** A game "needs attention" when it has a curated fix recipe, a poor rating
 *  badge, a save conflict, or a damaged Wine prefix (issue #144). */
export function needsAttention(g: Game): boolean {
  return g.has_save_conflict || g.prefix_damaged || hasRatingSignal(g);
}

/** The rating signal alone: a curated fix recipe or a poor rating badge. */
function hasRatingSignal(g: Game): boolean {
  if (g.fix_info?.description) return true;
  const tier = ratingBadge(g)?.tone;
  return tier === 'bronze' || tier === 'borked';
}

export function attentionReason(g: Game, kind?: AttentionIssue['kind']): string {
  // The actionable signals speak first; the rating speaks when alone or on
  // its own row. A kind narrows to that row's text.
  if ((kind === undefined || kind === 'conflict') && g.has_save_conflict) return i18n.t('fixes:attention.conflict');
  if ((kind === undefined || kind === 'prefix') && g.prefix_damaged) return i18n.t('fixes:attention.prefix');
  const f = g.fix_info;
  if (f?.description) return f.description;
  const badge = ratingBadge(g);
  if (badge?.tone !== 'bronze' && badge?.tone !== 'borked') return '';
  return badge.tone === 'borked'
    ? i18n.t('fixes:attention.borked', { source: badge.source })
    : i18n.t('fixes:attention.bronze', { source: badge.source });
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

/** Relative "ago" for the atlas list rows (issue #128), mirroring `ago()` in
 *  tools/design-lab-2/src/core/launcher.js: "6 h ago", "yesterday",
 *  "2 mo ago". Never a calendar date: anything past a month counts in months,
 *  past a year in years. last_played_at is unix seconds; now is ms. */
export function lastPlayedAgo(g: Game, now = Date.now()): string | undefined {
  if (!g.last_played_at) return undefined;
  const ms = Math.max(0, now - g.last_played_at * 1000);
  const m = ms / 60e3;
  if (m < 1) return i18n.t('game:timeAgo.justNow');
  if (m < 60) return i18n.t('game:timeAgo.minutes', { count: Math.round(m) });
  const h = m / 60;
  if (h < 24) return i18n.t('game:timeAgo.hours', { count: Math.round(h) });
  const d = h / 24;
  if (d < 2) return i18n.t('game:timeAgo.yesterday');
  if (d < 30) return i18n.t('game:timeAgo.days', { count: Math.round(d) });
  if (d < 365) return i18n.t('game:timeAgo.months', { count: Math.round(d / 30) });
  return i18n.t('game:timeAgo.years', { count: Math.round(d / 365) });
}

/** Most recently played first; never-played titles keep scan order at the end. */
export function byRecent(a: Game, b: Game): number {
  return (b.last_played_at ?? 0) - (a.last_played_at ?? 0);
}

/** Merge a fresh backend scan into the current list (FE-5). Edited settings
 *  are kept across rescans like before; `fix_info` is carried over too
 *  so curated recipes and Deck verdicts survive a rescan.
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

/** Take only the needs-attention signals (issue #144) from a fresh library
 *  read, so a reset clears its card without a rescan. Returns `prev` itself
 *  when nothing changed. */
export function mergeAttentionSignals(prev: Game[], fresh: Game[]): Game[] {
  const byId = new Map(fresh.map((x) => [x.id, x]));
  let changed = false;
  const next = prev.map((g) => {
    const f = byId.get(g.id);
    if (!f) return g;
    const same = g.has_save_conflict === f.has_save_conflict && g.prefix_damaged === f.prefix_damaged;
    if (same) return g;
    changed = true;
    return { ...g, has_save_conflict: f.has_save_conflict, prefix_damaged: f.prefix_damaged };
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
  // The user's own status and notes (issue #51), merged like their art.
  'status', 'user_notes',
] as const;

/** Take only what the user's own settings set (their title, art, status and
 *  notes) from a fresh library read, so editing them shows without a rescan.
 *  Returns `prev` itself when nothing changed. */
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
