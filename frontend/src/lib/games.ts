import type { Game } from '../types';

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
  return { steam: 'Steam', gog: 'GOG', epic: 'Epic', ubisoft: 'Ubisoft', custom: 'Sideloaded' }[s] ?? s;
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
