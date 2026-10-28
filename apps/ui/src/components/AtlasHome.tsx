// i18n: converted
// The /22 Home page (issue #138): greeting, the suggestion card, Continue
// playing cards, the Recently added strip and the Waiting for you lists.
// Moved out of Dashboard.tsx (issue #156) so each file holds one
// presentation; Dashboard branches on dashboard.presentation.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { ActivityEvent, Game } from '../types';
import { ATLAS_PAGE, Cover } from './ui';
import { Button } from './ds/button';
import { Download, Play, X } from 'lucide-react';
import { byRecent, coverImages, formatBytes, heroImages, lastPlayedAgo, needsAttention } from '../lib/games';
import { formatHours } from '../lib/format';
import { sortKey } from '../lib/customize';
import { ratingBadge } from '../lib/compat';
import { AtlasMainAction } from './GameTabs';
import { PluginLogo } from './PluginLogo';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import { attentionReason, byAddedAt, DashboardEmpty, storeLabel, type DashboardProps } from './Dashboard';

const byTitle = (a: Game, b: Game) => sortKey(a).localeCompare(sortKey(b)) || a.id.localeCompare(b.id);

/** The suggestion card's fallback for a plain backlog game (no fix, no low
 *  rating): it is in the library but not installed, plus the ProtonDB tier
 *  when one is known. */
function backlogReason(g: Game, t: (k: string, o?: Record<string, unknown>) => string): string {
  if (g.is_installed) return '';
  const tone = ratingBadge(g)?.tone ?? g.fix_info?.tier ?? null;
  const known = tone && tone !== 'native' ? tone.charAt(0).toUpperCase() + tone.slice(1) : null;
  const base = t('dashboard:home.backlogFallback');
  return known ? `${base} ${t('dashboard:home.backlogRated', { tier: known })}` : base;
}

/** Recently added (issues #138, #145, #151): newest added_at first; without
 *  added_at the newest install/add activity matched back by id or title, with
 *  sideloaded games standing in when neither names anything. */
function recentAdded(games: Game[], activity: ActivityEvent[]): Game[] {
  if (games.some((g) => (g.added_at ?? 0) > 0)) return [...games].sort(byAddedAt).slice(0, 5);
  const byId = new Map(games.map((g) => [g.id, g]));
  const byTitleMap = new Map(games.map((g) => [g.title, g]));
  const seen = new Set<string>();
  const out: Game[] = [];
  for (const e of activity) {
    if (e.type !== 'download' && e.type !== 'add') continue;
    const g = (e.game && (byId.get(e.game) ?? byTitleMap.get(e.game))) || undefined;
    if (g && !seen.has(g.id)) {
      seen.add(g.id);
      out.push(g);
      if (out.length >= 5) break;
    }
  }
  if (out.length > 0) return out;
  return games.filter((g) => g.store === 'custom').slice(0, 5);
}

export function AtlasHome({ games, allGames, activity, dismissedHero, onDismissHero, onOpenGame, onNavigate, jobs, sessions, sessionStartedAt, onPlay, onInstall, onStop, onJobAction, social = [], onOpenFriends, sourceLogos }: DashboardProps & { allGames: Game[] }) {
  const { t } = useTranslation(['dashboard', 'common', 'game', 'activity']);
  const waiting = games.filter(needsAttention);
  const hasSocial = social.some((v) => v.enabled);
  const pending = waiting.filter((g) => !dismissedHero.has(g.id));
  // The lab's suggestion: the first owned, not-installed game, else an
  // installed never-played one, skipping dismissed ones, else the game
  // needing attention.
  const owned = allGames.length > 0 ? allGames : games;
  const suggested = owned.find((g) => !g.is_installed && !dismissedHero.has(g.id))
    ?? games.find((g) => g.is_installed && !g.last_played_at && !dismissedHero.has(g.id))
    ?? pending[0];
  const installed = games.length;
  const hasPlayData = games.some((g) => g.last_played_at);
  const continueGames = [...games].sort(hasPlayData ? byRecent : byTitle).slice(0, 3);
  const added = recentAdded(owned, activity);
  const toFinish = [...games].filter((g) => (g.playtime_minutes ?? 0) > 0).sort(byRecent).slice(0, 5);
  const toInstall = owned.filter((g) => !g.is_installed).slice(0, 5);
  // The strip's main action on Home (R4-15): the same job/running/idle
  // behaviour as the game page, with Home's own navigation buttons idle.
  const jobsById = React.useMemo(() => new Map((jobs ?? []).map((j) => [j.game_id, j])), [jobs]);
  const mainAction = (g: Game, idle: React.ReactNode) => (
    <AtlasMainAction
      job={jobsById.get(g.id)}
      session={sessions?.[g.id]}
      sessionStartedAt={sessionStartedAt?.[g.id]}
      onJobAction={onJobAction}
      onStop={() => onStop?.(g.id)}
    >
      {idle}
    </AtlasMainAction>
  );
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
  const reason = suggested ? attentionReason(suggested, t) || backlogReason(suggested, t) : '';
  const suggestedRunning = !!suggested && ['running', 'starting'].includes(sessions?.[suggested.id]?.state ?? '');
  const suggestedDescription = suggestedRunning ? t('dashboard:home.running', { title: suggested?.title }) : reason;
  const installSize = suggested && !suggested.is_installed && suggested.size_bytes ? formatBytes(suggested.size_bytes) : null;
  return (
    <div className={ATLAS_PAGE}>
      <header>
        <h1 className="text-[24px] font-extrabold tracking-tight">{greeting}</h1>
        <p className="mt-[3px] text-[13px] text-text-muted">{t('dashboard:home.ready', { count: installed })}</p>
      </header>
      {suggested && (
        <section aria-label={suggested.title} className="relative min-h-[230px] overflow-hidden rounded-2xl text-white">
          <img src={heroImages(suggested)[0]} alt="" aria-hidden loading="lazy" className="absolute inset-0 h-full min-h-[230px] w-full object-cover" style={{ objectPosition: 'center 30%' }} />
          <div aria-hidden className="absolute inset-0 bg-[linear-gradient(90deg,rgba(8,10,14,.92)_0%,rgba(8,10,14,.6)_45%,transparent_75%)]" />
          <div className="relative grid min-h-[230px] max-w-[460px] content-end gap-2 px-7 py-[26px]">
            <small className="text-[12px] font-semibold text-white opacity-70">{t('dashboard:home.fromBacklog')}</small>
            <h2 className="text-[28px] font-extrabold leading-tight tracking-[-0.01em]">{suggested.title}</h2>
            {suggestedDescription && <p className={`leading-normal text-white/[.82] ${suggestedRunning ? '' : 'line-clamp-2'}`}>{suggestedDescription}</p>}
            <div className="mt-1 flex gap-2.5">
              {mainAction(suggested, (
                <Button variant="primary" onClick={() => { if (suggested.is_installed) { if (onPlay) onPlay(suggested.id); else onOpenGame(suggested.id); } else if (onInstall) onInstall(suggested); else onOpenGame(suggested.id); }} className="h-[38px] px-4">
                  {suggested.is_installed ? t('game:play.play') : (<><Download className="size-3.5" aria-hidden />{t('game:install.install')}{installSize && <span className="font-mono text-[12px]">{installSize}</span>}</>)}
                </Button>
              ))}
              <Button variant="ghost" onClick={() => onOpenGame(suggested.id)} className="h-[38px] bg-transparent px-4 text-white hover:bg-white/10 hover:text-white">
                {t('dashboard:hero.openGame')}
              </Button>
            </div>
          </div>
          <button
            type="button"
            onClick={() => onDismissHero([suggested.id])}
            aria-label={t('dashboard:home.dismiss')}
            className="absolute right-3 top-3 grid size-8 place-items-center rounded-lg bg-black/35 text-white transition hover:bg-black/55 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <X className="size-4" aria-hidden />
          </button>
        </section>
      )}
      <section aria-label={t('dashboard:continuePlaying')}>
        <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{hasPlayData ? t('dashboard:continuePlaying') : t('dashboard:yourGames')}</h2>
        <ul className="grid gap-3.5 md:grid-cols-3">
          {continueGames.map((g) => {
            const sub = ['running', 'starting'].includes(sessions?.[g.id]?.state ?? '') ? t('dashboard:home.running', { title: g.title }) : [lastPlayedAgo(g), g.playtime_minutes ? `${Math.round(g.playtime_minutes / 60)} h` : ''].filter(Boolean).join(' · ') || '—';
            return (
              <li key={g.id}>
                <button type="button" onClick={() => onOpenGame(g.id)} aria-label={t('dashboard:hero.openGame') + ': ' + g.title} className="block w-full overflow-hidden rounded-xl text-left ring-1 ring-hairline transition hover:ring-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus">
                  <Cover src={heroImages(g)} title={g.title} className="aspect-[16/7] w-full" />
                </button>
                <div className="mt-2 flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-[13px] font-bold">{g.title}</div>
                    <div className="truncate text-[12px] text-text-faint"><ContinueMeta game={g} sub={sub} /></div>
                  </div>
                  {mainAction(g, (
                    <button
                      type="button"
                      onClick={() => (onPlay ? onPlay(g.id) : onOpenGame(g.id))}
                      aria-label={`${t('game:play.play')}: ${g.title}`}
                      className="inline-flex h-[38px] flex-none items-center gap-1.5 rounded-lg bg-accent px-4 text-[13px] font-bold text-on-accent transition hover:brightness-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                    >
                      <Play className="size-3.5 fill-current" aria-hidden />
                      {t('game:play.play')}
                    </button>
                  ))}
                </div>
              </li>
            );
          })}
        </ul>
      </section>
      <div className="grid gap-6 lg:grid-cols-2">
        {added.length > 0 && (
          <section aria-label={t('dashboard:recentlyAdded')}>
            <h2 className="mb-2.5 text-[13px] font-bold text-text-muted">{t('dashboard:recentlyAdded')}</h2>
            <ul className="grid grid-cols-5 gap-2.5">
              {added.map((g) => (
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
        <section aria-label={t('dashboard:home.waitingForYou')} className={added.length === 0 ? 'lg:col-span-2' : undefined}>
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
                      <span className="flex-none text-[12px] text-text-faint">{g.size_bytes ? formatBytes(g.size_bytes) : ''}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </section>
      </div>
      <div className="grid gap-4 md:grid-cols-3">
        <section aria-label={t('dashboard:home.yourLibrary')} className="grid content-start gap-3 rounded-2xl bg-surface p-4 ring-1 ring-hairline">
          <div className="flex items-center justify-between gap-2.5">
            <h2 className="text-[14px] font-bold">{t('dashboard:home.yourLibrary')}</h2>
            <Button variant="link" onClick={() => onNavigate({ name: 'library' })}>{t('dashboard:home.openLibrary')}</Button>
          </div>
          <dl className="grid grid-cols-4 gap-2">
            {([
              [t('dashboard:home.statGames'), String(owned.length)],
              [t('dashboard:home.statInstalled'), String(installed)],
              [t('dashboard:home.statPlayed'), `${Math.round(games.reduce((a, g) => a + (g.playtime_minutes ?? 0), 0) / 60)} h`],
              [t('dashboard:home.statOnDisk'), formatBytes(games.reduce((a, g) => a + (g.size_bytes ?? 0), 0)) || '—'],
            ] as [string, string][]).map(([k, v]) => (
              <div key={k}>
                <dt className="text-[11.5px] text-text-faint">{k}</dt>
                <dd className="truncate text-[15px] font-bold tabular-nums" title={v}>{v}</dd>
              </div>
            ))}
          </dl>
          <div className="flex h-6 gap-0.5 overflow-hidden rounded-md">
            {[...owned.reduce((counts, game) => counts.set(game.source, (counts.get(game.source) ?? 0) + 1), new Map<string, number>())].map(([s, n]) => (
                <span key={s} title={`${storeLabel(s, t)}: ${n}`} style={{ flex: n }} className="flex min-w-0 items-center gap-1 overflow-hidden bg-surface-strong px-1.5 text-[11.5px] tabular-nums text-text-muted">
                  <PluginLogo src={sourceLogos?.[s]} name={storeLabel(s, t)} size="glyph" />{n}
                </span>
            ))}
          </div>
        </section>
        <section aria-label={t('dashboard:home.needsAttention')} className="grid content-start gap-3 rounded-2xl bg-surface p-4 ring-1 ring-hairline">
          <div className="flex items-center justify-between gap-2.5">
            <h2 className="text-[14px] font-bold">{t('dashboard:home.needsAttention')}</h2>
            {waiting.length > 0 && (
              <Button variant="link" onClick={() => onNavigate({ name: 'fixes' })}>
                {t('dashboard:home.seeAll', { count: waiting.length })}
              </Button>
            )}
          </div>
          {waiting.length === 0 ? (
            <p className="text-[13px] text-text-muted">{t('dashboard:home.everythingWorks')}</p>
          ) : (
            <ul className="grid gap-2">
              {waiting.slice(0, 3).map((g) => (
                <li key={g.id} className="flex items-center gap-2.5">
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[13px] font-bold">{g.title}</span>
                    <span className="block truncate text-[12px] text-text-muted">{attentionReason(g, t)}</span>
                  </span>
                  <Button variant="neutral" size="sm" className="flex-none" onClick={() => onOpenGame(g.id)}>
                    {t('dashboard:hero.openGame')}
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </section>
        {hasSocial ? <section aria-label={t('dashboard:home.friendsOnline')} className="grid content-start gap-3 rounded-2xl bg-surface p-4 ring-1 ring-hairline">
          <div className="flex items-center justify-between gap-2.5">
            <h2 className="text-[14px] font-bold">{t('dashboard:home.friendsOnline')}</h2>
            <Button variant="link" onClick={onOpenFriends}>{t('dashboard:home.onlineCount', { count: social.flatMap((v) => v.friends).filter((f) => f.state === 'online').length })}</Button>
          </div>
          <ul className="grid gap-2">
            {social.filter((v) => v.enabled).flatMap((v) => v.friends.filter((f) => f.state === 'online').map((f) => ({ ...f, plugin: v.plugin }))).slice(0, 4).map((f) => (
              <li key={`${f.plugin}:${f.id}`} className="flex min-w-0 items-center gap-2.5">
                {f.avatarUrl ? <img src={f.avatarUrl} alt="" className="size-8 rounded-full object-cover" /> : <span className="grid size-8 place-items-center rounded-full bg-surface-strong text-[12px] font-bold">{f.name.slice(0, 1)}</span>}
                <span className="min-w-0"><span className="block truncate text-[13px] font-semibold">{f.name}</span><span className="block truncate text-[12px] text-text-faint">{f.game?.title ?? t('dashboard:home.online')}</span></span>
              </li>
            ))}
          </ul>
        </section> : null}
      </div>
    </div>
  );
}

function ContinueMeta({ game, sub }: { game: Game; sub: string }) {
  const { t } = useTranslation('dashboard');
  const [progress, setProgress] = useState<{ unlocked: number; total: number } | null>(null);
  useEffect(() => {
    if (typeof LauncherApi.getGameAchievements !== 'function') return;
    let alive = true;
    Effect.runPromise(LauncherApi.getGameAchievements(game)).then((r) => {
      if (alive && r) setProgress(r);
    }).catch(() => {});
    return () => { alive = false; };
    // Keyed on the id: library refreshes replace the game object.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);
  return <>{sub}{progress ? ` · ${t('dashboard:home.achievements', { unlocked: progress.unlocked, total: progress.total })}` : ''}</>;
}
