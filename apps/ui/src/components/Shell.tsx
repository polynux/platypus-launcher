// The app shell as a view (issue #104, ADR 0002 answer 4). The shell has one
// region per shell.navigation value (catalog shell.sidebar, shell.topbar,
// shell.sourcebar for atlas in #125): the sidebar's parts (inventory §1.2)
// are blocks each region arranges. The sidebar region is the Sidebar itself,
// unchanged; the topbar region lays the same blocks out in two rows, with the
// Tools items in a menu; the sourcebar region is the /21 chrome (D21.jsx):
// a 44px top bar, a 48px source bar and a 28px activity footer.
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ArrowDownToLine, ChevronDown, CircleAlert, HardDrive, Layers, LibraryBig, Plus, Search, Settings, ShoppingBag, Square, Ticket, WifiOff } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ds/dropdown-menu';
import { cx, clock, APP_VERSION } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { storeLabel } from '../lib/games';
import { FriendsSummary } from './FriendsSummary';
import { PluginLogo } from './PluginLogo';
import { Sidebar } from './Sidebar';
import { useVariant } from '../lib/theme';
import type { GameTone } from '../lib/gameTone';
import { GameToneSwitch } from './GameToneSwitch';
import { openCommandPalette } from './CommandPalette';
import type { Collection, Job, SessionEvent, SocialView, StoreType, View } from '../types';
import type { Ownership } from './LibraryView';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'storage' | 'settings';

type SidebarProps = Parameters<typeof Sidebar>[0];

interface ShellProps extends SidebarProps {
  /** Library ownership for the sourcebar All/Installed tabs. */
  ownership?: Ownership;
  onOwnership?: (o: Ownership) => void;
  /** Every game the library holds (for the All count). Defaults to gameCount. */
  ownedCount?: number;
  /** Selected stores for the sourcebar (empty = all). Single-select writes through to onPickSource. */
  storeSelection?: StoreType[];
  onSelectStores?: (s: StoreType[]) => void;
  /** Queued or running jobs for the sourcebar Downloads tab and the activity footer. */
  jobs?: Job[];
  /** When each session started (ms epoch), for the footer's elapsed time. */
  sessionStartedAt?: Record<string, number>;
  /** The game tone App applies; the sourcebar shows its switch (#123). */
  gameTone?: GameTone;
}

const SOURCES: { key: StoreType; label: string }[] = [
  { key: 'steam', label: 'Steam' },
  { key: 'gog', label: 'GOG' },
  { key: 'epic', label: 'Epic' },
];

type NavItem = { key: NavKey; label: string; count?: number; dot?: boolean };

export function Shell(p: ShellProps) {
  const navigation = useVariant('shell.navigation');
  if (navigation === 'sourcebar') return <SourcebarChrome {...p} />;
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
  // A tool view marks the trigger, but the menu stays closed until opened:
  // unlike the sidebar's disclosure group, a floating menu never pops up on
  // page load or steals focus on navigation. Plugins stays one click away.
  const toolsHasActive = tools.some((item) => item.key === active);
  const toolsAttention = tools.reduce((n, item) => n + (item.count ?? 0), 0);
  const toolsDot = tools.some((item) => item.dot);

  const navButton = (n: NavItem) => (
    <button
      key={n.key}
      onClick={() => p.onNavigate({ name: n.key } as View)}
      aria-current={active === n.key ? 'page' : undefined}
      className={cx(
        'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
        active === n.key ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-white'
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
        <span className="grid h-8 w-8 flex-none place-items-center rounded-lg bg-gradient-to-br from-accent-strong to-danger text-[14px] font-black text-on-accent">P</span>
        {/* shell.nav */}
        <nav aria-label={t('nav:main')} className="flex min-w-0 items-center gap-0.5 overflow-x-auto">
          {main.map((n) => navButton(n))}
          {/* shell.tools: a menu in topbar mode, so Plugins stays one click away.
              The panel portals outside the nav's overflow container (D1); the
              menu owns focus, arrows and Escape (D2). */}
          <DropdownMenu open={toolsOpen} onOpenChange={setToolsOpen} modal={false}>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className={cx(
                  'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 font-mono text-micro uppercase tracking-[0.14em] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
                  toolsHasActive && !toolsOpen ? 'text-accent-strong' : 'text-text-muted hover:text-text-body'
                )}
              >
                <span>{t('nav:tools')}</span>
                {!toolsOpen && toolsAttention > 0 && <Badge variant="neutral" className="h-4 rounded-full px-1.5 tracking-normal tabular-nums">{toolsAttention}</Badge>}
                {!toolsOpen && toolsDot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
                <ChevronDown className={cx('h-3.5 w-3.5 transition-transform motion-reduce:transition-none', toolsOpen ? '' : '-rotate-90')} aria-hidden />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              side="bottom"
              align="start"
            >
                {tools.map((n) => (
                  <DropdownMenuItem key={n.key} asChild onSelect={() => p.onNavigate({ name: n.key } as View)}>
                    <button
                      aria-current={active === n.key ? 'page' : undefined}
                      className={cx(
                        'flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-[12.5px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/60',
                        active === n.key ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-white'
                      )}
                    >
                      <span>{n.label}</span>
                      {n.count != null && <Badge variant="neutral" className="h-5 rounded-full tracking-normal tabular-nums">{n.count}</Badge>}
                      {n.dot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
                    </button>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
          </DropdownMenu>
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
            active === 'settings' ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-white'
          )}
        >
          <Settings className="h-4 w-4" aria-hidden />
        </button>
      </div>
      {/* The context strip wraps instead of scrolling (D3): at 1280 px every
          block stays fully visible and no scrollbar shows. */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-1.5">
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
                    current ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface hover:text-white'
                  )}
                >
                  {c.builtin ? t(`collections:builtin.${c.id}`) : c.name}
                </button>
              );
            })}
            {p.onManageCollections && (
              <button type="button" onClick={p.onManageCollections} className="whitespace-nowrap font-mono text-micro uppercase tracking-[0.14em] text-text-muted transition hover:text-text">
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
                  p.activeStore === s.id ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface hover:text-white'
                )}
              >
                <PluginLogo src={s.icon_url} name={s.name} size="xs" />
                {s.name}
              </button>
            ))}
          </div>
        )}
        <div className="ml-auto flex min-w-0 flex-none items-center gap-2">
          <div className="min-w-0 max-w-72 [&>button]:w-auto [&>button]:max-w-72">
            <FriendsSummary views={p.friends} labels={p.friendLabels} onOpen={p.onOpenFriends} unread={p.friendsUnread} />
          </div>
          {p.sessions.map((e: SessionEvent & { title: string }) => {
            const running = e.state === 'running';
            const label = running ? t('nav:playing') : e.state === 'stopping' ? t('nav:stopping') : t('nav:starting');
            return (
              <span
                key={e.game_id}
                title={e.message}
                className={cx('flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2 py-1 ring-1', running ? 'bg-ok/[0.08] ring-ok/25' : 'bg-accent-strong/[0.07] ring-accent-strong/25')}
              >
                <span className={cx('h-2 w-2 rounded-full', running ? 'bg-ok' : 'animate-pulse bg-accent-strong')} />
                <span className={cx('font-mono text-micro uppercase tracking-wider', running ? 'text-ok' : 'text-accent-strong')}>{label}</span>
                <span className="max-w-32 truncate text-hint font-semibold text-text">{e.title}</span>
                {e.state !== 'stopping' && (
                  <Button variant="ghost" size="icon" onClick={() => p.onStop(e.game_id)} aria-label={t('nav:stopGame', { title: e.title })} className="size-5 flex-none text-text-muted hover:text-danger">
                    <Square className="h-2.5 w-2.5 fill-current" />
                  </Button>
                )}
              </span>
            );
          })}
          <span className="whitespace-nowrap font-mono text-[12px] text-text-muted">{APP_VERSION}{p.lastScan ? ` · scan ${clock(p.lastScan)}` : ''}</span>
        </div>
      </div>
    </header>
  );
}

// The /21 Atlas chrome (issue #125, D21.jsx `.tb`, `.sources`, `.tele`). The
// grid rows are 44px, 48px, 1fr and 28px: the top bar and source bar render
// here, the 1fr body is App's main, and the 28px footer renders after it
// (SourceFooter). Library and game views stay as they are inside the new
// shell (#126, #127 come later).
function SourcebarChrome(p: ShellProps) {
  const { t } = useTranslation(['nav', 'common']);
  const ownership = p.ownership ?? 'installed';
  const selection = p.storeSelection ?? [];
  const selectStores = p.onSelectStores ?? (() => {});
  const ownedCount = p.ownedCount ?? p.gameCount;
  const jobs = p.jobs ?? [];
  const downloadCount = p.downloadCount;
  const active: NavKey | 'store' =
    p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;
  const goLibrary = (o: Ownership) => {
    p.onOwnership?.(o);
    selectStores([]);
    if (p.view.name !== 'library') p.onNavigate({ name: 'library' });
  };
  const pickStore = (s: StoreType, shift: boolean) => {
    if (shift) {
      const next = selection.includes(s) ? selection.filter((x) => x !== s) : [...selection, s];
      selectStores(next);
    } else {
      selectStores(selection.length === 1 && selection[0] === s ? [] : [s]);
    }
    if (p.view.name !== 'library') p.onNavigate({ name: 'library' });
  };

  const viewTabs: { id: 'all' | 'installed' | 'downloads' | 'attention'; label: string; count?: number; warn?: boolean; progress?: number }[] = [
    { id: 'all', label: t('nav:sourcebar.all'), count: ownedCount },
    { id: 'installed', label: t('nav:sourcebar.installed'), count: p.gameCount },
    {
      id: 'downloads',
      label: t('nav:sourcebar.downloads'),
      count: downloadCount,
      progress: jobs.length > 0 && jobs[0].percent >= 0 ? jobs[0].percent / 100 : undefined,
    },
    { id: 'attention', label: t('nav:sourcebar.attention'), count: p.fixCount, warn: p.fixCount > 0 },
  ];
  const isViewOn = (id: string) => {
    // A collection is its own scope: the All/Installed tabs only look active
    // on the unfiltered library.
    const collection = p.view.name === 'library' ? p.view.collection : undefined;
    if (id === 'all') return p.view.name === 'library' && !collection && ownership === 'owned' && selection.length === 0;
    if (id === 'installed') return p.view.name === 'library' && !collection && ownership === 'installed' && selection.length === 0;
    if (id === 'downloads') return p.view.name === 'downloads';
    return p.view.name === 'fixes';
  };
  const onViewTab = (id: string) => {
    if (id === 'all') goLibrary('owned');
    else if (id === 'installed') goLibrary('installed');
    else if (id === 'downloads') p.onNavigate({ name: 'downloads' });
    else p.onNavigate({ name: 'fixes' });
  };
  const viewIcon = (id: string) => {
    if (id === 'all') return LibraryBig;
    if (id === 'installed') return HardDrive;
    if (id === 'downloads') return ArrowDownToLine;
    return CircleAlert;
  };
  // Stores with games, like the sidebar's Sources rows: the known storefronts
  // first, then anything a plugin reported, then sideloaded games.
  const storeTabs = [
    ...SOURCES.filter((s) => (p.storeCounts[s.key] ?? 0) > 0),
    ...Object.keys(p.storeCounts)
      .filter((k) => k !== 'custom' && !SOURCES.some((s) => s.key === k) && (p.storeCounts[k] ?? 0) > 0)
      .map((k) => ({ key: k as StoreType, label: storeLabel(k) })),
    ...((p.storeCounts.custom ?? 0) > 0 ? [{ key: 'custom' as StoreType, label: t('nav:sideloaded') }] : []),
  ];

  return (
    <div data-region="sourcebar" className="flex-none text-[13.5px]">
      {/* Top bar (44px): the brand mark in the accent, a centred command button,
          and the game colour slot (#123) plus the places menu at the right. */}
      <header className="grid h-[44px] grid-cols-[1fr_minmax(280px,520px)_1fr] items-center bg-ground px-3.5">
        <div className="flex items-center gap-2.5 font-bold text-[14.5px] text-text">
          <span aria-hidden className="grid size-6 place-items-center rounded-xl bg-accent text-[13px] font-extrabold text-on-accent">P</span>
          <span>Platypus</span>
        </div>
        <button
          type="button"
          onClick={openCommandPalette}
          className="flex h-[30px] items-center gap-2.5 rounded-xl bg-surface px-1.5 pl-3 text-left text-text-faint transition hover:text-text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <Search className="h-3.5 w-3.5" aria-hidden />
          <span className="flex-1 text-left text-[13px]">{t('nav:sourcebar.searchOrCommand')}</span>
          <kbd className="rounded-md border border-hairline px-1.5 font-mono text-hint text-text-faint">Ctrl K</kbd>
        </button>
        <div className="flex items-center justify-end gap-2">
          {/* Game colour switch (#123): hidden unless the theme supports "whole". */}
          {p.gameTone && <GameToneSwitch tone={p.gameTone} />}
          <StoresMenu p={p} />
          <PlacesMenu p={p} active={active} />
        </div>
      </header>
      {/* Source bar (48px): library views with counts, a separator, then one
          tab per store with its game count. Shift-click combines stores. */}
      <nav aria-label={t('nav:main')} className="flex h-12 items-center gap-1 border-y border-hairline bg-[color-mix(in_srgb,var(--color-ground)_60%,var(--color-raised))] px-3">
        {viewTabs.map((v) => {
          const Icon = viewIcon(v.id);
          const on = isViewOn(v.id);
          return (
            <button
              key={v.id}
              type="button"
              aria-pressed={on}
              onClick={() => onViewTab(v.id)}
              className={cx(
                'relative inline-flex h-[34px] flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
              )}
            >
              <Icon className="h-[15px] w-[15px]" aria-hidden />
              {v.label}
              {v.progress != null ? (
                <span
                  role="progressbar"
                  aria-valuenow={Math.round(v.progress * 100)}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-label={`${v.label} ${Math.round(v.progress * 100)}%`}
                  className="h-1 w-7 overflow-hidden rounded-full bg-hairline"
                >
                  <span className="block h-full rounded-full bg-accent" style={{ width: `${Math.round(v.progress * 100)}%` }} />
                </span>
              ) : (
                <span className={cx('font-medium tabular-nums text-[12px]', v.warn ? 'rounded-lg bg-danger px-1.5 font-bold text-on-accent' : 'text-text-faint')}>{v.count ?? 0}</span>
              )}
              {on && <span aria-hidden className="absolute inset-x-3 -bottom-[9px] h-0.5 rounded-full bg-accent" />}
            </button>
          );
        })}
        <span aria-hidden className="mx-2 h-[22px] w-px flex-none bg-hairline" />
        <div
          className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto pr-6 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          style={{ maskImage: 'linear-gradient(to right, black calc(100% - 24px), transparent)' }}
        >
          {storeTabs.map((s) => {
            const on = selection.includes(s.key);
            return (
              <button
                key={s.key}
                type="button"
                aria-pressed={on}
                title={t('nav:sourcebar.shiftHint')}
                onClick={(e) => pickStore(s.key, e.shiftKey)}
                className={cx(
                  'relative inline-flex h-[34px] flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
                )}
              >
                <PluginLogo src={p.sourceLogos?.[s.key]} name={s.label} size="xs" />
                {s.label}
                <span className="font-medium tabular-nums text-[12px] text-text-faint">{p.storeCounts[s.key] ?? 0}</span>
                {on && <span aria-hidden className="absolute inset-x-3 -bottom-[9px] h-0.5 rounded-full bg-accent" />}
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => p.onNavigate({ name: 'plugins' })}
            className="inline-flex h-[34px] flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-medium text-text-faint transition hover:text-text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <Plus className="h-3.5 w-3.5" aria-hidden />
            {t('nav:sourcebar.addStore')}
          </button>
        </div>
        <CollectionsMenu p={p} />
      </nav>
    </div>
  );
}

// Atlas parity (issue #132): the storefront plugins and redeem live in the
// top bar between the game-tone switch and the places menu, like the
// sidebar's Store section and redeem entry. Hidden when there is nothing.
function StoresMenu({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav']);
  const [open, setOpen] = useState(false);
  const stores = p.stores ?? [];
  if (stores.length === 0 && !p.canRedeem) return null;
  return (
    <DropdownMenu open={open} onOpenChange={setOpen} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('nav:sourcebar.stores')}
          className="grid size-7 place-items-center rounded-lg text-text-muted transition hover:bg-surface-strong hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <ShoppingBag className="h-4 w-4" aria-hidden />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end">
        {stores.map((s) => (
          <DropdownMenuItem key={s.id} asChild onSelect={() => p.onOpenStore?.(s.id)}>
            <button aria-current={p.activeStore === s.id ? 'page' : undefined} className={cx('w-full', p.activeStore === s.id && 'text-accent-strong')}>
              <PluginLogo src={s.icon_url} name={s.name} size="xs" />
              <span>{s.name}</span>
            </button>
          </DropdownMenuItem>
        ))}
        {stores.length > 0 && p.canRedeem && <DropdownMenuSeparator />}
        {p.canRedeem && (
          <DropdownMenuItem asChild onSelect={() => p.onRedeem?.()}>
            <button>
              <Ticket className="h-3.5 w-3.5" aria-hidden />
              <span>{t('nav:sourcebar.redeem')}</span>
            </button>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// Atlas parity (issue #132): collections pinned at the right edge of the
// source bar, outside the scrolling store strip, in the same 34px tab style
// with the accent underline when a collection is active.
function CollectionsMenu({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav', 'collections']);
  const [open, setOpen] = useState(false);
  if (!p.onManageCollections && (p.collections ?? []).length === 0) return null;
  const activeId = p.view.name === 'library' ? p.view.collection : undefined;
  const active = (p.collections ?? []).find((c) => c.id === activeId);
  const label = active ? (active.builtin ? t(`collections:builtin.${active.id}`) : active.name) : t('collections:title');
  return (
    <DropdownMenu open={open} onOpenChange={setOpen} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-pressed={active != null}
          className={cx(
            'relative inline-flex h-[34px] flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
            active ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
          )}
        >
          <Layers className="h-[15px] w-[15px]" aria-hidden />
          <span className="max-w-40 truncate">{label}</span>
          {active && <span aria-hidden className="absolute inset-x-3 -bottom-[9px] h-0.5 rounded-full bg-accent" />}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end">
        <DropdownMenuItem asChild onSelect={() => p.onOpenCollection?.(undefined)}>
          <button>
            <span>{t('collections:allGames')}</span>
          </button>
        </DropdownMenuItem>
        {(p.collections ?? []).map((c) => {
          const on = c.id === activeId;
          return (
            <DropdownMenuItem key={c.id} asChild onSelect={() => p.onOpenCollection?.(c.id)}>
              <button aria-current={on ? 'page' : undefined} className={cx('w-full', on && 'text-accent-strong')}>
                <span className="min-w-0 flex-1 truncate text-left">{c.builtin ? t(`collections:builtin.${c.id}`) : c.name}</span>
                <span className="font-medium tabular-nums text-[12px] text-text-faint">{p.collectionCounts?.[c.id] ?? 0}</span>
              </button>
            </DropdownMenuItem>
          );
        })}
        {p.onManageCollections && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild onSelect={() => p.onManageCollections?.()}>
              <button>
                <span>{t('collections:manage')}</span>
              </button>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function PlacesMenu({ p, active }: { p: ShellProps; active: NavKey | 'store' }) {
  const { t } = useTranslation(['nav', 'chat']);
  const [open, setOpen] = useState(false);
  // Atlas parity (issue #132): the Friends entry carries the unread count and
  // the trigger shows a dot while there are unread messages; the duplicate
  // Chat entry opened the same modal, so it is gone. The auth dot mirrors the
  // sidebar/tools attention when a plugin needs approval or login.
  const friendsLabel = p.friendsUnread > 0 ? `${t('nav:sourcebar.friends')} (${p.friendsUnread})` : t('nav:sourcebar.friends');
  const items: { key: string; label: string; onSelect: () => void; current?: boolean; dot?: boolean }[] = [
    { key: 'dashboard', label: t('nav:dashboard'), onSelect: () => p.onNavigate({ name: 'dashboard' }), current: active === 'dashboard' },
    { key: 'storage', label: t('nav:storage'), onSelect: () => p.onNavigate({ name: 'storage' }), current: active === 'storage' },
    { key: 'plugins', label: t('nav:plugins'), onSelect: () => p.onNavigate({ name: 'plugins' }), current: active === 'plugins', dot: p.authNeeded },
    {
      key: 'settings',
      label: t('nav:settings'),
      onSelect: () => (p.onSettingsOpen ? p.onSettingsOpen() : p.onNavigate({ name: 'settings' })),
      current: active === 'settings',
    },
    { key: 'activity', label: t('nav:activity'), onSelect: () => p.onNavigate({ name: 'activity' }), current: active === 'activity' },
    { key: 'runners', label: t('nav:runners'), onSelect: () => p.onNavigate({ name: 'runners' }), current: active === 'runners' },
    { key: 'friends', label: friendsLabel, onSelect: () => p.onOpenFriends() },
  ];
  return (
    <DropdownMenu open={open} onOpenChange={setOpen} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('nav:sourcebar.places')}
          className="relative grid size-7 place-items-center rounded-lg text-text-muted transition hover:bg-surface-strong hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <Settings className="h-4 w-4" aria-hidden />
          {((p.friendsUnread ?? 0) > 0 || p.authNeeded) && (
            <span aria-label={t('nav:needsAttention')} className="absolute right-1 top-1 size-1.5 rounded-full bg-accent-strong" />
          )}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end">
        {items.map((n) => (
          <DropdownMenuItem key={n.key} asChild onSelect={n.onSelect}>
            <button aria-current={n.current ? 'page' : undefined} className={cx(n.current && 'text-accent-strong')}>
              <span className="min-w-0 flex-1 text-left">{n.label}</span>
              {n.dot && <span className="size-1.5 flex-none rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
            </button>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** Ticks once a second while mounted, so the footer elapsed time stays live. */
function useNow(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);
  return now;
}

function formatElapsed(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`;
}

// The activity footer (28px, D21 `.tele`): the running game with its elapsed
// time, plus one item per job with its percentage and a 2px progress line.
// Atlas parity (issue #132): each session carries the same Stop action as the
// sidebar rows, and the right side shows the last scan, the offline marker
// and the version, from the same sources as the classic header.
export function SourceFooter(p: Pick<ShellProps, 'sessions' | 'jobs' | 'sessionStartedAt'> & {
  onStop?: (gameId: string) => void;
  lastScan?: number | null;
  online?: boolean;
}) {
  const { t } = useTranslation(['nav', 'library']);
  const now = useNow();
  const sessions = p.sessions ?? [];
  const jobs = (p.jobs ?? []).filter((j) => j.state === 'queued' || j.state === 'running');
  return (
    <footer aria-label={t('activity')} data-region="sourcebar-footer" className="flex h-7 flex-none items-stretch gap-2 overflow-hidden whitespace-nowrap border-t border-hairline bg-ground text-[12px] text-text-faint">
      {sessions.map((e) => {
        const started = p.sessionStartedAt?.[e.game_id];
        const title = (e as SessionEvent & { title: string }).title;
        return (
          <span key={e.game_id} className="relative inline-flex items-center gap-2 border-r border-hairline px-3.5">
            <span aria-hidden className="size-[7px] rounded-full bg-ok" />
            <span className="font-semibold text-text">{title}</span>
            {started != null && <b className="font-medium tabular-nums text-text">{formatElapsed(now - started)}</b>}
            {p.onStop && e.state !== 'stopping' && (
              <button
                type="button"
                onClick={() => p.onStop?.(e.game_id)}
                aria-label={t('nav:stopGame', { title })}
                className="grid size-5 place-items-center rounded text-text-faint transition hover:bg-surface-strong hover:text-danger focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
              >
                <Square className="h-2.5 w-2.5 fill-current" aria-hidden />
              </button>
            )}
          </span>
        );
      })}
      {jobs.map((j) => {
        const pct = j.percent >= 0 ? Math.round(j.percent) : 0;
        return (
          <span key={j.id} className="relative inline-flex items-center gap-2 border-r border-hairline px-3.5">
            {j.title}
            <b className="font-medium tabular-nums text-text">{pct}%</b>
            <span aria-hidden className="absolute inset-x-0 bottom-0 h-0.5 bg-accent opacity-70" style={{ width: `${pct}%` }} />
          </span>
        );
      })}
      <span className="ml-auto inline-flex min-w-0 flex-none items-center gap-2 self-center truncate px-3">
        {p.online === false && (
          <span role="status" className="inline-flex flex-none items-center gap-1 text-danger">
            <WifiOff className="h-3.5 w-3.5" aria-hidden />
            {t('library:offline')}
          </span>
        )}
        {p.lastScan != null && <span className="truncate tabular-nums">{t('library:scannedFooter', { time: clock(p.lastScan) })}</span>}
        <span className="flex-none">{APP_VERSION}</span>
      </span>
    </footer>
  );
}
