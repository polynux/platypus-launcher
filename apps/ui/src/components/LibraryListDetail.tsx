// i18n: converted
// The Atlas library as a game list with the game on the stage (issue #126,
// D22.jsx `.body.is-split` list + stage). Only the list-detail presentation
// renders here; other themes keep their own path, so warm, cool and slate
// render as before.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { ArrowUpDown, CircleAlert, LogIn, Plus, Search, SlidersHorizontal, X } from 'lucide-react';
import type { Collection, Game, Job, SessionEvent, SourceWarning, StoreType } from '../types';
import { Input } from './ds/input';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ds/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { cx } from './ui';
import { heroImages, lastPlayedAgo } from '../lib/games';
import { missingNames } from '../lib/collections';
import { filterLibrary } from '../lib/libraryFilter';
import { TierFilterControl, StatusFilterControl, sortGames, defaultDir, type Sort, type StatusFilter } from './LibraryView';
import { AtlasActiveChips, AtlasGameBadges } from './AtlasLibraryViews';
import type { TierFilter } from '../lib/compat';
import { useVariant } from '../lib/theme';

type ListSort = 'recent' | 'title' | 'hours' | 'year' | 'size' | 'tier';

/** Atlas 'recent' is the classic 'last played' order, newest first. */
const toSort = (s: ListSort, dir: 'asc' | 'desc'): Sort =>
  s === 'recent' ? { key: 'last', dir } : { key: s, dir };

interface ListDetailProps {
  games: Game[];
  /** Source-bar state the selection follows: ownership, store selection and collection. */
  ownership: 'installed' | 'owned';
  source: StoreType | 'all';
  storeSelection?: StoreType[];
  collection?: string;
  collections?: Collection[];
  /** App search is not used here: the list has its own filter field (D21). */
  onPlay: (id: string) => void;
  /** The stage for the selected game (the game page). */
  renderStage: (game: Game) => React.ReactNode;
  /** Opening a game from elsewhere selects it in the list. */
  initialSelectedId?: string | null;
  /** The staged game changed (for the game tone). */
  onStageChange?: (id: string | null) => void;
  /** "Show all games" resets the source-bar filters too. */
  onResetFilters?: () => void;
  /** Atlas parity (issue #132): the list header adds a game like the library mode switch does. */
  onAddGame?: () => void;
  /** Source warnings (issue #132): the classic LibraryWarnings content as a compact one-line notice. */
  warnings?: SourceWarning[];
  /** Store names by source id, for the logged-out warning (like the classic list). */
  pluginName?: (id: string) => string;
  /** Opens the Plugins view for a warning's login/plugins action. */
  onOpenPlugins?: () => void;
  /** Right-click (or the context-menu key) on a row opens the game's collections dialog. */
  onManageCollections?: (gameId: string) => void;
  /** The row menu's settings entry opens the game settings for that game. */
  onOpenSettings?: (gameId: string) => void;
  /** The row menu's install entry queues an install for that game. */
  onInstall?: (game: Game) => void;
  /** Queued or running jobs, for the download badges (issue #136). */
  jobs?: Job[];
  /** Live sessions by game id, for the playing dot (R4-13). */
  sessions?: Record<string, SessionEvent>;
  /** Controlled filter state from the /22 library bar (App owns it, #135).
   *  When set, the list filters through these values and hides its own
   *  filter popover (the shell carries it); otherwise it keeps its internal
   *  state, like the classic header. Query (issue #157) is shared the same
   *  way: the grid/table and the list read App's search, so a search typed
   *  in one view is not lost in the other. */
  tierValue?: TierFilter;
  onTierChange?: (v: TierFilter) => void;
  statusValue?: StatusFilter;
  onStatusChange?: (v: StatusFilter) => void;
  lendingValue?: 'all' | 'with-me';
  onLendingChange?: (v: 'all' | 'with-me') => void;
  queryValue?: string;
  onQueryChange?: (q: string) => void;
}

/** Sub-line under the title: the relative "ago" ("6 h ago", "yesterday",
 *  "2 mo ago" like ago() in the design lab), or "Not installed" greyed. */
function subLine(t: (k: string) => string, g: Game): { text: string; absent: boolean } {
  if (!g.is_installed) return { text: t('library:listDetail.notInstalled'), absent: true };
  const last = lastPlayedAgo(g);
  if (last) return { text: last, absent: false };
  return { text: t('library:listDetail.neverPlayed'), absent: false };
}

export function LibraryListDetail(p: ListDetailProps) {
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  const { t } = useTranslation(['library', 'game', 'collections', 'nav']);
  const [queryInner, setQueryInner] = useState('');
  const query = p.queryValue ?? queryInner;
  const setQuery: (q: string) => void = p.onQueryChange ?? setQueryInner;
  const [sort, setSort] = useState<ListSort>('recent');
  const [dir, setDir] = useState<'asc' | 'desc'>('desc');
  // Atlas parity (issue #132): the classic tier/status/lending filters, reused
  // from LibraryView (no duplicated predicates). Under the /22 shell (#135)
  // App owns this state so the library bar and the list share it, and the
  // list hides its own popover (the shell carries it).
  const controlled = p.tierValue !== undefined || p.statusValue !== undefined || p.lendingValue !== undefined || p.queryValue !== undefined;
  const [tierInner, setTierInner] = useState<TierFilter>('all');
  const [statusInner, setStatusInner] = useState<StatusFilter>('all');
  const [lendingInner, setLendingInner] = useState<'all' | 'with-me'>('all');
  const tier = p.tierValue ?? tierInner;
  const setTier: (v: TierFilter) => void = p.onTierChange ?? setTierInner;
  const statusFilter = p.statusValue ?? statusInner;
  const setStatusFilter: (v: StatusFilter) => void = p.onStatusChange ?? setStatusInner;
  const lending = p.lendingValue ?? lendingInner;
  const setLending: (v: 'all' | 'with-me') => void = p.onLendingChange ?? setLendingInner;
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [warnDismissed, setWarnDismissed] = useState(false);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  // The row menu needs a reason to exist: collections, settings or install.
  const hasMenu = !!(p.onManageCollections || p.onOpenSettings || p.onInstall);
  const menuGame = menu ? p.games.find((g) => g.id === menu.id) : undefined;
  const hasBorrowed = p.games.some((g) => g.shared_by != null);
  const lendingFilter = hasBorrowed ? lending : 'all';
  const extraCount = [tier !== 'all', statusFilter !== 'all', lendingFilter !== 'all'].filter(Boolean).length;
  const resetExtra = () => { setTier('all'); setStatusFilter('all'); setLending('all'); };
  // Atlas filter selects in Albert Sans (label.style = plain): human values,
  // never uppercase or mono.
  const atlasSelect = 'h-[30px] w-full text-[13px]';
  const atlasItem = 'text-[13px] capitalize';
  const atlasAny = t('library:listDetail.filterAny');
  // The active collection's missing titles (issue #45's staleTitles): the same
  // names the classic header lists, as a one-line note at the top of the list.
  const activeCollection = (p.collections ?? []).find((c) => c.id === p.collection);
  const missing = activeCollection && !activeCollection.smart ? missingNames(activeCollection, p.games) : [];
  const [selectedId, setSelectedId] = useState<string | null>(p.initialSelectedId ?? null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);

  // Opening a game from elsewhere (palette, dashboard) selects it in the list.
  useEffect(() => {
    if (p.initialSelectedId) setSelectedId(p.initialSelectedId);
  }, [p.initialSelectedId]);
  useEffect(() => { setWarnDismissed(false); }, [p.warnings]);
  const jobsById = useMemo(() => new Map((p.jobs ?? []).map((j) => [j.game_id, j])), [p.jobs]);

  const filtered = useMemo(() => {
    const list = filterLibrary(p.games, {
      ownership: p.ownership,
      source: p.source,
      storeSelection: p.storeSelection,
      collection: p.collection,
      collections: p.collections,
      tier,
      status: statusFilter,
      lending,
      query,
    });
    return sortGames(list, toSort(sort, dir));
  }, [p.games, p.ownership, p.source, p.storeSelection, p.collection, p.collections, query, sort, dir, tier, statusFilter, lending]);

  // The selection is kept across filter changes when possible.
  useEffect(() => {
    if (filtered.length === 0) {
      if (selectedId !== p.initialSelectedId && selectedId !== null) setSelectedId(null);
      return;
    }
    // A game opened from elsewhere stays selected even when the filters hide
    // it (like /22's select): the stage still shows the routed game.
    if (selectedId && selectedId === p.initialSelectedId && p.games.some((g) => g.id === selectedId)) return;
    if (!selectedId || !filtered.some((g) => g.id === selectedId)) setSelectedId(filtered[0].id);
  }, [filtered, selectedId, p.initialSelectedId, p.games]);

  const selected = filtered.find((g) => g.id === selectedId)
    ?? (selectedId && selectedId === p.initialSelectedId ? p.games.find((g) => g.id === selectedId) : undefined)
    ?? null;

  useEffect(() => {
    if (!selectedId) return;
    listRef.current?.querySelector<HTMLElement>(`[data-game-row="${CSS.escape(selectedId)}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [selectedId]);

  useEffect(() => {
    if (!atlas) return;
    const onGlobalKey = (e: KeyboardEvent) => {
      // The shared controller handler (capture) runs first and marks what
      // it moves: never handle the same press twice.
      if (e.defaultPrevented) return;
      const target = e.target as HTMLElement | null;
      if (!target || target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable) return;
      if (target.closest('[role=listbox],[role=menu],[role=dialog],[data-keys-local]') || document.querySelector('[role=dialog]')) return;
      const list = listRef.current;
      if (list && !list.contains(target) && target !== document.body) return;
      if (target.closest('[data-game-row]')) return;
      if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); move(1); }
      else if (e.key === 'k' || e.key === 'ArrowUp') { e.preventDefault(); move(-1); }
      else if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) { e.preventDefault(); searchRef.current?.focus(); }
    };
    window.addEventListener('keydown', onGlobalKey);
    return () => window.removeEventListener('keydown', onGlobalKey);
  }, [atlas, filtered, selectedId, selected, p.onPlay]);

  useEffect(() => {
    p.onStageChange?.(selected?.id ?? null);
  }, [selected?.id]);

  // The stage is its own scroll container (like the list): it scrolls back
  // to the top when the game changes.
  useEffect(() => {
    stageRef.current?.scrollTo({ top: 0 });
  }, [selected?.id]);

  const move = (dir: 1 | -1) => {
    if (filtered.length === 0) return;
    const i = filtered.findIndex((g) => g.id === selectedId);
    const next = filtered[(i < 0 ? (dir === 1 ? -1 : 0) : i + dir + filtered.length) % filtered.length];
    setSelectedId(next.id);
    requestAnimationFrame(() => {
      listRef.current?.querySelector<HTMLElement>(`[data-game-row="${CSS.escape(next.id)}"]`)?.focus({ preventScroll: true });
      listRef.current?.querySelector<HTMLElement>(`[data-game-row="${CSS.escape(next.id)}"]`)?.scrollIntoView({ block: 'nearest' });
    });
  };

  const onListKey = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown' || e.key === 'j') { e.preventDefault(); move(1); }
    else if (e.key === 'ArrowUp' || e.key === 'k') { e.preventDefault(); move(-1); }
    // One launch for the selected row (the stage shows it). The rows have no
    // key handler of their own: a focused row bubbles here, so Enter can't
    // play twice. Keyboard and click selection keep focus on the selected row.
    else if (e.key === 'Enter') { e.preventDefault(); if (selected) p.onPlay(selected.id); }
    // The context-menu key (or Shift+F10) opens the row menu for the focused row.
    else if (e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) {
      e.preventDefault();
      const el = (e.target as HTMLElement)?.closest?.('[data-game-row]');
      const id = el?.getAttribute('data-game-row');
      if (id && hasMenu) {
        setSelectedId(id);
        const r = (el as Element).getBoundingClientRect();
        setMenu({ id, x: r.left, y: r.bottom });
      }
    }
  };

  return (
    // The list and the stage are each their own scroll container under fixed
    // tools: App's main never scrolls in this mode (/21 `.list` + `.stage`).
    <div className="flex h-full min-h-0 flex-1 items-stretch overflow-x-hidden">
      <aside aria-label={t('library:listDetail.listLabel')} className="flex min-h-0 w-[260px] min-w-0 flex-none flex-col border-r border-hairline bg-[color-mix(in_srgb,var(--color-ground)_60%,var(--color-raised))] min-[1100px]:w-[296px]">
        {missing.length > 0 && (
          <p className="mx-2.5 mt-2.5 truncate text-[12px] text-text-muted" title={missing.join(', ')}>
            {t('collections:missing', { count: missing.length })}
          </p>
        )}
        <div className="flex gap-1.5 p-2.5">
          <label className="flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-[7px] bg-surface px-2.5 text-text-faint">
            <Search className="h-3.5 w-3.5 flex-none" aria-hidden />
            <span className="sr-only">{t('library:listDetail.filterAria')}</span>
            <input
              data-search
              ref={searchRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('library:listDetail.filterPlaceholder')}
              className="min-w-0 flex-1 bg-transparent text-[13px] text-text outline-none placeholder:text-text-faint"
              onKeyDown={(e) => {
                if (e.key === 'Escape') { setQuery(''); e.currentTarget.blur(); }
                if (e.key === 'ArrowDown') { e.preventDefault(); e.currentTarget.blur(); move(1); listRef.current?.querySelector<HTMLElement>('[data-game-row]')?.focus({ preventScroll: true }); }
              }}
            />
          </label>
          {/* Sort key and direction as one joined control, so the filter field keeps its width. */}
          <div className="flex flex-none">
          <Select value={sort} onValueChange={(v) => { const next = v as ListSort; setSort(next); setDir(defaultDir(next === 'recent' ? 'last' : next)); }}>
            <SelectTrigger aria-label={t('library:listDetail.sortAria')} className="h-8 w-[104px] flex-none rounded-r-none bg-surface font-sans text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="recent" className="font-sans text-[13px]">{t('library:listDetail.sort.recent')}</SelectItem>
              <SelectItem value="title" className="font-sans text-[13px]">{t('library:listDetail.sort.title')}</SelectItem>
              <SelectItem value="hours" className="font-sans text-[13px]">{t('library:listDetail.sort.hours')}</SelectItem>
              <SelectItem value="year" className="font-sans text-[13px]">{t('library:listDetail.sort.year')}</SelectItem>
              <SelectItem value="size" className="font-sans text-[13px]">{t('library:listDetail.sort.size')}</SelectItem>
              <SelectItem value="tier" className="font-sans text-[13px]">{t('library:listDetail.sort.tier')}</SelectItem>
            </SelectContent>
          </Select>
          <button
            type="button"
            aria-label={dir === 'asc' ? t('library:ascToDesc') : t('library:descToAsc')}
            title={dir === 'asc' ? t('library:ascending') : t('library:descending')}
            aria-pressed={dir === 'desc'}
            onClick={() => setDir((d) => (d === 'asc' ? 'desc' : 'asc'))}
            className={cx(
              'grid h-8 w-6 flex-none place-items-center rounded-r-[7px] border-l border-hairline transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              dir === 'desc' ? 'bg-surface-strong text-text' : 'bg-surface text-text-faint hover:text-text'
            )}
          >
            <ArrowUpDown className="h-3.5 w-3.5" aria-hidden />
          </button>
          </div>
          {/* The /22 shell carries the filters popover (#135); the list keeps
              only its filter field and the sort control here. */}
          {!controlled && (
          <DropdownMenu open={filtersOpen} onOpenChange={setFiltersOpen} modal={false}>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={t('library:listDetail.filtersAria')}
                className={cx(
                  'relative grid h-[30px] w-[30px] flex-none place-items-center rounded-[7px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  filtersOpen || extraCount > 0 ? 'bg-surface-strong text-text' : 'bg-surface text-text-faint hover:text-text-muted'
                )}
              >
                <SlidersHorizontal className="h-3.5 w-3.5" aria-hidden />
                {extraCount > 0 && (
                  <span aria-label={t('nav:needsAttention')} className="absolute right-1 top-1 size-1.5 rounded-full bg-accent-strong" />
                )}
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              side="bottom"
              align="start"
              className="w-[260px] p-3"
              // The selects portal their listbox outside the menu: picking a
              // value must not dismiss the popover (like the Tune flyout).
              onPointerDownOutside={(e) => {
                if ((e.target as Element)?.closest?.('[role="listbox"], [data-radix-popper-content-wrapper]')) e.preventDefault();
              }}
            >
              <div className="grid gap-3">
                <div>
                  <p className="mb-1 text-[13px] text-text-muted">{t('library:listDetail.filterCompat')}</p>
                  <TierFilterControl value={tier} onChange={setTier} triggerClassName={atlasSelect} itemClassName={atlasItem} allLabel={atlasAny} />
                </div>
                <div>
                  <p className="mb-1 text-[13px] text-text-muted">{t('library:listDetail.filterStatus')}</p>
                  <StatusFilterControl value={statusFilter} onChange={setStatusFilter} triggerClassName={atlasSelect} itemClassName={atlasItem} allLabel={atlasAny} />
                </div>
                {hasBorrowed && (
                  <div>
                    <p className="mb-1 text-[13px] text-text-muted">{t('library:listDetail.filterLending')}</p>
                    <Select value={lendingFilter} onValueChange={(v) => setLending(v as 'all' | 'with-me')}>
                      <SelectTrigger aria-label={t('library:family')} className={atlasSelect}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all" className={atlasItem}>{atlasAny}</SelectItem>
                        <SelectItem value="with-me" className={atlasItem}>{t('library:listDetail.filterLentOnly')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
              {extraCount > 0 && (
                <>
                  <DropdownMenuSeparator />
                  <button
                    type="button"
                    onClick={resetExtra}
                    className="w-full rounded-lg px-2 py-1.5 text-left text-[13px] font-semibold text-accent transition hover:bg-surface-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                  >
                    {t('library:listDetail.filterReset')}
                  </button>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
          )}
          {p.onAddGame && (
            <button
              type="button"
              aria-label={t('library:listDetail.addGame')}
              title={t('library:listDetail.addGame')}
              onClick={() => p.onAddGame?.()}
              className="grid h-[30px] w-[30px] flex-none place-items-center rounded-[7px] bg-surface text-text-faint transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
            >
              <Plus className="h-3.5 w-3.5" aria-hidden />
            </button>
          )}
        </div>
        {(p.warnings ?? []).length > 0 && !warnDismissed && (
          <div role="status" className="mx-2.5 mb-2 flex items-center gap-2 rounded-[7px] bg-danger/[0.08] px-2.5 py-2 text-[12px] ring-1 ring-danger/25">
            {(p.warnings ?? []).some((w) => w.needs_auth) ? (
              <>
                <LogIn className="h-3.5 w-3.5 flex-none text-danger" aria-hidden />
                <span className="min-w-0 flex-1 truncate text-text-body">
                  <Trans
                    i18nKey="library:warningLoggedOut"
                    values={{ name: (p.pluginName ?? ((id: string) => id))((p.warnings ?? []).find((w) => w.needs_auth)!.source) }}
                    components={{ name: <span className="font-bold text-danger" /> }}
                  />
                </span>
                <button
                  type="button"
                  onClick={() => p.onOpenPlugins?.()}
                  className="flex-none font-semibold text-accent-strong hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  {t('library:login')}
                </button>
              </>
            ) : (
              <>
                <CircleAlert className="h-3.5 w-3.5 flex-none text-danger" aria-hidden />
                <span className="min-w-0 flex-1 truncate text-text-body">{t('library:listDetail.warnings', { count: (p.warnings ?? []).length })}</span>
                <button
                  type="button"
                  onClick={() => p.onOpenPlugins?.()}
                  className="flex-none font-semibold text-accent-strong hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
                >
                  {t('library:plugins')}
                </button>
              </>
            )}
            <button
              type="button"
              onClick={() => setWarnDismissed(true)}
              aria-label={t('library:atlasViews.warnings.dismiss')}
              className="grid size-5 flex-none place-items-center rounded text-text-faint transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
            >
              <X className="h-3.5 w-3.5" aria-hidden />
            </button>
          </div>
        )}
        {(activeCollection || tier !== 'all' || statusFilter !== 'all' || lendingFilter !== 'all' || query.trim() !== '') && (
          <div className="mx-2.5 mb-2">
            <AtlasActiveChips
              collection={activeCollection ?? undefined}
              onClearCollection={() => p.onResetFilters?.()}
              tier={tier}
              onTier={setTier}
              status={statusFilter}
              onStatus={setStatusFilter}
              lending={lending}
              onLending={setLending}
              query={query}
              onQuery={setQuery}
            />
          </div>
        )}
        {filtered.length === 0 ? (
          <p className="px-4 py-4 text-[13px] text-text-muted">
            {t('library:listDetail.empty')}{' '}
            <button
              type="button"
              onClick={() => { setQuery(''); resetExtra(); p.onResetFilters?.(); }}
              className="text-accent-strong underline decoration-accent-strong/40 underline-offset-2 hover:text-text"
            >
              {t('library:listDetail.showAll')}
            </button>
          </p>
        ) : (
          <ul ref={listRef} onKeyDown={onListKey} data-keys-local className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-3">
            {filtered.map((g) => {
              const sub = subLine(t, g);
              const on = g.id === selected?.id;
              const art = heroImages(g)[0];
              // Plain rows: no trigger wrapper, so click focus and j/k, Enter
              // and Space behave like any button. One controlled menu at the
              // list level opens anchored at the right-clicked row (issue #132).
              // Badges (issue #136) sit at the right, like the screenshots.
              return (
                <li key={g.id}>
                  <button
                    type="button"
                    data-game-row={g.id}
                    aria-current={on ? 'true' : undefined}
                    onClick={() => setSelectedId(g.id)}
                    onDoubleClick={() => p.onPlay(g.id)}
                    onContextMenu={hasMenu ? (e) => { e.preventDefault(); setSelectedId(g.id); setMenu({ id: g.id, x: e.clientX, y: e.clientY }); } : undefined}
                    className={cx(
                      'flex w-full items-center gap-2.5 rounded-lg px-2 py-[5px] pl-[5px] text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                      on ? 'bg-accent/[0.15] text-text' : 'text-text-muted hover:bg-surface hover:text-text',
                    )}
                  >
                    {art ? (
                      <img src={art} alt="" loading="lazy" className={cx('h-7 w-[60px] flex-none rounded-[5px] object-cover', !g.is_installed && 'brightness-[0.55] grayscale')} />
                    ) : (
                      <span aria-hidden className="h-7 w-[60px] flex-none rounded-[5px] bg-surface-strong" />
                    )}
                    <span className="grid min-w-0 flex-1 leading-[1.2]">
                      <span className={cx('truncate text-[13.5px] font-semibold', !g.is_installed && 'text-text-faint')}>{g.title}</span>
                      <span className="truncate text-[12px] text-text-faint">{sub.text}</span>
                    </span>
                    <AtlasGameBadges game={g} job={jobsById.get(g.id)} session={p.sessions?.[g.id]} />
                  </button>
                </li>
              );
            })}
          </ul>
        )}
        {/* The classic tile's right-click opens the game's collections; the
            Atlas row opens one small menu with the same entry, plus Play (or
            Install for a game that isn't installed) and the game settings. */}
        {menuGame && (
          <DropdownMenu
            open
            onOpenChange={(o) => {
              if (!o) {
                setMenu(null);
                document.querySelector<HTMLElement>(`[data-game-row="${CSS.escape(menu!.id)}"]`)?.focus();
              }
            }}
            modal={false}
          >
            <DropdownMenuTrigger asChild>
              <span aria-hidden className="pointer-events-none fixed h-0 w-0" style={{ left: menu!.x, top: menu!.y }} />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="start" onCloseAutoFocus={(e) => e.preventDefault()}>
              {menuGame.is_installed ? (
                <DropdownMenuItem asChild onSelect={() => p.onPlay(menuGame.id)}>
                  <button>
                    <span>{t('game:play.play')}</span>
                  </button>
                </DropdownMenuItem>
              ) : menuGame.installable && p.onInstall ? (
                <DropdownMenuItem asChild onSelect={() => p.onInstall?.(menuGame)}>
                  <button>
                    <span>{t('game:install.install')}</span>
                  </button>
                </DropdownMenuItem>
              ) : null}
              {p.onOpenSettings && (
                <DropdownMenuItem asChild onSelect={() => p.onOpenSettings?.(menuGame.id)}>
                  <button>
                    <span>{t('game:hero.gameSettings')}</span>
                  </button>
                </DropdownMenuItem>
              )}
              {p.onManageCollections && (
                <DropdownMenuItem asChild onSelect={() => p.onManageCollections?.(menuGame.id)}>
                  <button>
                    <span>{t('library:listDetail.rowCollections')}</span>
                  </button>
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </aside>
      {/* The stage is its own scroll container (like the list): it scrolls back
          to the top when the game changes. No stable gutter: it left a
          permanent 10px gap beside the full-bleed hero on short content. */}
      <div ref={stageRef} className="min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden bg-raised">
        {selected ? p.renderStage(selected) : null}
      </div>
    </div>
  );
}
