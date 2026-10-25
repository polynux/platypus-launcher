// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useState } from 'react';
import { Download, Pause, Play, RotateCcw, Trash2, X } from 'lucide-react';
import type { Game, Job } from '../types';
import { ATLAS_PAGE, Cover, EmptyState, ListRow, MonoLabel, Section, ViewHeader, cx, relTime } from './ui';
import { Button } from './ds/button';
import { formatBytes, heroImages } from '../lib/games';
import { currentLocale } from '../lib/format';
import { useVariant } from '../lib/theme';

interface DownloadsViewProps {
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
          <button onClick={() => onOpenGame(job.game_id)} className="truncate text-left text-[13px] font-bold hover:text-dl">
            {t(VERB_KEY[job.kind])} · {job.title}
          </button>
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
  const { t } = useTranslation(['downloads', 'nav']);
  // Atlas renders the /22 Downloads page (issue #138); other themes keep the groups below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasDownloads jobs={jobs} games={games} onAction={onAction} onClear={onClear} onOpenGame={onOpenGame} />;
  }
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

/** The /22 Downloads page (issue #138, Atlas only): the queue as cards with
 *  art and progress, then the finished jobs as a recent table. Same jobs and
 *  actions; only the markup is /22. There is no separate background-work
 *  feed in the app: every job is a queue job, so that lab row has no card. */
function AtlasDownloads({ jobs, games, onAction, onClear, onOpenGame }: Required<Pick<DownloadsViewProps, 'jobs' | 'games' | 'onAction' | 'onClear' | 'onOpenGame'>>) {
  const { t } = useTranslation(['downloads', 'nav', 'common']);
  const active = jobs.filter((j) => j.state === 'queued' || j.state === 'running' || j.state === 'paused');
  const finished = jobs.filter((j) => !active.includes(j)).sort((a, b) => b.updated_at - a.updated_at);
  const byId = React.useMemo(() => new Map(games.map((g) => [g.id, g])), [games]);
  const artOf = (gameId: string): string[] => {
    const g = byId.get(gameId);
    return g ? heroImages(g) : [];
  };
  const [clearing, setClearing] = useState(false);
  useEffect(() => {
    if (finished.length === 0) setClearing(false);
  }, [finished.length]);
  const speed = active.reduce((a, j) => a + (j.speed > 0 ? j.speed : 0), 0);
  return (
    <div className={ATLAS_PAGE}>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.downloads')}</h1>
          <p className="mt-[3px] text-[13px] text-text-muted">
            {active.length > 0
              ? (speed > 0
                ? t('downloads:atlas.queueSubSpeed', { count: active.length, speed: formatBytes(speed) })
                : t('downloads:atlas.queueSub', { count: active.length }))
              : t('downloads:atlas.queueEmptySub')}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {active.some((j) => j.state === 'running' || j.state === 'queued') && (
            <Button variant="neutral" onClick={() => active.forEach((j) => onAction('pause', j.id))}>
              <Pause className="h-3.5 w-3.5" /> {t('downloads:atlas.pauseAll')}
            </Button>
          )}
          {active.some((j) => j.state === 'paused') && (
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
        <EmptyState icon={Download} label={t('downloads:atlas.emptyTitle')}>{t('downloads:atlas.emptyBody')}</EmptyState>
      ) : (
        <>
          {active.length > 0 && (
            <section aria-label={t('downloads:atlas.queue')}>
              <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('downloads:atlas.queue')}</h2>
              <ul className="grid gap-2">
                {active.map((j) => {
                  const pct = j.percent >= 0 ? Math.round(j.percent) : null;
                  const paused = j.state === 'paused';
                  const meta = paused
                    ? t('downloads:state.paused')
                    : j.state === 'queued'
                      ? t('downloads:state.queued')
                      : [j.speed > 0 ? t('downloads:job.speedPerSec', { speed: formatBytes(j.speed) }) : '', j.phase].filter(Boolean).join(' · ');
                  return (
                    <li key={j.id} className={paused ? 'opacity-70' : undefined}>
                      <div className="flex items-center gap-3.5 rounded-xl bg-surface px-2.5 py-2.5 ring-1 ring-hairline">
                        <Cover src={artOf(j.game_id)} title={j.title} className="h-14 w-28 flex-none rounded-lg ring-1 ring-hairline" />
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-baseline gap-x-2">
                            <button onClick={() => onOpenGame(j.game_id)} className="truncate text-left text-[13px] font-bold hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                              {j.title}
                            </button>
                            <span className="truncate text-[12px] text-text-faint">{t(VERB_KEY[j.kind])}</span>
                          </div>
                          <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-surface-strong" role="progressbar" aria-label={t('downloads:aria.progressOf', { title: j.title })} aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
                            <div className={cx('h-full rounded-full bg-accent transition-[width]', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
                          </div>
                          <div className="mt-1 truncate text-[12px] text-text-faint">
                            {pct != null && `${pct}% · `}{meta}
                          </div>
                        </div>
                        <div className="flex flex-none items-center gap-1">
                          {j.state === 'running' || j.state === 'queued' ? (
                            <Button variant="ghost" size="icon" className="size-8" onClick={() => onAction('pause', j.id)} aria-label={t('downloads:aria.pauseJob', { title: j.title })}>
                              <Pause className="size-4" />
                            </Button>
                          ) : (
                            <Button variant="ghost" size="icon" className="size-8" onClick={() => onAction('resume', j.id)} aria-label={t('downloads:aria.resumeJob', { title: j.title })}>
                              <Play className="size-4" />
                            </Button>
                          )}
                          <Button variant="ghost" size="icon" className="size-8" onClick={() => onAction('cancel', j.id)} aria-label={t('downloads:aria.cancelJob', { title: j.title })}>
                            <X className="size-4" />
                          </Button>
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          )}
          {finished.length > 0 && (
            <section aria-label={t('downloads:atlas.recent')}>
              <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('downloads:atlas.recent')}</h2>
              <ul className="divide-y divide-hairline">
                {finished.map((j) => (
                  <li key={j.id} className="grid grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)_auto_auto_auto] items-center gap-3 py-1.5">
                    <button onClick={() => onOpenGame(j.game_id)} className="flex min-w-0 items-center gap-2.5 rounded-lg px-1 py-1 text-left hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                      <Cover src={artOf(j.game_id)} title={j.title} className="h-5 w-11 flex-none rounded ring-1 ring-hairline" />
                      <span className="truncate text-[13px] font-semibold">{j.title}</span>
                    </button>
                    <span className={cx('truncate text-[12.5px]', j.state === 'failed' ? 'text-danger' : 'text-text-muted')}>
                      {j.state === 'failed' ? (j.error || t('downloads:state.failed')) : t(STATE_KEY[j.state])}
                    </span>
                    <span className="text-[12px] tabular-nums text-text-muted">{j.total > 0 ? formatBytes(j.total) : ''}</span>
                    <span className="text-[12px] text-text-muted">{relTime(j.updated_at)}</span>
                    <span className="flex justify-end gap-1.5">
                      {(j.state === 'failed' || j.state === 'canceled') && (
                        <Button variant="ghost" size="sm" onClick={() => onAction('retry', j.id)} className="text-danger">
                          <RotateCcw className="h-3 w-3" /> {t('common:retry')}
                        </Button>
                      )}
                      {j.state === 'canceled' && j.cleaned_at == null && j.kind !== 'verify' && j.kind !== 'move' && j.kind !== 'remove' && (
                        <Button variant="neutral" size="sm" onClick={() => onAction('discard', j.id)} aria-label={t('downloads:aria.discardJob', { title: j.title })}>
                          <Trash2 className="h-3 w-3" /> {t('downloads:job.discard')}
                        </Button>
                      )}
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}
