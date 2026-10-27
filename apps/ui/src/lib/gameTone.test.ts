import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({ getLauncherSettings: vi.fn(), setGameTone: vi.fn() }));
vi.mock('../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));

import {
  applyGameTone,
  contrastRatio,
  effectiveTone,
  hslOf,
  loadArtTone,
  readable,
  sampleTone,
  toneAt,
  toneVariables,
  vivid,
  type Tone,
} from './gameTone';
import { applyTheme } from './theme';
import type { ThemeTokens } from '../types';

const root = () => document.documentElement;
const css = (name: string) => root().style.getPropertyValue(name);
/** The channels out of a `rgb(r g b)` value this module wrote. */
const rgb = (value: string): Tone => (value.match(/\d+/g) ?? []).map(Number) as Tone;
const pixels = (colors: Tone[]): Uint8ClampedArray => new Uint8ClampedArray(colors.flatMap((c) => [...c, 255]));

beforeEach(() => {
  api.getLauncherSettings.mockReset();
  api.setGameTone.mockReset();
  api.getLauncherSettings.mockReturnValue(Effect.succeed({ game_tone: '' }));
  api.setGameTone.mockReturnValue(Effect.succeed({ game_tone: 'whole' }));
  root().style.cssText = '';
  root().className = '';
});

afterEach(() => {
  vi.unstubAllGlobals();
  root().style.cssText = '';
  root().className = '';
});

describe('sampling a tone', () => {
  it('averages the saturated pixels, so a mostly dark picture still names its colour', () => {
    expect(sampleTone(pixels([[255, 0, 0], [255, 0, 0]]))).toEqual([255, 0, 0]);
    const dark = pixels([[2, 2, 2], [2, 2, 2], [2, 2, 2], [230, 40, 60]]);
    const tone = sampleTone(dark) as Tone;
    expect(tone[0]).toBeGreaterThan(tone[1]);
    expect(tone[0]).toBeGreaterThan(100);
  });

  it('names no colour for a grey picture, or for no pixels', () => {
    expect(sampleTone(pixels([[128, 128, 128], [20, 20, 20]]))).toBeNull();
    expect(sampleTone(pixels([[255, 255, 255]]))).toBeNull();
    expect(sampleTone(new Uint8ClampedArray(0))).toBeNull();
  });
});

describe('vivid and toneAt (ported from the reference design)', () => {
  it('pulls a muted sample up to a usable saturation at the chosen lightness', () => {
    const [, s, l] = hslOf(vivid([128, 120, 116]));
    expect(l).toBeCloseTo(0.62, 1);
    expect(s).toBeGreaterThanOrEqual(0.54);
  });

  it('keeps the sampled hue and puts it at the chosen saturation and lightness', () => {
    // A red-leaning sample: hue 0.
    const tone: Tone = [200, 60, 60];
    expect(hslOf(tone)[0]).toBeCloseTo(0, 0);
    const [, s, l] = hslOf(toneAt(tone, 0.42, 0.2));
    expect(l).toBeCloseTo(0.2, 1);
    expect(s).toBeGreaterThan(0.2);
  });

  it('reads the hue of any colour, and of grey', () => {
    expect(hslOf([0, 128, 0])[0]).toBeCloseTo(120, 0);
    expect(hslOf([128, 128, 128])).toEqual([0, 0, 128 / 255]);
  });
});

describe('the mode a theme allows', () => {
  it('caps the stored pick at what the theme supports', () => {
    expect(effectiveTone('off', 'whole')).toBe('off');
    expect(effectiveTone('off', 'accent')).toBe('off');
    expect(effectiveTone('accent', 'whole')).toBe('accent');
    expect(effectiveTone('accent', 'accent')).toBe('accent');
    expect(effectiveTone('whole', 'accent')).toBe('accent');
    expect(effectiveTone('whole', 'whole')).toBe('whole');
  });

  it('leaves no colour when the user picks off, whatever the theme allows', () => {
    expect(effectiveTone('whole', 'off')).toBe('off');
    expect(effectiveTone('accent', 'off')).toBe('off');
    expect(effectiveTone('off', 'off')).toBe('off');
  });
});

describe('tone variables', () => {
  const tone: Tone = [60, 120, 200];

  it('moves nothing when the axis is off or the art named no colour', () => {
    expect(toneVariables('off', tone)).toEqual({});
    expect(toneVariables('accent', null)).toEqual({});
    expect(toneVariables('whole', null)).toEqual({});
  });

  it('takes only the accent pair in accent mode', () => {
    const vars = toneVariables('accent', tone);
    expect(vars['--color-accent']).toBe(vars['--primary']);
    expect(vars['--color-accent-strong']).toBeTruthy();
    expect(vars['--color-on-accent']).toBe(vars['--primary-foreground']);
    // Surfaces, lines and text keep the theme's values.
    expect(vars['--color-ground']).toBeUndefined();
    expect(vars['--border']).toBeUndefined();
    expect(vars['--color-text']).toBeUndefined();
  });

  it('takes the surfaces, lines and text in whole mode', () => {
    const vars = toneVariables('whole', tone);
    for (const name of ['--color-ground', '--background', '--card', '--popover', '--color-raised', '--color-surface',
      '--color-surface-strong', '--color-hairline', '--border', '--input', '--color-text', '--color-text-body',
      '--color-text-muted', '--color-text-faint', '--color-accent', '--color-accent-strong', '--color-on-accent']) {
      expect(vars[name], name).toMatch(/^rgb\(\d+ \d+ \d+\)$/);
    }
    expect(vars['--color-ground']).toBe(vars['--background']);
    expect(vars['--color-text']).toBe(vars['--card-foreground']);
  });

  it('paints the new tokens in whole mode so the light scheme has no gaps', () => {
    const vars = toneVariables('whole', tone);
    for (const name of ['--color-ground-2', '--color-hairline-strong', '--color-popover']) {
      expect(vars[name], name).toMatch(/^rgb\(\d+ \d+ \d+\)$/);
    }
  });
});

describe('schemes x tints (issue #134: 3 schemes, 3 tints)', () => {
  const tone: Tone = [60, 120, 200];

  it('off sets nothing in any scheme: the scheme tokens stand', () => {
    for (const scheme of ['dark', 'light', 'black'] as const) {
      expect(toneVariables('off', tone, scheme)).toEqual({});
      expect(toneVariables('off', null, scheme)).toEqual({});
    }
  });

  it('accent sets the accent pair per scheme and leaves the surfaces alone', () => {
    for (const scheme of ['dark', 'light', 'black'] as const) {
      const vars = toneVariables('accent', tone, scheme);
      expect(vars['--color-accent'], `${scheme} accent`).toMatch(/^rgb\(\d+ \d+ \d+\)$/);
      expect(vars['--color-accent-dim'], `${scheme} accent-dim`).toMatch(/^rgb\(\d+ \d+ \d+\)$/);
      expect(vars['--color-ground'], `${scheme} ground`).toBeUndefined();
      expect(vars['--color-text'], `${scheme} text`).toBeUndefined();
    }
    // Light takes the tone at (0.75, 0.38); dark takes vivid. Black dims the dim shade.
    const darkDim = rgb(toneVariables('accent', tone, 'dark')['--color-accent-dim']);
    const blackDim = rgb(toneVariables('accent', tone, 'black')['--color-accent-dim']);
    const lightDim = rgb(toneVariables('accent', tone, 'light')['--color-accent-dim']);
    expect(contrastRatio(blackDim, [0, 0, 0])).toBeLessThan(contrastRatio(darkDim, [0, 0, 0]));
    expect(contrastRatio(lightDim, [0, 0, 0])).toBeGreaterThan(contrastRatio(darkDim, [0, 0, 0]));
  });

  it('whole sets every surface and text token per scheme', () => {
    const expected = ['--color-ground', '--color-ground-2', '--color-raised', '--color-surface',
      '--color-surface-strong', '--color-hairline', '--color-hairline-strong', '--popover',
      '--color-text', '--color-text-body', '--color-text-muted', '--color-text-faint',
      '--color-accent', '--color-accent-dim'];
    for (const scheme of ['dark', 'light', 'black'] as const) {
      const vars = toneVariables('whole', tone, scheme);
      for (const name of expected) expect(vars[name], `${scheme} ${name}`).toMatch(/^rgb\(\d+ \d+ \d+\)$/);
    }
    // Light grounds are light, dark grounds are dark, black grounds are darker still.
    const lightGround = rgb(toneVariables('whole', tone, 'light')['--color-ground']);
    const darkGround = rgb(toneVariables('whole', tone, 'dark')['--color-ground']);
    const blackGround = rgb(toneVariables('whole', tone, 'black')['--color-ground']);
    const lum = (t: Tone): number => (0.2126 * t[0] + 0.7152 * t[1] + 0.0722 * t[2]) / 255;
    expect(lum(lightGround)).toBeGreaterThan(0.6);
    expect(lum(darkGround)).toBeLessThan(0.2);
    expect(lum(blackGround)).toBeLessThanOrEqual(lum(darkGround));
  });

  it('keeps whole-mode text readable in every scheme', () => {
    for (const scheme of ['dark', 'light', 'black'] as const) {
      for (const sample of [[60, 120, 200], [250, 240, 246], [14, 10, 12]] as Tone[]) {
        const vars = toneVariables('whole', sample, scheme);
        const ground = rgb(vars['--color-ground']);
        expect(contrastRatio(rgb(vars['--color-text']), ground), `${scheme} text`).toBeGreaterThanOrEqual(4.5);
        expect(contrastRatio(rgb(vars['--color-text-muted']), ground), `${scheme} muted`).toBeGreaterThanOrEqual(3);
      }
    }
  });
});

describe('derived text stays readable', () => {
  // The two ends of the scale: a nearly white picture and a nearly black one.
  const light: Tone = [250, 240, 246];
  const dark: Tone = [14, 10, 12];

  it.each([['a very light sample', light], ['a very dark sample', dark]] as const)('keeps text, muted text and button text above their ratios for %s', (_label, tone) => {
    const whole = toneVariables('whole', tone);
    const ground = rgb(whole['--color-ground']);
    expect(contrastRatio(rgb(whole['--color-text']), ground)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(rgb(whole['--color-text-body']), ground)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(rgb(whole['--color-text-muted']), ground)).toBeGreaterThanOrEqual(3);
    expect(contrastRatio(rgb(whole['--color-text-faint']), ground)).toBeGreaterThanOrEqual(3);
    expect(contrastRatio(rgb(whole['--color-on-accent']), rgb(whole['--color-accent']))).toBeGreaterThanOrEqual(4.5);

    const accent = toneVariables('accent', tone);
    expect(contrastRatio(rgb(accent['--color-on-accent']), rgb(accent['--color-accent']))).toBeGreaterThanOrEqual(4.5);
  });

  it('darkens a light text on a light background until the pair clears the ratio', () => {
    // Asking for near-white text on white mixes: the clamp moves it to black.
    const white: Tone = [255, 255, 255];
    const out = readable([210, 60, 60], 0.3, 0.95, white, 4.5, -1);
    expect(contrastRatio(out, white)).toBeGreaterThanOrEqual(4.5);
  });

  it('measures contrast like WCAG does: 21:1 for black on white, 1:1 for a colour on itself', () => {
    expect(contrastRatio([0, 0, 0], [255, 255, 255])).toBeCloseTo(21, 1);
    expect(contrastRatio([90, 40, 200], [90, 40, 200])).toBeCloseTo(1, 5);
  });
});

describe('painting the tone', () => {
  const tone: Tone = [60, 120, 200];

  it('sets the variables on the root and hands them back when the tone goes', () => {
    applyGameTone('whole', tone);
    expect(css('--color-ground')).toMatch(/^rgb\(/);
    expect(css('--color-accent')).toMatch(/^rgb\(/);
    // The 0.6s fade is on while it paints, and off again afterwards.
    expect(root().classList.contains('game-tone')).toBe(true);

    applyGameTone('off', null);
    expect(css('--color-ground')).toBe('');
    expect(css('--color-accent')).toBe('');
    expect(css('--color-text')).toBe('');
  });

  it('drops the wider palette when switching from whole back to accent', () => {
    applyGameTone('whole', tone);
    applyGameTone('accent', tone);
    expect(css('--color-accent')).toMatch(/^rgb\(/);
    expect(css('--color-ground')).toBe('');
    expect(css('--color-text')).toBe('');
  });
});

describe('sampling the hero art', () => {
  /** A bare Image whose src load succeeds at once. */
  class FakeImage {
    static last: FakeImage | null = null;
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    crossOrigin = '';
    set src(_url: string) {
      FakeImage.last = this;
      queueMicrotask(() => this.onload?.());
    }
  }

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('asks for CORS and averages the drawn pixels', async () => {
    vi.stubGlobal('Image', FakeImage);
    const data = pixels([[200, 40, 40], [200, 40, 40]]);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      drawImage: vi.fn(),
      getImageData: () => ({ data }),
    } as unknown as CanvasRenderingContext2D);

    await expect(loadArtTone('/platypus-art/echo/abc')).resolves.toEqual([200, 40, 40]);
    expect(FakeImage.last?.crossOrigin).toBe('anonymous');
  });

  it('keeps null when the canvas is tainted, the picture fails, or there is no canvas', async () => {
    vi.stubGlobal('Image', FakeImage);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
      drawImage: vi.fn(),
      getImageData: () => { throw new DOMException('tainted', 'SecurityError'); },
    } as unknown as CanvasRenderingContext2D);
    await expect(loadArtTone('/platypus-art/echo/abc')).resolves.toBeNull();

    class FailingImage extends FakeImage {
      override set src(_url: string) {
        queueMicrotask(() => this.onerror?.());
      }
    }
    vi.stubGlobal('Image', FailingImage);
    await expect(loadArtTone('/platypus-art/echo/abc')).resolves.toBeNull();

    vi.stubGlobal('Image', FakeImage);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null);
    await expect(loadArtTone('/platypus-art/echo/abc')).resolves.toBeNull();
  });
});

describe('the theme\'s own colours come back', () => {
  // Last in the file: applying a theme is module state the earlier tests rely
  // on not having (they expect the variables to end up empty).
  const themed = (id: string, tokens: Record<string, string> = {}): ThemeTokens => ({
    id,
    tokens: {
      ground: '#0d0b09', raised: '#161310', hairline: '#2a2521',
      text: '#f4efe9', accent: '#ffb648', 'on-accent': '#0d0b09',
      ...tokens,
    },
    wash: [],
    variants: {},
  });

  it('sets the theme values back when the tone clears, instead of removing them', () => {
    const atlas = themed('atlas', { ground: '#070a10', accent: '#7dd3fc' });
    applyTheme(atlas);
    applyGameTone('whole', [60, 120, 200]);
    // The tone is on, so the surfaces carry the game's colour.
    expect(css('--color-ground')).toMatch(/^rgb\(/);

    // Back to the Dashboard: the tone clears, and the theme stays what it was.
    applyGameTone('off', null);
    expect(css('--color-ground')).toBe('#070a10');
    expect(css('--background')).toBe('#070a10');
    expect(css('--color-accent')).toBe('#7dd3fc');
    expect(css('--color-text')).toBe('#f4efe9');
  });

  it('sets the new theme\'s values across a theme switch, not the old ones', () => {
    applyTheme(themed('atlas', { ground: '#070a10', accent: '#7dd3fc' }));
    applyGameTone('whole', [60, 120, 200]);

    // Atlas -> cool, which sets no tone: the theme paints, then the tone clears.
    applyTheme(themed('cool', { ground: '#0b1220', accent: '#38bdf8' }));
    applyGameTone('off', null);
    expect(css('--color-ground')).toBe('#0b1220');
    expect(css('--color-accent')).toBe('#38bdf8');
    expect(css('--color-text')).toBe('#f4efe9');
  });
});
