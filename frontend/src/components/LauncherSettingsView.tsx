import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { FolderOpen } from 'lucide-react';
import { LauncherApi } from '../effect/runtime';
import type { LauncherSettings } from '../types';
import { MonoLabel, Panel, btnNeutral, btnPrimary, cx } from './ui';

const inputClass =
  'w-full min-w-0 rounded-xl bg-white/[0.06] px-3.5 py-2.5 font-mono text-[11.5px] text-white placeholder-white/30 outline-none ring-amber-300/50 focus:ring-1';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface LauncherSettingsViewProps {
  onNotify: (msg: string, kind?: 'ok' | 'err') => void;
}

export function LauncherSettingsView({ onNotify }: LauncherSettingsViewProps) {
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
            <input
              value={draft}
              onChange={(e) => { setDraft(e.target.value); setError(null); }}
              placeholder={saved?.default_games_folder ?? '~/Games'}
              spellCheck={false}
              disabled={!saved}
              aria-invalid={error != null}
              className={cx(inputClass, error && 'ring-1 ring-red-400/60')}
            />
          </label>
          <button type="button" onClick={browse} disabled={!saved || busy} className={cx(btnNeutral, 'flex items-center gap-2 px-4 py-2.5 text-[12.5px] disabled:opacity-60')}>
            <FolderOpen className="h-3.5 w-3.5" /> Browse
          </button>
          <button type="submit" disabled={!dirty || busy} className={cx(btnPrimary, 'px-4 py-2.5 text-[12.5px] disabled:cursor-default')}>
            Save
          </button>
        </form>

        <div className="mt-2.5 flex min-h-4 items-center justify-between gap-4">
          {error ? <p role="alert" className="font-mono text-[11.5px] text-red-300">{error}</p> : <span />}
          {saved && !isDefault && (
            <button onClick={() => save('')} disabled={busy} className="flex-none rounded font-mono text-[11.5px] uppercase tracking-widest text-white/40 transition hover:text-white/80">
              reset to {saved.default_games_folder.replace(/^\/home\/[^/]+/, '~')}
            </button>
          )}
        </div>
      </Panel>

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
            <input
              type="number"
              min={0}
              max={365}
              step={1}
              value={cleanupDays}
              onChange={(e) => {
                const v = e.target.value === '' ? '' : Math.max(0, Math.min(365, Number(e.target.value)));
                setCleanupDays(v as number | '');
                setCleanupError(null);
              }}
              disabled={!saved}
              aria-label="Days to keep canceled downloads' data"
              className={cx(inputClass, 'w-24', cleanupError && 'ring-1 ring-red-400/60')}
            />
            <span className="text-[12.5px] text-white/70">days</span>
          </label>
          <button type="submit" disabled={!cleanupDirty || busy} className={cx(btnPrimary, 'px-4 py-2.5 text-[12.5px] disabled:cursor-default')}>
            Save
          </button>
        </form>
        <div className="mt-2.5 flex min-h-4 items-center gap-4">
          {cleanupError ? <p role="alert" className="font-mono text-[11.5px] text-red-300">{cleanupError}</p> : <span />}
        </div>
      </Panel>
    </div>
  );
}
