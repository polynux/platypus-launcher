// i18n: converted
import { useTranslation } from 'react-i18next';
import React from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import type { ProtonVersion } from '../types';
import { EmptyState, ListRow, Section, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { useVariant } from '../lib/theme';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
  scanning: boolean;
}

export function SystemView({ runners, onRefresh, scanning }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners']);
  // Atlas renders the /22 Runners page (issue #138); other themes keep the rows below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasRunners runners={runners} onRefresh={onRefresh} scanning={scanning} />;
  }
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.runners')}
        subtitle={<><p className="mt-0.5 text-[12.5px] text-text-muted">{t('runners:intro')}</p>
        <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-text-muted">
          {t('runners:about')}
        </p></>}
        actions={<Button variant="neutral" onClick={onRefresh} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? t('runners:scanning') : t('runners:rescan')}
        </Button>}
      />

      {runners.length === 0 ? (
        <EmptyState label={t('runners:emptyTitle')}>
          {t('runners:emptyBody')}
        </EmptyState>
      ) : (
        <Section className="space-y-2">
          {runners.map((r) => (
            <ListRow key={r.id} kind="runner">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-bold">{r.name}</span>
                  {r.is_custom && <Badge variant="gold">{t('runners:badge.custom')}</Badge>}
                  {r.is_system && <Badge variant="neutral">{t('runners:badge.system')}</Badge>}
                </div>
                <div className="truncate font-mono text-hint text-text-muted select-text">{r.path}</div>
              </div>
              <Badge variant="ok" className="flex-none gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-ok" /> {t('runners:badge.found')}
              </Badge>
            </ListRow>
          ))}
        </Section>
      )}
    </div>
  );
}

/** The /22 Runners page (issue #138, Atlas only): the default picker row is
 *  the current default, and each row names the build with its path. The
 *  lab's per-game usage thumbs, sizes and Install/Remove buttons have no
 *  backing in the core (runners are detected, not installed, and each game
 *  picks its own in its settings), so this page lists and rescans only. */
function AtlasRunners({ runners, onRefresh, scanning }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners']);
  const fallback = runners.find((r) => r.is_default) ?? runners[0];
  return (
    <div className="mx-auto grid w-full max-w-6xl content-start gap-6 px-7 py-7">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.runners')}</h1>
          <p className="mt-1 text-[13px] text-text-muted">{t('runners:atlas.sub')}</p>
        </div>
        <Button variant="neutral" onClick={onRefresh} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? t('runners:scanning') : t('runners:rescan')}
        </Button>
      </header>
      {runners.length === 0 ? (
        <EmptyState label={t('runners:emptyTitle')}>{t('runners:emptyBody')}</EmptyState>
      ) : (
        <>
          <section aria-label={t('runners:atlas.defaultLabel')} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 border-b border-hairline pb-4">
            <div>
              <h2 className="text-[13px] font-bold">{t('runners:atlas.defaultLabel')}</h2>
              <p className="mt-0.5 text-[12.5px] text-text-muted">{t('runners:atlas.defaultDesc')}</p>
            </div>
            <span className="rounded-lg bg-surface px-3 py-1.5 text-[13px] font-semibold ring-1 ring-hairline" aria-live="polite">
              {fallback?.name ?? '—'}
            </span>
          </section>
          <section aria-label={t('runners:atlas.installed')}>
            <h2 className="mb-1.5 text-[13px] font-bold text-text-muted">{t('runners:atlas.installed')}</h2>
            <ul className="divide-y divide-hairline">
              {runners.map((r) => (
                <li key={r.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 py-2.5">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[13px] font-bold">{r.name}</span>
                      {r.is_default && (
                        <span className="rounded-md bg-surface-strong px-1.5 py-0.5 text-[11.5px] font-semibold text-text-muted">
                          {t('runners:atlas.defaultBadge')}
                        </span>
                      )}
                      {r.is_custom && <Badge variant="gold">{t('runners:badge.custom')}</Badge>}
                      {r.is_system && <Badge variant="neutral">{t('runners:badge.system')}</Badge>}
                    </div>
                    <div className="mt-0.5 truncate font-mono text-hint text-text-faint select-text">{r.path}</div>
                  </div>
                  <Badge variant="ok" className="flex-none gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-ok" /> {t('runners:badge.found')}
                  </Badge>
                </li>
              ))}
            </ul>
          </section>
        </>
      )}
    </div>
  );
}
