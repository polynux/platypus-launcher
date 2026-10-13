// Runs in the sandboxed renderer before the page. It is the page's only way
// out: the three calls of the bridge (apps/ui/src/effect/bridge.ts), each a
// message to the main process, which checks it.
import { contextBridge, ipcRenderer } from 'electron';

type Reply = { ok: true; result: unknown } | { ok: false; error: string; data?: { code: string; params?: Record<string, string> } };
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

type NotifyReply = { ok: true; shown: boolean } | { ok: false; error: string };

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
    // A coded core error (issue #39, phase 3) travels with its translatable
    // payload on the thrown error's `data`, so the UI can look it up.
    if (!reply.ok) {
      const err = new Error(reply.error) as Error & { data?: unknown };
      if (reply.data !== undefined) err.data = reply.data;
      throw err;
    }
    return reply.result;
  },
  // The shell session for i18n (issue #39): the OS language list behind the
  // UI's System choice, and whether this is a dev session (the pseudo-locale
  // is offered). Main answers from Electron, not the page.
  async getSessionInfo(): Promise<{ systemLanguages: string[]; devtools: boolean }> {
    const reply = (await ipcRenderer.invoke('platypus:session-info')) as
      | { ok: true; result: { systemLanguages: string[]; devtools: boolean } }
      | { ok: false; error: string };
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
  // Desktop notifications for chat (issue #12): the page asks, main decides
  // and shows (focus, the launcher switch, per-conversation coalescing). The
  // reply says whether a notification went out.
  async notifyChat(msg: ChatNotifyPayload): Promise<boolean> {
    const reply = (await ipcRenderer.invoke('platypus:notify-chat', msg)) as NotifyReply;
    if (!reply.ok) throw new Error(reply.error);
    return reply.shown;
  },
  // A clicked notification opens its conversation: main focuses the window and
  // sends its target here; the UI opens it through its own navigation.
  onChatOpen(cb: (target: ChatOpenTarget) => void): () => void {
    const fn = (_e: unknown, target: ChatOpenTarget) => cb(target);
    ipcRenderer.on('platypus:chat-open', fn);
    return () => {
      ipcRenderer.removeListener('platypus:chat-open', fn);
    };
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
