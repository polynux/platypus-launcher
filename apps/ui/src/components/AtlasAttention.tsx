// i18n: converted
// The /22 Needs attention page (issue #138): each game as a card.
// Moved out of FixesView.tsx (issue #156) so each file holds one
// presentation; FixesView branches on fixes.presentation.
import { useTranslation } from 'react-i18next';
import React, { useState } from 'react';
import type { Game, Job, ProtonVersion } from '../types';
import { AlertTriangle, ArrowDownToLine, Check, Cloud, Download, FolderOpen, HardDrive, Wrench } from 'lucide-react';
import { ATLAS_PAGE, Cover, EmptyState } from './ui';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { attentionIssues, attentionReason, heroImages } from '../lib/games';
import { plainError } from '../lib/plainError';
import type { FixesProps } from './FixesView';

/** The /22 Needs attention page (issue #138, Atlas only): each game as a
 *  card with art, the rating as the kind line, and Open game. The lab's
 *  per-kind actions (retry install, install runner, move, repair, update,
 *  compare) have no backing in this view: those fixes live on the game page,
 *  so the card opens it. */
export function AtlasAttention({ games, jobs = [], runners = [], onOpenGame, onJobAction, onJob, onAllowFolder, onResetPrefix, onSaveSettings, onOpenRunners }: FixesProps) {
  const { t } = useTranslation(['nav', 'fixes', 'common']);
  const issues = attentionIssues(games, jobs, runners);
  const gameCount = new Set(issues.map((issue) => issue.game.id)).size;
  // Resetting a prefix asks first: the game confirmed is the one reset.
  const [resetTarget, setResetTarget] = useState<Game | null>(null);
  return (
    <div className={ATLAS_PAGE}>
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{t('fixes:atlas.title')}</h1>
        <p className="mt-[3px] text-[13px] text-text-muted">
          {gameCount > 0 ? t('fixes:atlas.sub', { count: gameCount }) : t('fixes:atlas.emptySub')}
        </p>
      </header>
      {issues.length === 0 ? (
        <EmptyState icon={Check} label={t('fixes:atlas.emptyTitle')}>{t('fixes:atlas.emptyBody')}</EmptyState>
      ) : (
        <ul className="grid gap-2.5">
          {issues.map((issue, index) => {
            const { game: g } = issue;
            const KindIcon = { install: Download, outside: FolderOpen, runner: Wrench, update: ArrowDownToLine, conflict: Cloud, prefix: HardDrive, rating: AlertTriangle }[issue.kind];
            const title = issue.kind === 'install' && issue.failedJobKind === 'update'
              ? t('fixes:atlas.issue.updateFailed')
              : ({
              install: t('fixes:atlas.issue.install'), outside: t('fixes:atlas.issue.outside'), runner: t('fixes:atlas.issue.runner'),
              update: t('fixes:atlas.issue.update'), conflict: t('fixes:atlas.issue.conflict'), prefix: t('fixes:atlas.issue.prefix'), rating: t('fixes:atlas.issue.rating'),
              })[issue.kind];
            const fix = issue.kind === 'runner'
              ? t('fixes:atlas.useDefaultRunner', { runner: runners.find((r) => r.is_default)?.name ?? t('fixes:atlas.defaultRunner') })
              : ({
                install: t('fixes:atlas.fix.install'), outside: t('fixes:atlas.fix.outside'), runner: t('fixes:atlas.fix.runner'),
                update: t('fixes:atlas.fix.update'), conflict: t('fixes:atlas.fix.conflict'), prefix: t('fixes:atlas.fix.prefix'), rating: t('fixes:atlas.fix.rating'),
              })[issue.kind];
            const issueJob = jobs.filter((j) => j.game_id === g.id && j.state === 'failed' && (j.kind === 'install' || j.kind === 'update')).sort((a, b) => b.updated_at - a.updated_at)[0];
            const fallbackRunner = runners.find((r) => r.is_default);
            const action = () => {
              if (issue.kind === 'install' && issueJob) onJobAction?.('retry', issueJob.id);
              else if (issue.kind === 'outside') onAllowFolder?.(g);
              else if (issue.kind === 'runner' && fallbackRunner) onSaveSettings?.(g, fallbackRunner.id);
              else if (issue.kind === 'update') onJob?.('update', g);
              else if (issue.kind === 'prefix') setResetTarget(g);
              // Save conflicts resolve on the game page: the cloud panel compares them.
              else onOpenGame(g.id);
            };
            const ghost = issue.kind === 'runner'
              ? { label: t('fixes:atlas.openRunners'), run: () => onOpenRunners?.() }
              : issue.kind === 'prefix'
                ? { label: t('fixes:atlas.openGame'), run: () => onOpenGame(g.id) }
                : (issue.kind === 'install' || issue.kind === 'outside' || issue.kind === 'update')
                  ? { label: t('fixes:atlas.openGame'), run: () => onOpenGame(g.id) }
                  : null;
            return (
              <li key={`${g.id}-${issue.kind}-${index}`} className="grid grid-cols-[150px_minmax(0,1fr)_auto] items-center gap-4 rounded-xl bg-surface p-3">
                <button type="button" onClick={() => onOpenGame(g.id)} className="block overflow-hidden rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus" aria-label={g.title}>
                  <Cover src={heroImages(g)} title={g.title} className="h-[70px] w-[150px] object-cover" />
                </button>
                <div className="grid min-w-0 gap-[3px]">
                  <div className={`inline-flex items-center gap-1.5 text-[12px] font-bold ${issue.kind === 'install' ? 'text-danger' : 'text-attention'}`}>
                    <KindIcon className="h-[13px] w-[13px] flex-none" aria-hidden />
                    <span className="truncate">{title}</span>
                  </div>
                  <button type="button" onClick={() => onOpenGame(g.id)} className="truncate justify-self-start text-left text-[15px] font-[750] hover:underline underline-offset-[3px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                    {g.title}
                  </button>
                  <p className="line-clamp-2 text-[12.5px] leading-[1.45] text-text-muted">{issue.kind === 'install' ? plainError(issue.error, t('fixes:atlas.reason.install')) : issue.kind === 'outside' ? t('fixes:atlas.reason.outside', { title: g.title, dir: g.install_dir }) : issue.kind === 'runner' ? t('fixes:atlas.reason.runner', { title: g.title, runner: issue.runnerId }) : issue.kind === 'update' ? t('fixes:atlas.reason.update') : issue.kind === 'conflict' ? t('fixes:atlas.reason.conflict') : issue.kind === 'prefix' ? t('fixes:atlas.reason.prefix') : attentionReason(g, 'rating')}</p>
                </div>
                <div className="grid gap-1">
                  <Button variant="primary" onClick={action}>{fix}</Button>
                  {ghost && <Button variant="ghost" onClick={ghost.run}>{ghost.label}</Button>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {resetTarget && (
        <Modal label={t('fixes:atlas.resetTitle')} onClose={() => setResetTarget(null)} size="sm">
          <div className="p-6">
            <h2 className="mb-2 text-[17px] font-bold">{t('fixes:atlas.resetTitle')}</h2>
            <p className="text-[13px] leading-relaxed text-text-muted">{t('fixes:atlas.resetBody', { title: resetTarget.title })}</p>
            <p className="mt-1.5 text-[13px] leading-relaxed text-text-muted">{t('fixes:atlas.resetBackup')}</p>
            <div className="mt-5 flex justify-end gap-2">
              <Button variant="ghost" autoFocus onClick={() => setResetTarget(null)}>{t('common:cancel')}</Button>
              <Button variant="primary" onClick={() => { onResetPrefix?.(resetTarget); setResetTarget(null); }}>{t('fixes:atlas.fix.prefix')}</Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
