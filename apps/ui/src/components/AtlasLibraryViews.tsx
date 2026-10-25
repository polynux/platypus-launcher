// Atlas /22 library views (issue #136, D22.jsx `lib-full` + `appLibrary.jsx`).
// The list view lives in LibraryListDetail; the grid and table live here so
// warm, cool and slate keep today's LibraryView path exactly. Only the Atlas
// list-detail presentation renders these; every colour comes from a token.
import React, { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, ArrowDown, ArrowDownToLine, ArrowUp, ArrowUpDown, Cloud, Download, FolderPlus, Play, Search, Settings2, X } from 'lucide-react';
import type { Collection, Game, Job, ProtonVersion, StoreType } from '../types';
import { Cover, cx } from './ui';
import { coverImages, formatBytes, hoursPlayed, lastPlayedAgo, needsAttention, attentionReason, byRecent } from '../lib/games';
import { formatCount } from '../lib/format';
import { tierFilterValue } from '../lib/compat';
import { collectionPredicate } from '../lib/collections';
import { matchesExtraFilters, sortGames, type Sort, type SortKey, type StatusFilter } from './LibraryView';
import type { TierFilter } from '../lib/compat';

export type AtlasSortKey = 'recent' | 'title' | 'hours' | 'year' | 'size' | 'tier';
const toSort = (s: AtlasSortKey, dir: 'asc' | 'desc'): Sort =>
  s === 'recent' ? { key: 'last', dir } : { key: s, dir };

/** Shared filter for the Atlas grid/table (issue #136): the same predicates
 *  the list and the classic library filter through, so the logic can't drift. */
export function atlasFilteredGames(games: Game[], o: {
  ownership: 'installed' | 'owned';
  source: StoreType | 'all';
  storeSelection?: StoreType[];
  collection?: string;
  collections?: Collection[];
  tier: TierFilter;
  status: StatusFilter;
  lending: 'all' | 'with-me';
  query: string;
  sort: AtlasSortKey;
  dir: 'asc' | 'desc';
}): Game[] {
  const hasBorrowed = games.some((g) => g.shared_by != null);
  const lendingFilter = hasBorrowed ? o.lending : 'all';
  const inCollection = collectionPredicate(o.collections ?? [], o.collection ?? '', Math.floor(Date.now() / 1000));
  const q = o.query.trim().toLowerCase();
  const multi = o.storeSelection;
  const list = games.filter(
    (g) =>
      (o.ownership === 'owned' || g.is_installed) &&
      (multi !== undefined ? multi.length === 0 || multi.includes(g.store) : o.source === 'all' || g.store === o.source) &&
      (!inCollection || inCollection(g)) &&
      matchesExtraFilters(g, o.tier, o.status, lendingFilter) &&
      (q === '' || g.title.toLowerCase().includes(q)),
  );
  return sortGames(list, toSort(o.sort, o.dir));
}

/** Attention / update / download badges at the right of a row or card, as in
 *  the screenshots (issue #136 §1-2). Shared by the list, grid and table. */
export function AtlasGameBadges({ game, job }: { game: Game; job?: Job }) {
  const { t } = useTranslation('library');
  const active = job && (job.state === 'queued' || job.state === 'running');
  const badges: React.ReactNode[] = [];
  if (game.update_available) {
    badges.push(
      <span key="update" title={t('library:atlasViews.badges.update')} aria-label={t('library:atlasViews.badges.update')} className="grid size-6 flex-none place-items-center rounded-md bg-accent-dim text-accent-strong ring-1 ring-accent-strong/30">
        <ArrowDownToLine className="h-3.5 w-3.5" aria-hidden />
      </span>,
    );
  }
  if (active) {
    const pct = job.percent >= 0 ? Math.round(job.percent) : 0;
    badges.push(
      <span key="dl" title={t('library:atlasViews.badges.downloading', { percent: pct })} aria-label={t('library:atlasViews.badges.downloading', { percent: pct })} className="grid size-6 flex-none place-items-center rounded-md bg-surface-strong text-text ring-1 ring-hairline">
        <Download className="h-3.5 w-3.5" aria-hidden />
      </span>,
    );
  }
  if (game.shared_by) {
    badges.push(
      <span key="lent" title={t('library:sharedBy', { name: game.shared_by.name })} aria-label={t('library:sharedBy', { name: game.shared_by.name })} className="grid size-6 flex-none place-items-center rounded-md bg-surface-strong text-text-muted ring-1 ring-hairline">
        <Cloud className="h-3.5 w-3.5" aria-hidden />
      </span>,
    );
  }
  if (needsAttention(game)) {
    badges.push(
      <span key="attn" title={attentionReason(game)} aria-label={t('library:listDetail.needsAttention')} className="grid size-6 flex-none place-items-center rounded-md bg-attention/15 text-attention ring-1 ring-attention/30">
        <AlertTriangle className="h-3.5 w-3.5" aria-hidden />
      </span>,
    );
  }
  if (badges.length === 0) return null;
  return <span className="flex flex-none items-center gap-1">{badges}</span>;
}

export interface AtlasChipsProps {
  collection?: Collection;
  onClearCollection?: () => void;
  tier: TierFilter;
  onTier: (v: TierFilter) => void;
  status: StatusFilter;
  onStatus: (v: StatusFilter) => void;
  lending: 'all' | 'with-me';
  onLending: (v: 'all' | 'with-me') => void;
  query: string;
  onQuery: (q: string) => void;
}

/** Active collection/filters as removable chips (D22 `ActiveChips`). */
export function AtlasActiveChips(p: AtlasChipsProps) {
  const { t } = useTranslation(['library', 'collections']);
  const chips: { key: string; label: string; off: () => void }[] = [];
  if (p.collection) {
    const label = p.collection.builtin ? t(`collections:builtin.${p.collection.id}`) : p.collection.name;
    if (p.onClearCollection) {
      const off = p.onClearCollection;
      chips.push({ key: 'collection', label, off });
    }
  }
  if (p.tier !== 'all') chips.push({ key: 'tier', label: p.tier, off: () => p.onTier('all') });
  if (p.status !== 'all') chips.push({ key: 'status', label: p.status, off: () => p.onStatus('all') });
  if (p.lending !== 'all') chips.push({ key: 'lending', label: t('library:listDetail.filterLentOnly'), off: () => p.onLending('all') });
  if (p.query.trim() !== '') {
    const q = p.query;
    chips.push({ key: 'query', label: `“${q.trim()}”`, off: () => p.onQuery('') });
  }
  if (chips.length === 0) return null;
  const reset = () => {
    p.onTier('all');
    p.onStatus('all');
    p.onLending('all');
    p.onQuery('');
    p.onClearCollection?.();
  };
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {chips.map((c) => (
        <button
          key={c.key}
          type="button"
          onClick={c.off}
          aria-label={t('library:atlasViews.chips.remove', { label: c.label })}
          className="inline-flex h-[26px] items-center gap-1.5 rounded-md bg-surface-strong px-2 text-[12px] font-semibold text-text transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <span className="max-w-44 truncate">{c.label}</span>
          <X className="h-3 w-3 flex-none" aria-hidden />
        </button>
      ))}
      <button
        type="button"
        onClick={reset}
        className="h-[26px] rounded-md px-2 text-[12px] font-semibold text-text-faint transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {t('library:atlasViews.chips.reset')}
      </button>
    </div>
  );
}

/** Compact source warnings for the Atlas library (D22 `SourceWarnings`
 *  compact): one card with a Fix action and a dismiss ×. */
export function AtlasWarnings({ warnings, pluginName, onOpenPlugins }: {
  warnings: { source: string; needs_auth?: boolean; message?: string }[];
  pluginName: (id: string) => string;
  onOpenPlugins: () => void;
}) {
  const { t } = useTranslation('library');
  const [dismissed, setDismissed] = useState(false);
  if (dismissed || warnings.length === 0) return null;
  const loggedOut = warnings.find((w) => w.needs_auth);
  return (
    <div role="status" className="flex items-center gap-2.5 rounded-lg bg-danger/[0.08] px-3 py-2 text-[12.5px] ring-1 ring-danger/25">
      <AlertTriangle className="h-4 w-4 flex-none text-danger" aria-hidden />
      <p className="min-w-0 flex-1 truncate text-text-body">
        {loggedOut
          ? t('library:warningLoggedOut', { name: pluginName(loggedOut.source) })
          : t('library:listDetail.warnings', { count: warnings.length })}
      </p>
      <button
        type="button"
        onClick={onOpenPlugins}
        className="flex-none font-semibold text-accent-strong hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {t('library:atlasViews.warnings.fix')}
      </button>
      <button
        type="button"
        onClick={() => setDismissed(true)}
        aria-label={t('library:atlasViews.warnings.dismiss')}
        className="grid size-6 flex-none place-items-center rounded-md text-text-faint transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <X className="h-3.5 w-3.5" aria-hidden />
      </button>
    </div>
  );
}

/** Sort key + direction as one joined control (D22 `SortMenu`). */
export function AtlasSortMenu({ sort, dir, onSort, onDir }: {
  sort: AtlasSortKey;
  dir: 'asc' | 'desc';
  onSort: (s: AtlasSortKey) => void;
  onDir: (d: 'asc' | 'desc') => void;
}) {
  const { t } = useTranslation('library');
  return (
    <span className="flex flex-none items-center">
      <label className="sr-only" htmlFor="atlas-sort">{t('library:listDetail.sortAria')}</label>
      <select
        id="atlas-sort"
        value={sort}
        onChange={(e) => onSort(e.target.value as AtlasSortKey)}
        className="h-[32px] flex-none rounded-l-lg border-r border-hairline bg-surface px-2 font-sans text-[13px] text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {(['recent', 'title', 'hours', 'year', 'size', 'tier'] as const).map((k) => (
          <option key={k} value={k}>{t(`library:listDetail.sort.${k}`)}</option>
        ))}
      </select>
      <button
        type="button"
        aria-label={dir === 'asc' ? t('library:ascToDesc') : t('library:descToAsc')}
        title={dir === 'asc' ? t('library:ascending') : t('library:descending')}
        onClick={() => onDir(dir === 'asc' ? 'desc' : 'asc')}
        className="grid h-[32px] w-7 flex-none place-items-center rounded-r-lg bg-surface text-text-faint transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        {dir === 'asc' ? <ArrowUp className="h-3.5 w-3.5" aria-hidden /> : <ArrowDown className="h-3.5 w-3.5" aria-hidden />}
      </button>
    </span>
  );
}

/** The library filter field + sort for the grid/table `lib-bar` (D22). */
export function AtlasLibBar({ query, onQuery, sort, dir, onSort, onDir }: {
  query: string;
  onQuery: (q: string) => void;
  sort: AtlasSortKey;
  dir: 'asc' | 'desc';
  onSort: (s: AtlasSortKey) => void;
  onDir: (d: 'asc' | 'desc') => void;
}) {
  const { t } = useTranslation('library');
  return (
    <div className="flex items-center gap-2">
      <label className="flex h-[32px] min-w-0 max-w-[360px] flex-1 items-center gap-2 rounded-lg bg-surface px-2.5 text-text-faint">
        <Search className="h-3.5 w-3.5 flex-none" aria-hidden />
        <span className="sr-only">{t('library:searchAria')}</span>
        <input
          value={query}
          onChange={(e) => onQuery(e.target.value)}
          placeholder={t('library:atlasViews.filterPlaceholder')}
          className="min-w-0 flex-1 bg-transparent text-[13px] text-text outline-none placeholder:text-text-faint"
        />
      </label>
      <AtlasSortMenu sort={sort} dir={dir} onSort={onSort} onDir={onDir} />
    </div>
  );
}

/** Continue playing / Recently added above the grid when no filter is active. */
export function AtlasLibraryBlocks({ games, onOpen }: { games: Game[]; onOpen: (id: string) => void }) {
  const { t } = useTranslation('library');
  const continued = useMemo(
    () => games.filter((g) => g.is_installed && g.last_played_at).sort(byRecent).slice(0, 4),
    [games],
  );
  const added = useMemo(() => games.filter((g) => g.store === 'custom').slice(0, 6), [games]);
  if (continued.length === 0 && added.length === 0) return null;
  return (
    <div className="grid gap-5 md:grid-cols-2">
      {continued.length > 0 && (
        <section>
          <h2 className="mb-2.5 text-[13px] font-bold uppercase tracking-[0.12em] text-text-muted">{t('library:atlasViews.blocks.continue')}</h2>
          <ul className="grid gap-1.5">
            {continued.map((g) => (
              <li key={g.id}>
                <button
                  type="button"
                  onClick={() => onOpen(g.id)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  <Cover src={coverImages(g)} title="" className="h-8 w-14 flex-none rounded-md" />
                  <span className="min-w-0 flex-1 truncate text-[13px] font-semibold text-text">{g.title}</span>
                  <span className="flex-none text-[12px] tabular-nums text-text-faint">{lastPlayedAgo(g)}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
      {added.length > 0 && (
        <section>
          <h2 className="mb-2.5 text-[13px] font-bold uppercase tracking-[0.12em] text-text-muted">{t('library:atlasViews.blocks.recent')}</h2>
          <ul className="grid gap-1.5">
            {added.map((g) => (
              <li key={g.id}>
                <button
                  type="button"
                  onClick={() => onOpen(g.id)}
                  className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  <Cover src={coverImages(g)} title="" className="h-8 w-14 flex-none rounded-md" />
                  <span className="min-w-0 flex-1 truncate text-[13px] font-semibold text-text">{g.title}</span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function atlasSubLine(t: (k: string) => string, g: Game): string {
  if (!g.is_installed) return t('library:listDetail.notInstalled');
  const h = hoursPlayed(g);
  const ago = lastPlayedAgo(g);
  if (h != null && ago) return `${Math.round(h)} h · ${ago}`;
  if (ago) return ago;
  return t('library:listDetail.neverPlayed');
}

/** Quick actions on hover (D22 `QuickActions`): play/install, settings, collections. */
export function AtlasQuickActions({ game, onPlay, onInstall, onOpenSettings, onManageCollections }: {
  game: Game;
  onPlay?: (id: string) => void;
  onInstall?: (game: Game) => void;
  onOpenSettings?: (id: string) => void;
  onManageCollections?: (id: string) => void;
}) {
  const { t } = useTranslation(['library', 'game']);
  const hasActions = !!(onPlay || onInstall || onOpenSettings || onManageCollections);
  if (!hasActions) return null;
  return (
    <span className="absolute right-2 top-2 flex items-center gap-1 opacity-0 transition focus-within:opacity-100 group-hover:opacity-100">
      {game.is_installed ? (
        onPlay && (
          <button
            type="button"
            aria-label={t('game:play.play')}
            title={t('game:play.play')}
            onClick={(e) => { e.stopPropagation(); onPlay(game.id); }}
            className="grid size-8 place-items-center rounded-lg bg-raised/90 text-text ring-1 ring-hairline transition hover:text-accent-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <Play className="h-3.5 w-3.5 fill-current" aria-hidden />
          </button>
        )
      ) : (
        game.installable && onInstall && (
          <button
            type="button"
            aria-label={t('game:install.install')}
            title={t('game:install.install')}
            onClick={(e) => { e.stopPropagation(); onInstall(game); }}
            className="grid size-8 place-items-center rounded-lg bg-raised/90 text-text ring-1 ring-hairline transition hover:text-accent-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <Download className="h-3.5 w-3.5" aria-hidden />
          </button>
        )
      )}
      {onOpenSettings && (
        <button
          type="button"
          aria-label={t('game:hero.gameSettings')}
          title={t('game:hero.gameSettings')}
          onClick={(e) => { e.stopPropagation(); onOpenSettings(game.id); }}
          className="grid size-8 place-items-center rounded-lg bg-raised/90 text-text-faint ring-1 ring-hairline transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <Settings2 className="h-3.5 w-3.5" aria-hidden />
        </button>
      )}
      {onManageCollections && (
        <button
          type="button"
          aria-label={t('library:listDetail.rowCollections')}
          title={t('library:listDetail.rowCollections')}
          onClick={(e) => { e.stopPropagation(); onManageCollections(game.id); }}
          className="grid size-8 place-items-center rounded-lg bg-raised/90 text-text-faint ring-1 ring-hairline transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <FolderPlus className="h-3.5 w-3.5" aria-hidden />
        </button>
      )}
    </span>
  );
}

/** Atlas grid of covers (D22 `LibraryGrid`) with real data. */
export function AtlasLibraryGrid({ games, jobs, onOpen, onPlay, onInstall, onOpenSettings, onManageCollections }: {
  games: Game[];
  jobs: Map<string, Job>;
  onOpen: (id: string) => void;
  onPlay?: (id: string) => void;
  onInstall?: (game: Game) => void;
  onOpenSettings?: (id: string) => void;
  onManageCollections?: (id: string) => void;
}) {
  const { t } = useTranslation('library');
  return (
    <ul aria-label={t('library:table.label')} className="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,180px),1fr))]">
      {games.map((g) => (
        <li key={g.id} className={cx(!g.is_installed && 'opacity-90')}>
          <div className="group relative">
            <button
              type="button"
              data-game-row={g.id}
              onClick={() => onOpen(g.id)}
              aria-label={g.title}
              className="block w-full overflow-hidden rounded-xl text-left ring-1 ring-hairline transition hover:ring-accent-strong/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
            >
              <Cover src={coverImages(g)} title={g.title} className={cx('aspect-[3/4] w-full', !g.is_installed && 'brightness-[0.55] grayscale')} />
            </button>
            <AtlasQuickActions game={g} onPlay={onPlay} onInstall={onInstall} onOpenSettings={onOpenSettings} onManageCollections={onManageCollections} />
          </div>
          <div className="mt-1.5 flex items-start gap-1.5 px-0.5">
            <div className="min-w-0 flex-1">
              <p className={cx('truncate text-[13px] font-semibold', g.is_installed ? 'text-text' : 'text-text-faint')}>{g.title}</p>
              <p className="truncate text-[12px] tabular-nums text-text-faint">{atlasSubLine(t, g)}</p>
            </div>
            <AtlasGameBadges game={g} job={jobs.get(g.id)} />
          </div>
        </li>
      ))}
    </ul>
  );
}

type AtlasTableSort = { key: SortKey; dir: 'asc' | 'desc' } | null;

function AtlasTableHead({ label, sortKey, sort, onSort, right }: {
  label: string;
  sortKey: SortKey | null;
  sort: AtlasTableSort;
  onSort: (k: SortKey) => void;
  right?: boolean;
}) {
  const active = sortKey != null && sort?.key === sortKey;
  if (!sortKey) return <th className={right ? 'text-right font-medium' : 'font-medium'}>{label}</th>;
  return (
    <th aria-sort={active ? (sort!.dir === 'asc' ? 'ascending' : 'descending') : 'none'} className={right ? 'text-right' : undefined}>
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className={cx('inline-flex items-center gap-1 transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus', active ? 'text-text' : 'text-text-muted')}
      >
        {label}
        {active ? (sort!.dir === 'asc' ? <ArrowUp className="h-3 w-3" aria-hidden /> : <ArrowDown className="h-3 w-3" aria-hidden />) : <ArrowUpDown className="h-3 w-3 opacity-0 transition group-hover:opacity-100" aria-hidden />}
      </button>
    </th>
  );
}

/** Atlas table with sortable columns (D22 `LibraryTable`) on real data. */
export function AtlasLibraryTable({ games, runners, sort, onSort, jobs, onOpen, onPlay, onInstall, onOpenSettings, onManageCollections }: {
  games: Game[];
  runners: ProtonVersion[];
  sort: AtlasTableSort;
  onSort: (k: SortKey) => void;
  jobs: Map<string, Job>;
  onOpen: (id: string) => void;
  onPlay?: (id: string) => void;
  onInstall?: (game: Game) => void;
  onOpenSettings?: (id: string) => void;
  onManageCollections?: (id: string) => void;
}) {
  const { t } = useTranslation(['library', 'game']);
  void runners;
  const rows = useMemo(() => sortGames(games, sort), [games, sort]);
  const statusLabel = (g: Game): string => {
    switch (g.status) {
      case 'backlog': return t('library:statusFilter.backlog');
      case 'playing': return t('library:statusFilter.playing');
      case 'finished': return t('library:statusFilter.finished');
      case 'abandoned': return t('library:statusFilter.abandoned');
      default: return t('library:statusFilter.none');
    }
  };
  return (
    <div className="overflow-x-auto rounded-xl ring-1 ring-hairline">
      <table className="w-full min-w-[820px] border-collapse bg-raised text-[13px]">
        <thead>
          <tr className="group border-b border-hairline text-left font-mono text-[11px] uppercase tracking-[0.12em] text-text-muted">
            <AtlasTableHead label={t('library:table.title')} sortKey="title" sort={sort} onSort={onSort} />
            <AtlasTableHead label={t('library:table.statusSr')} sortKey={null} sort={sort} onSort={onSort} />
            <AtlasTableHead label={t('library:table.hours')} sortKey="hours" sort={sort} onSort={onSort} right />
            <AtlasTableHead label={t('library:table.lastPlayed')} sortKey="last" sort={sort} onSort={onSort} right />
            <AtlasTableHead label={t('library:table.size')} sortKey="size" sort={sort} onSort={onSort} right />
            <AtlasTableHead label={t('library:table.tier')} sortKey="tier" sort={sort} onSort={onSort} />
            <AtlasTableHead label={t('library:table.year')} sortKey="year" sort={sort} onSort={onSort} right />
            <th aria-label={t('library:table.statusSr')} />
          </tr>
        </thead>
        <tbody>
          {rows.map((g) => {
            const h = hoursPlayed(g);
            const tier = tierFilterValue(g);
            return (
              <tr key={g.id} className={cx('border-b border-hairline/60 last:border-0 transition hover:bg-surface/60', !g.is_installed && 'text-text-faint')}>
                <td className="max-w-[280px] px-3 py-2">
                  <span className="flex items-center gap-2.5">
                    <Cover src={coverImages(g)} title="" className="h-7 w-[52px] flex-none rounded" />
                    <button
                      type="button"
                      data-game-row={g.id}
                      onClick={() => onOpen(g.id)}
                      className="min-w-0 flex-1 truncate text-left font-semibold text-text hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                    >
                      {g.title}
                    </button>
                  </span>
                </td>
                <td className="whitespace-nowrap px-2 py-2 text-text-muted">{statusLabel(g)}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right tabular-nums text-text-muted">{h != null ? `${formatCount(Math.round(h * 10) / 10)} h` : '—'}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right tabular-nums text-text-muted">{lastPlayedAgo(g) ?? '—'}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right tabular-nums text-text-muted">{g.is_installed ? (g.size_bytes != null ? formatBytes(g.size_bytes) : '—') : t('library:table.notInstalled')}</td>
                <td className="whitespace-nowrap px-2 py-2 text-text-muted">{tier ?? '—'}</td>
                <td className="whitespace-nowrap px-2 py-2 text-right tabular-nums text-text-muted">{g.release_year ?? '—'}</td>
                <td className="px-2 py-2">
                  <span className="flex items-center justify-end gap-1">
                    <AtlasGameBadges game={g} job={jobs.get(g.id)} />
                    <span className="relative hidden items-center gap-0.5 group-hover:inline-flex">
                      {g.is_installed ? (
                        onPlay && (
                          <button type="button" aria-label={`${t('game:play.play')}: ${g.title}`} onClick={() => onPlay(g.id)} className="grid size-7 place-items-center rounded-md text-text-faint hover:text-text">
                            <Play className="h-3.5 w-3.5 fill-current" aria-hidden />
                          </button>
                        )
                      ) : (
                        g.installable && onInstall && (
                          <button type="button" aria-label={`${t('game:install.install')}: ${g.title}`} onClick={() => onInstall(g)} className="grid size-7 place-items-center rounded-md text-text-faint hover:text-text">
                            <Download className="h-3.5 w-3.5" aria-hidden />
                          </button>
                        )
                      )}
                      {onOpenSettings && (
                        <button type="button" aria-label={t('game:hero.gameSettings')} onClick={() => onOpenSettings(g.id)} className="grid size-7 place-items-center rounded-md text-text-faint hover:text-text">
                          <Settings2 className="h-3.5 w-3.5" aria-hidden />
                        </button>
                      )}
                      {onManageCollections && (
                        <button type="button" aria-label={t('library:listDetail.rowCollections')} onClick={() => onManageCollections(g.id)} className="grid size-7 place-items-center rounded-md text-text-faint hover:text-text">
                          <FolderPlus className="h-3.5 w-3.5" aria-hidden />
                        </button>
                      )}
                    </span>
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/** Loading skeletons per Atlas view (D22 `Skeleton` rows/grid). */
export function AtlasSkeleton({ view }: { view: 'list' | 'grid' | 'table' }) {
  if (view === 'grid') {
    return (
      <ul aria-busy="true" aria-label="Loading" className="grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(min(100%,180px),1fr))]">
        {Array.from({ length: 8 }, (_, i) => (
          <li key={i} className="animate-pulse rounded-xl bg-surface ring-1 ring-hairline motion-reduce:animate-none">
            <span className="block aspect-[3/4] rounded-xl bg-surface-strong" />
            <span className="mx-2 mb-1 mt-2 block h-3.5 rounded bg-surface-strong" />
            <span className="mx-2 mb-2 block h-3 w-2/3 rounded bg-surface-strong" />
          </li>
        ))}
      </ul>
    );
  }
  if (view === 'table') {
    return (
      <div aria-busy="true" aria-label="Loading" className="overflow-hidden rounded-xl ring-1 ring-hairline">
        {Array.from({ length: 8 }, (_, i) => (
          <div key={i} className="flex animate-pulse items-center gap-3 border-b border-hairline/60 px-3 py-2.5 last:border-0 motion-reduce:animate-none">
            <span className="h-7 w-[52px] flex-none rounded bg-surface-strong" />
            <span className="h-3.5 w-40 rounded bg-surface-strong" />
            <span className="ml-auto h-3 w-16 rounded bg-surface-strong" />
          </div>
        ))}
      </div>
    );
  }
  return (
    <ul aria-busy="true" aria-label="Loading" className="grid gap-1.5 px-1.5">
      {Array.from({ length: 9 }, (_, i) => (
        <li key={i} className="flex animate-pulse items-center gap-2.5 rounded-lg px-2 py-[5px] pl-[5px] motion-reduce:animate-none">
          <span className="h-7 w-[60px] flex-none rounded-[5px] bg-surface-strong" />
          <span className="grid flex-1 gap-1.5">
            <span className="h-3.5 w-2/3 rounded bg-surface-strong" />
            <span className="h-3 w-1/3 rounded bg-surface-strong" />
          </span>
        </li>
      ))}
    </ul>
  );
}

export type AtlasPageView = 'grid' | 'table';

/** The Atlas grid/table page (D22 `lib-full`): lib-bar, warnings, chips,
 *  blocks when no filter is active, then the grid or table on real data. */
export function AtlasLibraryPage(p: {
  games: Game[];
  ownership: 'installed' | 'owned';
  source: StoreType | 'all';
  storeSelection?: StoreType[];
  collection?: string;
  collections?: Collection[];
  tier: TierFilter;
  onTier: (v: TierFilter) => void;
  status: StatusFilter;
  onStatus: (v: StatusFilter) => void;
  lending: 'all' | 'with-me';
  onLending: (v: 'all' | 'with-me') => void;
  query: string;
  onQuery: (q: string) => void;
  warnings: { source: string; needs_auth?: boolean; message?: string }[];
  pluginName: (id: string) => string;
  onOpenPlugins: () => void;
  jobs: Job[];
  runners: ProtonVersion[];
  view: AtlasPageView;
  onOpen: (id: string) => void;
  onPlay?: (id: string) => void;
  onInstall?: (game: Game) => void;
  onOpenSettings?: (id: string) => void;
  onManageCollections?: (gameId: string) => void;
  onResetFilters?: () => void;
}) {
  const { t } = useTranslation(['library', 'game']);
  const [sort, setSort] = useState<AtlasSortKey>('recent');
  const [dir, setDir] = useState<'asc' | 'desc'>('desc');
  const [tableSort, setTableSort] = useState<AtlasTableSort>(null);
  const toggleTableSort = (key: SortKey) =>
    setTableSort((s) => {
      if (s?.key !== key) return { key, dir: key === 'title' ? 'asc' : 'desc' };
      return { key, dir: s.dir === 'asc' ? 'desc' : 'asc' };
    });
  const filtered = useMemo(
    () => atlasFilteredGames(p.games, {
      ownership: p.ownership,
      source: p.source,
      storeSelection: p.storeSelection,
      collection: p.collection,
      collections: p.collections,
      tier: p.tier,
      status: p.status,
      lending: p.lending,
      query: p.query,
      sort,
      dir,
    }),
    [p.games, p.ownership, p.source, p.storeSelection, p.collection, p.collections, p.tier, p.status, p.lending, p.query, sort, dir],
  );
  const jobsById = useMemo(() => new Map(p.jobs.map((j) => [j.game_id, j])), [p.jobs]);
  const activeCollection = (p.collections ?? []).find((c) => c.id === p.collection);
  const hasFilter =
    p.query.trim() !== '' ||
    p.tier !== 'all' ||
    p.status !== 'all' ||
    p.lending !== 'all' ||
    p.collection != null ||
    (p.storeSelection !== undefined && p.storeSelection.length > 0);
  const showBlocks = !hasFilter && p.view === 'grid';
  return (
    <div className="mx-auto flex w-full max-w-[1200px] flex-col gap-4 p-5">
      <AtlasLibBar query={p.query} onQuery={p.onQuery} sort={sort} dir={dir} onSort={setSort} onDir={setDir} />
      {p.warnings.length > 0 && (
        <AtlasWarnings warnings={p.warnings} pluginName={p.pluginName} onOpenPlugins={p.onOpenPlugins} />
      )}
      {(activeCollection || p.tier !== 'all' || p.status !== 'all' || p.lending !== 'all' || p.query.trim() !== '') && (
        <AtlasActiveChips
          collection={activeCollection ?? undefined}
          onClearCollection={p.onResetFilters}
          tier={p.tier}
          onTier={p.onTier}
          status={p.status}
          onStatus={p.onStatus}
          lending={p.lending}
          onLending={p.onLending}
          query={p.query}
          onQuery={p.onQuery}
        />
      )}
      {filtered.length === 0 ? (
        <p className="rounded-xl bg-surface px-4 py-6 text-center text-[13px] text-text-muted">
          {t('library:listDetail.empty')}{' '}
          <button
            type="button"
            onClick={() => { p.onQuery(''); p.onResetFilters?.(); }}
            className="text-accent-strong underline decoration-accent-strong/40 underline-offset-2 hover:text-text"
          >
            {t('library:listDetail.showAll')}
          </button>
        </p>
      ) : (
        <>
          {showBlocks && <AtlasLibraryBlocks games={p.games} onOpen={p.onOpen} />}
          {p.view === 'grid' ? (
            <AtlasLibraryGrid
              games={filtered}
              jobs={jobsById}
              onOpen={p.onOpen}
              onPlay={p.onPlay}
              onInstall={p.onInstall}
              onOpenSettings={p.onOpenSettings}
              onManageCollections={p.onManageCollections}
            />
          ) : (
            <AtlasLibraryTable
              games={filtered}
              runners={p.runners}
              sort={tableSort}
              onSort={toggleTableSort}
              jobs={jobsById}
              onOpen={p.onOpen}
              onPlay={p.onPlay}
              onInstall={p.onInstall}
              onOpenSettings={p.onOpenSettings}
              onManageCollections={p.onManageCollections}
            />
          )}
        </>
      )}
    </div>
  );
}
