import type { CopyComparison, CopyDiff, FamilyMember, Game, GameCopy } from '../types';
import i18n from './i18n';

/** The role as the Families section names it. A role Platypus doesn't know is
 *  shown as none. */
export function roleLabel(role: string): string {
  if (role === 'adult') return i18n.t('family:role.adult');
  if (role === 'child') return i18n.t('family:role.child');
  return '';
}

/** The first letter of a member's name, for an avatar that failed to load. */
export function memberInitial(member: Pick<FamilyMember, 'name' | 'id'>): string {
  return (member.name || member.id).trim().charAt(0).toUpperCase() || '?';
}

/** The games borrowed from a family member: those the library marks as shared. */
export function sharedWithMe(games: Game[]): Game[] {
  return games.filter((g) => g.shared_by != null);
}

/** A warning's countdown as the toast shows it: "42 s left", or nothing when
 *  the plugin gave no time. */
export function countdownLabel(secondsLeft: number): string {
  return secondsLeft > 0 ? i18n.t('family:countdown', { count: secondsLeft }) : '';
}

/** The lender whose copy a borrowed game is played from: the one the user
 *  picked, else the store's default. Null for the user's own games. */
export function playedCopy(game: Pick<Game, 'shared_by' | 'preferred_copy'>): string | null {
  return game.shared_by ? game.preferred_copy || game.shared_by.id : null;
}

/** The copies the picker offers. Only a borrowed game has any: the user's own
 *  copy hides the picker. */
export function pickableCopies(game: Pick<Game, 'shared_by' | 'copies'>): GameCopy[] {
  return game.shared_by ? game.copies ?? [] : [];
}

/** What keeps a copy from being picked or played, for its label: '' when it is
 *  ready. An unavailable copy can't be picked at all. */
export function copyStatus(copy: Pick<GameCopy, 'available' | 'in_use'>): string {
  if (!copy.available) return 'unavailable';
  return copy.in_use ? 'in use' : '';
}

/** The add-ons a copy has, as the picker names them. */
export function dlcLabel(copy: Pick<GameCopy, 'dlc'>): string {
  const names = (copy.dlc ?? []).map((d) => d.name);
  return names.length > 0 ? names.join(', ') : 'no add-ons';
}

/** One line about a copy against the baseline copy, for the picker: what it
 *  has more or fewer of, soundtracks apart ("…has 3 more DLC and 1 more
 *  soundtrack than Ana's."), or that it has the same add-ons. */
export function copyDiffSummary(copy: string, baseline: string, diff: CopyDiff): string {
  const parts = [
    phrase(diff.only_here, diff.only_baseline, 'DLC'),
    phrase(diff.soundtracks_only_here, diff.soundtracks_only_baseline, 'soundtrack'),
  ].filter(Boolean);
  if (parts.length === 0) return `${copy}'s copy has the same add-ons as ${baseline}'s.`;
  return `${copy}'s copy has ${parts.join(' and ')} than ${baseline}'s.`;
}

/** "3 more", "2 fewer", "1 more and 2 fewer", or '' when neither side has any. */
function phrase(more: number, fewer: number, noun: 'DLC' | 'soundtrack'): string {
  const word = noun === 'DLC' ? 'DLC' : more + fewer === 1 ? 'soundtrack' : 'soundtracks';
  if (more > 0 && fewer > 0) return `${more} more and ${fewer} fewer ${word}`;
  if (more > 0) return `${more} more ${word}`;
  if (fewer > 0) return `${fewer} fewer ${word}`;
  return '';
}

/** The copy's diff against the baseline, by owner id (undefined for the baseline). */
export function diffFor(compare: CopyComparison | null | undefined, ownerID: string): CopyDiff | undefined {
  return compare?.diffs.find((d) => d.owner_id === ownerID);
}

/** One row of the comparison table: an add-on and which copies have it. */
export interface CompareRow {
  id: string;
  name: string;
  kind: 'dlc' | 'soundtrack' | 'other';
  /** The copies (by owner id) that have the add-on. */
  owners: string[];
}

/** The comparison rows: every add-on any copy has, in first-seen order. With
 *  `differencesOnly`, the add-ons all copies share are left out; with
 *  `soundtracks` false, soundtracks are left out. */
export function compareRows(copies: GameCopy[], opts: { differencesOnly: boolean; soundtracks: boolean }): CompareRow[] {
  const rows = new Map<string, CompareRow>();
  for (const copy of copies) {
    for (const d of copy.dlc ?? []) {
      const kind = d.kind === 'soundtrack' || d.kind === 'other' ? d.kind : 'dlc';
      const row = rows.get(d.id) ?? { id: d.id, name: d.name, kind, owners: [] };
      row.owners.push(copy.owner_id);
      rows.set(d.id, row);
    }
  }
  return [...rows.values()].filter((row) => {
    if (!opts.soundtracks && row.kind === 'soundtrack') return false;
    return !opts.differencesOnly || row.owners.length < copies.length;
  });
}
