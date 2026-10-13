// i18n: converted
import React, { useState } from 'react';
import { Effect } from 'effect';
import { useTranslation } from 'react-i18next';
import { Loader2 } from 'lucide-react';
import type { CloudConflict, CloudInfo, CloudStatus, Game } from '../types';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { LauncherApi } from '../effect/runtime';
import { fileSize, whenLabel } from '../lib/cloud';
import { MonoLabel, SegRow, cx } from './ui';
import { ErrorNotice } from './ErrorNotice';

type Choice = 'local' | 'remote' | 'later';

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

interface Props {
  game: Game;
  status: CloudStatus;
  onClose: () => void;
  /** Called with the game's new cloud info after files were settled. */
  onResolved: (info: CloudInfo) => void;
  /** Set when a launch is waiting on the answer: adds "Launch without resolving". */
  onLaunchAnyway?: () => void;
}

/** Saves that changed on this device and in the cloud: per file, both sides'
 *  sizes and dates and a choice (Keep this device / Keep cloud / Decide later).
 *  Platypus never picks for the user. */
export function CloudConflictDialog({ game, status, onClose, onResolved, onLaunchAnyway }: Props) {
  const { t } = useTranslation('cloud');
  const conflicts = status.conflicts ?? [];
  const [choices, setChoices] = useState<Record<string, Choice>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const choiceOf = (path: string): Choice => choices[path] ?? 'later';
  const chosen = conflicts.filter((c) => choiceOf(c.path) !== 'later');
  const setAll = (v: Choice) => setChoices(Object.fromEntries(conflicts.map((c) => [c.path, v])));

  const apply = () => {
    setBusy(true);
    setError(null);
    const payload: Record<string, 'local' | 'remote'> = {};
    for (const c of chosen) payload[c.path] = choiceOf(c.path) as 'local' | 'remote';
    Effect.runPromise(LauncherApi.resolveCloudSaves(game, payload))
      .then(onResolved)
      .catch((e) => setError(errMsg(e)))
      .finally(() => setBusy(false));
  };

  return (
    <Modal label={t('cloud:conflict.label')} onClose={onClose} size="lg">
      <h2 className="text-[19px] font-extrabold tracking-tight">{t('cloud:conflict.heading')}</h2>
      <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">
        {t('cloud:conflict.intro', { title: game.title, count: conflicts.length })}{' '}
        {t('cloud:conflict.explanation')}
      </p>

      {conflicts.length > 1 && (
        <div className="mt-3 flex items-center gap-3 font-mono text-hint text-white/55">
          {t('cloud:conflict.allFiles')}
          <Button variant="link" onClick={() => setAll('local')}>{t('cloud:conflict.keepLocal')}</Button>
          <Button variant="link" onClick={() => setAll('remote')}>{t('cloud:conflict.keepRemote')}</Button>
          <Button variant="link" onClick={() => setAll('later')}>{t('cloud:conflict.decideLater')}</Button>
        </div>
      )}

      <ul aria-label={t('cloud:conflict.filesLabel')} className="mt-4 space-y-3">
        {conflicts.map((c) => (
          <li key={c.path} className="rounded-2xl bg-white/[0.04] p-4 ring-1 ring-white/[0.07]">
            <div className="truncate font-mono text-[12px] text-white/85 select-text" title={c.path}>{c.path}</div>
            <div className="mt-2.5 grid gap-2 sm:grid-cols-2">
              <Side title={t('cloud:conflict.local')} file={c.local} newer={newerSide(c) === 'local'} />
              <Side title={t('cloud:conflict.remote')} file={c.remote} newer={newerSide(c) === 'remote'} from={c.remote.machine} />
            </div>
            <div className="mt-3">
              <SegRow<Choice>
                label={t('cloud:conflict.choice', { path: c.path })}
                value={choiceOf(c.path)}
                onChange={(v) => setChoices((prev) => ({ ...prev, [c.path]: v }))}
                options={[
                  { value: 'local', label: t('cloud:conflict.optLocal') },
                  { value: 'remote', label: t('cloud:conflict.optRemote') },
                  { value: 'later', label: t('cloud:conflict.optLater') },
                ]}
              />
            </div>
          </li>
        ))}
      </ul>

      {status.backupDir && <p className="mt-3 font-mono text-hint text-white/55 select-text">{t('cloud:conflict.backups', { dir: status.backupDir })}</p>}
      {error && <ErrorNotice className="mt-3" message={error} fallback={t('cloud:conflict.error')} />}

      <div className="mt-5 flex flex-wrap items-center justify-end gap-2">
        {onLaunchAnyway && (
          <Button variant="neutral" onClick={onLaunchAnyway} disabled={busy} className="mr-auto">
            {t('cloud:conflict.launchAnyway')}
          </Button>
        )}
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common:close')}</Button>
        <Button variant="primary" onClick={apply} disabled={busy || chosen.length === 0}>
          {busy && <Loader2 className="h-4 w-4 animate-spin" />}
          {chosen.length === 0 ? t('cloud:conflict.applyNone') : t('cloud:conflict.apply', { count: chosen.length })}
        </Button>
      </div>
    </Modal>
  );
}

function newerSide(c: CloudConflict): 'local' | 'remote' | null {
  if (c.local.mtime === c.remote.mtime || !c.local.mtime || !c.remote.mtime) return null;
  return c.local.mtime > c.remote.mtime ? 'local' : 'remote';
}

function Side({ title, file, newer, from }: { title: string; file: CloudConflict['local']; newer: boolean; from?: string }) {
  const { t } = useTranslation('cloud');
  return (
    <div className={cx('rounded-xl px-3 py-2.5 ring-1', newer ? 'bg-amber-300/[0.06] ring-amber-300/25' : 'ring-white/[0.07]')}>
      <div className="flex items-center justify-between gap-2">
        <MonoLabel>{title}</MonoLabel>
        {newer && <Badge variant="accent">{t('cloud:conflict.newer')}</Badge>}
      </div>
      <div className="mt-1.5 font-mono text-[12px] text-white/85">{fileSize(file.size)}</div>
      <div className="font-mono text-hint text-white/55">{whenLabel(file.mtime)}</div>
      {from && <div className="font-mono text-hint text-white/55">{t('cloud:conflict.from', { machine: from })}</div>}
    </div>
  );
}
