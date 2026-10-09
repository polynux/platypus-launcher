import React, { useState } from 'react';
import { ArrowDown, ArrowUp, Download, LogIn, Play, Plus, X } from 'lucide-react';
import type { ActivityEvent, Game, Job, ProtonVersion, SourceWarning, StorePlugin, StoreType, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { RecentList } from './Dashboard';
import { Cover, EmptyState, Panel, SegRow, TierBadge, TweakDot, TIER_DOT, TIER_TEXT, cardClass, clock, cx } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { byRecent, formatBytes, heroImages, hoursPlayed, lastPlayedLabel, needsAttention, storeLabel } from '../lib/games';
import { runnerShort } from '../lib/settings';
import { sortKey } from '../lib/customize';

export type LibraryMode = 'cards' | 'table';
/** installed: what can be played now; owned: everything store plugins report too. */
export type Ownership = 'installed' | 'owned';

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
  ownership: Ownership;
  onOwnership: (o: Ownership) => void;
  warnings: SourceWarning[];
  plugins: StorePlugin[];
  jobs: Job[]; // queued or running
  lastScan: number | null;
  onOpenGame: (id: string) => void;
  onAddGame: () => void;
  onNavigate: (v: View) => void;
}

const CORE_SOURCES: StoreType[] = ['steam', 'gog', 'epic'];

type SortKey = 'title' | 'hours' | 'size' | 'last' | 'tier';
type Sort = { key: SortKey; dir: 'asc' | 'desc' };
/** ProtonDB quality order, best first. Unknown or missing tiers have no rank and sort last. */
export const TIER_RANK: Record<string, number> = { platinum: 5, gold: 4, silver: 3, bronze: 2, borked: 1 };
const sortValue = (g: Game, k: SortKey): string | number =>
  k === 'tier' ? TIER_RANK[g.fix_info?.tier ?? ''] ?? -1 : k === 'title' ? sortKey(g) : k === 'hours' ? hoursPlayed(g) ?? -1 : k === 'size' ? (g.is_installed ? g.size_bytes ?? -1 : -1) : g.last_played_at ?? -1;

/** Source chips come from what the library really holds: no chip for a store with zero games. */
export function sourceChips(games: Game[]): (StoreType | 'all')[] {
  const present = new Set(games.map((g) => g.store));
  return ['all', ...CORE_SOURCES.filter((s) => present.has(s)), ...[...present].filter((s) => s !== 'custom' && !CORE_SOURCES.includes(s))];
}

/** Games with no value for the key (no tier, never played...) stay last in both directions. */
export function sortGames(games: Game[], sort: Sort | null): Game[] {
  if (!sort) return games;
  return [...games].sort((a, b) => {
    const x = sortValue(a, sort.key), y = sortValue(b, sort.key);
    if (typeof x === 'number' && (x === -1 || y === -1) && x !== y) return x === -1 ? 1 : -1;
    const c = typeof x === 'string' ? x.localeCompare(y as string) : (x as number) - (y as number);
    return (sort.dir === 'asc' ? c : -c) || a.title.localeCompare(b.title);
  });
}
const SORT_OPTIONS: [SortKey | '', string][] = [['', 'default'], ['title', 'title'], ['last', 'recently played'], ['hours', 'hours played'], ['size', 'size'], ['tier', 'tier']];
const defaultDir = (k: SortKey): Sort['dir'] => (k === 'title' ? 'asc' : 'desc');

const DEFAULT_SORT = 'default'; // Radix forbids '' as an item value

function SortControl({ sort, onChange }: { sort: Sort | null; onChange: (s: Sort | null) => void }) {
  const asc = sort?.dir === 'asc';
  return (
    <div className="flex items-center gap-1.5">
      <Select
        value={sort?.key ?? DEFAULT_SORT}
        onValueChange={(v) => onChange(v === DEFAULT_SORT ? null : { key: v as SortKey, dir: defaultDir(v as SortKey) })}
      >
        <SelectTrigger aria-label="Sort by" className="w-44 uppercase tracking-wider">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SORT_OPTIONS.map(([v, l]) => <SelectItem key={v || DEFAULT_SORT} value={v || DEFAULT_SORT}>{v ? l : 'sort: default'}</SelectItem>)}
        </SelectContent>
      </Select>
      {sort && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="neutral"
              size="icon"
              aria-label={asc ? 'Ascending, switch to descending' : 'Descending, switch to ascending'}
              onClick={() => onChange({ ...sort, dir: asc ? 'desc' : 'asc' })}
            >
              {asc ? <ArrowUp className="h-3.5 w-3.5" /> : <ArrowDown className="h-3.5 w-3.5" />}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{asc ? 'Ascending' : 'Descending'}</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

function jobLabel(j: Job): string {
  if (j.state === 'queued') return `${j.kind} queued`;
  return j.percent >= 0 ? `${j.kind === 'uninstall' ? 'removing' : 'installing'} ${Math.round(j.percent)}%` : j.phase ?? j.kind;
}
const TABLE_COLS = 'grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_6rem_7rem]';

function SortHead({ k, label, sort, onSort, right }: { k: SortKey; label: string; sort: Sort | null; onSort: (k: SortKey) => void; right?: boolean }) {
  const active = sort?.key === k;
  return (
    <span role="columnheader" aria-sort={active ? (sort!.dir === 'asc' ? 'ascending' : 'descending') : 'none'} className={right ? 'text-right' : undefined}>
      <button onClick={() => onSort(k)} className={cx('inline-flex items-center gap-1 uppercase tracking-[0.14em] transition hover:text-white/80', active && 'text-amber-200')}>
        {label}
        {active && (sort!.dir === 'asc' ? <ArrowUp className="h-2.5 w-2.5" /> : <ArrowDown className="h-2.5 w-2.5" />)}
      </button>
    </span>
  );
}

export function LibraryView(p: LibraryProps) {
  const [sort, setSort] = useState<Sort | null>(null);
  // Family: the games borrowed from a family member, when the library has any.
  const [lending, setLending] = useState<'all' | 'with-me'>('all');
  const hasBorrowed = p.games.some((g) => g.shared_by != null);
  // The filter is only in effect while the control shows: once the borrowed
  // games are gone (uninstalled, logged out) it would hide the whole library.
  const lendingFilter = hasBorrowed ? lending : 'all';
  const q = p.search.trim().toLowerCase();
  const owned = p.games.length;
  const installedCount = p.games.filter((g) => g.is_installed).length;
  const sources = sourceChips(p.games);
  // A store can drop to zero games (logout, uninstall) while selected: its chip is gone, so show all.
  const source = sources.includes(p.source) ? p.source : 'all';
  const filtered = p.games.filter(
    (g) => (p.ownership === 'owned' || g.is_installed) && (source === 'all' || g.store === source) && (lendingFilter === 'all' || g.shared_by != null) && g.title.toLowerCase().includes(q)
  );
  const jobOf = (g: Game) => p.jobs.find((j) => j.game_id === g.id);
  const pluginName = (id: string) => p.plugins.find((x) => x.id === id)?.name ?? id;
  // The Last played strip belongs to the default order; an explicit sort shows one ordered grid.
  const played = sort ? [] : filtered.filter((g) => g.last_played_at).sort(byRecent).slice(0, 3);
  const tableRows = sortGames(filtered, sort);
  const toggleSort = (key: SortKey) =>
    setSort((s) => (s?.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: defaultDir(key) }));
  const rest = (sort ? tableRows : filtered).filter((g) => !played.includes(g));
  const maxHours = Math.max(1, ...p.games.map((g) => hoursPlayed(g) ?? 0));
  const recentlyAdded = p.games.filter((g) => g.store === 'custom').slice(0, 6);

  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Library</h1>
          <p className="mt-0.5 text-[12.5px] tabular-nums text-white/50">
            {filtered.length} shown · {installedCount} installed{owned > installedCount ? ` · ${owned} owned` : ''}
            {p.lastScan ? ` · scanned ${clock(p.lastScan)}` : ''}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          <div className="relative">
            <Input
              type="search"
              aria-label="Filter library"
              value={p.search}
              onChange={(e) => p.onSearch(e.target.value)}
              placeholder="filter…"
              className="w-48 pr-9 [&::-webkit-search-cancel-button]:hidden"
            />
            {p.search && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Clear search"
                    onClick={() => p.onSearch('')}
                    className="absolute right-1 top-1/2 size-7 -translate-y-1/2 text-white/45"
                  >
                    <X className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Clear search</TooltipContent>
              </Tooltip>
            )}
          </div>
          {owned > installedCount && (
            <SegRow label="Ownership" value={p.ownership} onChange={p.onOwnership} options={[{ value: 'installed', label: 'installed' }, { value: 'owned', label: 'owned' }]} />
          )}
          <SegRow label="Source" value={source} onChange={p.onSource} options={sources.map((s) => ({ value: s, label: s }))} />
          {hasBorrowed && (
            <SegRow label="Family" value={lendingFilter} onChange={(v) => setLending(v)} options={[{ value: 'all', label: 'all' }, { value: 'with-me', label: 'shared with me' }]} />
          )}
          <SortControl sort={sort} onChange={setSort} />
          <SegRow label="View" value={p.mode} onChange={p.onMode} options={[{ value: 'cards', label: 'cards' }, { value: 'table', label: 'table' }]} />
          <Button variant="primary" onClick={p.onAddGame}>
            <Plus className="h-3.5 w-3.5" strokeWidth={3} /> Add game
          </Button>
        </div>
      </header>

      {p.warnings.length > 0 && (
        <div className="mb-6 space-y-2">
          {p.warnings.map((w) => (
            <div key={w.source} role="status" className="flex items-center justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-4 py-3 ring-1 ring-amber-300/25">
              <p className="min-w-0 text-[12.5px] text-white/75">
                <span className="font-bold text-amber-200">{pluginName(w.source)}</span>{' '}
                {w.needs_auth ? 'is logged out, so its games are missing.' : w.unavailable ? <>isn't available right now: <span className="font-mono text-[12px] text-white/55">{w.message}</span></> : <>couldn't list its games: <span className="font-mono text-[12px] text-white/55">{w.message}</span></>}
              </p>
              <Button variant="primary" size="sm" className="flex-none" onClick={() => p.onNavigate({ name: 'plugins' })}>
                {w.needs_auth && <LogIn className="h-3.5 w-3.5" />} {w.needs_auth ? 'Log in' : 'Plugins'}
              </Button>
            </div>
          ))}
        </div>
      )}

      {filtered.length === 0 ? (
        <EmptyState label="no titles match">
          {p.games.length === 0
            ? 'No games were found in any source. Install something in Steam, or add a store plugin in Plugins.'
            : p.ownership === 'installed' && installedCount === 0
              ? 'Nothing is installed yet. Switch to "owned" to install a game from a store plugin.'
              : 'Nothing matches this filter. Clear the search or pick another source.'}
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
                        {g.fix_info?.tier && <TierBadge tier={g.fix_info.tier} />}
                      </div>
                      <div className="mt-1 flex items-center gap-2 font-mono text-[11px] text-white/50">
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
              {rest.map((g) => {
                const job = jobOf(g);
                return (
                  <button key={g.id} onClick={() => p.onOpenGame(g.id)} className={cardClass}>
                    <div className="relative">
                      <Cover src={g.cover_url} title={g.title} className={cx('aspect-[2/3] w-full', !g.is_installed && !job && 'opacity-45 grayscale-[.6]')} />
                      {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
                      {g.shared_by && (
                        <Badge variant="neutral" className="absolute left-2 top-2 max-w-[85%] truncate bg-black/65 text-white/80">Shared by {g.shared_by.name}</Badge>
                      )}
                      {job ? (
                        <div className="absolute inset-x-2 bottom-2 rounded-lg bg-black/70 px-2 py-1.5">
                          <div className="font-mono text-[11px] uppercase tracking-wider text-amber-200">{jobLabel(job)}</div>
                          <div className="mt-1 h-1 rounded-full bg-white/15">
                            <div className="h-full rounded-full bg-amber-300" style={{ width: `${Math.max(job.percent, 2)}%` }} />
                          </div>
                        </div>
                      ) : (
                        !g.is_installed && (
                          <Badge variant="neutral" className="absolute bottom-2 left-2 gap-1 bg-black/65 text-white/70">
                            {g.installable && <Download className="h-2.5 w-2.5" />} {g.installable ? 'not installed' : 'owned'}
                          </Badge>
                        )
                      )}
                    </div>
                    <div className="px-2.5 py-2">
                      <div className="truncate text-[11.5px] font-bold">{g.title}</div>
                      <div className="truncate font-mono text-[11px] uppercase tracking-wider text-white/40">
                        {[hoursPlayed(g) != null && `${hoursPlayed(g)}h`, lastPlayedLabel(g), storeLabel(g.store), g.update_available && 'update'].filter(Boolean).join(' · ')}
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </section>

          <section className={cx('mb-4 grid gap-5', recentlyAdded.length > 0 && 'xl:grid-cols-[1.5fr_1fr]')}>
            <ActivityPanel events={p.activity} limit={6} onFullLog={() => p.onNavigate({ name: 'activity' })} />
            {recentlyAdded.length > 0 && (
              <Panel className="bg-white/[0.04]">
                <h2 className="mb-3 text-[14px] font-bold">Recently added</h2>
                <RecentList games={recentlyAdded} onOpenGame={p.onOpenGame} />
              </Panel>
            )}
          </section>
        </>
      ) : (
        <section className="mb-8 overflow-x-auto rounded-2xl bg-black/25 ring-1 ring-white/[0.06]">
          <div role="table" aria-label="Library" className="min-w-[760px]">
            <div role="row" className={cx('grid gap-3 border-b border-white/[0.05] px-4 py-2 font-mono text-[11px] uppercase tracking-[0.12em] text-white/40', TABLE_COLS)}>
              <span role="columnheader"><span className="sr-only">status</span></span>
              <SortHead k="title" label="title" sort={sort} onSort={toggleSort} />
              <SortHead k="tier" label="tier" sort={sort} onSort={toggleSort} /><span role="columnheader">runner</span>
              <SortHead k="hours" label="hrs" sort={sort} onSort={toggleSort} right />
              <SortHead k="size" label="size" sort={sort} onSort={toggleSort} right />
              <SortHead k="last" label="last played" sort={sort} onSort={toggleSort} right />
            </div>
            {tableRows.map((g, i) => {
              const tier = g.fix_info?.tier;
              return (
                <div key={g.id} role="row" className={cx('grid items-center gap-3 px-4 py-2 transition hover:bg-amber-300/[0.05]', TABLE_COLS, i % 2 && 'bg-white/[0.015]')}>
                  <span role="cell" className={cx('h-1.5 w-1.5 rounded-full', TIER_DOT[tier ?? ''] ?? 'bg-white/15')} />
                  <span role="cell" className="min-w-0">
                    <button onClick={() => p.onOpenGame(g.id)} className="group flex w-full min-w-0 items-center gap-2.5 text-left">
                      <Cover src={g.cover_url} title="" className="h-7 w-7 flex-none rounded ring-1 ring-white/10" />
                      <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
                      {g.shared_by && <Badge variant="neutral" className="flex-none">Shared by {g.shared_by.name}</Badge>}
                    </button>
                  </span>
                  <span role="cell" className={cx('font-mono text-[11.5px]', TIER_TEXT[tier ?? ''] ?? 'text-white/40')}>{tier ?? '—'}</span>
                  <span role="cell" className="truncate font-mono text-[11.5px] text-white/45">{runnerShort(g.settings.runner_id, p.runners)}</span>
                  <span role="cell" className="text-right font-mono text-[11.5px] tabular-nums text-white/55">{hoursPlayed(g) ?? '—'}</span>
                  <span role="cell" className="text-right font-mono text-[11.5px] text-white/45">{(g.is_installed ? formatBytes(g.size_bytes) : 'not installed') || '—'}</span>
                  <span role="cell" className="text-right font-mono text-[11.5px] text-white/45">{lastPlayedLabel(g) ?? '—'}</span>
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
