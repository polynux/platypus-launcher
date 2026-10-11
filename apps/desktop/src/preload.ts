// Runs in the sandboxed renderer before the page. It is the page's only way
// out: the three calls of the bridge (apps/ui/src/effect/bridge.ts), each a
// message to the main process, which checks it.
import { contextBridge, ipcRenderer } from 'electron';

type Reply = { ok: true; result: unknown } | { ok: false; error: string };
type StoreReply = { ok: true } | { ok: false; error: string };

export interface StoreBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface StoreEvent {
  kind: 'navigated' | 'loading' | 'blocked';
  pluginId: string;
  url: string;
  canGoBack: boolean;
  canGoForward: boolean;
  title: string;
  loading: boolean;
}

const listeners = new Map<string, Set<(data: unknown) => void>>();
const storeListeners = new Set<(ev: StoreEvent) => void>();

ipcRenderer.on('platypus:event', (_e, name: string, data: unknown) => {
  for (const cb of listeners.get(name) ?? []) cb(data);
});

ipcRenderer.on('platypus:store-event', (_e, ev: StoreEvent) => {
  for (const cb of storeListeners) cb(ev);
});

contextBridge.exposeInMainWorld('platypus', {
  async invoke(method: string, args: unknown[]): Promise<unknown> {
    const reply = (await ipcRenderer.invoke('platypus:invoke', method, args)) as Reply;
    if (!reply.ok) throw new Error(reply.error);
    return reply.result;
  },
  on(event: string, cb: (data: unknown) => void): () => void {
    let set = listeners.get(event);
    if (!set) listeners.set(event, (set = new Set()));
    set.add(cb);
    return () => {
      set.delete(cb);
    };
  },
  openExternal(url: string): void {
    ipcRenderer.send('platypus:openExternal', url);
  },
  // The embedded store view (issue #17): the native view over the Store
  // page's content area. Main validates every call (known enabled plugin,
  // addresses inside its domains).
  store: {
    async open(pluginId: string, url?: string): Promise<void> {
      const reply = (await ipcRenderer.invoke('platypus:store-open', pluginId, url)) as StoreReply;
      if (!reply.ok) throw new Error(reply.error);
    },
    setBounds(rect: StoreBounds): void {
      ipcRenderer.send('platypus:store-bounds', rect);
    },
    back(): void {
      ipcRenderer.send('platypus:store-back');
    },
    forward(): void {
      ipcRenderer.send('platypus:store-forward');
    },
    reload(): void {
      ipcRenderer.send('platypus:store-reload');
    },
    close(pluginId?: string): void {
      ipcRenderer.send('platypus:store-close', pluginId);
    },
    onEvent(cb: (ev: StoreEvent) => void): () => void {
      storeListeners.add(cb);
      return () => {
        storeListeners.delete(cb);
      };
    },
  },
});
