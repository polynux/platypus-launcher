import type { GameTerm } from '../types';

/** The terms the user still has to accept: a version they haven't accepted
 *  counts, so a bumped version is asked again. */
export function pendingTerms(terms: GameTerm[]): GameTerm[] {
  return terms.filter((t) => !t.accepted_at);
}

/** When the user accepted a term, as a date for the Legal section ("" when not accepted). */
export function acceptedLabel(term: Pick<GameTerm, 'accepted_at'>): string {
  if (!term.accepted_at) return '';
  return new Date(term.accepted_at * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

/** The term's link, when it is a web address the browser may open ('' otherwise). */
export function termURL(term: Pick<GameTerm, 'url'>): string {
  try {
    const u = new URL(term.url ?? '');
    return u.protocol === 'https:' || u.protocol === 'http:' ? u.toString() : '';
  } catch {
    return '';
  }
}
