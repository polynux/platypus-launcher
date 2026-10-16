// `bun run test:e2e`: the scripted end-to-end suite (issue #69, slow).
// Starts the sandbox itself (its own process group), runs the four
// scenarios over CDP, stops everything by PID. The restart scenario needs
// the same sandbox dir across two app starts, so the runner owns the dir
// and passes --sandbox-dir to sandbox.sh twice; nothing else in sandbox.sh
// changed for this.
//
// Slow-labelled: needs PLATYPUS_TEST_SLOW=1, like the Go and vitest slow
// suites (docs/testing.md). Without it the suite skips instead of failing.
import { spawn, execSync } from 'node:child_process';
import { mkdtempSync, existsSync, readFileSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { Cdp } from './cdp.mjs';
import { SCENARIOS } from './scenarios.mjs';

const ROOT = path.resolve(import.meta.dirname, '..', '..', '..');

if (process.env.PLATYPUS_TEST_SLOW !== '1') {
  console.log('test:e2e: skipped (needs PLATYPUS_TEST_SLOW=1)');
  process.exit(0);
}

// MEMORY IS TIGHT (15 GB shared): one sandbox at a time, and refuse when
// the machine has less than 4 GB available.
function memAvailableGB() {
  try {
    const out = readFileSync('/proc/meminfo', 'utf8');
    const m = out.match(/MemAvailable:\s+(\d+)\s+kB/);
    if (m) return Number(m[1]) / 1024 / 1024;
  } catch {}
  return 99;
}
const avail = memAvailableGB();
console.log(`test:e2e: MemAvailable ${avail.toFixed(1)} GB`);
if (avail < 4) {
  console.error('test:e2e: refusing to start: less than 4 GB available');
  process.exit(2);
}

function startSandbox(sandboxDir) {
  return new Promise((resolve, reject) => {
    const child = spawn('bash', ['apps/desktop/scripts/sandbox.sh', '--sandbox-dir', sandboxDir, 'dev'], {
      cwd: ROOT,
      detached: true, // own process group: kill(-pid) stops Xvfb+Electron+Vite+core
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        // Debugging/Vite ports from the e2e range when pinned (task rule);
        // sandbox.sh still claims free ones per run, so parallel checkouts
        // never share them.
        PLATYPUS_VITE_PORT: process.env.PLATYPUS_VITE_PORT ?? '',
        PLATYPUS_REMOTE_DEBUGGING_PORT: process.env.PLATYPUS_REMOTE_DEBUGGING_PORT ?? '',
      },
    });
    let out = '';
    let debugPort = 0;
    const timer = setTimeout(() => reject(new Error('sandbox never printed its debug port:\n' + out.slice(-3000))), 240000);
    child.stdout.on('data', (b) => {
      const s = b.toString();
      out += s;
      process.stdout.write('[sandbox] ' + s);
      const m = out.match(/debug:\s+http:\/\/127\.0\.0\.1:(\d+)\/json\/list/);
      if (m && !debugPort) {
        debugPort = Number(m[1]);
        clearTimeout(timer);
        resolve({ child, debugPort, output: () => out });
      }
    });
    child.stderr.on('data', (b) => process.stderr.write('[sandbox:err] ' + b.toString()));
    child.on('exit', (code) => {
      if (!debugPort) {
        clearTimeout(timer);
        reject(new Error(`sandbox exited ${code} before printing its debug port:\n` + out.slice(-3000)));
      }
    });
  });
}

function stopSandbox(child, label, sandboxDir = '') {
  return new Promise((resolve) => {
    const done = () => resolve();
    if (!child || child.exitCode != null) return done();
    // First the leader only, so sandbox.sh's trap (kill HEADLESS, release
    // reservations) cascades down Xvfb/Electron/Vite/core/plugins in order.
    // The group kill below is the backstop for stragglers that outlive it.
    try {
      process.kill(child.pid, 'SIGTERM');
    } catch {}
    const killer = setTimeout(() => {
      try { process.kill(-child.pid, 'SIGKILL'); } catch {}
      // Backstop for stragglers that outlive the cascade (never a pattern
      // kill): only PIDs whose args or environment hold our sandbox dir, so
      // other checkouts are untouched. The core binary carries the dir in
      // its environment, not its args.
      if (sandboxDir) {
        try {
          const ps = execSync('ps -eo pid,args').toString();
          for (const line of ps.split('\n')) {
            if (line.includes(sandboxDir) && !line.includes('ps -eo')) {
              const pid = Number(line.trim().split(/\s+/)[0]);
              if (pid > 1 && pid !== process.pid) {
                try { process.kill(pid, 'SIGKILL'); } catch {}
              }
            }
          }
        } catch {}
        try {
          for (const name of readdirSync('/proc')) {
            if (!/^[0-9]+$/.test(name)) continue;
            const pid = Number(name);
            if (pid <= 1 || pid === process.pid) continue;
            let env = '';
            try { env = readFileSync(`/proc/${pid}/environ`, 'utf8'); } catch { continue; }
            if (env.includes(sandboxDir)) {
              try { process.kill(pid, 'SIGKILL'); } catch {}
            }
          }
        } catch {}
      }
    }, 10000);
    child.once('exit', () => { clearTimeout(killer); resolve(); });
    setTimeout(() => resolve(), 20000);
  });
}

async function runSteps(driver, sandboxDir, scenarioName, steps) {
  for (const step of steps) {
    const label = `${scenarioName}/${step.name}`;
    console.log(`test:e2e: step ${label}`);
    try {
      await step.run({ driver, sandboxDir });
      console.log(`test:e2e: ok ${label}`);
    } catch (err) {
      const shot = path.join(sandboxDir, `e2e-${scenarioName}-${step.name}.png`);
      try {
        await driver.screenshot(shot);
        console.error(`test:e2e: FAIL ${label}: ${err?.message ?? err}\n  screenshot: ${shot}`);
      } catch {
        console.error(`test:e2e: FAIL ${label}: ${err?.message ?? err} (screenshot failed)`);
      }
      const named = new Error(`e2e step ${label}: ${err?.message ?? err}`);
      named.cause = err;
      throw named;
    }
  }
}

const sandboxDir = mkdtempSync(path.join(tmpdir(), 'platypus-e2e-'));
console.log(`test:e2e: sandbox dir ${sandboxDir}`);

let failures = 0;
let sandbox = null;
let driver = null;
const t0 = Date.now();

try {
  sandbox = await startSandbox(sandboxDir);
  console.log(`test:e2e: CDP on :${sandbox.debugPort}`);
  driver = await Cdp.connect(sandbox.debugPort, { timeoutMs: 180000 });

  const byName = Object.fromEntries(SCENARIOS.map((s) => [s.name, s]));
  const lib = byName['library-smart'];
  const coll = byName['collection-restart'];
  const crash = byName['crash-winetricks'];
  const play = byName['playtime-stop'];

  // Before the restart.
  await runSteps(driver, sandboxDir, lib.name, lib.steps);
  await runSteps(driver, sandboxDir, coll.name, [coll.steps[0]]);

  // Restart on the same dir: the collection must survive it.
  driver.close();
  driver = null;
  await stopSandbox(sandbox.child, 'first', sandboxDir);
  sandbox = null;
  console.log('test:e2e: restarted sandbox on the same dir');
  sandbox = await startSandbox(sandboxDir);
  console.log(`test:e2e: CDP on :${sandbox.debugPort}`);
  driver = await Cdp.connect(sandbox.debugPort, { timeoutMs: 180000 });

  await runSteps(driver, sandboxDir, coll.name, [coll.steps[1]]);
  await runSteps(driver, sandboxDir, crash.name, crash.steps);
  await runSteps(driver, sandboxDir, play.name, play.steps);
} catch (err) {
  failures = 1;
  console.error(`test:e2e: ${err?.message ?? err}`);
} finally {
  try { driver?.close(); } catch {}
  if (sandbox) await stopSandbox(sandbox.child, 'last', sandboxDir);
  // Leftover check: list what we started (should be none).
  try {
    const ps = execSync('ps -eo pid,args | grep -E "platypus|Xvfb|electron" | grep -v grep || true').toString().trim();
    console.log(`test:e2e: leftover platypus/Xvfb/electron processes:\n${ps || '(none)'}`);
  } catch {}
}

const secs = Math.round((Date.now() - t0) / 1000);
console.log(`test:e2e: ${failures ? 'FAIL' : 'PASS'} in ${secs}s (sandbox ${sandboxDir})`);
process.exit(failures ? 1 : 0);
