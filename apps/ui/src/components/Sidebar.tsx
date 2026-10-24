// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, Search, Settings, Square, Ticket } from 'lucide-react';
import { cx, clock } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { storeLabel } from '../lib/games';
import { FriendsSummary } from './FriendsSummary';
import { PluginLogo } from './PluginLogo';
import type { Collection, SessionEvent, SocialView, StoreType, View } from '../types';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'storage' | 'settings';

interface SidebarProps {
  view: View;
  onNavigate: (v: View) => void;
  onSettingsOpen?: () => void;
  search: string;
  onSearch: (q: string) => void;
  storeCounts: Record<string, number>;
  onPickSource: (s: StoreType) => void;
  /** Plugin logos by store (and plugin id): the Sources rows' tiles. */
  sourceLogos?: Record<string, string>;
  gameCount: number;
  /** The first library scan is still running: counts show as pending, not zero. */
  loading?: boolean;
  downloadCount: number;
  authNeeded: boolean; // a plugin waits for approval or login
  fixCount: number;
  runnerCount: number;
  lastScan: number | null;
  sessions: (SessionEvent & { title: string })[];
  onStop: (gameId: string) => void;
  /** Friends lists of the stores that offer them (none: no block). */
  friends: SocialView[];
  /** Store names by plugin id. */
  friendLabels: Record<string, string>;
  onOpenFriends: () => void;
  /** Some ready store offers redeem: the one "Redeem a code" entry shows. */
  canRedeem: boolean;
  onRedeem: () => void;
  /** Unread messages from friends across the stores: the friends block's badge. */
  friendsUnread?: number;
  /** Enabled plugins that declare a storefront (the embedded store section). Empty when none. */
  stores?: { id: string; name: string; icon_url?: string }[];
  /** The store whose embedded view is shown, if any. */
  activeStore?: string | null;
  onOpenStore?: (pluginId: string) => void;
  /** The collections (issue #45): the pinned ones show under their own heading, in the list's order. */
  collections?: Collection[];
  /** Library games per collection id, for the counts. */
  collectionCounts?: Record<string, number>;
  onOpenCollection?: (id: string) => void;
  onManageCollections?: () => void;
}

const SOURCES: { key: StoreType; label: string }[] = [
  { key: 'steam', label: 'Steam' },
  { key: 'gog', label: 'GOG' },
  { key: 'epic', label: 'Epic' },
];

/** Whether the Tools group is open. Remembered per machine; closed until the user opens it. */
const TOOLS_OPEN_KEY = 'platypus.sidebar.toolsOpen';
function readToolsOpen(): boolean {
  try {
    return localStorage.getItem(TOOLS_OPEN_KEY) === 'true';
  } catch {
    return false;
  }
}

type NavItem = { key: NavKey; label: string; count?: number; dot?: boolean };

export function Sidebar(p: SidebarProps) {
  const { t } = useTranslation(['nav', 'common', 'collections']);
  const active: NavKey | 'store' = p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;
  // Enabled plugins that declare a storefront: the embedded store section (issue #17).
  const storeEntries = p.stores ?? [];
  const [toolsOpen, setToolsOpen] = useState(readToolsOpen);
  const toggleTools = () => {
    setToolsOpen((open) => {
      try {
        localStorage.setItem(TOOLS_OPEN_KEY, String(!open));
      } catch {
        // Storage can be unavailable; the group still toggles for this session.
      }
      return !open;
    });
  };

  const main: NavItem[] = [
    { key: 'dashboard', label: t('nav:dashboard') },
    { key: 'library', label: t('nav:library'), count: p.gameCount },
    { key: 'downloads', label: t('nav:downloads'), count: p.downloadCount || undefined },
    { key: 'activity', label: t('nav:activity') },
  ];
  const tools: NavItem[] = [
    { key: 'fixes', label: t('nav:fixes'), count: p.fixCount || undefined },
    { key: 'runners', label: t('nav:runners'), count: p.runnerCount || undefined },
    { key: 'plugins', label: t('nav:plugins'), dot: p.authNeeded },
    { key: 'storage', label: t('nav:storage') },
  ];
  // A closed Tools group still shows that a tool is active or needs attention on its header.
  const toolsHasActive = tools.some((t) => t.key === active);
  const toolsAttention = tools.reduce((n, t) => n + (t.count ?? 0), 0);
  const toolsDot = tools.some((t) => t.dot);
  // Opening a tool's view (a warning's Plugins button, a deep link) shows its item; the choice is not stored.
  useEffect(() => {
    if (toolsHasActive) setToolsOpen(true);
  }, [toolsHasActive]);

  const navButton = (n: NavItem, indent = false) => (
    <button
      key={n.key}
      onClick={() => p.onNavigate({ name: n.key } as View)}
      aria-current={active === n.key ? 'page' : undefined}
      className={cx(
        'mb-0.5 flex w-full items-center justify-between rounded-xl py-2 pr-3 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
        indent ? 'pl-7' : 'px-3',
        active === n.key ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface-strong hover:text-white'
      )}
    >
      <span>{n.label}</span>
      {n.key === 'library' && p.loading ? (
        <span aria-label={t('nav:countingGames')} className="h-4 w-7 animate-pulse rounded-full bg-white/15 motion-reduce:animate-none" />
      ) : (
        n.count != null && <Badge variant="neutral" className="h-5 rounded-full tracking-normal tabular-nums">{n.count}</Badge>
      )}
      {n.dot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
    </button>
  );

  return (
    <aside className="flex h-full w-56 flex-none flex-col border-r border-hairline bg-black/25 p-4">
      <div className="mb-5 flex items-center gap-2.5 px-1.5">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-accent-strong to-rose-500 text-[15px] font-black text-on-accent">P</span>
        <div>
          <div className="text-[15px] font-extrabold tracking-tight">Platypus</div>
          <div className="font-mono text-micro uppercase tracking-[0.16em] text-text-muted">{t('nav:tagline')}</div>
        </div>
      </div>

      <label className="relative mb-4 block flex-none">
        <span className="sr-only">{t('nav:searchLibrary')}</span>
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-muted" />
        <Input
          type="search"
          value={p.search}
          // Typing never navigates (FE-12): the view only changes on Enter.
          onChange={(e) => p.onSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && p.view.name !== 'library') p.onNavigate({ name: 'library' });
            if (e.key === 'Escape') p.onSearch('');
          }}
          placeholder={t('nav:searchPlaceholder')}
          className="pl-8.5 font-sans"
        />
      </label>

      {/* The nav and sources scroll when the window is short; the search above and the footer below stay put. */}
      <div className="min-h-0 flex-1 overflow-y-auto pr-1 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
        <nav aria-label={t('nav:main')}>
          {main.map((n) => navButton(n))}

          <button
            type="button"
            onClick={toggleTools}
            aria-expanded={toolsOpen}
            aria-controls="sidebar-tools"
            className={cx(
              'mt-2 flex w-full items-center justify-between rounded-xl px-3 py-1.5 font-mono text-micro uppercase tracking-[0.18em] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
              toolsHasActive && !toolsOpen ? 'text-amber-200' : 'text-text-muted hover:text-text-body'
            )}
          >
            <span>{t('nav:tools')}</span>
            <span className="flex items-center gap-2">
              {!toolsOpen && toolsAttention > 0 && <Badge variant="neutral" className="h-4 rounded-full px-1.5 tracking-normal tabular-nums">{toolsAttention}</Badge>}
              {!toolsOpen && toolsDot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
              <ChevronDown className={cx('h-3.5 w-3.5 transition-transform motion-reduce:transition-none', toolsOpen ? '' : '-rotate-90')} aria-hidden />
            </span>
          </button>
          <div id="sidebar-tools" hidden={!toolsOpen}>
            {tools.map((n) => navButton(n, true))}
          </div>
        </nav>

        {p.onManageCollections && (
          <>
            <div className="flex items-center justify-between px-2 pb-1.5 pt-4">
              <span className="font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('collections:title')}</span>
              <button type="button" onClick={p.onManageCollections} className="font-mono text-micro uppercase tracking-[0.14em] text-text-muted transition hover:text-white/80">
                {t('collections:manage')}
              </button>
            </div>
            {(p.collections ?? []).filter((c) => c.pinned).map((c) => {
              const current = p.view.name === 'library' && p.view.collection === c.id;
              return (
                <button
                  key={c.id}
                  onClick={() => p.onOpenCollection?.(c.id)}
                  aria-current={current ? 'page' : undefined}
                  className={cx(
                    'mb-0.5 flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-[12.5px] transition',
                    current ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface hover:text-white'
                  )}
                >
                  <span className="truncate">{c.builtin ? t(`collections:builtin.${c.id}`) : c.name}</span>
                  <span className="font-mono text-hint tabular-nums text-text-muted">{p.collectionCounts?.[c.id] ?? '—'}</span>
                </button>
              );
            })}
          </>
        )}

        <div className="px-2 pb-1.5 pt-4 font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('nav:sources')}</div>
        {[
          ...SOURCES,
          // Stores that only plugins provide, then sideloaded games.
          ...Object.keys(p.storeCounts)
            .filter((k) => k !== 'custom' && !SOURCES.some((s) => s.key === k))
            .map((k) => ({ key: k, label: storeLabel(k) })),
          ...(p.storeCounts.custom ? [{ key: 'custom', label: t('nav:sideloaded') }] : []),
        ].map((s) => {
          const count = p.storeCounts[s.key] ?? 0;
          return (
            <button
              key={s.key}
              onClick={() => p.onPickSource(s.key)}
              className="mb-0.5 flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-[12.5px] text-text-body transition hover:bg-surface hover:text-white"
            >
              <span className="flex items-center gap-2">
                <PluginLogo src={p.sourceLogos?.[s.key]} name={s.label} size="xs" />
                {s.label}
              </span>
              <span className="font-mono text-hint tabular-nums text-text-muted">
                {p.loading ? <span aria-label={t('nav:countingGames')} className="inline-block h-2.5 w-5 animate-pulse rounded bg-white/15 motion-reduce:animate-none" /> : count || '—'}
              </span>
            </button>
          );
        })}
        {storeEntries.length > 0 && (
          <>
            <div className="px-2 pb-1.5 pt-4 font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('nav:store')}</div>
            {storeEntries.map((s) => (
              <button
                key={s.id}
                onClick={() => p.onOpenStore?.(s.id)}
                aria-current={p.activeStore === s.id ? 'page' : undefined}
                className={cx(
                  'mb-0.5 flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-[12.5px] transition',
                  p.activeStore === s.id ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface hover:text-white'
                )}
              >
                <PluginLogo src={s.icon_url} name={s.name} size="xs" />
                {s.name}
              </button>
            ))}
          </>
        )}
      </div>

      {/* Pinned footer: fixed content, never grows. The friends block is the only part that changes size. */}
      <div className="mt-auto flex flex-none flex-col gap-2 pt-4">
        {p.canRedeem && (
          <Button variant="neutral" onClick={p.onRedeem} className="w-full">
            <Ticket className="h-3.5 w-3.5" /> {t('nav:redeemCode')}
          </Button>
        )}
        <button
          type="button"
          onClick={() => p.onSettingsOpen ? p.onSettingsOpen() : p.onNavigate({ name: 'settings' })}
          aria-current={active === 'settings' ? 'page' : undefined}
          className={cx(
            'flex w-full items-center gap-2 rounded-xl px-3 py-2 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
            active === 'settings' ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface-strong hover:text-white'
          )}
        >
          <Settings className="h-3.5 w-3.5" aria-hidden />
          {t('nav:settings')}
        </button>
        <FriendsSummary views={p.friends} labels={p.friendLabels} onOpen={p.onOpenFriends} unread={p.friendsUnread} />
        {p.sessions.map((e) => {
          const running = e.state === 'running';
          const label = running ? t('nav:playing') : e.state === 'stopping' ? t('nav:stopping') : t('nav:starting');
          return (
            <div
              key={e.game_id}
              title={e.message}
              className={cx('flex items-center gap-2 rounded-xl px-3 py-2 ring-1', running ? 'bg-emerald-400/[0.08] ring-emerald-300/25' : 'bg-accent-strong/[0.07] ring-accent-strong/25')}
            >
              <span className={cx('h-2 w-2 flex-none rounded-full', running ? 'bg-emerald-400' : 'animate-pulse bg-accent-strong')} />
              <span className="min-w-0 flex-1">
                <span className={cx('block font-mono text-micro uppercase tracking-wider', running ? 'text-emerald-200' : 'text-amber-200')}>
                  {label}
                </span>
                <span className="block truncate text-hint font-semibold text-white/85">{e.title}</span>
              </span>
              {e.state !== 'stopping' && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => p.onStop(e.game_id)}
                      aria-label={t('nav:stopGame', { title: e.title })}
                      className="size-7 flex-none text-text-muted hover:text-red-200"
                    >
                      <Square className="h-3 w-3 fill-current" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('nav:stop')}</TooltipContent>
                </Tooltip>
              )}
            </div>
          );
        })}
        <div className="truncate px-1.5 font-mono text-[12px] text-text-muted">
          platypus v0.2{p.lastScan ? ` · scan ${clock(p.lastScan)}` : ''}
        </div>
      </div>
    </aside>
  );
}
