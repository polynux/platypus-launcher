import type { CloudEvent, CloudInfo, CloudMode, CloudState, CloudStatus } from '../types';
import { formatBytes } from './games';
import { currentLocale } from './format';
import i18n from './i18n';
import { plainError } from './plainError';

export const CLOUD_MODES: CloudMode[] = ['off', 'ask', 'auto'];

/** The wire carries modes as plain strings; anything Platypus doesn't know
 *  means "ask" (the default), never a crash. */
export function asCloudMode(s: string): CloudMode {
  return s === 'off' || s === 'auto' ? s : 'ask';
}

/** What a sync mode means, as the game page and Settings name it. Unknown wire
 *  values read as "ask", like asCloudMode. */
const MODE_HELP_KEY: Record<CloudMode, string> = {
  off: 'cloud:modeHelp.off',
  ask: 'cloud:modeHelp.ask',
  auto: 'cloud:modeHelp.auto',
};

export function modeHelp(mode: string): string {
  return i18n.t(MODE_HELP_KEY[asCloudMode(mode)]);
}

/** A sync mode on its own, as the mode controls name it. */
const MODE_NAME_KEY: Record<CloudMode, string> = {
  off: 'cloud:mode.off',
  ask: 'cloud:mode.ask',
  auto: 'cloud:mode.auto',
};

export function modeName(mode: string): string {
  return i18n.t(MODE_NAME_KEY[asCloudMode(mode)]);
}

type ChipVariant = 'ok' | 'info' | 'bronze' | 'neutral' | 'borked';

const STATE_LABEL_KEY: Record<CloudState, string> = {
  synced: 'cloud:state.synced',
  pending: 'cloud:state.pending',
  conflict: 'cloud:state.conflict',
  unsupported: 'cloud:state.unsupported',
  offline: 'cloud:state.offline',
  error: 'cloud:state.error',
};

const STATE_VARIANT: Record<CloudState, ChipVariant> = {
  synced: 'ok',
  pending: 'info',
  conflict: 'bronze',
  unsupported: 'neutral',
  offline: 'bronze',
  error: 'borked',
};

/** The state chip of the game page: the translated label with its colour, or
 *  null for a state Platypus doesn't know (the panel falls back to its
 *  unchecked badge, as before). */
export function stateChip(state: string): { label: string; variant: ChipVariant } | null {
  if (!(state in STATE_LABEL_KEY)) return null;
  const s = state as CloudState;
  return { label: i18n.t(STATE_LABEL_KEY[s]), variant: STATE_VARIANT[s] };
}

/** "3 in sync · 1 to download · 1 to upload": zero counts are left out. */
export function describeCounts(c: CloudStatus['counts']): string {
  const parts = [
    c.same > 0 && i18n.t('cloud:counts.same', { count: c.same }),
    c.download > 0 && i18n.t('cloud:counts.download', { count: c.download }),
    c.upload > 0 && i18n.t('cloud:counts.upload', { count: c.upload }),
    c.conflict > 0 && i18n.t('cloud:counts.conflict', { count: c.conflict }),
    c.unmapped > 0 && i18n.t('cloud:counts.unmapped', { count: c.unmapped }),
  ].filter(Boolean) as string[];
  return parts.join(' · ');
}

/** A file size. Save files are small and two copies often differ by a few
 *  bytes, so below 1 MB it is the exact count ("3,705 B"); an empty file is "0 B". */
export function fileSize(n: number, locale: string = currentLocale()): string {
  if (n < 1_000_000) return `${Math.max(0, n).toLocaleString(locale)} B`;
  return formatBytes(n);
}

/** "Mar 3, 2026, 4:12 PM" in the user's locale. */
export function whenLabel(unixSeconds: number, locale: string = currentLocale()): string {
  if (!unixSeconds) return i18n.t('cloud:unknownDate');
  return new Date(unixSeconds * 1000).toLocaleString(locale, { dateStyle: 'medium', timeStyle: 'short' });
}

/** "just now", "5 min ago", "3 h ago", or a date for anything older. */
export function agoLabel(unixSeconds: number, nowMs = Date.now()): string {
  if (!unixSeconds) return '';
  const s = Math.max(0, Math.round(nowMs / 1000 - unixSeconds));
  if (s < 45) return i18n.t('cloud:ago.now');
  if (s < 3600) return i18n.t('cloud:ago.minutes', { count: Math.max(1, Math.round(s / 60)) });
  if (s < 86400) return i18n.t('cloud:ago.hours', { count: Math.round(s / 3600) });
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
    case 'pushed': {
      const msg = e.message ?? i18n.t('cloud:notice.uploaded');
      return { toast: { msg: `${msg} · ${title}`, kind: 'ok' }, activity: { detail: `${msg} · ${title}`, meta: 'cloud' } };
    }
    case 'status':
      return e.message ? { toast: { msg: `${e.message} · ${title}`, kind: 'ok' }, activity: { detail: `${e.message} · ${title}`, meta: 'cloud' } } : null;
    case 'failed':
      return { toast: { msg: e.message ?? i18n.t('cloud:notice.failed', { title }), kind: 'err' }, activity: { detail: i18n.t('cloud:notice.failedDetail', { title }), meta: plainError(e.message, i18n.t('cloud:notice.openGame')) } };
    case 'skipped':
      return { toast: { msg: e.message ?? i18n.t('cloud:notice.skipped', { title }), kind: 'busy' }, activity: { detail: i18n.t('cloud:notice.skippedDetail', { title }), meta: e.message?.slice(0, 80) } };
    case 'canceled':
      return { toast: { msg: i18n.t('cloud:notice.canceled'), kind: 'busy' }, activity: { detail: `${i18n.t('cloud:notice.canceled')} · ${title}` } };
    case 'prompt':
      return { activity: { detail: i18n.t('cloud:notice.prompt', { title }), meta: e.prompt } };
    default:
      return null;
  }
}
