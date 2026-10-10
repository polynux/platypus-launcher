import type { CloudEvent, CloudInfo, CloudMode, CloudState, CloudStatus } from '../types';
import { formatBytes } from './games';

export const CLOUD_MODES: CloudMode[] = ['off', 'ask', 'auto'];

export const MODE_HELP: Record<CloudMode, string> = {
  off: "Platypus doesn't sync this game's saves.",
  ask: 'Syncs on its own when nothing clashes; asks when saves changed on both sides or the store is unreachable.',
  auto: 'Like ask, but once you launch anyway while the store is unreachable it stops asking for this game. Conflicts always ask.',
};

type ChipVariant = 'ok' | 'info' | 'bronze' | 'neutral' | 'borked';

export const STATE_CHIP: Record<CloudState, { label: string; variant: ChipVariant }> = {
  synced: { label: 'in sync', variant: 'ok' },
  pending: { label: 'changes waiting', variant: 'info' },
  conflict: { label: 'conflict', variant: 'bronze' },
  unsupported: { label: 'not supported', variant: 'neutral' },
  offline: { label: 'offline', variant: 'bronze' },
  error: { label: 'error', variant: 'borked' },
};

/** "3 in sync · 1 to download · 1 to upload": zero counts are left out. */
export function describeCounts(c: CloudStatus['counts']): string {
  const parts = [
    c.same > 0 && `${c.same} in sync`,
    c.download > 0 && `${c.download} to download`,
    c.upload > 0 && `${c.upload} to upload`,
    c.conflict > 0 && `${c.conflict} in conflict`,
    c.unmapped > 0 && `${c.unmapped} not placed on this device`,
  ].filter(Boolean) as string[];
  return parts.join(' · ');
}

/** A file size. Save files are small and two copies often differ by a few
 *  bytes, so below 1 MB it is the exact count ("3,705 B"); an empty file is "0 B". */
export function fileSize(n: number): string {
  if (n < 1_000_000) return `${Math.max(0, n).toLocaleString()} B`;
  return formatBytes(n);
}

/** "Mar 3, 2026, 4:12 PM" in the user's locale. */
export function whenLabel(unixSeconds: number): string {
  if (!unixSeconds) return 'unknown date';
  return new Date(unixSeconds * 1000).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
}

/** "just now", "5 min ago", "3 h ago", or a date for anything older. */
export function agoLabel(unixSeconds: number, nowMs = Date.now()): string {
  if (!unixSeconds) return '';
  const s = Math.max(0, Math.round(nowMs / 1000 - unixSeconds));
  if (s < 45) return 'just now';
  if (s < 3600) return `${Math.max(1, Math.round(s / 60))} min ago`;
  if (s < 86400) return `${Math.round(s / 3600)} h ago`;
  return whenLabel(unixSeconds);
}

/** What the mode control shows for a game: its own choice, or the plugin's default. */
export function modeLabel(info: Pick<CloudInfo, 'game_mode' | 'plugin_mode'>): string {
  return info.game_mode ? info.game_mode : `follows plugin · ${info.plugin_mode}`;
}

export interface CloudNotice {
  toast?: { msg: string; kind: 'ok' | 'err' | 'busy' };
  activity?: { detail: string; meta?: string };
}

/** What the user should see for a cloud event: a toast and/or an Activity line.
 *  Progress and plain status changes are silent; the game page shows them. */
export function cloudNotice(e: CloudEvent, title: string): CloudNotice | null {
  switch (e.kind) {
    case 'pushed':
      return { toast: { msg: `${e.message ?? 'Saves uploaded'} · ${title}`, kind: 'ok' }, activity: { detail: `${e.message ?? 'Saves uploaded'} · ${title}`, meta: 'cloud' } };
    case 'status':
      return e.message ? { toast: { msg: `${e.message} · ${title}`, kind: 'ok' }, activity: { detail: `${e.message} · ${title}`, meta: 'cloud' } } : null;
    case 'failed':
      return { toast: { msg: e.message ?? `${title}'s cloud sync failed`, kind: 'err' }, activity: { detail: `Cloud sync failed · ${title}`, meta: e.message?.slice(0, 80) } };
    case 'skipped':
      return { toast: { msg: e.message ?? `${title} was not synced`, kind: 'busy' }, activity: { detail: `Cloud sync skipped · ${title}`, meta: e.message?.slice(0, 80) } };
    case 'canceled':
      return { toast: { msg: 'Cloud sync canceled', kind: 'busy' }, activity: { detail: `Cloud sync canceled · ${title}` } };
    case 'prompt':
      return { activity: { detail: `Cloud saves need a decision · ${title}`, meta: e.prompt } };
    default:
      return null;
  }
}
