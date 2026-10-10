// The window remembers its size, position and maximized state across runs
// (in userData/window-state.json). A saved state that no longer fits any
// display — the monitor moved, unplugged, or is smaller — falls back to the
// default, so the window can never strand itself off-screen.
import { readFileSync, writeFileSync } from 'node:fs';

export interface WindowState {
  x?: number;
  y?: number;
  width: number;
  height: number;
  maximized: boolean;
}

export interface WorkArea {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const DEFAULT_STATE: WindowState = { width: 1024, height: 768, maximized: false };

const MIN_WIDTH = 640;
const MIN_HEIGHT = 480;

/** Whether the window's top-left sits on a visible work area. */
function onScreen(s: WindowState, displays: WorkArea[]): WorkArea | undefined {
  if (s.x === undefined || s.y === undefined) return undefined;
  return displays.find((d) => s.x! >= d.x && s.x! < d.x + d.width && s.y! >= d.y && s.y! < d.y + d.height);
}

/**
 * Fits a saved state onto the displays now attached: unknown or off-screen
 * positions fall back to the default; sizes clamp to the display the window
 * is on. Maximized survives (Electron maximizes after creation).
 */
export function clampState(saved: WindowState, displays: WorkArea[]): WindowState {
  const home = onScreen(saved, displays);
  if (!home) return { ...DEFAULT_STATE, maximized: saved.maximized };
  const clamp = (v: number, lo: number, hi: number): number => Math.min(Math.max(v, lo), hi);
  return {
    x: clamp(saved.x!, home.x, home.x + home.width - 100),
    y: clamp(saved.y!, home.y, home.y + home.height - 100),
    width: clamp(Math.round(saved.width) || DEFAULT_STATE.width, MIN_WIDTH, home.width),
    height: clamp(Math.round(saved.height) || DEFAULT_STATE.height, MIN_HEIGHT, home.height),
    maximized: saved.maximized,
  };
}

/** Reads the saved state, or undefined when there is none (first run) or it is corrupt. */
export function loadState(file: string): WindowState | undefined {
  try {
    const raw = JSON.parse(readFileSync(file, 'utf8')) as Partial<WindowState>;
    if (typeof raw.width !== 'number' || typeof raw.height !== 'number') return undefined;
    return { x: typeof raw.x === 'number' ? raw.x : undefined, y: typeof raw.y === 'number' ? raw.y : undefined, width: raw.width, height: raw.height, maximized: raw.maximized === true };
  } catch {
    return undefined;
  }
}

/** Persists the state; a failure (read-only profile) never stops a quit. */
export function saveState(file: string, state: WindowState): void {
  try {
    writeFileSync(file, JSON.stringify(state));
  } catch {
    /* the next run falls back to the default */
  }
}
