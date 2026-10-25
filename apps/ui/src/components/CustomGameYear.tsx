// A sideloaded game's release year (issue #35), shared by the tab dialog and
// the atlas drawer. Empty clears it back to unknown.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import type { Game } from '../types';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { useVariant } from '../lib/theme';
import { parseReleaseYear } from './AddGameModal';
import { LauncherApi } from '../effect/runtime';
import { ErrorNotice } from './ErrorNotice';

export function CustomGameYear({ game, onChanged }: { game: Game; onChanged: () => void }) {
  const { t } = useTranslation('gameSettings');
  const plainKicker = useVariant('label.style') === 'plain';
  const [raw, setRaw] = useState(game.release_year ? String(game.release_year) : '');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  useEffect(() => { setRaw(game.release_year ? String(game.release_year) : ''); }, [game.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const parsed = parseReleaseYear(raw);
  const saved = game.release_year ?? 0;
  const dirty = !parsed.error && (parsed.year ?? 0) !== saved;
  const save = () => {
    if (parsed.error || !dirty || saving) return;
    setSaving(true);
    setError(null);
    Effect.runPromise(LauncherApi.setCustomGameYear(game.id, parsed.year ?? 0)).then(
      () => { setSaving(false); onChanged(); },
      (e) => { setSaving(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };
  return (
    <div className="mb-6">
      {plainKicker ? (
        <div className="mb-1.5 text-[12px] font-bold text-text-faint">{t('gameSettings:year.kicker')}</div>
      ) : (
        <span className="mb-1.5 block font-mono text-[12px] uppercase tracking-[0.14em] text-white/55">{t('gameSettings:year.kicker')}</span>
      )}
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); save(); }}
      >
        <Input
          aria-label={t('gameSettings:year.aria')}
          value={raw}
          onChange={(e) => { setRaw(e.target.value); setError(null); }}
          placeholder={t('gameSettings:year.placeholder')}
          inputMode="numeric"
          className="w-32"
        />
        <Button type="submit" variant="primary" size="sm" disabled={!dirty || saving}>{t('gameSettings:year.save')}</Button>
        {saved > 0 && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={saving}
            onClick={() => {
              setSaving(true);
              setError(null);
              Effect.runPromise(LauncherApi.setCustomGameYear(game.id, 0)).then(
                () => { setRaw(''); setSaving(false); onChanged(); },
                (e) => { setSaving(false); setError(e instanceof Error ? e.message : String(e)); },
              );
            }}
          >
            {t('gameSettings:year.clear')}
          </Button>
        )}
      </form>
      {parsed.error
        ? <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{parsed.error}</p>
        : error && <ErrorNotice message={error} fallback={t('gameSettings:year.saveError')} />}
    </div>
  );
}
