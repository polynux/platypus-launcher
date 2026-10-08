import type { Game, GameSettings } from '../types';

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
    ? 'ProtonDB reports this title as borked. Try a different runner or check the reports.'
    : 'ProtonDB rates this title bronze. It may need tweaks to run well.';
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

/** 1.4 GB, 820 MB: decimal units, like file managers on Linux. */
export function formatBytes(n?: number): string {
  if (!n || n <= 0) return '';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let i = 0;
  let v = n;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
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
  if (!g.last_played_at) return undefined;
  const days = Math.floor((now / 1000 - g.last_played_at) / 86400);
  if (days < 1) return 'today';
  if (days < 2) return 'yesterday';
  if (days < 30) return `${days}d ago`;
  return new Date(g.last_played_at * 1000).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
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
