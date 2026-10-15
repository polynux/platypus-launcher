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
  error?: CoreErrorBody;
}

/** The JSON-RPC error body the core sends (internal/corerpc). Data carries a
 *  translatable core error (issue #39, phase 3): the code and its
 *  already-formatted params. The message stays the English text. */
export interface CoreErrorBody {
  code: number;
  message: string;
  data?: CoreErrorData;
}

/** The translatable payload of a core error. */
export interface CoreErrorData {
  code: string;
  params?: Record<string, string>;
}

/** A core failure with its translatable payload attached, if it had one. */
export function coreError(message: string, data?: CoreErrorData): Error & { data?: CoreErrorData } {
  const err = new Error(message) as Error & { data?: CoreErrorData };
  if (data !== undefined) err.data = data;
  return err;
}

/** The translatable payload riding on an error, if any. */
export function errorData(err: unknown): CoreErrorData | undefined {
  return (err as { data?: CoreErrorData } | null | undefined)?.data;
}

type RequestHandler = (params: unknown) => Promise<unknown>;

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

  constructor(binary: string, args: string[] = [], env: NodeJS.ProcessEnv = process.env) {
    // The core must not inherit anything the shell holds: Chromium opens
    // files and sockets without close-on-exec (verified: the core once held
    // the GPU cache, Local Storage, even the remote debugger's socket), and
    // libuv does not close them at spawn. The core closes every fd but 0/1/2
    // itself at the very start of main, before the Go runtime opens its own
    // (netpoll's epoll, signal pipes), so no shell wrapper: dash only allows
    // single-digit fds in redirections ([n] is 0-9), and `exec 14>&-` dies
    // with `exec: 14: not found`. The core's own children (plugins, games)
    // get only 0/1/2 in turn: Go's os/exec closes the rest, and nothing in
    // internal/ passes ExtraFiles.
    // Env is inherited explicitly (issue #41): PLATYPUS_* reaches the core.
    this.child = spawn(binary, args, {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: { ...env },
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
        // A missing binary surfaces as a spawn error above; an early exit
        // still says what failed to start.
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
    if (m.error) p.reject(coreError(m.error.message, m.error.data));
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
