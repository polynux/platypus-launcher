// The page's allowlist is exactly the methods the UI calls (issue #89): a
// new UI call without a list entry fails closed, and a method the UI never
// calls stays refused even when the core serves it.
import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { test } from 'node:test';
import { APP_PAGE_METHODS, isMethodAllowed, notAllowedMessage } from './allowlist.ts';

// Every App.<Method> call in the UI (outside gen/) goes through
// window.platypus.invoke by name (apps/ui/src/effect/bridge.ts).
function uiMethods(): Set<string> {
  const root = path.resolve(import.meta.dirname, '..', '..', 'ui', 'src');
  const found = new Set<string>();
  const walk = (dir: string): void => {
    for (const name of readdirSync(dir)) {
      const p = path.join(dir, name);
      if (statSync(p).isDirectory()) {
        if (name !== 'gen') walk(p);
        continue;
      }
      if (!/\.(ts|tsx)$/.test(name)) continue;
      const src = readFileSync(p, 'utf8');
      for (const m of src.matchAll(/App\.([A-Za-z0-9_]+)/g)) {
        if (m[1] !== 'tsx') found.add(m[1]);
      }
    }
  };
  walk(root);
  return found;
}

test("the page's list is exactly the methods the UI calls", () => {
  assert.deepEqual(new Set(APP_PAGE_METHODS), uiMethods());
});

test('methods the UI never calls stay refused', () => {
  // GetGameStatus/RefreshSavePaths/SetSystemLanguages are served but unused
  // (issue #89); Locale is internal too (only main calls SetSystemLanguages,
  // straight at the core, and GetGameStatus only backs its setters in Go).
  for (const m of ['GetGameStatus', 'RefreshSavePaths', 'SetSystemLanguages', 'Locale']) {
    assert.equal(isMethodAllowed('app', m), false, m);
  }
  assert.equal(isMethodAllowed('app', 'ApprovePlugin'), true);
  assert.equal(notAllowedMessage('GetGameStatus'), 'refused: GetGameStatus is not allowed');
});

test('unknown surfaces allow nothing', () => {
  assert.equal(isMethodAllowed('theme', 'GetLibrary'), false);
  assert.equal(isMethodAllowed('app', 5), false);
});
