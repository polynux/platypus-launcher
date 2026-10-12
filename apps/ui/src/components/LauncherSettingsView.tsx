import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { LauncherSettings, StorePlugin } from '../types';
import { NumberField } from '@/components/ds/number-field';
import { Input } from '@/components/ds/input';
import { Switch } from '@/components/ds/switch';
import { Button } from '@/components/ds/button';
import { CloudSavesDefaults } from './CloudSavesSetting';
import { GameFoldersSettings } from './GameFoldersSettings';
import { ErrorNotice } from './ErrorNotice';

/** A settings section: a hairline and air, not a card (no card inside the page). */
const SECTION = 'max-w-2xl border-t border-white/[0.07] py-6';
const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface LauncherSettingsViewProps {
  onNotify: (msg: string, kind?: 'ok' | 'err') => void;
  /** The plugins list: the cloud-saves defaults come from the stores that offer them. */
  plugins: StorePlugin[];
}

export function LauncherSettingsView({ onNotify, plugins }: LauncherSettingsViewProps) {
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [cleanupDays, setCleanupDays] = useState<number | ''>('');
  const [cleanupError, setCleanupError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setCleanupDays(s.cancel_cleanup_days); })
      .catch((e) => setError(errMsg(e)));
  }, []);

  const saveCleanupDays = (days: number) => {
    setBusy(true);
    setCleanupError(null);
    Effect.runPromise(LauncherApi.setCancelCleanupDays(days))
      .then((s) => {
        setSaved(s);
        setCleanupDays(s.cancel_cleanup_days);
        onNotify('Download cleanup setting saved');
      })
      .catch((e) => setCleanupError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  const cleanupDirty = saved != null && typeof cleanupDays === 'number' && cleanupDays !== saved.cancel_cleanup_days;

  return (
    <div className="p-7">
      <header className="mb-2">
        <h1 className="text-[23px] font-extrabold tracking-tight">Settings</h1>
        <p className="mt-0.5 max-w-[75ch] text-[12.5px] text-white/70">Platypus-wide settings, saved in ~/.local/share/platypus/settings.json.</p>
      </header>
      {error && <ErrorNotice className="mt-2" message={error} fallback="Platypus couldn't load the settings." />}

      <GameFoldersSettings onNotify={onNotify} />

      <PrefixesFolderSetting onNotify={onNotify} />

      <section className={SECTION}>
        <h2 className="text-[14px] font-bold">Canceled downloads</h2>
        <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
          How long Platypus keeps the partially downloaded data of a canceled download before deleting it. The download can still be resumed during that
          time. 0 deletes it right away; a paused download is never touched.
        </p>
        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (cleanupDirty && typeof cleanupDays === 'number') saveCleanupDays(cleanupDays);
          }}
        >
          <label className="flex items-center gap-2">
            <span className="text-[12.5px] text-white/70">Keep for</span>
            <NumberField
              min={0}
              max={365}
              step={1}
              hideSteppers
              value={cleanupDays === '' ? undefined : cleanupDays}
              onChange={(v) => { setCleanupDays(v ?? ''); setCleanupError(null); }}
              disabled={!saved}
              aria-label="Days to keep canceled downloads' data"
              className="w-24"
            />
            <span className="text-[12.5px] text-white/70">days</span>
          </label>
          <Button type="submit" variant="primary" disabled={!cleanupDirty || busy}>Save</Button>
        </form>
        <div className="mt-2.5 flex min-h-4 items-center gap-4">
          {cleanupError ? <ErrorNotice message={cleanupError} fallback="Platypus couldn't clean up those folders." /> : <span />}
        </div>
      </section>

      <RatingsSetting onNotify={onNotify} />

      <CloudSavesDefaults plugins={plugins} />
    </div>
  );
}

/** "Fetch game info online" and the credits for the data Platypus shows. Off, Platypus
 *  keeps what it fetched before and asks the network for nothing new. */
export function RatingsSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => setOn(s.ratings_online ?? true))
      .catch(() => setOn(true));
  }, []);
  const toggle = (next: boolean) => {
    setBusy(true);
    Effect.runPromise(LauncherApi.setRatingsOnline(next))
      .then((s) => {
        setOn(s.ratings_online ?? next);
        onNotify(next ? 'Game info will be fetched online' : 'Game info stays offline');
      })
      .catch((e) => onNotify(errMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <section className={SECTION}>
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">Ratings, protection and anti-cheat data</h2>
          <p className="mt-1 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
            Fetch ProtonDB ratings, PCGamingWiki protection facts and the anti-cheat list online. Off, Platypus shows what it saved before and asks nothing new.
          </p>
        </div>
        <Switch aria-label="Fetch game info online" checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-hint leading-relaxed text-white/55">
        <p>Compatibility ratings: ProtonDB community reports (protondb.com).</p>
        <p>Protection facts: PCGamingWiki contributors, CC BY-NC-SA 3.0 (pcgamingwiki.com). Used for this non-commercial launcher only.</p>
        <p>Anti-cheat list: AreWeAntiCheatYet, MIT licence (github.com/AreWeAntiCheatYet/AreWeAntiCheatYet).</p>
      </div>
    </section>
  );
}

/** Where new Wine prefixes are created (issue #37): one folder per game in
 *  this folder, unless the game has its own prefix. Changing it never moves
 *  or deletes existing prefixes; empty resets it to the default. */
export function PrefixesFolderSetting({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [folder, setFolder] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setFolder(s.prefixes_folder ?? ''); })
      .catch((e) => setError(errMsg(e)));
  }, []);

  const dirty = saved != null && folder.trim() !== (saved.prefixes_folder ?? '');
  const custom = (saved?.prefixes_folder ?? '') !== '';

  const save = (path: string) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setPrefixesFolder(path))
      .then((s) => {
        setSaved(s);
        setFolder(s.prefixes_folder ?? '');
        onNotify(path ? 'Wine prefix folder saved' : 'Wine prefix folder reset to the default');
      })
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  const browse = () => {
    Effect.runPromise(LauncherApi.pickFolder('Choose the default Wine prefix folder', folder.trim() || saved?.prefixes_folder || saved?.default_prefixes_folder || ''))
      .then((picked) => { if (picked) { setFolder(picked); setError(null); } })
      .catch((e) => setError(errMsg(e)));
  };

  return (
    <section className={SECTION}>
      <h2 className="text-[14px] font-bold">Wine prefix folder</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
        Where new Wine prefixes are created, one folder per game. A game's own prefix setting still wins.
        Changing this folder does not move or delete existing prefixes. Games that already have a prefix keep it:
        Platypus pins it in their settings.
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (dirty) save(folder.trim());
        }}
      >
        <Input
          aria-label="Default Wine prefix folder"
          value={folder}
          onChange={(e) => { setFolder(e.target.value); setError(null); }}
          placeholder={saved?.default_prefixes_folder || 'The default prefix folder'}
          spellCheck={false}
          disabled={!saved}
          className="min-w-0 max-w-xl flex-1"
        />
        <Button type="button" variant="neutral" onClick={browse} disabled={!saved || busy}>
          Browse…
        </Button>
        <Button type="submit" variant="primary" disabled={!dirty || busy}>Save</Button>
        {custom && (
          <Button type="button" variant="ghost" onClick={() => save('')} disabled={busy}>
            Reset to default
          </Button>
        )}
      </form>
      <div className="mt-2.5 flex min-h-4 flex-wrap items-center gap-4">
        {error ? <ErrorNotice message={error} fallback="Platypus couldn't change the Wine prefix folder." /> : <span />}
        {saved && (
          <span className="font-mono text-hint text-white/55">
            Prefixes are created in {folder.trim() || saved.prefixes_folder || saved.default_prefixes_folder || 'the default folder'}.
          </span>
        )}
      </div>
    </section>
  );
}
