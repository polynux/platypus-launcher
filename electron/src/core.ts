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

/** The Go core as a child process, spoken to in JSON-RPC 2.0, one object per line (internal/corerpc). */
export class Core {
  private child: ChildProcessByStdio<Writable, Readable, Readable>;
  private nextID = 0;
  private pending = new Map<number, Pending>();
  private exited: Promise<number | null>;
  private closing = false;

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
    this.child = spawn(binary, args, { stdio: ['pipe', 'pipe', 'pipe'] });
    this.child.stderr.on('data', (b: Buffer) => process.stderr.write(b));
    this.child.stdin.on('error', () => {}); // a dying core is reported through exit
    createInterface({ input: this.child.stdout }).on('line', (line) => this.read(line));
    this.exited = new Promise((resolve) => {
      this.child.once('error', (err) => {
        this.failAll(new Error(`could not start the core (${binary}): ${err.message}`));
        resolve(null);
      });
      this.child.once('exit', (code) => {
        this.failAll(new Error('the core stopped'));
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
