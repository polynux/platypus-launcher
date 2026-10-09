import { describe, it, expect } from 'vitest';
import { alertText, canSendChat, chatKey, clockLabel, dropConversation, messageKey, splitLinks, unreadFor, unreadTotal, upsertConversation } from './chat';
import type { ChatView } from '../types';

const conv = (over: Partial<ChatView> = {}): ChatView => ({
  seq: 1,
  plugin: 'echo',
  friendId: 'ana',
  open: false,
  loaded: false,
  more: false,
  unread: 0,
  typing: false,
  messages: [],
  ...over,
});

describe('canSendChat', () => {
  it('refuses blank text and text over the cap, counting characters not UTF-16 units', () => {
    expect(canSendChat('')).toBe(false);
    expect(canSendChat('  \n\t ')).toBe(false);
    expect(canSendChat('hi')).toBe(true);
    expect(canSendChat('a'.repeat(4096))).toBe(true);
    expect(canSendChat('a'.repeat(4097))).toBe(false);
    // 2048 astral characters are 4096 code points but 4096 UTF-16 units twice over.
    expect(canSendChat('😀'.repeat(4096))).toBe(true);
    expect(canSendChat('😀'.repeat(4097))).toBe(false);
  });
});

describe('upsertConversation', () => {
  it('replaces the conversation with the same store and friend, and keeps the newest seq', () => {
    const list = upsertConversation([], conv({ seq: 2, unread: 1 }));
    const newer = upsertConversation(list, conv({ seq: 5, unread: 3 }));
    expect(newer).toHaveLength(1);
    expect(unreadFor(newer, 'echo', 'ana')).toBe(3);
    const late = upsertConversation(newer, conv({ seq: 4, unread: 9 }));
    expect(late).toBe(newer);
  });

  it('keeps conversations apart by store and by friend', () => {
    let list: ChatView[] = [];
    list = upsertConversation(list, conv({ plugin: 'echo', friendId: 'ana', unread: 1 }));
    list = upsertConversation(list, conv({ plugin: 'gog', friendId: 'ana', unread: 2 }));
    list = upsertConversation(list, conv({ plugin: 'echo', friendId: 'bo', unread: 4 }));
    expect(list).toHaveLength(3);
    expect(unreadFor(list, 'gog', 'ana')).toBe(2);
    expect(unreadTotal(list)).toBe(7);
    expect(chatKey('echo', 'ana')).not.toBe(chatKey('echo', 'an'));
  });
});

describe('dropConversation', () => {
  it('removes the evicted conversation, and keeps a newer view of it', () => {
    const list = [conv({ friendId: 'a', seq: 5 }), conv({ friendId: 'b', seq: 2 })];
    expect(dropConversation(list, { plugin: 'echo', friendId: 'b', seq: 3 }).map((v) => v.friendId)).toEqual(['a']);
    expect(dropConversation(list, { plugin: 'echo', friendId: 'a', seq: 4 }).map((v) => v.friendId)).toEqual(['a', 'b']);
    expect(dropConversation(list, { plugin: 'other', friendId: 'b', seq: 9 })).toHaveLength(2);
  });
});

describe('alertText', () => {
  it('names the friend of one message, and the friends and the count of several', () => {
    expect(alertText(1, ['Ana'], 'Steam')).toBe('New message from Ana on Steam');
    expect(alertText(3, ['Ana'], 'Steam')).toBe('3 new messages from Ana on Steam');
    expect(alertText(4, ['Ana', 'Bo'], 'Steam')).toBe('4 new messages from Ana and Bo on Steam');
    expect(alertText(5, ['Ana', 'Bo', 'Cy'], 'Steam')).toBe('5 new messages from Ana and 2 others on Steam');
  });
});

describe('messageKey', () => {
  it('uses the clientId while a message is in flight and the id once confirmed', () => {
    expect(messageKey({ id: '', from: 'self', text: 'x', sentAt: 1, status: 'sending', clientId: 'c-1' })).toBe('c-1');
    expect(messageKey({ id: 's9', from: 'self', text: 'x', sentAt: 1 })).toBe('s9');
  });
});

describe('splitLinks', () => {
  it('keeps plain text as text and makes http and https links clickable', () => {
    expect(splitLinks('see https://example.com/a?b=1 now')).toEqual([
      { text: 'see ' },
      { text: 'https://example.com/a?b=1', href: 'https://example.com/a?b=1' },
      { text: ' now' },
    ]);
  });

  it('leaves a trailing full stop outside the link', () => {
    expect(splitLinks('go to http://x.test.')).toEqual([{ text: 'go to ' }, { text: 'http://x.test', href: 'http://x.test' }, { text: '.' }]);
  });

  it('never makes other schemes or markup into links', () => {
    const parts = splitLinks('<b>hi</b> javascript:alert(1) file:///etc/passwd');
    expect(parts.every((p) => p.href === undefined)).toBe(true);
    expect(parts.map((p) => p.text).join('')).toBe('<b>hi</b> javascript:alert(1) file:///etc/passwd');
  });
});

describe('clockLabel', () => {
  it('formats the local time of day as HH:MM', () => {
    const d = new Date(2026, 9, 9, 7, 5);
    expect(clockLabel(d.getTime())).toBe('07:05');
  });
});
