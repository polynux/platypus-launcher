import { describe, it, expect } from 'vitest';
import i18next from 'i18next';
import enCommon from '../locales/en/common.json';
import i18n, {
  PSEUDO_LOCALE,
  applyLanguage,
  loadSessionInfo,
  pseudoAvailable,
  pseudoLocalize,
  resolveLanguage,
  setSessionInfo,
  systemLanguages,
} from './i18n';

describe('resolveLanguage', () => {
  it('prefers the explicit setting, with base fallback', () => {
    expect(resolveLanguage('fr', ['en'])).toBe('fr');
    expect(resolveLanguage('fr-CA', ['en'])).toBe('fr');
    expect(resolveLanguage('en', ['fr'])).toBe('en');
    expect(resolveLanguage(' fr ', ['en'])).toBe('fr');
  });

  it('looks up the system list per entry, exact then base', () => {
    expect(resolveLanguage('', ['fr-CA', 'en'])).toBe('fr');
    expect(resolveLanguage('', ['en-US'])).toBe('en');
    expect(resolveLanguage('', ['de', 'fr'])).toBe('fr');
    expect(resolveLanguage('', ['de', 'it'])).toBe('en');
    expect(resolveLanguage('', [])).toBe('en');
  });

  it('falls through an unsupported explicit setting to the system list', () => {
    expect(resolveLanguage('de', ['fr'])).toBe('fr');
    expect(resolveLanguage('de', ['ja'])).toBe('en');
  });

  it('reaches the pseudo-locale only as an explicit setting', () => {
    expect(resolveLanguage(PSEUDO_LOCALE, ['en'])).toBe(PSEUDO_LOCALE);
    expect(resolveLanguage('', [PSEUDO_LOCALE])).toBe('en');
  });
});

describe('pseudoLocalize', () => {
  it('ships a generated en-XA bundle from English', async () => {
    expect(i18n.hasResourceBundle(PSEUDO_LOCALE, 'common')).toBe(true);
    await i18n.changeLanguage(PSEUDO_LOCALE);
    const out = i18n.t('nav:library');
    expect(out.startsWith('[')).toBe(true);
    expect(out).not.toBe('Library');
    await i18n.changeLanguage('en');
  });
  it('accents, pads ~30% and wraps in brackets, keeping placeholders', () => {
    const out = pseudoLocalize('Library {{count}}');
    expect(out.startsWith('[')).toBe(true);
    expect(out.endsWith(']')).toBe(true);
    expect(out).toContain('{{count}}');
    expect(out).not.toContain('Library');
    // "Library {{count}}" is 17 chars: padding is ~5 tildes.
    const pad = out.match(/~+/)?.[0].length ?? 0;
    expect(pad).toBeGreaterThanOrEqual(4);
    expect(pad).toBeLessThanOrEqual(7);
  });
});

describe('pluralization (JSON v4 _one/_other via Intl.PluralRules)', () => {
  async function pluralInstance(lng: string) {
    const inst = i18next.createInstance();
    await inst.init({
      lng,
      fallbackLng: 'en',
      ns: ['translation'],
      defaultNS: 'translation',
      resources: {
        en: { translation: { games_one: '{{count}} game', games_other: '{{count}} games' } },
        fr: { translation: { games_one: '{{count}} jeu', games_other: '{{count}} jeux' } },
      },
      interpolation: { escapeValue: true },
    });
    return inst;
  }

  it('selects one/other in English', async () => {
    const inst = await pluralInstance('en');
    expect(inst.t('games', { count: 1 })).toBe('1 game');
    expect(inst.t('games', { count: 0 })).toBe('0 games');
    expect(inst.t('games', { count: 2 })).toBe('2 games');
  });

  it('treats 0 and 1 as singular in French', async () => {
    const inst = await pluralInstance('fr');
    expect(inst.t('games', { count: 1 })).toBe('1 jeu');
    expect(inst.t('games', { count: 0 })).toBe('0 jeu');
    expect(inst.t('games', { count: 2 })).toBe('2 jeux');
  });

  it('reads the shipped catalogs the same way', async () => {
    // The shipped JSON plus a plural pair, on a fresh instance: proves the
    // catalog format pluralizes without mutating the shared test instance.
    const inst = i18next.createInstance();
    await inst.init({
      lng: 'en',
      fallbackLng: 'en',
      ns: ['common'],
      defaultNS: 'common',
      resources: {
        en: {
          common: {
            ...(enCommon as Record<string, string>),
            i18nTestGames_one: '{{count}} game',
            i18nTestGames_other: '{{count}} games',
          },
        },
      },
      interpolation: { escapeValue: true },
    });
    expect(inst.t('i18nTestGames', { count: 1 })).toBe('1 game');
    expect(inst.t('i18nTestGames', { count: 5 })).toBe('5 games');
  });
});

describe('session info and live switching', () => {
  it('falls back to the browser languages without a shell', () => {
    setSessionInfo(null);
    expect(systemLanguages().length).toBeGreaterThan(0);
    expect(pseudoAvailable()).toBe(false);
  });

  it('caches the shell report for resolution and the pseudo-locale gate', async () => {
    setSessionInfo({ systemLanguages: ['fr-CA'], devtools: true });
    expect(systemLanguages()).toEqual(['fr-CA']);
    expect(pseudoAvailable()).toBe(true);
    expect(await loadSessionInfo()).toEqual({ systemLanguages: ['fr-CA'], devtools: true });
    setSessionInfo(null);
  });

  it('applyLanguage resolves, switches and sets <html lang>', async () => {
    setSessionInfo({ systemLanguages: ['fr'], devtools: false });
    try {
      await expect(applyLanguage('')).resolves.toBe('fr');
      expect(i18n.language).toBe('fr');
      expect(document.documentElement.lang).toBe('fr');
      expect(i18n.t('nav:library')).toBe('Bibliothèque');
      await expect(applyLanguage('en')).resolves.toBe('en');
      expect(i18n.t('nav:library')).toBe('Library');
    } finally {
      setSessionInfo(null);
      await i18n.changeLanguage('en');
    }
  });
});
