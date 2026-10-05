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
 *  the library hero art from the same CDN when it exists. */
export function heroImages(g: Game): string[] {
  const list: string[] = [];
  if (g.store === 'steam' && g.store_app_id) {
    list.push(`https://steamcdn-a.akamaihd.net/steam/apps/${g.store_app_id}/library_hero.jpg`);
  }
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
