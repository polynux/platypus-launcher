// i18n: converted
// The command palette (issue #124): games, places, and the current game's
// actions, opened with Ctrl K or by the shell's "Search or run a command"
// entry. It is one dialog for every theme and speaks tokens only.
import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { TFunction } from 'i18next';
import type { Game, SessionEvent, StorePlugin, View } from '../types';
import { Modal } from './Modal';
import { Input } from './ds/input';
import { cx } from './ui';
import { filterPaletteItems, type PaletteItem } from '../lib/palette';
import { isLiveState } from '../lib/sessions';
import { hoursPlayed } from '../lib/games';

/** The event the shell dispatches to open the palette (issue #125). */
export const COMMAND_PALETTE_EVENT = 'platypus:command-palette';

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
}

/** Mounted once at the app root: listens for Ctrl K and for the shell's event,
 *  and renders the dialog only while it is open. */
export function CommandPalette(p: CommandPaletteProps) {
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const show = () => setOpen(true);
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.altKey || e.key.toLowerCase() !== 'k') return;
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

function PaletteDialog({ p, onClose }: { p: CommandPaletteProps; onClose: () => void }) {
  const { t } = useTranslation(['palette', 'nav', 'library']);
  const [query, setQuery] = useState('');
  const [highlight, setHighlight] = useState(0);
  const items = useMemo(() => filterPaletteItems(paletteItems(p, t), query), [p, t, query]);
  // A filter can shrink the list under the highlight; the last row stands in.
  const hi = Math.max(0, Math.min(highlight, items.length - 1));
  useEffect(() => setHighlight(0), [query]);
  const run = (item: PaletteItem | undefined) => {
    if (!item) return;
    onClose();
    item.run();
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
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
  const rows = useMemo(() => {
    let last = '';
    return items.map((item, index) => {
      const heading = item.group !== last ? item.group : undefined;
      last = item.group;
      return { item, index, heading };
    });
  }, [items]);
  return (
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
            {rows.map(({ item, index: i, heading }) => (
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
