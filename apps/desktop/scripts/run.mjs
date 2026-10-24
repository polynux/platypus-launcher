// `node scripts/run.mjs dev|start`: builds the Go core and the shell, then runs
// the desktop app. `dev` serves the UI from Vite (hot reload); `start` uses
// apps/ui/dist (rebuilt when missing or older than the UI sources, or with --rebuild).
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, readdirSync, readFileSync, realpathSync, statSync } from 'node:fs';
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
  const pkg = path.join(here, 'node_modules/electron');
  const bin = () => {
    try {
      const rel = readFileSync(path.join(pkg, 'path.txt'), 'utf8').trim();
      const candidate = path.join(pkg, 'dist', rel);
      return existsSync(candidate) ? candidate : '';
    } catch { return ''; }
  };
  if (existsSync(pkg)) {
    let resolved;
    try { resolved = realpathSync(pkg); } catch {}
    if (resolved) {
      let binary = bin();
      if (binary) return binary;
      const version = JSON.parse(readFileSync(path.join(resolved, 'package.json'), 'utf8')).version;
      console.error(`» downloading Electron ${version} (its install script never ran)…`);
      try {
        const childEnv = { ...process.env };
        delete childEnv.ELECTRON_RUN_AS_NODE;
        execFileSync(process.execPath, ['install.js'], { cwd: resolved, stdio: 'inherit', env: childEnv });
        binary = bin();
        if (binary) return binary;
        console.error('» Electron install script completed, but its binary is still missing.');
      } catch (error) {
        console.error(`» Electron install script failed${error.status == null ? `: ${error.message}` : ` (exit ${error.status})`}.`);
      }
    } else {
      console.error('» Electron package is present but its real path could not be resolved.');
    }
  } else {
    console.error('» Electron package is not installed.');
  }
  console.error('» Falling back to system electron, which may be too old. Fix with `bun install` or `(cd apps/desktop/node_modules/electron && node install.js)`.');
  return 'electron';
}

// The UI build is stale when it is missing or any of its inputs changed after
// it: otherwise `start` would keep serving an old bundle after a pull or merge.
function uiStale() {
  const built = path.join(frontend, 'dist/index.html');
  if (!existsSync(built)) return true;
  const at = statSync(built).mtimeMs;
  const newer = (p) => {
    const s = statSync(p);
    if (!s.isDirectory()) return s.mtimeMs > at;
    return readdirSync(p).some((name) => newer(path.join(p, name)));
  };
  const inputs = ['src', 'public', 'index.html', 'package.json', 'vite.config.ts', 'tsconfig.json'];
  return inputs.map((p) => path.join(frontend, p)).some((p) => existsSync(p) && newer(p));
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
console.log('» building the bundled plugins');
run('node', ['scripts/bundled-plugins.mjs', path.join(here, '.build/bundled-plugins')], here);
console.log('» building the shell');
run('node', ['scripts/build.mjs'], here);

const env = {
  ...process.env,
  PLATYPUS_CORE: path.join(here, '.build/platypus-core'),
  // Where the core finds the bundled steam plugin (issue #54).
  PLATYPUS_BUNDLED_PLUGINS: path.join(here, '.build/bundled-plugins'),
  // Explicit, so the page never depends on how Electron judges "packaged".
  PLATYPUS_FRONTEND_DIST: path.join(frontend, 'dist'),
};
// Set when this runs from inside another Electron app (an IDE terminal): it would make Electron run our main script as plain Node.
delete env.ELECTRON_RUN_AS_NODE;
const children = [];
const stopAll = () => children.forEach((c) => c.kill('SIGTERM'));
process.on('SIGINT', stopAll);
process.on('SIGTERM', stopAll);

if (mode === 'dev') {
  // PLATYPUS_VITE_PORT pins the port (the sandbox uses 5193 so parallel
  // runs don't fight over the default); otherwise a free one is picked.
  const port = Number(process.env.PLATYPUS_VITE_PORT) || await freePort();
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
} else if (rebuild || uiStale()) {
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
