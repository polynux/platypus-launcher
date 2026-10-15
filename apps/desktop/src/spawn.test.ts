import assert from 'node:assert/strict';
import { execFileSync, spawn, spawnSync } from 'node:child_process';
import { mkdtempSync, readdirSync, readlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';

// The core must not inherit the shell's sockets and files (Chromium does not
// set close-on-exec, and libuv does not close them at spawn): it closes every
// fd but 0/1/2 itself at the start of main. The old sh wrapper (`eval
// "exec $n>&-"`) died under dash past fd 9 (`exec: 14: not found`: dash only
// allows single-digit fds in redirections), which passed where /bin/sh is
// bash and broke where it is dash. So this spawns the real core binary with
// fifty inheritable fds held open, directly (as Core does) and through dash
// and sh: the core must stay alive with none of them left in every case. The
// fifty fds are held by a Python holder (os.set_inheritable), not shell
// `exec $i>file`, so the setup itself works under any shell.
test('the core closes inherited fds itself, under dash and sh too', async () => {
  const root = path.resolve(import.meta.dirname, '../../..');
  const dir = mkdtempSync(path.join(tmpdir(), 'platypus-spawn-test-'));
  const core = path.join(dir, 'platypus-core');
  const filler = path.join(dir, 'filler');
  writeFileSync(filler, 'x');
  execFileSync('go', ['build', '-o', core, './cmd/platypus-core'], { cwd: root, stdio: 'pipe' });

  const holder = path.join(dir, 'holder.py');
  writeFileSync(
    holder,
    [
      'import os, subprocess, sys, time',
      'core, filler, method = sys.argv[1], sys.argv[2], sys.argv[3]',
      'for _ in range(50):',
      '    fd = os.open(filler, os.O_WRONLY)',
      '    os.set_inheritable(fd, True)',
      'if method == "direct": argv = [core]',
      'elif method == "dash": argv = ["dash", "-c", \'exec "$0"\', core]',
      'else: argv = ["sh", "-c", \'exec "$0"\', core]',
      'p = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, close_fds=False)',
      'print(f"COREPID={p.pid}", flush=True)',
      'time.sleep(30)',
      '',
    ].join('\n'),
  );

  const methods = ['direct'];
  for (const shell of ['dash', 'sh']) {
    try {
      const r = spawnSync(shell, ['-c', 'true']);
      if (r.status === 0) methods.push(shell);
    } catch {
      // No such shell here: the direct run still covers the fix.
    }
  }
  assert.ok(methods.includes('dash') || methods.includes('sh'), 'need dash or sh to cover the shell pass-through');

  for (const method of methods) {
    const child = spawn('python3', [holder, core, filler, method], { stdio: ['ignore', 'pipe', 'pipe'] });
    let out = '';
    try {
      const pid = await new Promise<number>((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error(`no core pid from the ${method} holder`)), 15000);
        child.stdout.on('data', (b: Buffer) => {
          out += b.toString();
          const m = out.match(/COREPID=(\d+)/);
          if (m) {
            clearTimeout(timer);
            resolve(Number(m[1]));
          }
        });
        child.once('error', reject);
        child.once('exit', (code) => reject(new Error(`the ${method} holder exited (${code}): ${out}`)));
      });
      await new Promise((r) => setTimeout(r, 1000));
      let alive = true;
      try {
        process.kill(pid, 0);
      } catch {
        alive = false;
      }
      assert.equal(alive, true, `${method}: the core died: ${out}`);
      const fds = readdirSync(`/proc/${pid}/fd`).map((fd) => {
        try {
          return `${fd}->${readlinkSync(`/proc/${pid}/fd/${fd}`)}`;
        } catch {
          return `${fd}->?`;
        }
      });
      assert.deepEqual(
        fds.filter((f) => f.split('->')[1] === filler),
        [],
        `${method}: inherited fds leaked:\n${fds.join('\n')}`,
      );
      try {
        process.kill(pid, 'SIGKILL');
      } catch {
        // Already gone; the holder kill below still cleans up.
      }
    } finally {
      child.kill('SIGKILL');
    }
  }
});
