// i18n: converted
// The Atlas library as a game list with the game on the stage (issue #126,
// D21.jsx `.list` + `.stage`). Only the list-detail presentation renders
// here; the grid keeps today's LibraryView path exactly, so warm, cool and
// slate render as before.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { CircleAlert, Search } from 'lucide-react';
import type { Collection, Game, StoreType } from '../types';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { cx } from './ui';
import { byRecent, heroImages, hoursPlayed, lastPlayedLabel, needsAttention, attentionReason } from '../lib/games';
import { sortKey } from '../lib/customize';
import { collectionPredicate } from '../lib/collections';

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
}

function sortList(games: Game[], sort: ListSort): Game[] {
  const list = [...games];
  if (sort === 'title') list.sort((a, b) => sortKey(a).localeCompare(sortKey(b)));
  else if (sort === 'hours') list.sort((a, b) => (hoursPlayed(b) ?? -1) - (hoursPlayed(a) ?? -1) || a.title.localeCompare(b.title));
  else list.sort(byRecent);
  return list;
}

/** Sub-line under the title: the last played word, or "Not installed" greyed. */
function subLine(t: (k: string) => string, g: Game): { text: string; absent: boolean } {
  if (!g.is_installed) return { text: t('library:listDetail.notInstalled'), absent: true };
  const last = lastPlayedLabel(g);
  if (last) return { text: last, absent: false };
  return { text: t('library:listDetail.neverPlayed'), absent: false };
}

export function LibraryListDetail(p: ListDetailProps) {
  const { t } = useTranslation('library');
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<ListSort>('recent');
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
        (q === '' || g.title.toLowerCase().includes(q)),
    );
    return sortList(list, sort);
  }, [p.games, p.ownership, p.source, p.storeSelection, inCollection, query, sort]);

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

  // The stage scrolls back to the top when the game changes.
  useEffect(() => {
    const root = stageRef.current?.closest('[data-scroll-root]') as HTMLElement | null;
    if (root) root.scrollTo({ top: 0 });
    else stageRef.current?.scrollIntoView({ block: 'start' });
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
    else if (e.key === 'Enter') { e.preventDefault(); if (selected) p.onPlay(selected.id); }
  };

  return (
    <div className="flex min-h-0 flex-1 items-stretch">
      <aside aria-label={t('library:listDetail.listLabel')} className="flex w-[260px] min-w-0 flex-none flex-col border-r border-hairline bg-raised min-[1100px]:w-[296px]">
        <div className="flex gap-1.5 p-2.5">
          <label className="flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-xl bg-surface px-2.5 text-text-faint">
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
            <SelectTrigger aria-label={t('library:listDetail.sortAria')} className="w-[118px] flex-none">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="recent">{t('library:listDetail.sort.recent')}</SelectItem>
              <SelectItem value="title">{t('library:listDetail.sort.title')}</SelectItem>
              <SelectItem value="hours">{t('library:listDetail.sort.hours')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {filtered.length === 0 ? (
          <p className="px-4 py-4 text-[13px] text-text-muted">
            {t('library:listDetail.empty')}{' '}
            <button
              type="button"
              onClick={() => { setQuery(''); p.onResetFilters?.(); }}
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
                    aria-current={on}
                    onClick={() => setSelectedId(g.id)}
                    onDoubleClick={() => p.onPlay(g.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') { e.preventDefault(); p.onPlay(g.id); }
                    }}
                    className={cx(
                      'flex w-full items-center gap-2.5 rounded-xl px-1.5 py-[5px] text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                      on ? 'bg-accent/[0.15] text-text' : 'text-text-muted hover:bg-surface hover:text-text',
                    )}
                  >
                    {art ? (
                      <img src={art} alt="" loading="lazy" className={cx('h-7 w-[60px] flex-none rounded-md object-cover', !g.is_installed && 'brightness-[0.55] grayscale')} />
                    ) : (
                      <span aria-hidden className="h-7 w-[60px] flex-none rounded-md bg-surface-strong" />
                    )}
                    <span className="grid min-w-0 flex-1">
                      <span className={cx('truncate text-[13px] font-semibold', !g.is_installed && 'text-text-faint')}>{g.title}</span>
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
      <div ref={stageRef} className="min-w-0 flex-1">
        {selected ? p.renderStage(selected) : null}
      </div>
    </div>
  );
}
