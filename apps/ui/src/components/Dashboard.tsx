// i18n: converted
import React from 'react';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import type { ActivityEvent, Game, ProtonVersion, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { Cover, EmptyState, Panel, TierBadge, TweakDot, cardClass, cx } from './ui';
import { CompatChip, GameCardBody } from './CompatBadges';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { byRecent, coverImages, heroImages, needsAttention, storeLabel as storeLabelFallback } from '../lib/games';
import { formatHours } from '../lib/format';
import { runnerShort } from '../lib/settings';
import { sortKey } from '../lib/customize';

const byTitle = (a: Game, b: Game) => sortKey(a).localeCompare(sortKey(b)) || a.id.localeCompare(b.id);

interface DashboardProps {
  games: Game[];
  runners: ProtonVersion[];
  activity: ActivityEvent[];
  /** Library-wide owned count (installed + store-only). Defaults to games.length. */
  ownedCount?: number;
  dismissedHero: ReadonlySet<string>;
  onDismissHero: (ids: string[]) => void;
  onOpenGame: (id: string) => void;
  onNavigate: (v: View) => void;
}

export function Dashboard({ games, runners, activity, ownedCount, dismissedHero, onDismissHero, onOpenGame, onNavigate }: DashboardProps) {
  const { t } = useTranslation(['dashboard', 'common']);
  const waiting = games.filter(needsAttention);
  const pending = waiting.filter((g) => !dismissedHero.has(g.id));
  const hero = pending[0];

  const hasPlayData = games.some((g) => g.last_played_at);
  // Without play data byRecent has nothing to order by, so fall back to a stable title sort.
  const continueGames = [...games].sort(hasPlayData ? byRecent : byTitle).slice(0, 8);
  const recentlyAdded = games.filter((g) => g.store === 'custom').slice(0, 5);
  const installed = games.length;
  const owned = Math.max(ownedCount ?? installed, installed);

  if (games.length === 0) {
    return (
      <div className="p-7">
        <WelcomeBanner installed={0} owned={ownedCount ?? 0} fixes={0} />
        <EmptyState label={t('dashboard:empty.label')}>
          {t('dashboard:empty.detail')}
        </EmptyState>
      </div>
    );
  }

  return (
    <div className="p-7">
      {hero ? (
        <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <Cover src={heroImages(hero)} title={hero.title} className="h-[290px] w-full" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)' }} />
          <div className="absolute bottom-6 left-7 max-w-lg">
            <Badge variant="bronze" className="bg-orange-400 text-[#130f0a]">
              {hero.fix_info?.source === 'protondb' ? t('dashboard:hero.needsAttention') : t('dashboard:hero.fixAvailable')}
            </Badge>
            <h1 className="mt-2.5 text-[27px] font-extrabold leading-tight tracking-tight">
              {hero.fix_info?.source === 'protondb'
                ? t('dashboard:hero.titleTweaks', { title: hero.title })
                : t('dashboard:hero.titleFix', { title: hero.title })}
            </h1>
            <p className="mt-1.5 line-clamp-2 text-[12.5px] leading-relaxed text-white/70">{attentionReason(hero, t)}</p>
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
              [t('dashboard:hero.statInstalled'), String(installed), 'text-amber-300'],
              ...(owned > installed ? [[t('dashboard:hero.statOwned'), String(owned), 'text-white/80']] : []),
              [t('dashboard:hero.statFixes'), String(waiting.length), 'text-orange-300'],
            ].map(([k, v, c]) => (
              <div key={k} className="w-28 rounded-2xl bg-black/45 p-3.5 backdrop-blur">
                <div className={cx('text-[19px] font-extrabold tabular-nums', c)}>{v}</div>
                <div className="font-mono text-micro uppercase tracking-widest text-white/55">{k}</div>
              </div>
            ))}
          </div>
        </section>
      ) : (
        <WelcomeBanner installed={installed} owned={owned} fixes={waiting.length} />
      )}

      <section className="grid gap-5 xl:grid-cols-[1.75fr_1fr]">
        <div className="min-w-0">
          <div className="mb-3 flex items-baseline justify-between">
            <h2 className="text-[16px] font-bold tracking-tight">{hasPlayData ? t('dashboard:continuePlaying') : t('dashboard:yourGames')}</h2>
            <Button variant="link" onClick={() => onNavigate({ name: 'library' })}>{t('dashboard:fullLibrary')}</Button>
          </div>
          <div className="cover-grid items-stretch">
            {continueGames.map((g) => (
              <button key={g.id} onClick={() => onOpenGame(g.id)} className={cx(cardClass, 'card-compat flex h-full min-w-0 flex-col')}>
                <div className="relative flex-none">
                  <Cover src={coverImages(g)} title={g.title} className="aspect-[3/4] w-full" />
                  {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
                </div>
                <GameCardBody
                  title={g.title}
                  meta={`${g.playtime_minutes ? `${formatHours(g.playtime_minutes)} · ` : ''}${runnerShort(g.settings.runner_id, runners)}`}
                  game={g}
                  hideNative
                  className="px-3 py-2.5"
                />
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-5">
          <div>
            <h2 className="mb-2.5 text-[15px] font-bold">{t('dashboard:waiting')}</h2>
            {waiting.length === 0 ? (
              <p className="rounded-2xl bg-white/[0.03] p-4 text-[12px] text-white/70 ring-1 ring-white/[0.05]">
                {t('dashboard:waitingEmpty')}
              </p>
            ) : (
              <div className="space-y-3">
                {waiting.slice(0, 3).map((g) => (
                  <button
                    key={g.id}
                    onClick={() => onOpenGame(g.id)}
                    className="block w-full rounded-2xl bg-white/[0.045] p-4 text-left ring-1 ring-white/[0.06] transition hover:ring-orange-400/40"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="truncate text-[13px] font-bold">{g.title}</span>
                      {g.fix_info?.tier && <TierBadge tier={g.fix_info.tier} />}
                    </div>
                    <p className="mt-1 line-clamp-2 text-[12px] leading-relaxed text-white/70">{attentionReason(g, t)}</p>
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

          {recentlyAdded.length > 0 && (
            <Panel as="section" className="bg-white/[0.04]">
              <h2 className="mb-2.5 text-[14px] font-bold">{t('dashboard:recentlyAdded')}</h2>
              <RecentList games={recentlyAdded} onOpenGame={onOpenGame} />
            </Panel>
          )}
        </div>
      </section>

      <div className="mt-6">
        <ActivityPanel events={activity} limit={8} onFullLog={() => onNavigate({ name: 'activity' })} />
      </div>
    </div>
  );
}

/** Why a game needs attention, in words. Curated recipes speak for
 *  themselves (the plugin's own description); ProtonDB tiers are rendered
 *  from the catalog so they follow the UI language. */
function attentionReason(g: Game, t: TFunction): string {
  const f = g.fix_info;
  if (!f) return '';
  if (f.source !== 'protondb') return f.description;
  return t(f.tier === 'borked' ? 'dashboard:reason.borked' : 'dashboard:reason.bronze');
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

function WelcomeBanner({ installed, owned, fixes }: { installed: number; owned: number; fixes: number }) {
  const { t } = useTranslation('dashboard');
  const hour = new Date().getHours();
  const greeting = hour < 5
    ? t('dashboard:greeting.night')
    : hour < 12
      ? t('dashboard:greeting.morning')
      : hour < 18
        ? t('dashboard:greeting.afternoon')
        : t('dashboard:greeting.evening');
  return (
    <Panel as="section" className="mb-6 p-6">
      <h1 className="text-[25px] font-extrabold tracking-tight">{greeting}</h1>
      <p className="mt-1 font-mono text-[12px] tabular-nums text-white/55">
        {t('dashboard:welcome.installed', { count: installed })}
        {owned > installed ? ` · ${t('dashboard:welcome.owned', { count: owned })}` : ''} · {t('dashboard:welcome.fixes', { count: fixes })}
      </p>
    </Panel>
  );
}

export function RecentList({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const { t } = useTranslation('dashboard');
  return (
    <ul className="divide-y divide-white/[0.05]">
      {games.map((g) => (
        <li key={g.id}>
          <button onClick={() => onOpenGame(g.id)} className="-mx-1 flex w-full items-center gap-3 rounded-lg px-1 py-2 text-left hover:bg-white/[0.04]">
            <Cover src={coverImages(g)} title="" className="h-8 w-8 flex-none rounded ring-1 ring-white/10" />
            <div className="min-w-0 flex-1 leading-tight">
              <div className="truncate text-[12px] font-semibold">{g.title}</div>
              <div className="font-mono text-micro uppercase tracking-wider text-white/55">{storeLabel(g.store, t)}</div>
            </div>
            {!g.native && <CompatChip game={g} className="flex-none" />}
          </button>
        </li>
      ))}
    </ul>
  );
}
