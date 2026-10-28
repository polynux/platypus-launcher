// i18n: converted
import { useTranslation } from 'react-i18next';
import React, { useEffect, useState } from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import { Effect } from 'effect';
import type { Game, ProtonVersion } from '../types';
import { LauncherApi } from '../effect/runtime';
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
  /** Makes a runner the launcher's default (issue #146): what "auto" runs. */
  onDefaultRunner?: (id: string) => void;
}

/** The picker's value for "no default": Radix needs a non-empty item value. */
const AUTOMATIC = 'automatic';

export function SystemView({ runners, onRefresh, scanning, games, onDefaultRunner }: SystemViewProps) {
  const { t } = useTranslation(['nav', 'runners']);
  // The stored pick (issue #146): "" is the automatic one, null is not read yet.
  // The runner list names the runner "auto" runs, not whether the user picked
  // it, so the page asks the core for the setting and can offer a way back.
  const [picked, setPicked] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => live && setPicked(s.default_runner_id ?? ''))
      .catch(() => {});
    return () => { live = false; };
  }, []);
  const choose = (id: string) => {
    setPicked(id);
    onDefaultRunner?.(id);
  };
  // Atlas renders the /22 Runners page (issue #138); other themes keep the rows below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasRunners runners={runners} games={games ?? []} onRefresh={onRefresh} scanning={scanning} picked={picked} onDefaultRunner={onDefaultRunner ? choose : undefined} />;
  }
  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.runners')}
        subtitle={<><p className="mt-0.5 text-[12.5px] text-text-muted">{t('runners:intro')}</p>
        <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-text-muted">
          {t('runners:about')}
        </p></>}
        actions={<>
          {onDefaultRunner && picked ? (
            <Button variant="ghost" onClick={() => choose('')}>{t('runners:useAutomatic')}</Button>
          ) : null}
          <Button variant="neutral" onClick={onRefresh} disabled={scanning}>
            {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
            {scanning ? t('runners:scanning') : t('runners:rescan')}
          </Button>
        </>}
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
                  {r.is_default && <Badge variant="neutral">{t('runners:badge.default')}</Badge>}
                </div>
                <div className="truncate font-mono text-hint text-text-muted select-text">{r.path}</div>
              </div>
              <div className="flex flex-none items-center gap-2">
                {onDefaultRunner && !r.is_default && (
                  <Button variant="ghost" size="sm" onClick={() => onDefaultRunner(r.id)}>{t('runners:makeDefault')}</Button>
                )}
                <Badge variant="ok" className="flex-none gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full bg-ok" /> {t('runners:badge.found')}
                </Badge>
              </div>
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
 *  picks its own in its settings), so this page lists, rescans and picks the
 *  default for Automatic games only. */
function AtlasRunners({ runners, games, onRefresh, scanning, picked, onDefaultRunner }: SystemViewProps & { games: Game[]; picked: string | null }) {
  const { t } = useTranslation(['nav', 'runners']);
  const fallback = runners.find((r) => r.is_default) ?? runners[0];
  // A default that is no longer installed runs the automatic pick, so the
  // picker says so rather than showing a runner the list doesn't have.
  const stored = picked && runners.some((r) => r.id === picked) ? picked : '';
  const shown = picked === null ? (fallback?.id ?? '') : stored === '' ? AUTOMATIC : stored;
  const uses = (id: string, name: string): Game[] =>
    games.filter((g) => !g.native && (g.settings.runner_id === id || g.settings.runner_id === name || ((!g.settings.runner_id || g.settings.runner_id === 'auto') && fallback?.id === id)));
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
            {/* The core names the default and takes the pick (issue #146). */}
            <Select
              value={shown}
              onValueChange={(id) => onDefaultRunner?.(id === AUTOMATIC ? '' : id)}
              disabled={!onDefaultRunner}
              aria-label={t('runners:atlas.defaultLabel')}
            >
              <SelectTrigger className="h-8 w-[210px] flex-none bg-surface font-sans text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={AUTOMATIC} className="font-sans text-[13px]">{t('runners:atlas.automatic')}</SelectItem>
                {runners.map((r) => (
                  <SelectItem key={r.id} value={r.id} className="font-sans text-[13px]">{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </section>
          <section aria-label={t('runners:atlas.installed')}>
            <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('runners:atlas.installed')}</h2>
            <ul className="divide-y divide-hairline">
              {runners.map((r) => {
                const used = uses(r.id, r.name);
                return (
                  <li key={r.id} className="grid min-h-14 grid-cols-[minmax(200px,1.3fr)_minmax(160px,1fr)_70px_130px] items-center gap-3.5 border-b border-hairline">
                    <div className="min-w-0 flex flex-wrap items-center gap-2">
                      <div className="flex basis-full items-center gap-2"><span className="basis-auto text-[13px] font-bold text-text">{r.name}</span>
                        {r.is_default && (
                          <span className="rounded-md bg-surface-strong px-1.5 py-0.5 text-[11.5px] font-semibold text-text-muted">
                            {t('runners:atlas.defaultBadge')}
                          </span>
                        )}
                      </div>
                      <div className="basis-full truncate text-[12.5px] text-text-faint select-text">{r.path}</div>
                    </div>
                    <div className="flex min-w-0 items-center gap-1">
                      {used.length > 0 ? (
                        <>
                          <span className="flex items-center gap-1" aria-hidden>
                            {used.slice(0, 5).map((g) => (
                              <Cover key={g.id} src={heroImages(g)} title="" className="h-5 w-[42px] rounded" />
                            ))}
                          </span>
                          <span className="ml-1.5 text-[12.5px] text-text-faint">{t('runners:atlas.gamesCount', { count: used.length })}</span>
                        </>
                      ) : (
                        <span className="text-[12.5px] text-text-faint">{t('runners:atlas.noGames')}</span>
                      )}
                    </div>
                    <span className="text-right tabular-nums text-text-faint">—</span>
                    {/* GetProtonVersions carries no size, and runners are
                        detected, not installed (no remove call): the column
                        reads unknown until the core reports it. */}
                    {r.is_system && <span className="text-[12.5px] text-text-faint">{t('runners:atlas.fromSystem')}</span>}
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
