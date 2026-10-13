import { describe, it, expect } from 'vitest';
import {
  formatBytes,
  formatClock,
  formatFreeBytes,
  formatCount,
  formatDate,
  formatHours,
  formatHoursValue,
  formatLastPlayed,
  formatList,
  formatPercentValue,
  formatRelativeAge,
} from './format';

describe('formatBytes', () => {
  it('keeps the English output byte-identical', () => {
    expect(formatBytes(undefined, 'en')).toBe('');
    expect(formatBytes(0, 'en')).toBe('');
    expect(formatBytes(999, 'en')).toBe('999 B');
    expect(formatBytes(3_700_000, 'en')).toBe('3.7 MB');
    expect(formatBytes(30e9, 'en')).toBe('30.0 GB');
    expect(formatBytes(120e9, 'en')).toBe('120 GB');
    expect(formatBytes(820e6, 'en')).toBe('820 MB');
    expect(formatBytes(1e6, 'en')).toBe('1.0 MB');
  });

  it('localizes the room suffix in French', () => {
    expect(formatFreeBytes(120e9, 'fr')).toContain("d'espace libre");
    expect(formatFreeBytes(120e9, 'en')).toBe('120 GB free');
  });

  it('localizes the number in French (comma decimal)', () => {
    const bytes = formatBytes(3_700_000, 'fr');
    expect(bytes).toBe(
      `${new Intl.NumberFormat('fr', { minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(3.7)} MB`,
    );
    expect(bytes).toContain(',');
    expect(formatBytes(120e9, 'fr')).toBe(
      `${new Intl.NumberFormat('fr', { minimumFractionDigits: 0, maximumFractionDigits: 0 }).format(120)} GB`,
    );
  });
});

describe('playtime', () => {
  it('keeps the English hour figures', () => {
    expect(formatHoursValue(30)).toBe(0.5);
    expect(formatHours(30, 'en')).toBe('0.5 h');
    expect(formatHours(150, 'en')).toBe('2.5 h');
    expect(formatHours(0, 'en')).toBe('0 h');
  });

  it('localizes the hour figure in French', () => {
    expect(formatHours(30, 'fr')).toBe(
      `${new Intl.NumberFormat('fr', { maximumFractionDigits: 1 }).format(0.5)} h`,
    );
  });

  it('formats the global share like before in English', () => {
    expect(formatPercentValue(84.14, 'en')).toBe('84.1%');
  });
});

describe('dates and times', () => {
  it('matches Intl exactly per locale', () => {
    for (const locale of ['en', 'fr']) {
      expect(formatDate(1700, locale)).toBe(
        new Date(1700 * 1000).toLocaleDateString(locale, { month: 'short', day: 'numeric', year: 'numeric' }),
      );
    }
    // French months are lowercase words, English abbreviated capitals: proof
    // the locale actually flows through.
    expect(formatDate(1767225600, 'fr')).toMatch(/janv/i);
  });

  it('formats clock times per locale', () => {
    const ts = new Date(2026, 9, 13, 21, 4).getTime();
    expect(formatClock(ts, 'en')).toBe(
      new Date(ts).toLocaleTimeString('en', { hour: '2-digit', minute: '2-digit', hour12: false }),
    );
  });

  it('keeps the compact relative ages', () => {
    const now = 1_000_000_000_000;
    expect(formatRelativeAge(now - 10_000, now)).toBe('just now');
    expect(formatRelativeAge(now - 5 * 60_000, now)).toBe('5m ago');
    expect(formatRelativeAge(now - 3 * 3_600_000, now)).toBe('3h ago');
    expect(formatRelativeAge(now - 2 * 86_400_000, now)).toBe('2d ago');
  });

  it('labels recency, then falls back to the date', () => {
    const now = 1_700_000_000_000;
    expect(formatLastPlayed(undefined, now)).toBeUndefined();
    expect(formatLastPlayed(1_699_999_000, now)).toBe('today');
    expect(formatLastPlayed(1_699_900_000, now)).toBe('yesterday');
    expect(formatLastPlayed(1_699_000_000, now)).toBe('11d ago');
  });
});

describe('lists and counts', () => {
  it('joins the locale way', () => {
    expect(formatList(['a', 'b', 'c'], 'en')).toBe(
      new Intl.ListFormat('en', { style: 'long', type: 'conjunction' }).format(['a', 'b', 'c']),
    );
    expect(formatList(['x'], 'en')).toBe('x');
  });

  it('groups integers per locale', () => {
    expect(formatCount(1234, 'en')).toBe(new Intl.NumberFormat('en', { maximumFractionDigits: 0 }).format(1234));
  });
});
