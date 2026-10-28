import type { ChatMessage, ChatView } from '../types';
import i18n from './i18n';
import { formatList } from './format';

/** The most characters one message may hold (protocol 1.9). */
export const CHAT_MAX_TEXT = 4096;

/** Whether text can be sent: something other than blanks, and within the cap. Counts code points. */
export function canSendChat(text: string): boolean {
  return text.trim() !== '' && [...text].length <= CHAT_MAX_TEXT;
}

/** The key of one conversation in a list: a store and a friend. */
export function chatKey(plugin: string, friendId: string): string {
  return `${plugin}|${friendId}`;
}

/** Replaces (or adds) one conversation. A change older than the one held (a lower seq, arrived late)
 *  is ignored, like the friends views. */
export function upsertConversation(list: ChatView[], view: ChatView): ChatView[] {
  const key = chatKey(view.plugin, view.friendId);
  const i = list.findIndex((v) => chatKey(v.plugin, v.friendId) === key);
  if (i < 0) return [...list, view];
  if (list[i].seq > view.seq) return list;
  const next = list.slice();
  next[i] = view;
  return next;
}

/** Drops a conversation the backend evicted from memory, or, when the event
 *  names no friend, every conversation of that store: a logout cleared them,
 *  and the rows held here include previews the core sent once. A view held that
 *  is newer than the eviction (a later change of the same conversation) stays. */
export function dropConversation(list: ChatView[], e: Pick<ChatView, 'plugin' | 'friendId' | 'seq'>): ChatView[] {
  if (!e.friendId) return list.filter((v) => v.plugin !== e.plugin);
  const key = chatKey(e.plugin, e.friendId);
  return list.filter((v) => chatKey(v.plugin, v.friendId) !== key || v.seq > e.seq);
}

/** Unread messages from one friend on one store. */
export function unreadFor(list: ChatView[], plugin: string, friendId: string): number {
  return list.find((v) => v.plugin === plugin && v.friendId === friendId)?.unread ?? 0;
}

/** Unread messages across every conversation: the sidebar's badge. */
export function unreadTotal(list: ChatView[]): number {
  return list.reduce((sum, v) => sum + v.unread, 0);
}

/** The conversations the Chats tab lists: every held conversation with
 *  something to show (a message or an unread count), newest first by the last
 *  message's sentAt, falling back to seq when no message is held. */
export function chatRows(list: ChatView[]): ChatView[] {
  return list
    .filter((v) => v.messages.length > 0 || v.unread > 0)
    .sort((a, b) => {
      const aLast = a.messages[a.messages.length - 1];
      const bLast = b.messages[b.messages.length - 1];
      return (bLast ? bLast.sentAt : b.seq) - (aLast ? aLast.sentAt : a.seq);
    });
}

/** A message's key on screen: the clientId while it is in flight, the id once confirmed. */
export function messageKey(m: ChatMessage): string {
  return m.clientId ?? m.id;
}

export interface TextPart {
  text: string;
  /** Set for an http or https link: the only kind that opens, in the browser. */
  href?: string;
}

const LINK = /https?:\/\/[^\s<>"']+/gi;

/** Splits message text into plain parts and http(s) links. Text is never HTML: a link is only text
 *  with an address, and a trailing full stop or comma stays outside it. */
export function splitLinks(text: string): TextPart[] {
  const parts: TextPart[] = [];
  let last = 0;
  for (const m of text.matchAll(LINK)) {
    const start = m.index ?? 0;
    const url = m[0].replace(/[.,;:!?)]+$/, '');
    if (start > last) parts.push({ text: text.slice(last, start) });
    // A bare scheme ("http://") with nothing after it is text, not a link.
    parts.push(/^https?:\/\/./i.test(url) ? { text: url, href: url } : { text: url });
    last = start + url.length;
  }
  if (last < text.length) parts.push({ text: text.slice(last) });
  return parts;
}

/** The toast for the new messages of one store that arrived together: the one message names its friend,
 *  several name the friends they came from (never their text). */
export function alertText(count: number, names: string[], store: string): string {
  if (count <= 1) {
    return i18n.t('chat:alert', { count: 1, name: names[0] ?? i18n.t('chat:unknownFriend'), who: '', store });
  }
  const who = names.length <= 2
    ? formatList(names)
    : i18n.t('chat:alertOthers', { first: names[0], count: names.length - 1 });
  return i18n.t('chat:alert', { count, name: '', who, store });
}

/** The most message text a desktop notification carries, in characters (issue #12). */
export const NOTIFY_BODY_CHARS = 140;

/** Clips message text to what a desktop notification carries, with an ellipsis when cut. Counts code points. */
export function clipNotifyText(text: string): string {
  const points = [...text];
  if (points.length <= NOTIFY_BODY_CHARS) return text;
  return `${points.slice(0, NOTIFY_BODY_CHARS).join('')}…`;
}

/** The title of a chat notification: the friend's name with the store. Display names only, never ids. */
export function chatNotifyTitle(friendName: string, storeName: string): string {
  return `${friendName} · ${storeName}`;
}

/** The text a desktop notification shows for an alert event: the newest message from a friend, or null
 *  when the event carries none (then no notification goes out). Never ids, only the text. */
export function notifyTextOf(messages: ChatMessage[]): string | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (m.from !== 'self' && m.text.trim() !== '') return m.text;
  }
  return null;
}

/** Local time of day, HH:MM, for a message's sentAt (Unix milliseconds). */
export function clockLabel(sentAt: number): string {
  const d = new Date(sentAt);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}
