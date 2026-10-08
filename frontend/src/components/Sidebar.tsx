import React from 'react';
import { Search, Square } from 'lucide-react';
import { cx, clock } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { Tooltip, TooltipContent, TooltipTrigger } from './ds/tooltip';
import { storeLabel } from '../lib/games';
import type { SessionEvent, StoreType, View } from '../types';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'settings';

interface SidebarProps {
  view: View;
  onNavigate: (v: View) => void;
  search: string;
  onSearch: (q: string) => void;
  storeCounts: Record<string, number>;
  onPickSource: (s: StoreType) => void;
  gameCount: number;
  downloadCount: number;
  authNeeded: boolean; // a plugin waits for approval or login
  fixCount: number;
  runnerCount: number;
  lastScan: number | null;
  sessions: (SessionEvent & { title: string })[];
  onStop: (gameId: string) => void;
}

const SOURCES: { key: StoreType; label: string }[] = [
  { key: 'steam', label: 'Steam' },
  { key: 'gog', label: 'GOG' },
  { key: 'epic', label: 'Epic' },
];

export function Sidebar(p: SidebarProps) {
  const active: NavKey = p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;

  const nav: { key: NavKey; label: string; count?: number; dot?: boolean }[] = [
    { key: 'dashboard', label: 'Dashboard' },
    { key: 'library', label: 'Library', count: p.gameCount },
    { key: 'downloads', label: 'Downloads', count: p.downloadCount || undefined },
    { key: 'activity', label: 'Activity' },
    { key: 'fixes', label: 'Fixes', count: p.fixCount || undefined },
    { key: 'runners', label: 'Runners', count: p.runnerCount || undefined },
    { key: 'plugins', label: 'Plugins', dot: p.authNeeded },
    { key: 'settings', label: 'Settings' },
  ];

  return (
    <aside className="flex h-full w-56 flex-none flex-col border-r border-white/[0.06] bg-black/25 p-4">
      <div className="mb-5 flex items-center gap-2.5 px-1.5">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[15px] font-black text-[#130f0a]">H</span>
        <div>
          <div className="text-[15px] font-extrabold tracking-tight">Hydra</div>
          <div className="font-mono text-[11px] uppercase tracking-[0.16em] text-white/40">game hub</div>
        </div>
      </div>

      <label className="relative mb-4 block">
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

      <nav aria-label="Main">
        {nav.map((n) => (
          <button
            key={n.key}
            onClick={() => p.onNavigate({ name: n.key } as View)}
            aria-current={active === n.key ? 'page' : undefined}
            className={cx(
              'mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2 text-[12.5px] font-semibold transition',
              active === n.key ? 'bg-amber-300/12 text-amber-200' : 'text-white/55 hover:bg-white/[0.05] hover:text-white'
            )}
          >
            <span>{n.label}</span>
            {n.count != null && <Badge variant="neutral" className="h-5 rounded-full tracking-normal tabular-nums">{n.count}</Badge>}
            {n.dot && <span className="h-1.5 w-1.5 rounded-full bg-amber-300" aria-label="needs attention" />}
          </button>
        ))}
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
            <span className="font-mono text-[11px] tabular-nums text-white/40">{count || '—'}</span>
          </button>
        );
      })}

      {/* Pinned footer: fixed content, never grows (friends panel omitted until presence is real). */}
      <div className="mt-auto flex flex-none flex-col gap-2 pt-4">
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
          hydra v0.2{p.lastScan ? ` · scan ${clock(p.lastScan)}` : ''}
        </div>
      </div>
    </aside>
  );
}
