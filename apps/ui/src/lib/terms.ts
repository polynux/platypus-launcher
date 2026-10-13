import type { GameTerm } from '../types';
import { webURL } from './external';
import { formatDate } from './format';

/** The terms the user still has to accept: a version they haven't accepted
 *  counts, so a bumped version is asked again. */
export function pendingTerms(terms: GameTerm[]): GameTerm[] {
  return terms.filter((t) => !t.accepted_at);
}

/** When the user accepted a term, as a date for the Legal section ("" when not accepted). */
export function acceptedLabel(term: Pick<GameTerm, 'accepted_at'>): string {
  if (!term.accepted_at) return '';
  return formatDate(term.accepted_at);
}

/** The term's link, when it is a web address the browser may open ('' otherwise). */
export function termURL(term: Pick<GameTerm, 'url'>): string {
  return webURL(term.url);
}

const acceptedListeners = new Set<() => void>();

/** Calls fn after the user accepts a game's terms, so a view that shows the
 *  acceptance (the Legal section) reads it again. Returns the unsubscribe. */
export function onTermsAccepted(fn: () => void): () => void {
  acceptedListeners.add(fn);
  return () => { acceptedListeners.delete(fn); };
}

/** Tells the views that show acceptance that it changed. */
export function notifyTermsAccepted(): void {
  acceptedListeners.forEach((fn) => fn());
}
