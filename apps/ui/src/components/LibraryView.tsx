// i18n: converted
import { useTranslation, Trans } from 'react-i18next';
import React, { useDeferredValue, useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, LogIn, PackagePlus, Play, Plus, SlidersHorizontal, WifiOff, X } from 'lucide-react';
import type { ActivityEvent, Collection, Game, Job, ProtonVersion, SourceWarning, StorePlugin, StoreType, View } from '../types';
import { ActivityPanel } from './ActivityFeed';
import { RecentList } from './Dashboard';
import { Cover, EmptyState, MonoLabel, Panel, SegRow, TierBadge, clock, cx } from './ui';
import { tierMeaning } from './CompatBadges';
import { TIER_FILTERS, tierFilterValue, type TierFilter } from '../lib/compat';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { byRecent, heroImages, hoursPlayed, storeLabel } from '../lib/games';
import { collectionPredicate, missingNames } from '../lib/collections';
import { formatCount } from '../lib/format';
import { formatDecimal1, recencyOf, renderRecency } from '../lib/gameLocale';
import { runnerShort } from '../lib/settings';
import { sortKey } from '../lib/customize';
import { useNetState } from './useNetState';
import { WindowedGrid, WindowedRows } from './WindowedList';
import { PluginLogo } from './PluginLogo';
import { ThemeLayout } from '../lib/layout';
import { useThemeLayout, useVariant } from '../lib/theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { GameRow, GameTile } from './GameTile';
import type { ThemeLayout as LibraryLayoutData } from '../types';

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
  /** Multi-store selection for the sourcebar (issue #125): empty = every
      store. When defined, it replaces the single source above. */
  storeSelection?: StoreType[];
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

/** Atlas styling for a filter select (issue #132): Albert Sans, human values,
 *  no uppercase or mono. Warm keeps the default trigger and item classes. */
export interface FilterControlStyle {
  triggerClassName?: string;
  itemClassName?: string;
  /** Label for the 'all' option ("Any" in Atlas, "all" in warm). */
  allLabel?: string;
}

/** Filters the library by compatibility: native builds, a ProtonDB tier, or none. */
export function TierFilterControl({ value, onChange, triggerClassName = 'w-40 uppercase tracking-wider', itemClassName, allLabel }: {
  value: TierFilter; onChange: (v: TierFilter) => void;
} & FilterControlStyle) {
  const { t } = useTranslation('library');
  return (
    <Select value={value} onValueChange={(v) => onChange(v as TierFilter)}>
      <SelectTrigger aria-label={t('library:tierFilterAria')} className={triggerClassName}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {TIER_FILTERS.map((v) => (
          <SelectItem key={v} value={v} className={itemClassName}>
            {v === 'all' && allLabel ? allLabel : tierOptionLabel(t, v)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** The status the user gave a game (issue #51): 'all' and 'none' are filter
 *  values only; the rest are the game statuses. */
export type StatusFilter = 'all' | 'none' | 'backlog' | 'playing' | 'finished' | 'abandoned';
export const STATUS_FILTERS: StatusFilter[] = ['all', 'none', 'backlog', 'playing', 'finished', 'abandoned'];

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

/** Filters the library by the user's own status (issue #51): all games, only
 *  the ones without one, or only one status. */
export function StatusFilterControl({ value, onChange, triggerClassName = 'w-40 uppercase tracking-wider', itemClassName, allLabel }: {
  value: StatusFilter; onChange: (v: StatusFilter) => void;
} & FilterControlStyle) {
  const { t } = useTranslation('library');
  return (
    <Select value={value} onValueChange={(v) => onChange(v as StatusFilter)}>
      <SelectTrigger aria-label={t('library:statusFilterAria')} className={triggerClassName}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {STATUS_FILTERS.map((v) => (
          <SelectItem key={v} value={v} className={itemClassName}>
            {v === 'all' && allLabel ? allLabel : statusOptionLabel(t, v)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** The store name for display: brand stores keep their name, sideloaded games translate. */
function displayStore(t: T, s: string): string {
  return s === 'custom' ? t('library:storeSideloaded') : storeLabel(s);
}

/** The tier, status and lent-to-me filters as one predicate (issue #132):
 *  both library presentations filter through it, so the logic can't drift. */
export function matchesExtraFilters(g: Game, tier: TierFilter, status: StatusFilter, lending: 'all' | 'with-me'): boolean {
  return (
    (tier === 'all' || tierFilterValue(g) === tier) &&
    (status === 'all' || (status === 'none' ? !g.status : g.status === status)) &&
    (lending === 'all' || g.shared_by != null)
  );
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
const TABLE_ROW_HEIGHT = 44;


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
  // The sourcebar selects any number of stores (empty = all): when it names
  // stores, they replace the single source above. A selected store with no
  // games shows nothing, so it stays selected instead of falling back.
  const multi = p.storeSelection;
  const filtered = useMemo(
    () =>
      p.games.filter(
        (g) =>
          (p.ownership === 'owned' || g.is_installed) &&
          (multi !== undefined ? multi.length === 0 || multi.includes(g.store) : source === 'all' || g.store === source) &&
          matchesExtraFilters(g, tier, statusFilter, lendingFilter) &&
          (!inCollection || inCollection(g)) &&
          g.title.toLowerCase().includes(q)
      ),
    [p.games, p.ownership, source, multi, lendingFilter, tier, statusFilter, inCollection, q]
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

  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's library before the core answers over RPC.
  const layout = useThemeLayout('library') ?? BUILTIN_LAYOUTS['library'];
  // The activity and recent grid spans the bottom when the library holds
  // sideloaded games, like the built-in grid does; otherwise activity fills
  // the row on its own. Frames stay with the view: the layout only names
  // the row's blocks.
  const hasRecent = recentlyAdded.length > 0;
  return (
    <div className="p-7">
      <ThemeLayout
        layout={layout}
        blocks={libraryBlocks({ p, sort, setSort, tier, setTier, statusFilter, setStatusFilter, lending, setLending, filtersOpen, setFiltersOpen, activeCollection, extraFilters, owned, installedCount, sources, source, filtered, missing, jobs, pluginName, played, tableRows, toggleSort, rest, maxHours, recentlyAdded, online, hasBorrowed, lendingFilter }, layout)}
        className="grid gap-5 xl:grid-cols-[1.75fr_1fr]"
        regionClassName={libraryRegionClass}
        rowClassName={(row) =>
          row.some((n) => n.block === 'activity' || n.block === 'recent')
            ? hasRecent
              ? 'grid gap-5 xl:grid-cols-[1.5fr_1fr]'
              : 'grid gap-5'
            : undefined}
      />
    </div>
  );
}

/** One region's frame in a library layout (issue #108): the header and the
 *  catalog span the full width, like the built-in page; the main column
 *  breathes and the side column packs tighter for themes that split them.
 *  The top margin makes up the wrapper's gap, so the built-in spacing is
 *  exact. */
function libraryRegionClass(region: string): string {
  switch (region) {
    case 'top': return 'col-span-full min-w-0 space-y-6 mb-1';
    case 'bottom': return 'col-span-full min-w-0 space-y-8';
    case 'side': return 'min-w-0 space-y-5';
    default: return 'min-w-0 space-y-6';
  }
}

interface LibraryData {
  p: LibraryProps;
  sort: Sort | null;
  setSort: (s: Sort | null | ((s: Sort | null) => Sort | null)) => void;
  tier: TierFilter;
  setTier: (v: TierFilter) => void;
  statusFilter: StatusFilter;
  setStatusFilter: (v: StatusFilter) => void;
  lending: 'all' | 'with-me';
  setLending: (v: 'all' | 'with-me') => void;
  filtersOpen: boolean;
  setFiltersOpen: (v: boolean | ((o: boolean) => boolean)) => void;
  activeCollection: Collection | undefined;
  extraFilters: number;
  owned: number;
  installedCount: number;
  sources: (StoreType | 'all')[];
  source: StoreType | 'all';
  filtered: Game[];
  missing: string[];
  jobs: Map<string, Job>;
  pluginName: (id: string) => string;
  played: Game[];
  tableRows: Game[];
  toggleSort: (key: SortKey) => void;
  rest: Game[];
  maxHours: number;
  recentlyAdded: Game[];
  online: boolean;
  hasBorrowed: boolean;
  lendingFilter: 'all' | 'with-me';
}

// The library's layout blocks (issue #98): each takes its data as props only
// (filter state stays in the view above) and renders null when it has nothing
// to show, so a layout orders blocks without conditionals of its own.
function libraryBlocks(d: LibraryData, layout?: LibraryLayoutData): Record<string, React.ReactNode> {
  const { p } = d;
  return {
    header: <LibraryHeader d={d} />,
    title: (
      <LibraryTitle
        filteredCount={d.filtered.length}
        installedCount={d.installedCount}
        owned={d.owned}
        lastScan={p.lastScan}
        collection={d.activeCollection}
        missing={d.missing}
        online={d.online}
        onClearCollection={() => p.onCollection?.(undefined)}
      />
    ),
    mode: <LibraryMode mode={p.mode} onMode={p.onMode} onAddGame={p.onAddGame} />,
    search: <LibrarySearch search={p.search} onSearch={p.onSearch} />,
    chips: <LibraryChips sources={d.sources} source={d.source} onSource={p.onSource} sourceLogos={p.sourceLogos} />,
    filters: (
      <LibraryFilters
        filtersOpen={d.filtersOpen}
        setFiltersOpen={d.setFiltersOpen}
        extraFilters={d.extraFilters}
        owned={d.owned}
        installedCount={d.installedCount}
        ownership={p.ownership}
        onOwnership={p.onOwnership}
        hasBorrowed={d.hasBorrowed}
        lendingFilter={d.lendingFilter}
        setLending={d.setLending}
        tier={d.tier}
        setTier={d.setTier}
        statusFilter={d.statusFilter}
        setStatusFilter={d.setStatusFilter}
        collections={p.collections}
        onCollection={p.onCollection}
        activeCollection={d.activeCollection}
        sort={d.sort}
        setSort={d.setSort}
      />
    ),
    warnings: <LibraryWarnings warnings={p.warnings} pluginName={d.pluginName} onNavigate={p.onNavigate} />,
    empty: <LibraryEmpty filteredCount={d.filtered.length} gamesCount={p.games.length} installedCount={d.installedCount} ownership={p.ownership} />,
    'last-played': <LibraryLastPlayed played={d.played} runners={p.runners} maxHours={d.maxHours} onOpenGame={p.onOpenGame} mode={p.mode} filteredEmpty={d.filtered.length === 0} />,
    'all-titles': <LibraryAllTitles rest={d.rest} jobs={d.jobs} onOpenGame={p.onOpenGame} onManage={p.onManageCollections} mode={p.mode} filteredEmpty={d.filtered.length === 0} />,
    activity: <LibraryActivity events={p.activity} onNavigate={p.onNavigate} limit={layoutLimit(layout, 'activity', 6)} mode={p.mode} filteredEmpty={d.filtered.length === 0} />,
    recent: <LibraryRecent recentlyAdded={d.recentlyAdded} onOpenGame={p.onOpenGame} mode={p.mode} filteredEmpty={d.filtered.length === 0} />,
    table: <LibraryTable tableRows={d.tableRows} runners={p.runners} onOpenGame={p.onOpenGame} onManage={p.onManageCollections} sort={d.sort} onSort={d.toggleSort} mode={p.mode} filteredEmpty={d.filtered.length === 0} />,
  };
}

/** The layout's limit for a block, or the built-in default without one. The
 *  core validated the number; the UI parses it again so a bad bridge value
 *  can't widen a feed. */
function layoutLimit(layout: LibraryLayoutData | undefined, block: string, fallback: number): number {
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

/** The built-in library header (issue #108): the title, the mode switch
 *  with the add button, and the search, store and filters toolbar, arranged
 *  exactly as the built-in header shows them. It carries no outer margin:
 *  the top region's frame spaces it from the warnings below. */
export function LibraryHeader({ d }: { d: LibraryData }) {
  const { p } = d;
  return (
    <div>
      <div className="flex flex-wrap items-end justify-between gap-4">
        <LibraryTitle
          filteredCount={d.filtered.length}
          installedCount={d.installedCount}
          owned={d.owned}
          lastScan={p.lastScan}
          collection={d.activeCollection}
          missing={d.missing}
          online={d.online}
          onClearCollection={() => p.onCollection?.(undefined)}
        />
        <LibraryMode mode={p.mode} onMode={p.onMode} onAddGame={p.onAddGame} />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2.5">
        <LibrarySearch search={p.search} onSearch={p.onSearch} />
        <LibraryChips sources={d.sources} source={d.source} onSource={p.onSource} sourceLogos={p.sourceLogos} />
        <LibraryFilters
          filtersOpen={d.filtersOpen}
          setFiltersOpen={d.setFiltersOpen}
          extraFilters={d.extraFilters}
          owned={d.owned}
          installedCount={d.installedCount}
          ownership={p.ownership}
          onOwnership={p.onOwnership}
          hasBorrowed={d.hasBorrowed}
          lendingFilter={d.lendingFilter}
          setLending={d.setLending}
          tier={d.tier}
          setTier={d.setTier}
          statusFilter={d.statusFilter}
          setStatusFilter={d.setStatusFilter}
          collections={p.collections}
          onCollection={p.onCollection}
          activeCollection={d.activeCollection}
          sort={d.sort}
          setSort={d.setSort}
        />
      </div>
    </div>
  );
}

export function LibraryTitle({ filteredCount, installedCount, owned, lastScan, collection, missing, online, onClearCollection }: {
  filteredCount: number;
  installedCount: number;
  owned: number;
  lastScan: number | null;
  collection?: Collection;
  missing: string[];
  online: boolean;
  onClearCollection: () => void;
}) {
  const { t } = useTranslation(['library', 'nav', 'collections']);
  const showSubtitle = useVariant('view-header.subtitle');
  const size = useVariant('view-header.size');
  const titleClass = size === 'compact' ? 'text-[20px] font-extrabold tracking-tight' : 'text-[23px] font-extrabold tracking-tight';
  return (
    <div>
      <h1 className={titleClass}>{t('nav:title.library')}</h1>
      {showSubtitle === 'shown' && (
        <>
          <p className="mt-0.5 text-[12.5px] tabular-nums text-text-body">
            {t('library:counts', { shown: formatCount(filteredCount), installed: formatCount(installedCount) })}
            {owned > installedCount ? t('library:ownedSuffix', { owned: formatCount(owned) }) : ''}
            {lastScan ? t('library:scannedSuffix', { time: clock(lastScan) }) : ''}
          </p>
          {collection && (
            <p className="mt-1.5 flex flex-wrap items-center gap-2 text-[12.5px] text-text-body">
              <Badge variant="accent" className="gap-1.5" data-testid="collection-chip">
                {collection.builtin ? t(`collections:builtin.${collection.id}`) : collection.name}
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button type="button" aria-label={t('collections:clearCollection')} onClick={onClearCollection} className="rounded-full hover:text-white">
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
          )}
        </>
      )}
    </div>
  );
}

export function LibraryMode({ mode, onMode, onAddGame }: { mode: LibraryMode; onMode: (m: LibraryMode) => void; onAddGame: () => void }) {
  const { t } = useTranslation('library');
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <FilterGroup label={t('library:view')}>
        <SegRow label={t('library:view')} value={mode} onChange={onMode} options={[{ value: 'cards', label: t('library:mode.cards') }, { value: 'table', label: t('library:mode.table') }]} />
      </FilterGroup>
      <Button variant="primary" onClick={onAddGame}>
        <Plus className="h-3.5 w-3.5" strokeWidth={3} /> {t('library:addGame')}
      </Button>
    </div>
  );
}

export function LibrarySearch({ search, onSearch }: { search: string; onSearch: (q: string) => void }) {
  const { t } = useTranslation('library');
  return (
    <div className="relative">
      <Input
        type="search"
        aria-label={t('library:searchAria')}
        value={search}
        onChange={(e) => onSearch(e.target.value)}
        placeholder={t('library:searchPlaceholder')}
        className="w-48 pr-9 [&::-webkit-search-cancel-button]:hidden"
      />
      {search && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t('library:clearSearch')}
              onClick={() => onSearch('')}
              className="absolute right-1 top-1/2 size-7 -translate-y-1/2 text-text-muted"
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('library:clearSearch')}</TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

export function LibraryChips({ sources, source, onSource, sourceLogos }: { sources: (StoreType | 'all')[]; source: StoreType | 'all'; onSource: (s: StoreType | 'all') => void; sourceLogos?: Record<string, string> }) {
  const { t } = useTranslation('library');
  // One store (or none) has nothing to choose between: the chips show only with two or more.
  if (sources.length <= 2) return null;
  return (
    <FilterGroup label={t('library:store')}>
      <SegRow
        label={t('library:store')}
        value={source}
        onChange={onSource}
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
                <PluginLogo src={sourceLogos?.[s]} name={storeLabel(s)} size="xs" />
                <span>{s}</span>
              </span>
            ),
        }))}
      />
    </FilterGroup>
  );
}

export function LibraryFilters({ filtersOpen, setFiltersOpen, extraFilters, owned, installedCount, ownership, onOwnership, hasBorrowed, lendingFilter, setLending, tier, setTier, statusFilter, setStatusFilter, collections, onCollection, activeCollection, sort, setSort }: {
  filtersOpen: boolean;
  setFiltersOpen: (v: boolean | ((o: boolean) => boolean)) => void;
  extraFilters: number;
  owned: number;
  installedCount: number;
  ownership: Ownership;
  onOwnership: (o: Ownership) => void;
  hasBorrowed: boolean;
  lendingFilter: 'all' | 'with-me';
  setLending: (v: 'all' | 'with-me') => void;
  tier: TierFilter;
  setTier: (v: TierFilter) => void;
  statusFilter: StatusFilter;
  setStatusFilter: (v: StatusFilter) => void;
  collections?: Collection[];
  onCollection?: (id: string | undefined) => void;
  activeCollection?: Collection;
  sort: Sort | null;
  setSort: (s: Sort | null | ((s: Sort | null) => Sort | null)) => void;
}) {
  const { t } = useTranslation(['library', 'collections']);
  return (
    <div>
      <Button
        variant="neutral"
        aria-expanded={filtersOpen}
        aria-controls="library-filters"
        onClick={() => setFiltersOpen((o) => !o)}
      >
        <SlidersHorizontal className="h-3.5 w-3.5" /> {t('library:filters')}
        {extraFilters > 0 && <Badge variant="accent" className="h-5 rounded-full tracking-normal tabular-nums">{extraFilters}</Badge>}
      </Button>
      {filtersOpen && (
        // One row per group, labels in a shared column: the panel never wraps a group onto a second line.
        <div id="library-filters" className="mt-2.5 flex flex-col items-start gap-2.5 rounded-2xl bg-surface px-4 py-3 ring-1 ring-hairline">
          {owned > installedCount && (
            <FilterGroup label={t('library:show')} labelClass="w-14">
              <SegRow label={t('library:show')} value={ownership} onChange={onOwnership} options={[{ value: 'installed', label: t('library:installed') }, { value: 'owned', label: t('library:owned') }]} />
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
          {collections && onCollection && (
            <FilterGroup label={t('collections:filterLabel')} labelClass="w-14">
              <Select value={activeCollection?.id ?? 'all'} onValueChange={(v) => onCollection?.(v === 'all' ? undefined : v)}>
                <SelectTrigger aria-label={t('collections:filterAria')} className="w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t('collections:allGames')}</SelectItem>
                  {collections.map((c) => (
                    <SelectItem key={c.id} value={c.id}>
                      {c.builtin ? t(`collections:builtin.${c.id}`) : c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </FilterGroup>
          )}
          <FilterGroup label={t('library:sort')} labelClass="w-14">
            <SortControl sort={sort} onChange={(s) => setSort(s as Sort | null)} />
          </FilterGroup>
        </div>
      )}
    </div>
  );
}

export function LibraryWarnings({ warnings, pluginName, onNavigate }: { warnings: SourceWarning[]; pluginName: (id: string) => string; onNavigate: (v: View) => void }) {
  const { t } = useTranslation('library');
  if (warnings.length === 0) return null;
  return (
    <div className="space-y-2">
      {warnings.map((w) => (
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
          <Button variant="primary" size="sm" className="flex-none" onClick={() => onNavigate({ name: 'plugins' })}>
            {w.needs_auth && <LogIn className="h-3.5 w-3.5" />} {w.needs_auth ? t('library:login') : t('library:plugins')}
          </Button>
        </div>
      ))}
    </div>
  );
}

export function LibraryEmpty({ filteredCount, gamesCount, installedCount, ownership }: { filteredCount: number; gamesCount: number; installedCount: number; ownership: Ownership }) {
  const { t } = useTranslation('library');
  if (filteredCount !== 0) return null;
  return (
    <EmptyState label={t('library:empty')}>
      {gamesCount === 0
        ? t('library:emptyNoGames')
        : ownership === 'installed' && installedCount === 0
          ? t('library:emptyNotInstalled')
          : t('library:emptyNoMatch')}
    </EmptyState>
  );
}

export function LibraryLastPlayed({ played, runners, maxHours, onOpenGame, mode, filteredEmpty }: { played: Game[]; runners: ProtonVersion[]; maxHours: number; onOpenGame: (id: string) => void; mode: LibraryMode; filteredEmpty: boolean }) {
  const { t } = useTranslation('library');
  if (filteredEmpty || mode !== 'cards' || played.length === 0) return null;
  return (
    <section>
      <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-text-muted">{t('library:lastPlayed')}</h2>
      <div className="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,300px),1fr))]">
        {played.map((g) => (
          <button key={g.id} onClick={() => onOpenGame(g.id)} className="group relative overflow-hidden rounded-3xl text-left ring-1 ring-hairline transition hover:ring-accent-strong/40">
            <Cover src={heroImages(g)} title={g.title} className="h-[210px] w-full transition duration-500 group-hover:scale-[1.04]" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.95) 8%, transparent 60%)' }} />
            <div className="absolute bottom-4 left-5 right-5">
              <div className="flex items-baseline justify-between gap-3">
                <span className="truncate text-[16px] font-extrabold tracking-tight">{g.title}</span>
                {g.fix_info?.tier && <TierBadge tier={g.fix_info.tier} title={tierMeaning(t, g.fix_info.tier)} />}
              </div>
              <div className="mt-1 flex items-center gap-2 font-mono text-hint text-text-muted">
                {hoursPlayed(g) != null && <><span>{t('library:cardHours', { hours: formatDecimal1(hoursPlayed(g)!) })}</span><span>·</span></>}
                <span>{recencyWord(t, g)}</span><span>·</span><span>{runnerShort(g.settings.runner_id, runners)}</span>
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
  );
}

export function LibraryAllTitles({ rest, jobs, onOpenGame, onManage, mode, filteredEmpty }: { rest: Game[]; jobs: Map<string, Job>; onOpenGame: (id: string) => void; onManage?: (id: string) => void; mode: LibraryMode; filteredEmpty: boolean }) {
  const { t } = useTranslation('library');
  const aspect = useVariant('library-tile.aspect');
  if (filteredEmpty || mode !== 'cards') return null;
  const ratio = aspect === '2:3' ? 1.5 : aspect === '16:9' ? 9 / 16 : 4 / 3;
  // The footer matches the built-in cards (issue #109): the title, the
  // details line and the two badge lines, so the built-in 2:3 cards stay
  // pixel-identical (the virtual grid forces this height).
  const rowHeight = (width: number) => width * ratio + CARD_FOOTER;
  // The card meta matches the built-in cards: playtime, recency, store, year, update.
  const metaFor = (g: Game) => {
    const hours = hoursPlayed(g);
    return [hours != null && t('library:cardHours', { hours: formatDecimal1(hours) }), recencyWord(t, g), displayStore(t, g.store), g.release_year ? String(g.release_year) : null, g.update_available && t('library:cardUpdate')].filter(Boolean).join(' · ');
  };
  return (
    <section>
      <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-text-muted">{t('library:allTitles')}</h2>
      <WindowedGrid
        items={rest}
        itemKey={(g) => g.id}
        minColumn={CARD_MIN_WIDTH}
        gap={CARD_GAP}
        rowHeight={rowHeight}
        className="cover-grid-sm"
        renderItem={(g) => <GameTile game={g} job={jobs.get(g.id)} meta={metaFor(g)} coverBadges site="library" onOpen={onOpenGame} onManage={onManage} />}
      />
    </section>
  );
}

export function LibraryActivity({ events, onNavigate, limit, mode, filteredEmpty }: { events: ActivityEvent[]; onNavigate: (v: View) => void; limit?: number; mode: LibraryMode; filteredEmpty: boolean }) {
  if (filteredEmpty || mode !== 'cards') return null;
  return <ActivityPanel events={events} limit={limit ?? 6} onFullLog={() => onNavigate({ name: 'activity' })} />;
}

export function LibraryRecent({ recentlyAdded, onOpenGame, mode, filteredEmpty }: { recentlyAdded: Game[]; onOpenGame: (id: string) => void; mode: LibraryMode; filteredEmpty: boolean }) {
  const { t } = useTranslation('library');
  if (filteredEmpty || mode !== 'cards' || recentlyAdded.length === 0) return null;
  return (
    <Panel className="bg-surface">
      <h2 className="mb-3 text-[14px] font-bold">{t('library:recentlyAdded')}</h2>
      <RecentList games={recentlyAdded} onOpenGame={onOpenGame} />
    </Panel>
  );
}

export function LibraryTable({ tableRows, runners, onOpenGame, onManage, sort, onSort, mode, filteredEmpty }: { tableRows: Game[]; runners: ProtonVersion[]; onOpenGame: (id: string) => void; onManage?: (id: string) => void; sort: Sort | null; onSort: (k: SortKey) => void; mode: LibraryMode; filteredEmpty: boolean }) {
  const { t } = useTranslation('library');
  const columns = useVariant('game-row.columns');
  const rowMode = useVariant('game-row.mode');
  const minimal = columns === 'minimal';
  if (filteredEmpty || mode !== 'table') return null;
  const rowHeight = rowMode === 'compact' ? 32 : TABLE_ROW_HEIGHT;
  return (
    <section className="overflow-x-auto rounded-2xl bg-black/25 ring-1 ring-hairline">
      <div role="table" aria-label={t('library:table.label')} aria-rowcount={tableRows.length + 1} className="min-w-[800px]">
        <div role="row" aria-rowindex={1} className={cx('grid gap-3 border-b border-hairline px-4 py-2 font-mono text-micro uppercase tracking-[0.12em] text-text-muted', minimal ? 'grid-cols-[16px_minmax(180px,2fr)_5rem_7rem]' : TABLE_COLS)}>
          <span role="columnheader"><span className="sr-only">{t('library:table.statusSr')}</span></span>
          <SortHead k="title" label={t('library:table.title')} sort={sort} onSort={onSort} />
          <SortHead k="tier" label={t('library:table.tier')} sort={sort} onSort={onSort} />
          {!minimal && <span role="columnheader">{t('library:table.runner')}</span>}
          {!minimal && <SortHead k="hours" label={t('library:table.hours')} sort={sort} onSort={onSort} right />}
          {!minimal && <SortHead k="year" label={t('library:table.year')} sort={sort} onSort={onSort} right />}
          {!minimal && <SortHead k="size" label={t('library:table.size')} sort={sort} onSort={onSort} right />}
          <SortHead k="last" label={t('library:table.lastPlayed')} sort={sort} onSort={onSort} right />
        </div>
        <WindowedRows
          items={tableRows}
          itemKey={(g) => g.id}
          rowHeight={rowHeight}
          renderRow={(g, i) => <GameRow game={g} index={i} runners={runners} recency={recencyWord(t, g)} onOpen={onOpenGame} onManage={onManage} />}
        />
      </div>
    </section>
  );
}
