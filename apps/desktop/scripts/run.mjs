// `node scripts/run.mjs dev|start`: builds the Go core and the shell, then runs
// the desktop app. `dev` serves the UI from Vite (hot reload); `start` uses
// apps/ui/dist (built if missing, or with --rebuild).
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import net from 'node:net';
import path from 'node:path';

const mode = process.argv[2];
if (mode !== 'dev' && mode !== 'start') {
  console.error('usage: run.mjs dev|start [--rebuild]');
  process.exit(2);
}
const rebuild = process.argv.includes('--rebuild');
const here = path.resolve(import.meta.dirname, '..');
const repo = path.resolve(here, '..', '..');
const frontend = path.join(repo, 'apps', 'ui');
const run = (cmd, args, cwd, env = {}) => execFileSync(cmd, args, { cwd, stdio: 'inherit', env: { ...process.env, ...env } });

// Electron: PLATYPUS_ELECTRON, else the npm package's download, else the system one.
function electronBinary() {
  if (process.env.PLATYPUS_ELECTRON) return process.env.PLATYPUS_ELECTRON;
  try {
    const rel = readFileSync(path.join(here, 'node_modules/electron/path.txt'), 'utf8').trim();
    const bin = path.join(here, 'node_modules/electron/dist', rel);
    if (existsSync(bin)) return bin;
  } catch {}
  return 'electron';
}

function freePort() {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.once('error', reject);
    s.listen(0, '127.0.0.1', () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
  });
}

async function waitFor(port, ms = 30000) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    const ok = await new Promise((r) => {
      const c = net.connect(port, '127.0.0.1', () => (c.destroy(), r(true)));
      c.once('error', () => r(false));
    });
    if (ok) return;
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`nothing listening on ${port}`);
}

console.log('» building the Go core');
run('go', ['build', '-o', path.join(here, '.build/platypus-core'), './cmd/platypus-core'], repo);
console.log('» building the shell');
run('node', ['scripts/build.mjs'], here);

const env = { ...process.env, PLATYPUS_CORE: path.join(here, '.build/platypus-core') };
// Set when this runs from inside another Electron app (an IDE terminal): it would make Electron run our main script as plain Node.
delete env.ELECTRON_RUN_AS_NODE;
const children = [];
const stopAll = () => children.forEach((c) => c.kill('SIGTERM'));
process.on('SIGINT', stopAll);
process.on('SIGTERM', stopAll);

if (mode === 'dev') {
  const port = await freePort();
  console.log(`» starting Vite on ${port}`);
  const vite = spawn('bunx', ['vite', '--port', String(port), '--strictPort', '--host', '127.0.0.1'], {
    cwd: frontend,
    stdio: 'inherit',
    env: { ...process.env, PLATYPUS_VITE_PORT: String(port) },
  });
  children.push(vite);
  await waitFor(port);
  env.PLATYPUS_DEV_URL = `http://127.0.0.1:${port}`;
  env.PLATYPUS_DEVTOOLS = '1';
} else if (rebuild || !existsSync(path.join(frontend, 'dist/index.html'))) {
  console.log('» building the UI');
  run('bun', ['run', 'build'], frontend);
}

console.log('» starting Electron');
// PLATYPUS_ELECTRON_ARGS: extra Chromium/Electron flags, e.g. "--inspect=9229 --disable-gpu".
const extra = (process.env.PLATYPUS_ELECTRON_ARGS ?? '').split(/\s+/).filter(Boolean);
const electron = spawn(electronBinary(), [...extra, path.join(here, 'dist/main.cjs')], { stdio: 'inherit', env });
children.push(electron);
electron.on('exit', (code) => {
  stopAll();
  process.exit(code ?? 0);
});
