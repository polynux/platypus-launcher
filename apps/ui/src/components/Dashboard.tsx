// i18n: converted
import React from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import type { ActivityEvent, Game, ProtonVersion, ThemeLayout as DashboardLayoutData, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { Cover, EmptyState, Panel, TierBadge, TweakDot, cardClass, cx, relTime } from './ui';
import { CompatChip, GameCardBody } from './CompatBadges';
import { GameTile } from './GameTile';
import { Hero, heroChipClass } from './Hero';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { X, Play } from 'lucide-react';
import { byRecent, coverImages, heroImages, needsAttention, storeLabel as storeLabelFallback } from '../lib/games';
import { formatHours } from '../lib/format';
import { runnerShort } from '../lib/settings';
import { sortKey } from '../lib/customize';
import { ratingBadge } from '../lib/compat';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';

const byTitle = (a: Game, b: Game) => sortKey(a).localeCompare(sortKey(b)) || a.id.localeCompare(b.id);

interface DashboardProps {
  games: Game[];
  runners: ProtonVersion[];
  activity: ActivityEvent[];
  /** Library-wide owned count (installed + store-only). Defaults to games.length. */
  ownedCount?: number;
  /** The full library (installed + not): Home's "to install" list. Defaults to games. */
  allGames?: Game[];
  dismissedHero: ReadonlySet<string>;
  onDismissHero: (ids: string[]) => void;
  onOpenGame: (id: string) => void;
  onNavigate: (v: View) => void;
}

export function Dashboard({ games, ownedCount, allGames, ...rest }: DashboardProps) {
  // Atlas renders the /22 Home page (issue #138); other themes keep the
  // layout blocks below.
  if (useVariant('shell.navigation') === 'sourcebar') {
    return <AtlasHome games={games} allGames={allGames ?? games} ownedCount={ownedCount} {...rest} />;
  }
  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's dashboard before the core answers over RPC.
  const layout = useThemeLayout('dashboard') ?? BUILTIN_LAYOUTS['dashboard'];
  return (
    <div className="p-7">
      <ThemeLayout
        layout={layout}
        blocks={dashboardBlocks({ games, ownedCount, ...rest }, layout)}
        className="grid gap-5 xl:grid-cols-[1.75fr_1fr]"
        regionClassName={dashboardRegionClass}
      />
    </div>
  );
}

/** One region's frame in a dashboard layout (issue #108): the top banner and
 *  the bottom feed span the full width, the main column breathes like the
 *  built-in page and the side column packs tighter. The top and bottom
 *  margins make up the wrapper's gap, so the built-in spacing is exact. */
function dashboardRegionClass(region: string): string {
  switch (region) {
    case 'top': return 'col-span-full min-w-0 space-y-6 mb-1';
    case 'bottom': return 'col-span-full min-w-0 mt-1';
    case 'side': return 'min-w-0 space-y-5';
    default: return 'min-w-0 space-y-6';
  }
}

// The dashboard's layout blocks (issue #96): each takes its data as props
// only (it all comes from App) and renders null when it has nothing to show,
// so a layout orders blocks without conditionals of its own.
function dashboardBlocks({ games, runners, activity, ownedCount, dismissedHero, onDismissHero, onOpenGame, onNavigate }: DashboardProps, layout?: DashboardLayoutData): Record<string, ReactNode> {
  const waiting = games.filter(needsAttention);
  const pending = waiting.filter((g) => !dismissedHero.has(g.id));
  const hero = pending[0];
  const installed = games.length;
  const owned = Math.max(ownedCount ?? installed, installed);
  const hasPlayData = games.some((g) => g.last_played_at);
  // Without play data byRecent has nothing to order by, so fall back to a stable title sort.
  const continueGames = [...games].sort(hasPlayData ? byRecent : byTitle).slice(0, 8);
  const recentlyAdded = games.filter((g) => g.store === 'custom').slice(0, 5);
  return {
    hero: (
      <DashboardHero
        hero={hero}
        installed={installed}
        owned={owned}
        waitingCount={waiting.length}
        pending={pending}
        onOpenGame={onOpenGame}
        onDismissHero={onDismissHero}
      />
    ),
    welcome: <DashboardWelcome hero={hero} installed={installed} owned={owned} fixes={waiting.length} />,
    empty: <DashboardEmpty games={games} />,
    attention: (
      <DashboardAttention
        hero={hero}
        installed={installed}
        owned={owned}
        waitingCount={waiting.length}
        pending={pending}
        onOpenGame={onOpenGame}
        onDismissHero={onDismissHero}
      />
    ),
    continue: (
      <DashboardContinue
        games={continueGames}
        runners={runners}
        hasPlayData={hasPlayData}
        onOpenGame={onOpenGame}
        onNavigate={onNavigate}
      />
    ),
    waiting: <DashboardWaiting waiting={waiting} onOpenGame={onOpenGame} onNavigate={onNavigate} />,
    recent: <DashboardRecent games={recentlyAdded} onOpenGame={onOpenGame} />,
    activity: <DashboardActivity events={activity} limit={layoutLimit(layout, 'activity', 8)} onNavigate={onNavigate} />,
  };
}

/** The layout's limit for a block, or the built-in default without one. The
 *  core validated the number; the UI parses it again so a bad bridge value
 *  can't widen a feed. */
function layoutLimit(layout: DashboardLayoutData | undefined, block: string, fallback: number): number {
  const nodes = layout?.regions.flatMap((r) => r.nodes) ?? [];
  for (const node of nodes) {
    for (const child of node.row ?? [node]) {
      const raw = child.block === block ? child.attrs?.limit : undefined;
      const parsed = raw === undefined ? NaN : Number.parseInt(raw, 10);
      if (Number.isFinite(parsed) && parsed > 0) return parsed;
    }
  }
  return fallback;
}

export function DashboardHero({ hero, installed, owned, waitingCount, pending, onOpenGame, onDismissHero }: {
  hero: Game | undefined;
  installed: number;
  owned: number;
  waitingCount: number;
  pending: Game[];
  onOpenGame: (id: string) => void;
  onDismissHero: (ids: string[]) => void;
}) {
  if (!hero) return null;
  return (
    <Hero site="dashboard" art={heroImages(hero)} title={hero.title}>
      <DashboardAttention
        hero={hero}
        installed={installed}
        owned={owned}
        waitingCount={waitingCount}
        pending={pending}
        onOpenGame={onOpenGame}
        onDismissHero={onDismissHero}
      />
    </Hero>
  );
}

// Today's dashboard hero content (catalog dashboard.attention): why the game
// needs attention, what to do about it, and the stat chips. The dashboard's
// hero block shows it inside the Hero frame; a layout may also place it on
// its own. Positioning is structural (as today): the frame is the relative
// parent.
export function DashboardAttention({ hero, installed, owned, waitingCount, pending, onOpenGame, onDismissHero }: {
  hero: Game | undefined;
  installed: number;
  owned: number;
  waitingCount: number;
  pending: Game[];
  onOpenGame: (id: string) => void;
  onDismissHero: (ids: string[]) => void;
}) {
  const { t } = useTranslation(['dashboard', 'common']);
  const chips = useVariant('hero.chips');
  if (!hero) return null;
  return (
    <>
      <div className="absolute bottom-6 left-7 max-w-lg">
        <Badge variant="bronze" className="bg-attention text-on-accent">
          {['bronze', 'borked'].includes(ratingBadge(hero)?.tone ?? '') ? t('dashboard:hero.needsAttention') : t('dashboard:hero.fixAvailable')}
        </Badge>
        <h1 className="mt-2.5 text-[27px] font-extrabold leading-tight tracking-tight">
          {['bronze', 'borked'].includes(ratingBadge(hero)?.tone ?? '')
            ? t('dashboard:hero.titleTweaks', { title: hero.title })
            : t('dashboard:hero.titleFix', { title: hero.title })}
        </h1>
        <p className="mt-1.5 line-clamp-2 text-[12.5px] leading-relaxed text-text-body">{attentionReason(hero, t)}</p>
        <div className="mt-3.5 flex gap-2.5">
          <Button variant="primary" onClick={() => onOpenGame(hero.id)}>{t('dashboard:hero.openGame')}</Button>
          <Button variant="neutral" onClick={() => onDismissHero([hero.id])}>{t('common:later')}</Button>
          {pending.length > 1 && (
            <Button variant="ghost" onClick={() => onDismissHero(pending.map((g) => g.id))}>
              {t('dashboard:hero.hideAll', { count: pending.length })}
            </Button>
          )}
        </div>
      </div>
      <div className="absolute bottom-6 right-7 hidden gap-3 lg:flex">
        {[
          [t('dashboard:hero.statInstalled'), String(installed), 'text-accent-strong'],
          ...(owned > installed ? [[t('dashboard:hero.statOwned'), String(owned), 'text-text-muted']] : []),
          [t('dashboard:hero.statFixes'), String(waitingCount), 'text-attention'],
        ].map(([k, v, c]) => (
          <div key={k} className={cx(heroChipClass(chips), 'w-28 rounded-2xl p-3.5')}>
            <div className={cx('text-[19px] font-extrabold tabular-nums', c)}>{v}</div>
            <div className="font-mono text-micro uppercase tracking-widest text-text-muted">{k}</div>
          </div>
        ))}
      </div>
    </>
  );
}

export function DashboardWelcome({ hero, installed, owned, fixes }: {
  hero?: Game | undefined;
  installed: number;
  owned: number;
  fixes: number;
}) {
  const { t } = useTranslation('dashboard');
  // The greeting stands in for the hero: with something to attend to, the
  // hero takes the banner's place.
  if (hero) return null;
  const hour = new Date().getHours();
  const greeting = hour < 5
    ? t('dashboard:greeting.night')
    : hour < 12
      ? t('dashboard:greeting.morning')
      : hour < 18
        ? t('dashboard:greeting.afternoon')
        : t('dashboard:greeting.evening');
  return (
    <Panel as="section" className="p-6">
      <h1 className="text-[25px] font-extrabold tracking-tight">{greeting}</h1>
      <p className="mt-1 font-mono text-[12px] tabular-nums text-text-muted">
        {t('dashboard:welcome.installed', { count: installed })}
        {owned > installed ? ` · ${t('dashboard:welcome.owned', { count: owned })}` : ''} · {t('dashboard:welcome.fixes', { count: fixes })}
      </p>
    </Panel>
  );
}

export function DashboardEmpty({ games }: { games: Game[] }) {
  const { t } = useTranslation(['dashboard', 'common']);
  if (games.length > 0) return null;
  return (
    <EmptyState label={t('dashboard:empty.label')}>
      {t('dashboard:empty.detail')}
    </EmptyState>
  );
}

export function DashboardContinue({ games, runners, hasPlayData, onOpenGame, onNavigate }: {
  games: Game[];
  runners: ProtonVersion[];
  hasPlayData: boolean;
  onOpenGame: (id: string) => void;
  onNavigate: (v: View) => void;
}) {
  const { t } = useTranslation(['dashboard', 'common']);
  // One root: rows count a block as one column, so a fragment here would
  // split the header and the grid into separate columns.
  return (
    <section>
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">{hasPlayData ? t('dashboard:continuePlaying') : t('dashboard:yourGames')}</h2>
        <Button variant="link" onClick={() => onNavigate({ name: 'library' })}>{t('dashboard:fullLibrary')}</Button>
      </div>
      <div className="cover-grid items-stretch">
        {games.map((g) => (
          <GameTile
            key={g.id}
            game={g}
            meta={`${g.playtime_minutes ? `${formatHours(g.playtime_minutes)} · ` : ''}${runnerShort(g.settings.runner_id, runners)}`}
            hideNative
            onOpen={onOpenGame}
          />
        ))}
      </div>
    </section>
  );
}

export function DashboardWaiting({ waiting, onOpenGame, onNavigate }: {
  waiting: Game[];
  onOpenGame: (id: string) => void;
  onNavigate: (v: View) => void;
}) {
  const { t } = useTranslation('dashboard');
  return (
    <div>
      <h2 className="mb-2.5 text-[15px] font-bold">{t('dashboard:waiting')}</h2>
      {waiting.length === 0 ? (
        <p className="rounded-2xl bg-surface p-4 text-[12px] text-text-body ring-1 ring-hairline">
          {t('dashboard:waitingEmpty')}
        </p>
      ) : (
        <div className="space-y-3">
          {waiting.slice(0, 3).map((g) => (
            <button
              key={g.id}
              onClick={() => onOpenGame(g.id)}
              className="block w-full rounded-2xl bg-surface p-4 text-left ring-1 ring-hairline transition hover:ring-attention/40"
            >
              <div className="flex items-center justify-between gap-3">
                <span className="truncate text-[13px] font-bold">{g.title}</span>
                {(ratingBadge(g)?.tone || g.fix_info?.tier) && <TierBadge tier={ratingBadge(g)?.tone || g.fix_info!.tier} />}
              </div>
              <p className="mt-1 line-clamp-2 text-[12px] leading-relaxed text-text-body">{attentionReason(g, t)}</p>
            </button>
          ))}
          {waiting.length > 3 && (
            <Button variant="link" onClick={() => onNavigate({ name: 'fixes' })}>
              {t('dashboard:moreInFixes', { count: waiting.length - 3 })}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

export function DashboardRecent({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation('dashboard');
  if (games.length === 0) return null;
  return (
    <Panel as="section" className="bg-surface">
      <h2 className="mb-2.5 text-[14px] font-bold">{t('dashboard:recentlyAdded')}</h2>
      <RecentList games={games} onOpenGame={onOpenGame} />
    </Panel>
  );
}

export function DashboardActivity({ events, limit = 8, onNavigate }: {
  events: ActivityEvent[];
  limit?: number;
  onNavigate: (v: View) => void;
}) {
  return <ActivityPanel events={events} limit={limit} onFullLog={() => onNavigate({ name: 'activity' })} />;
}

/** Why a game needs attention, in words. Curated recipes speak for
 *  themselves; rating explanations name the badge's source. */
function attentionReason(g: Game, t: TFunction): string {
  const f = g.fix_info;
  if (f?.description) return f.description;
  const badge = ratingBadge(g);
  if (badge?.tone !== 'bronze' && badge?.tone !== 'borked') return '';
  return t(badge.tone === 'borked' ? 'dashboard:reason.borked' : 'dashboard:reason.bronze', { source: badge.source });
}

/** The store of a game, in words. The shared helper stays as is for the
 *  areas that still render it; the dashboard reads the catalog so the one
 *  coined word ("Sideloaded") follows the UI language. */
function storeLabel(store: string, t: TFunction): string {
  switch (store) {
    case 'steam': return t('dashboard:store.steam');
    case 'gog': return t('dashboard:store.gog');
    case 'epic': return t('dashboard:store.epic');
    case 'ubisoft': return t('dashboard:store.ubisoft');
    case 'custom': return t('dashboard:store.custom');
    default: return storeLabelFallback(store);
  }
}

export function RecentList({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation('dashboard');
  // The recent rows share the game-row density: warm (full) is today's row,
  // compact tightens the row and the cover.
  const mode = useVariant('game-row.mode');
  const compact = mode === 'compact';
  return (
    <ul className="divide-y divide-hairline">
      {games.map((g) => (
        <li key={g.id}>
          <button onClick={() => onOpenGame(g.id)} className={compact ? '-mx-1 flex w-full items-center gap-2.5 rounded-lg px-1 py-1 text-left hover:bg-surface' : '-mx-1 flex w-full items-center gap-3 rounded-lg px-1 py-2 text-left hover:bg-surface'}>
            <Cover src={coverImages(g)} title="" className={compact ? 'h-7 w-7 flex-none rounded ring-1 ring-hairline' : 'h-8 w-8 flex-none rounded ring-1 ring-hairline'} />
            <div className="min-w-0 flex-1 leading-tight">
              <div className="truncate text-[12px] font-semibold">{g.title}</div>
              <div className="font-mono text-micro uppercase tracking-wider text-text-muted">{storeLabel(g.store, t)}</div>
              {!g.native && <CompatChip game={g} className="mt-1" />}
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}

/** The /22 Home page (issue #138, Atlas only): greeting, the suggestion card
 *  (the first game needing attention, dismissable), Continue playing cards
 *  with Play, the Recently added strip and the Waiting for you lists. Same
 *  data and handlers as the layout blocks above; only the markup is /22.
 *  The Play button opens the game page, where Play lives: Home gets no new
 *  launch wiring of its own. */
function AtlasHome({ games, allGames, activity, dismissedHero, onDismissHero, onOpenGame, onNavigate }: DashboardProps & { allGames: Game[] }) {
  const { t } = useTranslation(['dashboard', 'common', 'game', 'activity']);
  const waiting = games.filter(needsAttention);
  const pending = waiting.filter((g) => !dismissedHero.has(g.id));
  const suggested = pending[0];
  const installed = games.length;
  const hasPlayData = games.some((g) => g.last_played_at);
  const continueGames = [...games].sort(hasPlayData ? byRecent : byTitle).slice(0, 3);
  const recentlyAdded = games.filter((g) => g.store === 'custom').slice(0, 5);
  const toFinish = [...games].filter((g) => (g.playtime_minutes ?? 0) > 0).sort(byRecent).slice(0, 5);
  const toInstall = allGames.filter((g) => !g.is_installed).slice(0, 5);
  const hour = new Date().getHours();
  const greeting = hour < 5
    ? t('dashboard:greeting.night')
    : hour < 12
      ? t('dashboard:greeting.morning')
      : hour < 18
        ? t('dashboard:greeting.afternoon')
        : t('dashboard:greeting.evening');
  if (games.length === 0) {
    return (
      <div className="mx-auto w-full max-w-6xl px-7 py-7">
        <DashboardEmpty games={games} />
      </div>
    );
  }
  return (
    <div className="mx-auto grid w-full max-w-6xl content-start gap-6 px-7 py-7">
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{greeting}, polynux</h1>
        <p className="mt-1 text-[13px] text-text-muted">{t('dashboard:home.ready', { count: installed })}</p>
      </header>
      {suggested && (
        <Hero site="dashboard" art={heroImages(suggested)} title={suggested.title}>
          <div className="absolute left-7 top-6 max-w-md">
            <span className="font-mono text-micro uppercase tracking-widest text-text-muted">{t('dashboard:home.fromBacklog')}</span>
            <h2 className="mt-1 text-[22px] font-extrabold tracking-tight text-white">{suggested.title}</h2>
            <p className="mt-1 line-clamp-2 text-[12.5px] leading-relaxed text-white/85">{attentionReason(suggested, t)}</p>
            <div className="mt-3 flex gap-2.5">
              <Button variant="primary" onClick={() => onOpenGame(suggested.id)}>{t('dashboard:hero.openGame')}</Button>
            </div>
          </div>
          <button
            type="button"
            onClick={() => onDismissHero([suggested.id])}
            aria-label={t('dashboard:home.dismiss')}
            className="absolute right-4 top-4 grid size-8 place-items-center rounded-lg bg-glass text-text ring-1 ring-hairline transition hover:bg-surface-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <X className="size-4" aria-hidden />
          </button>
        </Hero>
      )}
      <section aria-label={t('dashboard:continuePlaying')}>
        <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{hasPlayData ? t('dashboard:continuePlaying') : t('dashboard:yourGames')}</h2>
        <ul className="grid gap-3.5 md:grid-cols-3">
          {continueGames.map((g) => (
            <li key={g.id}>
              <button type="button" onClick={() => onOpenGame(g.id)} aria-label={t('dashboard:hero.openGame') + ': ' + g.title} className="block w-full overflow-hidden rounded-xl text-left ring-1 ring-hairline transition hover:ring-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                <Cover src={heroImages(g)} title={g.title} className="aspect-[16/7] w-full" />
              </button>
              <div className="mt-2 flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="truncate text-[13px] font-bold">{g.title}</div>
                  <div className="truncate text-[12px] text-text-faint">
                    {g.last_played_at ? `${relTime(g.last_played_at)} · ` : ''}{g.playtime_minutes ? formatHours(g.playtime_minutes) : ''}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => onOpenGame(g.id)}
                  aria-label={`${t('game:play.play')}: ${g.title}`}
                  className="inline-flex h-[34px] flex-none items-center gap-1.5 rounded-lg bg-accent px-3.5 text-[13px] font-bold text-on-accent transition hover:brightness-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  <Play className="size-3.5 fill-current" aria-hidden />
                  {t('game:play.play')}
                </button>
              </div>
            </li>
          ))}
        </ul>
      </section>
      <div className="grid gap-6 lg:grid-cols-2">
        {recentlyAdded.length > 0 && (
          <section aria-label={t('dashboard:recentlyAdded')}>
            <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('dashboard:recentlyAdded')}</h2>
            <ul className="grid grid-cols-5 gap-2.5">
              {recentlyAdded.map((g) => (
                <li key={g.id}>
                  <button type="button" onClick={() => onOpenGame(g.id)} className="block w-full text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus rounded-lg">
                    <Cover src={coverImages(g)} title={g.title} className="aspect-[2/3] w-full rounded-lg ring-1 ring-hairline" />
                    <span className="mt-1.5 block truncate text-[12px] font-semibold">{g.title}</span>
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )}
        <section aria-label={t('dashboard:home.waitingForYou')}>
          <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('dashboard:home.waitingForYou')}</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <h3 className="mb-1 text-[12px] font-semibold text-text-faint">{t('dashboard:home.toFinish')}</h3>
              <ul>
                {toFinish.map((g) => (
                  <li key={g.id}>
                    <button type="button" onClick={() => onOpenGame(g.id)} className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                      <span className="truncate text-[13px] font-semibold">{g.title}</span>
                      <span className="flex-none text-[12px] tabular-nums text-text-faint">{g.playtime_minutes ? formatHours(g.playtime_minutes) : ''}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <h3 className="mb-1 text-[12px] font-semibold text-text-faint">{t('dashboard:home.toInstall')}</h3>
              <ul>
                {toInstall.map((g) => (
                  <li key={g.id}>
                    <button type="button" onClick={() => onOpenGame(g.id)} className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                      <span className="truncate text-[13px] font-semibold">{g.title}</span>
                      <span className="flex-none text-[12px] text-text-faint">{storeLabel(g.store, t)}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>
      </div>
      {activity.length > 0 && (
        <section aria-label={t('activity:panel.title')} className="rounded-2xl bg-surface p-4 ring-1 ring-hairline">
          <div className="mb-1 flex items-center justify-between">
            <h2 className="text-[14px] font-bold">{t('activity:panel.title')}</h2>
            <Button variant="link" onClick={() => onNavigate({ name: 'activity' })}>{t('activity:panel.fullLog')}</Button>
          </div>
          <DashboardActivity events={activity} limit={5} onNavigate={onNavigate} />
        </section>
      )}
    </div>
  );
}
