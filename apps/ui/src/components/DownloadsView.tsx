// i18n: converted
import { useTranslation } from 'react-i18next';
import { useEffect, useState } from 'react';
import { Download, Pause, Play, RotateCcw, Trash2, X } from 'lucide-react';
import type { Game, Job } from '../types';
import { Cover, EmptyState, ListRow, MonoLabel, Section, ViewHeader, cx, relTime } from './ui';
import { Button } from './ds/button';
import { formatBytes } from '../lib/games';
import { currentLocale } from '../lib/format';
import { useVariant } from '../lib/theme';
import { AtlasDownloads } from './AtlasDownloads';

export interface DownloadsViewProps {
  jobs: Job[];
  /** The library, for the queue art and recent thumbnails. Optional: without it the rows render text-only. */
  games?: Game[];
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
  'runner-install': 'downloads:action.install',
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
  'runner-install': 'downloads:kind.install',
};

const STATE_STYLE: Record<Job['state'], string> = {
  queued: 'text-text-muted',
  running: 'text-dl',
  paused: 'text-info',
  done: 'text-ok',
  failed: 'text-danger',
  canceled: 'text-text-muted',
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
  // Runner installs are not games: their row names the build instead of
  // linking to a game page that doesn't exist (issue #142, m13).
  const isRunnerJob = job.game_id.startsWith('runner:');
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
    <ListRow kind="job">
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          {isRunnerJob ? (
            <span className="truncate text-left text-[13px] font-bold">
              {t(VERB_KEY[job.kind])} · {job.title}
            </span>
          ) : (
            <button onClick={() => onOpenGame(job.game_id)} className="truncate text-left text-[13px] font-bold hover:text-dl">
              {t(VERB_KEY[job.kind])} · {job.title}
            </button>
          )}
          <div className={cx('truncate font-mono text-hint', STATE_STYLE[job.state])} title={detail}>
            {t(STATE_KEY[job.state])}{detail ? ` · ${detail}` : ''}
            {job.attempts > 1 && active ? ` · ${t('downloads:job.attempt', { count: job.attempts })}` : ''}
          </div>
          {job.state === 'canceled' && job.leftover && (
            <div className="mt-0.5 truncate font-mono text-hint text-attention/80" title={job.leftover}>
              {job.leftover}
            </div>
          )}
        </div>
        <div className="flex flex-none items-center gap-2">
          {pct != null && job.state === 'running' && <span className="font-mono text-[12px] tabular-nums text-text-muted">{pct}%</span>}
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
              {/* A check, a move, a removal or a runner install works on the
                  installed game or needs no cleanup: there is no download to discard. */}
              {job.kind !== 'verify' && job.kind !== 'move' && job.kind !== 'remove' && job.kind !== 'runner-install' && (
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
        <div className="mt-2.5 h-1.5 rounded-full bg-surface-strong" role="progressbar" aria-label={t('downloads:aria.progressOf', { title: job.title })} aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
          <div className={cx('h-full rounded-full transition-[width]', job.state === 'paused' ? 'bg-info' : 'bg-dl', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
        </div>
      )}
      {job.install_dir && job.kind !== 'uninstall' && job.kind !== 'move' && <div className="mt-1.5 truncate font-mono text-[12px] text-text-muted">{job.install_dir}</div>}
      {job.kind === 'move' && job.destination && <div className="mt-1.5 truncate font-mono text-[12px] text-text-muted">{job.install_dir} → {job.destination}</div>}
    </ListRow>
  );
}

export function DownloadsView({ jobs, games = [], onAction, onClear, onOpenGame }: DownloadsViewProps) {
  // One branch per view root (issue #156): the downloads presentation, not the shell.
  const presentation = useVariant('downloads.presentation');
  if (presentation === 'cards') {
    return <AtlasDownloads jobs={jobs} games={games} onAction={onAction} onClear={onClear} onOpenGame={onOpenGame} />;
  }
  return <ClassicDownloads jobs={jobs} onAction={onAction} onClear={onClear} onOpenGame={onOpenGame} />;
}

function ClassicDownloads({ jobs, onAction, onClear, onOpenGame }: Omit<DownloadsViewProps, 'games'>) {
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
      <ViewHeader
        title={t('nav:title.downloads')}
        subtitle={<p className="mt-0.5 text-[12.5px] text-text-muted">{t('downloads:header.subtitle')}</p>}
        actions={<div className="flex items-center gap-2">
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
        </div>}
      />

      {jobs.length === 0 ? (
        <EmptyState label={t('downloads:empty.label')}>{t('downloads:empty.detail')}</EmptyState>
      ) : (
        <>
          {active.length > 0 && (
            <Section className="mb-8">
              <MonoLabel className="mb-3">{t('downloads:section.queue')}</MonoLabel>
              <div className="space-y-2">{active.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </Section>
          )}
          {finished.length > 0 && (
            <Section>
              <MonoLabel className="mb-3">{t('downloads:section.finished')}</MonoLabel>
              <div className="space-y-2">{finished.map((j) => <JobRow key={j.id} job={j} onAction={onAction} onOpenGame={onOpenGame} />)}</div>
            </Section>
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
