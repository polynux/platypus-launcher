// i18n: converted
import { useEffect, useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { Game } from '../types';
import { useCompat } from '../lib/useCompat';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel } from './ui';
import { ErrorNotice } from './ErrorNotice';
import { ExternalLink } from './ExternalLink';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The Steam AppID the ratings use for a game, set by the user. It saves on its
 *  own, apart from the form's Save: a Steam AppID is the only thing it changes.
 *  Platypus never looks a game up by its title; the links are for checking by hand. */
export function SteamAppIdField({ game }: { game: Game }) {
  const { t } = useTranslation('game');
  const { compat, update } = useCompat(game);
  const userSet = compat?.steam_app_id_source === 'user' ? compat.steam_app_id ?? '' : '';
  const effective = compat?.steam_app_id ?? '';
  const [draft, setDraft] = useState(userSet);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => setDraft(userSet), [userSet]);

  const save = () => {
    const next = draft.trim();
    if (next === userSet) return;
    if (next !== '' && !/^\d+$/.test(next)) {
      setError(t('game:steamAppId.numberError'));
      return;
    }
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setGameSteamAppID(game, next))
      .then((c) => update(c))
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  const href = effective
    ? `https://store.steampowered.com/app/${effective}`
    : `https://store.steampowered.com/search/?term=${encodeURIComponent(game.title)}`;
  const source =
    compat?.steam_app_id_source === 'user' ? t('game:appIdSource.user')
    : compat?.steam_app_id_source === 'plugin' ? t('game:appIdSource.plugin')
    : compat?.steam_app_id_source === 'steam' ? t('game:appIdSource.steam')
    : t('game:appIdSource.none');

  return (
    <div className="border-b border-white/[0.07] pb-5">
      <MonoLabel className="mb-2">{t('game:steamAppId.heading')}</MonoLabel>
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <Input
          aria-label={t('game:steamAppId.aria')}
          aria-invalid={!!error}
          inputMode="numeric"
          value={draft}
          placeholder={effective ? effective : t('game:steamAppId.placeholder')}
          onChange={(e) => {
            setDraft(e.target.value);
            setError(null);
          }}
          onBlur={save}
          disabled={busy}
          spellCheck={false}
          className="w-32 font-mono"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={busy || draft.trim() === userSet}>
          {t('common:save')}
        </Button>
      </form>
      <p className="mt-2 text-[12px] leading-relaxed text-white/70">
        {effective ? (
          <Trans
            i18nKey="game:steamAppId.current"
            values={{ id: effective, source }}
            components={{ a: <ExternalLink href={href} className="text-amber-200/80 hover:text-amber-200" /> }}
          />
        ) : (
          <Trans
            i18nKey="game:steamAppId.none"
            components={{ a: <ExternalLink href={href} className="text-amber-200/80 hover:text-amber-200" /> }}
          />
        )}
      </p>
      {error && <ErrorNotice className="mt-2" message={error} fallback={t('game:steamAppId.failed')} />}
    </div>
  );
}
