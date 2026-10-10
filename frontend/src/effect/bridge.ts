import * as WailsApp from '../../wailsjs/go/main/App';
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime';
import type { CoreApi } from '../gen/api';

/** What the Electron shell's preload script exposes as window.platypus (electron/src/preload.ts). */
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
    // Wails injects window.go bindings into the webview at runtime.
    go?: {
      main?: {
        App?: Record<string, (...args: unknown[]) => Promise<unknown>>;
      };
    };
  }
}

/** Whether a Go core is reachable: the Electron bridge, or the Wails bindings. */
export function backendAvailable(): boolean {
  return Boolean(window.platypus || window.go?.main?.App);
}

/**
 * The Go App's methods. Under Electron (window.platypus exists) every method
 * goes through the bridge by name, typed by the generated CoreApi
 * (src/gen/api.ts, from `go run ./cmd/gen-ts`); under Wails it is the
 * generated Wails binding itself (kept until step 2 removes it).
 */
export const App: CoreApi = new Proxy({} as CoreApi, {
  get(_, name) {
    const bridge = window.platypus;
    if (bridge && typeof name === 'string') return (...args: unknown[]) => bridge.invoke(name, args);
    return WailsApp[name as keyof typeof WailsApp] as unknown;
  },
});

/** Subscribes to a core event under whichever shell runs the UI. Returns the unsubscribe function. */
export function onEvent<T>(event: string, cb: (data: T) => void): () => void {
  if (window.platypus) return window.platypus.on(event, (d) => cb(d as T));
  if (window.go?.main?.App) return EventsOn(event, cb);
  return () => {};
}

/** Opens an http(s) address in the user's browser; the caller has checked the scheme. */
export function browserOpen(url: string): void {
  if (window.platypus) window.platypus.openExternal(url);
  else BrowserOpenURL(url);
}
