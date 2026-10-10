import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { readdirSync, readlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { CLOSE_FDS } from './core.ts';

// The core must not inherit the shell's sockets and files (Chromium does not
// set close-on-exec, and libuv does not close them at spawn). The exact
// wrapper Core starts its child through, run here with fifty non-close-on-exec
// fds held open (a shell opens files without CLOEXEC, like Chromium does):
// the grandchild may own only 0/1/2.
test('the spawn wrapper leaves the child only stdin, stdout and stderr', async () => {
  const filler = path.join(tmpdir(), 'platypus-spawn-test-filler');
  writeFileSync(filler, 'x');
  const open =
    `i=10; while [ "$i" -lt 60 ]; do eval "exec $i>${filler}"; i=$((i + 1)); done; ` +
    `${CLOSE_FDS}exec /bin/sleep 30`;
  const child = spawn('sh', ['-c', open], { stdio: ['pipe', 'pipe', 'pipe'] });
  try {
    await new Promise((r) => setTimeout(r, 500));
    const fds = readdirSync(`/proc/${child.pid}/fd`).map((fd) => {
      try {
        return `${fd}->${readlinkSync(`/proc/${child.pid}/fd/${fd}`)}`;
      } catch {
        return `${fd}->?`;
      }
    });
    assert.deepEqual(
      fds.map((f) => f.split('->')[0]),
      ['0', '1', '2'],
      fds.join('\n'),
    );
  } finally {
    child.kill('SIGKILL');
  }
});
