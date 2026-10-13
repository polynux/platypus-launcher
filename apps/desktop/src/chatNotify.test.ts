import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  clipNotifyText,
  coalescedBody,
  createChatNotifier,
  shouldShowNotification,
  validNotifyMessage,
  type ChatNotifyMessage,
  type ShownNotification,
} from './chatNotify.ts';

const msg = (over: Partial<ChatNotifyMessage> = {}): ChatNotifyMessage => ({
  plugin: 'echo',
  friendId: '7656119800000001',
  title: 'Ana Ruiz · Echo',
  name: 'Ana Ruiz',
  body: 'hey, are you on the new build?',
  open: null,
  ...over,
});

const ctx = (over = {}) => ({ focused: false, settingOn: true, ...over });

test('shows only when unfocused, switched on, and about another conversation', () => {
  assert.equal(shouldShowNotification(msg(), ctx()), true);
  assert.equal(shouldShowNotification(msg(), ctx({ focused: true })), false);
  assert.equal(shouldShowNotification(msg(), ctx({ settingOn: false })), false);
  assert.equal(
    shouldShowNotification(msg({ open: { plugin: 'echo', friendId: '7656119800000001' } }), ctx()),
    false,
  );
  assert.equal(
    shouldShowNotification(msg({ open: { plugin: 'echo', friendId: 'other' } }), ctx()),
    true,
  );
});

test('malformed payloads are refused', () => {
  assert.equal(validNotifyMessage(msg()), true);
  assert.equal(validNotifyMessage(msg({ open: { plugin: 'echo', friendId: 'x' } })), true);
  for (const bad of [
    null,
    'x',
    msg({ title: '' }),
    msg({ body: '' }),
    msg({ plugin: '' }),
    msg({ friendId: '' }),
    msg({ name: 'x'.repeat(101) }),
    msg({ title: 'x'.repeat(201) }),
    msg({ open: { plugin: 'echo' } as unknown as { plugin: string; friendId: string } }),
  ]) {
    assert.equal(validNotifyMessage(bad), false);
  }
});

test('the body is the message text clipped to ~140 chars', () => {
  assert.equal(clipNotifyText('hi'), 'hi');
  const long = 'x'.repeat(200);
  const clipped = clipNotifyText(long);
  assert.ok(clipped.length <= 141 && clipped.endsWith('…'));
  assert.equal(coalescedBody('Ana Ruiz', 1, 'hello'), 'hello');
  assert.equal(coalescedBody('Ana Ruiz', 3, 'hello'), '3 new messages from Ana Ruiz');
});

test('the coalesced count follows the shell language', () => {
  assert.equal(coalescedBody('Ana Ruiz', 3, 'hello', 'fr'), '3 nouveaux messages de Ana Ruiz');
  assert.equal(coalescedBody('Ana Ruiz', 1, 'hello', 'fr'), 'hello');
});

test('a burst for one conversation shows once, then replaces with the count', () => {
  const shown: { title: string; body: string }[] = [];
  const closed: string[] = [];
  const notifier = createChatNotifier({
    windowMs: 1000,
    show: (title, body, _onClick) => {
      shown.push({ title, body });
      const i = shown.length - 1;
      return { close: () => closed.push(`n${i}`) } satisfies ShownNotification;
    },
  });
  const click = () => {};
  assert.equal(notifier.notify(msg(), ctx(), click), 'shown');
  assert.equal(notifier.notify(msg({ body: 'second' }), ctx(), click), 'updated');
  assert.equal(notifier.notify(msg({ body: 'third' }), ctx(), click), 'updated');
  assert.deepEqual(
    shown.map((s) => s.body),
    ['hey, are you on the new build?', '2 new messages from Ana Ruiz', '3 new messages from Ana Ruiz'],
  );
  assert.deepEqual(closed, ['n0', 'n1']);
  assert.equal(shown[0].title, 'Ana Ruiz · Echo');
  notifier.dispose();
});

test('another conversation is its own notification, and the switch off drops', () => {
  let n = 0;
  const notifier = createChatNotifier({ show: () => { n++; return { close: () => {} }; } });
  const click = () => {};
  assert.equal(notifier.notify(msg(), ctx(), click), 'shown');
  assert.equal(notifier.notify(msg({ friendId: 'other', title: 'Bo · Echo', name: 'Bo' }), ctx(), click), 'shown');
  assert.equal(n, 2);
  assert.equal(notifier.notify(msg(), ctx({ settingOn: false }), click), 'dropped');
  assert.equal(notifier.notify(msg(), ctx({ focused: true }), click), 'dropped');
  assert.equal(n, 2);
  notifier.dispose();
});
