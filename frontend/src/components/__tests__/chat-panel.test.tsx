import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { ChatPanel } from '../ChatPanel';
import type { ChatMessage, ChatView } from '../../types';

const open = vi.hoisted(() => vi.fn());

vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: open }));

const msg = (over: Partial<ChatMessage> = {}): ChatMessage => ({ id: 'm1', from: 'ana', text: 'hello', sentAt: new Date(2026, 9, 9, 7, 5).getTime(), ...over });

const view = (over: Partial<ChatView> = {}): ChatView => ({
  seq: 1,
  plugin: 'echo',
  friendId: 'ana',
  open: true,
  loaded: true,
  more: false,
  unread: 0,
  typing: false,
  messages: [msg()],
  ...over,
});

function panel(over: Partial<React.ComponentProps<typeof ChatPanel>> = {}) {
  const props = {
    view: view(),
    friendName: 'Ana Ruiz',
    storeName: 'Echo',
    onLoadOlder: vi.fn(),
    onSend: vi.fn(),
    onResend: vi.fn(),
    onClose: vi.fn(),
    ...over,
  };
  render(<ChatPanel {...props} />);
  return props;
}

describe('ChatPanel', () => {
  it('shows the friend, the store and the messages with their times', () => {
    panel();
    expect(screen.getByRole('heading', { name: 'Ana Ruiz' })).toBeTruthy();
    expect(screen.getByText('Echo')).toBeTruthy();
    expect(screen.getByText('hello')).toBeTruthy();
    expect(screen.getByText('07:05')).toBeTruthy();
  });

  it('sends on Enter and keeps Shift+Enter for a new line', () => {
    const props = panel();
    const box = screen.getByRole('textbox', { name: 'Message Ana Ruiz' });
    fireEvent.change(box, { target: { value: 'see you' } });
    fireEvent.keyDown(box, { key: 'Enter', shiftKey: true });
    expect(props.onSend).not.toHaveBeenCalled();
    fireEvent.keyDown(box, { key: 'Enter' });
    expect(props.onSend).toHaveBeenCalledWith('see you');
    expect((box as HTMLTextAreaElement).value).toBe('');
  });

  it('does not send blank text, and the Send button stays off for it', () => {
    const props = panel();
    const box = screen.getByRole('textbox', { name: 'Message Ana Ruiz' });
    fireEvent.change(box, { target: { value: '   ' } });
    expect((screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.keyDown(box, { key: 'Enter' });
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it('shows a message in flight as sending and a refused one with Send again', () => {
    const props = panel({
      view: view({
        messages: [
          msg({ id: '', from: 'self', text: 'on its way', status: 'sending', clientId: 'c-1' }),
          msg({ id: '', from: 'self', text: 'refused', status: 'failed', clientId: 'c-2' }),
        ],
      }),
    });
    expect(screen.getByText('Sending…')).toBeTruthy();
    expect(screen.getByText('Not sent')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Send again' }));
    expect(props.onResend).toHaveBeenCalledWith('c-2');
    expect(screen.getAllByRole('button', { name: 'Send again' })).toHaveLength(1);
  });

  it('opens http and https links in the browser and shows anything else as text', () => {
    panel({ view: view({ messages: [msg({ text: 'go to https://example.com now' }), msg({ id: 'm2', text: 'not javascript:alert(1)' })] }) });
    const link = screen.getByRole('link', { name: 'https://example.com' });
    fireEvent.click(link);
    expect(open).toHaveBeenCalledWith('https://example.com');
    expect(screen.queryByRole('link', { name: /javascript/ })).toBeNull();
    expect(screen.getByText(/not javascript:alert\(1\)/)).toBeTruthy();
  });

  it('renders message text as text, not as markup', () => {
    panel({ view: view({ messages: [msg({ text: '<img src=x onerror=alert(1)>' })] }) });
    expect(screen.getByText('<img src=x onerror=alert(1)>')).toBeTruthy();
    expect(document.querySelector('img')).toBeNull();
  });

  it('offers older messages only when the store has more', () => {
    const props = panel({ view: view({ more: true }) });
    fireEvent.click(screen.getByRole('button', { name: 'Load older messages' }));
    expect(props.onLoadOlder).toHaveBeenCalled();
  });

  it('says when the friend is typing, when the history is loading, and when there is nothing yet', () => {
    const { unmount } = render(<ChatPanel view={view({ typing: true })} friendName="Ana Ruiz" storeName="Echo" onLoadOlder={vi.fn()} onSend={vi.fn()} onResend={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('Ana Ruiz is typing…')).toBeTruthy();
    unmount();
    const { unmount: again } = render(<ChatPanel friendName="Ana Ruiz" storeName="Echo" onLoadOlder={vi.fn()} onSend={vi.fn()} onResend={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('Loading…')).toBeTruthy();
    again();
    render(<ChatPanel view={view({ messages: [] })} friendName="Ana Ruiz" storeName="Echo" onLoadOlder={vi.fn()} onSend={vi.fn()} onResend={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('No messages yet. Say hello.')).toBeTruthy();
  });

  it('shows why the conversation failed, as an alert', () => {
    panel({ view: undefined, error: 'chat is off for Echo' });
    expect(screen.getByRole('alert').textContent).toBe('chat is off for Echo');
  });
});

describe('ChatPanel layout', () => {
  it('lets a long unbroken word wrap inside its bubble', () => {
    panel({ view: view({ messages: [msg({ text: 'x'.repeat(300) })] }) });
    const bubble = screen.getByText('x'.repeat(300));
    expect(bubble.className).toContain('[overflow-wrap:anywhere]');
  });

  it('follows new messages only from the bottom, and keeps the place when older ones come in above', () => {
    let height = 500;
    const { rerender } = render(<ChatPanel {...panelProps(view({ messages: [msg({ id: 'm2', text: 'two' })] }))} />);
    const log = screen.getByRole('log');
    Object.defineProperty(log, 'scrollHeight', { configurable: true, get: () => height });
    Object.defineProperty(log, 'clientHeight', { configurable: true, get: () => 0 });
    log.scrollTop = 500;

    // At the bottom: a new message follows it down.
    height = 600;
    rerender(<ChatPanel {...panelProps(view({ messages: [msg({ id: 'm2', text: 'two' }), msg({ id: 'm3', text: 'three' })] }))} />);
    expect(log.scrollTop).toBe(600);

    // Scrolled up: a new message leaves the place where it is.
    log.scrollTop = 100;
    fireEvent.scroll(log);
    height = 700;
    rerender(<ChatPanel {...panelProps(view({ messages: [msg({ id: 'm2', text: 'two' }), msg({ id: 'm3', text: 'three' }), msg({ id: 'm4', text: 'four' })] }))} />);
    expect(log.scrollTop).toBe(100);

    // An older message comes in above: the messages the user reads stay where they were.
    height = 800;
    rerender(<ChatPanel {...panelProps(view({ messages: [msg({ id: 'm1', text: 'one' }), msg({ id: 'm2', text: 'two' }), msg({ id: 'm3', text: 'three' }), msg({ id: 'm4', text: 'four' })] }))} />);
    expect(log.scrollTop).toBe(200);
  });
});

function panelProps(v: ChatView): React.ComponentProps<typeof ChatPanel> {
  return {
    view: v,
    friendName: 'Ana Ruiz',
    storeName: 'Echo',
    onLoadOlder: vi.fn(),
    onSend: vi.fn(),
    onResend: vi.fn(),
    onClose: vi.fn(),
  };
}
