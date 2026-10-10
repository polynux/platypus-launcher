import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { FolderOpen } from 'lucide-react';
import { LauncherApi } from '../effect/runtime';
import type { LauncherSettings, StorePlugin } from '../types';
import { NumberField } from '@/components/ds/number-field';
import { Switch } from '@/components/ds/switch';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel, Panel } from './ui';
import { CloudSavesDefaults } from './CloudSavesSetting';
import { GameFoldersSettings } from './GameFoldersSettings';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface LauncherSettingsViewProps {
  onNotify: (msg: string, kind?: 'ok' | 'err') => void;
  /** The plugins list: the cloud-saves defaults come from the stores that offer them. */
  plugins: StorePlugin[];
}

export function LauncherSettingsView({ onNotify, plugins }: LauncherSettingsViewProps) {
  const [saved, setSaved] = useState<LauncherSettings | null>(null);
  const [draft, setDraft] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [cleanupDays, setCleanupDays] = useState<number | ''>('');
  const [cleanupError, setCleanupError] = useState<string | null>(null);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setDraft(s.games_folder); setCleanupDays(s.cancel_cleanup_days); })
      .catch((e) => setError(errMsg(e)));
  }, []);

  const save = (path: string) => {
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.setGamesFolder(path))
      .then((s) => {
        setSaved(s);
        setDraft(s.games_folder);
        onNotify('Games folder saved');
      })
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  const browse = () => {
    Effect.runPromise(LauncherApi.chooseGamesFolder())
      .then((path) => { if (path) save(path); })
      .catch((e) => onNotify(errMsg(e), 'err'));
  };

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

  const dirty = saved != null && draft.trim() !== saved.games_folder;
  const isDefault = saved != null && saved.games_folder === saved.default_games_folder;

  return (
    <div className="p-7">
      <header className="mb-6">
        <h1 className="text-[23px] font-extrabold tracking-tight">Settings</h1>
        <p className="mt-0.5 text-[12.5px] text-white/50">Hydra-wide settings, saved in ~/.local/share/hydra/settings.json.</p>
      </header>

      <Panel as="section" className="max-w-2xl">
        <div className="mb-1 flex items-baseline justify-between gap-4">
          <h2 className="text-[14px] font-bold">Games folder</h2>
          {saved && <MonoLabel>{isDefault ? 'default' : 'custom'}</MonoLabel>}
        </div>
        <p className="mb-4 text-[12.5px] leading-relaxed text-white/55">
          Store plugins install games here, each in its own subfolder, for example{' '}
          <span className="font-mono text-[11.5px] text-white/75">{(saved?.games_folder ?? '~/Games') + '/epic'}</span>. Steam keeps using its own
          library folders.
        </p>

        <form
          className="flex flex-wrap gap-2"
          onSubmit={(e) => { e.preventDefault(); if (dirty) save(draft); }}
        >
          <label className="min-w-64 flex-1">
            <span className="sr-only">Games folder path</span>
            <Input
              value={draft}
              onChange={(e) => { setDraft(e.target.value); setError(null); }}
              placeholder={saved?.default_games_folder ?? '~/Games'}
              spellCheck={false}
              disabled={!saved}
              aria-invalid={error != null}
            />
          </label>
          <Button variant="neutral" onClick={browse} disabled={!saved || busy}>
            <FolderOpen className="size-3.5" /> Browse
          </Button>
          <Button type="submit" variant="primary" disabled={!dirty || busy}>Save</Button>
        </form>

        <div className="mt-2.5 flex min-h-4 items-center justify-between gap-4">
          {error ? <p role="alert" className="font-mono text-[11.5px] text-red-300">{error}</p> : <span />}
          {saved && !isDefault && (
            <Button variant="link" size="none" onClick={() => save('')} disabled={busy} className="flex-none text-[11.5px]">
              reset to {saved.default_games_folder.replace(/^\/home\/[^/]+/, '~')}
            </Button>
          )}
        </div>
      </Panel>

      <GameFoldersSettings onNotify={onNotify} />

      <Panel as="section" className="mt-4 max-w-2xl">
        <h2 className="text-[14px] font-bold">Canceled downloads</h2>
        <p className="mb-4 text-[12.5px] leading-relaxed text-white/55">
          How long Hydra keeps the partially downloaded data of a canceled download before deleting it. The download can still be resumed during that
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
          {cleanupError ? <p role="alert" className="font-mono text-[11.5px] text-red-300">{cleanupError}</p> : <span />}
        </div>
      </Panel>

      <RatingsSetting onNotify={onNotify} />

      <CloudSavesDefaults plugins={plugins} />
    </div>
  );
}

/** "Fetch ratings online" and the credits for the data Hydra shows. Off, Hydra
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
        onNotify(next ? 'Ratings will be fetched online' : 'Ratings stay offline');
      })
      .catch((e) => onNotify(errMsg(e), 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <Panel as="section" className="mt-4 max-w-2xl">
      <div className="flex items-start justify-between gap-6">
        <div>
          <h2 className="text-[14px] font-bold">Ratings and anti-cheat data</h2>
          <p className="mt-1 text-[12.5px] leading-relaxed text-white/55">
            Fetch ProtonDB ratings and the anti-cheat list online. Off, Hydra shows what it saved before and asks nothing new.
          </p>
        </div>
        <Switch aria-label="Fetch ratings online" checked={on ?? true} disabled={on === null || busy} onCheckedChange={toggle} />
      </div>
      <div className="mt-4 space-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-[11px] leading-relaxed text-white/45">
        <p>Compatibility ratings: ProtonDB community reports (protondb.com).</p>
        <p>Anti-cheat list: AreWeAntiCheatYet, MIT licence (github.com/AreWeAntiCheatYet/AreWeAntiCheatYet).</p>
      </div>
    </Panel>
  );
}
