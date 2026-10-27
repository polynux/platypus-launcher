// The app shell as a view (issue #104, ADR 0002 answer 4). The shell has one
// region per shell.navigation value (catalog shell.sidebar, shell.topbar,
// shell.sourcebar for atlas in #125): the sidebar's parts (inventory §1.2)
// are blocks each region arranges. The sidebar region is the Sidebar itself,
// unchanged; the topbar region lays the same blocks out in two rows, with the
// Tools items in a menu; the sourcebar region is the /22 chrome (D22.jsx):
// a 48px top bar, a 50px library bar and a 28px status footer. Library and
// game views render the library bar (#135); other views get the full-width
// body. The game-colour switch lives in Settings > Appearance (#134), not here.
import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Activity, ArrowDownToLine, Check, ChevronDown, CircleAlert, Clock, Cpu, HardDriveDownload, Layers, Menu, Plus, Puzzle, Search, Settings, SlidersHorizontal, Square, Star, Sparkles, Ticket, Users, Rows3, LayoutGrid, Table2, WifiOff, type LucideIcon } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ds/dropdown-menu';
import { cx, clock, relTime, APP_VERSION } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';
import { Input } from './ds/input';
import { storeLabel } from '../lib/games';
import { avatarHue, avatarHueStyle } from '../lib/social';
import { TIER_FILTERS, tierFilterValue } from '../lib/compat';
import { FriendsSummary } from './FriendsSummary';
import { PluginLogo } from './PluginLogo';
import { Sidebar } from './Sidebar';
import { useVariant } from '../lib/theme';
import { openCommandPalette } from './CommandPalette';
import { STATUS_FILTERS, tierOptionLabel, statusOptionLabel, type StatusFilter } from './LibraryView';
import type { TierFilter } from '../lib/compat';
import type { Collection, Game, Job, SessionEvent, SocialView, StoreType, View } from '../types';
import type { Ownership } from './LibraryView';

type NavKey = 'dashboard' | 'library' | 'downloads' | 'activity' | 'fixes' | 'runners' | 'plugins' | 'storage' | 'settings';

type SidebarProps = Parameters<typeof Sidebar>[0];

interface ShellProps extends SidebarProps {
  /** Library ownership for the library bar All/Installed tabs. */
  ownership?: Ownership;
  onOwnership?: (o: Ownership) => void;
  /** Every game the library holds (for the All count). Defaults to gameCount. */
  ownedCount?: number;
  /** Selected stores for the library bar (empty = all). Single-select writes through to onPickSource. */
  storeSelection?: StoreType[];
  onSelectStores?: (s: StoreType[]) => void;
  /** Queued or running jobs for the Downloads ring and the activity footer. */
  jobs?: Job[];
  /** When each session started (ms epoch), for the footer's elapsed time. */
  sessionStartedAt?: Record<string, number>;
  /** Opens the add-game dialog (the More places menu entry, #132 parity). */
  onAddGame?: () => void;
  onNewCollection?: () => void;
  /** The friends side panel is open (the avatar stays marked, #135). */
  friendsOpen?: boolean;
  /** Banners between the top bar and the library bar (migration, #135). */
  banner?: React.ReactNode;
  /** Library filter state, owned by App so the library bar and the list share it (#135). */
  tier?: TierFilter;
  onTier?: (v: TierFilter) => void;
  statusFilter?: StatusFilter;
  onStatusFilter?: (v: StatusFilter) => void;
  lending?: 'all' | 'with-me';
  onLending?: (v: 'all' | 'with-me') => void;
  /** The library games, for the filters popover's per-option counts. */
  filterGames?: Game[];
  /** Whether any game is lent through a family (shows the lent row). */
  hasBorrowed?: boolean;
  /** Which library view the /22 switch shows (list-detail or the classic grid/table, #136). */
  atlasView?: 'list' | 'grid' | 'table';
  onAtlasView?: (v: 'list' | 'grid' | 'table') => void;
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
        active === n.key ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-text'
      )}
    >
      <span>{n.label}</span>
      {n.key === 'library' && p.loading ? (
        <span aria-label={t('nav:countingGames')} className="h-4 w-7 animate-pulse rounded-full bg-surface-strong motion-reduce:animate-none" />
      ) : (
        n.count != null && <Badge variant="neutral" className="h-5 rounded-full tracking-normal tabular-nums">{n.count}</Badge>
      )}
      {n.dot && <span className="h-1.5 w-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
    </button>
  );

  return (
    <header data-region="topbar" className="flex-none border-b border-hairline bg-ground">
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
                        active === n.key ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-text'
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
        <label className="relative ml-auto block w-36 flex-none xl:w-52">
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
            active === 'settings' ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface-strong hover:text-text'
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
                    current ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface hover:text-text'
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
                className="flex items-center gap-1.5 whitespace-nowrap rounded-lg px-2 py-1 text-[12px] text-text-body transition hover:bg-surface hover:text-text"
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
                  p.activeStore === s.id ? 'bg-accent-strong/12 text-accent-strong' : 'text-text-body hover:bg-surface hover:text-text'
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

// The /22 Atlas chrome (issue #135, D22.jsx `.tb`, `nav.sources`, `.body`,
// `.social`, `StatusFooter`, `Banners`). The grid rows are 48px, 50px and
// 28px: the top bar and library bar render here, the 1fr body is App's main
// plus the friends side panel, and the 28px footer renders after it
// (SourceFooter). Only the library and game views show the library bar;
// Home and the pages get the full-width body.
function SourcebarChrome(p: ShellProps) {
  const { t } = useTranslation(['nav', 'common']);
  const libPage = p.view.name === 'library' || p.view.name === 'game';
  return (
    <div data-region="sourcebar" className="flex-none text-[13.5px]">
      <TopBar p={p} />
      {p.banner}
      {libPage && <LibraryBar p={p} />}
    </div>
  );
}

// The /22 top bar (D22 `.tb`): brand, Home/Library/Store nav, a centred
// command button, and the Downloads ring, Attention count, Friends avatar and
// More places menu on the right.
function TopBar({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav', 'common']);
  const libOn = p.view.name === 'library' || p.view.name === 'game';
  const storeOn = p.view.name === 'store';
  const goStore = () => {
    // The /22 stores page lists every storefront (#138): open the first, or
    // Plugins to add one when there is none.
    if (p.view.name === 'store') return;
    if ((p.stores ?? []).length > 0) p.onOpenStore?.(p.stores![0].id);
    else p.onNavigate({ name: 'plugins' });
  };
  const nav = [
    // /22 calls the dashboard Home (nav.home), like the lab's top bar. The
    // sidebar/topbar variants keep nav.dashboard.
    { id: 'home', label: t('nav:home'), on: p.view.name === 'dashboard', go: () => p.onNavigate({ name: 'dashboard' }) },
    { id: 'library', label: t('nav:library'), on: libOn, go: () => p.onNavigate({ name: 'library' }) },
    { id: 'store', label: t('nav:store'), on: storeOn, go: goStore },
  ];
  return (
    <header className="grid h-12 grid-cols-[minmax(max-content,1fr)_minmax(200px,460px)_minmax(max-content,1fr)] items-center gap-4 bg-ground px-3.5">
      <div className="flex items-center gap-5">
        <div className="flex flex-none items-center gap-2.5 font-bold text-[14.5px] text-text">
          <span aria-hidden className="grid size-6 place-items-center rounded-xl bg-accent text-[13px] font-extrabold text-on-accent">P</span>
          <span>Platypus</span>
        </div>
        <nav aria-label={t('nav:main')} className="flex min-w-0 items-center gap-0.5">
          {nav.map((n) => (
            <button
              key={n.id}
              type="button"
              onClick={n.go}
              aria-current={n.on ? 'page' : undefined}
              className={cx(
                'relative h-8 whitespace-nowrap rounded-lg px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                n.on ? 'text-text' : 'text-text-muted hover:bg-surface hover:text-text'
              )}
            >
              {n.label}
              {n.on && <span aria-hidden className="absolute inset-x-3 -bottom-[9px] h-0.5 rounded-full bg-accent" />}
            </button>
          ))}
        </nav>
      </div>
      <button
        type="button"
        onClick={openCommandPalette}
        className="flex h-[30px] min-w-0 items-center gap-2.5 rounded-lg bg-surface pl-3 pr-1.5 text-left text-text-faint transition hover:text-text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <Search className="h-3.5 w-3.5 flex-none" aria-hidden />
        <span className="min-w-0 flex-1 truncate text-left text-[13px]">{t('nav:sourcebar.searchOrCommand')}</span>
        <kbd className="flex-none rounded-[3px] px-[5px] py-px font-mono text-[11px] text-text-muted ring-1 ring-inset ring-hairline-strong">Ctrl K</kbd>
      </button>
      <div className="flex min-w-0 items-center justify-end gap-1">
        <DownloadsButton p={p} />
        <AttentionButton p={p} />
        <FriendsButton p={p} />
        <MorePlacesMenu p={p} />
      </div>
    </header>
  );
}

// Downloads icon with the progress ring (D22 `.tb-ring`): the average of the
// active download jobs, like the lab.
function DownloadsButton({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav']);
  const on = p.view.name === 'downloads';
  const DL_KINDS = new Set(['install', 'update']);
  const dl = (p.jobs ?? []).filter((j) => DL_KINDS.has(j.kind) && (j.state === 'queued' || j.state === 'running' || j.state === 'paused'));
  const progress = dl.length ? dl.reduce((s, j) => s + Math.max(j.percent, 0), 0) / dl.length / 100 : 0;
  const label = dl.length > 0 ? `${t('nav:downloads')}, ${dl.length} active` : t('nav:downloads');
  return (
    <button
      type="button"
      onClick={() => p.onNavigate({ name: 'downloads' })}
      aria-label={label}
      title={t('nav:downloads')}
      aria-current={on ? 'page' : undefined}
      className={cx(
        'relative grid size-[34px] flex-none place-items-center rounded-[9px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
        on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
      )}
    >
      <ArrowDownToLine className="h-4 w-4" aria-hidden />
      {dl.length > 0 && (
        <svg aria-hidden viewBox="0 0 36 36" className="pointer-events-none absolute inset-[1px] h-8 w-8 -rotate-90">
          <circle cx="18" cy="18" r="16" fill="none" strokeWidth="2.2" strokeLinecap="round" className="stroke-accent transition-[stroke-dasharray] duration-700 ease-linear" style={{ strokeDasharray: `${Math.round(progress * 100)} 100` }} />
        </svg>
      )}
    </button>
  );
}

function AttentionButton({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav']);
  const on = p.view.name === 'fixes';
  const label = p.fixCount > 0 ? `${t('nav:sourcebar.attention')}, ${p.fixCount}` : t('nav:sourcebar.attention');
  return (
    <button
      type="button"
      onClick={() => p.onNavigate({ name: 'fixes' })}
      aria-label={label}
      title={t('nav:sourcebar.attention')}
      aria-current={on ? 'page' : undefined}
      className={cx(
        'relative grid size-[34px] flex-none place-items-center rounded-[9px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
        on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
      )}
    >
      <CircleAlert className="h-4 w-4" aria-hidden />
      {p.fixCount > 0 && (
        <span className="absolute -right-[3px] -top-[1px] grid h-4 min-w-4 place-items-center rounded-full bg-danger px-1 text-[10px] font-bold tabular-nums text-white ring-2 ring-ground">
          {p.fixCount}
        </span>
      )}
    </button>
  );
}

// The friends avatar (D22 `.tb-friends`): the unread badge toggles the side
// panel App renders (#135); other themes keep the modal.
function dotFor(state: string): string {
  switch (state) {
    case 'online':
    case 'looking':
      return 'bg-ok';
    case 'away':
    case 'busy':
    case 'snooze':
      return 'bg-attention';
    default:
      return 'bg-text-faint';
  }
}

function FriendsButton({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav']);
  const self = (p.friends ?? []).find((v) => v.self)?.self;
  const initial = (self?.name ?? '?').slice(0, 1).toUpperCase();
  const label = (p.friendsUnread ?? 0) > 0 ? `${t('nav:sourcebar.friends')}, ${p.friendsUnread} unread` : t('nav:sourcebar.friends');
  return (
    <button
      type="button"
      onClick={() => p.onOpenFriends()}
      aria-label={label}
      title={t('nav:sourcebar.friends')}
      aria-expanded={p.friendsOpen}
      className={cx(
        'relative grid size-[34px] flex-none place-items-center rounded-[9px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
        p.friendsOpen ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
      )}
    >
      {self?.avatarUrl ? (
        <span className="relative h-6 w-6 flex-none">
          <img src={self.avatarUrl} alt="" className="h-6 w-6 rounded-full object-cover" />
          <span aria-hidden className={cx('absolute -bottom-px -right-px size-2.5 rounded-full ring-2 ring-ground', dotFor(self.state))} />
        </span>
      ) : (
        <span className="relative h-6 w-6 flex-none">
          <span aria-hidden className="grid h-6 w-6 place-items-center rounded-full text-[12px] font-bold text-white" style={avatarHueStyle(self?.name ?? '?')}>
            {self ? initial : <Users className="h-4 w-4" />}
          </span>
          {self && <span aria-hidden className={cx('absolute -bottom-px -right-px size-2.5 rounded-full ring-2 ring-ground', dotFor(self.state))} />}
        </span>
      )}
      {(p.friendsUnread ?? 0) > 0 && (
        <span className="absolute -right-[3px] -top-[1px] grid h-4 min-w-4 place-items-center rounded-full bg-danger px-1 text-[10px] font-bold tabular-nums text-white ring-2 ring-ground">
          {p.friendsUnread}
        </span>
      )}
    </button>
  );
}

// Atlas parity (issue #132): every place behind one menu — Activity, Runners,
// Plugins (with the attention badge), Storage, Settings, then Redeem a code
// and Add a game.
function MorePlacesMenu({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav', 'library']);
  const [open, setOpen] = useState(false);
  const active = p.view.name === 'game' ? (p.view.from === 'fixes' ? 'fixes' : p.view.from) : p.view.name;
  const pluginAttention = p.authNeeded;
  const managing = ['activity', 'runners', 'plugins', 'storage', 'settings'].includes(active ?? '');
  const warnCount = (p.stores ?? []).filter((s) => (s as { status?: string }).status !== 'ready').length;
  const items: { key: string; label: string; Icon: LucideIcon; onSelect: () => void; current?: boolean; badge?: boolean }[] = [
    { key: 'activity', label: t('nav:activity'), Icon: Activity, onSelect: () => p.onNavigate({ name: 'activity' }), current: active === 'activity' },
    { key: 'runners', label: t('nav:runners'), Icon: Cpu, onSelect: () => p.onNavigate({ name: 'runners' }), current: active === 'runners' },
    { key: 'plugins', label: t('nav:plugins'), Icon: Puzzle, onSelect: () => p.onNavigate({ name: 'plugins' }), current: active === 'plugins', badge: pluginAttention },
    { key: 'storage', label: t('nav:storage'), Icon: HardDriveDownload, onSelect: () => p.onNavigate({ name: 'storage' }), current: active === 'storage' },
    {
      key: 'settings',
      label: t('nav:settings'),
      Icon: Settings,
      onSelect: () => (p.onSettingsOpen ? p.onSettingsOpen() : p.onNavigate({ name: 'settings' })),
      current: active === 'settings',
    },
  ];
  return (
    <DropdownMenu open={open} onOpenChange={setOpen} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('nav:sourcebar.morePlaces')}
          aria-expanded={open}
          className={cx('relative grid size-[34px] flex-none place-items-center rounded-[9px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus', managing ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text')}
        >
          <Menu className="size-[17px]" aria-hidden />
          {pluginAttention && (
            <span aria-label={t('nav:needsAttention')} className="absolute right-[6px] top-[6px] size-[7px] rounded-full bg-attention" />
          )}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end">
        {items.map((n) => (
          <DropdownMenuItem key={n.key} asChild onSelect={n.onSelect}>
            <button aria-current={n.current ? 'page' : undefined} className={cx('w-full', n.current && 'font-semibold text-text')}>
              <n.Icon aria-hidden className="size-[15px] flex-none text-text-muted" />
              <span className="min-w-0 flex-1 text-left">{n.label}</span>
              {n.badge && <span aria-label={t('nav:needsAttention')} className="ml-auto rounded-full bg-attention px-1.5 text-[11px] font-bold text-on-accent">{warnCount > 0 ? warnCount : '!'}</span>}
            </button>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        {p.canRedeem && (
          <DropdownMenuItem asChild onSelect={() => p.onRedeem?.()}>
            <button>
              <Ticket aria-hidden className="size-[15px] flex-none text-text-muted" />
              <span>{t('nav:sourcebar.redeem')}</span>
            </button>
          </DropdownMenuItem>
        )}
        {p.onAddGame && (
          <DropdownMenuItem asChild onSelect={() => p.onAddGame?.()}>
            <button>
              <Plus aria-hidden className="size-[15px] flex-none text-text-muted" />
              <span>{t('library:listDetail.addGame')}</span>
            </button>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// The /22 library bar (D22 `nav.sources`): the All/Installed own switch with
// counts, Every store plus one tab per store (warning dots for plugins that
// need attention, the lab's Sideloaded toggle), and the Collections menu,
// Filters button and list/grid/table view switch on the right.
function LibraryBar({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav', 'common', 'library']);
  const ownership = p.ownership ?? 'installed';
  const selection = p.storeSelection ?? [];
  const selectStores = p.onSelectStores ?? (() => {});
  const ownedCount = p.ownedCount ?? p.gameCount;
  const pickStore = (s: StoreType, shift: boolean) => {
    if (shift) {
      const next = selection.includes(s) ? selection.filter((x) => x !== s) : [...selection, s];
      selectStores(next);
    } else {
      selectStores(selection.length === 1 && selection[0] === s ? [] : [s]);
    }
    if (p.view.name !== 'library') p.onNavigate({ name: 'library' });
  };
  const goLibrary = (o: Ownership) => {
    p.onOwnership?.(o);
    selectStores([]);
    if (p.view.name !== 'library') p.onNavigate({ name: 'library' });
  };
  // A collection is its own scope: All/Installed look active only on the
  // unfiltered library.
  const collection = p.view.name === 'library' ? p.view.collection : undefined;
  const allOn = ownership === 'owned' && selection.length === 0 && !collection;
  const installedOn = ownership === 'installed' && selection.length === 0 && !collection;
  // Stores with games, like the sidebar's Sources rows: the known storefronts
  // first, then anything a plugin reported, then sideloaded games.
  const storeTabs = [
    ...SOURCES.filter((s) => (p.storeCounts[s.key] ?? 0) > 0),
    ...Object.keys(p.storeCounts)
      .filter((k) => k !== 'custom' && !SOURCES.some((s) => s.key === k) && (p.storeCounts[k] ?? 0) > 0)
      .map((k) => ({ key: k, label: storeLabel(k) })),
    ...((p.storeCounts.custom ?? 0) > 0 ? [{ key: 'custom', label: t('nav:sideloaded') }] : []),
  ];
  // Warning dots for stores whose plugin needs attention (not ready).
  const warn = new Set(
    (p.stores ?? []).filter((s) => (s as { status?: string }).status !== 'ready').map((s) => (s as { store?: string }).store ?? s.id),
  );
  return (
    <nav aria-label={t('nav:library')} className="flex h-[50px] items-center gap-1.5 border-y border-hairline bg-raised px-3">
      {/* // #134: ground-2 when the token lands. */}
      <div role="group" aria-label={t('nav:sourcebar.ownership')}>
        {([
          { id: 'owned', label: t('nav:sourcebar.all'), count: ownedCount, on: allOn },
          { id: 'installed', label: t('nav:sourcebar.installed'), count: p.gameCount, on: installedOn },
        ] as const).map((v) => (
          <button
            key={v.id}
            type="button"
            aria-pressed={v.on}
            onClick={() => goLibrary(v.id)}
            className={cx(
              'relative inline-flex h-7 flex-none items-center gap-1.5 whitespace-nowrap rounded-[7px] px-[11px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              v.on ? 'bg-[color-mix(in_srgb,var(--color-accent)_20%,var(--color-surface-strong))] text-text ring-1 ring-inset ring-accent/40' : 'text-text-muted hover:bg-surface hover:text-text'
            )}
          >
            {v.label}
            <span className="font-medium tabular-nums text-[12px] text-text-faint">{v.count}</span>
          </button>
        ))}
      </div>
      <span aria-hidden className="mx-1 h-[22px] w-px flex-none bg-hairline" />
      <div
        className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto pr-6 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        style={{ maskImage: 'linear-gradient(to right, black calc(100% - 24px), transparent)' }}
      >
        <button
          type="button"
          aria-pressed={selection.length === 0}
          onClick={() => {
            selectStores([]);
            if (p.view.name !== 'library') p.onNavigate({ name: 'library' });
          }}
          className={cx(
            'relative inline-flex h-8 flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
            selection.length === 0 ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
          )}
        >
          {t('nav:sourcebar.everyStore')}
          {selection.length === 0 && <span aria-hidden className="absolute inset-x-3 -bottom-[9px] h-0.5 rounded-full bg-accent" />}
        </button>
        {storeTabs.map((s) => {
          const on = selection.includes(s.key as StoreType);
          return (
            <button
              key={s.key}
              type="button"
              aria-pressed={on}
              title={t('nav:sourcebar.shiftHint')}
              onClick={(e) => pickStore(s.key as StoreType, e.shiftKey)}
              className={cx(
                'relative inline-flex h-8 flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
              )}
            >
              <span className={cx('grid place-items-center', on && 'text-accent')}>
                <PluginLogo src={p.sourceLogos?.[s.key]} name={s.label} size="xs" />
              </span>
              {s.label}
              <span className="font-medium tabular-nums text-[12px] text-text-faint">{p.storeCounts[s.key] ?? 0}</span>
              {warn.has(s.key) && <span className="size-1.5 rounded-full bg-attention" aria-label={t('nav:needsAttention')} />}
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
      <div className="flex flex-none items-center gap-1 pl-2">
        <CollectionsMenu p={p} />
        <LibraryFiltersButton p={p} />
        <AtlasViewSwitch p={p} />
      </div>
    </nav>
  );
}

// The /22 filters button (D22 FilterButton + 08-filters FilterPanel):
// fieldsets of checkboxes sharing App's filter state with the list (#135).
// Single-value semantics with checkboxes as radios: picking the active option
// clears it. Counts ride with the options like the lab.
function LibraryFiltersButton({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav', 'library']);
  const [open, setOpen] = useState(false);
  const games = p.filterGames ?? [];
  const tierCounts = useMemo(() => {
    const m = new Map<TierFilter, number>();
    for (const g of games) {
      const v = tierFilterValue(g) as TierFilter;
      m.set(v, (m.get(v) ?? 0) + 1);
    }
    return m;
  }, [games]);
  const statusCounts = useMemo(() => {
    const m = new Map<StatusFilter, number>();
    for (const g of games) {
      const v = ((g.status ?? 'none') as StatusFilter) ?? 'none';
      m.set(v, (m.get(v) ?? 0) + 1);
    }
    return m;
  }, [games]);
  const lentCount = useMemo(() => games.filter((g) => g.shared_by != null).length, [games]);
  const tier = p.tier ?? 'all';
  const statusFilter = p.statusFilter ?? 'all';
  const lending = p.lending ?? 'all';
  const activeCount = [tier !== 'all', statusFilter !== 'all', lending !== 'all'].filter(Boolean).length;
  const reset = () => {
    p.onTier?.('all');
    p.onStatusFilter?.('all');
    p.onLending?.('all');
  };
  const checkRow = (key: string, label: string, count: number, checked: boolean, onToggle: () => void) => (
    <button
      key={key}
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={label}
      onClick={onToggle}
      className="flex h-8 w-full items-center gap-[9px] rounded-lg px-2.5 text-left text-[13.5px] font-normal text-text transition hover:bg-surface-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
    >
      <span aria-hidden className={cx('grid size-4 flex-none place-items-center rounded-[4px] ring-1 ring-inset transition', checked ? 'bg-accent ring-accent' : 'ring-hairline-strong')}>
        {checked && <Check className="size-3 text-on-accent" aria-hidden />}
      </span>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <span className="flex-none text-[12px] tabular-nums text-text-faint">{count}</span>
    </button>
  );
  return (
    <DropdownMenu open={open} onOpenChange={setOpen} modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={t('library:listDetail.filtersAria')}
          aria-expanded={open}
          className={cx(
            'relative inline-flex h-[34px] flex-none items-center gap-1.5 whitespace-nowrap rounded-xl px-3 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
            open || activeCount > 0 ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
          )}
        >
          <SlidersHorizontal className="h-[15px] w-[15px]" aria-hidden />
          {t('library:filters')}
          {activeCount > 0 && <span className="size-1.5 rounded-full bg-accent-strong" aria-label={t('nav:needsAttention')} />}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="bottom" align="end" className="w-[300px] p-1.5">
        <fieldset>
          <legend className="px-2.5 pb-1 pt-2 text-[12px] font-bold text-text-faint">{t('library:listDetail.filterTiers')}</legend>
          {TIER_FILTERS.filter((v) => v !== 'all').map((v) => checkRow(v, tierOptionLabel(t, v), tierCounts.get(v) ?? 0, tier === v, () => p.onTier?.(tier === v ? 'all' : v)))}
        </fieldset>
        <fieldset className="border-t border-hairline">
          <legend className="px-2.5 pb-1 pt-2 text-[12px] font-bold text-text-faint">{t('library:listDetail.filterStatuses')}</legend>
          {STATUS_FILTERS.filter((v) => v !== 'all').map((v) => checkRow(v, statusOptionLabel(t, v), statusCounts.get(v) ?? 0, statusFilter === v, () => p.onStatusFilter?.(statusFilter === v ? 'all' : v)))}
        </fieldset>
        {(p.hasBorrowed ?? false) && (
          <fieldset className="border-t border-hairline">
            <legend className="px-2.5 pb-1 pt-2 text-[12px] font-bold text-text-faint">{t('library:listDetail.filterFamily')}</legend>
            {checkRow('with-me', t('library:listDetail.filterLentOnly'), lentCount, lending === 'with-me', () => p.onLending?.(lending === 'with-me' ? 'all' : 'with-me'))}
          </fieldset>
        )}
        <div className="border-t border-hairline p-1">
          <button
            type="button"
            onClick={reset}
            className={cx(
              'w-full rounded-lg px-2 py-1.5 text-left text-[13px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              activeCount > 0 ? 'text-accent hover:bg-surface-strong' : 'text-text-faint'
            )}
          >
            {t('library:listDetail.filterClear')}
          </button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

// The /22 view switch (D22 ViewSwitch): list with the game beside it, grid
// of covers, table. The Atlas grid/table are the lib-full page (issue #136).
function AtlasViewSwitch({ p }: { p: ShellProps }) {
  const { t } = useTranslation(['nav']);
  const view = p.atlasView ?? 'list';
  const opts = [
    { id: 'list', label: t('nav:sourcebar.viewList'), Icon: Rows3 },
    { id: 'grid', label: t('nav:sourcebar.viewGrid'), Icon: LayoutGrid },
    { id: 'table', label: t('nav:sourcebar.viewTable'), Icon: Table2 },
  ] as const;
  return (
    <div role="group" aria-label={t('nav:sourcebar.viewSwitch')} className="flex flex-none items-center gap-0.5">
      {opts.map(({ id, label, Icon }) => {
        const on = view === id;
        return (
          <button
            key={id}
            type="button"
            aria-label={label}
            title={label}
            aria-pressed={on}
            onClick={() => p.onAtlasView?.(id)}
            className={cx(
              'grid size-[34px] flex-none place-items-center rounded-xl transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
              on ? 'bg-surface-strong text-text' : 'text-text-muted hover:bg-surface hover:text-text'
            )}
          >
            <Icon className="h-4 w-4" aria-hidden />
          </button>
        );
      })}
    </div>
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
  const builtins = (p.collections ?? []).filter((c) => c.builtin);
  const customs = (p.collections ?? []).filter((c) => !c.builtin);
  const rowIcon = (c: { id: string; name: string; builtin?: boolean; smart?: unknown }): React.ReactNode => {
    if (c.builtin) {
      const Icon = c.id === 'favourites' ? Star : c.id === 'installed' ? HardDriveDownload : Clock;
      return <Icon aria-hidden className="size-[15px] flex-none text-text-muted" />;
    }
    if (c.smart) return <Sparkles aria-hidden className="size-[15px] flex-none text-text-muted" />;
    return <span aria-hidden className="mx-[3.5px] size-2 flex-none rounded-full" style={avatarHueStyle(c.name)} />;
  };
  const row = (c: { id: string; name: string; builtin?: boolean; smart?: unknown }) => {
    const on = c.id === activeId;
    return (
      <DropdownMenuItem key={c.id} asChild onSelect={() => p.onOpenCollection?.(c.id)}>
        <button aria-current={on ? 'page' : undefined} className={cx('w-full', on && 'text-accent-strong')}>
          {rowIcon(c)}
          <span className="min-w-0 flex-1 truncate text-left">{c.builtin ? t(`collections:builtin.${c.id}`) : c.name}</span>
          <span className="font-medium tabular-nums text-[12px] text-text-faint">{p.collectionCounts?.[c.id] ?? 0}</span>
        </button>
      </DropdownMenuItem>
    );
  };
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
        {builtins.map(row)}
        {builtins.length > 0 && customs.length > 0 && <DropdownMenuSeparator />}
        {customs.map(row)}
        {p.onManageCollections && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild onSelect={() => (p.onNewCollection ?? p.onManageCollections)?.()}>
              <button>
                <Plus aria-hidden className="size-[15px] flex-none text-text-faint" />
                <span>{t('collections:newHeading')}</span>
              </button>
            </DropdownMenuItem>
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
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}` : `${m}:${String(s % 60).padStart(2, '0')}`;
}

// The activity footer (28px, D21 `.tele`): the running game with its elapsed
// time, plus one item per job with its percentage and a 2px progress line.
// Atlas parity (issue #132): each session carries the same Stop action as the
// sidebar rows, and the right side shows the last scan, the offline marker
// and the version, from the same sources as the classic header.
export function SourceFooter(p: Pick<ShellProps, 'sessions' | 'jobs' | 'sessionStartedAt'> & {
  onStop?: (gameId: string) => void;
  /** Opens Downloads (the job items are buttons, R4-14). */
  onOpenDownloads?: () => void;
  /** The library, for the custom game titles (R4-14). */
  games?: Game[];
  lastScan?: number | null;
  online?: boolean;
}) {
  const { t } = useTranslation(['nav', 'library', 'game']);
  const now = useNow();
  const sessions = p.sessions ?? [];
  const jobs = (p.jobs ?? []).filter((j) => j.state === 'queued' || j.state === 'running' || j.state === 'paused');
  const titles = useMemo(() => new Map((p.games ?? []).map((g) => [g.id, g.title])), [p.games]);
  const titleOf = (j: Job) => titles.get(j.game_id) ?? j.title;
  return (
    <footer aria-label={t('activity')} data-region="sourcebar-footer" className="flex h-7 flex-none items-stretch gap-2 overflow-hidden whitespace-nowrap border-t border-hairline bg-ground text-[12px] text-text-faint">
      {sessions.map((e) => {
        const started = p.sessionStartedAt?.[e.game_id];
        const title = (e as SessionEvent & { title: string }).title;
        return (
          <span key={e.game_id} className="relative inline-flex items-center gap-2 border-r border-hairline px-3 text-ok">
            <span aria-hidden className="size-[7px] rounded-full bg-ok" />
            <span className="font-semibold">{title}</span>
            {started != null && <b className="font-semibold tabular-nums text-text">{formatElapsed(now - started)}</b>}
            {p.onStop && e.state !== 'stopping' && (
              <button
                type="button"
                onClick={() => p.onStop?.(e.game_id)}
                aria-label={t('nav:stopGame', { title })}
                className="inline-flex h-5 items-center gap-[5px] rounded-[5px] bg-surface-strong px-2 font-semibold text-text hover:bg-[color-mix(in_srgb,var(--color-danger)_40%,var(--color-surface-strong))] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
              >
                <Square className="size-[9px] fill-current" aria-hidden />
                {t('game:play.stop')}
              </button>
            )}
          </span>
        );
      })}
      {jobs.map((j) => {
        const pct = j.percent >= 0 ? Math.round(j.percent) : 0;
        return (
          <button key={j.id} type="button" onClick={p.onOpenDownloads} className="relative inline-flex items-center gap-2 border-r border-hairline px-3 text-text-muted hover:bg-surface hover:text-text">
            {titleOf(j)}
            <b className="font-semibold tabular-nums text-text">{pct}%</b>
            <span aria-hidden className={cx('absolute inset-x-0 bottom-0 h-0.5 opacity-75', j.state === 'paused' ? 'bg-text-faint' : 'bg-accent')} style={{ width: `${pct}%` }} />
          </button>
        );
      })}
      <span className="ml-auto inline-flex min-w-0 flex-none items-stretch self-center">
        {p.online === false && (
          <span role="status" className="inline-flex flex-none items-center gap-1 border-l border-hairline px-3 text-danger">
            <WifiOff className="h-3.5 w-3.5" aria-hidden />
            {t('library:offline')}
          </span>
        )}
        {p.lastScan != null && <span className="inline-flex items-center truncate border-l border-hairline px-3 tabular-nums">{t('library:scannedFooter', { time: relTime(p.lastScan, now) })}</span>}
        <span className="inline-flex flex-none items-center border-l border-hairline px-3">{APP_VERSION}</span>
      </span>
    </footer>
  );
}
