// Game colour from the art (issue #123): sample a tone from the selected
// game's hero art, then derive token values from it. Ported from the reference
// design (tools/design-lab-2/src/core/launcher.js: useArtTone, vivid, toneAt;
// the vars object in designs/d21/D21.jsx). The lab returns CSS strings; here
// the maths returns channel triples so readability can be checked before
// anything is painted.
import { Effect } from 'effect';
import { useEffect, useState } from 'react';
import { LauncherApi } from '../effect/runtime';
import { TOKEN_VARS, onThemeApplied, useVariant } from './theme';

/** A sampled colour: sRGB channels, 0..255. */
export type Tone = [number, number, number];

/** What the theme's color.game-tone axis allows. */
export type GameToneSupport = 'off' | 'accent' | 'whole';

/** What is on screen: the axis value, capped by the theme. */
export type GameToneMode = GameToneSupport;

/** The user's pick, stored as a launcher setting. */
export type GameToneChoice = 'accent' | 'whole';

/** The width the hero art is sampled at, in pixels (the lab's 48x16). */
const SAMPLE_W = 48;
const SAMPLE_H = 16;

/** A picture with no hue worth naming: 8-bit channels this close are grey, and
 *  a grey tone would pick a hue out of nothing. */
const MIN_CHROMA = 6;

/** The average of a picture's saturated pixels: bright, colourful areas weigh
 *  most, so a dark or mostly plain picture still names its colour. RGBA data as
 *  canvas getImageData returns it; null for a picture with no colour. */
export function sampleTone(pixels: ArrayLike<number>): Tone | null {
  let r = 0;
  let g = 0;
  let b = 0;
  let w = 0;
  for (let i = 0; i + 3 < pixels.length; i += 4) {
    const pr = pixels[i];
    const pg = pixels[i + 1];
    const pb = pixels[i + 2];
    const mx = Math.max(pr, pg, pb);
    const mn = Math.min(pr, pg, pb);
    const sat = mx ? (mx - mn) / mx : 0;
    const weight = sat * sat * (mx / 255) + 0.002;
    r += pr * weight;
    g += pg * weight;
    b += pb * weight;
    w += weight;
  }
  if (w <= 0) return null;
  const tone: Tone = [Math.round(r / w), Math.round(g / w), Math.round(b / w)];
  if (Math.max(...tone) - Math.min(...tone) < MIN_CHROMA) return null;
  return tone;
}

/** A tone's hue (0..360) and saturation and lightness (0..1). */
export function hslOf(tone: Tone): [number, number, number] {
  const [ri, gi, bi] = tone;
  const r = ri / 255;
  const g = gi / 255;
  const b = bi / 255;
  const mx = Math.max(r, g, b);
  const mn = Math.min(r, g, b);
  const l = (mx + mn) / 2;
  const d = mx - mn;
  if (!d) return [0, 0, l];
  const s = d / (1 - Math.abs(mx + mn - 1));
  let h = mx === r ? ((g - b) / d) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
  h *= 60;
  if (h < 0) h += 360;
  return [h, s, l];
}

/** Hue and saturation to channels at a chosen lightness. */
function fromHSL(h: number, s: number, l: number): Tone {
  const sat = Math.min(Math.max(s, 0), 1);
  const light = Math.min(Math.max(l, 0), 1);
  const c = (1 - Math.abs(2 * light - 1)) * sat;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = light - c / 2;
  const [r, g, b] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return [Math.round((r + m) * 255), Math.round((g + m) * 255), Math.round((b + m) * 255)];
}

/** The tone as a usable signal colour: saturated, mid-light (the lab's vivid). */
export function vivid(tone: Tone, l = 0.62, sMin = 0.55): Tone {
  const [h, s] = hslOf(tone);
  return fromHSL(h, Math.max(s, sMin), l);
}

/** The tone at a chosen saturation and lightness (the lab's toneAt). The
 *  sampled saturation floors at 0.6, so a muted picture still reads. */
export function toneAt(tone: Tone, s: number, l: number): Tone {
  const [h, sampled] = hslOf(tone);
  return fromHSL(h, Math.min(Math.max(sampled, 0.6), 1) * s, l);
}

/** WCAG 2.2 relative luminance, 0..1. */
export function luminance(tone: Tone): number {
  const [r, g, b] = tone.map((v) => {
    const c = v / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG 2.2 contrast ratio between two colours, 1..21. */
export function contrastRatio(a: Tone, b: Tone): number {
  const la = luminance(a);
  const lb = luminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** The tone at (s, l), its lightness moved away from `against` until the pair
 *  clears `min` (the text keeps its hue and saturation). dir -1 darkens the
 *  text, 1 lightens it. This is the clamp that keeps derived text readable for
 *  a very light or a very dark sample: lightness never comes from the sample. */
export function readable(tone: Tone, s: number, l: number, against: Tone, min: number, dir: 1 | -1): Tone {
  let best = toneAt(tone, s, l);
  if (contrastRatio(best, against) >= min) return best;
  for (let step = 1; step <= 48; step += 1) {
    const next = l + dir * step * 0.01;
    if (next <= 0.02 || next >= 0.98) break;
    best = toneAt(tone, s, next);
    if (contrastRatio(best, against) >= min) return best;
  }
  return best;
}

/** Channels as a CSS colour (the syntax the app's tokens are set with). */
export function cssColor(tone: Tone): string {
  return `rgb(${tone[0]} ${tone[1]} ${tone[2]})`;
}

/** The CSS variables of a game tone: the accent pair for `accent`, the whole
 *  palette for `whole` (the `whole` branch of D21.jsx, onto this app's token
 *  names). Empty when there is no tone, so the theme's own tokens stand. */
export function toneVariables(mode: GameToneMode, tone: Tone | null): Record<string, string> {
  if (!tone || mode === 'off') return {};
  const derived: Record<string, Tone> = {};
  if (mode === 'accent') {
    const accent = vivid(tone, 0.7, 0.6);
    derived.accent = accent;
    derived['accent-strong'] = vivid(tone, 0.68, 0.76);
    // The dim shade of the accent, worn as the text on it.
    derived['on-accent'] = readable(tone, 0.45, 0.17, accent, 4.5, -1);
  } else {
    // The surfaces first: the text is clamped against the ground it sits on.
    const ground = toneAt(tone, 0.5, 0.06);
    derived.ground = ground;
    derived.raised = toneAt(tone, 0.48, 0.08);
    derived.surface = toneAt(tone, 0.42, 0.13);
    derived['surface-strong'] = toneAt(tone, 0.38, 0.18);
    derived.hairline = toneAt(tone, 0.34, 0.17);
    const accent = toneAt(tone, 0.9, 0.66);
    derived.accent = accent;
    derived['accent-strong'] = toneAt(tone, 0.7, 0.76);
    derived['on-accent'] = readable(tone, 0.5, 0.07, accent, 4.5, -1);
    derived.text = readable(tone, 0.3, 0.95, ground, 4.5, 1);
    derived['text-body'] = readable(tone, 0.25, 0.72, ground, 4.5, 1);
    derived['text-muted'] = readable(tone, 0.25, 0.62, ground, 3, 1);
    derived['text-faint'] = readable(tone, 0.22, 0.52, ground, 3, 1);
  }
  const vars: Record<string, string> = {};
  for (const [token, value] of Object.entries(derived)) {
    const color = cssColor(value);
    for (const name of TOKEN_VARS[token] ?? []) vars[name] = color;
  }
  return vars;
}

/** What the theme's axis allows, capped by the user's pick. */
export function effectiveTone(supported: GameToneSupport, choice: GameToneChoice): GameToneMode {
  if (supported === 'off') return 'off';
  return supported === 'whole' ? choice : 'accent';
}

/** The tone painted on :root last, for a repaint after a theme change. */
let paintedTone: { mode: GameToneMode; tone: Tone | null } = { mode: 'off', tone: null };
let painted: string[] = [];
/** How long the painted colours take to reach the new ones (style.css). */
const FADE_MS = 600;
let fadeTimer: ReturnType<typeof setTimeout> | undefined;

/** Paints the game's tone over the theme's tokens. With no tone (the axis is
 *  off, there is no art, or sampling failed) the variables are removed again
 *  and the theme's own values show. */
export function applyGameTone(mode: GameToneMode, tone: Tone | null): void {
  paintedTone = { mode, tone };
  const vars = toneVariables(mode, tone);
  const names = Object.keys(vars);
  // Nothing to paint and nothing painted: leave the stylesheet alone (no
  // transition class on a theme that never took the game's colour).
  if (!names.length && !painted.length) return;
  const root = document.documentElement;
  const style = root.style;
  // The 0.6s fade is on only while a tone paints, so a theme switch (which
  // rewrites the same variables) stays instant.
  root.classList.add('game-tone');
  if (fadeTimer) clearTimeout(fadeTimer);
  fadeTimer = setTimeout(() => {
    root.classList.remove('game-tone');
    fadeTimer = undefined;
  }, FADE_MS + 50);
  for (const name of painted) if (!(name in vars)) style.removeProperty(name);
  for (const [name, value] of Object.entries(vars)) style.setProperty(name, value);
  painted = names;
}

/** Applies the last tone again: a theme change rewrites its tokens, and the
 *  game's colour goes back on top of them. */
export function repaintGameTone(): void {
  applyGameTone(paintedTone.mode, paintedTone.tone);
}

/** Samples one picture: loaded off the render path, drawn into a small canvas
 *  and averaged. Null when the picture fails, or when the canvas is tainted
 *  (a cross-origin image without CORS headers, whose pixels can't be read). */
export function loadArtTone(url: string): Promise<Tone | null> {
  return new Promise((resolve) => {
    if (typeof Image === 'undefined') {
      resolve(null);
      return;
    }
    const img = new Image();
    // The art route is same-origin; a store CDN's art is not, and asking for
    // CORS keeps the canvas readable instead of tainted.
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      try {
        const canvas = document.createElement('canvas');
        canvas.width = SAMPLE_W;
        canvas.height = SAMPLE_H;
        const ctx = canvas.getContext('2d', { willReadFrequently: true });
        if (!ctx) {
          resolve(null);
          return;
        }
        ctx.drawImage(img, 0, 0, SAMPLE_W, SAMPLE_H);
        resolve(sampleTone(ctx.getImageData(0, 0, SAMPLE_W, SAMPLE_H).data));
      } catch {
        // A tainted canvas throws here: the theme's tokens stand.
        resolve(null);
      }
    };
    img.onerror = () => resolve(null);
    img.src = url;
  });
}

// One tone per art URL for the session, failures included: switching back to
// a game repaints at once instead of sampling again.
const toneCache = new Map<string, Tone | null>();

/** The selected game's tone, sampled once per URL. Null while loading, for a
 *  game with no art, and for a picture that can't be read. */
export function useArtTone(url: string | null): Tone | null {
  const [tone, setTone] = useState<Tone | null>(() => (url ? toneCache.get(url) ?? null : null));
  useEffect(() => {
    if (!url) {
      setTone(null);
      return;
    }
    if (toneCache.has(url)) {
      setTone(toneCache.get(url) ?? null);
      return;
    }
    let live = true;
    void loadArtTone(url).then((sampled) => {
      toneCache.set(url, sampled);
      if (live) setTone(sampled);
    });
    return () => { live = false; };
  }, [url]);
  return tone;
}

/** The stored choice, or accent (the lighter one) when nothing is stored. */
function choiceOf(settings: { game_tone?: string } | null): GameToneChoice {
  return settings?.game_tone === 'whole' ? 'whole' : 'accent';
}

/** Reads the launcher setting; a failed read keeps accent. */
export function loadGameToneChoice(): Promise<GameToneChoice> {
  try {
    return Effect.runPromise(LauncherApi.getLauncherSettings())
      .then((settings) => choiceOf(settings))
      .catch(() => 'accent' as const);
  } catch {
    return Promise.resolve('accent');
  }
}

/** Saves the pick as a launcher setting. A failed write keeps what is on
 *  screen: a colour choice is never a reason to break the UI. */
export function saveGameToneChoice(choice: GameToneChoice): Promise<void> {
  try {
    return Effect.runPromise(LauncherApi.setGameTone(choice)).then(() => {}).catch(() => {});
  } catch {
    return Promise.resolve();
  }
}

/** The game tone of the selected game's art: the theme's axis, the stored
 *  pick, the sampled tone, and the variables painted from them. The shell
 *  calls it with the selected game's hero art (issue #123). */
export interface GameTone {
  /** The most the theme supports (color.game-tone). */
  supported: GameToneSupport;
  /** What is painted: off, accent or whole. */
  mode: GameToneMode;
  /** The user's pick among accent and whole: what the switch shows. */
  choice: GameToneChoice;
  /** The sampled tone, if the art named one. */
  tone: Tone | null;
  setChoice(next: GameToneChoice): void;
}

export function useGameTone(heroUrl?: string | null): GameTone {
  const supported = useVariant('color.game-tone');
  const [choice, setChoiceState] = useState<GameToneChoice>('accent');
  const tone = useArtTone(supported === 'off' ? null : heroUrl ?? null);
  const mode = effectiveTone(supported, choice);

  useEffect(() => {
    let live = true;
    void loadGameToneChoice().then((stored) => { if (live) setChoiceState(stored); });
    return () => { live = false; };
  }, []);

  useEffect(() => { applyGameTone(mode, tone); }, [mode, tone]);

  // A theme change rewrites its tokens; the game's colour goes back on top.
  useEffect(() => onThemeApplied(repaintGameTone), []);

  const setChoice = (next: GameToneChoice) => {
    setChoiceState(next);
    void saveGameToneChoice(next);
  };

  return { supported, mode, choice, tone, setChoice };
}
