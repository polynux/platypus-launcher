// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useState } from 'react';
import type { Game } from '../types';
import { Cover, EmptyState, ListRow, Section, SegRow, TierBadge, TweakDot, ViewHeader } from './ui';
import { Badge } from './ds/badge';
import { attentionReason, coverImages, needsAttention, storeLabel } from '../lib/games';

type Filter = 'attention' | 'all';

export function FixesView({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation(['nav', 'fixes']);
  const [filter, setFilter] = useState<Filter>('attention');
  const withData = games.filter((g) => g.fix_info || g.compat_badges?.some((b) => b.kind === 'rating'));
  const shown = filter === 'attention' ? withData.filter(needsAttention) : withData;

  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.fixes')}
        subtitle={<><p className="mt-0.5 max-w-[75ch] text-[12.5px] text-white/70">
          {t('fixes:introWhat')} {t('fixes:introCount', { have: withData.length, total: games.length })}
        </p>
        <p className="mt-1.5 max-w-[75ch] text-[12px] leading-relaxed text-white/70">
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
              <ListRow key={g.id} kind="fix" as="button" buttonProps={{ onClick: () => onOpenGame(g.id) }} media={<Cover src={coverImages(g)} title={g.title} className="h-24 w-16 flex-none rounded-xl ring-1 ring-white/10" />}>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2.5">
                    {needsAttention(g) && <TweakDot />}
                    <span className="truncate text-[14px] font-bold">{g.title}</span>
                    {(rating?.tone || f?.tier) && <TierBadge tier={rating?.tone || f!.tier} />}
                    <Badge variant="neutral">{storeLabel(g.store)} · {rating?.source || f?.source}</Badge>
                  </div>
                  <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">{needsAttention(g) ? attentionReason(g) : f?.description}</p>
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
