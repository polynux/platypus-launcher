// Image URLs that failed to load (HTTP 404, e.g. Steam's portrait
// library_600x900.jpg for older titles), so the UI doesn't request a
// known-bad cover on every render. Entries expire after MISS_TTL_MS: the
// store may add the art later, and a stale miss must not hide it forever.
// In-memory first (shared across components), persisted to localStorage so
// a relaunch doesn't refetch every known 404.

export const COVER_MISS_TTL_MS = 30 * 86400 * 1000;
const STORAGE_KEY = 'platypus.coverMiss.v1';
const MAX_MISSES = 500;

export interface MissClock {
  now(): number;
}

export interface MissStore {
  load(): Record<string, number>;
  save(misses: Record<string, number>): void;
}

function memoryStore(): MissStore {
  let kept: Record<string, number> = {};
  return {
    load: () => ({ ...kept }),
    save: (m) => {
      kept = { ...m };
    },
  };
}

function localStorageStore(): MissStore {
  const mem = memoryStore();
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) mem.save(JSON.parse(raw) as Record<string, number>);
  } catch {
    // Private mode, no DOM, or a corrupt entry: memory only.
  }
  return {
    load: () => mem.load(),
    save: (m) => {
      mem.save(m);
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(m));
      } catch {
        // Quota or private mode: the memory copy still applies.
      }
    },
  };
}

function storage(): MissStore {
  try {
    if (typeof localStorage !== 'undefined') return localStorageStore();
  } catch {
    // No DOM (SSR): memory only.
  }
  return memoryStore();
}

/** A miss cache with an injectable clock and store, so tests run offline. */
export function createMissCache(
  opts: { ttlMs?: number; clock?: MissClock; store?: MissStore } = {},
) {
  const ttlMs = opts.ttlMs ?? COVER_MISS_TTL_MS;
  const clock = opts.clock ?? { now: () => Date.now() };
  const store = opts.store ?? storage();
  let misses = store.load();

  const prune = (at: number): boolean => {
    let changed = false;
    for (const [url, at2] of Object.entries(misses)) {
      if (at - at2 >= ttlMs) {
        delete misses[url];
        changed = true;
      }
    }
    return changed;
  };

  return {
    /** Whether url failed recently enough to skip requesting it. */
    isBad(url: string): boolean {
      const at = misses[url];
      if (at == null) return false;
      if (clock.now() - at >= ttlMs) {
        delete misses[url];
        return false;
      }
      return true;
    },
    /** Remember that url just failed. */
    markBad(url: string): void {
      misses[url] = clock.now();
      prune(clock.now());
      // Cap the size so one bad library can't grow the stored map
      // without bound; the oldest misses go first.
      const keys = Object.keys(misses);
      if (keys.length > MAX_MISSES) {
        const drop = keys
          .sort((a, b) => misses[a] - misses[b])
          .slice(0, keys.length - MAX_MISSES);
        for (const k of drop) delete misses[k];
      }
      store.save(misses);
    },
    /** Forget everything (tests, and a future "retry art" action). */
    clear(): void {
      misses = {};
      store.save(misses);
    },
  };
}

export type MissCache = ReturnType<typeof createMissCache>;

/** The app-wide miss cache behind the Cover component. */
export const coverMisses: MissCache = createMissCache();
