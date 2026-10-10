import React, { useState } from 'react';
import { Effect } from 'effect';
import { ExternalLink } from 'lucide-react';
import type { Game, GameTerm } from '../types';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { LauncherApi } from '../effect/runtime';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { notifyTermsAccepted, termURL } from '../lib/terms';
import { cx } from './ui';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface TermsDialogProps {
  game: Game;
  /** The terms the user hasn't accepted yet (their current versions). */
  terms: GameTerm[];
  /** Every term was accepted and recorded: the install or update goes on. */
  onAccepted: () => void;
  /** The user cancelled: nothing is installed. */
  onCancel: () => void;
}

/** The store's terms before an install or an update: each one with a link to
 *  read it, and "I accept" or Cancel. Platypus records the acceptance; it never
 *  accepts for the user, and the dialog offers nothing that would. */
export function TermsDialog({ game, terms, onAccepted, onCancel }: TermsDialogProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const accept = async () => {
    setBusy(true);
    setError(null);
    try {
      await Effect.runPromise(LauncherApi.acceptGameTerms(game, terms.map((t) => ({ id: t.id, version: t.version }))));
      notifyTermsAccepted();
      onAccepted();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal label={`Terms for ${game.title}`} onClose={onCancel} size="md" dismissable={!busy}>
      <h2 className="text-[20px] font-extrabold tracking-tight">Terms for {game.title}</h2>
      <p className="mt-1 text-[12.5px] text-white/70">
        Read these before {game.title} is installed. Platypus doesn't accept them for you.
      </p>

      <ul className="mt-5 space-y-2" aria-label="Terms to accept">
        {terms.map((t) => {
          const link = termURL(t);
          return (
            <li key={`${t.id}@${t.version}`} className="flex items-center justify-between gap-3 rounded-2xl bg-white/[0.04] px-4 py-3">
              <span className="min-w-0">
                <span className="block text-[13px] text-white/90">{t.title}</span>
                <span className="block font-mono text-hint text-white/55">version {t.version}</span>
              </span>
              {link && (
                <button
                  type="button"
                  onClick={() => BrowserOpenURL(link)}
                  className={cx('inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 font-mono text-hint text-sky-200 hover:bg-white/[0.06]')}
                >
                  <ExternalLink className="h-3.5 w-3.5" /> Read
                </button>
              )}
            </li>
          );
        })}
      </ul>

      {error && (
        <ErrorNotice className="mt-4" message={error} fallback="Platypus couldn't read the terms just now." />
      )}

      <div className="mt-6 flex justify-end gap-2.5">
        <Button variant="neutral" size="sm" disabled={busy} onClick={onCancel}>Cancel</Button>
        <Button variant="primary" size="sm" disabled={busy} onClick={() => void accept()}>I accept</Button>
      </div>
    </Modal>
  );
}
