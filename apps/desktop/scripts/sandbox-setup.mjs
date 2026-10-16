// `node scripts/sandbox-setup.mjs <core-binary>`: approves the echo plugin
// and logs it in, so the sandbox opens on a seeded library. It drives the
// real core over stdio JSON-RPC with the methods the Plugins view uses
// (ApprovePlugin, PluginAuthComplete) — no new core surface for the sandbox.
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';

const binary = process.argv[2];
if (!binary) {
  console.error('usage: sandbox-setup.mjs <core-binary>');
  process.exit(2);
}

const core = spawn(binary, [], { stdio: ['pipe', 'pipe', 'pipe'] });
core.stderr.on('data', (b) => process.stderr.write(b));
const pending = new Map();
let nextID = 0;
createInterface({ input: core.stdout }).on('line', (line) => {
  let m;
  try {
    m = JSON.parse(line);
  } catch {
    return;
  }
  if (typeof m?.id !== 'number' || !pending.has(m.id)) return; // events and requests need no answer here
  const p = pending.get(m.id);
  pending.delete(m.id);
  if (m.error) p.reject(new Error(`${m.error.message ?? m.error}`));
  else p.resolve(m.result);
});

function call(method, params) {
  return new Promise((resolve, reject) => {
    const id = ++nextID;
    pending.set(id, { resolve, reject });
    core.stdin.write(JSON.stringify({ jsonrpc: '2.0', id, method, params }) + '\n');
  });
}

const done = new Promise((resolve) => core.once('exit', resolve));
const timeout = setTimeout(() => {
  console.error('sandbox-setup: timed out talking to the core');
  process.exit(1);
}, 120000);

await call('initialize', [{ protocol: '1.0' }]);
await call('ApprovePlugin', ['echo']);
const status = await call('PluginAuthComplete', ['echo', { code: 'letmein' }]);
if (!status?.loggedIn) throw new Error(`echo login failed: ${JSON.stringify(status)}`);
console.log(`sandbox-setup: echo approved and logged in as ${status.user ?? 'echo-user'}`);

clearTimeout(timeout);
core.stdin.end();
await done;
