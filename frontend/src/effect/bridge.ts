import type { CoreApi } from '../gen/api';

/** What the desktop shell's preload script exposes as window.platypus (apps/desktop/src/preload.ts). */
export interface PlatypusBridge {
  /** Calls an exported method of the Go core by name; args are its positional arguments. */
  invoke(method: string, args: unknown[]): Promise<unknown>;
  /** Subscribes to a core event. Returns the unsubscribe function. */
  on(event: string, cb: (data: unknown) => void): () => void;
  /** Opens an http(s) address in the user's browser. The shell refuses anything else. */
  openExternal(url: string): void;
}

declare global {
  interface Window {
    platypus?: PlatypusBridge;
  }
}

/** Whether the Go core is reachable through the desktop shell. */
export function backendAvailable(): boolean {
  return Boolean(window.platypus);
}

/** The Go App's methods, by name through the bridge, typed by the generated
 *  CoreApi (src/gen/api.ts, from `go run ./cmd/gen-ts`). Outside the shell
 *  every call fails with an explicit error instead of silently faking data. */
export const App: CoreApi = new Proxy({} as CoreApi, {
  get(_, name) {
    if (typeof name !== 'string') return undefined;
    return (...args: unknown[]) => {
      const bridge = window.platypus;
      if (!bridge) throw backendUnavailable(name);
      return bridge.invoke(name, args);
    };
  },
});

/** Subscribes to a core event. Outside the shell it returns a no-op unsubscribe. */
export function onEvent<T>(event: string, cb: (data: T) => void): () => void {
  if (window.platypus) return window.platypus.on(event, (d) => cb(d as T));
  return () => {};
}

/** Opens an http(s) address in the user's browser; the caller has checked the scheme. */
export function browserOpen(url: string): void {
  const bridge = window.platypus;
  if (!bridge) throw backendUnavailable('openExternal');
  bridge.openExternal(url);
}

function backendUnavailable(op: string): Error {
  return new Error(
    `Platypus backend not available (${op}). The UI must run inside the app — see docs/electron.md, not a plain vite server.`
  );
}
