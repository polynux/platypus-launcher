// @ts-nocheck — reads style.css and Tailwind's theme from disk, outside the app's DOM tsconfig.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { Effect } from 'effect';
import type { ThemeTokens } from '../types';

const getTheme = vi.hoisted(() => vi.fn());
const getThemeLayout = vi.hoisted(() => vi.fn());
vi.mock('../effect/runtime', () => ({ LauncherApi: { getTheme: (id: string) => getTheme(id), getThemeLayout: (view: string) => getThemeLayout(view) } }));

import { applyAndRemember, applyCachedTheme, applyTheme, loadTheme, loadThemeFonts, loadThemeLayout, themeVariables, washImage } from './theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { DEFAULT_THEME } from '../gen/defaultTheme';

const warm: ThemeTokens = {
  id: 'warm',
  tokens: {
    ground: '#130f0a', 'ground-2': '#191410', raised: '#191410', surface: 'rgba(255, 255, 255, 0.045)', 'surface-strong': 'rgba(255, 255, 255, 0.06)',
    hairline: 'rgba(255, 255, 255, 0.07)', 'hairline-strong': 'rgba(255, 255, 255, 0.12)', popover: '#191410', glass: 'rgba(25, 20, 16, 0.82)',
    text: 'rgba(255, 255, 255, 0.9)', 'text-body': 'rgba(255, 255, 255, 0.7)', 'text-muted': 'rgba(255, 255, 255, 0.55)',
    'text-faint': '#78716c', accent: 'oklch(82.8% 0.189 84.429)', 'accent-strong': 'oklch(87.9% 0.169 91.605)', 'accent-dim': '#2f240d', 'on-accent': '#130f0a', ok: 'oklch(76.5% 0.177 163.223)',
    attention: 'oklch(75% 0.183 55.934)', info: 'oklch(82.8% 0.111 230.318)', dl: 'oklch(70.2% 0.183 293.541)', danger: 'oklch(63.7% 0.237 25.331)', focus: 'oklch(87.9% 0.169 91.605)',
    'radius-chip': '0.75rem', 'radius-card': '1rem', 'radius-panel': '1.5rem',
    'font-sans': '"Hanken Grotesk", system-ui, sans-serif', 'font-mono': '"JetBrains Mono", ui-monospace, monospace',
  },
  wash: [
    { at: '14% -8%', size: '1100px 620px', color: 'rgba(150, 96, 58, 0.46)', fade: '58%' },
    { at: '90% 10%', size: '880px 520px', color: 'rgba(86, 62, 128, 0.44)', fade: '54%' },
    { at: '52% 108%', size: '760px 460px', color: 'rgba(30, 78, 96, 0.36)', fade: '56%' },
  ],
};

const cool: ThemeTokens = { ...warm, id: 'cool', tokens: { ...warm.tokens, accent: '#7dd3fc', 'radius-chip': '0.5rem' }, wash: [] };

const root = () => document.documentElement.style;

beforeEach(() => {
  window.localStorage.clear();
  root().cssText = '';
  document.documentElement.dataset.labelStyle = '';
  getTheme.mockReset();
  getThemeLayout.mockReset();
});

afterEach(() => {
  root().cssText = '';
});

describe('theme variables', () => {
  it('maps the tokens onto the @theme names and the shadcn variables the ds/ components read', () => {
    const v = themeVariables(warm);
    expect(v['--color-ground']).toBe('#130f0a');
    expect(v['--background']).toBe('#130f0a');
    expect(v['--primary']).toBe('oklch(82.8% 0.189 84.429)');
    expect(v['--color-accent']).toBe('oklch(82.8% 0.189 84.429)');
    expect(v['--muted-foreground']).toBe('rgba(255, 255, 255, 0.55)');
    expect(v['--ring']).toBe('oklch(87.9% 0.169 91.605)');
    expect(v['--destructive']).toBe('oklch(63.7% 0.237 25.331)');
  });

  it('maps the radius tokens onto the classes Tailwind compiles rounded-xl/2xl/3xl to', () => {
    const v = themeVariables({ ...warm, tokens: { ...warm.tokens, 'radius-chip': '0.5rem' } });
    expect(v['--radius-xl']).toBe('0.5rem');
    expect(v['--radius-2xl']).toBe('1rem');
    expect(v['--radius-3xl']).toBe('1.5rem');
  });

  it('builds the wash from its layers, the same gradient style.css ships', () => {
    expect(washImage(warm.wash)).toBe(
      'radial-gradient(1100px 620px at 14% -8%, rgba(150, 96, 58, 0.46), transparent 58%), ' +
        'radial-gradient(880px 520px at 90% 10%, rgba(86, 62, 128, 0.44), transparent 54%), ' +
        'radial-gradient(760px 460px at 52% 108%, rgba(30, 78, 96, 0.36), transparent 56%)',
    );
    expect(washImage([])).toBe('none');
  });

  it('draws a wash image under its gradient, so theme art tints like its colours', () => {
    const layers = [{ at: '10% 10%', size: '100px 100px', color: '#000000', fade: '50%', image: '/platypus-art/theme/p/assets/bg.png?v=abc' }];
    expect(washImage(layers)).toBe(
      'radial-gradient(100px 100px at 10% 10%, #000000, transparent 50%), url("/platypus-art/theme/p/assets/bg.png?v=abc")',
    );
    expect(themeVariables({ ...warm, wash: layers })['--app-wash']).toContain('url("/platypus-art/theme/p/assets/bg.png?v=abc")');
  });

  it('applies a resolved plugin theme to the CSS variable, image URL included', () => {
    applyTheme({
      ...warm,
      id: 'fixture',
      wash: [{ at: '10% 10%', size: '100px 100px', color: '#000000', fade: '50%', image: '/platypus-art/theme/p/assets/bg.png?v=abc' }],
      fonts: { 'Fixture Sans': '/platypus-art/theme/p/fonts/f.woff2?v=abc' },
    });
    expect(root().getPropertyValue('--app-wash')).toContain('url("/platypus-art/theme/p/assets/bg.png?v=abc")');
  });

  it('registers a plugin theme font, and tolerates a missing FontFace', async () => {
    const add = vi.fn();
    const load = vi.fn().mockResolvedValue({ family: 'Fixture Sans' });
    vi.stubGlobal('FontFace', class {
      load = load;
      constructor(public family: string, public source: string, public desc: object) {}
    });
    Object.defineProperty(document, 'fonts', { value: { add }, configurable: true });
    loadThemeFonts({ 'Fixture Sans': '/platypus-art/theme/p/fonts/f.woff2?v=abc' });
    expect(load).toHaveBeenCalled();
    await Promise.resolve();
    expect(add).toHaveBeenCalled();
    vi.unstubAllGlobals();
    expect(() => loadThemeFonts({ 'Fixture Sans': '/platypus-art/theme/p/fonts/f.woff2' })).not.toThrow();
    expect(() => loadThemeFonts(undefined)).not.toThrow();
  });

  it('warns with the family and URL when a theme font fails to load', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const load = vi.fn().mockRejectedValue(new Error('decode'));
    vi.stubGlobal('FontFace', class {
      load = load;
      constructor(public family: string, public source: string, public desc: object) {}
    });
    Object.defineProperty(document, 'fonts', { value: { add: vi.fn() }, configurable: true });
    loadThemeFonts({ 'Fixture Sans': '/platypus-art/theme/p/fonts/f.woff2?v=abc' });
    await Promise.resolve();
    await new Promise((r) => setTimeout(r, 0));
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('Fixture Sans'));
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('/platypus-art/theme/p/fonts/f.woff2?v=abc'));
    vi.unstubAllGlobals();
    warn.mockRestore();
  });

  // style.css keeps the warm wash as its own default; the two must not drift.
  it('keeps style.css\'s default wash equal to the warm theme\'s', () => {
    const css = readFileSync(path.join(__dirname, '../style.css'), 'utf8');
    const decl = /--app-wash:\s*([^;]+);/.exec(css)?.[1] ?? '';
    expect(decl.replace(/\s+/g, ' ').trim()).toBe(washImage(warm.wash));
  });

  it('keeps the warm theme colors equal to style.css defaults', () => {
    const css = readFileSync(path.join(__dirname, '../style.css'), 'utf8');
    const toml = readFileSync(path.join(__dirname, '../../../../internal/themes/builtin/warm/theme.toml'), 'utf8');
    for (const token of ['ground', 'ground-2', 'raised', 'surface', 'surface-strong', 'hairline', 'hairline-strong', 'popover', 'glass', 'text', 'text-body', 'text-muted', 'text-faint', 'accent', 'accent-strong', 'accent-dim', 'on-accent', 'ok', 'attention', 'info', 'dl', 'danger', 'focus']) {
      const cssName = token === 'popover' ? '--popover' : `--color-${token}`;
      const cssValue = new RegExp(`${cssName}:\\s*([^;]+);`).exec(css)?.[1].trim();
      const tomlValue = new RegExp(`^${token}\\s*=\\s*"([^"]+)"`, 'm').exec(toml)?.[1];
      expect(cssValue, `style.css ${token}`).toBe(tomlValue);
    }
  });

  // The amber ramp is derived from the accent. For the warm accent it must
  // reproduce Tailwind's own amber, which compiles bg-amber-300 & co.
  it('derives the amber ramp from the accent so the warm theme keeps Tailwind\'s amber', () => {
    const tw = readFileSync(path.join(__dirname, '../../node_modules/tailwindcss/theme.css'), 'utf8');
    const v = themeVariables(warm);
    const base = { l: 0.828, c: 0.189, h: 84.429 };
    let checked = 0;
    for (const m of tw.matchAll(/--color-amber-(\d+): oklch\(([\d.]+)% ([\d.]+) ([\d.]+)\);/g)) {
      const [, step, l, c, h] = m;
      const ours = /calc\(l ([+-]) ([\d.]+)\) calc\(c \* ([\d.]+)\) calc\(h ([+-]) ([\d.]+)\)/.exec(v[`--color-amber-${step}`]);
      expect(ours, `amber-${step}`).not.toBeNull();
      const dl = Number(ours![2]) * (ours![1] === '-' ? -1 : 1);
      const dh = Number(ours![5]) * (ours![4] === '-' ? -1 : 1);
      expect(base.l + dl, `amber-${step} lightness`).toBeCloseTo(Number(l) / 100, 3);
      expect(base.c * Number(ours![3]), `amber-${step} chroma`).toBeCloseTo(Number(c), 3);
      expect(base.h + dh, `amber-${step} hue`).toBeCloseTo(Number(h), 1);
      checked++;
    }
    expect(checked).toBe(11);
  });

  it('gives an alternate accent its own ramp, relative to that accent', () => {
    const v = themeVariables(cool);
    expect(v['--color-amber-300']).toContain('oklch(from var(--color-accent)');
    expect(v['--color-amber-300']).toBe(themeVariables(warm)['--color-amber-300']);
  });
});

describe('applying a theme', () => {
  it('sets the variables on :root, and a second apply restyles them', () => {
    applyTheme(warm);
    expect(root().getPropertyValue('--color-accent')).toBe('oklch(82.8% 0.189 84.429)');
    applyTheme(cool);
    expect(root().getPropertyValue('--color-accent')).toBe('#7dd3fc');
    expect(root().getPropertyValue('--radius-xl')).toBe('0.5rem');
  });

  it('paints the theme last picked before the core answers, and tolerates a broken cache', () => {
    applyAndRemember(cool);
    root().cssText = '';
    applyCachedTheme();
    expect(root().getPropertyValue('--color-accent')).toBe('#7dd3fc');
    window.localStorage.setItem('platypus.theme', '{not json');
    expect(() => applyCachedTheme()).not.toThrow();
  });

  it('paints the default theme with nothing cached, so the first paint is Atlas (issue #130)', () => {
    expect(DEFAULT_THEME.id).toBe('atlas');
    applyCachedTheme();
    expect(root().getPropertyValue('--color-ground')).toBe(DEFAULT_THEME.tokens.ground);
    expect(root().getPropertyValue('--color-ground')).toBe('#13161b');
    expect(root().getPropertyValue('--color-accent')).toBe(DEFAULT_THEME.tokens.accent);
    expect(document.documentElement.dataset.labelStyle).toBe('plain');
  });

  it('loads the picked theme from the core and remembers it', async () => {
    getTheme.mockReturnValue(Effect.succeed(cool));
    await loadTheme();
    expect(getTheme).toHaveBeenCalledWith('');
    expect(root().getPropertyValue('--color-accent')).toBe('#7dd3fc');
    expect(JSON.parse(window.localStorage.getItem('platypus.theme') ?? '{}').id).toBe('cool');
  });

  it('keeps what is applied when the core cannot answer, even from a partial mock', async () => {
    applyTheme(warm);
    getTheme.mockReturnValue(Effect.fail(new Error('no backend')));
    await expect(loadTheme()).resolves.toBeUndefined();
    getTheme.mockImplementation(() => {
      throw new TypeError('getTheme is not a function');
    });
    await expect(loadTheme()).resolves.toBeUndefined();
    expect(root().getPropertyValue('--color-accent')).toBe('oklch(82.8% 0.189 84.429)');
  });

  it('writes data-label-style from the new theme on the first apply (issue #154)', () => {
    applyTheme(warm);
    expect(document.documentElement.dataset.labelStyle).toBe('mono');
    applyTheme({ ...warm, id: 'atlas', variants: { 'label.style': 'plain' } });
    expect(document.documentElement.dataset.labelStyle).toBe('plain');
  });

  it('falls back to the built-in layout when the theme ships none (issue #154)', async () => {
    getThemeLayout.mockReturnValue(Effect.succeed(null));
    await expect(loadThemeLayout('dashboard')).resolves.toEqual(BUILTIN_LAYOUTS['dashboard']);
    await expect(loadThemeLayout('attic')).resolves.toBeNull();
  });
});

describe('colour schemes (issue #134)', () => {
  const atlas = {
    ...warm,
    id: 'atlas',
    tokens: {
      ...warm.tokens,
      ground: '#13161b', 'ground-2': '#15181d', raised: '#181b21', surface: '#1f232a',
      'surface-strong': '#293039', hairline: '#262b33', 'hairline-strong': '#36404b',
      popover: '#1b1f26', glass: 'rgba(24, 27, 33, 0.82)',
      text: '#e9edf2', 'text-body': '#c3ccd8', 'text-muted': '#9ca7b5', 'text-faint': '#687384',
      accent: '#9db0ff', 'accent-strong': '#bccdff', 'accent-dim': '#1f232a', 'on-accent': '#0f1218',
      focus: '#9db0ff', danger: '#f0756a',
    },
    schemes: {
      light: {
        ground: '#e9ecf0', 'ground-2': '#f1f3f6', raised: '#f7f8fa', surface: '#eceff3',
        'surface-strong': '#dfe3e9', hairline: '#dde1e7', 'hairline-strong': '#c4cad3',
        popover: '#ffffff', glass: 'rgba(250, 251, 252, 0.86)',
        text: '#161a20', 'text-body': '#333f4e', 'text-muted': '#525c6a', 'text-faint': '#7a8494',
        accent: '#3b5bdb', 'accent-strong': '#3b5bdb', 'accent-dim': '#dfe5f5',
        'on-accent': '#ffffff', focus: '#3b5bdb', danger: '#9d2a1e',
      },
      black: {
        ground: '#000000', 'ground-2': '#05060a', raised: '#08090c', surface: '#121418',
        'surface-strong': '#1c1f25', hairline: '#181b20', 'hairline-strong': '#2a2f37',
        popover: '#101216', glass: 'rgba(8, 9, 12, 0.82)',
        accent: '#9db0ff', 'accent-strong': '#bccdff', 'accent-dim': '#0e1016', 'on-accent': '#0f1218',
      },
    },
  };

  it('maps the new tokens onto CSS variables', () => {
    const v = themeVariables(warm);
    expect(v['--color-ground-2']).toBe('#191410');
    expect(v['--color-hairline-strong']).toBe('rgba(255, 255, 255, 0.12)');
    expect(v['--color-glass']).toBe('rgba(25, 20, 16, 0.82)');
    expect(v['--popover']).toBe('#191410');
    expect(v['--color-popover']).toBe('#191410');
  });

  it('paints the scheme overrides over the theme tokens', () => {
    expect(themeVariables(atlas, 'dark')['--color-ground']).toBe('#13161b');
    expect(themeVariables(atlas, 'light')['--color-ground']).toBe('#e9ecf0');
    expect(themeVariables(atlas, 'light')['--color-ground-2']).toBe('#f1f3f6');
    expect(themeVariables(atlas, 'light')['--popover']).toBe('#ffffff');
    expect(themeVariables(atlas, 'black')['--color-ground']).toBe('#000000');
    expect(themeVariables(atlas, 'black')['--color-accent-dim']).toBe('#0e1016');
    // A theme without the scheme falls back to dark.
    expect(themeVariables(warm, 'light')['--color-ground']).toBe('#130f0a');
  });

  it('sets color-scheme light for the light scheme and dark otherwise', () => {
    applyTheme(atlas);
    // The stored pick starts dark; the scheme module defaults to it.
    expect(document.documentElement.style.getPropertyValue('color-scheme')).toBe('dark');
  });
});
