// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useState } from 'react';
import type { Game, Job, ProtonVersion } from '../types';
import { AlertTriangle, ArrowDownToLine, Check, Download, FolderOpen, Info, Wrench } from 'lucide-react';
import { ATLAS_PAGE, Cover, EmptyState, ListRow, Section, SegRow, TierBadge, TweakDot, ViewHeader } from './ui';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { attentionIssues, attentionReason, heroImages, coverImages, needsAttention, storeLabel } from '../lib/games';
import { useVariant } from '../lib/theme';
import { plainError } from '../lib/plainError';

type Filter = 'attention' | 'all';
type FixesProps = {
  games: Game[]; onOpenGame: (id: string) => void; jobs?: Job[]; runners?: ProtonVersion[];
  onJobAction?: (action: 'retry', id: string) => void; onJob?: (kind: 'update', game: Game) => void;
  onAllowFolder?: (game: Game) => void;
  onSaveSettings?: (game: Game, runner: string) => Promise<void>; // i18n:allow: Promise return type
  onOpenRunners?: () => void;
};

export function FixesView({ games, onOpenGame, jobs = [], runners = [], onJobAction, onJob, onAllowFolder, onSaveSettings, onOpenRunners }: FixesProps) {
  // Atlas renders the /22 Needs attention page (issue #138); other themes keep the rows below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasAttention games={games} jobs={jobs} runners={runners} onOpenGame={onOpenGame} onJobAction={onJobAction} onJob={onJob} onAllowFolder={onAllowFolder} onSaveSettings={onSaveSettings} onOpenRunners={onOpenRunners} />;
  }
  return <ClassicFixesView games={games} onOpenGame={onOpenGame} onJobAction={onJobAction} onJob={onJob} onAllowFolder={onAllowFolder} onSaveSettings={onSaveSettings} onOpenRunners={onOpenRunners} />;
}

function ClassicFixesView({ games, onOpenGame, onJobAction, onJob, onAllowFolder, onSaveSettings, onOpenRunners }: FixesProps) {
  const { t } = useTranslation(['nav', 'fixes']);
  const [filter, setFilter] = useState<Filter>('attention');
  const withData = games.filter((g) => g.fix_info || g.compat_badges?.some((b) => b.kind === 'rating'));
  const shown = filter === 'attention' ? withData.filter(needsAttention) : withData;

  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.fixes')}
        subtitle={<><p className="mt-0.5 max-w-[75ch] text-[12.5px] text-text-muted">
          {t('fixes:introWhat')} {t('fixes:introCount', { have: withData.length, total: games.length })}
        </p>
        <p className="mt-1.5 max-w-[75ch] text-[12px] leading-relaxed text-text-muted">
          {t('fixes:about')}
        </p></>}
        actions={<SegRow<Filter>
          label={t('fixes:filter.label')}
          value={filter}
          onChange={setFilter}
          options={[{ value: 'attention', label: t('fixes:filter.attention') }, { value: 'all', label: t('fixes:filter.all') }]}
        />}
      />

      {shown.length === 0 ? (
        <EmptyState label={withData.length ? t('fixes:empty.clearTitle') : t('fixes:empty.noneTitle')}>
          {withData.length
            ? t('fixes:empty.clearBody', { allLabel: t('fixes:filter.all') })
            : t('fixes:empty.noneBody')}
        </EmptyState>
      ) : (
        <Section className="space-y-3">
          {shown.map((g) => {
            const f = g.fix_info;
            const rating = g.compat_badges?.find((b) => b.kind === 'rating');
            return (
              <ListRow key={g.id} kind="fix" as="button" buttonProps={{ onClick: () => onOpenGame(g.id) }} media={<Cover src={coverImages(g)} title={g.title} className="h-24 w-16 flex-none rounded-xl ring-1 ring-hairline" />}>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2.5">
                    {needsAttention(g) && <TweakDot />}
                    <span className="truncate text-[14px] font-bold">{g.title}</span>
                    {(rating?.tone || f?.tier) && <TierBadge tier={rating?.tone || f!.tier} />}
                    <Badge variant="neutral">{storeLabel(g.store)} · {rating?.source || f?.source}</Badge>
                  </div>
                  <p className="mt-1.5 text-[12.5px] leading-relaxed text-text-muted">{needsAttention(g) ? attentionReason(g) : f?.description}</p>
                  {(f?.winetricks?.length || f?.dll_overrides) && (
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {f?.winetricks?.map((t) => <Badge key={t} variant="info" className="normal-case tracking-normal">{t}</Badge>)}
                      {f?.dll_overrides && Object.entries(f.dll_overrides).map(([k, v]) => <Badge key={k} variant="neutral" className="normal-case tracking-normal">{k}={v}</Badge>)}
                    </div>
                  )}
                </div>
              </ListRow>
            );
          })}
        </Section>
      )}
    </div>
  );
}

/** The /22 Needs attention page (issue #138, Atlas only): each game as a
 *  card with art, the rating as the kind line, and Open game. The lab's
 *  per-kind actions (retry install, install runner, move, repair, update,
 *  compare) have no backing in this view: those fixes live on the game page,
 *  so the card opens it. */
function AtlasAttention({ games, jobs = [], runners = [], onOpenGame, onJobAction, onJob, onAllowFolder, onSaveSettings, onOpenRunners }: FixesProps) {
  const { t } = useTranslation(['nav', 'fixes']);
  const issues = attentionIssues(games, jobs, runners);
  const gameCount = new Set(issues.map((issue) => issue.game.id)).size;
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
            const KindIcon = { install: Download, outside: FolderOpen, runner: Wrench, update: ArrowDownToLine, rating: AlertTriangle }[issue.kind];
            const title = issue.kind === 'install' && issue.failedJobKind === 'update'
              ? t('fixes:atlas.issue.updateFailed')
              : ({
              install: t('fixes:atlas.issue.install'), outside: t('fixes:atlas.issue.outside'), runner: t('fixes:atlas.issue.runner'),
              update: t('fixes:atlas.issue.update'), rating: t('fixes:atlas.issue.rating'),
              })[issue.kind];
            const fix = issue.kind === 'runner'
              ? t('fixes:atlas.useDefaultRunner', { runner: runners.find((r) => r.is_default)?.name ?? t('fixes:atlas.defaultRunner') })
              : ({
                install: t('fixes:atlas.fix.install'), outside: t('fixes:atlas.fix.outside'), runner: t('fixes:atlas.fix.runner'),
                update: t('fixes:atlas.fix.update'), rating: t('fixes:atlas.fix.rating'),
              })[issue.kind];
            const issueJob = jobs.filter((j) => j.game_id === g.id && j.state === 'failed' && (j.kind === 'install' || j.kind === 'update')).sort((a, b) => b.updated_at - a.updated_at)[0];
            const fallbackRunner = runners.find((r) => r.is_default);
            const action = () => {
              if (issue.kind === 'install' && issueJob) onJobAction?.('retry', issueJob.id);
              else if (issue.kind === 'outside') onAllowFolder?.(g);
              else if (issue.kind === 'runner' && fallbackRunner) onSaveSettings?.(g, fallbackRunner.id);
              else if (issue.kind === 'update') onJob?.('update', g);
              else onOpenGame(g.id);
            };
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
                  <p className="line-clamp-2 text-[12.5px] leading-[1.45] text-text-muted">{issue.kind === 'install' ? plainError(issue.error, t('fixes:atlas.reason.install')) : issue.kind === 'outside' ? t('fixes:atlas.reason.outside', { title: g.title, dir: g.install_dir }) : issue.kind === 'runner' ? t('fixes:atlas.reason.runner', { title: g.title, runner: issue.runnerId }) : issue.kind === 'update' ? t('fixes:atlas.reason.update') : attentionReason(g)}</p>
                </div>
                <div className="grid gap-1">
                  <Button variant="primary" onClick={action}>{fix}</Button>
                  {issue.kind !== 'rating' && <Button variant="ghost" onClick={() => issue.kind === 'runner' ? onOpenRunners?.() : onOpenGame(g.id)}>{issue.kind === 'runner' ? t('fixes:atlas.openRunners') : t('fixes:atlas.openGame')}</Button>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
