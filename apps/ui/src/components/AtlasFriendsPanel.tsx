// i18n: converted
// The /22 friends side panel (issue #135, D22 `.social`): the FriendsModal
// content (friends, chat, families) as a panel right of the stage, in the /22
// look (a MeCard, tabs for Friends/Chats/Family, Playing/Online/Offline
// groups). Atlas only; other themes keep the modal. Escape backs out of the
// open chat first, then closes the panel.
import React, { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown, Search, X } from 'lucide-react';
import { Button } from './ds/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ds/dropdown-menu';
import { Input } from './ds/input';
import { Switch } from './ds/switch';
import { ChatPanel } from './ChatPanel';
import { FamiliesPanel } from './FamiliesPanel';
import { agoLabel } from '../lib/cloud';
import { LinkButton, cx } from './ui';
import { PluginLogo } from './PluginLogo';
import {
  avatarHueStyle,
  canSetStatus,
  connectionLine,
  friendStateLabel,
  groupFriends,
  initials,
  SOCIAL_STATUSES,
  statusLabel,
} from '../lib/social';
import type { SocialFriend, SocialStatus, SocialView, StorePlugin } from '../types';

interface AtlasFriendsPanelProps {
  views: SocialView[];
  /** Store names by plugin id. */
  labels: Record<string, string>;
  /** The store plugins, for the family of each store that offers it. */
  plugins: StorePlugin[];
  /** Whether a Platypus game key is in the library (a friend's game links there). */
  inLibrary: (gameId: string) => boolean;
  onClose: () => void;
  onSetEnabled: (plugin: string, enabled: boolean) => void;
  onSetStatus: (plugin: string, state: SocialStatus) => void;
  onSetPlaying: (plugin: string, show: boolean) => void;
  /** Turns a store's friends off and on again, after its session was replaced or logged out. */
  onReconnect: (plugin: string) => void;
  onOpenGame: (gameId: string) => void;
  /** Unread messages from a friend on a store (the chat capability). */
  unreadOf?: (plugin: string, friendId: string) => number;
  /** Opens the conversation with a friend (the chat capability). */
  onOpenChat?: (plugin: string, friend: SocialFriend) => void;
  /** The open conversation, shown in place of the tabs; the chat's own Back and Close drive it. */
  chat?: React.ReactNode;
  /** Leaves the open conversation for the tabs. Esc does this first, then closes the panel. */
  onBackFromChat?: () => void;
  /** Whether a store's message alerts are on, for each store that offers chat. */
  chatAlerts?: Record<string, boolean>;
  onSetChatAlerts?: (plugin: string, on: boolean) => void;
}

const ROWS_PER_GROUP = 200;

/** Dot colour for a status or friend state, from the theme tokens only. */
function tokenDot(state: string): string {
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

function PanelAvatar({ name, url, dot }: { name: string; url?: string; dot: string }) {
  return (
    <span className="relative h-[26px] w-[26px] flex-none">
      <span className="grid h-full w-full place-items-center overflow-hidden rounded-full text-[12px] font-bold text-white" style={avatarHueStyle(name)}>
        {url ? <img src={url} alt="" className="h-full w-full object-cover" /> : initials(name)}
      </span>
      <span className={cx('absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full', dot)} />
    </span>
  );
}

export function AtlasFriendsPanel(p: AtlasFriendsPanelProps) {
  const { t } = useTranslation(['chat', 'family', 'common']);
  const [tab, setTab] = useState<'friends' | 'chats' | 'family'>('friends');
  const [query, setQuery] = useState('');
  // Escape backs out of the open chat first, then closes the panel (like the
  // modal). Typing in the chat box or a select stays local.
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return;
      const el = e.target as Element | null;
      if (el?.closest?.('[role="listbox"], [data-radix-popper-content-wrapper], input, textarea')) return;
      e.stopPropagation();
      if (p.chat) p.onBackFromChat?.();
      else p.onClose();
    };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, [p.chat, p.onBackFromChat, p.onClose]);
  const unreadTotal = useMemo(() => {
    if (!p.unreadOf) return 0;
    const unreadOf = p.unreadOf;
    return p.views.reduce((n, v) => n + v.friends.reduce((m, f) => m + unreadOf(v.plugin, f.id), 0), 0);
  }, [p.views, p.unreadOf]);
  const chatsLabel = unreadTotal > 0 ? `${t('chat:tabs.chats')} ${unreadTotal}` : t('chat:tabs.chats');
  return (
    <aside aria-label={t('chat:modal.title')} className="flex min-h-0 w-[340px] flex-none flex-col border-l border-hairline bg-raised">
      <header className="flex flex-none items-center justify-between py-2.5 pl-[18px] pr-2.5">
        <h2 className="text-[15px] font-extrabold">{t('chat:modal.title')}</h2>
        <button
          type="button"
          onClick={() => (p.chat ? p.onBackFromChat?.() : p.onClose())}
          aria-label={t('common:close')}
          className="grid size-8 place-items-center rounded-lg text-text-muted transition hover:bg-surface-strong hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <X className="h-[15px] w-[15px]" aria-hidden />
        </button>
      </header>
      {p.chat ? (
        <div className="flex min-h-0 flex-1 flex-col px-[18px] pb-4">{p.chat}</div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <MeCard views={p.views} labels={p.labels} onSetStatus={p.onSetStatus} />
          <div role="tablist" aria-label={t('chat:modal.title')} className="flex flex-none gap-1 px-[18px]">
            {([
              ['friends', t('chat:tabs.friends')],
              ['chats', chatsLabel],
              ['family', t('chat:tabs.family')],
            ] as const).map(([id, label]) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={tab === id}
                onClick={() => setTab(id)}
                className={cx(
                  'h-9 flex-1 rounded-lg text-[13px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  tab === id ? 'bg-surface-strong text-text' : 'text-text-muted hover:text-text'
                )}
              >
                {label}
              </button>
            ))}
          </div>
          {tab === 'friends' && (
            <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-[18px] py-3">
              <label className="relative block flex-none">
                <span className="sr-only">{t('chat:modal.searchLabel')}</span>
                <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-text-faint" />
                <Input
                  type="search"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t('chat:findFriend')}
                  className="h-[30px] rounded-[7px] bg-surface pl-8.5 placeholder:text-text-faint"
                />
              </label>
              {(() => {
                // The lab has no per-store header: with a single connected
                // store it only repeats the MeCard/store notice below.
                const single = p.views.filter((v) => v.enabled).length <= 1;
                return p.views.map((v) => (
                  <StoreSection key={v.plugin} {...p} view={v} label={p.labels[v.plugin] ?? v.plugin} query={query} hideHeader={single} />
                ));
              })()}
            </div>
          )}
          {tab === 'chats' && <ChatsTab {...p} />}
          {tab === 'family' && (
            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-[18px] py-3">
              {p.plugins.map((plugin) => (
                <FamiliesPanel key={plugin.id} plugin={plugin} />
              ))}
            </div>
          )}
        </div>
      )}
    </aside>
  );
}

// The current user above the tabs (D22 MeCard): the first enabled store with
// a self, with a chevron menu for the status (the lab has no separate Select).
function MeCard({ views, labels, onSetStatus }: Pick<AtlasFriendsPanelProps, 'views' | 'labels' | 'onSetStatus'>) {
  const { t } = useTranslation('chat');
  const view = views.find((v) => v.enabled && v.self) ?? views.find((v) => v.self);
  if (!view?.self) return null;
  const label = labels[view.plugin] ?? view.plugin;
  return (
    <div className="flex flex-none items-center gap-3 px-[18px] pb-3">
      <PanelAvatar name={view.self.name} url={view.self.avatarUrl} dot={tokenDot(view.self.state)} />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[13px] font-semibold text-text">{view.self.name}</div>
        <div className="truncate text-hint text-text-faint">{label}</div>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t('chat:modal.statusLabel', { label })}
            disabled={!canSetStatus(view)}
            className="inline-flex h-8 flex-none items-center gap-1 rounded-lg px-2 text-[13px] text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
          >
            <span className="max-w-24 truncate">{statusLabel(view.self.state)}</span>
            <ChevronDown className="h-3.5 w-3.5 flex-none" aria-hidden />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {SOCIAL_STATUSES.map((s) => (
            <DropdownMenuItem key={s} onSelect={() => onSetStatus(view.plugin, s)}>
              <span>{statusLabel(s)}</span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

type SectionProps = AtlasFriendsPanelProps & { view: SocialView; label: string; query: string; hideHeader?: boolean };

function StoreSection({ view, label, query, hideHeader, plugins, onSetEnabled, onSetStatus, onSetPlaying, onReconnect, unreadOf, onOpenChat, chatAlerts, onSetChatAlerts }: SectionProps) {
  const { t } = useTranslation('chat');
  const line = connectionLine(view.connection);
  const groups = useMemo(() => groupFriends(view.friends, query), [view.friends, query]);
  const total = groups.inGame.length + groups.online.length + groups.offline.length;
  const canReconnect = view.connection.state === 'replaced' || view.connection.state === 'loggedOut';
  const logoUrl = useMemo(() => plugins.find((pl) => pl.id === view.plugin)?.icon_url ?? undefined, [plugins, view.plugin]);
  return (
    <section aria-label={t('chat:modal.section', { label })} className="flex flex-col gap-2.5">
      {!hideHeader && (
        <header>
          <h3 className="text-[12px] font-bold text-text-faint">{label}</h3>
        </header>
      )}

      {!view.enabled ? (
        <div role="status" className="flex items-center justify-between gap-3 rounded-lg bg-surface px-3 py-2 text-[12px] text-text-muted ring-1 ring-hairline">
          <span>{t('chat:modal.friendsOff', { label })}</span>
          <Button variant="ghost" size="sm" className="flex-none" onClick={() => onSetEnabled(view.plugin, true)}>
            {t('chat:modal.turnOn')}
          </Button>
        </div>
      ) : (
        <>
          {total === 0 && (
            <p className="text-[12.5px] text-text-muted">
              {view.friends.length === 0 ? t('chat:modal.noFriends', { label }) : t('chat:modal.noMatch', { query: query.trim() })}
            </p>
          )}

          <FriendGroup title={t('chat:group.playing')} friends={groups.inGame} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} />
          <FriendGroup title={t('chat:group.online')} friends={groups.online} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} />
          <FriendGroup title={t('chat:group.offline')} friends={groups.offline} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} />

          {/* Store state lives at the bottom (the lab's notices row): a
              notice only when the store is off or disconnected; the
              presence toggles stay reachable here instead of Settings. */}
          <div className="flex flex-col gap-2.5 border-t border-hairline pt-2.5">
            {line && (
              <div role="status" className="flex items-center justify-between gap-3 rounded-lg bg-attention/[0.08] px-3 py-2 text-[12px] text-text-body ring-1 ring-attention/25">
                <span>{line}</span>
                {canReconnect && (
                  <Button variant="ghost" size="sm" onClick={() => onReconnect(view.plugin)}>
                    {t('chat:modal.reconnect')}
                  </Button>
                )}
              </div>
            )}
            <label className="flex items-center justify-between gap-3 text-[12.5px] text-text-muted">
              <span>{t('chat:modal.showPlaying')}</span>
              <Switch
                aria-label={t('chat:modal.showPlayingLabel', { label })}
                checked={view.showPlaying}
                onCheckedChange={(show) => onSetPlaying(view.plugin, show)}
              />
            </label>
            {chatAlerts && onSetChatAlerts && view.plugin in chatAlerts && (
              <label className="flex items-center justify-between gap-3 text-[12.5px] text-text-muted">
                <span>{t('chat:modal.chatAlerts')}</span>
                <Switch
                  aria-label={t('chat:modal.chatAlertsLabel', { label })}
                  checked={chatAlerts[view.plugin]}
                  onCheckedChange={(on) => onSetChatAlerts(view.plugin, on)}
                />
              </label>
            )}
          </div>
        </>
      )}
    </section>
  );
}

// Unread conversations across the stores: each row opens its chat.
function ChatsTab(p: AtlasFriendsPanelProps) {
  const { t } = useTranslation('chat');
  const unreadOf = p.unreadOf;
  const rows = useMemo(() => {
    if (!unreadOf) return [];
    return p.views.flatMap((v) =>
      v.friends.filter((f) => unreadOf(v.plugin, f.id) > 0).map((f) => ({ view: v, friend: f, unread: unreadOf(v.plugin, f.id) })),
    );
  }, [p.views, unreadOf]);
  if (rows.length === 0) return <p className="px-[18px] py-3 text-[13px] text-text-muted">{t('chat:tabs.empty')}</p>;
  return (
    <ul className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-[18px] py-3">
      {rows.map(({ view, friend: f, unread }) => (
        <li key={`${view.plugin}:${f.id}`}>
          <button
            type="button"
            onClick={() => p.onOpenChat?.(view.plugin, f)}
            className="flex w-full items-center gap-3 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            <PanelAvatar name={f.name} url={f.avatarUrl} dot={tokenDot(f.state)} />
            <span className="grid min-w-0 flex-1">
              <span className="truncate text-[13px] font-semibold text-text">{f.name}</span>
              <span className="truncate text-hint text-text-faint">{p.labels[view.plugin] ?? view.plugin}</span>
            </span>
            <span aria-label={t('chat:row.unread', { count: unread })} className="rounded-full bg-danger px-1.5 text-hint font-bold tabular-nums text-on-accent">
              {unread}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}

function FriendGroup({ title, friends, plugin, logoUrl, unreadOf, onOpenChat }: {
  title: string;
  friends: SocialFriend[];
  plugin: string;
  logoUrl?: string;
  unreadOf?: (plugin: string, friendId: string) => number;
  onOpenChat?: (plugin: string, friend: SocialFriend) => void;
}) {
  const [shown, setShown] = useState(ROWS_PER_GROUP);
  const { t } = useTranslation('chat');
  if (friends.length === 0) return null;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="flex items-center justify-between px-1 text-[12px] font-bold text-text-faint">
        <span>{title}</span>
        <span>{friends.length}</span>
      </h4>
      <ul className="flex flex-col">
        {friends.slice(0, shown).map((f) => (
          <li key={f.id} className="flex items-center gap-2 rounded-lg px-2 py-1 hover:bg-surface">
            <FriendRowMain friend={f} plugin={plugin} logoUrl={logoUrl} onOpenChat={onOpenChat && (() => onOpenChat(plugin, f))} />
            {unreadOf && unreadOf(plugin, f.id) > 0 && (
              <span aria-label={t('chat:row.unread', { count: unreadOf(plugin, f.id) })} className="rounded-full bg-danger px-1.5 text-hint font-bold tabular-nums text-on-accent">
                {unreadOf(plugin, f.id)}
              </span>
            )}
          </li>
        ))}
      </ul>
      {friends.length > shown && (
        <LinkButton onClick={() => setShown((n) => n + ROWS_PER_GROUP)}>
          {t('chat:group.showMore', { count: Math.min(ROWS_PER_GROUP, friends.length - shown) })}
        </LinkButton>
      )}
    </div>
  );
}

function FriendRowMain({ friend: f, plugin, logoUrl, onOpenChat }: { friend: SocialFriend; plugin: string; logoUrl?: string; onOpenChat?: () => void }) {
  const { t } = useTranslation('chat');
  const body = (
    <>
      <PanelAvatar name={f.name} url={f.avatarUrl} dot={tokenDot(f.state)} />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[13px] font-semibold text-text">{f.name}</span>
          <PluginLogo src={logoUrl} name={plugin} size="glyph" />
        </div>
        <div className="truncate text-hint text-text-muted">
          {f.game ? (
            <span className="text-accent">{t('chat:row.playing', { title: f.game.title })}</span>
          ) : f.state === 'offline' || f.state === 'invisible' ? (f.lastSeen ? t('chat:row.lastSeen', { when: agoLabel(f.lastSeen) }) : t('chat:row.offline')) : friendStateLabel(f.state)}
        </div>
      </div>
    </>
  );
  if (!onOpenChat) return <div className="flex min-w-0 flex-1 items-center gap-3">{body}</div>;
  return (
    <button type="button" onClick={onOpenChat} className="flex min-w-0 flex-1 items-center gap-3 rounded-md text-left outline-none focus-visible:ring-2 focus-visible:ring-focus">
      {body}
    </button>
  );
}
