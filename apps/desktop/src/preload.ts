// Runs in the sandboxed renderer before the page. It is the page's only way
// out: the three calls of the bridge (apps/ui/src/effect/bridge.ts), each a
// message to the main process, which checks it.
import { contextBridge, ipcRenderer } from 'electron';

type Reply = { ok: true; result: unknown } | { ok: false; error: string };

const listeners = new Map<string, Set<(data: unknown) => void>>();

ipcRenderer.on('platypus:event', (_e, name: string, data: unknown) => {
  for (const cb of listeners.get(name) ?? []) cb(data);
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
});
