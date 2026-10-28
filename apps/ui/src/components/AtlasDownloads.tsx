// i18n: converted
// The /22 Downloads page (issue #138): queue cards + recent table.
// Moved out of DownloadsView.tsx (issue #156) so each file holds one presentation.
import { useTranslation } from 'react-i18next';
import React, { useEffect, useState } from 'react';
import { Download, Pause, Play, RotateCcw, Trash2, X } from 'lucide-react';
import type { Game, Job } from '../types';
import { ATLAS_PAGE, Cover, EmptyState, cx, relTime } from './ui';
import { Button } from './ds/button';
import { formatBytes, heroImages } from '../lib/games';

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

const STATE_KEY: Record<Job['state'], string> = {
  queued: 'downloads:state.queued',
  running: 'downloads:state.running',
  paused: 'downloads:state.paused',
  done: 'downloads:state.done',
  failed: 'downloads:state.failed',
  canceled: 'downloads:state.canceled',
};

export type AtlasDownloadsProps = {
  jobs: Job[];
  games: Game[];
  onAction: (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => void;
  onClear: () => void;
  onOpenGame: (gameId: string) => void;
};

/** The /22 Downloads page (issue #138, Atlas only): the queue as cards with
 *  art and progress, then the finished jobs as a recent table. Same jobs and
 *  actions; only the markup is /22. There is no separate background-work
 *  feed in the app: every job is a queue job, so that lab row has no card. */
export function AtlasDownloads({ jobs, games, onAction, onClear, onOpenGame }: AtlasDownloadsProps) {
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
          <section aria-label={t('downloads:atlas.queue')}>
            <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('downloads:atlas.queue')}</h2>
            {active.length > 0 ? (
            <ul className="grid gap-2">
                {active.map((j, i) => {
                  const pct = j.percent >= 0 ? Math.round(j.percent) : null;
                  const paused = j.state === 'paused';
                  const name = byId.get(j.game_id)?.title ?? j.title;
                  const running = j.state === 'running';
                  const eta = running && j.total > 0 && j.speed > 0 && j.total > j.bytes
                    ? t('downloads:job.etaMins', { count: Math.max(1, Math.round((j.total - j.bytes) / j.speed / 60)) })
                    : null;
                  const stateNote = paused
                    ? t('downloads:atlas.state.paused')
                    : j.state === 'queued'
                      ? t('downloads:atlas.state.queued')
                      : j.speed > 0
                        ? t('downloads:job.speedPerSec', { speed: formatBytes(j.speed) })
                        : null;
                  return (
                    <li key={j.id} className={paused ? 'opacity-70' : undefined}>
                      <div className={cx('flex items-center gap-3.5 rounded-xl bg-surface py-2.5 pl-2.5 pr-3', i === 0 && 'ring-1 ring-inset ring-accent/35')}>
                        <button type="button" onClick={() => onOpenGame(j.game_id)} aria-label={name} className="flex-none rounded-[7px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                          <Cover src={artOf(j.game_id)} title={name} className="h-14 w-[120px] rounded-[7px]" />
                        </button>
                        <div className="grid min-w-0 flex-1 gap-1.5">
                          <div className="flex flex-wrap items-baseline gap-x-2">
                            <span className="truncate text-left text-[13px] font-bold">
                              {name}
                            </span>
                            <span className="truncate text-[12px] text-text-faint">{t(VERB_KEY[j.kind])}</span>
                          </div>
                          <div className="h-1.5 overflow-hidden rounded-[3px] bg-surface-strong" role="progressbar" aria-label={t('downloads:aria.progressOf', { title: name })} aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
                            <div className={cx('h-full rounded-[3px] transition-[width]', paused ? 'bg-text-faint' : 'bg-accent', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
                          </div>
                          <div className="flex gap-3.5 text-[12px] text-text-faint tabular-nums">
                            {pct != null && <span>{pct}%</span>}
                            {stateNote && <span>{stateNote}</span>}
                            {i === 0 && eta && <span>{eta}</span>}
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
            ) : (
              <EmptyState icon={Download} label={t('downloads:atlas.emptyTitle')}>{t('downloads:atlas.emptyBody')}</EmptyState>
            )}
          </section>
          {finished.length > 0 && (
            <section aria-label={t('downloads:atlas.recent')}>
              <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('downloads:atlas.recent')}</h2>
              <ul className="divide-y divide-hairline">
                {finished.map((j) => (
                  <li key={j.id} className="grid grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)_auto_auto_auto] items-center gap-3 py-1.5">
                    <button onClick={() => onOpenGame(j.game_id)} className="flex min-w-0 items-center gap-2.5 rounded-lg px-1 py-1 text-left hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                      <Cover src={artOf(j.game_id)} title={byId.get(j.game_id)?.title ?? j.title} className="h-5 w-11 flex-none rounded ring-1 ring-hairline" />
                      <span className="truncate text-[13px] font-semibold">{byId.get(j.game_id)?.title ?? j.title}</span>
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
