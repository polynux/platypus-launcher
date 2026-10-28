// i18n: converted
import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import type { Game, Job, ProtonVersion } from '../types';
import { Cover, EmptyState, ListRow, Section, SegRow, TierBadge, TweakDot, ViewHeader } from './ui';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { Modal } from './Modal';
import { attentionReason, coverImages, needsAttention, storeLabel } from '../lib/games';
import { useVariant } from '../lib/theme';
import { AtlasAttention } from './AtlasAttention';

type Filter = 'attention' | 'all';
export type FixesProps = {
  games: Game[]; onOpenGame: (id: string) => void; jobs?: Job[]; runners?: ProtonVersion[];
  onJobAction?: (action: 'retry', id: string) => void; onJob?: (kind: 'update', game: Game) => void;
  onAllowFolder?: (game: Game) => void;
  onResetPrefix?: (game: Game) => void;
  onSaveSettings?: (game: Game, runner: string) => Promise<void>; // i18n:allow: Promise return type
  onOpenRunners?: () => void;
};

export function FixesView({ games, onOpenGame, jobs = [], runners = [], onJobAction, onJob, onAllowFolder, onResetPrefix, onSaveSettings, onOpenRunners }: FixesProps) {
  // One branch per view root (issue #156): the fixes presentation, not the shell.
  const presentation = useVariant('fixes.presentation');
  if (presentation === 'cards') {
    return <AtlasAttention games={games} jobs={jobs} runners={runners} onOpenGame={onOpenGame} onJobAction={onJobAction} onJob={onJob} onAllowFolder={onAllowFolder} onResetPrefix={onResetPrefix} onSaveSettings={onSaveSettings} onOpenRunners={onOpenRunners} />;
  }
  return <ClassicFixesView games={games} onOpenGame={onOpenGame} onJobAction={onJobAction} onJob={onJob} onAllowFolder={onAllowFolder} onResetPrefix={onResetPrefix} onSaveSettings={onSaveSettings} onOpenRunners={onOpenRunners} />;
}

function ClassicFixesView({ games, onOpenGame, onJobAction, onJob, onAllowFolder, onResetPrefix, onSaveSettings, onOpenRunners }: FixesProps) {
  const { t } = useTranslation(['nav', 'fixes', 'common']);
  const [filter, setFilter] = useState<Filter>('attention');
  // Resetting a prefix asks first (the same words as the Atlas page: the
  // dialog is the fix's, not the presentation's).
  const [resetTarget, setResetTarget] = useState<Game | null>(null);
  const withData = games.filter((g) => g.fix_info || g.compat_badges?.some((b) => b.kind === 'rating') || g.has_save_conflict || g.prefix_damaged);
  const shown = filter === 'attention' ? withData.filter(needsAttention) : withData;
  const canResetPrefix = (g: Game) => !!g.prefix_damaged && !g.native && !g.client_launch;

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
              <div key={g.id} className="flex items-center gap-2">
                <div className="min-w-0 flex-1">
                  <ListRow kind="fix" as="button" buttonProps={{ onClick: () => onOpenGame(g.id) }} media={<Cover src={coverImages(g)} title={g.title} className="h-24 w-16 flex-none rounded-xl ring-1 ring-hairline" />}>
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
                </div>
                {canResetPrefix(g) && (
                  <Button variant="neutral" size="sm" onClick={() => setResetTarget(g)}>{t('fixes:atlas.fix.prefix')}</Button>
                )}
              </div>
            );
          })}
        </Section>
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
