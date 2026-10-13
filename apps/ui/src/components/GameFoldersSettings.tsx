// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { FolderOpen, X } from 'lucide-react';
import { LauncherApi } from '../effect/runtime';
import type { GameFolder } from '../types';
import { Button } from '@/components/ds/button';
import { freeLabel, shortPath } from '../lib/installLocation';
import { MonoLabel } from './ui';
import { ErrorNotice } from './ErrorNotice';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The game folders in Settings: the default one, where installs go, then the
 *  library folders. Each game's "Install to…" step offers all of them. */
export function GameFoldersSettings({ onNotify }: { onNotify: (msg: string, kind?: 'ok' | 'err') => void }) {
  const { t } = useTranslation('settings');
  const [folders, setFolders] = useState<GameFolder[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    Effect.runPromise(LauncherApi.getGameFolders()).then(setFolders).catch((e) => setError(errMsg(e)));
  }, []);

  const run = (job: Promise<GameFolder[]>, done?: string) => {
    setBusy(true);
    setError(null);
    return job
      .then((list) => { setFolders(list); if (done) onNotify(done); })
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  const add = () => {
    Effect.runPromise(LauncherApi.chooseGameFolder())
      .then((path) => { if (path) return run(Effect.runPromise(LauncherApi.addGameFolder(path)), t('folders.added')); })
      .catch((e) => setError(errMsg(e)));
  };

  const defaultPath = folders?.find((f) => f.is_default)?.path;

  return (
    <section className="max-w-2xl border-t border-white/[0.07] py-6">
      <h2 className="text-[14px] font-bold">{t('folders.heading')}</h2>
      <p className="mb-4 max-w-[75ch] text-[12.5px] leading-relaxed text-white/70">
        {t('folders.description.before')}{' '}
        <span className="font-mono text-hint text-white/75">{shortPath(defaultPath ?? '~/Games') + '/epic'}</span>{t('folders.description.after')}
      </p>
      <ul className="divide-y divide-white/[0.06] border-y border-white/[0.07]" aria-label={t('folders.heading')}>
        {(folders ?? []).map((f) => (
          <li key={f.path} className="flex items-center gap-3 px-3 py-2">
            <span className="min-w-0 flex-1">
              <span className="block truncate font-mono text-[12px] text-white/85" title={f.path}>{shortPath(f.path)}</span>
              <span className="block font-mono text-hint text-white/55">
                {f.exists ? freeLabel(f.free_bytes) : t('folders.pending')}
              </span>
            </span>
            {f.is_default ? (
              <MonoLabel>{t('folders.default')}</MonoLabel>
            ) : (
              <>
                <Button
                  variant="link"
                  size="none"
                  aria-label={t('folders.makeDefaultLabel', { folder: f.path })}
                  disabled={busy}
                  className="flex-none text-hint"
                  onClick={() => run(Effect.runPromise(LauncherApi.makeDefaultGameFolder(f.path)), t('folders.madeDefault'))}
                >
                  {t('folders.makeDefault')}
                </Button>
                <Button
                  variant="neutral"
                  size="icon"
                  aria-label={t('folders.removeLabel', { folder: f.path })}
                  disabled={busy}
                  onClick={() => run(Effect.runPromise(LauncherApi.removeGameFolder(f.path)), t('folders.removed'))}
                >
                  <X className="size-3.5" />
                </Button>
              </>
            )}
          </li>
        ))}
      </ul>
      <div className="mt-3 flex min-h-8 items-center justify-between gap-4">
        {error ? <ErrorNotice message={error} fallback={t('folders.error')} /> : <span />}
        <Button variant="neutral" onClick={add} disabled={busy || folders == null}>
          <FolderOpen className="size-3.5" /> {t('folders.add')}
        </Button>
      </div>
    </section>
  );
}
