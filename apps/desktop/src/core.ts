import { spawn, type ChildProcessByStdio } from 'node:child_process';
import type { Readable, Writable } from 'node:stream';
import { createInterface } from 'node:readline';

/** The core protocol this shell speaks (internal/corerpc.Protocol). A minor newer than the core's is refused there. */
export const PROTOCOL = '1.0';

export interface CoreInfo {
  /** The core's art server (127.0.0.1) and the token its requests must carry. */
  artURL: string;
  artToken: string;
}

interface Pending {
  resolve: (v: unknown) => void;
  reject: (e: Error) => void;
}

interface Message {
  id?: number | string | null;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: { code: number; message: string };
}

type RequestHandler = (params: unknown) => Promise<unknown>;

/**
 * Closes every inherited file descriptor but stdin/stdout/stderr, then execs
 * the core (`$0` is its path, `$@` its arguments). Runs inside `sh -c`, so no
 * script file of its own can leak. Deliberately no redirections inside the
 * loop: redirecting stderr (`2>/dev/null`) makes the shell stash stderr in a
 * spare fd, and the loop then closes that very fd — stderr stays pointed at
 * /dev/null for the core. Single-threaded there is no race to guard against,
 * so every listed fd still exists when closed; the existence check only keeps
 * dash honest. The loop is POSIX (dash-safe) and /proc is a given on Linux.
 */
export const CLOSE_FDS =
  'for fd in /proc/self/fd/*; do n=${fd##*/}; ' +
  'case $n in 0|1|2) ;; *) if [ -e /proc/self/fd/"$n" ]; then eval "exec $n>&-"; fi;; esac; done; ';

/** The Go core as a child process, spoken to in JSON-RPC 2.0, one object per line (internal/corerpc). */
export class Core {
  private child: ChildProcessByStdio<Writable, Readable, Readable>;
  private nextID = 0;
  private pending = new Map<number, Pending>();
  private exited: Promise<number | null>;
  private closing = false;
  private started = false;

  /** Methods the core serves, from initialize. */
  methods = new Set<string>();
  info: CoreInfo = { artURL: '', artToken: '' };

  /** Events the core emits: {name, data}. */
  onEvent: (name: string, data: unknown) => void = () => {};
  /** Requests the core makes of the shell (dialogs), by method. */
  handlers = new Map<string, RequestHandler>();
  /** The core process ended without being asked to. */
  onCrash: (code: number | null) => void = () => {};

  constructor(binary: string, args: string[] = []) {
    // The core must not inherit anything the shell holds: Chromium opens
    // files and sockets without close-on-exec (verified: the core once held
    // the GPU cache, Local Storage, even the remote debugger's socket), and
    // libuv does not close them at spawn. So the core is started through a
    // shell one-liner that closes every fd but 0/1/2 and only then execs it;
    // the loop is POSIX (dash-safe) and /proc is a given on Linux. The core's
    // own children (plugins, games) get only 0/1/2 in turn: Go's os/exec
    // closes the rest, and nothing in internal/ passes ExtraFiles.
    this.child = spawn('sh', ['-c', `${CLOSE_FDS}exec "$0" "$@"`, binary, ...args], {
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    this.child.stderr.on('data', (b: Buffer) => process.stderr.write(b));
    this.child.stdin.on('error', () => {}); // a dying core is reported through exit
    createInterface({ input: this.child.stdout }).on('line', (line) => this.read(line));
    this.exited = new Promise((resolve) => {
      this.child.once('error', (err) => {
        this.failAll(new Error(`could not start the core (${binary}): ${err.message}`));
        resolve(null);
      });
      this.child.once('exit', (code) => {
        // Through the sh wrapper a missing binary surfaces as exit 127, not
        // a spawn error: say what failed to start.
        this.failAll(this.started ? new Error('the core stopped') : new Error(`the core exited during startup (exit ${code}): ${binary}`));
        if (!this.closing) this.onCrash(code);
        resolve(code);
      });
    });
  }

  /** The version handshake; the core starts the App while answering. */
  async initialize(): Promise<void> {
    const hello = (await this.call('initialize', [{ protocol: PROTOCOL }])) as { protocol: string; methods: string[]; info?: Partial<CoreInfo> };
    this.methods = new Set(hello.methods);
    this.info = { artURL: hello.info?.artURL ?? '', artToken: hello.info?.artToken ?? '' };
    this.started = true;
  }

  call(method: string, params: unknown[]): Promise<unknown> {
    return new Promise((resolve, reject) => {
      if (this.child.stdin.destroyed || this.closing) return reject(new Error('the core is not running'));
      const id = ++this.nextID;
      this.pending.set(id, { resolve, reject });
      this.send({ jsonrpc: '2.0', id, method, params });
    });
  }

  /**
   * Ends the stream, which makes the core shut the App down (stop games it
   * started, plugins, a final cloud sync), and waits for it to exit.
   */
  async close(graceMs = 15000): Promise<void> {
    this.closing = true;
    this.child.stdin.end();
    const timer = setTimeout(() => this.child.kill('SIGKILL'), graceMs);
    await this.exited;
    clearTimeout(timer);
  }

  private send(m: object): void {
    this.child.stdin.write(JSON.stringify(m) + '\n');
  }

  private read(line: string): void {
    let m: Message;
    try {
      m = JSON.parse(line) as Message;
    } catch {
      console.error('core: unreadable line:', line.slice(0, 200));
      return;
    }
    if (m.method !== undefined) {
      if (m.id === undefined || m.id === null) {
        if (m.method === 'event') {
          const e = m.params as { name: string; data: unknown };
          this.onEvent(e.name, e.data);
        }
        return;
      }
      void this.answer(m.id, m.method, m.params);
      return;
    }
    if (typeof m.id !== 'number') return;
    const p = this.pending.get(m.id);
    if (!p) return;
    this.pending.delete(m.id);
    if (m.error) p.reject(new Error(m.error.message));
    else p.resolve(m.result);
  }

  private async answer(id: number | string, method: string, params: unknown): Promise<void> {
    const h = this.handlers.get(method);
    if (!h) return this.send({ jsonrpc: '2.0', id, error: { code: -32601, message: `unknown method ${method}` } });
    try {
      this.send({ jsonrpc: '2.0', id, result: (await h(params)) ?? null });
    } catch (err) {
      this.send({ jsonrpc: '2.0', id, error: { code: -32000, message: err instanceof Error ? err.message : String(err) } });
    }
  }

  private failAll(err: Error): void {
    for (const p of this.pending.values()) p.reject(err);
    this.pending.clear();
  }
}
