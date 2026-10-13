// Desktop notifications for chat (issue #12): whether a new message raises a
// system notification, and the per-conversation coalescing of bursts. Pure (no
// Electron imports) so node:test covers it; main.ts applies it to real
// Notifications.
//
// A new message the user should hear about reaches the shell as a request
// from the page (the renderer asks through `platypus:notify-chat`; the page
// itself has no notification power). Main shows it only when the window isn't
// focused, the switch is on, and the conversation isn't the one on screen.
// Bursts coalesce per conversation: at most one notification every windowMs,
// later messages replacing it ("3 new messages from X"). Clicking focuses the
// window and opens the conversation.

/** One message asking for a desktop notification. Title and body carry display
 *  names and the message text only: never tokens or ids. */
export interface ChatNotifyMessage {
  plugin: string;
  friendId: string;
  /** The friend's name with the store, e.g. "Ana Ruiz · Echo". */
  title: string;
  /** The friend's display name, for the coalesced "N new messages from X". */
  name: string;
  /** The message text, clipped to NOTIFY_BODY_CHARS. */
  body: string;
  /** The conversation on screen, if any. */
  open: { plugin: string; friendId: string } | null;
}

/** What the shell knows when deciding: focus, the launcher switch, and the
 *  conversation on screen. In-app toasts stay as they are when focused. */
export interface ChatNotifyContext {
  /** Whether any window is focused right now. */
  focused: boolean;
  /** The "Desktop notifications for chat" switch. */
  settingOn: boolean;
  /** The shell language for the coalesced count (issue #39); English when unset. */
  lang?: string;
}

/** The most message text a notification carries, in characters. */
export const NOTIFY_BODY_CHARS = 140;

/** How long one notification stands for its conversation: later messages
 *  within the window replace it instead of showing another one. */
export const NOTIFY_WINDOW_MS = 5000;

/** The most characters a title or name may hold; longer ones are refused. */
const MAX_TITLE = 200;
const MAX_NAME = 100;

/** Clips text to max characters (code points), with an ellipsis when cut. */
export function clipNotifyText(text: string, max: number = NOTIFY_BODY_CHARS): string {
  const points = Array.from(text);
  if (points.length <= max) return text;
  return points.slice(0, max).join('') + '…';
}

/** Whether the payload is shaped like a notification request: non-empty
 *  strings within their bounds. */
export function validNotifyMessage(raw: unknown): raw is ChatNotifyMessage {
  if (typeof raw !== 'object' || raw === null) return false;
  const m = raw as Record<string, unknown>;
  if (typeof m.plugin !== 'string' || m.plugin === '' || m.plugin.length > 64) return false;
  if (typeof m.friendId !== 'string' || m.friendId === '' || m.friendId.length > 64) return false;
  if (typeof m.title !== 'string' || m.title === '' || m.title.length > MAX_TITLE) return false;
  if (typeof m.name !== 'string' || m.name === '' || m.name.length > MAX_NAME) return false;
  if (typeof m.body !== 'string' || m.body === '') return false;
  const open = m.open as { plugin?: unknown; friendId?: unknown } | null | undefined;
  if (open !== null && open !== undefined) {
    if (typeof open !== 'object' || typeof open.plugin !== 'string' || typeof open.friendId !== 'string') return false;
  }
  return true;
}

/** Whether the message becomes a notification: only when unfocused, switched
 *  on, and about another conversation than the one on screen. */
export function shouldShowNotification(msg: ChatNotifyMessage, ctx: ChatNotifyContext): boolean {
  if (ctx.focused || !ctx.settingOn) return false;
  if (msg.open && msg.open.plugin === msg.plugin && msg.open.friendId === msg.friendId) return false;
  return true;
}

import { shellText } from './locale.ts';

/** The body of a coalesced notification: the message text for the first one,
 *  "N new messages from X" once more arrived within the window. The count
 *  follows the shell language's plural rules (issue #39). */
export function coalescedBody(name: string, count: number, text: string, lang = 'en'): string {
  if (count <= 1) return clipNotifyText(text);
  const rule = new Intl.PluralRules(lang).select(count);
  const key = rule === 'one' ? 'chatNewMessages_one' : 'chatNewMessages_other';
  return shellText(lang, key, { count, name });
}

export interface ShownNotification {
  close(): void;
}

export type NotifyOutcome = 'shown' | 'updated' | 'dropped';

/**
 * Coalesces chat notifications per conversation. notify drops what shouldn't
 * show (focused, switched off, the open conversation, malformed) and otherwise
 * shows at most one notification per conversation per window: the first message
 * shows its text, later ones within the window replace it with the count.
 */
export function createChatNotifier(deps: {
  show: (title: string, body: string, onClick: () => void) => ShownNotification;
  windowMs?: number;
}): { notify(msg: ChatNotifyMessage, ctx: ChatNotifyContext, onClick: () => void): NotifyOutcome; dispose(): void } {
  const windowMs = deps.windowMs ?? NOTIFY_WINDOW_MS;
  const active = new Map<string, { notification: ShownNotification; count: number; timer: ReturnType<typeof setTimeout> }>();
  const keyOf = (msg: ChatNotifyMessage) => `${msg.plugin}|${msg.friendId}`;

  const drop = (key: string): void => {
    const e = active.get(key);
    if (e) {
      clearTimeout(e.timer);
      active.delete(key);
    }
  };

  return {
    notify(msg, ctx, onClick): NotifyOutcome {
      if (!shouldShowNotification(msg, ctx)) return 'dropped';
      const key = keyOf(msg);
      const prev = active.get(key);
      const count = (prev?.count ?? 0) + 1;
      prev?.notification.close();
      if (prev) clearTimeout(prev.timer);
      const notification = deps.show(msg.title, coalescedBody(msg.name, count, msg.body, ctx.lang ?? 'en'), onClick);
      const timer = setTimeout(() => drop(key), windowMs);
      // A late timer must not drop a newer entry that reused the key.
      const entry = { notification, count, timer };
      active.set(key, entry);
      return prev ? 'updated' : 'shown';
    },
    dispose(): void {
      for (const e of active.values()) {
        clearTimeout(e.timer);
        e.notification.close();
      }
      active.clear();
    },
  };
}
