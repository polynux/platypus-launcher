/** The new messages of one store that arrived within one window: their count and the friends they came from. */
export interface AlertBatch {
  count: number;
  names: string[];
}

/** Coalesces the message toasts per store: the messages that arrive within windowMs of the first one of a
 *  store become one batch, flushed once. dispose drops what is still waiting. */
export function createAlertCoalescer(flush: (plugin: string, batch: AlertBatch) => void, windowMs = 3000) {
  const pending = new Map<string, { batch: AlertBatch; timer: ReturnType<typeof setTimeout> }>();
  return {
    add(plugin: string, name: string) {
      let p = pending.get(plugin);
      if (!p) {
        const batch: AlertBatch = { count: 0, names: [] };
        const timer = setTimeout(() => {
          pending.delete(plugin);
          flush(plugin, batch);
        }, windowMs);
        p = { batch, timer };
        pending.set(plugin, p);
      }
      p.batch.count++;
      if (!p.batch.names.includes(name)) p.batch.names.push(name);
    },
    dispose() {
      for (const p of pending.values()) clearTimeout(p.timer);
      pending.clear();
    },
  };
}
