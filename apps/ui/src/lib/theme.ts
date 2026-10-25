// Themes at runtime (issue #93). The core resolves a theme's tokens; this
// module sets them as CSS custom properties on :root. Components keep their
// Tailwind classes: `bg-amber-300` compiles to `var(--color-amber-300)`,
// `rounded-2xl` to `var(--radius-2xl)`, so variables set here restyle them.
// Token names are the ones in apps/ui/src/style.css (@theme and :root).
import { Effect } from 'effect';
import { useEffect, useState, useSyncExternalStore } from 'react';
import { LauncherApi } from '../effect/runtime';
import type { ThemeLayout, ThemeTokens, WashLayer } from '../types';
import { VARIANT_DEFAULTS, type VariantAxis, type VariantValues } from '../gen/catalog';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';

/** The theme the app ships with. Matches the core's default (themes.DefaultID). */
export const DEFAULT_THEME_ID = 'warm';

const CACHE_KEY = 'platypus.theme';
let currentVariants: VariantValues = { ...VARIANT_DEFAULTS };
const variantListeners = new Set<() => void>();

/** Reads one resolved catalog choice ("entry.axis", see gen/catalog.ts) and
 *  updates when the selected theme changes. */
export function useVariant<A extends VariantAxis>(axis: A): VariantValues[A] {
  return useSyncExternalStore(
    (listener) => { variantListeners.add(listener); return () => variantListeners.delete(listener); },
    () => currentVariants[axis],
    () => VARIANT_DEFAULTS[axis],
  );
}

/** Runs the listener on every applied theme, like the variant hook. Layouts
 *  arrive over their own call, so a view refetches its layout here. */
export function onThemeApplied(listener: () => void): () => void {
  variantListeners.add(listener);
  return () => { variantListeners.delete(listener); };
}

/** Where each token lands. `--color-*` are the @theme names; the rest are the
 *  variables the ds/ components and Tailwind's radius classes read. */
export const TOKEN_VARS: Record<string, string[]> = {
  ground: ['--color-ground', '--background'],
  'ground-2': ['--color-ground-2'],
  raised: ['--color-raised', '--card'],
  surface: ['--color-surface', '--secondary', '--muted'],
  'surface-strong': ['--color-surface-strong'],
  hairline: ['--color-hairline', '--border', '--input'],
  'hairline-strong': ['--color-hairline-strong'],
  popover: ['--popover', '--color-popover'],
  glass: ['--color-glass'],
  text: ['--color-text', '--card-foreground', '--popover-foreground'],
  'text-body': ['--color-text-body'],
  'text-muted': ['--color-text-muted', '--muted-foreground'],
  'text-faint': ['--color-text-faint'],
  accent: ['--color-accent', '--primary'],
  'accent-strong': ['--color-accent-strong'],
  'accent-dim': ['--color-accent-dim'],
  'on-accent': ['--color-on-accent', '--primary-foreground', '--accent-foreground'],
  ok: ['--color-ok'],
  attention: ['--color-attention'],
  info: ['--color-info'],
  dl: ['--color-dl'],
  danger: ['--color-danger', '--destructive'],
  focus: ['--color-focus', '--ring'],
  'radius-chip': ['--radius-xl'],
  'radius-card': ['--radius-2xl'],
  'radius-panel': ['--radius-3xl'],
  'font-sans': ['--font-sans'],
  'font-mono': ['--font-mono'],
};

/** Colour schemes (issue #134): dark is the theme's own tokens, light and
 *  black are the [schemes.*] overrides. Themes without schemes are dark only. */
export type ColorScheme = 'dark' | 'light' | 'black';

const SCHEME_CACHE_KEY = 'platypus.scheme';
let schemeChoice: ColorScheme = 'dark';
let effectiveScheme: ColorScheme = 'dark';
let currentThemeTokens: ThemeTokens | null = null;
const schemeListeners = new Set<() => void>();

function normalizeScheme(s: string | null | undefined): ColorScheme {
  return s === 'light' ? 'light' : s === 'black' ? 'black' : 'dark';
}

/** The stored scheme pick, dark when nothing is stored. */
export function getColorSchemeChoice(): ColorScheme {
  return schemeChoice;
}

/** The scheme actually painted: the stored pick when the theme declares it,
 *  else dark. Dark is always valid: it is the theme's own tokens. */
export function getEffectiveScheme(): ColorScheme {
  return effectiveScheme;
}

function computeEffective(choice: ColorScheme, theme: ThemeTokens | null): ColorScheme {
  if (choice === 'dark') return 'dark';
  if (theme?.schemes?.[choice] && Object.keys(theme.schemes[choice]).length > 0) return choice;
  return 'dark';
}

function notifyScheme(): void {
  for (const listener of schemeListeners) listener();
}

/** Reads the stored scheme pick and updates when it changes. */
export function useColorScheme(): ColorScheme {
  return useSyncExternalStore(
    (listener) => { schemeListeners.add(listener); return () => schemeListeners.delete(listener); },
    () => schemeChoice,
    () => 'dark' as ColorScheme,
  );
}

/** The schemes the applied theme declares (dark plus its overrides). */
export function useThemeSchemes(): ColorScheme[] {
  const [, tick] = useState(0);
  useEffect(() => {
    const bump = (): void => tick((n) => n + 1);
    schemeListeners.add(bump);
    variantListeners.add(bump);
    return () => { schemeListeners.delete(bump); variantListeners.delete(bump); };
  }, []);
  const out: ColorScheme[] = ['dark'];
  if (currentThemeTokens?.schemes?.light) out.push('light');
  if (currentThemeTokens?.schemes?.black) out.push('black');
  return out;
}

/** The stored scheme pick, or dark when nothing is stored. */
export function loadColorSchemeChoice(): Promise<ColorScheme> {
  try {
    return Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((s) => {
        const next = normalizeScheme((s as { color_scheme?: string })?.color_scheme ?? '');
        schemeChoice = next;
        if (currentThemeTokens) effectiveScheme = computeEffective(next, currentThemeTokens);
        notifyScheme();
        return next;
      })
      .catch(() => schemeChoice);
  } catch {
    return Promise.resolve(schemeChoice);
  }
}

/** Saves the scheme pick. A failed write keeps what is on screen. */
export function saveColorSchemeChoice(next: ColorScheme): Promise<void> {
  schemeChoice = next;
  try {
    window.localStorage.setItem(SCHEME_CACHE_KEY, next);
  } catch {
    // No storage: the next boot starts dark.
  }
  if (currentThemeTokens) effectiveScheme = computeEffective(next, currentThemeTokens);
  notifyScheme();
  // Repaint with the new scheme at once; the settings answer confirms it.
  if (currentThemeTokens) applyTheme(currentThemeTokens);
  try {
    return Effect.runPromise(LauncherApi.setColorScheme(next)).then(() => {}).catch(() => {});
  } catch {
    return Promise.resolve();
  }
}

/** Tailwind 4's amber ramp as exact offsets from the accent (amber-400):
 *  [step, lightness, chroma factor, hue]. Each step is `oklch(from accent …)`,
 *  so an alternate accent brings its own ramp. For the warm accent the steps
 *  are Tailwind's amber. */
const AMBER_RAMP: [number, number, number, number][] = [
  [50, 0.159, 0.1164021164, 10.848],
  [100, 0.134, 0.3121693122, 11.188],
  [200, 0.096, 0.6349206349, 11.317],
  [300, 0.051, 0.8941798942, 7.176],
  [400, 0, 1, 0],
  [500, -0.059, 0.9947089947, -14.349],
  [600, -0.162, 0.9470899471, -26.111],
  [700, -0.273, 0.8624338624, -35.431],
  [800, -0.355, 0.7248677249, -38.228],
  [900, -0.414, 0.5925925926, -38.525],
  [950, -0.549, 0.4074074074, -38.794],
];

/** `calc(l + 0.051)`, or `calc(l - 0.059)`: a signed offset of a channel. */
function shifted(channel: 'l' | 'h', by: number): string {
  return `calc(${channel} ${by < 0 ? '-' : '+'} ${Math.abs(by)})`;
}

/** One radial gradient per layer, the wash's CSS. A layer with an image draws
 *  the picture under its gradient, so a theme's art tints like its colours.
 *  The image is a bare url(): background-image layers can't carry position or
 *  size (those belong to the background shorthand), and anything more would
 *  invalidate the whole declaration. Layers are composed from validated
 *  parts, so nothing here can carry more than the core allowed. */
export function washImage(layers: WashLayer[]): string {
  if (layers.length === 0) return 'none';
  return layers
    .map((l) => {
      const gradient = `radial-gradient(${l.size} at ${l.at}, ${l.color}, transparent ${l.fade})`;
      if (!l.image) return gradient;
      return `${gradient}, url("${l.image}")`;
    })
    .join(', ');
}

/** Registers a plugin theme's fonts (issue #105): each shipped family becomes
 *  a FontFace from its art URL, so a font token naming it paints at once.
 *  A failed load stays silent for the user (the stack falls back) but warns
 *  in the console with the family and URL, so a theme author can find it. */
export function loadThemeFonts(fonts?: Record<string, string>): void {
  if (!fonts) return;
  try {
    for (const [family, href] of Object.entries(fonts)) {
      if (!family || !href) continue;
      const face = new FontFace(family, `url("${href}") format("woff2")`, {});
      void face.load().then((loaded) => document.fonts.add(loaded)).catch(() => {
        console.warn(`theme font "${family}" failed to load from ${href}`);
      });
    }
  } catch {
    // No FontFace (an old engine, a test without fonts): the stacks fall back.
  }
}

/** Every custom property the theme sets, name to value. Pure: no DOM.
 *  The scheme overrides the theme's tokens (issue #134): dark is the theme
 *  itself, light and black are the [schemes.*] tables. */
export function themeVariables(theme: ThemeTokens, scheme: ColorScheme = 'dark'): Record<string, string> {
  const tokens: Record<string, string> = { ...(theme.tokens ?? {}) };
  const over = scheme === 'dark' ? undefined : theme.schemes?.[scheme];
  if (over) for (const [token, value] of Object.entries(over)) tokens[token] = value;
  const vars: Record<string, string> = {};
  for (const [token, names] of Object.entries(TOKEN_VARS)) {
    const value = tokens[token];
    if (value) for (const name of names) vars[name] = value;
  }
  for (const [step, dl, kc, dh] of AMBER_RAMP) {
    vars[`--color-amber-${step}`] = `oklch(from var(--color-accent) ${shifted('l', dl)} calc(c * ${kc}) ${shifted('h', dh)})`;
  }
  vars['--app-wash'] = washImage(theme.wash ?? []);
  return vars;
}

/** The variables the theme applied last set on :root, by name. The game tone
 *  reads them to put the theme's own values back when it clears (issue #123). */
let appliedThemeVars: Record<string, string> = {};

/** The variables the applied theme set on :root, name to value. Empty until
 *  the first theme is applied, when style.css's own values stand. Read-only:
 *  the game tone restores these instead of removing them (issue #123). */
export function currentThemeVariables(): Record<string, string> {
  return appliedThemeVars;
}

/** Sets the theme on :root. Live: calling it again restyles what is on screen. */
export function applyTheme(theme: ThemeTokens): void {
  currentThemeTokens = theme;
  effectiveScheme = computeEffective(schemeChoice, theme);
  const style = document.documentElement.style;
  const vars = themeVariables(theme, effectiveScheme);
  // Kept before the listeners run, so a tone repainted from one of them
  // restores this theme's values, not the previous theme's.
  appliedThemeVars = vars;
  for (const [name, value] of Object.entries(vars)) style.setProperty(name, value);
  // Native controls and scrollbars follow the scheme (issue #134).
  style.setProperty('color-scheme', effectiveScheme === 'light' ? 'light' : 'dark');
  try {
    document.documentElement.dataset.scheme = effectiveScheme;
  } catch {
    // No DOM (a test without a document): the variables still apply.
  }
  loadThemeFonts(theme.fonts);
  // The core sends every axis resolved; a cached theme from an older build
  // may lack new axes, which keep their built-in value.
  currentVariants = { ...VARIANT_DEFAULTS, ...(theme.variants ?? {}) } as VariantValues;
  for (const listener of variantListeners) listener();
  notifyScheme();
}

/** Applies a theme and remembers it, so the next boot can paint it before the
 *  core answers (see applyCachedTheme). */
export function applyAndRemember(theme: ThemeTokens): void {
  applyTheme(theme);
  try {
    window.localStorage.setItem(CACHE_KEY, JSON.stringify(theme));
  } catch {
    // No storage: the next boot starts from the defaults and loads the pick.
  }
}

/** Boot, before the first paint (main.tsx): the theme applied last time. With
 *  nothing cached, style.css's own values stand, and those are the warm theme. */
export function applyCachedTheme(): void {
  try {
    const rawScheme = window.localStorage.getItem(SCHEME_CACHE_KEY);
    if (rawScheme === 'light' || rawScheme === 'black' || rawScheme === 'dark') schemeChoice = rawScheme;
  } catch {
    // No storage: the scheme starts dark.
  }
  try {
    const raw = window.localStorage.getItem(CACHE_KEY);
    if (raw) applyTheme(JSON.parse(raw) as ThemeTokens);
  } catch {
    // A missing or broken entry leaves the defaults in place.
  }
}

/** Loads the picked theme from the core and applies it. A failed read keeps
 *  what is applied: a theme is never a reason to break startup (or tests with
 *  a partial core mock, which throws here instead of rejecting). */
export function loadTheme(): Promise<void> {
  const loadScheme = (): Promise<void> => {
    try {
      return Effect.runPromise(LauncherApi.getLauncherSettings())
        .then((s) => {
          const next = normalizeScheme((s as { color_scheme?: string })?.color_scheme ?? '');
          schemeChoice = next;
          try {
            window.localStorage.setItem(SCHEME_CACHE_KEY, next);
          } catch {
            // No storage: the next boot starts dark.
          }
          if (currentThemeTokens) applyTheme(currentThemeTokens);
        })
        .catch(() => {});
    } catch {
      return Promise.resolve();
    }
  };
  try {
    return Effect.runPromise(LauncherApi.getTheme(''))
      .then((theme) => {
        applyAndRemember(theme);
        return loadScheme();
      })
      .catch(() => loadScheme());
  } catch {
    return loadScheme();
  }
}

/** Reads one view's validated layout from the core: the theme's own layout
 *  when it ships one, else the built-in arrangement (issue #108). Null only
 *  for an unknown view, or when the read fails: the view then keeps showing
 *  its built-in arrangement, like a failed theme read keeps the applied theme. */
export function loadThemeLayout(view: string): Promise<ThemeLayout | null> {
  try {
    return Effect.runPromise(LauncherApi.getThemeLayout(view)).catch(() => BUILTIN_LAYOUTS[view] ?? null);
  } catch {
    return Promise.resolve(BUILTIN_LAYOUTS[view] ?? null);
  }
}

/** One view's layout in the picked theme. It starts from the generated
 *  built-in arrangement, so the first paint already shows the default
 *  instead of flashing empty, and refetches when the picked theme changes. */
export function useThemeLayout(view: string): ThemeLayout | null {
  const [layout, setLayout] = useState<ThemeLayout | null>(() => BUILTIN_LAYOUTS[view] ?? null);
  useEffect(() => {
    let live = true;
    const fetch = () => { void loadThemeLayout(view).then((l) => { if (live && l) setLayout(l); }); };
    fetch();
    return onThemeApplied(fetch);
  }, [view]);
  return layout;
}
