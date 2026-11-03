// i18n: converted
// The /22 Needs attention page (issue #138): each game as a card.
// Moved out of FixesView.tsx (issue #156) so each file holds one
// presentation; FixesView branches on fixes.presentation.
import { useTranslation } from 'react-i18next';
import React, { useState } from 'react';
import type { Game, Job, ProtonVersion } from '../types';
import { AlertTriangle, ArrowDownToLine, Check, Cloud, Download, FolderOpen, Wrench } from 'lucide-react';
import { ATLAS_PAGE, Cover, EmptyState } from './ui';
import { Button } from './ds/button';
import { Modal, ModalFoot, ModalHead } from './Modal';
import { attentionIssues, attentionReason, heroImages, isSystemFolder, type AttentionIssue } from '../lib/games';
import { RawDetails } from './ErrorNotice';
import { notify } from '../lib/notify';
import type { FixesProps } from './FixesView';

/** One issue's fix behaviour, shared by the Needs attention page and the Home
 *  card (audit finding P8): the same sentence and primary action per kind,
 *  the page's secondary ghost action, and the two confirmation dialogs. */
export function useAttentionFix({ jobs = [], runners = [], onOpenGame, onOpenRunners, onRetryJob, onJob, onAllowFolder, onResetPrefix, onSaveSettings }: {
  jobs?: Job[]; runners?: ProtonVersion[];
  onOpenGame: (id: string) => void; onOpenRunners?: () => void;
  onRetryJob?: (id: string) => void; onJob?: (kind: 'update', game: Game) => void;
  onAllowFolder?: (game: Game) => void; onResetPrefix?: (game: Game) => void;
  onSaveSettings?: (game: Game, runner: string, notifySaved?: boolean) => Promise<void>; // i18n:allow: Promise return type
}): {
  issueText: (issue: AttentionIssue) => string;
  primaryAction: (issue: AttentionIssue) => { label: string; run: () => void };
  ghostAction: (issue: AttentionIssue) => { label: string; run: () => void } | null;
  fixDialogs: React.ReactNode;
} {
  const { t } = useTranslation(['fixes', 'common']);
  // Resetting a prefix asks first: the game confirmed is the one reset.
  const [resetTarget, setResetTarget] = useState<Game | null>(null);
  const [allowTarget, setAllowTarget] = useState<Game | null>(null);
  const issueText = (issue: AttentionIssue): string => {
    const g = issue.game;
    switch (issue.kind) {
      case 'install': return t('fixes:atlas.reason.install');
      case 'outside':
        if (isSystemFolder(g.install_dir)) {
          return t('fixes:atlas.reason.outsideSystem', { title: g.title, dir: g.install_dir });
        }
        return t('fixes:atlas.reason.outside', { title: g.title, dir: g.install_dir });
      case 'runner': return t('fixes:atlas.reason.runner', { title: g.title, runner: issue.runnerId });
      case 'update': return t('fixes:atlas.reason.update');
      case 'conflict': return t('fixes:atlas.reason.conflict');
      case 'prefix': return t('fixes:atlas.reason.prefix');
      default: return attentionReason(g, 'rating');
    }
  };
  const primaryLabel = (issue: AttentionIssue): string => {
    if (issue.kind === 'outside' && isSystemFolder(issue.game.install_dir)) {
      return t('fixes:atlas.openGame');
    }
    return issue.kind === 'runner'
    ? t('fixes:atlas.useDefaultRunner', { runner: runners.find((r) => r.is_default)?.name ?? t('fixes:atlas.defaultRunner') })
    : ({
      install: t('fixes:atlas.fix.install'), outside: t('fixes:atlas.fix.outside'), runner: t('fixes:atlas.fix.runner'),
      update: t('fixes:atlas.fix.update'), conflict: t('fixes:atlas.fix.conflict'), prefix: t('fixes:atlas.fix.prefix'), rating: t('fixes:atlas.fix.rating'),
    })[issue.kind];
  };
  const runIssueFix = (issue: AttentionIssue) => {
    const g = issue.game;
    if (issue.kind === 'install') {
      const job = jobs.filter((j) => j.game_id === g.id && j.state === 'failed' && (j.kind === 'install' || j.kind === 'update')).sort((a, b) => b.updated_at - a.updated_at)[0];
      if (job && onRetryJob) { onRetryJob(job.id); return; }
    } else if (issue.kind === 'outside') {
      // System folders never allow (audit F16, R2-1): the core also refuses
      // to move out of them, so open the game instead of offering a dialog
      // the core will refuse. The card text suggests reinstalling.
      if (isSystemFolder(g.install_dir)) { onOpenGame(g.id); return; }
      setAllowTarget(g); return;
    } else if (issue.kind === 'runner') {
      const fallback = runners.find((r) => r.is_default);
      if (fallback && onSaveSettings) {
        const previous = g.settings.runner_id || '';
        const save = onSaveSettings(g, fallback.id, false);
        if (save) void save.then(() => notify.success(
          t('fixes:atlas.runnerChanged', { runner: fallback.name, title: g.title }),
          { action: { label: t('fixes:atlas.undo'), onClick: () => {
            const undo = onSaveSettings?.(g, previous, false);
            if (undo) void undo.catch(() => {});
          } } },
        )).catch(() => {});
        return;
      }
    } else if (issue.kind === 'update') {
      if (onJob) { onJob('update', g); return; }
    } else if (issue.kind === 'prefix') { setResetTarget(g); return; }
    // Save conflicts resolve on the game page: the cloud panel compares them.
    onOpenGame(g.id);
  };
  const ghostAction = (issue: AttentionIssue): { label: string; run: () => void } | null => {
    const g = issue.game;
    if (issue.kind === 'runner') return { label: t('fixes:atlas.openRunners'), run: () => onOpenRunners?.() };
    // A system-folder card has Open game as its only action (R2-1).
    if (issue.kind === 'outside' && isSystemFolder(g.install_dir)) return null;
    if (issue.kind === 'prefix' || issue.kind === 'install' || issue.kind === 'outside' || issue.kind === 'update') {
      return { label: t('fixes:atlas.openGame'), run: () => onOpenGame(g.id) };
    }
    return null;
  };
  const fixDialogs = (
    <>
      {resetTarget && (
        <Modal label={t('fixes:atlas.resetTitle')} onClose={() => setResetTarget(null)} size="sm">
          <div className="-m-7">
            <ModalHead title={t('fixes:atlas.resetTitle')} onClose={() => setResetTarget(null)} />
            <div className="space-y-1.5 p-5">
              <p className="text-[13px] leading-relaxed text-text-muted">{t('fixes:atlas.resetBody', { title: resetTarget.title })}</p>
              <p className="text-[13px] leading-relaxed text-text-muted">{t('fixes:atlas.resetBackup')}</p>
            </div>
            <ModalFoot>
              <Button variant="ghost" autoFocus onClick={() => setResetTarget(null)}>{t('common:cancel')}</Button>
              <Button variant="primary" onClick={() => { onResetPrefix?.(resetTarget); setResetTarget(null); }}>{t('fixes:atlas.fix.prefix')}</Button>
            </ModalFoot>
          </div>
        </Modal>
      )}
      {allowTarget && (
        <Modal label={t('fixes:atlas.allowTitle')} onClose={() => setAllowTarget(null)} size="sm">
          <div className="-m-7">
            <ModalHead title={t('fixes:atlas.allowTitle')} onClose={() => setAllowTarget(null)} />
            <p className="p-5 text-[13px] leading-relaxed text-text-muted">{t('fixes:atlas.allowBody', { folder: allowTarget.install_dir || t('fixes:atlas.unknownFolder') })}</p>
            <ModalFoot>
              <Button variant="ghost" autoFocus onClick={() => setAllowTarget(null)}>{t('common:cancel')}</Button>
              <Button variant="primary" onClick={() => { onAllowFolder?.(allowTarget); setAllowTarget(null); }}>{t('fixes:atlas.fix.outside')}</Button>
            </ModalFoot>
          </div>
        </Modal>
      )}
    </>
  );
  return {
    issueText,
    primaryAction: (issue) => ({ label: primaryLabel(issue), run: () => runIssueFix(issue) }),
    ghostAction,
    fixDialogs,
  };
}

/** The /22 Needs attention page (issue #138, Atlas only): each game as a
 *  card with art, the rating as the kind line, and Open game. The lab's
 *  per-kind actions (retry install, install runner, move, repair, update,
 *  compare) have no backing in this view: those fixes live on the game page,
 *  so the card opens it. */
export function AtlasAttention({ games, jobs = [], runners = [], onOpenGame, onJobAction, onJob, onAllowFolder, onResetPrefix, onSaveSettings, onOpenRunners }: FixesProps) {
  const { t } = useTranslation(['nav', 'fixes', 'common']);
  const issues = attentionIssues(games, jobs, runners);
  const gameCount = new Set(issues.map((issue) => issue.game.id)).size;
  const { issueText, primaryAction, ghostAction, fixDialogs } = useAttentionFix({
    jobs, runners, onOpenGame, onOpenRunners,
    onRetryJob: (id) => onJobAction?.('retry', id),
    onJob, onAllowFolder, onResetPrefix, onSaveSettings,
  });
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
            const KindIcon = { install: Download, outside: FolderOpen, runner: Wrench, update: ArrowDownToLine, conflict: Cloud, prefix: Wrench, rating: AlertTriangle }[issue.kind];
            const title = issue.kind === 'install' && issue.failedJobKind === 'update'
              ? t('fixes:atlas.issue.updateFailed')
              : ({
              install: t('fixes:atlas.issue.install'), outside: t('fixes:atlas.issue.outside'), runner: t('fixes:atlas.issue.runner'),
              update: t('fixes:atlas.issue.update'), conflict: t('fixes:atlas.issue.conflict'), prefix: t('fixes:atlas.issue.prefix'), rating: t('fixes:atlas.issue.rating'),
              })[issue.kind];
            const primary = primaryAction(issue);
            const ghost = ghostAction(issue);
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
                  <p className="line-clamp-2 text-[13px] leading-[1.45] text-text-muted">{issueText(issue)}</p>
                  {issue.kind === 'install' && issue.error?.trim() && (
                    <RawDetails text={issue.error.trim()} />
                  )}
                </div>
                <div className="grid gap-1">
                  <Button variant="primary" onClick={primary.run}>{primary.label}</Button>
                  {ghost && <Button variant="ghost" onClick={ghost.run}>{ghost.label}</Button>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
      {fixDialogs}
    </div>
  );
}
