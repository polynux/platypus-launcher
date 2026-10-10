import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { clampState, DEFAULT_STATE, loadState, saveState, type WorkArea } from './window-state.ts';

const DISPLAYS: WorkArea[] = [{ x: 0, y: 0, width: 1920, height: 1080 }];

test('a fitting state survives, clamped to the work area', () => {
  const s = clampState({ x: 100, y: 100, width: 1024, height: 768, maximized: false }, DISPLAYS);
  assert.deepEqual(s, { x: 100, y: 100, width: 1024, height: 768, maximized: false });
  const big = clampState({ x: 0, y: 0, width: 9999, height: 9999, maximized: false }, DISPLAYS);
  assert.equal(big.width, 1920);
  assert.equal(big.height, 1080);
});

test('an off-screen or missing position falls back to the default', () => {
  assert.deepEqual(clampState({ width: 800, height: 600, maximized: false }, DISPLAYS), DEFAULT_STATE);
  assert.deepEqual(clampState({ x: 5000, y: 100, width: 800, height: 600, maximized: false }, DISPLAYS), DEFAULT_STATE);
  // Maximized survives the fallback: the window still opens full-size.
  assert.deepEqual(clampState({ x: 5000, y: 100, width: 800, height: 600, maximized: true }, DISPLAYS), { ...DEFAULT_STATE, maximized: true });
});

test('a second, smaller display still counts as on-screen', () => {
  const two: WorkArea[] = [...DISPLAYS, { x: 1920, y: 0, width: 1280, height: 1024 }];
  const s = clampState({ x: 2000, y: 100, width: 800, height: 600, maximized: false }, two);
  assert.equal(s.x, 2000);
});

test('the state round-trips through a file; corrupt files read as missing', () => {
  const dir = mkdtempSync(path.join(tmpdir(), 'platypus-ws-'));
  try {
    const file = path.join(dir, 'window-state.json');
    assert.equal(loadState(file), undefined);
    saveState(file, { x: 10, y: 20, width: 800, height: 600, maximized: true });
    assert.deepEqual(loadState(file), { x: 10, y: 20, width: 800, height: 600, maximized: true });
    writeFileSync(file, '{oops');
    assert.equal(loadState(file), undefined);
    writeFileSync(file, '{"width":"wide","height":600}');
    assert.equal(loadState(file), undefined);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
