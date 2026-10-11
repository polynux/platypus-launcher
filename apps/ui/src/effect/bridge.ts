import type { CoreApi } from '../gen/api';

/** What the desktop shell's preload script exposes as window.platypus (apps/desktop/src/preload.ts). */
export interface PlatypusBridge {
  /** Calls an exported method of the Go core by name; args are its positional arguments. */
  invoke(method: string, args: unknown[]): Promise<unknown>;
  /** Subscribes to a core event. Returns the unsubscribe function. */
  on(event: string, cb: (data: unknown) => void): () => void;
  /** Opens an http(s) address in the user's browser. The shell refuses anything else. */
  openExternal(url: string): void;
  /** The embedded store view (issue #17): the native view over the Store page. Absent outside the shell. */
  store?: StoreBridge;
}

/** The embedded store view's bridge: open / bounds / history / close, plus its navigation events. */
export interface StoreBridge {
  /** Opens the plugin's store (the storefront, or a deep link inside its domains). Rejects when refused. */
  open(pluginId: string, url?: string): Promise<void>;
  /** Places the native view over the Store page's content area. */
  setBounds(rect: StoreBounds): void;
  back(): void;
  forward(): void;
  reload(): void;
  /** Hides the native view, keeping it alive for a fast return. Names its
   * plugin so a stale close never hides the newly shown view. */
  close(pluginId?: string): void;
  /** Subscribes to the view's navigation events. Returns the unsubscribe function. */
  onEvent(cb: (ev: StoreEvent) => void): () => void;
}

/** The content-area rect the native store view fills. */
export interface StoreBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** A navigation event from the embedded store view. 'blocked' carries the
 * attempted outside address: the toolbar offers to open it in the browser. */
export interface StoreEvent {
  kind: 'navigated' | 'loading' | 'blocked';
  pluginId: string;
  url: string;
  canGoBack: boolean;
  canGoForward: boolean;
  title: string;
  loading: boolean;
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

/** The embedded store view's bridge, or null outside the shell. */
export function storeBridge(): StoreBridge | null {
  return window.platypus?.store ?? null;
}

/** Subscribes to the embedded store view's navigation events. Outside the shell it returns a no-op unsubscribe. */
export function onStoreEvent(cb: (ev: StoreEvent) => void): () => void {
  const store = window.platypus?.store;
  if (!store) return () => {};
  return store.onEvent(cb);
}

function backendUnavailable(op: string): Error {
  return new Error(
    `Platypus backend not available (${op}). The UI must run inside the app — see docs/electron.md, not a plain vite server.`
  );
}
