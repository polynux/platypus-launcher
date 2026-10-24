# Translating Platypus (issue #39)

The UI is translated with [i18next](https://www.i18next.com/) +
[react-i18next](https://react.i18next.com/), catalogs in JSON v4 format.
English (`en`) is the source and the fallback everywhere.

## Layout

Catalogs live in `apps/ui/src/locales/<lang>/<namespace>.json`, one
namespace per UI area:

| namespace   | area                                              | phase |
|-------------|---------------------------------------------------|-------|
| `common`    | shared buttons (Save, Cancel, Close, Back, …)     | 1     |
| `nav`       | sidebar, navigation, top-level view titles        | 1     |
| `settings`  | Launcher settings view                            | 1     |
| `game`      | game page, sideloading, launching (App chrome)    | 1 (toasts) |
| `store`     | embedded store view (titles)                      | 1 (titles) |
| `downloads` | install/update/uninstall queue (App notices)      | 1 (notices) |
| `library`   | library view                                      | 2     |
| `plugins`   | plugins view                                      | 2     |
| `chat`      | friends, chat, notifications                      | 2     |
| `errors`    | error fallbacks shown in toasts                   | 1     |
| `gameTone`  | the game colour switch (Accent / Whole app)       | 2     |
| …           | dashboard, activity, fixes, runners, game page…   | 2     |

Untranslated keys fall back to English key by key, so a partial locale
already works. The Electron shell's own strings (native dialogs, error
boxes) live separately in `apps/desktop/src/locales/<lang>.json` — a tiny
catalog on purpose, no i18next in main (see `apps/desktop/src/locale.ts`).

## Adding a language

1. Copy the `en` folder: `cp -r apps/ui/src/locales/en apps/ui/src/locales/<lang>`
   (use the BCP 47 tag, e.g. `de`, `pt-BR`).
2. Translate the values. Leave `{{placeholders}}` and tags byte-identical.
3. Add the tag to `SUPPORTED_LANGUAGES` in `apps/ui/src/lib/i18n.ts` and a
   `catalogsOf('<lang>')` resources entry next to `fr`. Catalog files are
   picked up by `import.meta.glob`: a new namespace file needs no import.
4. Add the shell strings: copy `apps/desktop/src/locales/en.json` to
   `<lang>.json`, translate, register in `SHELL_LANGUAGES`
   (`apps/desktop/src/locale.ts`).
5. Run `bun run i18n:check`: it prints per-locale completeness, e.g.
   `locale de: 42/137 keys (31%)`.
6. Open a PR. French (`fr`) is the proof locale for phase 1; every converted
   area ships complete French.

## Key and plural rules

- Keys are semantic and namespaced (`library.sort.year`), never English
  sentences. Always call `t` with the namespace prefix: `t('nav:library')`,
  even when the hook already defaults to that namespace — the check script
  only tracks fully-qualified keys.
- JSON files use nested objects; dots address them
  (`{ "sort": { "year": "…" } }` → `library.sort.year`).
- Placeholders use i18next's `{{title}}` interpolation. Values are escaped by
  React at render; keep `escapeValue: false` (see `lib/i18n.ts` — `true`
  corrupts values such as file paths inside `aria-label`s).
- Plurals use JSON v4 suffixes `_one` / `_other` (selected by
  `Intl.PluralRules`, so French treats 0 and 1 as singular):
  ```json
  { "games_one": "{{count}} game", "games_other": "{{count}} games" }
  ```
  ```tsx
  t('common:games', { count: n })
  ```

## Language resolution

The launcher setting `language` (`""` = System, the default) is stored in
the core (`internal/settings/language.go`, surfaced as
`LauncherSettings.language`). Resolution (`resolveLanguage` in
`apps/ui/src/lib/i18n.ts`, mirrored in `apps/desktop/src/locale.ts`):

1. explicit setting wins, with base fallback (`fr-CA` ships `fr`);
2. otherwise RFC 4647 lookup over the system list in order — per entry,
   exact then base, so `fr-CA` ships `fr` before a later exact `en` wins;
3. otherwise English.

System languages come from the shell (`app.getPreferredSystemLanguages()`
via the `platypus:session-info` IPC, exposed to the page as
`window.platypus.getSessionInfo()`), falling back to `navigator.languages`.
Switching applies live through `applyLanguage` — no restart.

## Pseudo-locale

`en-XA` is generated at load time from `en` (accents + ~30% padding, e.g.
`[Ŀîƀŕàŕý ~~~]`), so untranslated strings and layout overflow show up at a
glance. It is only offered in the language picker when the shell reports a
dev session (`PLATYPUS_DEVTOOLS=1`, see `registerSessionIPC` in
`apps/desktop/src/main.ts`). Tests use it unconditionally via
`pseudoLocalize` / explicit `resolveLanguage('en-XA', …)`.

## The check script

`bun run i18n:check` runs two scripts (`apps/ui/scripts/`):

- `i18n-check.mjs` — keys used in code but missing from `en` fail the run;
  keys in `en` never used are warnings (the `common` buttons are a deliberate
  reserve for phase 2); per-locale completeness is informational. Dynamic
  keys (`t(\`downloads:${k}\``)) mark their whole namespace used. The root
  `test` script runs it, so missing keys fail CI the same way tests do.
  Fixture tests live in `apps/ui/scripts/i18n-check.test.mjs`.
- `i18n-lint.mjs` — fails on hardcoded JSX text or literal `aria-label` /
  `placeholder` / `title` / `label` props in converted files. A file opts in
  with a `// i18n: converted` comment in its first 15 lines; there is no
  shared list to edit. Escape hatch for the rare legitimate literal:
  `{/* i18n:allow: <reason> */}` on the same or the previous line. Brand
  names (`Platypus`) and language endonyms (`English`, `Français`) stay
  literal. The text matcher is heuristic (it skips TypeScript generics);
  if it flags code, prefer restructuring the line over an exemption.

## Converting a component (phase-2 guide)

1. Pick your area's namespace file(s) and add the English strings there.
   One area per thread — never touch another area's namespace file.
2. In the component:
   ```tsx
   import { useTranslation } from 'react-i18next';
   const { t } = useTranslation('library'); // your area's namespace
   // …
   <h1>{t('library:title')}</h1>           // fully-qualified key, always
   <Button aria-label={t('library:sort.label')}>{t('common:save')}</Button>
   ```
   Shared buttons come from `common`; everything else from your namespace.
3. Counts and plurals go through keys with `{ count }`, never ternaries:
   `t('library:games', { count: n })` with `_one`/`_other` in the catalog.
4. Dates, sizes, durations and lists go through `apps/ui/src/lib/format.ts`
   (`formatBytes`, `formatDate`, `formatHours`, `formatList`, …), which takes
   an optional locale defaulting to the current language. English output is
   pinned by tests — keep it byte-identical and add the new locale's case.
5. Add French for every key you add (you are responsible for your area's
   French; the lead reviews it as a native speaker).
6. Put `// i18n: converted` at the top of every file you converted, so the
   lint guards it. Sentences with inline markup (links, `<code>`, bold) use
   react-i18next's `<Trans>` with one key, never a sentence split into
   `before`/`after` keys: word order differs between languages.
7. Test: render under the default `en` (the test setup initializes i18n with
   `en` and resets it between tests), assert the English strings as before,
   and add one live-switching test per area (change to `fr`, assert the
   French, change back) like
   `apps/ui/src/components/__tests__/sidebar-i18n.test.tsx`.
8. Run `bun run i18n:check`, `bun --filter platypus-ui test`, and
   `bun --filter platypus-ui typecheck` before committing.
