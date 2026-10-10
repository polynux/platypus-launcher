import type { InstallLocation } from '../types';
import { formatBytes } from './games';

// Install location (the install dialog): what the choices mean for the user.
// Core checks the folder again when the install is queued; these keep the
// dialog's view of the same facts.

/** The room at a place as the dialog says it ("120 GB free"); empty when unreadable. */
export function freeLabel(free: number | undefined): string {
  return free != null && free >= 0 ? `${formatBytes(free) || '0 B'} free` : '';
}

/** Whether `needed` bytes don't fit in `free`. Unknown on either side fits: the
 *  install checks the room again and nothing is blocked on a guess. */
export function isShort(needed: number | null | undefined, free: number | null | undefined): boolean {
  return needed != null && needed > 0 && free != null && free >= 0 && needed > free;
}

/** The name a kind of place is shown under. */
export function kindLabel(kind: InstallLocation['kind'] | 'picked'): string {
  return { default: 'Default', library: 'Game folder', recent: 'Recent', picked: 'Chosen' }[kind];
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
