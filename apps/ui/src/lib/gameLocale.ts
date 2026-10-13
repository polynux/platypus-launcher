import { currentLocale, formatDate } from './format';

/**
 * Local display helpers for the game page and library areas (issue #39).
 * lib/format.ts is shared and must stay untouched, but it has no compact
 * decimal and its formatLastPlayed answers in English words — so this area
 * carries its own, and reports it.
 */

/** A compact decimal with at most one fraction digit ("2.5" in English,
 *  "2,5" in French): for the long-standing "{hours}h" figures, where
 *  formatHours' "1.5 h" wording would change the English strings. */
export function formatDecimal1(n: number, locale: string = currentLocale()): string {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(n);
}

/** The recency of a timestamp as data, mirroring formatLastPlayed's bands
 *  ("today", "yesterday", under 30 days, else a calendar date). Components
 *  map the kind to their own catalog keys (game:recency.* / library:recency.*)
 *  so the words translate; dates go through format.ts. */
export type Recency =
  | { kind: 'today' }
  | { kind: 'yesterday' }
  | { kind: 'daysAgo'; count: number }
  | { kind: 'date'; at: number };

export function recencyOf(unixSeconds: number | undefined, now: number = Date.now()): Recency | undefined {
  if (!unixSeconds) return undefined;
  const days = Math.floor((now / 1000 - unixSeconds) / 86400);
  if (days < 1) return { kind: 'today' };
  if (days < 2) return { kind: 'yesterday' };
  if (days < 30) return { kind: 'daysAgo', count: days };
  return { kind: 'date', at: unixSeconds };
}

/** Render a recency through the caller's catalog namespace: `prefix` is
 *  'game:recency' on the game page, 'library:recency' in the library. */
export function renderRecency(
  t: (key: string, vars?: Record<string, unknown>) => string,
  prefix: string,
  r: Recency,
  locale: string = currentLocale(),
): string {
  switch (r.kind) {
    case 'today':
      return t(`${prefix}.today`);
    case 'yesterday':
      return t(`${prefix}.yesterday`);
    case 'daysAgo':
      return t(`${prefix}.daysAgo`, { count: r.count });
    case 'date':
      return formatDate(r.at, locale);
  }
}
