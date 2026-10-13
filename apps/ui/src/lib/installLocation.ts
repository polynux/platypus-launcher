// i18n: converted
import type { TFunction } from 'i18next';
import type { InstallLocation } from '../types';
import { formatFreeBytes } from './format';

// Install location (the install dialog): what the choices mean for the user.
// Core checks the folder again when the install is queued; these keep the
// dialog's view of the same facts.

/** The room at a place as the dialog says it ("120 GB free" in English);
 *  the suffix follows the UI locale (see lib/format). Empty when unreadable. */
export function freeLabel(free: number | undefined): string {
  return free != null && free >= 0 ? formatFreeBytes(free) : '';
}

/** Whether `needed` bytes don't fit in `free`. Unknown on either side fits: the
 *  install checks the room again and nothing is blocked on a guess. */
export function isShort(needed: number | null | undefined, free: number | null | undefined): boolean {
  return needed != null && needed > 0 && free != null && free >= 0 && needed > free;
}

/** The name a kind of place is shown under. */
export function kindLabel(kind: InstallLocation['kind'] | 'picked', t: TFunction): string {
  switch (kind) {
    case 'default': return t('gameOps:location.kinds.default');
    case 'library': return t('gameOps:location.kinds.library');
    case 'recent': return t('gameOps:location.kinds.recent');
    default: return t('gameOps:location.kinds.picked');
  }
}

/** A path with the home folder shortened, for the places the dialog lists. */
export function shortPath(path: string, home?: string): string {
  if (home && (path === home || path.startsWith(home + '/'))) return '~' + path.slice(home.length);
  return path;
}

/** The place a value ('' = the store's own folder) stands for, among those listed. */
export function locationFor(locations: InstallLocation[], folder: string): InstallLocation | undefined {
  return folder === '' ? locations.find((l) => l.kind === 'default') : locations.find((l) => l.folder === folder);
}
