import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  buildMenuModel,
  hideActionForSessionStart,
  isRunningState,
  restoreWindow,
  shouldShowOnSessionEnd,
  TrayState,
} from './tray.ts';

const text = {
  show: 'Show Platypus',
  stop: (title: string) => `Stop ${title}`,
  quit: 'Quit',
};

test('running states are starting, running and stopping', () => {
  assert.equal(isRunningState('starting'), true);
  assert.equal(isRunningState('running'), true);
  assert.equal(isRunningState('stopping'), true);
  assert.equal(isRunningState('stopped'), false);
  assert.equal(isRunningState('failed'), false);
  assert.equal(isRunningState(''), false);
});

test('hide on start only when the switch is on and the game runs', () => {
  assert.equal(hideActionForSessionStart(true, 'running', true), 'hide');
  assert.equal(hideActionForSessionStart(true, 'starting', true), 'hide');
  assert.equal(hideActionForSessionStart(false, 'running', true), 'none');
  assert.equal(hideActionForSessionStart(true, 'stopped', true), 'none');
  assert.equal(hideActionForSessionStart(true, 'failed', true), 'none');
});

test('no tray host falls back to minimizing instead of hiding', () => {
  assert.equal(hideActionForSessionStart(true, 'running', false), 'minimize');
  assert.equal(hideActionForSessionStart(false, 'running', false), 'none');
});

test('the window shows only when the last game exits', () => {
  assert.equal(shouldShowOnSessionEnd(0), true);
  assert.equal(shouldShowOnSessionEnd(1), false);
  assert.equal(shouldShowOnSessionEnd(2), false);
});

test('the first start hides, the second start while hidden does nothing', () => {
  const s = new TrayState();
  s.settingOn = true;
  assert.equal(s.onSessionEvent('g1', 'Doom', 'running'), 'hide');
  assert.equal(s.onSessionEvent('g2', 'Quake', 'running'), 'none');
  assert.equal(s.runningCount(), 2);
  // An end that leaves one game running shows nothing.
  assert.equal(s.onSessionEnd('g1'), false);
  assert.equal(s.onSessionEnd('g2'), true);
});

test('nothing hides when the switch is off, games are still tracked', () => {
  const s = new TrayState();
  s.settingOn = false;
  assert.equal(s.onSessionEvent('g1', 'Doom', 'running'), 'none');
  assert.equal(s.runningCount(), 1);
  assert.deepEqual(s.games(), [{ id: 'g1', title: 'Doom' }]);
  assert.equal(s.onSessionEnd('g1'), true);
});

test('unknown game ids never move the window', () => {
  const s = new TrayState();
  s.settingOn = true;
  assert.equal(s.onSessionEvent('', 'Doom', 'running'), 'none');
  assert.equal(s.onSessionEnd(''), false);
});

test('the menu shows Platypus, each running game with Stop, and Quit', () => {
  assert.deepEqual(buildMenuModel([], text), [
    { kind: 'show', label: 'Show Platypus' },
    { kind: 'separator' },
    { kind: 'quit', label: 'Quit' },
  ]);
  assert.deepEqual(
    buildMenuModel(
      [
        { id: 'g1', title: 'Doom' },
        { id: 'g2', title: '' },
      ],
      text,
    ),
    [
      { kind: 'show', label: 'Show Platypus' },
      { kind: 'separator' },
      { kind: 'stop', label: 'Stop Doom', gameId: 'g1' },
      { kind: 'stop', label: 'Stop g2', gameId: 'g2' },
      { kind: 'separator' },
      { kind: 'quit', label: 'Quit' },
    ],
  );
});

function fakeWindow(over: Partial<{ minimized: boolean; visible: boolean }> = {}) {
  const calls: string[] = [];
  return {
    calls,
    minimized: over.minimized ?? false,
    visible: over.visible ?? true,
    isMinimized() {
      return this.minimized;
    },
    isVisible() {
      return this.visible;
    },
    restore() {
      calls.push('restore');
      this.minimized = false;
    },
    show() {
      calls.push('show');
      this.visible = true;
    },
    focus() {
      calls.push('focus');
    },
  };
}

test('restore shows a hidden window (restore alone never unhides it)', () => {
  const w = fakeWindow({ visible: false, minimized: false });
  restoreWindow(w);
  assert.deepEqual(w.calls, ['show', 'focus']);
});

test('restore unhides and unminimizes, then focuses', () => {
  const w = fakeWindow({ visible: false, minimized: true });
  restoreWindow(w);
  assert.deepEqual(w.calls, ['show', 'restore', 'focus']);
});

test('restore of a visible minimized window restores and focuses', () => {
  const w = fakeWindow({ visible: true, minimized: true });
  restoreWindow(w);
  assert.deepEqual(w.calls, ['restore', 'focus']);
});

test('the single-instance path shows the window: a second launch focuses it', () => {
  // The hidden window reports visible=false, minimized=false; without the
  // show() the second launch would focus a window nobody can see.
  const w = fakeWindow({ visible: false, minimized: false });
  restoreWindow(w);
  assert.ok(w.visible);
  assert.deepEqual(w.calls, ['show', 'focus']);
});
