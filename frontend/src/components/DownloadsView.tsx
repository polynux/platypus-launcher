import React, { useEffect, useState } from 'react';
import { Pause, Play, RotateCcw, Trash2, X } from 'lucide-react';
import type { Job } from '../types';
import { EmptyState, MonoLabel, cx } from './ui';
import { Button } from './ds/button';
import { formatBytes } from '../lib/games';

interface DownloadsViewProps {
  jobs: Job[];
  onAction: (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => void;
  onClear: () => void;
  onOpenGame: (gameId: string) => void;
}

const VERB: Record<Job['kind'], string> = { install: 'Install', update: 'Update', uninstall: 'Uninstall', verify: 'Verify', move: 'Move' };

const STATE_STYLE: Record<Job['state'], string> = {
  queued: 'text-white/50',
  running: 'text-amber-200',
  paused: 'text-sky-300',
  done: 'text-emerald-300',
  failed: 'text-red-300',
  canceled: 'text-white/40',
};

function JobRow({ job, onAction, onOpenGame }: { job: Job } & Pick<DownloadsViewProps, 'onAction' | 'onOpenGame'>) {
  const active = job.state === 'queued' || job.state === 'running' || job.state === 'paused';
  const pct = job.percent >= 0 ? Math.round(job.percent) : null;
  const sizes = job.total > 0 ? `${formatBytes(job.bytes) || '0 B'} of ${formatBytes(job.total)}` : '';
  const detail =
    job.state === 'running'
      ? [job.phase, sizes, job.speed > 0 && `${formatBytes(job.speed)}/s`].filter(Boolean).join(' · ')
      : job.state === 'failed'
        ? job.error
        : job.state === 'paused'
          ? [sizes, 'resume continues where it stopped'].filter(Boolean).join(' · ')
          : job.state === 'queued'
            ? job.phase === 'resuming' || job.phase?.includes('restart') ? job.phase : 'waiting'
            : new Date(job.updated_at).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
  return (
    <div className="rounded-2xl bg-white/[0.045] px-4 py-3 ring-1 ring-white/[0.07]">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <button onClick={() => onOpenGame(job.game_id)} className="truncate text-left text-[13px] font-bold hover:text-amber-200">
            {VERB[job.kind]} · {job.title}
          </button>
          <div className={cx('truncate font-mono text-[11.5px]', STATE_STYLE[job.state])} title={detail}>
            {job.state}{detail ? ` · ${detail}` : ''}
            {job.attempts > 1 && active ? ` · attempt ${job.attempts}` : ''}
          </div>
          {job.state === 'canceled' && job.leftover && (
            <div className="mt-0.5 truncate font-mono text-[11px] text-orange-300/80" title={job.leftover}>
              {job.leftover}
            </div>
          )}
        </div>
        <div className="flex flex-none items-center gap-2">
          {pct != null && job.state === 'running' && <span className="font-mono text-[12px] tabular-nums text-white/70">{pct}%</span>}
          {job.state === 'running' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('pause', job.id)} aria-label={`Pause ${job.title}`}>
              <Pause className="h-3 w-3" /> Pause
            </Button>
          )}
          {job.state === 'paused' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('resume', job.id)} aria-label={`Resume ${job.title}`}>
              <Play className="h-3 w-3" /> Resume
            </Button>
          )}
          {job.state === 'queued' && (
            <Button variant="neutral" size="sm" onClick={() => onAction('cancel', job.id)} aria-label={`Cancel ${job.title}`}>
              <X className="h-3 w-3" /> Cancel
            </Button>
          )}
          {job.state === 'canceled' && job.cleaned_at == null && (
            <>
              <Button variant="neutral" size="sm" onClick={() => onAction('resume', job.id)} aria-label={`Resume ${job.title}`}>
                <Play className="h-3 w-3" /> Resume
              </Button>
              <Button variant="neutral" size="sm" onClick={() => onAction('discard', job.id)} aria-label={`Discard downloaded data of ${job.title}`}>
                <Trash2 className="h-3 w-3" /> Discard data
              </Button>
            </>
          )}
          {(job.state === 'failed' || job.state === 'canceled') && (
            <Button variant="neutral" size="sm" onClick={() => onAction('retry', job.id)}>
              <RotateCcw className="h-3 w-3" /> Retry
            </Button>
          )}
        </div>
      </div>
      {(job.state === 'running' || job.state === 'paused') && (
        <div className="mt-2.5 h-1.5 rounded-full bg-white/10" role="progressbar" aria-label={`${job.title} progress`} aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
          <div className={cx('h-full rounded-full transition-[width]', job.state === 'paused' ? 'bg-sky-300' : 'bg-amber-300', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
        </div>
      )}
      {job.install_dir && job.kind !== 'uninstall' && job.kind !== 'move' && <div className="mt-1.5 truncate font-mono text-[12px] text-white/55">{job.install_dir}</div>}
      {job.kind === 'move' && job.destination && <div className="mt-1.5 truncate font-mono text-[12px] text-white/55">{job.install_dir} → {job.destination}</div>}
    </div>
  );
}

export function DownloadsView({ jobs, onAction, onClear, onOpenGame }: DownloadsViewProps) {
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
          <h1 className="text-[23px] font-extrabold tracking-tight">Downloads</h1>
          <p className="mt-0.5 text-[12.5px] text-white/50">
            Installs, updates and uninstalls run one at a time. A paused download keeps its data and resumes from there; an interrupted install resumes when Hydra starts again.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {pausedCount > 0 && (
            <Button variant="neutral" onClick={() => active.filter((j) => j.state === 'paused').forEach((j) => onAction('resume', j.id))}>
              Resume all
            </Button>
          )}
          {finished.length > 0 && (
            <Button variant="neutral" onClick={() => { setClearing(true); onClear(); }} disabled={clearing}>
              {clearing ? 'Clearing…' : 'Clear finished'}
            </Button>
          )}
        </div>
      </header>

      {jobs.length === 0 ? (
        <EmptyState label="queue empty">Nothing is downloading. Install a game from a store plugin on its game page.</EmptyState>
      ) : (
        <>
          {active.length > 0 && (
            <section className="mb-8">
              <MonoLabel className="mb-3">queue</MonoLabel>
              <div className="space-y-2">{active.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </section>
          )}
          {finished.length > 0 && (
            <section>
              <MonoLabel className="mb-3">finished</MonoLabel>
              <div className="space-y-2">{finished.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </section>
          )}
        </>
      )}
    </div>
  );
}
