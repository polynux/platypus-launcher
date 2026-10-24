// The app shell as a view (issue #104, ADR 0002 answer 4). The shell has one
// region per shell.navigation value (catalog shell.sidebar, shell.topbar):
// the sidebar's parts (inventory §1.2) are blocks each region arranges. The
// sidebar region is the Sidebar itself, unchanged; the topbar region lays the
// same blocks out in two rows, with the Tools items in a menu.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, Search, Settings, Square, Ticket } from 'lucide-react';
import { cx, clock } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { storeLabel } from '../lib/games';
import { FriendsSummary } from './FriendsSummary';
import { PluginLogo } from './PluginLogo';
import { Sidebar } from './Sidebar';
import { useVariant } from '../lib/theme';
import type { Collection, SessionEvent, SocialView, StoreType, View } from '../types';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'storage' | 'settings';

type ShellProps = Parameters<typeof Sidebar>[0];

const SOURCES: { key: StoreType; label: string }[] = [
  { key: 'steam', label: 'Steam' },
  { key: 'gog', label: 'GOG' },
  { key: 'epic', label: 'Epic' },
];

type NavItem = { key: NavKey; label: string; count?: number; dot?: boolean };

export function Shell(p: ShellProps) {
  const navigation = useVariant('shell.navigation');
  if (navigation === 'topbar') return <Topbar {...p} />;
  return <Sidebar {...p} />;
}

function Topbar(p: ShellProps) {
  const { t } = useTranslation(['nav', 'common', 'collections']);
  const active: NavKey | 'store' = p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;
  const storeEntries = p.stores ?? [];
  const [toolsOpen, setToolsOpen] = useState(false);

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
  // Opening a tool's view shows its item; the menu opens with it.
  const toolsHasActive = tools.some((item) => item.key === active);
  const toolsAttention = tools.reduce((n, item) => n + (item.count ?? 0), 0);
  const toolsDot = tools.some((item) => item.dot);
  useEffect(() => {
    if (toolsHasActive) setToolsOpen(true);
  }, [toolsHasActive]);

  const navButton = (n: NavItem) => (
    <button
      key={n.key}
      onClick={() => { setToolsOpen(false); p.onNavigate({ name: n.key } as View); }}
      aria-current={active === n.key ? 'page' : undefined}
      className={cx(
        'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
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
    <header data-region="topbar" className="flex-none border-b border-hairline bg-black/25">
      <div className="flex items-center gap-2 px-4 pt-2">
        <span className="grid h-8 w-8 flex-none place-items-center rounded-lg bg-gradient-to-br from-accent-strong to-rose-500 text-[14px] font-black text-on-accent">P</span>
        {/* shell.nav */}
        <nav aria-label={t('nav:main')} className="flex min-w-0 items-center gap-0.5 overflow-x-auto">
          {main.map((n) => navButton(n))}
          {/* shell.tools: a menu in topbar mode, so Plugins stays one click away */}
          <div className="relative flex-none">
            <button
              type="button"
              onClick={() => setToolsOpen((open) => !open)}
              onKeyDown={(e) => { if (e.key === 'Escape') setToolsOpen(false); }}
              aria-expanded={toolsOpen}
              aria-controls="topbar-tools"
              className={cx(
                'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 font-mono text-micro uppercase tracking-[0.14em] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
                toolsHasActive && !toolsOpen ? 'text-amber-200' : 'text-text-muted hover:text-text-body'
              )}
            >
              <span>{t('nav:tools')}</span>
              {!toolsOpen && toolsAttention > 0 && <Badge variant="neutral" className="h-4 rounded-full px-1.5 tracking-normal tabular-nums">{toolsAttention}</Badge>}
              {!toolsOpen && toolsDot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
              <ChevronDown className={cx('h-3.5 w-3.5 transition-transform motion-reduce:transition-none', toolsOpen ? '' : '-rotate-90')} aria-hidden />
            </button>
            {toolsOpen && (
              <div id="topbar-tools" onKeyDown={(e) => { if (e.key === 'Escape') setToolsOpen(false); }} className="absolute left-0 top-full z-50 mt-1 flex min-w-44 flex-col gap-0.5 rounded-xl border border-hairline bg-raised p-1.5 shadow-xl">
                {tools.map((n) => navButton(n))}
              </div>
            )}
          </div>
        </nav>
        <label className="relative ml-auto block w-52 flex-none">
          <span className="sr-only">{t('nav:searchLibrary')}</span>
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-muted" />
          <Input
            type="search"
            value={p.search}
            // Typing never navigates: the view only changes on Enter.
            onChange={(e) => p.onSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && p.view.name !== 'library') p.onNavigate({ name: 'library' });
              if (e.key === 'Escape') p.onSearch('');
            }}
            placeholder={t('nav:searchPlaceholder')}
            className="pl-8.5 font-sans"
          />
        </label>
        {/* shell.footer: the Settings entry stays put in both variants */}
        {p.canRedeem && (
          <Button variant="neutral" onClick={p.onRedeem} className="flex-none">
            <Ticket className="h-3.5 w-3.5" /> {t('nav:redeemCode')}
          </Button>
        )}
        <button
          type="button"
          onClick={() => p.onSettingsOpen ? p.onSettingsOpen() : p.onNavigate({ name: 'settings' })}
          aria-current={active === 'settings' ? 'page' : undefined}
          aria-label={t('nav:settings')}
          title={t('nav:settings')}
          className={cx(
            'grid h-8 w-8 flex-none place-items-center rounded-lg transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
            active === 'settings' ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface-strong hover:text-white'
          )}
        >
          <Settings className="h-4 w-4" aria-hidden />
        </button>
      </div>
      <div className="flex items-center gap-4 overflow-x-auto px-4 py-1.5 [scrollbar-width:thin] [scrollbar-color:rgba(255,255,255,.18)_transparent]">
        {/* shell.collections */}
        {(p.collections ?? []).filter((c) => c.pinned).length > 0 && (
          <div className="flex flex-none items-center gap-1">
            <span className="font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('collections:title')}</span>
            {(p.collections ?? []).filter((c: Collection) => c.pinned).map((c) => {
              const current = p.view.name === 'library' && p.view.collection === c.id;
              return (
                <button
                  key={c.id}
                  onClick={() => p.onOpenCollection?.(c.id)}
                  aria-current={current ? 'page' : undefined}
                  className={cx(
                    'whitespace-nowrap rounded-lg px-2 py-1 text-[12px] transition',
                    current ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface hover:text-white'
                  )}
                >
                  {c.builtin ? t(`collections:builtin.${c.id}`) : c.name}
                </button>
              );
            })}
            {p.onManageCollections && (
              <button type="button" onClick={p.onManageCollections} className="whitespace-nowrap font-mono text-micro uppercase tracking-[0.14em] text-text-muted transition hover:text-white/80">
                {t('collections:manage')}
              </button>
            )}
          </div>
        )}
        {/* shell.sources */}
        <div className="flex flex-none items-center gap-1">
          <span className="font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('nav:sources')}</span>
          {[
            ...SOURCES,
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
                className="flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2 py-1 text-[12px] text-text-body transition hover:bg-surface hover:text-white"
              >
                <PluginLogo src={p.sourceLogos?.[s.key]} name={s.label} size="xs" />
                {s.label}
                <span className="font-mono text-hint tabular-nums text-text-muted">{count || '—'}</span>
              </button>
            );
          })}
        </div>
        {/* shell.store */}
        {storeEntries.length > 0 && (
          <div className="flex flex-none items-center gap-1">
            <span className="font-mono text-micro uppercase tracking-[0.18em] text-text-muted">{t('nav:store')}</span>
            {storeEntries.map((s) => (
              <button
                key={s.id}
                onClick={() => p.onOpenStore?.(s.id)}
                aria-current={p.activeStore === s.id ? 'page' : undefined}
                className={cx(
                  'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2 py-1 text-[12px] transition',
                  p.activeStore === s.id ? 'bg-accent-strong/12 text-amber-200' : 'text-text-body hover:bg-surface hover:text-white'
                )}
              >
                <PluginLogo src={s.icon_url} name={s.name} size="xs" />
                {s.name}
              </button>
            ))}
          </div>
        )}
        <div className="ml-auto flex flex-none items-center gap-2">
          <FriendsSummary views={p.friends} labels={p.friendLabels} onOpen={p.onOpenFriends} unread={p.friendsUnread} />
          {p.sessions.map((e: SessionEvent & { title: string }) => {
            const running = e.state === 'running';
            const label = running ? t('nav:playing') : e.state === 'stopping' ? t('nav:stopping') : t('nav:starting');
            return (
              <span
                key={e.game_id}
                title={e.message}
                className={cx('flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2 py-1 ring-1', running ? 'bg-emerald-400/[0.08] ring-emerald-300/25' : 'bg-accent-strong/[0.07] ring-accent-strong/25')}
              >
                <span className={cx('h-2 w-2 rounded-full', running ? 'bg-emerald-400' : 'animate-pulse bg-accent-strong')} />
                <span className={cx('font-mono text-micro uppercase tracking-wider', running ? 'text-emerald-200' : 'text-amber-200')}>{label}</span>
                <span className="max-w-32 truncate text-hint font-semibold text-white/85">{e.title}</span>
                {e.state !== 'stopping' && (
                  <Button variant="ghost" size="icon" onClick={() => p.onStop(e.game_id)} aria-label={t('nav:stopGame', { title: e.title })} className="size-5 flex-none text-text-muted hover:text-red-200">
                    <Square className="h-2.5 w-2.5 fill-current" />
                  </Button>
                )}
              </span>
            );
          })}
          <span className="whitespace-nowrap font-mono text-[12px] text-text-muted">platypus v0.2{p.lastScan ? ` · scan ${clock(p.lastScan)}` : ''}</span>
        </div>
      </div>
    </header>
  );
}
