import type { Game, ProgressAchievement } from '../types';

/** Hours from minutes, one decimal. */
export function hoursOf(minutes: number): number {
  return Math.round(minutes / 6) / 10;
}

/** The playtime tooltip: the local record next to the store's report. Only
 *  shown when a plugin reported; without one the library value is all there is.
 *  The local part is left out when core has no local record (plugin games). */
export function playtimeTooltip(g: Game): string | undefined {
  if (g.playtime_store_minutes === undefined) return undefined;
  const store = `store ${hoursOf(g.playtime_store_minutes)} h`;
  return g.playtime_local_minutes === undefined ? store : `local ${hoursOf(g.playtime_local_minutes)} h · ${store}`;
}

/** Second line under an achievement: when it was unlocked, else the global share. */
export function achievementMeta(a: ProgressAchievement, format: (unix: number) => string = defaultDate): string {
  if (a.unlocked && a.unlocked_at) return `unlocked ${format(a.unlocked_at)}`;
  if (a.global_percent !== undefined) return `${a.global_percent.toFixed(1)}% of players`;
  return '';
}

const defaultDate = (unix: number) =>
  new Date(unix * 1000).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' });
