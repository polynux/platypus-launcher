import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Every catalog under locales/<lang>/<namespace>.json, bundled by Vite. A new
// namespace or language is just a new file: nothing to register here, so
// threads converting different areas never touch this module.
const catalogFiles = import.meta.glob('../locales/*/*.json', { eager: true, import: 'default' }) as Record<string, Record<string, unknown>>;

function catalogsOf(lang: string): Record<string, Record<string, unknown>> {
  const out: Record<string, Record<string, unknown>> = {};
  for (const [file, catalog] of Object.entries(catalogFiles)) {
    const m = /\/locales\/([^/]+)\/([^/]+)\.json$/.exec(file);
    if (m && m[1] === lang) out[m[2]] = catalog;
  }
  return out;
}

/** Namespaces, one per UI area (docs/translating.md). Later threads add keys
 *  to their area's file without touching the others. */
export const NAMESPACES: readonly string[] = Object.keys(catalogsOf('en')).sort();

/** Languages shipped in the UI bundle. English is the source and the fallback. */
export const SUPPORTED_LANGUAGES = ['en', 'fr'] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

/** Pseudo-locale for spotting untranslated strings and layout overflow
 *  (issue #39): generated from `en` at load time, accents plus ~30% padding.
 *  Only offered in the UI when the shell reports a dev session
 *  (PLATYPUS_DEVTOOLS); tests use it unconditionally. */
export const PSEUDO_LOCALE = 'en-XA';

const ACCENTS: Record<string, string> = {
  a: 'à', b: 'ƀ', c: 'ç', d: 'ď', e: 'ë', f: 'ƒ', g: 'ğ', h: 'ĥ', i: 'î', j: 'ĵ', k: 'ķ', l: 'ŀ', m: 'ɱ',
  n: 'ñ', o: 'ô', p: 'þ', q: 'ǫ', r: 'ŕ', s: 'š', t: 'ŧ', u: 'û', v: 'ṽ', w: 'ŵ', x: 'ẋ', y: 'ý', z: 'ž',
  A: 'À', B: 'Ɓ', C: 'Ç', D: 'Ď', E: 'Ë', F: 'Ƒ', G: 'Ğ', H: 'Ĥ', I: 'Î', J: 'Ĵ', K: 'Ķ', L: 'Ŀ', M: 'Ṃ',
  N: 'Ñ', O: 'Ô', P: 'Þ', Q: 'Ǫ', R: 'Ŕ', S: 'Š', T: 'Ŧ', U: 'Û', V: 'Ṽ', W: 'Ŵ', X: 'Ẋ', Y: 'Ý', Z: 'Ž',
};

/** Accent one string and pad it ~30% longer, e.g. "[Ŀîƀŕàŕý ~~~]".
 *  Placeholders (`{{title}}`) and tags are left byte-identical so the string
 *  still interpolates. */
export function pseudoLocalize(s: string): string {
  const parts = s.split(/(\{\{[^}]*\}\})/g);
  const accented = parts
    .map((p, i) => {
      if (i % 2 === 1) return p;
      return p.replace(/[A-Za-z]/g, (c) => ACCENTS[c] ?? c);
    })
    .join('');
  const pad = '~'.repeat(Math.max(2, Math.round(s.length * 0.3)));
  return `[${accented} ${pad}]`;
}

type Catalog = Record<string, unknown>;

function pseudoCatalog(ns: Catalog): Catalog {
  const out: Catalog = {};
  for (const [k, v] of Object.entries(ns)) {
    if (typeof v === 'string') out[k] = pseudoLocalize(v);
    else if (v && typeof v === 'object') out[k] = pseudoCatalog(v as Catalog);
    else out[k] = v;
  }
  return out;
}

const enResources: Record<string, Catalog> = catalogsOf('en');

// Languages without a file for a namespace fall back to English key by key
// (fallbackLng below).
const frResources: Record<string, Catalog> = catalogsOf('fr');

const pseudoResources: Record<string, Catalog> = Object.fromEntries(
  Object.entries(enResources).map(([ns, catalog]) => [ns, pseudoCatalog(catalog)]),
);

function baseOf(tag: string): string {
  return tag.split('-')[0]?.toLowerCase() ?? '';
}

function supportedBase(tag: string): SupportedLanguage | null {
  const b = baseOf(tag);
  return (SUPPORTED_LANGUAGES as readonly string[]).includes(b) ? (b as SupportedLanguage) : null;
}

/**
 * Pick the UI language (issue #39): an explicit launcher setting wins (with
 * base fallback, so fr-CA ships fr); otherwise the best match over the system
 * languages in order — exact, then base (fr-CA → fr); otherwise English.
 * The pseudo-locale is only reachable as an explicit setting (dev sessions).
 */
export function resolveLanguage(setting: string, system: readonly string[]): string {
  const s = setting.trim();
  if (s !== '') {
    if (s === PSEUDO_LOCALE) return PSEUDO_LOCALE;
    const hit = supportedBase(s);
    if (hit) return hit;
    // An explicit but unsupported language still falls back through the
    // system list below rather than jumping straight to English.
  }
  // RFC 4647 lookup per entry: fr-CA ships fr before a later exact en wins.
  for (const t of system.map((x) => x.toLowerCase())) {
    if ((SUPPORTED_LANGUAGES as readonly string[]).includes(t)) return t;
    const hit = supportedBase(t);
    if (hit) return hit;
  }
  return 'en';
}

/** The system languages in preference order: the shell's report first
 *  (Electron's app.getPreferredSystemLanguages via the preload bridge),
 *  then the browser's list. */
export function systemLanguages(): string[] {
  if (sessionCache && sessionCache.systemLanguages.length > 0) return [...sessionCache.systemLanguages];
  if (typeof navigator !== 'undefined') {
    if (navigator.languages && navigator.languages.length > 0) return [...navigator.languages];
    if (navigator.language) return [navigator.language];
  }
  return ['en'];
}

/** The shell session, fetched once and cached (issue #39). Outside the shell
 *  this is null: resolution falls back to the browser languages. */
let sessionCache: { systemLanguages: string[]; devtools: boolean } | null = null;

export async function loadSessionInfo(): Promise<{ systemLanguages: string[]; devtools: boolean } | null> {
  if (sessionCache) return sessionCache;
  try {
    const info = await window.platypus?.getSessionInfo?.();
    if (info) sessionCache = { systemLanguages: [...info.systemLanguages], devtools: info.devtools };
  } catch {
    // Outside the shell (or an old shell): no session info, browser fallback.
  }
  return sessionCache;
}

/** Test and boot hook: seed the session cache without a shell. */
export function setSessionInfo(info: { systemLanguages: string[]; devtools: boolean } | null): void {
  sessionCache = info;
}

/** Whether the pseudo-locale may be offered: dev sessions only
 *  (PLATYPUS_DEVTOOLS, reported by the shell). */
export function pseudoAvailable(): boolean {
  return sessionCache?.devtools === true;
}

/** Apply a language setting live: resolves it and re-renders (no restart). */
export async function applyLanguage(setting: string, system: readonly string[] = systemLanguages()): Promise<string> {
  const lang = resolveLanguage(setting, system);
  if (!i18n.hasResourceBundle(lang, 'common')) {
    await i18n.changeLanguage('en');
    document.documentElement.lang = 'en';
    return 'en';
  }
  await i18n.changeLanguage(lang);
  document.documentElement.lang = baseOf(lang);
  return lang;
}

void i18n.use(initReactI18next).init({
  resources: {
    en: enResources,
    fr: frResources,
    [PSEUDO_LOCALE]: pseudoResources,
  },
  lng: 'en',
  fallbackLng: 'en',
  ns: [...NAMESPACES],
  // Untranslated keys in a converted area show the English source, not the key.
  fallbackNS: 'common',
  interpolation: {
    // React escapes rendered text itself; i18next must not pre-escape
    // interpolation values (it turns paths into "&#x2F;…" inside aria-labels,
    // which assistive tech would read out literally). Placeholders use {{title}}.
    escapeValue: false,
  },
});

export default i18n;
