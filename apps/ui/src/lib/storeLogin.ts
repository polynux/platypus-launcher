import type { NotifyAction } from './notify';
import { coreErrorDataOf } from './coreError';

/** Whether the refusal is the install gate asking for a store login: the
 *  store can't list the game's terms until the user logs in
 *  (protocol 1.11, "Terms"). */
export function isTermsLoginError(err: unknown): boolean {
  return coreErrorDataOf(err)?.code === 'terms.needsLogin';
}

let opener: ((source: string) => void) | null = null;

/** Registers how a refused install opens a store's login flow (App: the
 *  Plugins page login dialog for that store). Follows setTermsHost. */
export function setStoreLoginOpener(fn: ((source: string) => void) | null): () => void {
  opener = fn;
  return () => { if (opener === fn) opener = null; };
}

/** Opens the login flow for the store, or false when nothing registered it. */
export function openStoreLogin(source: string): boolean {
  if (!opener) return false;
  opener(source);
  return true;
}

/** The toast action for a terms.needsLogin refusal: "Log in" opens the
 *  store's existing login flow. Null for anything else, which keeps the
 *  plain error toast. */
export function loginActionFor(err: unknown, source: string, label: string): NotifyAction | null {
  if (!isTermsLoginError(err)) return null;
  return { label, onClick: () => { openStoreLogin(source); } };
}
