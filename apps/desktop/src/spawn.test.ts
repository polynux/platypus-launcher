import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { readdirSync, readlinkSync } from 'node:fs';
import net from 'node:net';
import { test } from 'node:test';
import { CLOSE_FDS } from './core.ts';

// The core must not inherit the shell's sockets and files (Chromium does not
// set close-on-exec, and libuv does not close them at spawn). The exact
// wrapper Core starts its child through, run here against a socket this very
// process holds open: the child may own only 0/1/2.
test('the spawn wrapper leaves the child only stdin, stdout and stderr', async () => {
  const server = net.createServer();
  await new Promise<void>((r) => server.listen(0, '127.0.0.1', r));
  const child = spawn('sh', ['-c', `${CLOSE_FDS}exec "$0" "$@"`, '/bin/sleep', '30'], { stdio: ['pipe', 'pipe', 'pipe'] });
  try {
    await new Promise((r) => setTimeout(r, 300));
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
    server.close();
  }
});
