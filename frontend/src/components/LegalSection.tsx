import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { Game, GameTerm, StorePlugin } from '../types';
import { CapabilitySurface } from '../lib/capabilities';
import { LauncherApi } from '../effect/runtime';
import { acceptedLabel, onTermsAccepted, termURL } from '../lib/terms';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { MonoLabel } from './ui';

/** The store's terms of a game and when the user accepted each (protocol 1.11,
 *  the `terms` capability). Read only: acceptance happens at install. Renders
 *  nothing when the plugin doesn't offer terms or the game lists none. */
export function LegalSection({ game, plugin }: { game: Game; plugin: StorePlugin | undefined }) {
  return (
    <CapabilitySurface plugin={plugin} capability="terms" game={game}>
      <LegalList game={game} />
    </CapabilitySurface>
  );
}

function LegalList({ game }: { game: Game }) {
  const [terms, setTerms] = useState<GameTerm[] | null>(null);
  const [error, setError] = useState('');
  // Bumped when the user accepts terms elsewhere (the install dialog), so the
  // list is read again while this view stays open.
  const [accepted, setAccepted] = useState(0);
  useEffect(() => onTermsAccepted(() => setAccepted((n) => n + 1)), []);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getGameTerms(game)).then(
      (t) => { if (live) setTerms(t); },
      (e: Error) => { if (live) setError(e.message || 'Could not read the terms.'); },
    );
    return () => { live = false; };
  }, [game, accepted]);

  if (error) return <p role="alert" className="font-mono text-[11.5px] text-red-300">{error}</p>;
  if (!terms || terms.length === 0) return null;
  return (
    <section className="mt-6" aria-label="Legal">
      <MonoLabel className="mb-2">legal</MonoLabel>
      <ul className="space-y-2">
        {terms.map((t) => {
          const link = termURL(t);
          return (
            <li key={`${t.id}@${t.version}`} className="flex items-center justify-between gap-3 font-mono text-[11.5px]">
              <span className="min-w-0 text-white/80">
                {t.title} <span className="text-white/40">v{t.version}</span>
              </span>
              <span className="flex shrink-0 items-center gap-3">
                {t.accepted_at ? (
                  <span className="text-white/55">accepted {acceptedLabel(t)}</span>
                ) : (
                  <span className="text-amber-200/80">not accepted yet</span>
                )}
                {link && (
                  <button type="button" className="text-sky-200 hover:underline" onClick={() => BrowserOpenURL(link)}>read</button>
                )}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
