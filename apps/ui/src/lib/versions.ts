import type { LedgerEntry, VersionBuild, VersionBuildSource, VersionChannel, VersionInfo, VersionPin, VersionTarget } from '../types';

/** What the user picked in the version dialog. */
export type VersionChoice =
  | { kind: 'channel'; id: string }
  | { kind: 'build'; id: string }
  | { kind: 'manual'; text: string };

/** Shown before installing a version other than the one the user has. A
 *  downgrade can corrupt saves, and online play may refuse a mismatched build;
 *  Platypus backs nothing up itself. */
export const VERSION_WARNING =
  'Cloud saves and online play may not match this version. An older build can corrupt saves made by a newer one, and Platypus backs nothing up itself.';

export const SOURCE_LABEL: Record<VersionBuildSource, string> = {
  store: 'from the store',
  archive: 'from your install history',
  manual: 'typed by you',
};

export const sourceLabel = (s: string | undefined): string =>
  s === 'archive' ? SOURCE_LABEL.archive : s === 'manual' ? SOURCE_LABEL.manual : SOURCE_LABEL.store;

/** The target the backend queues. Typed text rides in manual.input, as typed. */
export function targetOf(choice: VersionChoice): VersionTarget {
  switch (choice.kind) {
    case 'channel': return { channel: choice.id };
    case 'build': return { build: choice.id };
    case 'manual': return { manual: { input: choice.text.trim() } };
  }
}

/** A choice can be pinned unless it is typed text: the pin names something the
 *  plugin lists (or listed from the history). */
export function pinOf(choice: VersionChoice): VersionPin | null {
  return choice.kind === 'manual' ? null : { kind: choice.kind, id: choice.id };
}

export const samePin = (a: VersionPin | null | undefined, b: VersionPin | null | undefined): boolean =>
  (a == null && b == null) || (a != null && b != null && a.kind === b.kind && a.id === b.id);

export function choiceIsEmpty(choice: VersionChoice | null): boolean {
  return choice == null || (choice.kind === 'manual' && choice.text.trim() === '');
}

/** Whether the installed game already is what the choice names. */
export function isCurrent(info: VersionInfo, choice: VersionChoice | null): boolean {
  if (!choice || choice.kind === 'manual') return false;
  if (choice.kind === 'channel') return info.channels.some((c) => c.id === choice.id && c.current);
  return info.builds.some((b) => b.id === choice.id && b.current);
}

export function pinLabel(pin: VersionPin, info?: Pick<VersionInfo, 'channels' | 'builds'>): string {
  if (pin.kind === 'channel') {
    const c = info?.channels.find((x) => x.id === pin.id);
    return `channel ${c?.name ?? pin.id}`;
  }
  const b = info?.builds.find((x) => x.id === pin.id);
  return `build ${b?.name ?? pin.id}`;
}

export function buildTitle(b: VersionBuild): string {
  return b.name && b.name !== b.id ? b.name : b.id;
}

export function channelLocked(c: VersionChannel): boolean {
  return c.requires === 'password' && !c.unlocked;
}

/** The latest install in the history, or undefined. */
export function lastInstall(ledger: LedgerEntry[]): LedgerEntry | undefined {
  return ledger.length ? ledger[ledger.length - 1] : undefined;
}

export const shortDate = (iso?: string): string => (iso ? iso.slice(0, 10) : '');
