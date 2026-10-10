import assert from 'node:assert/strict';
import { test } from 'node:test';
import { isReloadOrInspect } from './shortcuts.ts';

test('reload and inspect keys are caught, typing keys pass through', () => {
  const key = (k: string, control = false, shift = false, meta = false) => ({ key: k, control, meta, shift });
  for (const blocked of [key('F5'), key('F12'), key('r', true), key('R', true), key('r', true, true), key('i', true, true), key('j', true, true), key('c', true, true), key('r', false, false, true)]) {
    assert.ok(isReloadOrInspect(blocked), JSON.stringify(blocked));
  }
  for (const free of [key('r'), key('i'), key('F1'), key('t', true), key('w', true), key('F11')]) {
    assert.ok(!isReloadOrInspect(free), JSON.stringify(free));
  }
});
