import React from 'react';
import { Play, Plus } from 'lucide-react';
import type { ActivityEvent, Game, ProtonVersion, StoreType, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { RecentList } from './Dashboard';
import { Cover, EmptyState, Panel, TweakDot, TIER_DOT, TIER_TEXT, btnPrimary, cardClass, clock, cx } from './ui';
import { byRecent, heroImages, hoursPlayed, lastPlayedLabel, needsAttention, storeLabel } from '../lib/games';
import { runnerShort } from '../lib/settings';

export type LibraryMode = 'cards' | 'table';

interface LibraryProps {
  games: Game[];
  runners: ProtonVersion[];
  activity: ActivityEvent[];
  search: string;
  onSearch: (q: string) => void;
  source: StoreType | 'all';
  onSource: (s: StoreType | 'all') => void;
  mode: LibraryMode;
  onMode: (m: LibraryMode) => void;
  lastScan: number | null;
  onOpenGame: (id: string) => void;
  onAddGame: () => void;
  onNavigate: (v: View) => void;
}

const SOURCES: (StoreType | 'all')[] = ['all', 'steam', 'gog', 'epic'];
const TABLE_COLS = 'grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem]';

function Segmented<T extends string>({ value, options, onChange, label, mono }: { value: T; options: [T, string][]; onChange: (v: T) => void; label: string; mono?: boolean }) {
  return (
    <div role="radiogroup" aria-label={label} className="flex rounded-xl bg-white/[0.05] p-1">
      {options.map(([v, l]) => (
        <button
          key={v}
          role="radio"
          aria-checked={value === v}
          onClick={() => onChange(v)}
          className={cx(
            'rounded-lg px-3.5 py-1.5 transition',
            mono ? 'font-mono text-[10px] uppercase tracking-wider' : 'text-[12px] font-bold',
            value === v ? 'bg-white/12 text-white' : 'text-white/45 hover:text-white/80'
          )}
        >
          {l}
        </button>
      ))}
    </div>
  );
}

export function LibraryView(p: LibraryProps) {
  const q = p.search.trim().toLowerCase();
  const filtered = p.games.filter((g) => (p.source === 'all' || g.store === p.source) && g.title.toLowerCase().includes(q));
  const played = filtered.filter((g) => g.last_played_at).sort(byRecent).slice(0, 3);
  const rest = filtered.filter((g) => !played.includes(g));
  const maxHours = Math.max(1, ...p.games.map((g) => hoursPlayed(g) ?? 0));
  const recentlyAdded = p.games.filter((g) => g.store === 'custom').slice(0, 6);

  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Library</h1>
          <p className="mt-0.5 text-[12.5px] tabular-nums text-white/50">
            {filtered.length} of {p.games.length} titles{p.lastScan ? ` · scanned ${clock(p.lastScan)}` : ''}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          <input
            type="search"
            aria-label="Filter library"
            value={p.search}
            onChange={(e) => p.onSearch(e.target.value)}
            placeholder="filter…"
            className="w-48 rounded-xl bg-white/[0.06] px-3.5 py-2 font-mono text-[11.5px] text-white placeholder-white/30 outline-none ring-amber-300/50 focus:ring-1"
          />
          <Segmented label="Source" mono value={p.source} onChange={p.onSource} options={SOURCES.map((s) => [s, s])} />
          <Segmented label="View" value={p.mode} onChange={p.onMode} options={[['cards', 'cards'], ['table', 'table']]} />
          <button onClick={p.onAddGame} className={cx(btnPrimary, 'flex items-center gap-1.5 px-4 py-2.5 text-[12.5px]')}>
            <Plus className="h-3.5 w-3.5" strokeWidth={3} /> Add game
          </button>
        </div>
      </header>

      {filtered.length === 0 ? (
        <EmptyState label="no titles match">
          {p.games.length === 0 ? 'No installed games were found in any source.' : 'Nothing matches this filter. Clear the search or pick another source.'}
        </EmptyState>
      ) : p.mode === 'cards' ? (
        <>
          {played.length > 0 && (
            <section className="mb-8">
              <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-white/60">Last played</h2>
              <div className="grid gap-4 lg:grid-cols-3">
                {played.map((g) => (
                  <button key={g.id} onClick={() => p.onOpenGame(g.id)} className="group relative overflow-hidden rounded-3xl text-left ring-1 ring-white/[0.08] transition hover:ring-amber-300/40">
                    <Cover src={heroImages(g)} title={g.title} className="h-[210px] w-full transition duration-500 group-hover:scale-[1.04]" />
                    <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.95) 8%, transparent 60%)' }} />
                    <div className="absolute bottom-4 left-5 right-5">
                      <div className="flex items-baseline justify-between gap-3">
                        <span className="truncate text-[16px] font-extrabold tracking-tight">{g.title}</span>
                        <span className={cx('font-mono text-[9px] uppercase tracking-wider', TIER_TEXT[g.fix_info?.tier ?? ''] ?? 'text-white/45')}>{g.fix_info?.tier}</span>
                      </div>
                      <div className="mt-1 flex items-center gap-2 font-mono text-[9px] text-white/50">
                        {hoursPlayed(g) != null && <><span>{hoursPlayed(g)}h</span><span>·</span></>}
                        <span>{lastPlayedLabel(g)}</span><span>·</span><span>{runnerShort(g.settings.runner_id, p.runners)}</span>
                      </div>
                      {hoursPlayed(g) != null && (
                        <div className="mt-2.5 h-1 rounded-full bg-white/[0.14]">
                          <div className="h-full rounded-full bg-amber-300/80" style={{ width: `${Math.min(100, (hoursPlayed(g)! / maxHours) * 100)}%` }} />
                        </div>
                      )}
                    </div>
                    <span className="absolute right-4 top-4 grid h-9 w-9 place-items-center rounded-full bg-black/55 text-white opacity-0 transition group-hover:opacity-100">
                      <Play className="h-3.5 w-3.5 fill-current" />
                    </span>
                  </button>
                ))}
              </div>
            </section>
          )}

          <section className="mb-8">
            <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-white/60">All titles</h2>
            <div className="cover-grid-sm">
              {rest.map((g) => (
                <button key={g.id} onClick={() => p.onOpenGame(g.id)} className={cardClass}>
                  <div className="relative">
                    <Cover src={g.cover_url} title={g.title} className="aspect-[2/3] w-full" />
                    {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
                  </div>
                  <div className="px-2.5 py-2">
                    <div className="truncate text-[11.5px] font-bold">{g.title}</div>
                    <div className="truncate font-mono text-[9px] uppercase tracking-wider text-white/40">
                      {[hoursPlayed(g) != null && `${hoursPlayed(g)}h`, lastPlayedLabel(g), storeLabel(g.store)].filter(Boolean).join(' · ')}
                    </div>
                  </div>
                </button>
              ))}
            </div>
          </section>

          <section className="mb-4 grid gap-5 xl:grid-cols-[1.5fr_1fr]">
            <ActivityPanel events={p.activity} limit={6} onFullLog={() => p.onNavigate({ name: 'activity' })} />
            <Panel className="bg-white/[0.04]">
              <h2 className="mb-3 text-[14px] font-bold">Recently added</h2>
              {recentlyAdded.length === 0 ? (
                <p className="text-[12px] leading-relaxed text-white/50">Store install dates aren't tracked yet. Games you sideload this session appear here.</p>
              ) : (
                <RecentList games={recentlyAdded} onOpenGame={p.onOpenGame} />
              )}
            </Panel>
          </section>
        </>
      ) : (
        <section className="mb-8 overflow-x-auto rounded-2xl bg-black/25 ring-1 ring-white/[0.06]">
          <div className="min-w-[760px]">
            <div className={cx('grid gap-3 border-b border-white/[0.05] px-4 py-2 font-mono text-[9px] uppercase tracking-[0.14em] text-white/40', TABLE_COLS)}>
              <span /><span>title</span><span>tier</span><span>runner</span>
              <span className="text-right">hrs</span><span className="text-right">size</span><span className="text-right">last played</span>
            </div>
            {filtered.map((g, i) => {
              const tier = g.fix_info?.tier;
              return (
                <button
                  key={g.id}
                  onClick={() => p.onOpenGame(g.id)}
                  className={cx('group grid w-full items-center gap-3 px-4 py-2 text-left transition hover:bg-amber-300/[0.05]', TABLE_COLS, i % 2 && 'bg-white/[0.015]')}
                >
                  <span className={cx('h-1.5 w-1.5 rounded-full', TIER_DOT[tier ?? ''] ?? 'bg-white/15')} />
                  <span className="flex min-w-0 items-center gap-2.5">
                    <Cover src={g.cover_url} title="" className="h-7 w-7 flex-none rounded ring-1 ring-white/10" />
                    <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
                  </span>
                  <span className={cx('font-mono text-[10px]', TIER_TEXT[tier ?? ''] ?? 'text-white/40')}>{tier ?? '—'}</span>
                  <span className="truncate font-mono text-[10px] text-white/45">{runnerShort(g.settings.runner_id, p.runners)}</span>
                  <span className="text-right font-mono text-[10.5px] tabular-nums text-white/55">{hoursPlayed(g) ?? '—'}</span>
                  <span className="text-right font-mono text-[10px] text-white/45">{g.size_on_disk ?? '—'}</span>
                  <span className="text-right font-mono text-[10px] text-white/45">{lastPlayedLabel(g) ?? '—'}</span>
                </button>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
