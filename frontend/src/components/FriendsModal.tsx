import React, { useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import { Modal } from './Modal';
import { cx, LinkButton } from './ui';
import { Button } from './ds/button';
import { Input } from './ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ds/select';
import { Switch } from './ds/switch';
import { agoLabel } from '../lib/cloud';
import { FamiliesPanel } from './FamiliesPanel';
import {
  canSetStatus,
  connectionLine,
  FRIEND_STATE_LABEL,
  groupFriends,
  initials,
  SOCIAL_STATUSES,
  stateDot,
  STATUS_LABEL,
} from '../lib/social';
import type { SocialFriend, SocialStatus, SocialView, StorePlugin } from '../types';

interface FriendsModalProps {
  views: SocialView[];
  /** Store names by plugin id. */
  labels: Record<string, string>;
  /** The store plugins, for the family of each store that offers it. */
  plugins: StorePlugin[];
  /** Whether a Hydra game key is in the library (a friend's game links there). */
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
  /** Whether a store's message alerts are on, for each store that offers chat. */
  chatAlerts?: Record<string, boolean>;
  onSetChatAlerts?: (plugin: string, on: boolean) => void;
}

/** Friends shown per group before "Show more", so a long list stays light. */
const ROWS_PER_GROUP = 200;

/** Every store's friends in one modal: a section per store, grouped In game /
 *  Online / Offline, with a search over all of them. */
export function FriendsModal(p: FriendsModalProps) {
  const [query, setQuery] = useState('');
  return (
    <Modal label="Friends" onClose={p.onClose} className="max-w-2xl">
      <div className="flex max-h-[80vh] flex-col gap-4 p-5">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-[17px] font-extrabold tracking-tight">Friends</h2>
          <label className="relative block w-64">
            <span className="sr-only">Search friends</span>
            <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-white/35" />
            <Input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search friends…"
              className="pl-8.5 font-sans"
            />
          </label>
        </div>

        <div className="flex flex-col gap-5 overflow-y-auto pr-1">
          {p.views.map((v) => (
            <StoreSection key={v.plugin} view={v} label={p.labels[v.plugin] ?? v.plugin} query={query} {...p} />
          ))}
        </div>
      </div>
    </Modal>
  );
}

type SectionProps = FriendsModalProps & { view: SocialView; label: string; query: string };

function StoreSection({ view, label, query, plugins, inLibrary, onSetEnabled, onSetStatus, onSetPlaying, onReconnect, onOpenGame, unreadOf, onOpenChat, chatAlerts, onSetChatAlerts }: SectionProps) {
  const line = connectionLine(view.connection);
  const storePlugin = plugins.find((x) => x.id === view.plugin);
  const groups = useMemo(() => groupFriends(view.friends, query), [view.friends, query]);
  const total = groups.inGame.length + groups.online.length + groups.offline.length;
  const canReconnect = view.connection.state === 'replaced' || view.connection.state === 'loggedOut';
  return (
    <section aria-label={`${label} friends`} className="flex flex-col gap-3 border-t border-white/[0.06] pt-4 first:border-t-0 first:pt-0">
      <header className="flex items-center justify-between gap-3">
        <h3 className="font-mono text-[11px] uppercase tracking-[0.18em] text-white/50">{label}</h3>
        <Button variant="ghost" size="sm" onClick={() => onSetEnabled(view.plugin, !view.enabled)}>
          {view.enabled ? 'Turn off' : 'Turn on'}
        </Button>
      </header>

      {!view.enabled ? (
        <p className="text-[12.5px] text-white/50">Friends are off for {label}. Turn them on to see who is online.</p>
      ) : (
        <>
          {line && (
            <div role="status" className="flex items-center justify-between gap-3 rounded-lg bg-amber-300/[0.08] px-3 py-2 text-[12px] text-amber-100 ring-1 ring-amber-300/25">
              <span>{line}</span>
              {canReconnect && (
                <Button variant="ghost" size="sm" onClick={() => onReconnect(view.plugin)}>
                  Reconnect
                </Button>
              )}
            </div>
          )}
          <label className="flex items-center justify-between gap-3 text-[12.5px] text-white/60">
            <span>Show what I&apos;m playing to friends</span>
            <Switch
              aria-label={`Show what I'm playing to friends on ${label}`}
              checked={view.showPlaying}
              onCheckedChange={(show) => onSetPlaying(view.plugin, show)}
            />
          </label>
          {chatAlerts && onSetChatAlerts && view.plugin in chatAlerts && (
            <label className="flex items-center justify-between gap-3 text-[12.5px] text-white/60">
              <span>Alert me to new messages from friends</span>
              <Switch
                aria-label={`Message alerts on ${label}`}
                checked={chatAlerts[view.plugin]}
                onCheckedChange={(on) => onSetChatAlerts(view.plugin, on)}
              />
            </label>
          )}
          {view.self && (
            <div className="flex items-center gap-3 rounded-xl bg-white/[0.04] px-3 py-2">
              <Avatar name={view.self.name} url={view.self.avatarUrl} dot={stateDot(view.self.state)} />
              <div className="min-w-0 flex-1">
                <div className="truncate text-[13px] font-semibold text-white/90">{view.self.name}</div>
              </div>
              <Select
                value={view.self.state}
                disabled={!canSetStatus(view)}
                onValueChange={(v) => onSetStatus(view.plugin, v as SocialStatus)}
              >
                <SelectTrigger aria-label={`Your status on ${label}`} className="w-36">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SOCIAL_STATUSES.map((s) => (
                    <SelectItem key={s} value={s}>{STATUS_LABEL[s]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {total === 0 && (
            <p className="text-[12.5px] text-white/50">
              {view.friends.length === 0 ? `No friends on ${label} yet.` : `No friends match “${query.trim()}”.`}
            </p>
          )}

          <FriendGroup title="In game" friends={groups.inGame} inLibrary={inLibrary} onOpenGame={onOpenGame} plugin={view.plugin} unreadOf={unreadOf} onOpenChat={onOpenChat} />
          <FriendGroup title="Online" friends={groups.online} inLibrary={inLibrary} onOpenGame={onOpenGame} plugin={view.plugin} unreadOf={unreadOf} onOpenChat={onOpenChat} />
          <FriendGroup title="Offline" friends={groups.offline} inLibrary={inLibrary} onOpenGame={onOpenGame} plugin={view.plugin} unreadOf={unreadOf} onOpenChat={onOpenChat} />
        </>
      )}

      {storePlugin && <FamiliesPanel plugin={storePlugin} />}
    </section>
  );
}

function FriendGroup({ title, friends, inLibrary, onOpenGame, plugin, unreadOf, onOpenChat }: {
  title: string;
  friends: SocialFriend[];
  inLibrary: (gameId: string) => boolean;
  onOpenGame: (gameId: string) => void;
  plugin: string;
  unreadOf?: (plugin: string, friendId: string) => number;
  onOpenChat?: (plugin: string, friend: SocialFriend) => void;
}) {
  const [shown, setShown] = useState(ROWS_PER_GROUP);
  if (friends.length === 0) return null;
  return (
    <div className="flex flex-col gap-1">
      <h4 className="px-1 font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/40">
        {title} · {friends.length}
      </h4>
      <ul className="flex flex-col">
        {friends.slice(0, shown).map((f) => (
          <li key={f.id} className="flex items-center gap-3 rounded-lg px-2 py-1.5 hover:bg-white/[0.03]">
            <Avatar name={f.name} url={f.avatarUrl} dot={stateDot(f.state)} />
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-semibold text-white/85">{f.name}</div>
              <div className="truncate text-[11.5px] text-white/45">
                {f.game ? (
                  <>
                    Playing {f.game.title}
                    {f.game.id && inLibrary(f.game.id) && (
                      <>
                        {' · '}
                        <LinkButton onClick={() => onOpenGame(f.game!.id!)}>open</LinkButton>
                      </>
                    )}
                  </>
                ) : f.state === 'offline' || f.state === 'invisible' ? (
                  f.lastSeen ? `Last seen ${agoLabel(f.lastSeen)}` : 'Offline'
                ) : (
                  FRIEND_STATE_LABEL[f.state]
                )}
              </div>
            </div>
            {unreadOf && unreadOf(plugin, f.id) > 0 && (
              <span aria-label={`${unreadOf(plugin, f.id)} unread messages`} className="rounded-full bg-amber-300 px-1.5 font-mono text-[10px] font-bold text-[#130f0a]">
                {unreadOf(plugin, f.id)}
              </span>
            )}
            {onOpenChat && (
              <Button variant="ghost" size="sm" aria-label={`Message ${f.name}`} onClick={() => onOpenChat(plugin, f)}>
                Message
              </Button>
            )}
          </li>
        ))}
      </ul>
      {friends.length > shown && (
        <LinkButton onClick={() => setShown((n) => n + ROWS_PER_GROUP)}>
          Show {Math.min(ROWS_PER_GROUP, friends.length - shown)} more
        </LinkButton>
      )}
    </div>
  );
}

function Avatar({ name, url, dot }: { name: string; url?: string; dot: string }) {
  return (
    // The dot sits outside the clipped picture: inside it, overflow-hidden would cut it.
    <span className="relative h-8 w-8 flex-none">
      <span className="grid h-full w-full place-items-center overflow-hidden rounded-full bg-white/10 font-mono text-[10px] font-bold text-white/70">
        {url ? <img src={url} alt="" className="h-full w-full object-cover" /> : initials(name)}
      </span>
      <span className={cx('absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full ring-2 ring-[#1a140f]', dot)} />
    </span>
  );
}
