import { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { Game } from '../types';
import { appIdSourceLabel } from '../lib/compat';
import { useCompat } from '../lib/useCompat';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel } from './ui';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The Steam AppID the ratings use for a game, set by the user. It saves on its
 *  own, apart from the form's Save: a Steam AppID is the only thing it changes.
 *  Platypus never looks a game up by its title; the links are for checking by hand. */
export function SteamAppIdField({ game }: { game: Game }) {
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
      setError('A Steam AppID is a number, like 620.');
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

  return (
    <div className="border-b border-white/[0.07] pb-5">
      <MonoLabel className="mb-2">steam appid for ratings</MonoLabel>
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          save();
        }}
      >
        <Input
          aria-label="Steam AppID for ratings"
          aria-invalid={!!error}
          inputMode="numeric"
          value={draft}
          placeholder={effective ? effective : 'e.g. 620'}
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
          Save
        </Button>
      </form>
      <p className="mt-2 text-[12px] leading-relaxed text-white/70">
        {effective
          ? `Currently ${effective} (${appIdSourceLabel(compat?.steam_app_id_source)}).`
          : 'No Steam AppID for this game, so there is no ProtonDB rating.'}{' '}
        Check it on{' '}
        <a href={href} target="_blank" rel="noreferrer" className="text-amber-200/80 hover:text-amber-200">
          {effective ? 'its Steam page' : 'Steam search'} ↗
        </a>
        .
      </p>
      {error && <ErrorNotice className="mt-2" message={error} fallback="Platypus couldn't check that Steam app ID." />}
    </div>
  );
}

