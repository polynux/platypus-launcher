import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { ChatEvent, ChatView, SocialView } from '../types';

const h = vi.hoisted(() => ({
  listener: null as null | ((e: unknown) => void),
  api: {} as Record<string, (...args: unknown[]) => unknown>,
}));

const b = vi.hoisted(() => ({
  notifyChat: vi.fn((_msg: unknown) => Promise.resolve(false)),
  chatOpenCb: null as null | ((t: { plugin: string; friendId: string }) => void),
}));

vi.mock('../effect/runtime', () => ({
  onChat: (cb: (e: unknown) => void) => {
    h.listener = cb;
    return () => {};
  },
  LauncherApi: new Proxy({}, { get: (_t, name: string) => (...args: unknown[]) => h.api[name]?.(...args) }),
}));

vi.mock('../effect/bridge', () => ({
  notifyChat: (msg: unknown) => b.notifyChat(msg),
  onChatOpen: (cb: (t: { plugin: string; friendId: string }) => void) => {
    b.chatOpenCb = cb;
    return () => { b.chatOpenCb = null; };
  },
}));

import { useChat } from './useChat';

const conv = (over: Partial<ChatView> = {}): ChatView => ({
  seq: 1,
  plugin: 'echo',
  friendId: '1',
  open: false,
  loaded: true,
  more: false,
  unread: 0,
  typing: false,
  messages: [],
  ...over,
});

const social: SocialView[] = [
  { seq: 1, plugin: 'echo', enabled: true, showPlaying: true, running: true, connection: { state: 'online' }, friends: [{ id: '1', name: 'Ana Ruiz', state: 'online' }] },
];

beforeEach(() => {
  h.listener = null;
  b.notifyChat.mockClear();
  b.chatOpenCb = null;
  h.api = {
    chatUnread: () => Effect.succeed([]),
    chatConversations: () => Effect.succeed([]),
    chatAlerts: () => Effect.succeed({ echo: true }),
    chatOpen: () => Effect.succeed(conv({ open: true })),
    chatClose: () => Effect.succeed(undefined),
    chatSend: () => Effect.succeed(undefined),
  };
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useChat', () => {
  it('counts unread messages across the stores, and toasts a new message by the friend\'s name, never its text', async () => {
    vi.useFakeTimers();
    const notify = vi.fn();
    const { result } = renderHook(() => useChat(social, notify));
    await act(async () => {});
    expect(result.current.alerts).toEqual({ echo: true });

    act(() => {
      h.listener?.({ ...conv({ unread: 2, seq: 3 }), alert: true, messages: [{ id: 'm1', from: '1', text: 'secret words', sentAt: 1 }] } as ChatEvent);
    });
    expect(result.current.unread).toBe(2);
    expect(result.current.unreadOf('echo', '1')).toBe(2);
    act(() => vi.advanceTimersByTime(3000));
    expect(notify).toHaveBeenCalledWith('New message from Ana Ruiz on echo', 'busy');
    expect(JSON.stringify(notify.mock.calls)).not.toContain('secret words');
  });

  it('sums the unread messages of two stores, and each store\'s toast names the store', async () => {
    vi.useFakeTimers();
    const notify = vi.fn();
    const two: SocialView[] = [
      ...social,
      { seq: 1, plugin: 'echo-b', enabled: true, showPlaying: true, running: true, connection: { state: 'online' }, friends: [{ id: '1', name: 'Ben Ortiz', state: 'online' }] },
    ];
    const { result } = renderHook(() => useChat(two, notify, { echo: 'Echo (test plugin)', 'echo-b': 'Echo B (test plugin)' }));
    await act(async () => {});

    act(() => {
      h.listener?.({ ...conv({ unread: 2, seq: 3 }), alert: true, messages: [] } as ChatEvent);
      h.listener?.({ ...conv({ plugin: 'echo-b', friendId: '1', unread: 3, seq: 2 }), alert: true, messages: [] } as ChatEvent);
    });
    expect(result.current.unread).toBe(5);
    expect(result.current.unreadOf('echo-b', '1')).toBe(3);
    act(() => vi.advanceTimersByTime(3000));
    expect(notify).toHaveBeenCalledWith('New message from Ana Ruiz on Echo (test plugin)', 'busy');
    expect(notify).toHaveBeenCalledWith('New message from Ben Ortiz on Echo B (test plugin)', 'busy');
  });

  it('coalesces the messages of one store that arrive together into one toast', async () => {
    vi.useFakeTimers();
    const notify = vi.fn();
    const two: SocialView[] = [{ ...social[0], friends: [...social[0].friends, { id: '2', name: 'Bo Lindqvist', state: 'away' }] }];
    renderHook(() => useChat(two, notify));
    await act(async () => {});

    act(() => {
      h.listener?.({ ...conv({ friendId: '1', unread: 1, seq: 3 }), alert: true, messages: [] } as ChatEvent);
      h.listener?.({ ...conv({ friendId: '1', unread: 2, seq: 4 }), alert: true, messages: [] } as ChatEvent);
      h.listener?.({ ...conv({ friendId: '2', unread: 1, seq: 5 }), alert: true, messages: [] } as ChatEvent);
    });
    expect(notify).not.toHaveBeenCalled();
    act(() => vi.advanceTimersByTime(3000));
    expect(notify).toHaveBeenCalledTimes(1);
    expect(notify).toHaveBeenCalledWith('3 new messages from Ana Ruiz and Bo Lindqvist on echo', 'busy');
  });

  it('closes the conversation on screen when its store clears its chats', async () => {
    const { result } = renderHook(() => useChat(social, vi.fn()));
    await act(async () => {});
    act(() => {
      h.listener?.({ ...conv({ unread: 1, seq: 5 }), alert: false } as ChatEvent);
    });
    act(() => result.current.open('echo', '1'));
    await waitFor(() => expect(result.current.target).not.toBeNull());
    // A logout: the core names the store and no friend.
    act(() => {
      h.listener?.({ ...conv({ friendId: '', seq: 0, messages: [] }), alert: false, dropped: true } as ChatEvent);
    });
    expect(result.current.target).toBeNull();
    expect(result.current.conversations).toEqual([]);
  });

  it('drops a conversation the backend evicted, and keeps a newer one', async () => {
    const { result } = renderHook(() => useChat(social, vi.fn()));
    await act(async () => {});
    act(() => {
      h.listener?.({ ...conv({ unread: 1, seq: 5 }), alert: false } as ChatEvent);
    });
    expect(result.current.unreadOf('echo', '1')).toBe(1);
    act(() => {
      h.listener?.({ ...conv({ seq: 4, messages: [] }), alert: false, dropped: true } as ChatEvent);
    });
    expect(result.current.unreadOf('echo', '1')).toBe(1);
    act(() => {
      h.listener?.({ ...conv({ seq: 6, messages: [] }), alert: false, dropped: true } as ChatEvent);
    });
    expect(result.current.unreadOf('echo', '1')).toBe(0);
  });

  it('exposes every held conversation, read or unread, for the Chats tab', async () => {
    const { result } = renderHook(() => useChat(social, vi.fn()));
    await act(async () => {});
    expect(result.current.conversations).toEqual([]);
    act(() => {
      h.listener?.({ ...conv({ unread: 0, seq: 3 }), alert: false } as ChatEvent);
    });
    expect(result.current.conversations).toHaveLength(1);
    expect(result.current.unread).toBe(0);
  });

  it('seeds the Chats tab from the persisted recent conversations, read or unread (issue #143)', async () => {
    const preview = (over: Partial<ChatView> = {}): ChatView =>
      conv({ seq: 0, unread: 0, loaded: false, more: true, messages: [{ id: 'm9', from: '1', text: 'see you tomorrow', sentAt: 1700000000000 }], ...over });
    h.api.chatConversations = () =>
      Effect.succeed([
        preview({ friendId: '1' }),
        preview({ friendId: '2', unread: 3, seq: 0, messages: [] }),
      ]);
    const { result } = renderHook(() => useChat(social, vi.fn()));
    await waitFor(() => expect(result.current.conversations).toHaveLength(2));
    // A read conversation with a preview shows its text, newest first.
    const rows = result.current.conversations;
    expect(rows.find((v) => v.friendId === '1')?.messages[0].text).toBe('see you tomorrow');
    expect(result.current.unreadOf('echo', '2')).toBe(3);
  });

  it('closes the conversation on screen when another one opens', async () => {
    const close = vi.fn(() => Effect.succeed(undefined));
    h.api.chatClose = close;
    const { result } = renderHook(() => useChat(social, vi.fn()));
    act(() => result.current.open('echo', '1'));
    await waitFor(() => expect(result.current.conversation?.open).toBe(true));
    expect(close).not.toHaveBeenCalled();
    act(() => result.current.open('echo', '2'));
    expect(close).toHaveBeenCalledWith('echo', '1');
    expect(close).not.toHaveBeenCalledWith('echo', '2');
  });

  it('does not toast a message that is not an alert', async () => {
    vi.useFakeTimers();
    const notify = vi.fn();
    renderHook(() => useChat(social, notify));
    act(() => {
      h.listener?.({ ...conv({ unread: 0, seq: 4 }), alert: false } as ChatEvent);
    });
    act(() => vi.advanceTimersByTime(3000));
    expect(notify).not.toHaveBeenCalled();
  });

  it('opens the conversation on screen and closes it again', async () => {
    const notify = vi.fn();
    const close = vi.fn(() => Effect.succeed(undefined));
    h.api.chatClose = close;
    const { result } = renderHook(() => useChat(social, notify));
    act(() => result.current.open('echo', '1'));
    await waitFor(() => expect(result.current.conversation?.open).toBe(true));
    expect(result.current.target).toEqual({ plugin: 'echo', friendId: '1' });
    act(() => result.current.close());
    expect(result.current.target).toBeNull();
    expect(close).toHaveBeenCalledWith('echo', '1');
  });

  it('keeps the open error on screen until the next request', async () => {
    h.api.chatOpen = () => Effect.fail(new Error('chat is off for echo'));
    const { result } = renderHook(() => useChat(social, vi.fn()));
    act(() => result.current.open('echo', '1'));
    await waitFor(() => expect(result.current.error).toBe('chat is off for echo'));
  });

  it('asks the shell for a desktop notification with names and text, never ids', async () => {
    const friends: SocialView[] = [
      { seq: 1, plugin: 'echo', enabled: true, showPlaying: true, running: true, connection: { state: 'online' }, friends: [{ id: '7656119800000001', name: 'Ana Ruiz', state: 'online' }] },
    ];
    renderHook(() => useChat(friends, vi.fn(), { echo: 'Echo (test plugin)' }));
    await act(async () => {});
    act(() => {
      h.listener?.({ ...conv({ friendId: '7656119800000001', unread: 1, seq: 3 }), alert: true, messages: [{ id: 'm1-xyz', from: '7656119800000001', text: 'hey, are you on the new build?', sentAt: 1 }] } as ChatEvent);
    });
    expect(b.notifyChat).toHaveBeenCalledTimes(1);
    const payload = b.notifyChat.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      plugin: 'echo',
      friendId: '7656119800000001',
      title: 'Ana Ruiz · Echo (test plugin)',
      name: 'Ana Ruiz',
      body: 'hey, are you on the new build?',
      open: null,
    });
    expect(`${payload.title} ${payload.body}`).not.toContain('7656119800000001');
    expect(`${payload.title} ${payload.body}`).not.toContain('m1-xyz');
  });

  it('opens the conversation a clicked notification names', async () => {
    const chatOpen = vi.fn(() => Effect.succeed(conv({ open: true })));
    h.api.chatOpen = chatOpen;
    const { result } = renderHook(() => useChat(social, vi.fn()));
    await act(async () => {});
    expect(b.chatOpenCb).not.toBeNull();
    act(() => b.chatOpenCb?.({ plugin: 'echo', friendId: '1' }));
    await waitFor(() => expect(chatOpen).toHaveBeenCalledWith('echo', '1'));
    expect(result.current.target).toEqual({ plugin: 'echo', friendId: '1' });
  });
});
