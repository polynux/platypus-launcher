import React, { useLayoutEffect, useRef, useState } from 'react';
import { cx, LinkButton } from './ui';
import { Button } from './ds/button';
import { Textarea } from './ds/textarea';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import { canSendChat, clockLabel, messageKey, splitLinks } from '../lib/chat';
import type { ChatMessage, ChatView } from '../types';

interface ChatPanelProps {
  /** The conversation as the backend last reported it; undefined until the first answer. */
  view?: ChatView;
  friendName: string;
  storeName: string;
  /** Why the conversation couldn't load or a request failed, shown above the messages. */
  error?: string | null;
  onLoadOlder: () => void;
  onSend: (text: string) => void;
  onResend: (clientId: string) => void;
  /** Back to the friends list. */
  onBack?: () => void;
  /** Closes the friends modal. */
  onClose: () => void;
}

/** One conversation with a friend: the messages (newest at the bottom), a box to write in, and what
 *  each message's send came to. Enter sends; Shift+Enter starts a new line. Links open in the browser
 *  only when they are http or https; everything else is plain text. It lives inside the friends modal,
 *  which provides the dialog and its Escape handling. */
export function ChatPanel(p: ChatPanelProps) {
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

  const submit = () => {
    if (!canSendChat(text)) return;
    // What the user sends is theirs to see: the log follows it down.
    atEnd.current = true;
    p.onSend(text);
    setText('');
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <header className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          {p.onBack && (
            <Button variant="ghost" size="sm" onClick={p.onBack} className="flex-none">
              Back
            </Button>
          )}
          <div className="min-w-0">
            <h2 className="truncate text-[17px] font-extrabold tracking-tight">{p.friendName}</h2>
            <div className="font-mono text-micro uppercase tracking-[0.16em] text-white/55">{p.storeName}</div>
          </div>
        </div>
        <Button variant="ghost" size="sm" onClick={p.onClose} className="flex-none">
          Close
        </Button>
      </header>

      {p.error && (
        <p role="alert" className="rounded-lg bg-red-500/10 px-3 py-2 text-[12px] text-red-200 ring-1 ring-red-400/25">
          {p.error}
        </p>
      )}

      <div
        ref={log}
        role="log"
        aria-label={`Messages with ${p.friendName}`}
        onScroll={(e) => {
          const el = e.currentTarget;
          atEnd.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }}
        className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1"
      >
        {p.view?.more && (
          <div className="self-center">
            <LinkButton onClick={p.onLoadOlder}>Load older messages</LinkButton>
          </div>
        )}
        {!p.view?.loaded && !p.error && <p className="text-[12.5px] text-white/70">Loading…</p>}
        {p.view?.loaded && messages.length === 0 && (
          <p className="text-[12.5px] text-white/70">No messages yet. Say hello.</p>
        )}
        {messages.map((m) => (
          <MessageRow key={messageKey(m)} message={m} onResend={p.onResend} />
        ))}
      </div>

      {p.view?.typing && <p className="text-hint italic text-white/55">{p.friendName} is typing…</p>}

      <div className="flex items-end gap-2">
        <Textarea
          aria-label={`Message ${p.friendName}`}
          rows={2}
          value={text}
          placeholder="Write a message"
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault();
              submit();
            }
          }}
          className="flex-1 font-sans"
        />
        <Button variant="primary" size="md" onClick={submit} disabled={!canSendChat(text)}>
          Send
        </Button>
      </div>
    </div>
  );
}

function MessageRow({ message, onResend }: {
  message: ChatMessage;
  onResend: (clientId: string) => void;
}) {
  const mine = message.from === 'self';
  const state = message.status === 'sending' ? 'Sending…' : message.status === 'failed' ? 'Not sent' : clockLabel(message.sentAt);
  return (
    <div className={cx('flex max-w-[82%] flex-col gap-0.5', mine ? 'self-end items-end' : 'self-start items-start')}>
      <div
        className={cx(
          'whitespace-pre-wrap rounded-2xl px-3 py-2 text-[13px] leading-snug [overflow-wrap:anywhere]',
          mine ? 'bg-amber-300/15 text-amber-50' : 'bg-white/[0.06] text-white/85',
          message.status === 'failed' && 'ring-1 ring-red-400/40',
        )}
      >
        {splitLinks(message.text).map((part, i) =>
          part.href ? (
            <a
              key={i}
              href={part.href}
              className="underline decoration-white/35 underline-offset-2 hover:text-white"
              onClick={(e) => {
                e.preventDefault();
                BrowserOpenURL(part.href!);
              }}
            >
              {part.text}
            </a>
          ) : (
            <React.Fragment key={i}>{part.text}</React.Fragment>
          ),
        )}
      </div>
      <div className="flex items-center gap-2 font-mono text-hint text-white/55">
        <span className={message.status === 'failed' ? 'text-red-300' : undefined}>{state}</span>
        {message.status === 'failed' && message.clientId && (
          <LinkButton onClick={() => onResend(message.clientId!)}>Send again</LinkButton>
        )}
      </div>
    </div>
  );
}
