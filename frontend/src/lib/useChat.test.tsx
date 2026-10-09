import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { ChatEvent, ChatView, SocialView } from '../types';

const h = vi.hoisted(() => ({
  listener: null as null | ((e: unknown) => void),
  api: {} as Record<string, (...args: unknown[]) => unknown>,
}));

vi.mock('../effect/runtime', () => ({
  onChat: (cb: (e: unknown) => void) => {
    h.listener = cb;
    return () => {};
  },
  LauncherApi: new Proxy({}, { get: (_t, name: string) => (...args: unknown[]) => h.api[name]?.(...args) }),
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
  h.api = {
    chatUnread: () => Effect.succeed([]),
    chatAlerts: () => Effect.succeed({ echo: true }),
    chatOpen: () => Effect.succeed(conv({ open: true })),
    chatClose: () => Effect.succeed(undefined),
    chatSend: () => Effect.succeed(undefined),
  };
});

describe('useChat', () => {
  it('counts unread messages across the stores, and toasts a new message by the friend\'s name, never its text', async () => {
    const notify = vi.fn();
    const { result } = renderHook(() => useChat(social, notify));
    await waitFor(() => expect(result.current.alerts).toEqual({ echo: true }));

    act(() => {
      h.listener?.({ ...conv({ unread: 2, seq: 3 }), alert: true, messages: [{ id: 'm1', from: '1', text: 'secret words', sentAt: 1 }] } as ChatEvent);
    });
    expect(result.current.unread).toBe(2);
    expect(result.current.unreadOf('echo', '1')).toBe(2);
    expect(notify).toHaveBeenCalledWith('New message from Ana Ruiz');
    expect(JSON.stringify(notify.mock.calls)).not.toContain('secret words');
  });

  it('does not toast a message that is not an alert', async () => {
    const notify = vi.fn();
    renderHook(() => useChat(social, notify));
    act(() => {
      h.listener?.({ ...conv({ unread: 0, seq: 4 }), alert: false } as ChatEvent);
    });
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
});
