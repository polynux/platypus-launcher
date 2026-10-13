// i18n: converted
import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import type { Game, GameTerm, StorePlugin } from '../types';
import { CapabilitySurface } from '../lib/capabilities';
import { LauncherApi } from '../effect/runtime';
import { acceptedLabel, onTermsAccepted, termURL } from '../lib/terms';
import { openExternal } from '../lib/external';
import { MonoLabel } from './ui';
import { ErrorNotice } from './ErrorNotice';

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
  const { t } = useTranslation('common');
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

  if (error) return <ErrorNotice message={error} fallback={t('common:legal.error')} />;
  if (!terms || terms.length === 0) return null;
  return (
    <section className="mt-6" aria-label={t('common:legal.label')}>
      <MonoLabel className="mb-2">{t('common:legal.heading')}</MonoLabel>
      <ul className="space-y-2">
        {terms.map((term) => {
          const link = termURL(term);
          return (
            <li key={`${term.id}@${term.version}`} className="flex items-center justify-between gap-3 font-mono text-hint">
              <span className="min-w-0 text-white/80">
                {term.title} <span className="text-white/55">{t('common:legal.version', { version: term.version })}</span>
              </span>
              <span className="flex shrink-0 items-center gap-3">
                {term.accepted_at ? (
                  <span className="text-white/55">{t('common:legal.accepted', { date: acceptedLabel(term) })}</span>
                ) : (
                  <span className="text-amber-200/80">{t('common:legal.pending')}</span>
                )}
                {link && (
                  <button type="button" className="text-sky-200 hover:underline" onClick={() => openExternal(link)}>{t('common:legal.read')}</button>
                )}
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
