// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useState } from 'react';
import { Pause, Play, RotateCcw, Trash2, X } from 'lucide-react';
import type { Job } from '../types';
import { EmptyState, MonoLabel, cx } from './ui';
import { Button } from './ds/button';
import { formatBytes } from '../lib/games';
import { currentLocale } from '../lib/format';

interface DownloadsViewProps {
  jobs: Job[];
  onAction: (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => void;
  onClear: () => void;
  onOpenGame: (gameId: string) => void;
}

const VERB_KEY: Record<Job['kind'], string> = {
  install: 'downloads:action.install',
  update: 'downloads:action.update',
  uninstall: 'downloads:action.uninstall',
  verify: 'downloads:action.verify',
  move: 'downloads:action.move',
  backup: 'downloads:action.backup',
  restore: 'downloads:action.restore',
  remove: 'downloads:action.remove',
};

const KIND_KEY: Record<Job['kind'], string> = {
  install: 'downloads:kind.install',
  update: 'downloads:kind.update',
  uninstall: 'downloads:kind.uninstall',
  verify: 'downloads:kind.verify',
  move: 'downloads:kind.move',
  backup: 'downloads:kind.backup',
  restore: 'downloads:kind.restore',
  remove: 'downloads:kind.remove',
};

const STATE_STYLE: Record<Job['state'], string> = {
  queued: 'text-white/55',
  running: 'text-amber-200',
  paused: 'text-sky-300',
  done: 'text-emerald-300',
  failed: 'text-red-300',
  canceled: 'text-white/55',
};

const STATE_KEY: Record<Job['state'], string> = {
  queued: 'downloads:state.queued',
  running: 'downloads:state.running',
  paused: 'downloads:state.paused',
  done: 'downloads:state.done',
  failed: 'downloads:state.failed',
  canceled: 'downloads:state.canceled',
};

/** When a finished job last changed: a full date and time, in the UI language.
 *  lib/format has no datetime formatter, so this stays a local helper. */
function formatFinished(ms: number): string {
  return new Date(ms).toLocaleString(currentLocale(), { dateStyle: 'medium', timeStyle: 'short' });
}

function JobRow({ job, onAction, onOpenGame }: { job: Job } & Pick<DownloadsViewProps, 'onAction' | 'onOpenGame'>) {
  const { t } = useTranslation(['downloads', 'common']);
  const active = job.state === 'queued' || job.state === 'running' || job.state === 'paused';
  const pct = job.percent >= 0 ? Math.round(job.percent) : null;
  const sizes = job.total > 0
    ? t('downloads:job.sizesOf', { done: formatBytes(job.bytes) || '0 B', total: formatBytes(job.total) })
    : '';
  const detail =
    job.state === 'running'
      ? [job.phase, sizes, job.speed > 0 && t('downloads:job.speedPerSec', { speed: formatBytes(job.speed) })].filter(Boolean).join(' · ')
      : job.state === 'failed'
        ? job.error
        : job.state === 'paused'
          ? [sizes, t('downloads:job.resumeNote')].filter(Boolean).join(' · ')
          : job.state === 'queued'
            ? job.phase === 'resuming' || job.phase?.includes('restart') ? job.phase : t('downloads:job.waiting')
            : formatFinished(job.updated_at);
  return (
    <div className="rounded-2xl bg-white/[0.045] px-4 py-3 ring-1 ring-white/[0.07]">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <button onClick={() => onOpenGame(job.game_id)} className="truncate text-left text-[13px] font-bold hover:text-amber-200">
            {t(VERB_KEY[job.kind])} · {job.title}
          </button>
          <div className={cx('truncate font-mono text-hint', STATE_STYLE[job.state])} title={detail}>
            {t(STATE_KEY[job.state])}{detail ? ` · ${detail}` : ''}
            {job.attempts > 1 && active ? ` · ${t('downloads:job.attempt', { count: job.attempts })}` : ''}
          </div>
          {job.state === 'canceled' && job.leftover && (
            <div className="mt-0.5 truncate font-mono text-hint text-orange-300/80" title={job.leftover}>
              {job.leftover}
            </div>
          )}
        </div>
        <div className="flex flex-none items-center gap-2">
          {pct != null && job.state === 'running' && <span className="font-mono text-[12px] tabular-nums text-white/70">{pct}%</span>}
          {job.state === 'running' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('pause', job.id)} aria-label={t('downloads:aria.pauseJob', { title: job.title })}>
              <Pause className="h-3 w-3" /> {t('downloads:job.pause')}
            </Button>
          )}
          {job.state === 'paused' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('resume', job.id)} aria-label={t('downloads:aria.resumeJob', { title: job.title })}>
              <Play className="h-3 w-3" /> {t('downloads:job.resume')}
            </Button>
          )}
          {job.state === 'queued' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('cancel', job.id)} aria-label={t('downloads:aria.cancelJob', { title: job.title })}>
              <X className="h-3 w-3" /> {t('common:cancel')}
            </Button>
          )}
          {job.state === 'canceled' && job.cleaned_at == null && (
            <>
              <Button variant="neutral" size="sm" onClick={() => onAction('resume', job.id)} aria-label={t('downloads:aria.resumeJob', { title: job.title })}>
                <Play className="h-3 w-3" /> {t('downloads:job.resume')}
              </Button>
              {/* A check, a move or a removal works on the installed game: there is no download to discard. */}
              {job.kind !== 'verify' && job.kind !== 'move' && job.kind !== 'remove' && (
                <Button variant="neutral" size="sm" onClick={() => onAction('discard', job.id)} aria-label={t('downloads:aria.discardJob', { title: job.title })}>
                  <Trash2 className="h-3 w-3" /> {t('downloads:job.discard')}
                </Button>
              )}
            </>
          )}
          {(job.state === 'failed' || job.state === 'canceled') && (
            <Button variant="neutral" size="sm" onClick={() => onAction('retry', job.id)}>
              <RotateCcw className="h-3 w-3" /> {t('common:retry')}
            </Button>
          )}
        </div>
      </div>
      {(job.state === 'running' || job.state === 'paused') && (
        <div className="mt-2.5 h-1.5 rounded-full bg-white/10" role="progressbar" aria-label={t('downloads:aria.progressOf', { title: job.title })} aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
          <div className={cx('h-full rounded-full transition-[width]', job.state === 'paused' ? 'bg-sky-300' : 'bg-amber-300', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
        </div>
      )}
      {job.install_dir && job.kind !== 'uninstall' && job.kind !== 'move' && <div className="mt-1.5 truncate font-mono text-[12px] text-white/55">{job.install_dir}</div>}
      {job.kind === 'move' && job.destination && <div className="mt-1.5 truncate font-mono text-[12px] text-white/55">{job.install_dir} → {job.destination}</div>}
    </div>
  );
}

export function DownloadsView({ jobs, onAction, onClear, onOpenGame }: DownloadsViewProps) {
  const { t } = useTranslation(['downloads', 'nav']);
  const active = jobs.filter((j) => j.state === 'queued' || j.state === 'running' || j.state === 'paused');
  const finished = jobs.filter((j) => !active.includes(j)).sort((a, b) => b.updated_at - a.updated_at);
  const [clearing, setClearing] = useState(false);
  useEffect(() => {
    if (finished.length === 0) setClearing(false);
  }, [finished.length]);
  const pausedCount = active.filter((j) => j.state === 'paused').length;
  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">{t('nav:title.downloads')}</h1>
          <p className="mt-0.5 text-[12.5px] text-white/70">
            {t('downloads:header.subtitle')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {pausedCount > 0 && (
            <Button variant="neutral" onClick={() => active.filter((j) => j.state === 'paused').forEach((j) => onAction('resume', j.id))}>
              {t('downloads:header.resumeAll')}
            </Button>
          )}
          {finished.length > 0 && (
            <Button variant="neutral" onClick={() => { setClearing(true); onClear(); }} disabled={clearing}>
              {clearing ? t('downloads:header.clearing') : t('downloads:header.clearFinished')}
            </Button>
          )}
        </div>
      </header>

      {jobs.length === 0 ? (
        <EmptyState label={t('downloads:empty.label')}>{t('downloads:empty.detail')}</EmptyState>
      ) : (
        <>
          {active.length > 0 && (
            <section className="mb-8">
              <MonoLabel className="mb-3">{t('downloads:section.queue')}</MonoLabel>
              <div className="space-y-2">{active.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </section>
          )}
          {finished.length > 0 && (
            <section>
              <MonoLabel className="mb-3">{t('downloads:section.finished')}</MonoLabel>
              <div className="space-y-2">{finished.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </section>
          )}
        </>
      )}
    </div>
  );
}

/** The lowercase kind noun for sentences ("install of Goose failed"). Exported
 *  so the activity log in App reuses the same words. */
export function jobKindKey(kind: Job['kind']): string {
  return KIND_KEY[kind];
}
