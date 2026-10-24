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
const TOKEN_VARS: Record<string, string[]> = {
  ground: ['--color-ground', '--background'],
  raised: ['--color-raised', '--card', '--popover'],
  surface: ['--color-surface', '--secondary', '--muted'],
  'surface-strong': ['--color-surface-strong'],
  hairline: ['--color-hairline', '--border', '--input'],
  text: ['--color-text', '--card-foreground', '--popover-foreground'],
  'text-body': ['--color-text-body'],
  'text-muted': ['--color-text-muted', '--muted-foreground'],
  'text-faint': ['--color-text-faint'],
  accent: ['--color-accent', '--primary'],
  'accent-strong': ['--color-accent-strong'],
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

/** Every custom property the theme sets, name to value. Pure: no DOM. */
export function themeVariables(theme: ThemeTokens): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const [token, names] of Object.entries(TOKEN_VARS)) {
    const value = theme.tokens[token];
    if (value) for (const name of names) vars[name] = value;
  }
  for (const [step, dl, kc, dh] of AMBER_RAMP) {
    vars[`--color-amber-${step}`] = `oklch(from var(--color-accent) ${shifted('l', dl)} calc(c * ${kc}) ${shifted('h', dh)})`;
  }
  vars['--app-wash'] = washImage(theme.wash ?? []);
  return vars;
}

/** Sets the theme on :root. Live: calling it again restyles what is on screen. */
export function applyTheme(theme: ThemeTokens): void {
  const style = document.documentElement.style;
  for (const [name, value] of Object.entries(themeVariables(theme))) style.setProperty(name, value);
  loadThemeFonts(theme.fonts);
  // The core sends every axis resolved; a cached theme from an older build
  // may lack new axes, which keep their built-in value.
  currentVariants = { ...VARIANT_DEFAULTS, ...(theme.variants ?? {}) } as VariantValues;
  for (const listener of variantListeners) listener();
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
  try {
    return Effect.runPromise(LauncherApi.getTheme(''))
      .then(applyAndRemember)
      .catch(() => {});
  } catch {
    return Promise.resolve();
  }
}

/** Reads one view's validated layout from the core, or null when the theme
 *  ships none: the view then uses its built-in arrangement. A failed read
 *  keeps the built-in one, like a failed theme read keeps the applied theme. */
export function loadThemeLayout(view: string): Promise<ThemeLayout | null> {
  try {
    return Effect.runPromise(LauncherApi.getThemeLayout(view)).catch(() => null);
  } catch {
    return Promise.resolve(null);
  }
}

/** One view's layout in the picked theme, and null for the built-in
 *  arrangement. Refetches when the picked theme changes. */
export function useThemeLayout(view: string): ThemeLayout | null {
  const [layout, setLayout] = useState<ThemeLayout | null>(null);
  useEffect(() => {
    let live = true;
    const fetch = () => { void loadThemeLayout(view).then((l) => { if (live) setLayout(l); }); };
    fetch();
    return onThemeApplied(fetch);
  }, [view]);
  return layout;
}
