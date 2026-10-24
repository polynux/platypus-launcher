// @ts-nocheck — reads style.css and Tailwind's theme from disk, outside the app's DOM tsconfig.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { Effect } from 'effect';
import type { ThemeTokens } from '../types';

const getTheme = vi.hoisted(() => vi.fn());
vi.mock('../effect/runtime', () => ({ LauncherApi: { getTheme: (id: string) => getTheme(id) } }));

import { applyAndRemember, applyCachedTheme, applyTheme, loadTheme, themeVariables, washImage } from './theme';

const warm: ThemeTokens = {
  id: 'warm',
  tokens: {
    ground: '#130f0a', raised: '#191410', surface: 'rgba(255, 255, 255, 0.045)', 'surface-strong': 'rgba(255, 255, 255, 0.06)',
    hairline: 'rgba(255, 255, 255, 0.07)', text: 'rgba(255, 255, 255, 0.9)', 'text-body': 'rgba(255, 255, 255, 0.7)', 'text-muted': 'rgba(255, 255, 255, 0.55)',
    'text-faint': '#78716c', accent: 'oklch(82.8% 0.189 84.429)', 'accent-strong': 'oklch(87.9% 0.169 91.605)', 'on-accent': '#130f0a', ok: 'oklch(76.5% 0.177 163.223)',
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
  getTheme.mockReset();
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

  // style.css keeps the warm wash as its own default; the two must not drift.
  it('keeps style.css\'s default wash equal to the warm theme\'s', () => {
    const css = readFileSync(path.join(__dirname, '../style.css'), 'utf8');
    const decl = /--app-wash:\s*([^;]+);/.exec(css)?.[1] ?? '';
    expect(decl.replace(/\s+/g, ' ').trim()).toBe(washImage(warm.wash));
  });

  it('keeps the warm theme colors equal to style.css defaults', () => {
    const css = readFileSync(path.join(__dirname, '../style.css'), 'utf8');
    const toml = readFileSync(path.join(__dirname, '../../../../internal/themes/builtin/warm/theme.toml'), 'utf8');
    for (const token of ['ground', 'raised', 'surface', 'surface-strong', 'hairline', 'text', 'text-body', 'text-muted', 'text-faint', 'accent', 'accent-strong', 'on-accent', 'ok', 'attention', 'info', 'dl', 'danger', 'focus']) {
      const cssValue = new RegExp(`--color-${token}:\\s*([^;]+);`).exec(css)?.[1].trim();
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
});
