// i18n: converted
// The /22 friends side panel (issue #135, D22 `.social`): the FriendsModal
// content (friends, chat, families) as a panel right of the stage, in the /22
// look (a MeCard, tabs for Friends/Chats/Family, Playing/Online/Offline
// groups). Atlas only; other themes keep the modal. Escape backs out of the
// open chat first, then closes the panel.
import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Bell, BellOff, Check, ChevronDown, ChevronLeft, MessageSquare, Search, Send, WifiOff, X } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from './ds/dropdown-menu';
import { Input } from './ds/input';
import { ExternalLink } from './ExternalLink';
import { FamiliesPanel } from './FamiliesPanel';
import { canSendChat, chatRows, clockLabel, messageKey, splitLinks } from '../lib/chat';
import { formatRelativeAge } from '../lib/format';
import { LinkButton, cx, EmptyState } from './ui';
import { PluginLogo } from './PluginLogo';
import {
  avatarHueStyle,
  canSetStatus,
  connectionLine,
  friendStateLabel,
  groupFriends,
  statusLabel,
  SOCIAL_STATUSES,
} from '../lib/social';
import type { ChatMessage, ChatView, SocialFriend, SocialStatus, SocialView, StorePlugin } from '../types';

export interface AtlasChatTarget {
  plugin: string;
  friendId: string;
}

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
  /** Every conversation the backend holds, read or unread (the Chats tab). */
  conversations?: ChatView[];
  /** The open conversation's store and friend (the Chats tab's highlight). */
  chatTarget?: AtlasChatTarget | null;
  /** The running game's title, if any (the MeCard's "Playing" line). */
  playingTitle?: string | null;
}

const ROWS_PER_GROUP = 200;

/** Dot colour for a friend or status, from the theme tokens only. A friend in
 *  a game reads blue; offline and invisible stay hollow. */
function presenceDot(state: string, inGame: boolean): string {
  if (inGame) return 'bg-info';
  switch (state) {
    case 'online':
    case 'looking':
      return 'bg-ok';
    case 'away':
    case 'busy':
    case 'snooze':
      return 'bg-attention';
    case 'offline':
    case 'invisible':
      return 'bg-transparent shadow-[inset_0_0_0_2px_var(--color-text-faint)]';
    default:
      return 'bg-text-faint';
  }
}

function PanelAvatar({ name, url, dot, size = 28 }: { name: string; url?: string; dot: string; size?: number }) {
  return (
    <span className="relative flex-none" style={{ width: size, height: size }}>
      <span
        className="grid h-full w-full place-items-center overflow-hidden rounded-full font-[750] text-white"
        style={{ ...avatarHueStyle(name), fontSize: size * 0.42 }}
      >
        {url ? <img src={url} alt="" className="h-full w-full object-cover" /> : (name.trim()[0] ?? '?').toUpperCase()}
      </span>
      <span className={cx('absolute -bottom-px -right-px h-2.5 w-2.5 rounded-full ring-2 ring-ground-2', dot)} />
    </span>
  );
}

/** The friend's sub-line: what they play, when they were last online, or their state. */
function FriendSub({ friend: f }: { friend: SocialFriend }) {
  const { t } = useTranslation('chat');
  if (f.game) return <span className="text-info">{t('chat:row.playing', { title: f.game.title })}</span>;
  if (f.state === 'offline' || f.state === 'invisible') {
    return <>{f.lastSeen ? t('chat:row.lastOnline', { ago: formatRelativeAge(f.lastSeen * 1000) }) : t('chat:row.offline')}</>;
  }
  return <>{friendStateLabel(f.state)}</>;
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
      if (document.querySelector('[role="dialog"]')) return;
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
    <aside aria-label={t('chat:modal.title')} data-controller-layer="friends" className="flex min-h-0 w-[340px] flex-none flex-col border-l border-hairline bg-ground-2">
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
        <div className="flex min-h-0 flex-1 flex-col">{p.chat}</div>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col">
          <MeCard
            views={p.views}
            labels={p.labels}
            onSetStatus={p.onSetStatus}
            onSetPlaying={p.onSetPlaying}
            chatAlerts={p.chatAlerts}
            onSetChatAlerts={p.onSetChatAlerts}
            playingTitle={p.playingTitle}
          />
          <div role="tablist" aria-label={t('chat:modal.title')} className="mx-2.5 mt-2 flex flex-none gap-0.5 rounded-[9px] bg-surface p-[3px]">
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
                  'h-7 flex-1 rounded-[7px] text-[13px] font-semibold text-text-muted transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                  tab === id && 'bg-[color-mix(in_srgb,var(--color-accent)_20%,var(--color-surface-strong))] text-text ring-1 ring-inset ring-accent/40'
                )}
              >
                {label}
              </button>
            ))}
          </div>
          {tab === 'friends' && (
            <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-2.5 py-3">
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
            <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-2.5 py-3">
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

// The current user above the tabs (D22 MeCard): a trigger button with a
// chevron menu for the status (the lab has no separate Select). The popover
// also holds the "what I'm playing" and message-alert check rows.
function MeCard({ views, labels, onSetStatus, onSetPlaying, chatAlerts, onSetChatAlerts, playingTitle }: Pick<AtlasFriendsPanelProps, 'views' | 'labels' | 'onSetStatus' | 'onSetPlaying' | 'chatAlerts' | 'onSetChatAlerts' | 'playingTitle'>) {
  const { t } = useTranslation('chat');
  const view = views.find((v) => v.enabled && v.self) ?? views.find((v) => v.self);
  if (!view?.self) return null;
  const label = labels[view.plugin] ?? view.plugin;
  const showAlerts = chatAlerts && onSetChatAlerts && view.plugin in chatAlerts;
  return (
    <div className="flex-none px-2.5 pb-3">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t('chat:modal.statusLabel', { label })}
            disabled={!canSetStatus(view)}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
          >
            <PanelAvatar name={view.self.name} url={view.self.avatarUrl} dot={presenceDot(view.self.state, false)} />
            <span className="grid min-w-0 flex-1">
              <span className="truncate font-[650] text-text">{view.self.name}</span>
              <span className="truncate text-[12px] text-text-faint">
                {view.showPlaying && playingTitle ? t('chat:row.playing', { title: playingTitle }) : statusLabel(view.self.state)}
              </span>
            </span>
            <ChevronDown className="size-[13px] flex-none text-text-muted" aria-hidden />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-60">
          {SOCIAL_STATUSES.map((s) => (
            <DropdownMenuItem key={s} onSelect={() => onSetStatus(view.plugin, s)}>
              <span className={cx('size-2.5 flex-none rounded-full', presenceDot(s, false))} aria-hidden />
              <span>{statusLabel(s)}</span>
              {view.self.state === s && <Check className="ml-auto size-[13px] text-accent" aria-hidden />}
            </DropdownMenuItem>
          ))}
          <DropdownMenuSeparator />
          <DropdownMenuItem onSelect={() => onSetPlaying(view.plugin, !view.showPlaying)}>
            <span
              aria-hidden
              className={cx(
                'grid size-4 flex-none place-items-center rounded-[4px]',
                view.showPlaying ? 'bg-accent text-on-accent' : 'text-transparent ring-1 ring-inset ring-hairline-strong'
              )}
            >
              <Check className="size-3" />
            </span>
            <span>{t('chat:modal.showPlayingMenu')}</span>
          </DropdownMenuItem>
          {showAlerts && (
            <DropdownMenuItem onSelect={() => onSetChatAlerts!(view.plugin, !chatAlerts![view.plugin])}>
              <span
                aria-hidden
                className={cx(
                  'grid size-4 flex-none place-items-center rounded-[4px]',
                  chatAlerts![view.plugin] ? 'bg-accent text-on-accent' : 'text-transparent ring-1 ring-inset ring-hairline-strong'
                )}
              >
                <Check className="size-3" />
              </span>
              <span>{t('chat:modal.chatAlertsMenu')}</span>
            </DropdownMenuItem>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

type SectionProps = AtlasFriendsPanelProps & { view: SocialView; label: string; query: string; hideHeader?: boolean };

function StoreSection({ view, label, query, hideHeader, plugins, onSetEnabled, onReconnect, unreadOf, onOpenChat, chatTarget }: SectionProps) {
  const { t } = useTranslation('chat');
  const line = connectionLine(view.connection);
  const groups = useMemo(() => groupFriends(view.friends, query), [view.friends, query]);
  const total = groups.inGame.length + groups.online.length + groups.offline.length;
  const canReconnect = view.connection.state === 'replaced' || view.connection.state === 'loggedOut';
  const logoUrl = useMemo(() => plugins.find((pl) => pl.id === view.plugin)?.icon_url ?? undefined, [plugins, view.plugin]);
  if (!view.enabled) {
    return (
      <section aria-label={t('chat:modal.section', { label })} className="flex flex-col gap-2.5">
        {!hideHeader && (
          <header>
            <h3 className="text-[12px] font-bold text-text-faint">{label}</h3>
          </header>
        )}
        <div role="status" className="mt-2 flex items-center gap-2 rounded-lg bg-surface px-2.5 py-2 text-[12.5px] text-text-muted">
          <PluginLogo src={logoUrl} name={view.plugin} size="glyph" />
          <span className="flex-1">{t('chat:modal.friendsOff', { label })}</span>
          <button type="button" onClick={() => onSetEnabled(view.plugin, true)} className="flex-none font-semibold text-accent">
            {t('chat:modal.turnOn')}
          </button>
        </div>
      </section>
    );
  }
  return (
    <section aria-label={t('chat:modal.section', { label })} className="flex flex-col gap-2.5">
      {!hideHeader && (
        <header>
          <h3 className="text-[12px] font-bold text-text-faint">{label}</h3>
        </header>
      )}
      {total === 0 && (
        <p className="text-[12.5px] text-text-muted">
          {view.friends.length === 0 ? t('chat:modal.noFriends', { label }) : t('chat:modal.noMatch', { query: query.trim() })}
        </p>
      )}

      <FriendGroup title={t('chat:group.playing')} friends={groups.inGame} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} chatTarget={chatTarget} />
      <FriendGroup title={t('chat:group.online')} friends={groups.online} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} chatTarget={chatTarget} />
      <FriendGroup title={t('chat:group.offline')} friends={groups.offline} plugin={view.plugin} logoUrl={logoUrl} unreadOf={unreadOf} onOpenChat={onOpenChat} chatTarget={chatTarget} />

      {/* The store state lives below the groups (the lab's notices row): a
          notice only when the store is off or disconnected. */}
      {line && (
        <div role="status" className="mt-2 flex items-center gap-2 rounded-lg bg-surface px-2.5 py-2 text-[12.5px] text-text-muted">
          <WifiOff className="h-3 w-3 flex-none text-attention" aria-hidden />
          <span className="flex-1">{line}</span>
          {canReconnect && (
            <button type="button" onClick={() => onReconnect(view.plugin)} className="flex-none font-semibold text-accent">
              {t('chat:modal.reconnect')}
            </button>
          )}
        </div>
      )}
    </section>
  );
}

// Every held conversation across the stores, read or unread, newest first:
// each row opens its chat. Opening a chat marks it read, so the row stays.
function ChatsTab(p: AtlasFriendsPanelProps) {
  const { t } = useTranslation('chat');
  const rows = useMemo(() => {
    const convs = chatRows(p.conversations ?? []);
    return convs.flatMap((conv) => {
      const view = p.views.find((v) => v.plugin === conv.plugin);
      const friend = view?.friends.find((f) => f.id === conv.friendId);
      return view && friend ? [{ view, friend, conv }] : [];
    });
  }, [p.conversations, p.views]);
  if (rows.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-2.5 py-3">
        <EmptyState icon={MessageSquare} label={t('chat:tabs.empty')}>
          {t('chat:tabs.emptyBody')}
        </EmptyState>
      </div>
    );
  }
  return (
    <ul className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 py-3">
      {rows.map(({ view, friend: f, conv }) => {
        const last = conv.messages[conv.messages.length - 1];
        const isOn = p.chatTarget?.plugin === view.plugin && p.chatTarget?.friendId === f.id;
        return (
          <li key={`${view.plugin}:${f.id}`} className={cx((f.state === 'offline' || f.state === 'invisible') && 'opacity-60')}>
            <button
              type="button"
              onClick={() => p.onOpenChat?.(view.plugin, f)}
              className={cx(
                'flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                isOn && 'bg-[color-mix(in_srgb,var(--color-accent)_14%,var(--color-surface))]'
              )}
            >
              <PanelAvatar name={f.name} url={f.avatarUrl} dot={presenceDot(f.state, !!f.game)} />
              <span className="grid min-w-0 flex-1">
                <span className="truncate text-[13px] font-[650] text-text">{f.name}</span>
                <span className="truncate text-[12px] text-text-faint">
                  {last ? last.text : t('chat:tabs.newMessages', { count: conv.unread })}
                </span>
              </span>
              {conv.unread > 0 && (
                <span aria-label={t('chat:row.unread', { count: conv.unread })} className="rounded-full bg-danger px-1.5 text-hint font-bold tabular-nums text-on-accent">
                  {conv.unread}
                </span>
              )}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function FriendGroup({ title, friends, plugin, logoUrl, unreadOf, onOpenChat, chatTarget }: {
  title: string;
  friends: SocialFriend[];
  plugin: string;
  logoUrl?: string;
  unreadOf?: (plugin: string, friendId: string) => number;
  onOpenChat?: (plugin: string, friend: SocialFriend) => void;
  chatTarget?: AtlasChatTarget | null;
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
        {friends.slice(0, shown).map((f) => {
          const unread = unreadOf?.(plugin, f.id) ?? 0;
          const isOn = chatTarget?.plugin === plugin && chatTarget?.friendId === f.id;
          const row = (
            <>
              <PanelAvatar name={f.name} url={f.avatarUrl} dot={presenceDot(f.state, !!f.game)} />
              <span className="grid min-w-0 flex-1">
                <span className="flex min-w-0 items-center gap-1.5">
                  <span className="truncate text-[13px] font-[650] text-text">{f.name}</span>
                  <PluginLogo src={logoUrl} name={plugin} size="glyph" />
                </span>
                <span className="truncate text-[12px] text-text-faint">
                  <FriendSub friend={f} />
                </span>
              </span>
              {unread > 0 && (
                <span aria-label={t('chat:row.unread', { count: unread })} className="rounded-full bg-danger px-1.5 text-hint font-bold tabular-nums text-on-accent">
                  {unread}
                </span>
              )}
            </>
          );
          return (
            <li key={f.id} className={cx((f.state === 'offline' || f.state === 'invisible') && 'opacity-60')}>
              {onOpenChat ? (
                <button
                  type="button"
                  onClick={() => onOpenChat(plugin, f)}
                  className={cx(
                    'flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus',
                    isOn && 'bg-[color-mix(in_srgb,var(--color-accent)_14%,var(--color-surface))]'
                  )}
                >
                  {row}
                </button>
              ) : (
                <div className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5">{row}</div>
              )}
            </li>
          );
        })}
      </ul>
      {friends.length > shown && (
        <LinkButton onClick={() => setShown((n) => n + ROWS_PER_GROUP)}>
          {t('chat:group.showMore', { count: Math.min(ROWS_PER_GROUP, friends.length - shown) })}
        </LinkButton>
      )}
    </div>
  );
}

/** One conversation with a friend under Atlas (the lab `ChatView`): the header
 *  with the avatar, presence, View game and the alerts bell; the message log;
 *  a one-row composer. Other themes keep the generic `ChatPanel`. Enter sends;
 *  Shift+Enter starts a new line. Links open in the browser only when they are
 *  http or https; everything else is plain text. */
export function AtlasChatView(p: {
  /** The conversation as the backend last reported it; undefined until the first answer. */
  view?: ChatView;
  /** The friend, when the friends list knows them. */
  friend?: SocialFriend;
  /** The friend's name (the friends list's, or the unknown-friend fallback). */
  friendName: string;
  /** The store plugin id. */
  plugin: string;
  /** The store glyph beside the name. */
  logoUrl?: string;
  storeName: string;
  /** Whether the store offers chat to plugins. */
  canChat: boolean;
  /** Whether a Platypus game key is in the library (a friend's game links there). */
  inLibrary: (gameId: string) => boolean;
  onOpenGame: (gameId: string) => void;
  /** Whether this store's message alerts are on. */
  chatAlerts?: Record<string, boolean>;
  onSetChatAlerts?: (plugin: string, on: boolean) => void;
  /** Why the conversation couldn't load or a request failed, shown above the messages. */
  error?: string | null;
  onLoadOlder: () => void;
  onSend: (text: string) => void;
  onResend: (clientId: string) => void;
  /** Back to the friends list. */
  onBack: () => void;
}) {
  const { t } = useTranslation('chat');
  const [text, setText] = useState('');
  const log = useRef<HTMLDivElement>(null);
  // Whether the log is at its newest end as the user left it: only then does a new message follow it down.
  const atEnd = useRef(true);
  // The ends of the messages and the log's height when it last settled, to tell older messages (added
  // above) from new ones (added below).
  const settled = useRef<{ first: string; last: string; height: number } | null>(null);

  const messages = p.view?.messages ?? [];

  // Keep the newest message in view while the user is at the bottom; when older messages come in above,
  // keep the messages the user is reading where they were.
  useLayoutEffect(() => {
    const el = log.current;
    if (!el) return;
    const first = messages.length ? messageKey(messages[0]) : '';
    const last = messages.length ? messageKey(messages[messages.length - 1]) : '';
    const before = settled.current;
    if (before && before.first !== '' && first !== before.first && last === before.last) {
      el.scrollTop += el.scrollHeight - before.height;
    } else if (atEnd.current) {
      el.scrollTop = el.scrollHeight;
    }
    settled.current = { first, last, height: el.scrollHeight };
  }, [p.view]);

  const submit = (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!canSendChat(text)) return;
    // What the user sends is theirs to see: the log follows it down.
    atEnd.current = true;
    p.onSend(text);
    setText('');
  };

  const alertsOn = p.chatAlerts?.[p.plugin] ?? false;
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <header className="flex flex-none items-center gap-2.5 border-b border-hairline px-3 py-2.5">
        <button
          type="button"
          onClick={p.onBack}
          aria-label={t('chat:panel.back')}
          className="grid size-8 flex-none place-items-center rounded-lg text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
        >
          <ChevronLeft className="size-4" aria-hidden />
        </button>
        <PanelAvatar name={p.friendName} url={p.friend?.avatarUrl} dot={p.friend ? presenceDot(p.friend.state, !!p.friend.game) : 'bg-text-faint'} size={30} />
        <span className="grid min-w-0 flex-1">
          <span className="flex min-w-0 items-center gap-1.5">
            <span className="truncate font-[650] text-text">{p.friendName}</span>
            <PluginLogo src={p.logoUrl} name={p.plugin} size="glyph" />
          </span>
          <span className="truncate text-[12px] text-text-faint">
            {p.friend && <FriendSub friend={p.friend} />}
          </span>
        </span>
        {p.friend?.game?.id && p.inLibrary(p.friend.game.id) && (
          <button
            type="button"
            onClick={() => p.friend?.game?.id && p.onOpenGame(p.friend.game.id)}
            className="flex-none rounded-lg px-2 py-1 text-[13px] font-semibold text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            {t('chat:panel.viewGame')}
          </button>
        )}
        {p.chatAlerts && p.onSetChatAlerts && p.plugin in p.chatAlerts && (
          <button
            type="button"
            onClick={() => p.onSetChatAlerts!(p.plugin, !alertsOn)}
            aria-label={t(alertsOn ? 'chat:panel.muteAlerts' : 'chat:panel.unmuteAlerts')}
            aria-pressed={alertsOn}
            className="grid size-8 flex-none place-items-center rounded-lg text-text-muted transition hover:bg-surface hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            {alertsOn ? <Bell className="size-[14px]" aria-hidden /> : <BellOff className="size-[14px]" aria-hidden />}
          </button>
        )}
      </header>

      <div
        ref={log}
        role="log"
        aria-label={t('chat:panel.logLabel', { name: p.friendName })}
        onScroll={(e) => {
          const el = e.currentTarget;
          atEnd.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }}
        className="flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto p-3.5"
      >
        {p.view?.more && (
          <button
            type="button"
            onClick={p.onLoadOlder}
            className="self-center rounded-xl bg-surface px-2.5 py-1 text-[12px] text-text-muted transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
          >
            {t('chat:panel.loadOlder')}
          </button>
        )}
        {p.error && (
          <p role="alert" className="rounded-lg bg-danger/10 px-3 py-2 text-[12px] text-danger ring-1 ring-danger/25">
            {p.error}
          </p>
        )}
        {!p.view?.loaded && !p.error && <p className="text-[12.5px] text-text-muted">{t('chat:panel.loading')}</p>}
        {p.view?.loaded && messages.length === 0 && !p.error && (
          <p className="m-auto text-text-muted">{t('chat:panel.sayHi', { name: p.friendName })}</p>
        )}
        {messages.map((m) => (
          <AtlasMessageRow key={messageKey(m)} message={m} onResend={p.onResend} />
        ))}
      </div>

      {p.view?.typing && <p className="px-3.5 text-[12px] italic text-text-faint">{t('chat:panel.typing', { name: p.friendName })}</p>}

      {p.canChat ? (
        <form onSubmit={submit} className="flex flex-none gap-1.5 border-t border-hairline p-2.5">
          <textarea
            rows={1}
            value={text}
            aria-label={t('chat:panel.messageLabel', { name: p.friendName })}
            placeholder={t('chat:panel.messageLabel', { name: p.friendName })}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault();
                submit();
              }
            }}
            className="h-[34px] flex-1 resize-none rounded-[17px] bg-surface px-3 py-[7px] text-text outline-none placeholder:text-text-faint focus:ring-[1.5px] focus:ring-inset focus:ring-accent"
          />
          <button
            type="submit"
            aria-label={t('chat:panel.send')}
            disabled={!canSendChat(text)}
            className="grid size-8 flex-none place-items-center rounded-lg text-text-muted transition hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus disabled:opacity-40"
          >
            <Send className="size-[15px]" aria-hidden />
          </button>
        </form>
      ) : (
        <p className="border-t border-hairline p-3 text-[12.5px] text-text-faint">{t('chat:panel.noChat', { store: p.storeName })}</p>
      )}
    </div>
  );
}

function AtlasMessageRow({ message, onResend }: {
  message: ChatMessage;
  onResend: (clientId: string) => void;
}) {
  const { t } = useTranslation('chat');
  const mine = message.from === 'self';
  const failed = message.status === 'failed';
  return (
    <div className={cx('grid max-w-[78%] justify-items-start gap-0.5', mine && 'self-end justify-items-end')}>
      <div
        className={cx(
          'rounded-[14px] px-[11px] py-[7px] leading-[1.4] [overflow-wrap:anywhere]',
          mine ? 'bg-[color-mix(in_srgb,var(--color-accent)_30%,var(--color-surface-strong))] text-text' : 'bg-surface text-text',
          failed && 'opacity-75 ring-1 ring-inset ring-danger'
        )}
      >
        {splitLinks(message.text).map((part, i) =>
          part.href ? (
            <ExternalLink key={i} href={part.href} className="underline decoration-hairline-strong underline-offset-2 hover:text-text">
              {part.text}
            </ExternalLink>
          ) : (
            <React.Fragment key={i}>{part.text}</React.Fragment>
          )
        )}
      </div>
      <div className="px-1.5 text-[11px] text-text-faint">
        {failed ? (
          <>
            <span className="text-danger">{t('chat:panel.sendFailed')}</span>{' '}
            {message.clientId && (
              <button type="button" onClick={() => onResend(message.clientId!)} className="font-semibold text-accent">
                {t('chat:panel.resend')}
              </button>
            )}
          </>
        ) : message.status === 'sending' ? (
          t('chat:panel.sending')
        ) : (
          clockLabel(message.sentAt)
        )}
      </div>
    </div>
  );
}
