import React, { useEffect, useRef, useState } from 'react';
import { Modal } from './Modal';
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
  onClose: () => void;
}

/** One conversation with a friend: the messages (newest at the bottom), a box to write in, and what
 *  each message's send came to. Enter sends; Shift+Enter starts a new line. Links open in the browser
 *  only when they are http or https; everything else is plain text. */
export function ChatPanel(p: ChatPanelProps) {
  const [text, setText] = useState('');
  const log = useRef<HTMLDivElement>(null);
  const count = p.view?.messages.length ?? 0;

  // Keep the newest message in view, as a conversation does.
  useEffect(() => {
    if (log.current) log.current.scrollTop = log.current.scrollHeight;
  }, [count]);

  const submit = () => {
    if (!canSendChat(text)) return;
    p.onSend(text);
    setText('');
  };

  const messages = p.view?.messages ?? [];
  return (
    <Modal label={`Chat with ${p.friendName}`} onClose={p.onClose} className="max-w-lg">
      <div className="flex h-[70vh] max-h-[640px] flex-col gap-3 p-5">
        <header className="flex items-center justify-between gap-3">
          <div className="min-w-0">
            <h2 className="truncate text-[17px] font-extrabold tracking-tight">{p.friendName}</h2>
            <div className="font-mono text-[10.5px] uppercase tracking-[0.16em] text-white/45">{p.storeName}</div>
          </div>
          <Button variant="ghost" size="sm" onClick={p.onClose}>
            Close
          </Button>
        </header>

        {p.error && (
          <p role="alert" className="rounded-lg bg-red-500/10 px-3 py-2 text-[12px] text-red-200 ring-1 ring-red-400/25">
            {p.error}
          </p>
        )}

        <div ref={log} role="log" aria-label={`Messages with ${p.friendName}`} className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1">
          {p.view?.more && (
            <div className="self-center">
              <LinkButton onClick={p.onLoadOlder}>Load older messages</LinkButton>
            </div>
          )}
          {!p.view?.loaded && !p.error && <p className="text-[12.5px] text-white/50">Loading…</p>}
          {p.view?.loaded && messages.length === 0 && (
            <p className="text-[12.5px] text-white/50">No messages yet. Say hello.</p>
          )}
          {messages.map((m) => (
            <MessageRow key={messageKey(m)} message={m} onResend={p.onResend} />
          ))}
        </div>

        {p.view?.typing && <p className="text-[11.5px] italic text-white/45">{p.friendName} is typing…</p>}

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
    </Modal>
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
          'whitespace-pre-wrap break-words rounded-2xl px-3 py-2 text-[13px] leading-snug',
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
      <div className="flex items-center gap-2 font-mono text-[10px] text-white/40">
        <span className={message.status === 'failed' ? 'text-red-300' : undefined}>{state}</span>
        {message.status === 'failed' && message.clientId && (
          <LinkButton onClick={() => onResend(message.clientId!)}>Send again</LinkButton>
        )}
      </div>
    </div>
  );
}
