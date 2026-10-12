import { useCallback, useEffect, useRef, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi, onChat } from '../effect/runtime';
import { notifyChat, onChatOpen } from '../effect/bridge';
import { alertText, chatKey, chatNotifyTitle, clipNotifyText, dropConversation, notifyTextOf, unreadFor, unreadTotal, upsertConversation } from './chat';
import { createAlertCoalescer } from './chatAlerts';
import type { ChatView, SocialFriend, SocialView } from '../types';

/** The conversation on screen: a store and a friend. */
export interface ChatTarget {
  plugin: string;
  friendId: string;
}

export interface ChatState {
  /** The conversation on screen, if any. */
  target: ChatTarget | null;
  conversation: ChatView | undefined;
  /** Why the last open, page or send failed; cleared by the next one. */
  error: string | null;
  /** Message alerts per store that offers chat. */
  alerts: Record<string, boolean>;
  /** Unread messages from friends across every store: the sidebar's badge. */
  unread: number;
  unreadOf: (plugin: string, friendId: string) => number;
  open: (plugin: string, friendId: string) => void;
  close: () => void;
  loadOlder: () => void;
  send: (text: string) => void;
  resend: (clientId: string) => void;
  setAlerts: (plugin: string, on: boolean) => void;
}

/** A friend's name from the friends lists, or the id when no list has them. */
export function friendNameOf(social: SocialView[], plugin: string, friendId: string): string {
  const f: SocialFriend | undefined = social.find((v) => v.plugin === plugin)?.friends.find((x) => x.id === friendId);
  return f?.name ?? 'a friend';
}

const messageOf = (err: unknown) => (err instanceof Error ? err.message : String(err));

/** The chat state of the launcher: the conversations the backend reports, the one on screen, alerts,
 *  and the unread badges. A new message the user should hear about raises a toast with the friend's
 *  name and the store's (never the text). `storeNames` maps a plugin id to its name; the id is the fallback. */
export function useChat(
  social: SocialView[],
  notify: (msg: string, kind?: 'ok' | 'err') => void,
  storeNames: Record<string, string> = {},
): ChatState {
  const [conversations, setConversations] = useState<ChatView[]>([]);
  const [alerts, setAlertsState] = useState<Record<string, boolean>>({});
  const [target, setTarget] = useState<ChatTarget | null>(null);
  const [error, setError] = useState<string | null>(null);

  // The subscription lives for the window's life, so it reads the latest friends and notify through refs.
  const socialRef = useRef(social);
  const notifyRef = useRef(notify);
  const storeNamesRef = useRef(storeNames);
  const targetRef = useRef<ChatTarget | null>(null);
  useEffect(() => {
    socialRef.current = social;
    notifyRef.current = notify;
    storeNamesRef.current = storeNames;
  }, [social, notify, storeNames]);

  useEffect(() => {
    // A store's new messages within a few seconds become one toast.
    const alerts = createAlertCoalescer((plugin, batch) => {
      const store = storeNamesRef.current[plugin] ?? plugin;
      notifyRef.current(alertText(batch.count, batch.names, store));
    });
    const unsubscribe = onChat((e) => {
      if (e.dropped) {
        setConversations((list) => dropConversation(list, e));
        return;
      }
      setConversations((list) => upsertConversation(list, e));
      if (!e.alert) return;
      alerts.add(e.plugin, friendNameOf(socialRef.current, e.plugin, e.friendId));
      // A desktop notification when the window isn't focused (issue #12): the
      // shell decides (focus, the launcher switch, coalescing). The payload
      // carries display names and the message text only, never ids.
      const text = notifyTextOf(e.messages);
      if (text === null) return;
      const name = friendNameOf(socialRef.current, e.plugin, e.friendId);
      const store = storeNamesRef.current[e.plugin] ?? e.plugin;
      void notifyChat({
        plugin: e.plugin,
        friendId: e.friendId,
        title: chatNotifyTitle(name, store),
        name,
        body: clipNotifyText(text),
        open: targetRef.current,
      });
    });
    void Effect.runPromise(LauncherApi.chatUnread()).then(
      (vs) => setConversations((list) => vs.reduce(upsertConversation, list)),
      () => {},
    );
    void Effect.runPromise(LauncherApi.chatAlerts()).then(setAlertsState, () => {});
    return () => {
      unsubscribe();
      alerts.dispose();
    };
  }, []);

  const apply = useCallback((view: ChatView) => setConversations((list) => upsertConversation(list, view)), []);

  const open = useCallback(
    (plugin: string, friendId: string) => {
      const next = { plugin, friendId };
      // Opening another friend closes the one on screen: the backend keeps it open (and its new
      // messages read) until it is told otherwise.
      const previous = targetRef.current;
      if (previous && (previous.plugin !== plugin || previous.friendId !== friendId)) {
        void Effect.runPromise(LauncherApi.chatClose(previous.plugin, previous.friendId)).catch(() => {});
      }
      targetRef.current = next;
      setTarget(next);
      setError(null);
      Effect.runPromise(LauncherApi.chatOpen(plugin, friendId)).then(
        (view) => {
          apply(view);
          // The user closed the conversation while it was opening: close it in the backend too.
          if (targetRef.current?.plugin !== plugin || targetRef.current?.friendId !== friendId) {
            void Effect.runPromise(LauncherApi.chatClose(plugin, friendId)).catch(() => {});
          }
        },
        (err) => setError(messageOf(err)),
      );
    },
    [apply],
  );

  const close = useCallback(() => {
    const current = targetRef.current;
    targetRef.current = null;
    setTarget(null);
    setError(null);
    if (current) void Effect.runPromise(LauncherApi.chatClose(current.plugin, current.friendId)).catch(() => {});
  }, []);

  const loadOlder = useCallback(() => {
    const current = targetRef.current;
    if (!current) return;
    Effect.runPromise(LauncherApi.chatOlder(current.plugin, current.friendId)).then(apply, (err) => setError(messageOf(err)));
  }, [apply]);

  // A send that the store refuses is also on its message ("Not sent", send again), so the error
  // here only says why; a refused send that never reached the store shows only here.
  const send = useCallback((text: string) => {
    const current = targetRef.current;
    if (!current) return;
    setError(null);
    Effect.runPromise(LauncherApi.chatSend(current.plugin, current.friendId, text)).then(
      () => {},
      (err) => setError(messageOf(err)),
    );
    // The new message arrives as an event; the view is refreshed then.
  }, []);

  const resend = useCallback((clientId: string) => {
    const current = targetRef.current;
    if (!current) return;
    setError(null);
    Effect.runPromise(LauncherApi.chatResend(current.plugin, current.friendId, clientId)).then(
      () => {},
      (err) => setError(messageOf(err)),
    );
  }, []);

  const setAlerts = useCallback((plugin: string, on: boolean) => {
    Effect.runPromise(LauncherApi.setChatAlerts(plugin, on)).then(
      () => setAlertsState((a) => ({ ...a, [plugin]: on })),
      (err) => notifyRef.current(messageOf(err), 'err'),
    );
  }, []);

  // A clicked desktop notification opens its conversation (issue #12): the
  // shell focuses the window and sends the target here.
  useEffect(() => {
    return onChatOpen((t) => {
      if (typeof t.plugin === 'string' && t.plugin !== '' && typeof t.friendId === 'string' && t.friendId !== '') {
        open(t.plugin, t.friendId);
      }
    });
  }, [open ]);

  const conversation = target ? conversations.find((v) => chatKey(v.plugin, v.friendId) === chatKey(target.plugin, target.friendId)) : undefined;

  return {
    target,
    conversation,
    error,
    alerts,
    unread: unreadTotal(conversations),
    unreadOf: (plugin, friendId) => unreadFor(conversations, plugin, friendId),
    open,
    close,
    loadOlder,
    send,
    resend,
    setAlerts,
  };
}
