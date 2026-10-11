import { describe, it, expect } from 'vitest';
import { createMissCache, COVER_MISS_TTL_MS } from './coverMisses';

function memStore() {
  let kept: Record<string, number> = {};
  return {
    load: () => ({ ...kept }),
    save: (m: Record<string, number>) => {
      kept = { ...m };
    },
    kept: () => kept,
  };
}

describe('cover miss cache', () => {
  it('marks a failed URL bad so it is skipped without a request', () => {
    const c = createMissCache({ store: memStore() });
    expect(c.isBad('http://cdn/a.jpg')).toBe(false);
    c.markBad('http://cdn/a.jpg');
    expect(c.isBad('http://cdn/a.jpg')).toBe(true);
    expect(c.isBad('http://cdn/b.jpg')).toBe(false);
  });

  it('forgets misses after the TTL, so later-added art shows', () => {
    let at = 1_000_000;
    const store = memStore();
    const c = createMissCache({ ttlMs: 60_000, clock: { now: () => at }, store });
    c.markBad('http://cdn/a.jpg');
    expect(c.isBad('http://cdn/a.jpg')).toBe(true);
    at += 59_999;
    expect(c.isBad('http://cdn/a.jpg')).toBe(true);
    at += 1;
    expect(c.isBad('http://cdn/a.jpg')).toBe(false);
  });

  it('persists misses across instances (a relaunch keeps them)', () => {
    const store = memStore();
    createMissCache({ store }).markBad('http://cdn/a.jpg');
    expect(createMissCache({ store }).isBad('http://cdn/a.jpg')).toBe(true);
  });

  it('caps the stored map so one bad library cannot grow it', () => {
    const store = memStore();
    const c = createMissCache({ store });
    for (let i = 0; i < 600; i++) c.markBad(`http://cdn/${i}.jpg`);
    expect(Object.keys(store.kept()).length).toBeLessThanOrEqual(500);
  });

  it('clears every miss', () => {
    const c = createMissCache({ store: memStore() });
    c.markBad('http://cdn/a.jpg');
    c.clear();
    expect(c.isBad('http://cdn/a.jpg')).toBe(false);
  });

  it('ships a 30-day TTL', () => {
    expect(COVER_MISS_TTL_MS).toBe(30 * 86400 * 1000);
  });
});
