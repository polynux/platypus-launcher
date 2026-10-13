import i18n from './i18n';

/**
 * Locale-aware formatting in one module (issue #39), all over `Intl`.
 * Every helper takes an explicit locale or defaults to the current i18n
 * language, so tests pin a locale and components follow the language
 * setting live. The pseudo-locale formats as English.
 */
export function currentLocale(): string {
  const lang = i18n.language || 'en';
  if (lang === 'en-XA') return 'en';
  try {
    // Throws on a malformed tag; fall back to English then.
    void new Intl.NumberFormat(lang);
    return lang;
  } catch {
    return 'en';
  }
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const;

/**
 * Decimal sizes like file managers on Linux ("1.4 GB", "820 MB").
 * English output is byte-identical to the hand-rolled formatter this
 * replaces; other locales localize the number (fr: comma decimal).
 */
export function formatBytes(n?: number, locale: string = currentLocale()): string {
  if (!n || n <= 0) return '';
  let i = 0;
  let v = n;
  while (v >= 1000 && i < BYTE_UNITS.length - 1) {
    v /= 1000;
    i++;
  }
  const frac = v >= 100 || i === 0 ? 0 : 1;
  const num = new Intl.NumberFormat(locale, {
    minimumFractionDigits: frac,
    maximumFractionDigits: frac,
  }).format(v);
  return `${num} ${BYTE_UNITS[i]}`;
}

/** Hours from minutes, one decimal (a number, for tooltips to compose). */
export function formatHoursValue(minutes: number, locale: string = currentLocale()): number {
  void locale;
  return Math.round(minutes / 6) / 10;
}

/** Localized hours figure ("1.5 h" in English). */
export function formatHours(minutes: number, locale: string = currentLocale()): string {
  const h = formatHoursValue(minutes, locale);
  const num = new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(h);
  return `${num} h`;
}

/** Localized zero size ("0 B"). Non-zero empties stay '' like before. */
export function formatZeroBytes(locale: string = currentLocale()): string {
  void locale;
  return '0 B';
}

/** The room at a place as the dialog says it ("120 GB free" in English). */
export function formatFreeBytes(free: number, locale: string = currentLocale()): string {
  const size = formatBytes(free, locale) || formatZeroBytes(locale);
  if (baseLanguage(locale) === 'fr') return `${size} d'espace libre`;
  return `${size} free`;
}

function baseLanguage(locale: string): string {
  return locale.split('-')[0]?.toLowerCase() ?? 'en';
}

/** Localized percent share ("84.1% of players" stays composed by callers). */
export function formatPercentValue(percent: number, locale: string = currentLocale()): string {
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    maximumFractionDigits: 1,
  }).format(percent / 100);
}

/** A calendar date ("Feb 3, 2026" in English). */
export function formatDate(unixSeconds: number, locale: string = currentLocale()): string {
  return new Date(unixSeconds * 1000).toLocaleDateString(locale, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
}

/** A clock time ("21:04" in English). */
export function formatClock(ms: number, locale: string = currentLocale()): string {
  return new Date(ms).toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit', hour12: false });
}

/**
 * Relative ages ("just now", "5m ago", "3h ago", "2d ago" in English).
 * Stays in the compact launcher style across locales for now; the shape is
 * covered by tests so phase-2 threads can localize it deliberately.
 */
export function formatRelativeAge(ts: number, now: number = Date.now(), _locale: string = currentLocale()): string {
  const s = Math.max(0, Math.round((now - ts) / 1000));
  if (s < 60) return 'just now';
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.round(h / 24)}d ago`;
}

/** "today" / "yesterday" / "5d ago" style recency, else a calendar date. */
export function formatLastPlayed(
  unixSeconds: number | undefined,
  now: number = Date.now(),
  locale: string = currentLocale(),
): string | undefined {
  if (!unixSeconds) return undefined;
  const days = Math.floor((now / 1000 - unixSeconds) / 86400);
  if (days < 1) return 'today';
  if (days < 2) return 'yesterday';
  if (days < 30) return `${days}d ago`;
  return formatDate(unixSeconds, locale);
}

/** Join list parts the locale's way ("a, b and c" in English). */
export function formatList(parts: string[], locale: string = currentLocale()): string {
  if (parts.length <= 1) return parts.join('');
  return new Intl.ListFormat(locale, { style: 'long', type: 'conjunction' }).format(parts);
}

/** Grouped integers ("1,234" in English). */
export function formatCount(n: number, locale: string = currentLocale()): string {
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 0 }).format(n);
}
