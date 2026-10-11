import React from 'react';
import type { ActivityEvent, Game, ProtonVersion, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { Cover, EmptyState, Panel, TierBadge, TweakDot, cardClass, cx } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { attentionReason, byRecent, coverImages, greeting, heroImages, hoursPlayed, needsAttention, storeLabel } from '../lib/games';
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
        <EmptyState label="no titles found">
          Platypus didn't find any installed games. Install something in Steam, or add a game from the Library.
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
              {hero.fix_info?.source === 'protondb' ? 'needs attention' : 'fix available'}
            </Badge>
            <h1 className="mt-2.5 text-[27px] font-extrabold leading-tight tracking-tight">
              {hero.fix_info?.source === 'protondb' ? `${hero.title} may need tweaks` : `${hero.title} has a community fix`}
            </h1>
            <p className="mt-1.5 line-clamp-2 text-[12.5px] leading-relaxed text-white/70">{attentionReason(hero)}</p>
            <div className="mt-3.5 flex gap-2.5">
              <Button variant="primary" onClick={() => onOpenGame(hero.id)}>Open game</Button>
              <Button variant="neutral" onClick={() => onDismissHero([hero.id])}>Later</Button>
              {pending.length > 1 && (
                <Button variant="ghost" onClick={() => onDismissHero(pending.map((g) => g.id))}>
                  Hide all {pending.length}
                </Button>
              )}
            </div>
          </div>
          <div className="absolute bottom-6 right-7 hidden gap-3 lg:flex">
            {[
              ['installed', String(installed), 'text-amber-300'],
              ...(owned > installed ? [['owned', String(owned), 'text-white/80']] : []),
              ['fixes', String(waiting.length), 'text-orange-300'],
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
            <h2 className="text-[16px] font-bold tracking-tight">{hasPlayData ? 'Continue playing' : 'Your games'}</h2>
            <Button variant="link" onClick={() => onNavigate({ name: 'library' })}>full library →</Button>
          </div>
          <div className="cover-grid">
            {continueGames.map((g) => (
              <button key={g.id} onClick={() => onOpenGame(g.id)} className={cardClass}>
                <div className="relative">
                  <Cover src={coverImages(g)} title={g.title} className="aspect-[3/4] w-full" />
                  {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
                </div>
                <div className="px-3 py-2.5">
                  <div className="truncate text-[12px] font-bold">{g.title}</div>
                  <div className="font-mono text-micro uppercase tracking-wider text-white/55">
                    {hoursPlayed(g) != null ? `${hoursPlayed(g)}h · ` : ''}
                    {runnerShort(g.settings.runner_id, runners)}
                  </div>
                </div>
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-5">
          <div>
            <h2 className="mb-2.5 text-[15px] font-bold">Waiting on you</h2>
            {waiting.length === 0 ? (
              <p className="rounded-2xl bg-white/[0.03] p-4 text-[12px] text-white/70 ring-1 ring-white/[0.05]">
                Nothing needs attention. Curated fixes and low ProtonDB ratings show up here.
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
                    <p className="mt-1 line-clamp-2 text-[12px] leading-relaxed text-white/70">{attentionReason(g)}</p>
                  </button>
                ))}
                {waiting.length > 3 && (
                  <Button variant="link" onClick={() => onNavigate({ name: 'fixes' })}>
                    +{waiting.length - 3} more in fixes →
                  </Button>
                )}
              </div>
            )}
          </div>

          {recentlyAdded.length > 0 && (
            <Panel as="section" className="bg-white/[0.04]">
              <h2 className="mb-2.5 text-[14px] font-bold">Recently added</h2>
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

function WelcomeBanner({ installed, owned, fixes }: { installed: number; owned: number; fixes: number }) {
  return (
    <Panel as="section" className="mb-6 p-6">
      <h1 className="text-[25px] font-extrabold tracking-tight">{greeting()}</h1>
      <p className="mt-1 font-mono text-[12px] tabular-nums text-white/55">
        {installed} installed{owned > installed ? ` · ${owned} owned` : ''} · {fixes} {fixes === 1 ? 'fix' : 'fixes'}
      </p>
    </Panel>
  );
}

export function RecentList({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  return (
    <ul className="divide-y divide-white/[0.05]">
      {games.map((g) => (
        <li key={g.id}>
          <button onClick={() => onOpenGame(g.id)} className="-mx-1 flex w-full items-center gap-3 rounded-lg px-1 py-2 text-left hover:bg-white/[0.04]">
            <Cover src={coverImages(g)} title="" className="h-8 w-8 flex-none rounded ring-1 ring-white/10" />
            <div className="min-w-0 flex-1 leading-tight">
              <div className="truncate text-[12px] font-semibold">{g.title}</div>
              <div className="font-mono text-micro uppercase tracking-wider text-white/55">{storeLabel(g.store)}</div>
            </div>
          </button>
        </li>
      ))}
    </ul>
  );
}
