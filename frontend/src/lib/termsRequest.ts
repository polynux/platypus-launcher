import { Effect } from 'effect';
import type { Game, GameTerm } from '../types';
import { LauncherApi } from '../effect/runtime';
import { pendingTerms } from './terms';

/** A request for the user to accept a game's terms, answered by the host that
 *  shows the dialog (TermsHost). */
export interface TermsRequest {
  game: Game;
  terms: GameTerm[];
  /** Called once: true when every pending term was accepted, false on cancel. */
  done: (accepted: boolean) => void;
}

let host: ((r: TermsRequest) => void) | null = null;

/** The TermsHost registers itself; the request goes to it. Returns the unregister. */
export function setTermsHost(fn: ((r: TermsRequest) => void) | null): () => void {
  host = fn;
  return () => { if (host === fn) host = null; };
}

/** Asks the user to accept the terms of a game that they haven't accepted yet,
 *  before an install or an update. Resolves true straight away when nothing is
 *  pending, and when the user accepts; false when they cancel or nothing can
 *  show the dialog. Never accepts on the user's behalf. Rejects when the store
 *  can't list the terms. */
export async function ensureTerms(game: Game): Promise<boolean> {
  const pending = pendingTerms(await Effect.runPromise(LauncherApi.getGameTerms(game)));
  if (pending.length === 0) return true;
  if (!host) return false;
  return new Promise<boolean>((resolve) => host?.({ game, terms: pending, done: resolve }));
}
