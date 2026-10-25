// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useState } from 'react';
import type { Game } from '../types';
import { AlertTriangle, Check, Info } from 'lucide-react';
import { Cover, EmptyState, ListRow, Section, SegRow, TierBadge, TweakDot, ViewHeader } from './ui';
import { Badge } from './ds/badge';
import { Button } from './ds/button';
import { attentionReason, coverImages, needsAttention, storeLabel } from '../lib/games';
import { useVariant } from '../lib/theme';

type Filter = 'attention' | 'all';

export function FixesView({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation(['nav', 'fixes']);
  // Atlas renders the /22 Needs attention page (issue #138); other themes keep the rows below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasAttention games={games} onOpenGame={onOpenGame} />;
  }
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
function AtlasAttention({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation(['nav', 'fixes']);
  const issues = games.filter(needsAttention);
  return (
    <div className="mx-auto grid w-full max-w-6xl content-start gap-6 px-7 py-7">
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{t('fixes:atlas.title')}</h1>
        <p className="mt-1 text-[13px] text-text-muted">
          {issues.length > 0 ? t('fixes:atlas.sub', { count: issues.length }) : t('fixes:atlas.emptySub')}
        </p>
      </header>
      {issues.length === 0 ? (
        <EmptyState icon={Check} label={t('fixes:atlas.emptyTitle')}>{t('fixes:atlas.emptyBody')}</EmptyState>
      ) : (
        <ul className="grid gap-2.5">
          {issues.map((g) => {
            const rating = g.compat_badges?.find((b) => b.kind === 'rating');
            const isFix = !!g.fix_info;
            const kind = rating?.tone || g.fix_info?.tier
              ? t('fixes:atlas.kindRated', { source: rating?.source || g.fix_info?.source })
              : t('fixes:atlas.kindFix');
            const KindIcon = isFix ? AlertTriangle : Info;
            return (
              <li key={g.id} className="grid grid-cols-[116px_minmax(0,1fr)_auto] items-center gap-4 rounded-xl bg-surface p-3 ring-1 ring-hairline">
                <button type="button" onClick={() => onOpenGame(g.id)} className="block overflow-hidden rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus" aria-label={g.title}>
                  <Cover src={coverImages(g)} title={g.title} className="aspect-[116/64] w-full" />
                </button>
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5 text-[12px] font-semibold text-attention">
                    <KindIcon className="h-3 w-3 flex-none" aria-hidden />
                    <span className="truncate">{kind}</span>
                  </div>
                  <button type="button" onClick={() => onOpenGame(g.id)} className="truncate text-left text-[15px] font-bold hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                    {g.title}
                  </button>
                  <p className="mt-0.5 line-clamp-2 text-[12.5px] leading-relaxed text-text-muted">{attentionReason(g)}</p>
                </div>
                <Button variant="primary" onClick={() => onOpenGame(g.id)}>
                  {t('fixes:atlas.openGame')}
                </Button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
