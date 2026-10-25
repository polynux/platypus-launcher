// i18n: converted
// The Atlas library as a game list with the game on the stage (issue #126,
// D21.jsx `.list` + `.stage`). Only the list-detail presentation renders
// here; the grid keeps today's LibraryView path exactly, so warm, cool and
// slate render as before.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CircleAlert, Plus, Search, SlidersHorizontal } from 'lucide-react';
import type { Collection, Game, SourceWarning, StoreType } from '../types';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { cx } from './ui';
import { byRecent, heroImages, hoursPlayed, lastPlayedAgo, needsAttention, attentionReason } from '../lib/games';
import { sortKey } from '../lib/customize';
import { collectionPredicate } from '../lib/collections';
import { TierFilterControl, StatusFilterControl, matchesExtraFilters, type StatusFilter } from './LibraryView';
import type { TierFilter } from '../lib/compat';

type ListSort = 'recent' | 'title' | 'hours';

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
  /** Opens the Plugins view for a warning's login/plugins action. */
  onOpenPlugins?: () => void;
}

function sortList(games: Game[], sort: ListSort): Game[] {
  const list = [...games];
  if (sort === 'title') list.sort((a, b) => sortKey(a).localeCompare(sortKey(b)));
  else if (sort === 'hours') list.sort((a, b) => (hoursPlayed(b) ?? -1) - (hoursPlayed(a) ?? -1) || a.title.localeCompare(b.title));
  else list.sort(byRecent);
  return list;
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
  const { t } = useTranslation('library');
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<ListSort>('recent');
  // Atlas parity (issue #132): the classic tier/status/lending filters, reused
  // from LibraryView (no duplicated predicates).
  const [tier, setTier] = useState<TierFilter>('all');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [lending, setLending] = useState<'all' | 'with-me'>('all');
  const [filtersOpen, setFiltersOpen] = useState(false);
  const hasBorrowed = p.games.some((g) => g.shared_by != null);
  const lendingFilter = hasBorrowed ? lending : 'all';
  const extraCount = [tier !== 'all', statusFilter !== 'all', lendingFilter !== 'all'].filter(Boolean).length;
  const resetExtra = () => { setTier('all'); setStatusFilter('all'); setLending('all'); };
  const [selectedId, setSelectedId] = useState<string | null>(p.initialSelectedId ?? null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const stageRef = useRef<HTMLDivElement>(null);

  // Opening a game from elsewhere (palette, dashboard) selects it in the list.
  useEffect(() => {
    if (p.initialSelectedId) setSelectedId(p.initialSelectedId);
  }, [p.initialSelectedId]);

  const inCollection = useMemo(
    () => collectionPredicate(p.collections ?? [], p.collection ?? '', Math.floor(Date.now() / 1000)),
    [p.collections, p.collection],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const multi = p.storeSelection;
    const list = p.games.filter(
      (g) =>
        (p.ownership === 'owned' || g.is_installed) &&
        (multi !== undefined ? multi.length === 0 || multi.includes(g.store) : p.source === 'all' || g.store === p.source) &&
        (!inCollection || inCollection(g)) &&
        matchesExtraFilters(g, tier, statusFilter, lendingFilter) &&
        (q === '' || g.title.toLowerCase().includes(q)),
    );
    return sortList(list, sort);
  }, [p.games, p.ownership, p.source, p.storeSelection, inCollection, query, sort, tier, statusFilter, lendingFilter]);

  // The selection is kept across filter changes when possible.
  useEffect(() => {
    if (filtered.length === 0) {
      if (selectedId !== null) setSelectedId(null);
      return;
    }
    if (!selectedId || !filtered.some((g) => g.id === selectedId)) setSelectedId(filtered[0].id);
  }, [filtered, selectedId]);

  const selected = filtered.find((g) => g.id === selectedId) ?? null;

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
  };

  return (
    // The list and the stage are each their own scroll container under fixed
    // tools: App's main never scrolls in this mode (/21 `.list` + `.stage`).
    <div className="flex h-full min-h-0 flex-1 items-stretch overflow-x-hidden">
      <aside aria-label={t('library:listDetail.listLabel')} className="flex min-h-0 w-[260px] min-w-0 flex-none flex-col border-r border-hairline bg-[color-mix(in_srgb,var(--color-ground)_60%,var(--color-raised))] min-[1100px]:w-[296px]">
        <div className="flex gap-1.5 p-2.5">
          <label className="flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-[7px] bg-surface px-2.5 text-text-faint">
            <Search className="h-3.5 w-3.5 flex-none" aria-hidden />
            <span className="sr-only">{t('library:listDetail.filterAria')}</span>
            <input
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
          <Select value={sort} onValueChange={(v) => setSort(v as ListSort)}>
            <SelectTrigger aria-label={t('library:listDetail.sortAria')} className="h-[30px] w-[120px] flex-none bg-surface font-sans text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="recent" className="font-sans text-[13px]">{t('library:listDetail.sort.recent')}</SelectItem>
              <SelectItem value="title" className="font-sans text-[13px]">{t('library:listDetail.sort.title')}</SelectItem>
              <SelectItem value="hours" className="font-sans text-[13px]">{t('library:listDetail.sort.hours')}</SelectItem>
            </SelectContent>
          </Select>
          <button
            type="button"
            aria-expanded={filtersOpen}
            aria-label={t('library:listDetail.filtersAria')}
            onClick={() => setFiltersOpen((v) => !v)}
            className={cx(
              'relative grid h-[30px] w-[30px] flex-none place-items-center rounded-[7px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              filtersOpen || extraCount > 0 ? 'bg-surface-strong text-text' : 'bg-surface text-text-faint hover:text-text-muted'
            )}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" aria-hidden />
            {extraCount > 0 && (
              <span className="absolute -right-1 -top-1 grid h-4 min-w-4 place-items-center rounded-full bg-accent px-1 font-mono text-[10px] font-bold tabular-nums text-on-accent">{extraCount}</span>
            )}
          </button>
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
        {filtersOpen && (
          <div className="mx-2.5 mb-2 grid gap-2 rounded-[7px] bg-surface p-2.5 ring-1 ring-hairline">
            <TierFilterControl value={tier} onChange={setTier} />
            <StatusFilterControl value={statusFilter} onChange={setStatusFilter} />
            {hasBorrowed && (
              <Select value={lendingFilter} onValueChange={(v) => setLending(v as 'all' | 'with-me')}>
                <SelectTrigger aria-label={t('library:family')} className="font-sans text-[13px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all" className="font-sans text-[13px]">{t('library:all')}</SelectItem>
                  <SelectItem value="with-me" className="font-sans text-[13px]">{t('library:sharedWithMe')}</SelectItem>
                </SelectContent>
              </Select>
            )}
          </div>
        )}
        {(p.warnings ?? []).length > 0 && (
          <div role="status" className="mx-2.5 mb-2 flex items-center gap-2 rounded-[7px] bg-danger/[0.08] px-2.5 py-2 text-[12px] ring-1 ring-danger/25">
            <CircleAlert className="h-3.5 w-3.5 flex-none text-danger" aria-hidden />
            <span className="min-w-0 flex-1 truncate text-text-body">{t('library:listDetail.warnings', { count: (p.warnings ?? []).length })}</span>
            <button
              type="button"
              onClick={() => p.onOpenPlugins?.()}
              className="flex-none font-semibold text-accent-strong hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
            >
              {t('library:plugins')}
            </button>
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
          <ul ref={listRef} onKeyDown={onListKey} className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-3">
            {filtered.map((g) => {
              const sub = subLine(t, g);
              const on = g.id === selected?.id;
              const art = heroImages(g)[0];
              const attn = needsAttention(g);
              return (
                <li key={g.id}>
                  <button
                    type="button"
                    data-game-row={g.id}
                    aria-current={on ? 'true' : undefined}
                    onClick={() => setSelectedId(g.id)}
                    onDoubleClick={() => p.onPlay(g.id)}
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
                    {attn && (
                      <span title={attentionReason(g)} aria-label={t('library:listDetail.needsAttention')} className="flex-none text-attention">
                        <CircleAlert className="h-4 w-4" aria-hidden />
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
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
