import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { Core, coreError, errorData } from './core.ts';

// A fake core: answers initialize, then a coded error, a plain error, or a
// result, speaking the core's line-based JSON-RPC (internal/corerpc).
function fakeCoreSource(): string {
  return `
const rl = require('readline').createInterface({ input: process.stdin });
rl.on('line', (line) => {
  const m = JSON.parse(line);
  const answer = (payload) => process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: m.id, ...payload }) + '\\n');
  if (m.method === 'initialize') return answer({ result: { protocol: '1.0', methods: ['CodedFail', 'PlainFail', 'Fine'], info: {} } });
  if (m.method === 'CodedFail') return answer({ error: { code: -32000, message: 'Doom is running', data: { code: 'game.running', params: { title: 'Doom' } } } });
  if (m.method === 'PlainFail') return answer({ error: { code: -32000, message: 'nope' } });
  return answer({ result: 'fine' });
});
`;
}

test("a coded core error keeps its message and its data across the shell bridge", async () => {
  const dir = mkdtempSync(path.join(tmpdir(), "platypus-core-error-"));
  const fake = path.join(dir, "fake-core.cjs");
  writeFileSync(fake, fakeCoreSource());
  const core = new Core(process.execPath, [fake]);
  try {
    await core.initialize();
    await assert.rejects(core.call("CodedFail", []), (err: unknown) => {
      assert.equal((err as Error).message, "Doom is running");
      assert.deepEqual(errorData(err), { code: "game.running", params: { title: "Doom" } });
      return true;
    });
    // A plain error travels as before, with no data attached.
    await assert.rejects(core.call("PlainFail", []), (err: unknown) => {
      assert.equal((err as Error).message, "nope");
      assert.equal(errorData(err), undefined);
      return true;
    });
    assert.equal(await core.call("Fine", []), "fine");
  } finally {
    await core.close();
  }
});

test("coreError carries data only when given", () => {
  assert.deepEqual(errorData(coreError("x", { code: "game.running" })), { code: "game.running" });
  assert.equal(errorData(coreError("x")), undefined);
  assert.equal(errorData(new Error("x")), undefined);
});
