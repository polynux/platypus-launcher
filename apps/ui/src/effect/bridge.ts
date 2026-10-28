import type { CoreApi } from '../gen/api';

/** What the desktop shell's preload script exposes as window.platypus (apps/desktop/src/preload.ts). */
export interface PlatypusBridge {
  /** Calls an exported method of the Go core by name; args are its positional arguments. */
  invoke(method: string, args: unknown[]): Promise<unknown>;
  /** Subscribes to a core event. Returns the unsubscribe function. */
  on(event: string, cb: (data: unknown) => void): () => void;
  /** Opens an http(s) address in the user's browser. The shell refuses anything else. */
  openExternal(url: string): void;
  /** A dropped file's real path (Electron removed File.path). Absent outside the shell. */
  pathForFile?: (file: File) => string;
  /** The shell session: OS languages and dev mode (issue #39). Absent outside the shell. */
  getSessionInfo?: () => Promise<SessionInfo>;
  /** Asks the shell for a desktop notification for a chat message; main
   *  decides (focus, the launcher switch, coalescing). Absent outside the shell. */
  notifyChat?: (msg: ChatNotifyPayload) => Promise<boolean>;
  /** Subscribes to clicked-notification opens. Absent outside the shell. */
  onChatOpen?: (cb: (target: ChatOpenTarget) => void) => () => void;
  /** The embedded store view (issue #17): the native view over the Store page. Absent outside the shell. */
  store?: StoreBridge;
}

/** What the shell reports about this session (issue #39). */
export interface SessionInfo {
  /** The OS language list in preference order (Electron's getPreferredSystemLanguages). */
  systemLanguages: string[];
  /** True in a dev session (PLATYPUS_DEVTOOLS): the pseudo-locale is offered. */
  devtools: boolean;
}

/** A chat message asking the shell for a desktop notification (issue #12):
 *  display names and the message text only, never tokens or ids. */
export interface ChatNotifyPayload {
  plugin: string;
  friendId: string;
  /** The friend's name with the store. */
  title: string;
  /** The friend's display name, for the coalesced count. */
  name: string;
  /** The message text. */
  body: string;
  /** The conversation on screen, if any. */
  open: { plugin: string; friendId: string } | null;
}

/** The conversation a clicked notification opens. */
export interface ChatOpenTarget {
  plugin: string;
  friendId: string;
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
  kind: 'navigated' | 'loading' | 'blocked' | 'failed';
  pluginId: string;
  url: string;
  canGoBack: boolean;
  canGoForward: boolean;
  title: string;
  loading: boolean;
  /** 'failed' only: the main-frame load error. */
  error?: string;
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

/** Asks the shell for a desktop notification for a chat message (issue #12).
 *  Resolves false outside the shell, or when main kept it (focused, switched
 *  off, the open conversation). Never rejects. */
export function notifyChat(msg: ChatNotifyPayload): Promise<boolean> {
  return window.platypus?.notifyChat?.(msg).catch(() => false) ?? Promise.resolve(false);
}

/** Subscribes to clicked-notification opens (issue #12). Outside the shell it returns a no-op unsubscribe. */
export function onChatOpen(cb: (target: ChatOpenTarget) => void): () => void {
  return window.platypus?.onChatOpen?.(cb) ?? (() => {});
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
