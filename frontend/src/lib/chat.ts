import type { ChatMessage, ChatView } from '../types';

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

/** Unread messages from one friend on one store. */
export function unreadFor(list: ChatView[], plugin: string, friendId: string): number {
  return list.find((v) => v.plugin === plugin && v.friendId === friendId)?.unread ?? 0;
}

/** Unread messages across every conversation: the sidebar's badge. */
export function unreadTotal(list: ChatView[]): number {
  return list.reduce((sum, v) => sum + v.unread, 0);
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

/** Local time of day, HH:MM, for a message's sentAt (Unix milliseconds). */
export function clockLabel(sentAt: number): string {
  const d = new Date(sentAt);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}
