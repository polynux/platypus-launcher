// i18n: converted
// The command palette (issue #124): games, places, and the current game's
// actions, opened with Ctrl K or by the shell's "Search or run a command"
// entry. It is one dialog for every theme and speaks tokens only.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import { ArrowRight, Check, CornerDownLeft, X } from 'lucide-react';
import type { Collection, Game, SessionEvent, StorePlugin, View } from '../types';
import { Modal } from './Modal';
import { Input } from './ds/input';
import { PluginLogo } from './PluginLogo';
import { cx } from './ui';
import { filterPaletteItems, type PaletteItem } from '../lib/palette';
import { isLiveState } from '../lib/sessions';
import { hoursPlayed, storeLabel } from '../lib/games';
import { useVariant } from '../lib/theme';

/** The event the shell dispatches to open the palette (issue #125). */
export const COMMAND_PALETTE_EVENT = 'platypus:command-palette';

/** The dialogs on screen. Radix keeps the page's other content inert instead
 *  of setting aria-modal, so the app's own dialog content slot marks a modal
 *  too; the lightbox and other hand-written modals use aria-modal. A
 *  non-modal flyout (the game page's Tune panel) is not one, and leaves Ctrl K
 *  alone. */
const MODAL_SELECTOR = '[role="dialog"][aria-modal="true"], [role="dialog"][data-slot="dialog-content"]';

/** Whether a modal other than the palette is on screen: the Settings drawer
 *  with unsaved edits, the lightbox, a game's dialog. `paletteOpen` says
 *  whether one of the dialogs is the palette's own. */
function foreignModalOpen(paletteOpen: boolean): boolean {
  if (typeof document === 'undefined') return false;
  const dialogs = document.querySelectorAll(MODAL_SELECTOR);
  return dialogs.length > (paletteOpen ? 1 : 0);
}

/** Open the command palette from anywhere. The app root listens for this. */
export function openCommandPalette(): void {
  window.dispatchEvent(new CustomEvent(COMMAND_PALETTE_EVENT));
}

export interface CommandPaletteProps {
  games: Game[];
  /** The game page's game: the actions act on it. */
  currentGame?: Game;
  /** The current game's session, so the palette offers Stop while it runs. */
  session?: SessionEvent;
  /** The ready stores: each one is a place to go. */
  stores: StorePlugin[];
  onOpenGame: (id: string) => void;
  onNavigate: (view: View) => void;
  onOpenStore: (pluginId: string) => void;
  /** The shell's Settings entry, so the palette reaches the same place. */
  onOpenSettings: () => void;
  onPlay: (game: Game) => void;
  onStop: (gameId: string) => void;
  onInstall: (game: Game) => void;
  onOpenGameSettings: (gameId: string) => void;
  /** Atlas parity (issue #132): the redeem dialog, when a store offers it. */
  onRedeem?: () => void;
  /** Atlas parity (issue #132): the collections manager dialog. */
  onManageCollections?: () => void;
  /** Atlas parity (issue #132): the add-a-game dialog. */
  onAddGame?: () => void;
  /** Atlas friends entry (review 4): opens the friends side panel. */
  onOpenFriends?: () => void;
  /** Atlas collections for the sub-mode, with their game counts. */
  collections?: Collection[];
  collectionTotals?: Record<string, number>;
  /** The library's current collection, for the sub-mode's check. */
  collectionId?: string | null;
  onShowCollection?: (id: string) => void;
  /** Atlas new-collection dialog (the manager until R4-27 lands it). */
  onNewCollection?: () => void;
  /** Every live session, so the palette offers Stop for each running game. */
  sessions?: (SessionEvent & { title: string })[];
  /** Store art by store key, for the game rows' glyphs. */
  sourceLogos?: Record<string, string>;
}

/** Mounted once at the app root: listens for Ctrl K and for the shell's event,
 *  and renders the dialog only while it is open. Another dialog on screen
 *  keeps both shortcuts: stacking the palette over a modal with unsaved edits
 *  leaves two dialogs open (issue #124). */
export function CommandPalette(p: CommandPaletteProps) {
  const [open, setOpen] = useState(false);
  // The listeners are added once, so they read the state from here.
  const openRef = useRef(open);
  useEffect(() => { openRef.current = open; }, [open]);
  useEffect(() => {
    const show = () => {
      if (foreignModalOpen(openRef.current)) return;
      setOpen(true);
    };
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.altKey || e.key.toLowerCase() !== 'k') return;
      if (foreignModalOpen(openRef.current)) return;
      e.preventDefault();
      setOpen((v) => !v);
    };
    window.addEventListener(COMMAND_PALETTE_EVENT, show);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener(COMMAND_PALETTE_EVENT, show);
      window.removeEventListener('keydown', onKey);
    };
  }, []);
  if (!open) return null;
  return <PaletteDialog p={p} onClose={() => setOpen(false)} />;
}

/** The /22 kbd (kit.css kbd, d22 radius 3px): the bar's esc and the footer's keys share it. */
const KBD = 'flex-none rounded-[3px] px-[5px] py-px font-mono text-[11px] text-text-muted ring-1 ring-inset ring-hairline-strong';

function PaletteDialog({ p, onClose }: { p: CommandPaletteProps; onClose: () => void }) {
  const { t } = useTranslation(['palette', 'nav', 'library', 'collections']);
  // Under Atlas only (label.style = plain): the /22 palette. Warm, cool and
  // slate keep the generic palette below, exactly as before.
  const plain = useVariant('label.style') === 'plain';
  const [mode, setMode] = useState<'root' | 'collection'>('root');
  const [query, setQuery] = useState('');
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  useEffect(() => { setQuery(''); setHighlight(0); }, [mode]);
  const rootItems = useMemo(() => (plain ? paletteItemsAtlas(p, t) : paletteItems(p, t)), [p, t, plain]);
  const subItems = useMemo(() => (plain ? collectionItems(p, t) : []), [p, t, plain]);
  const all = mode === 'root' ? rootItems : subItems;
  const items = useMemo(() => filterPaletteItems(all, query), [all, query]);
  // A filter can shrink the list under the highlight; the last row stands in.
  const hi = Math.max(0, Math.min(highlight, items.length - 1));
  useEffect(() => setHighlight(0), [query]);
  // Arrowing past the visible rows leaves the highlight off-screen; the list
  // follows it. Atlas only, so the other themes never touch scrollIntoView.
  useEffect(() => {
    if (!plain) return;
    (listRef.current?.querySelector('[data-highlighted]') as HTMLElement | null)?.scrollIntoView?.({ block: 'nearest' });
  }, [hi, plain]);
  const run = (item: PaletteItem | undefined) => {
    if (!item) return;
    // A sub-mode row switches the palette instead of closing it.
    if (plain && item.next === 'collection') {
      setMode('collection');
      return;
    }
    onClose();
    item.run();
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (plain && mode !== 'root' && e.key === 'Backspace' && query === '') {
      e.preventDefault();
      setMode('root');
      return;
    }
    if (items.length === 0) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlight(Math.min(items.length - 1, hi + 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlight(Math.max(0, hi - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(items[hi]);
    }
  };
  if (!plain) return (
    <Modal label={t('palette:label')} size="md" onClose={onClose}>
      <div onKeyDown={onKeyDown}>
        <Input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('palette:placeholder')}
          aria-label={t('palette:input')}
          role="combobox"
          aria-expanded
          aria-controls={items.length > 0 ? 'command-palette-list' : undefined}
          aria-autocomplete="list"
          aria-activedescendant={items.length > 0 ? `command-palette-item-${hi}` : undefined}
        />
        {items.length === 0 ? (
          <p className="mt-3 px-2 py-4 text-[13px] text-text-muted">{t('palette:empty', { query })}</p>
        ) : (
          <ul
            id="command-palette-list"
            role="listbox"
            aria-label={t('palette:listbox')}
            className="mt-3 max-h-[45vh] overflow-y-auto"
          >
            {rows(items).map(({ item, index: i, heading }) => (
              <React.Fragment key={item.id}>
                {heading !== undefined && (
                  <li role="presentation" aria-hidden="true" className="px-2 pb-1 pt-2 font-mono text-micro uppercase tracking-[0.14em] text-text-faint">
                    {heading}
                  </li>
                )}
                <li
                  id={`command-palette-item-${i}`}
                  role="option"
                  aria-selected={i === hi}
                  data-highlighted={i === hi ? 'true' : undefined}
                  onPointerMove={() => setHighlight(i)}
                  onClick={() => run(item)}
                  className={cx(
                    'flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-[13px]',
                    i === hi ? 'bg-surface-strong text-text' : 'text-text-body',
                  )}
                >
                  <span className="min-w-0 flex-1 truncate">{item.label}</span>
                  {item.hint && <span className="flex-none text-hint text-text-faint">{item.hint}</span>}
                </li>
              </React.Fragment>
            ))}
          </ul>
        )}
        <div className="mt-3 flex gap-4 border-t border-hairline pt-2 font-mono text-micro text-text-faint">
          <span><kbd>↑</kbd> <kbd>↓</kbd> {t('palette:hint.move')}</span>
          <span><kbd>↵</kbd> {t('palette:hint.run')}</span>
          <span><kbd>{t('palette:key.esc')}</kbd> {t('palette:hint.close')}</span>
        </div>
      </div>
    </Modal>
  );
  // The /22 palette (kit.jsx Palette): anchored from the top, a glass panel
  // with an edge-to-edge input bar, grouped rows with glyphs, and a key
  // footer. Esc in the sub-mode goes back to the root instead of closing.
  return (
    <Modal
      label={t('palette:label')}
      size="md"
      onClose={onClose}
      overlayClassName="place-items-start justify-items-center bg-black/[.38] p-0 pt-[14vh]"
      className="w-[min(620px,92vw)] max-w-none"
      contentClassName="rounded-[14px] bg-glass ring-hairline-strong backdrop-blur-[22px] backdrop-saturate-[1.4]"
      innerClassName="flex flex-col overflow-hidden p-0"
      contentStyle={{ boxShadow: '0 24px 64px rgba(0,0,0,.55)' }}
      onEscapeKeyDown={(e) => { if (mode !== 'root') { e.preventDefault(); setMode('root'); } }}
    >
      <div onKeyDown={onKeyDown}>
        <div className="flex h-[50px] flex-none items-center gap-2 border-b border-hairline px-3.5">
          {mode !== 'root' && (
            <button
              type="button"
              onClick={() => { setMode('root'); inputRef.current?.focus(); }}
              title={t('palette:chip.back')}
              className="inline-flex h-6 flex-none items-center gap-[5px] rounded-md bg-accent pl-2 pr-1.5 text-[12.4px] font-bold text-on-accent"
            >
              {t('palette:chip.collection')}
              <X size={11} aria-hidden />
            </button>
          )}
          <input
            ref={inputRef}
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={mode !== 'root' ? t('palette:searchCollections') : t('palette:placeholder')}
            aria-label={t('palette:input')}
            role="combobox"
            aria-expanded
            aria-controls={items.length > 0 ? 'command-palette-list' : undefined}
            aria-autocomplete="list"
            aria-activedescendant={items.length > 0 ? `command-palette-item-${hi}` : undefined}
            className="h-full min-w-0 flex-1 bg-transparent text-[15px] text-text outline-none placeholder:text-text-faint"
          />
          <kbd className={KBD}>{t('palette:key.esc')}</kbd>
        </div>
        <ul
          ref={listRef}
          id="command-palette-list"
          role="listbox"
          aria-label={t('palette:listbox')}
          className="max-h-[min(380px,52vh)] overflow-y-auto p-1.5"
        >
          {items.length === 0 ? (
            <li className="px-2.5 py-[18px] text-text-muted">{t('palette:empty', { query })}</li>
          ) : (
            rows(items).map(({ item, index: i, heading }) => (
              <React.Fragment key={item.id}>
                {heading !== undefined && (
                  <li role="presentation" aria-hidden="true" className="px-2.5 pb-1 pt-2.5 text-[11.6px] text-text-faint">
                    {heading}
                  </li>
                )}
                <li
                  id={`command-palette-item-${i}`}
                  role="option"
                  aria-selected={i === hi}
                  data-highlighted={i === hi ? 'true' : undefined}
                  onPointerMove={() => setHighlight(i)}
                  onClick={() => run(item)}
                  className={cx(
                    'flex h-[34px] cursor-default items-center gap-2.5 rounded-lg px-2.5 text-[13.5px] text-text',
                    i === hi && 'bg-[color-mix(in_srgb,var(--color-accent)_18%,transparent)]',
                  )}
                >
                  <span className="grid w-4 flex-none place-items-center text-text-muted">
                    {item.store ? (
                      <PluginLogo src={item.logoSrc ?? null} name={item.storeName ?? item.store} size="glyph" />
                    ) : item.active ? (
                      <Check size={13} className="text-ok" aria-hidden />
                    ) : item.next ? (
                      <ArrowRight size={13} aria-hidden />
                    ) : (
                      <span className="size-1 rounded-full bg-text-faint" aria-hidden />
                    )}
                  </span>
                  <span className={cx('min-w-0 flex-1 truncate', item.tone === 'bad' && 'text-danger')}>{item.label}</span>
                  {item.hint && <span className="whitespace-nowrap text-[12.4px] text-text-faint">{item.hint}</span>}
                  {item.kbd && <kbd className={KBD}>{item.kbd}</kbd>}
                  {i === hi && <CornerDownLeft className="size-3 flex-none text-text-muted" aria-hidden />}
                </li>
              </React.Fragment>
            ))
          )}
        </ul>
        <div className="flex gap-4 border-t border-hairline px-3.5 py-2 text-[11.6px] text-text-faint">
          <span className="inline-flex items-center gap-1"><kbd className={KBD}>↑</kbd><kbd className={KBD}>↓</kbd> {t('palette:hint.move')}</span>
          <span className="inline-flex items-center gap-1"><kbd className={KBD}>↵</kbd> {t('palette:hint.run')}</span>
          {mode !== 'root' && <span className="inline-flex items-center gap-1"><kbd className={KBD}>⌫</kbd> {t('palette:hint.back')}</span>}
        </div>
      </div>
    </Modal>
  );
}

/** Group the list's rows under their headings, in the list's own order. */
function rows(items: PaletteItem[]): { item: PaletteItem; index: number; heading: string | undefined }[] {
  let last = '';
  return items.map((item, index) => {
    const heading = item.group !== last ? item.group : undefined;
    last = item.group;
    return { item, index, heading };
  });
}

/** Every row the palette can show, in built order: the current game's actions,
 *  the places, then the games. The filter keeps the order within a score. */
function paletteItems(p: CommandPaletteProps, t: TFunction): PaletteItem[] {
  const items: PaletteItem[] = [];
  const game = p.currentGame;
  if (game) {
    const live = p.session ? isLiveState(p.session.state) : false;
    const action = (id: string, label: string, run: () => void) => items.push({ id, group: game.title, label, kind: 'action', run });
    if (live) action('action-stop', t('palette:action.stop', { title: game.title }), () => p.onStop(game.id));
    else if (game.is_installed) action('action-play', t('palette:action.play', { title: game.title }), () => p.onPlay(game));
    else action('action-install', t('palette:action.install', { title: game.title }), () => p.onInstall(game));
    action('action-settings', t('palette:action.gameSettings'), () => p.onOpenGameSettings(game.id));
  }
  const group = t('palette:group.places');
  const place = (id: string, label: string, run: () => void) => items.push({ id: `place-${id}`, group, label, kind: 'place', run });
  place('dashboard', t('nav:dashboard'), () => p.onNavigate({ name: 'dashboard' }));
  place('library', t('nav:library'), () => p.onNavigate({ name: 'library' }));
  place('downloads', t('nav:downloads'), () => p.onNavigate({ name: 'downloads' }));
  place('activity', t('nav:activity'), () => p.onNavigate({ name: 'activity' }));
  place('fixes', t('nav:fixes'), () => p.onNavigate({ name: 'fixes' }));
  place('runners', t('nav:runners'), () => p.onNavigate({ name: 'runners' }));
  place('plugins', t('nav:plugins'), () => p.onNavigate({ name: 'plugins' }));
  place('storage', t('nav:storage'), () => p.onNavigate({ name: 'storage' }));
  place('settings', t('nav:settings'), () => p.onOpenSettings());
  for (const store of p.stores) place(`store-${store.id}`, store.name, () => p.onOpenStore(store.id));
  if (p.onRedeem) place('redeem', t('palette:action.redeem'), () => p.onRedeem?.());
  if (p.onManageCollections) place('collections', t('palette:action.manageCollections'), () => p.onManageCollections?.());
  if (p.onAddGame) place('add-game', t('palette:action.addGame'), () => p.onAddGame?.());
  const games = t('palette:group.games');
  for (const g of p.games) {
    const hours = hoursPlayed(g);
    items.push({
      id: `game-${g.id}`,
      group: games,
      label: g.title,
      // Installed games say how long they were played; the rest say why not.
      hint: !g.is_installed
        ? t('library:cardNotInstalled')
        : hours === undefined
          ? undefined
          : t('library:cardHours', { hours }),
      kind: 'game',
      run: () => p.onOpenGame(g.id),
    });
  }
  return items;
}

/** The /22 palette's root (appPages.jsx paletteExtra): the Library set, the
 *  current game's actions, every running game, the storefronts and pages, and
 *  the game search. Atlas only; the other themes keep `paletteItems`. */
function paletteItemsAtlas(p: CommandPaletteProps, t: TFunction): PaletteItem[] {
  const items: PaletteItem[] = [];
  const game = p.currentGame;
  if (game) {
    const live = p.session ? isLiveState(p.session.state) : false;
    const action = (id: string, label: string, run: () => void, kbd?: string) =>
      items.push({ id, group: game.title, label, kind: 'action', kbd, run });
    if (live) action('action-stop', t('palette:action.stop', { title: game.title }), () => p.onStop(game.id), '↵');
    else if (game.is_installed) action('action-play', t('palette:action.play', { title: game.title }), () => p.onPlay(game), '↵');
    else action('action-install', t('palette:action.install', { title: game.title }), () => p.onInstall(game), '↵');
    action('action-settings', t('palette:action.gameSettingsAtlas'), () => p.onOpenGameSettings(game.id));
  }
  const library = t('palette:group.library');
  if (p.onRedeem) items.push({ id: 'library-redeem', group: library, label: t('palette:action.redeemAtlas'), kind: 'action', run: () => p.onRedeem?.() });
  if (p.onAddGame) items.push({ id: 'library-add', group: library, label: t('palette:action.addGameAtlas'), kind: 'action', run: () => p.onAddGame?.() });
  items.push({ id: 'library-collections', group: library, label: t('palette:action.showCollection'), kind: 'action', next: 'collection', run: () => {} });
  if (p.onNewCollection) items.push({ id: 'library-new-collection', group: library, label: t('palette:action.newCollection'), kind: 'action', run: () => p.onNewCollection?.() });
  const running = t('palette:group.running');
  const sessions = p.sessions ?? [];
  if (sessions.length === 0) {
    // Nothing to stop: running it just closes the palette.
    items.push({ id: 'running-none', group: running, label: t('palette:action.stopNone'), hint: t('palette:hint.nothingRunning'), kind: 'action', run: () => {} });
  } else {
    for (const s of sessions) {
      items.push({ id: `running-${s.game_id}`, group: running, label: t('palette:action.stop', { title: s.title }), kind: 'action', run: () => p.onStop(s.game_id) });
    }
  }
  const open = t('palette:group.open');
  for (const s of p.stores) {
    items.push({
      id: `open-store-${s.id}`,
      group: open,
      label: t('palette:action.openStore', { store: s.name }),
      kind: 'place',
      store: s.store || s.id,
      storeName: s.name,
      logoSrc: s.icon_url ?? p.sourceLogos?.[s.store] ?? p.sourceLogos?.[s.id],
      run: () => p.onOpenStore(s.id),
    });
  }
  const page = (id: string, label: string, run: () => void) => items.push({ id: `open-${id}`, group: open, label, kind: 'place', run });
  page('home', t('nav:home'), () => p.onNavigate({ name: 'dashboard' }));
  page('library', t('nav:library'), () => p.onNavigate({ name: 'library' }));
  if (p.stores.length > 0) page('store', t('nav:store'), () => p.onOpenStore(p.stores[0].id));
  if (p.onOpenFriends) page('friends', t('nav:sourcebar.friends'), () => p.onOpenFriends?.());
  page('downloads', t('nav:downloads'), () => p.onNavigate({ name: 'downloads' }));
  page('attention', t('nav:sourcebar.attention'), () => p.onNavigate({ name: 'fixes' }));
  page('activity', t('nav:activity'), () => p.onNavigate({ name: 'activity' }));
  page('runners', t('nav:runners'), () => p.onNavigate({ name: 'runners' }));
  page('plugins', t('nav:plugins'), () => p.onNavigate({ name: 'plugins' }));
  page('storage', t('nav:storage'), () => p.onNavigate({ name: 'storage' }));
  page('settings', t('nav:settings'), () => p.onOpenSettings());
  // The lab's Go to game group: dropping game search would be a regression.
  const goGame = t('palette:group.gotoGame');
  for (const g of p.games) {
    const hours = hoursPlayed(g);
    items.push({
      id: `game-${g.id}`,
      group: goGame,
      label: g.title,
      hint: !g.is_installed
        ? t('library:cardNotInstalled')
        : hours === undefined
          ? undefined
          : t('library:cardHours', { hours }),
      kind: 'game',
      store: g.source,
      storeName: storeLabel(g.source),
      logoSrc: p.sourceLogos?.[g.source],
      run: () => p.onOpenGame(g.id),
    });
  }
  return items;
}

/** The /22 palette's collection sub-mode: the built-in and user collections
 *  with their game counts. Enter sets the collection and opens the library. */
function collectionItems(p: CommandPaletteProps, t: TFunction): PaletteItem[] {
  const group = t('palette:group.collections');
  return (p.collections ?? []).map((c) => ({
    id: `collection-${c.id}`,
    group,
    label: c.builtin ? t(`collections:builtin.${c.id}`) : c.name,
    hint: t('palette:hint.gamesCount', { count: p.collectionTotals?.[c.id] ?? 0 }),
    kind: 'place' as const,
    active: p.collectionId != null && c.id === p.collectionId,
    run: () => p.onShowCollection?.(c.id),
  }));
}
