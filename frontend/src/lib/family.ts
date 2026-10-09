import type { FamilyMember, Game } from '../types';

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
