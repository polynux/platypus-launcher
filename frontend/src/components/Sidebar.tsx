import React, { useEffect, useState } from 'react';
import { ChevronDown, Search, Settings, Square, Ticket } from 'lucide-react';
import { cx, clock } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { storeLabel } from '../lib/games';
import { FriendsSummary } from './FriendsSummary';
import type { SessionEvent, SocialView, StoreType, View } from '../types';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'settings';

interface SidebarProps {
  view: View;
  onNavigate: (v: View) => void;
  search: string;
  onSearch: (q: string) => void;
  storeCounts: Record<string, number>;
  onPickSource: (s: StoreType) => void;
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
  const active: NavKey = p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;
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
    { key: 'dashboard', label: 'Dashboard' },
    { key: 'library', label: 'Library', count: p.gameCount },
    { key: 'downloads', label: 'Downloads', count: p.downloadCount || undefined },
    { key: 'activity', label: 'Activity' },
  ];
  const tools: NavItem[] = [
    { key: 'fixes', label: 'Fixes', count: p.fixCount || undefined },
    { key: 'runners', label: 'Runners', count: p.runnerCount || undefined },
    { key: 'plugins', label: 'Plugins', dot: p.authNeeded },
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
        'mb-0.5 flex w-full items-center justify-between rounded-xl py-2 pr-3 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300/60',
        indent ? 'pl-7' : 'px-3',
        active === n.key ? 'bg-amber-300/12 text-amber-200' : 'text-white/55 hover:bg-white/[0.05] hover:text-white'
      )}
    >
      <span>{n.label}</span>
      {n.key === 'library' && p.loading ? (
        <span aria-label="counting games" className="h-4 w-7 animate-pulse rounded-full bg-white/15 motion-reduce:animate-none" />
      ) : (
        n.count != null && <Badge variant="neutral" className="h-5 rounded-full tracking-normal tabular-nums">{n.count}</Badge>
      )}
      {n.dot && <span className="h-1.5 w-1.5 rounded-full bg-amber-300" aria-label="needs attention" />}
    </button>
  );

  return (
    <aside className="flex h-full w-56 flex-none flex-col border-r border-white/[0.06] bg-black/25 p-4">
      <div className="mb-5 flex items-center gap-2.5 px-1.5">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[15px] font-black text-[#130f0a]">P</span>
        <div>
          <div className="text-[15px] font-extrabold tracking-tight">Platypus</div>
          <div className="font-mono text-[11px] uppercase tracking-[0.16em] text-white/40">game hub</div>
        </div>
      </div>

      <label className="relative mb-4 block flex-none">
        <span className="sr-only">Search library</span>
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-white/35" />
        <Input
          type="search"
          value={p.search}
          // Typing never navigates (FE-12): the view only changes on Enter.
          onChange={(e) => p.onSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && p.view.name !== 'library') p.onNavigate({ name: 'library' });
            if (e.key === 'Escape') p.onSearch('');
          }}
          placeholder="Search…"
          className="pl-8.5 font-sans"
        />
      </label>

      {/* The nav and sources scroll when the window is short; the search above and the footer below stay put. */}
      <div className="min-h-0 flex-1 overflow-y-auto pr-1 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
        <nav aria-label="Main">
          {main.map((n) => navButton(n))}

          <button
            type="button"
            onClick={toggleTools}
            aria-expanded={toolsOpen}
            aria-controls="sidebar-tools"
            className={cx(
              'mt-2 flex w-full items-center justify-between rounded-xl px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.18em] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300/60',
              toolsHasActive && !toolsOpen ? 'text-amber-200' : 'text-white/40 hover:text-white/70'
            )}
          >
            <span>Tools</span>
            <span className="flex items-center gap-2">
              {!toolsOpen && toolsAttention > 0 && <Badge variant="neutral" className="h-4 rounded-full px-1.5 tracking-normal tabular-nums">{toolsAttention}</Badge>}
              {!toolsOpen && toolsDot && <span className="h-1.5 w-1.5 rounded-full bg-amber-300" aria-label="needs attention" />}
              <ChevronDown className={cx('h-3.5 w-3.5 transition-transform motion-reduce:transition-none', toolsOpen ? '' : '-rotate-90')} aria-hidden />
            </span>
          </button>
          <div id="sidebar-tools" hidden={!toolsOpen}>
            {tools.map((n) => navButton(n, true))}
          </div>
        </nav>

        <div className="px-2 pb-1.5 pt-4 font-mono text-[11px] uppercase tracking-[0.18em] text-white/40">sources</div>
        {[
          ...SOURCES,
          // Stores that only plugins provide, then sideloaded games.
          ...Object.keys(p.storeCounts)
            .filter((k) => k !== 'custom' && !SOURCES.some((s) => s.key === k))
            .map((k) => ({ key: k, label: storeLabel(k) })),
          ...(p.storeCounts.custom ? [{ key: 'custom', label: 'Sideloaded' }] : []),
        ].map((s) => {
          const count = p.storeCounts[s.key] ?? 0;
          return (
            <button
              key={s.key}
              onClick={() => p.onPickSource(s.key)}
              className="mb-0.5 flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-[12.5px] text-white/55 transition hover:bg-white/[0.04] hover:text-white"
            >
              <span className="flex items-center gap-2">
                <span className={cx('h-1.5 w-1.5 rounded-full', count ? 'bg-amber-300/60' : 'bg-white/15')} />
                {s.label}
              </span>
              <span className="font-mono text-[11px] tabular-nums text-white/40">
                {p.loading ? <span aria-label="counting games" className="inline-block h-2.5 w-5 animate-pulse rounded bg-white/15 motion-reduce:animate-none" /> : count || '—'}
              </span>
            </button>
          );
        })}
      </div>

      {/* Pinned footer: fixed content, never grows. The friends block is the only part that changes size. */}
      <div className="mt-auto flex flex-none flex-col gap-2 pt-4">
        {p.canRedeem && (
          <Button variant="neutral" onClick={p.onRedeem} className="w-full">
            <Ticket className="h-3.5 w-3.5" /> Redeem a code
          </Button>
        )}
        <button
          type="button"
          onClick={() => p.onNavigate({ name: 'settings' })}
          aria-current={active === 'settings' ? 'page' : undefined}
          className={cx(
            'flex w-full items-center gap-2 rounded-xl px-3 py-2 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300/60',
            active === 'settings' ? 'bg-amber-300/12 text-amber-200' : 'text-white/55 hover:bg-white/[0.05] hover:text-white'
          )}
        >
          <Settings className="h-3.5 w-3.5" aria-hidden />
          Settings
        </button>
        <FriendsSummary views={p.friends} labels={p.friendLabels} onOpen={p.onOpenFriends} unread={p.friendsUnread} />
        {p.sessions.map((e) => {
          const running = e.state === 'running';
          const label = running ? 'playing' : e.state === 'stopping' ? 'stopping' : 'starting';
          return (
            <div
              key={e.game_id}
              title={e.message}
              className={cx('flex items-center gap-2 rounded-xl px-3 py-2 ring-1', running ? 'bg-emerald-400/[0.08] ring-emerald-300/25' : 'bg-amber-300/[0.07] ring-amber-300/25')}
            >
              <span className={cx('h-2 w-2 flex-none rounded-full', running ? 'bg-emerald-400' : 'animate-pulse bg-amber-300')} />
              <span className="min-w-0 flex-1">
                <span className={cx('block font-mono text-[11px] uppercase tracking-wider', running ? 'text-emerald-200' : 'text-amber-200')}>
                  {label}
                </span>
                <span className="block truncate text-[11.5px] font-semibold text-white/85">{e.title}</span>
              </span>
              {e.state !== 'stopping' && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => p.onStop(e.game_id)}
                      aria-label={`Stop ${e.title}`}
                      className="size-7 flex-none text-white/45 hover:text-red-200"
                    >
                      <Square className="h-3 w-3 fill-current" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Stop</TooltipContent>
                </Tooltip>
              )}
            </div>
          );
        })}
        <div className="px-1.5 font-mono text-[12px] text-white/55">
          platypus v0.2{p.lastScan ? ` · scan ${clock(p.lastScan)}` : ''}
        </div>
      </div>
    </aside>
  );
}
