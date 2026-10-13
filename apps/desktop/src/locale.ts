import en from './locales/en.json' with { type: 'json' };
import fr from './locales/fr.json' with { type: 'json' };

/**
 * The shell's own message catalog (issue #39): native dialogs, menus and
 * error boxes in main. Tiny on purpose — no React, no i18next in main.
 * Same language resolution as the UI (explicit setting, then the OS list
 * with base fallback, then English); the UI owns the full catalogs.
 */
export const SHELL_LANGUAGES = ['en', 'fr'] as const;

const catalogs: Record<string, Record<string, string>> = { en, fr };

function baseOf(tag: string): string {
  return tag.split('-')[0]?.toLowerCase() ?? '';
}

/** Pick the shell language: the launcher setting wins (base fallback, so
 *  fr-CA ships fr), then the OS list in order (exact, then base), else en. */
export function resolveShellLanguage(setting: string, system: readonly string[]): string {
  const s = setting.trim();
  if (s !== '') {
    const base = baseOf(s);
    if ((SHELL_LANGUAGES as readonly string[]).includes(base)) return base;
  }
  // RFC 4647 lookup per entry: fr-CA ships fr before a later exact en wins.
  for (const t of system.map((x) => x.toLowerCase())) {
    if ((SHELL_LANGUAGES as readonly string[]).includes(t)) return t;
    const base = baseOf(t);
    if ((SHELL_LANGUAGES as readonly string[]).includes(base)) return base;
  }
  return 'en';
}

/** Render a shell string, filling `{{name}}` placeholders. Unknown keys fall
 *  back to English; unknown languages fall back to English. */
export function shellText(lang: string, key: string, params: Record<string, string | number> = {}): string {
  const catalog = catalogs[lang] ?? {};
  const template = catalog[key] ?? (catalogs.en[key] ?? key);
  return template.replace(/\{\{(\w+)\}\}/g, (_m, name: string) =>
    params[name] !== undefined ? String(params[name]) : `{{${name}}}`,
  );
}
