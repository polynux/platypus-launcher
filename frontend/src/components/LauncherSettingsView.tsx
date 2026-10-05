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

  useEffect(() => {
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => { setSaved(s); setDraft(s.games_folder); })
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
          library folders. Nothing installs here until store plugins arrive.
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
          {error ? <p role="alert" className="font-mono text-[10.5px] text-red-300">{error}</p> : <span />}
          {saved && !isDefault && (
            <button onClick={() => save('')} disabled={busy} className="flex-none rounded font-mono text-[10px] uppercase tracking-widest text-white/40 transition hover:text-white/80">
              reset to {saved.default_games_folder.replace(/^\/home\/[^/]+/, '~')}
            </button>
          )}
        </div>
      </Panel>
    </div>
  );
}
