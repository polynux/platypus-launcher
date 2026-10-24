// i18n: converted
import { useTranslation, Trans } from 'react-i18next';
import React, { memo, useDeferredValue, useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, Download, LogIn, PackagePlus, Play, Plus, SlidersHorizontal, WifiOff, X } from 'lucide-react';
import type { ActivityEvent, Collection, Game, Job, ProtonVersion, SourceWarning, StorePlugin, StoreType, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { RecentList } from './Dashboard';
import { Cover, EmptyState, MonoLabel, Panel, SegRow, TierBadge, TweakDot, TIER_DOT, TIER_TEXT, ViewHeader, cardClass, clock, cx } from './ui';
import { GameCardBody, tierMeaning } from './CompatBadges';
import { TIER_FILTERS, tierFilterValue, type TierFilter } from '../lib/compat';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { byRecent, coverImages, formatBytes, heroImages, hoursPlayed, needsAttention, storeLabel } from '../lib/games';
import { collectionPredicate, missingNames } from '../lib/collections';
import { formatCount } from '../lib/format';
import { formatDecimal1, recencyOf, renderRecency } from '../lib/gameLocale';
import { runnerShort } from '../lib/settings';
import { sortKey } from '../lib/customize';
import { useNetState } from './useNetState';
import { WindowedGrid, WindowedRows } from './WindowedList';
import { PluginLogo } from './PluginLogo';

export type LibraryMode = 'cards' | 'table';
/** installed: what can be played now; owned: everything store plugins report too. */
export type Ownership = 'installed' | 'owned';

type T = (key: string, vars?: Record<string, unknown>) => string;

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
  /** Plugin logos by store (and plugin id), as the sidebar Sources rows use. */
  sourceLogos?: Record<string, string>;
  onOpenGame: (id: string) => void;
  onAddGame: () => void;
  onNavigate: (v: View) => void;
  /** The collection the library shows (issue #45); absent: all games. */
  collection?: string;
  onCollection?: (id: string | undefined) => void;
  collections?: Collection[];
  /** Right-click on a game: its collections dialog. */
  onManageCollections?: (gameId: string) => void;
}

const CORE_SOURCES: StoreType[] = ['steam', 'gog', 'epic'];

type SortKey = 'title' | 'hours' | 'size' | 'last' | 'tier' | 'year';
type Sort = { key: SortKey; dir: 'asc' | 'desc' };
/** ProtonDB quality order, best first. Unknown or missing tiers have no rank and sort last. */
export const TIER_RANK: Record<string, number> = { platinum: 5, gold: 4, silver: 3, bronze: 2, borked: 1 };
const sortValue = (g: Game, k: SortKey): string | number =>
  k === 'tier' ? TIER_RANK[tierFilterValue(g)] ?? -1 : k === 'title' ? sortKey(g) : k === 'year' ? g.release_year ?? -1 : k === 'hours' ? hoursPlayed(g) ?? -1 : k === 'size' ? (g.is_installed ? g.size_bytes ?? -1 : -1) : g.last_played_at ?? -1;

/** Source chips come from what the library really holds: no chip for a store with zero games. Sideloaded games get their chip last. */
export function sourceChips(games: Game[]): (StoreType | 'all')[] {
  const present = new Set(games.map((g) => g.store));
  return ['all', ...CORE_SOURCES.filter((s) => present.has(s)), ...[...present].filter((s) => s !== 'custom' && !CORE_SOURCES.includes(s)), ...(present.has('custom') ? ['custom' as StoreType] : [])];
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
const SORT_KEYS: (SortKey | '')[] = ['', 'title', 'year', 'last', 'hours', 'size', 'tier'];
const defaultDir = (k: SortKey): Sort['dir'] => (k === 'title' ? 'asc' : 'desc');

function sortOptionLabel(t: T, k: SortKey | ''): string {
  switch (k) {
    case '':
      return t('library:sortDefault');
    case 'title':
      return t('library:sortOption.title');
    case 'year':
      return t('library:sortOption.year');
    case 'last':
      return t('library:sortOption.recent');
    case 'hours':
      return t('library:sortOption.hours');
    case 'size':
      return t('library:sortOption.size');
    case 'tier':
      return t('library:sortOption.tier');
  }
}

const DEFAULT_SORT = 'default'; // Radix forbids '' as an item value

function SortControl({ sort, onChange }: { sort: Sort | null; onChange: (s: Sort | null) => void }) {
  const { t } = useTranslation('library');
  const asc = sort?.dir === 'asc';
  return (
    <div className="flex items-center gap-1.5">
      <Select
        value={sort?.key ?? DEFAULT_SORT}
        onValueChange={(v) => onChange(v === DEFAULT_SORT ? null : { key: v as SortKey, dir: defaultDir(v as SortKey) })}
      >
        <SelectTrigger aria-label={t('library:sortBy')} className="w-44 uppercase tracking-wider">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SORT_KEYS.map((v) => <SelectItem key={v || DEFAULT_SORT} value={v || DEFAULT_SORT}>{v ? sortOptionLabel(t, v) : sortOptionLabel(t, '')}</SelectItem>)}
        </SelectContent>
      </Select>
      {sort && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="neutral"
              size="icon"
              aria-label={asc ? t('library:ascToDesc') : t('library:descToAsc')}
              onClick={() => onChange({ ...sort, dir: asc ? 'desc' : 'asc' })}
            >
              {asc ? <ArrowUp className="h-3.5 w-3.5" /> : <ArrowDown className="h-3.5 w-3.5" />}
            </Button>
          </TooltipTrigger>
          <TooltipContent>{asc ? t('library:ascending') : t('library:descending')}</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

/** A row of chips with its name in front, so each group reads as its own control (two "all" chips are not one). */
function FilterGroup({ label, labelClass, children }: { label: string; labelClass?: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <MonoLabel className={cx('flex-none', labelClass)}>{label}</MonoLabel>
      {children}
    </div>
  );
}

function tierOptionLabel(t: T, v: TierFilter): string {
  switch (v) {
    case 'all':
      return t('library:tierAll');
    case 'native':
      return t('library:tierNative');
    case 'none':
      return t('library:tierNone');
    case 'platinum':
      return t('library:tierName.platinum');
    case 'gold':
      return t('library:tierName.gold');
    case 'silver':
      return t('library:tierName.silver');
    case 'bronze':
      return t('library:tierName.bronze');
    case 'borked':
      return t('library:tierName.borked');
  }
}

/** Filters the library by compatibility: native builds, a ProtonDB tier, or none. */
function TierFilterControl({ value, onChange }: { value: TierFilter; onChange: (v: TierFilter) => void }) {
  const { t } = useTranslation('library');
  return (
    <Select value={value} onValueChange={(v) => onChange(v as TierFilter)}>
      <SelectTrigger aria-label={t('library:tierFilterAria')} className="w-40 uppercase tracking-wider">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TIER_FILTERS.map((v) => (
          <SelectItem key={v} value={v}>
            {tierOptionLabel(t, v)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** The status the user gave a game (issue #51): 'all' and 'none' are filter
 *  values only; the rest are the game statuses. */
type StatusFilter = 'all' | 'none' | 'backlog' | 'playing' | 'finished' | 'abandoned';
const STATUS_FILTERS: StatusFilter[] = ['all', 'none', 'backlog', 'playing', 'finished', 'abandoned'];

function statusOptionLabel(t: T, v: StatusFilter): string {
  switch (v) {
    case 'all':
      return t('library:statusFilter.all');
    case 'none':
      return t('library:statusFilter.none');
    case 'backlog':
      return t('library:statusFilter.backlog');
    case 'playing':
      return t('library:statusFilter.playing');
    case 'finished':
      return t('library:statusFilter.finished');
    case 'abandoned':
      return t('library:statusFilter.abandoned');
  }
}

/** The status badge label of a game card: null when it has no status, so the
 *  card shows no badge (issue #51). */
function statusBadgeLabel(t: T, status: string | undefined): string | null {
  switch (status) {
    case 'backlog':
      return t('library:statusFilter.backlog');
    case 'playing':
      return t('library:statusFilter.playing');
    case 'finished':
      return t('library:statusFilter.finished');
    case 'abandoned':
      return t('library:statusFilter.abandoned');
    default:
      return null;
  }
}

/** Filters the library by the user's own status (issue #51): all games, only
 *  the ones without one, or only one status. */
function StatusFilterControl({ value, onChange }: { value: StatusFilter; onChange: (v: StatusFilter) => void }) {
  const { t } = useTranslation('library');
  return (
    <Select value={value} onValueChange={(v) => onChange(v as StatusFilter)}>
      <SelectTrigger aria-label={t('library:statusFilterAria')} className="w-40 uppercase tracking-wider">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {STATUS_FILTERS.map((v) => (
          <SelectItem key={v} value={v}>
            {statusOptionLabel(t, v)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function jobLabel(t: T, j: Job): string {
  if (j.state === 'queued') return t('library:job.queued', { kind: j.kind });
  if (j.percent >= 0) {
    const percent = formatCount(Math.round(j.percent));
    return j.kind === 'uninstall' ? t('library:job.removing', { percent }) : t('library:job.installing', { percent });
  }
  return j.phase ?? j.kind;
}

/** The store name for display: brand stores keep their name, sideloaded games translate. */
function displayStore(t: T, s: string): string {
  return s === 'custom' ? t('library:storeSideloaded') : storeLabel(s);
}

/** The compact recency word for a game ("today", "5d ago"), through the library catalog. */
function recencyWord(t: T, g: Game): string | undefined {
  const r = recencyOf(g.last_played_at);
  return r ? renderRecency(t, 'library:recency', r) : undefined;
}

const TABLE_COLS = 'grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_6rem_4rem_7rem]';

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

/** Every card is as tall as its cover plus this footer, so the grid can leave the rows
 *  out of view unrendered. Room for the title, the details line and two lines of
 *  compat badges (the badge row keeps a stable height, see GameCardBody). */
const CARD_FOOTER = 112;
const CARD_GAP = 12;
const CARD_MIN_WIDTH = 140;
const cardRowHeight = (width: number) => width * 1.5 + CARD_FOOTER;
const TABLE_ROW_HEIGHT = 44;

const GameCard = memo(function GameCard({ game: g, job, onOpen, onManage }: { game: Game; job?: Job; onOpen: (id: string) => void; onManage?: (id: string) => void }) {
  const { t } = useTranslation('library');
  const hours = hoursPlayed(g);
  const statusLabel = statusBadgeLabel(t, g.status);
  const meta = [hours != null && t('library:cardHours', { hours: formatDecimal1(hours) }), recencyWord(t, g), displayStore(t, g.store), g.release_year ? String(g.release_year) : null, g.update_available && t('library:cardUpdate')].filter(Boolean).join(' · ');
  return (
    <button
      onClick={() => onOpen(g.id)}
      onContextMenu={onManage ? (e) => { e.preventDefault(); onManage(g.id); } : undefined}
      className={cx(cardClass, 'card-compat flex h-full min-w-0 flex-col')}
    >
      <div className="relative flex-none">
        <Cover src={coverImages(g)} title={g.title} className={cx('aspect-[2/3] w-full', !g.is_installed && !job && 'opacity-45 grayscale-[.6]')} />
        {needsAttention(g) && <TweakDot className="absolute right-2 top-2" />}
        {g.shared_by && (
          <Badge variant="neutral" className="absolute left-2 top-2 max-w-[85%] truncate bg-black/65 text-white/80">{t('library:sharedBy', { name: g.shared_by.name })}</Badge>
        )}
        {job ? (
          <div className="absolute inset-x-2 bottom-2 rounded-lg bg-black/70 px-2 py-1.5">
            <div className="font-mono text-micro uppercase tracking-wider text-amber-200">{jobLabel(t, job)}</div>
            <div className="mt-1 h-1 rounded-full bg-white/15">
              <div className="h-full rounded-full bg-accent-strong" style={{ width: `${Math.max(job.percent, 2)}%` }} />
            </div>
          </div>
        ) : (
          (g.status || !g.is_installed) && (
            <div className="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] flex-wrap gap-1">
              {statusLabel && (
                <Badge variant="neutral" className="bg-black/65 text-text-body" data-testid="status-badge">
                  {statusLabel}
                </Badge>
              )}
              {!g.is_installed && (
                <Badge variant="neutral" className="gap-1 bg-black/65 text-text-body">
                  {g.installable && <Download className="h-2.5 w-2.5" />} {g.installable ? t('library:cardNotInstalled') : t('library:cardOwned')}
                </Badge>
              )}
            </div>
          )
        )}
      </div>
      <GameCardBody title={g.title} meta={meta} game={g} className="px-2.5 py-2" />
    </button>
  );
});

const TableRow = memo(function TableRow({ game: g, index, runners, onOpen, onManage }: { game: Game; index: number; runners: ProtonVersion[]; onOpen: (id: string) => void; onManage?: (id: string) => void }) {
  const { t } = useTranslation('library');
  const tier = tierFilterValue(g);
  return (
    <div
      role="row"
      aria-rowindex={index + 2}
      onContextMenu={onManage ? (e) => { e.preventDefault(); onManage(g.id); } : undefined}
      className={cx('grid h-11 items-center gap-3 px-4 transition hover:bg-accent-strong/[0.05]', TABLE_COLS, index % 2 && 'bg-white/[0.015]')}
    >
      <span role="cell" className={cx('h-1.5 w-1.5 rounded-full', TIER_DOT[tier ?? ''] ?? 'bg-white/15')} />
      <span role="cell" className="min-w-0">
        <button onClick={() => onOpen(g.id)} className="group flex w-full min-w-0 items-center gap-2.5 text-left">
          <Cover src={coverImages(g)} title="" className="h-7 w-7 flex-none rounded ring-1 ring-white/10" />
          <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
          {g.shared_by && <Badge variant="neutral" className="flex-none">{t('library:sharedBy', { name: g.shared_by.name })}</Badge>}
        </button>
      </span>
      <span role="cell" className={cx('font-mono text-hint', TIER_TEXT[tier ?? ''] ?? 'text-text-muted')}>{tier ?? '—'}</span>
      <span role="cell" className="truncate font-mono text-hint text-text-muted">{runnerShort(g.settings.runner_id, runners)}</span>
      <span role="cell" className="text-right font-mono text-hint tabular-nums text-text-muted">{hoursPlayed(g) ?? '—'}</span>
      <span role="cell" className="text-right font-mono text-hint tabular-nums text-text-muted">{g.release_year ?? '—'}</span>
      <span role="cell" className="text-right font-mono text-hint text-text-muted">{(g.is_installed ? formatBytes(g.size_bytes) : t('library:table.notInstalled')) || '—'}</span>
      <span role="cell" className="text-right font-mono text-hint text-text-muted">{recencyWord(t, g) ?? '—'}</span>
    </div>
  );
});

export function LibraryView(p: LibraryProps) {
  const { t } = useTranslation(['library', 'nav', 'collections']);
  const [sort, setSort] = useState<Sort | null>(null);
  const [tier, setTier] = useState<TierFilter>('all');
  // The user's own per-game status (issue #51): all games, none, or one.
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  // Family: the games borrowed from a family member, when the library has any.
  const [lending, setLending] = useState<'all' | 'with-me'>('all');
  const hasBorrowed = p.games.some((g) => g.shared_by != null);
  // The filter is only in effect while the control shows: once the borrowed
  // games are gone (uninstalled, logged out) it would hide the whole library.
  const lendingFilter = hasBorrowed ? lending : 'all';
  // The less-used filters sit behind the Filters toggle; the toggle counts the ones that are in effect.
  const [filtersOpen, setFiltersOpen] = useState(false);
  // The collection the library shows (issue #45), evaluated on the loaded games only.
  const activeCollection = p.collections?.find((c) => c.id === p.collection);
  const inCollection = useMemo(
    () => collectionPredicate(p.collections ?? [], p.collection ?? '', Math.floor(Date.now() / 1000)),
    [p.collections, p.collection]
  );
  const extraFilters = [p.ownership === 'owned', lendingFilter !== 'all', tier !== 'all', statusFilter !== 'all', inCollection != null, sort !== null].filter(Boolean).length;
  // Typing in the search box paints the box first and filters the library after.
  const q = useDeferredValue(p.search).trim().toLowerCase();
  const owned = p.games.length;
  const installedCount = useMemo(() => p.games.filter((g) => g.is_installed).length, [p.games]);
  const sources = useMemo(() => sourceChips(p.games), [p.games]);
  // A store can drop to zero games (logout, uninstall) while selected: its chip is gone, so show all.
  const source = sources.includes(p.source) ? p.source : 'all';
  const filtered = useMemo(
    () =>
      p.games.filter(
        (g) =>
          (p.ownership === 'owned' || g.is_installed) &&
          (source === 'all' || g.store === source) &&
          (lendingFilter === 'all' || g.shared_by != null) &&
          (tier === 'all' || tierFilterValue(g) === tier) &&
          (statusFilter === 'all' || (statusFilter === 'none' ? !g.status : g.status === statusFilter)) &&
          (!inCollection || inCollection(g)) &&
          g.title.toLowerCase().includes(q)
      ),
    [p.games, p.ownership, source, lendingFilter, tier, statusFilter, inCollection, q]
  );
  // A manual collection's games that left the library: listed, not shown as cards.
  const missing = useMemo(() => (activeCollection && !activeCollection.smart ? missingNames(activeCollection, p.games) : []), [activeCollection, p.games]);
  const jobs = useMemo(() => new Map(p.jobs.map((j) => [j.game_id, j])), [p.jobs]);
  const pluginName = (id: string) => p.plugins.find((x) => x.id === id)?.name ?? id;
  // The Last played strip belongs to the default order; an explicit sort shows one ordered grid.
  const played = useMemo(() => (sort ? [] : filtered.filter((g) => g.last_played_at).sort(byRecent).slice(0, 3)), [filtered, sort]);
  const tableRows = useMemo(() => sortGames(filtered, sort), [filtered, sort]);
  const toggleSort = (key: SortKey) =>
    setSort((s) => (s?.key === key ? { key, dir: s.dir === 'asc' ? 'desc' : 'asc' } : { key, dir: defaultDir(key) }));
  const rest = useMemo(() => (sort ? tableRows : filtered).filter((g) => !played.includes(g)), [sort, tableRows, filtered, played]);
  const maxHours = useMemo(() => Math.max(1, ...p.games.map((g) => hoursPlayed(g) ?? 0)), [p.games]);
  const recentlyAdded = useMemo(() => p.games.filter((g) => g.store === 'custom').slice(0, 6), [p.games]);
  // Offline (issue #44): a small indicator in the toolbar while the machine
  // looks offline — the library shows the last known listing.
  const { online } = useNetState();

  return (
    <div className="p-7">
      <ViewHeader
        title={t('nav:title.library')}
        subtitle={<><p className="mt-0.5 text-[12.5px] tabular-nums text-text-body">
              {t('library:counts', { shown: formatCount(filtered.length), installed: formatCount(installedCount) })}
              {owned > installedCount ? t('library:ownedSuffix', { owned: formatCount(owned) }) : ''}
              {p.lastScan ? t('library:scannedSuffix', { time: clock(p.lastScan) }) : ''}
            </p>
            {activeCollection && (
              <p className="mt-1.5 flex flex-wrap items-center gap-2 text-[12.5px] text-text-body">
                <Badge variant="accent" className="gap-1.5" data-testid="collection-chip">
                  {activeCollection.builtin ? t(`collections:builtin.${activeCollection.id}`) : activeCollection.name}
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <button type="button" aria-label={t('collections:clearCollection')} onClick={() => p.onCollection?.(undefined)} className="rounded-full hover:text-white">
                        <X className="h-3 w-3" aria-hidden />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent>{t('collections:clearCollection')}</TooltipContent>
                  </Tooltip>
                </Badge>
                {missing.length > 0 && (
                  <span className="text-amber-200" title={missing.join(', ')} data-testid="collection-missing">
                    {t('collections:missing', { count: missing.length })}
                  </span>
                )}
              </p>
            )}
            {!online && (
              <p role="status" className="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-surface-strong px-2.5 py-1 text-[12px] font-semibold text-white/80 ring-1 ring-white/[0.1]">
                <WifiOff className="h-3.5 w-3.5" aria-hidden />
                {t('library:offline', 'Offline — showing the last known library')}
              </p>
            )}</>}
        actions={<div className="flex flex-wrap items-center gap-2.5">
            <FilterGroup label={t('library:view')}>
              <SegRow label={t('library:view')} value={p.mode} onChange={p.onMode} options={[{ value: 'cards', label: t('library:mode.cards') }, { value: 'table', label: t('library:mode.table') }]} />
            </FilterGroup>
            <Button variant="primary" onClick={p.onAddGame}>
              <Plus className="h-3.5 w-3.5" strokeWidth={3} /> {t('library:addGame')}
            </Button>
          </div>}
        below={<>
        {/* One row: search, the store, and a Filters toggle for the less-used groups (each keeps its label). */}
        <div className="flex flex-wrap items-center gap-2.5">
          <div className="relative">
            <Input
              type="search"
              aria-label={t('library:searchAria')}
              value={p.search}
              onChange={(e) => p.onSearch(e.target.value)}
              placeholder={t('library:searchPlaceholder')}
              className="w-48 pr-9 [&::-webkit-search-cancel-button]:hidden"
            />
            {p.search && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={t('library:clearSearch')}
                    onClick={() => p.onSearch('')}
                    className="absolute right-1 top-1/2 size-7 -translate-y-1/2 text-text-muted"
                  >
                    <X className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('library:clearSearch')}</TooltipContent>
              </Tooltip>
            )}
          </div>
          {/* One store (or none) has nothing to choose between: the chips show only with two or more. */}
          {sources.length > 2 && (
            <FilterGroup label={t('library:store')}>
              <SegRow
                label={t('library:store')}
                value={source}
                onChange={p.onSource}
                options={sources.map((s) => ({
                  value: s,
                  label:
                    s === 'all' ? (
                      t('library:all')
                    ) : s === 'custom' ? (
                      <span className="inline-flex items-center gap-1.5 leading-none">
                        <PackagePlus className="h-4 w-4" aria-hidden />
                        <span>{t('library:sideloaded')}</span>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5">
                        <PluginLogo src={p.sourceLogos?.[s]} name={storeLabel(s)} size="xs" />
                        <span>{s}</span>
                      </span>
                    ),
                }))}
              />
            </FilterGroup>
          )}
          <Button
            variant="neutral"
            aria-expanded={filtersOpen}
            aria-controls="library-filters"
            onClick={() => setFiltersOpen((o) => !o)}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" /> {t('library:filters')}
            {extraFilters > 0 && <Badge variant="accent" className="h-5 rounded-full tracking-normal tabular-nums">{extraFilters}</Badge>}
          </Button>
        </div>

        {filtersOpen && (
          // One row per group, labels in a shared column: the panel never wraps a group onto a second line.
          <div id="library-filters" className="flex flex-col items-start gap-2.5 rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline">
            {owned > installedCount && (
              <FilterGroup label={t('library:show')} labelClass="w-14">
                <SegRow label={t('library:show')} value={p.ownership} onChange={p.onOwnership} options={[{ value: 'installed', label: t('library:installed') }, { value: 'owned', label: t('library:owned') }]} />
              </FilterGroup>
            )}
            {hasBorrowed && (
              <FilterGroup label={t('library:family')} labelClass="w-14">
                <SegRow label={t('library:family')} value={lendingFilter} onChange={(v) => setLending(v)} options={[{ value: 'all', label: t('library:all') }, { value: 'with-me', label: t('library:sharedWithMe') }]} />
              </FilterGroup>
            )}
            <FilterGroup label={t('library:tier')} labelClass="w-14">
              <TierFilterControl value={tier} onChange={setTier} />
            </FilterGroup>
            <FilterGroup label={t('library:statusFilter.label')} labelClass="w-14">
              <StatusFilterControl value={statusFilter} onChange={setStatusFilter} />
            </FilterGroup>
            {p.collections && p.onCollection && (
              <FilterGroup label={t('collections:filterLabel')} labelClass="w-14">
                <Select value={activeCollection?.id ?? 'all'} onValueChange={(v) => p.onCollection?.(v === 'all' ? undefined : v)}>
                  <SelectTrigger aria-label={t('collections:filterAria')} className="w-48">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">{t('collections:allGames')}</SelectItem>
                    {p.collections.map((c) => (
                      <SelectItem key={c.id} value={c.id}>
                        {c.builtin ? t(`collections:builtin.${c.id}`) : c.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FilterGroup>
            )}
            <FilterGroup label={t('library:sort')} labelClass="w-14">
              <SortControl sort={sort} onChange={setSort} />
            </FilterGroup>
          </div>
        )}
        </>}
      />

      {p.warnings.length > 0 && (
        <div className="mb-6 space-y-2">
          {p.warnings.map((w) => (
            <div key={w.source} role="status" className="flex items-center justify-between gap-4 rounded-2xl bg-accent-strong/[0.07] px-4 py-3 ring-1 ring-accent-strong/25">
              <p className="min-w-0 text-[12.5px] text-white/75">
                {w.needs_auth ? (
                  <Trans i18nKey="library:warningLoggedOut" values={{ name: pluginName(w.source) }} components={{ name: <span className="font-bold text-amber-200" /> }} />
                ) : w.unavailable ? (
                  <Trans
                    i18nKey="library:warningUnavailable"
                    values={{ name: pluginName(w.source), message: w.message }}
                    components={{
                      name: <span className="font-bold text-amber-200" />,
                      msg: <span className="font-mono text-[12px] text-text-muted" />,
                    }}
                  />
                ) : (
                  <Trans
                    i18nKey="library:warningListFailed"
                    values={{ name: pluginName(w.source), message: w.message }}
                    components={{
                      name: <span className="font-bold text-amber-200" />,
                      msg: <span className="font-mono text-[12px] text-text-muted" />,
                    }}
                  />
                )}
              </p>
              <Button variant="primary" size="sm" className="flex-none" onClick={() => p.onNavigate({ name: 'plugins' })}>
                {w.needs_auth && <LogIn className="h-3.5 w-3.5" />} {w.needs_auth ? t('library:login') : t('library:plugins')}
              </Button>
            </div>
          ))}
        </div>
      )}

      {filtered.length === 0 ? (
        <EmptyState label={t('library:empty')}>
          {p.games.length === 0
            ? t('library:emptyNoGames')
            : p.ownership === 'installed' && installedCount === 0
              ? t('library:emptyNotInstalled')
              : t('library:emptyNoMatch')}
        </EmptyState>
      ) : p.mode === 'cards' ? (
        <>
          {played.length > 0 && (
            <section className="mb-8">
              <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-text-muted">{t('library:lastPlayed')}</h2>
              <div className="grid gap-4 lg:grid-cols-3">
                {played.map((g) => (
                  <button key={g.id} onClick={() => p.onOpenGame(g.id)} className="group relative overflow-hidden rounded-3xl text-left ring-1 ring-hairline transition hover:ring-accent-strong/40">
                    <Cover src={heroImages(g)} title={g.title} className="h-[210px] w-full transition duration-500 group-hover:scale-[1.04]" />
                    <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.95) 8%, transparent 60%)' }} />
                    <div className="absolute bottom-4 left-5 right-5">
                      <div className="flex items-baseline justify-between gap-3">
                        <span className="truncate text-[16px] font-extrabold tracking-tight">{g.title}</span>
                        {g.fix_info?.tier && <TierBadge tier={g.fix_info.tier} title={tierMeaning(t, g.fix_info.tier)} />}
                      </div>
                      <div className="mt-1 flex items-center gap-2 font-mono text-hint text-text-muted">
                        {hoursPlayed(g) != null && <><span>{t('library:cardHours', { hours: formatDecimal1(hoursPlayed(g)!) })}</span><span>·</span></>}
                        <span>{recencyWord(t, g)}</span><span>·</span><span>{runnerShort(g.settings.runner_id, p.runners)}</span>
                      </div>
                      {hoursPlayed(g) != null && (
                        <div className="mt-2.5 h-1 rounded-full bg-white/[0.14]">
                          <div className="h-full rounded-full bg-accent-strong/80" style={{ width: `${Math.min(100, (hoursPlayed(g)! / maxHours) * 100)}%` }} />
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
            <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-text-muted">{t('library:allTitles')}</h2>
            <WindowedGrid
              items={rest}
              itemKey={(g) => g.id}
              minColumn={CARD_MIN_WIDTH}
              gap={CARD_GAP}
              rowHeight={cardRowHeight}
              className="cover-grid-sm"
              renderItem={(g) => <GameCard game={g} job={jobs.get(g.id)} onOpen={p.onOpenGame} onManage={p.onManageCollections} />}
            />
          </section>

          <section className={cx('mb-4 grid gap-5', recentlyAdded.length > 0 && 'xl:grid-cols-[1.5fr_1fr]')}>
            <ActivityPanel events={p.activity} limit={6} onFullLog={() => p.onNavigate({ name: 'activity' })} />
            {recentlyAdded.length > 0 && (
              <Panel className="bg-surface">
                <h2 className="mb-3 text-[14px] font-bold">{t('library:recentlyAdded')}</h2>
                <RecentList games={recentlyAdded} onOpenGame={p.onOpenGame} />
              </Panel>
            )}
          </section>
        </>
      ) : (
        <section className="mb-8 overflow-x-auto rounded-2xl bg-black/25 ring-1 ring-hairline">
          <div role="table" aria-label={t('library:table.label')} aria-rowcount={tableRows.length + 1} className="min-w-[800px]">
            <div role="row" aria-rowindex={1} className={cx('grid gap-3 border-b border-hairline px-4 py-2 font-mono text-micro uppercase tracking-[0.12em] text-text-muted', TABLE_COLS)}>
              <span role="columnheader"><span className="sr-only">{t('library:table.statusSr')}</span></span>
              <SortHead k="title" label={t('library:table.title')} sort={sort} onSort={toggleSort} />
              <SortHead k="tier" label={t('library:table.tier')} sort={sort} onSort={toggleSort} /><span role="columnheader">{t('library:table.runner')}</span>
              <SortHead k="hours" label={t('library:table.hours')} sort={sort} onSort={toggleSort} right />
              <SortHead k="year" label={t('library:table.year')} sort={sort} onSort={toggleSort} right />
              <SortHead k="size" label={t('library:table.size')} sort={sort} onSort={toggleSort} right />
              <SortHead k="last" label={t('library:table.lastPlayed')} sort={sort} onSort={toggleSort} right />
            </div>
            <WindowedRows
              items={tableRows}
              itemKey={(g) => g.id}
              rowHeight={TABLE_ROW_HEIGHT}
              renderRow={(g, i) => <TableRow game={g} index={i} runners={p.runners} onOpen={p.onOpenGame} onManage={p.onManageCollections} />}
            />
          </div>
        </section>
      )}
    </div>
  );
}
