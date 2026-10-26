// i18n: converted
import { useTranslation } from 'react-i18next';
import React from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import type { Game, ProtonVersion } from '../types';
import { ATLAS_PAGE, Cover, EmptyState, ListRow, Section, ViewHeader } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { heroImages } from '../lib/games';
import { useVariant } from '../lib/theme';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
  scanning: boolean;
  /** Installed games, for each runner's usage strip (Atlas only). */
  games?: Game[];
}

export function SystemView({ runners, onRefresh, scanning, games }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners']);
  // Atlas renders the /22 Runners page (issue #138); other themes keep the rows below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasRunners runners={runners} games={games ?? []} onRefresh={onRefresh} scanning={scanning} />;
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
function AtlasRunners({ runners, games, onRefresh, scanning }: SystemViewProps & { games: Game[] }) {
  const { t } = useTranslation(['nav', 'runners']);
  const fallback = runners.find((r) => r.is_default) ?? runners[0];
  const uses = (id: string, name: string): Game[] =>
    games.filter((g) => !g.native && (g.settings.runner_id === id || g.settings.runner_id === name));
  return (
    <div className={ATLAS_PAGE}>
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[24px] font-extrabold tracking-tight">{t('nav:title.runners')}</h1>
          <p className="mt-[3px] text-[13px] text-text-muted">{t('runners:atlas.sub')}</p>
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
          <section aria-label={t('runners:atlas.defaultLabel')} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-[18px] border-b border-hairline py-3.5">
            <div className="grid min-w-0 gap-[3px]">
              <h2 className="text-[13px] font-semibold text-text">{t('runners:atlas.defaultLabel')}</h2>
              <p className="text-[0.93em] leading-normal text-text-muted">{t('runners:atlas.defaultDesc')}</p>
            </div>
            {/* The core names the default but offers no call to change it, so
                the picker reads only: a field, not bare text. */}
            <Select value={fallback?.id ?? ''} disabled aria-label={t('runners:atlas.defaultLabel')}>
              <SelectTrigger className="h-8 w-[210px] flex-none bg-surface font-sans text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {runners.map((r) => (
                  <SelectItem key={r.id} value={r.id} className="font-sans text-[13px]">{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </section>
          <section aria-label={t('runners:atlas.installed')}>
            <h2 className="mb-1.5 text-[13px] font-bold text-text-muted">{t('runners:atlas.installed')}</h2>
            <ul className="divide-y divide-hairline">
              {runners.map((r) => {
                const used = uses(r.id, r.name);
                return (
                  <li key={r.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 py-2.5">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-[13px] font-semibold text-text">{r.name}</span>
                        {r.is_default && (
                          <span className="rounded-md bg-surface-strong px-1.5 py-0.5 text-[11.5px] font-semibold text-text-muted">
                            {t('runners:atlas.defaultBadge')}
                          </span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate text-[12px] text-text-faint select-text">{r.path}</div>
                      {used.length > 0 ? (
                        <div className="mt-1.5 flex items-center gap-1.5">
                          <span className="flex items-center" aria-hidden>
                            {used.slice(0, 5).map((g) => (
                              <Cover key={g.id} src={heroImages(g)} title="" className="h-4 w-8 rounded-[4px] ring-1 ring-hairline [&:not(:first-child)]:-ml-1" />
                            ))}
                          </span>
                          <span className="text-[12px] tabular-nums text-text-muted">{t('runners:atlas.gamesCount', { count: used.length })}</span>
                        </div>
                      ) : (
                        <div className="mt-1.5 text-[12px] text-text-faint">{t('runners:atlas.noGames')}</div>
                      )}
                    </div>
                    {/* GetProtonVersions carries no size, and runners are
                        detected, not installed (no remove call): the column
                        reads unknown until the core reports it. */}
                    {r.is_system && <span className="flex-none text-[12px] text-text-faint">{t('runners:badge.system')}</span>}
                  </li>
                );
              })}
            </ul>
          </section>
        </>
      )}
    </div>
  );
}
