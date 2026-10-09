import type { FamilyMember, Game, GameCopy } from '../types';

/** The role as the Families section names it. A role Hydra doesn't know is
 *  shown as none. */
export function roleLabel(role: string): string {
  return role === 'adult' || role === 'child' ? role : '';
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
  return secondsLeft > 0 ? `${secondsLeft} s left` : '';
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
