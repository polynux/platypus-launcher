// Gamepad input for controller navigation (issue #53). The Gamepad API lives
// in the renderer: navigator.getGamepads() is polled on
// requestAnimationFrame only while a pad is connected
// (gamepadconnected/gamepaddisconnected). Standard mapping: D-pad and the
// left stick (with a dead zone and key-repeat timing) move, A activates, B
// goes back, LB/RB switch sections. Pad input translates into the same
// actions the keyboard uses (ArrowLeft/Right/Up/Down events for moves, so
// local handlers like the library list or the palette see them); there is no
// second navigation system.
import { useEffect, useRef } from 'react';

export type PadAction =
  | 'up'
  | 'down'
  | 'left'
  | 'right'
  | 'activate'
  | 'back'
  | 'prevSection'
  | 'nextSection';

/** How far the left stick leans before it counts as a direction. */
export const PAD_DEAD_ZONE = 0.45;
/** A held D-pad/stick direction repeats like a held key. */
export const PAD_REPEAT_MS = 220;

export interface PadSnapshot {
  buttons: boolean[];
  axes: number[];
}

interface PadLike {
  buttons?: Array<{ pressed?: boolean } | null> | null;
  axes?: Array<number> | null;
  connected?: boolean;
}

/** Read one gamepad into pressed buttons + axes. Null when there is no pad. */
export function readPadSnapshot(gp: PadLike | null | undefined): PadSnapshot | null {
  if (!gp) return null;
  if (gp.connected === false) return null;
  return {
    buttons: (gp.buttons ?? []).map((b) => !!b?.pressed),
    axes: [...(gp.axes ?? [])],
  };
}

/** The directions a snapshot holds: D-pad plus the left stick past the dead
 *  zone. Pure, so tests drive it without a gamepad. */
export function padDirectionHolds(snap: PadSnapshot): PadAction[] {
  const out: PadAction[] = [];
  const push = (a: PadAction): void => {
    if (!out.includes(a)) out.push(a);
  };
  const b = snap.buttons;
  if (b[12]) push('up');
  if (b[13]) push('down');
  if (b[14]) push('left');
  if (b[15]) push('right');
  const ax = snap.axes[0] ?? 0;
  const ay = snap.axes[1] ?? 0;
  if (ax > PAD_DEAD_ZONE) push('right');
  else if (ax < -PAD_DEAD_ZONE) push('left');
  if (ay > PAD_DEAD_ZONE) push('down');
  else if (ay < -PAD_DEAD_ZONE) push('up');
  return out;
}

function firstPad(): PadLike | null {
  try {
    if (typeof navigator === 'undefined' || typeof navigator.getGamepads !== 'function') return null;
    const pads = navigator.getGamepads();
    if (!pads) return null;
    for (const gp of Array.from(pads)) {
      if (gp && gp.connected !== false) return gp as unknown as PadLike;
    }
    return null;
  } catch {
    return null;
  }
}

/** Poll the first connected pad and call `onAction` for edges (A/B/LB/RB)
 *  and held directions (with repeat). Starts on mount when a pad is already
 *  present, otherwise on gamepadconnected; stops on gamedisconnected. */
export function useGamepad(onAction: (a: PadAction) => void): void {
  const cb = useRef(onAction);
  cb.current = onAction;
  useEffect(() => {
    let raf = 0;
    let alive = false;
    let prevPressed: boolean[] = [];
    let heldDirs: PadAction[] = [];
    let lastFire = 0;
    const frame = (now: number): void => {
      if (!alive) return;
      raf = requestAnimationFrame(frame);
      const gp = firstPad();
      if (!gp) return;
      const snap = readPadSnapshot(gp);
      if (!snap) return;
      const pressed = snap.buttons;
      const edge = (i: number): boolean => !!pressed[i] && !prevPressed[i];
      if (edge(0)) cb.current('activate');
      if (edge(1)) cb.current('back');
      if (edge(4)) cb.current('prevSection');
      if (edge(5)) cb.current('nextSection');
      prevPressed = pressed;
      const dirs = padDirectionHolds(snap);
      const same =
        dirs.length === heldDirs.length && dirs.every((d) => heldDirs.includes(d));
      if (dirs.length > 0 && (!same || now - lastFire >= PAD_REPEAT_MS)) {
        for (const d of dirs) cb.current(d);
        heldDirs = dirs;
        lastFire = now;
      } else if (dirs.length === 0) {
        heldDirs = [];
      }
    };
    const start = (): void => {
      if (alive) return;
      if (!firstPad()) return;
      alive = true;
      prevPressed = [];
      heldDirs = [];
      lastFire = 0;
      raf = requestAnimationFrame(frame);
    };
    const stop = (): void => {
      if (firstPad()) return;
      alive = false;
      cancelAnimationFrame(raf);
    };
    window.addEventListener('gamepadconnected', start);
    window.addEventListener('gamepaddisconnected', stop);
    start();
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      window.removeEventListener('gamepadconnected', start);
      window.removeEventListener('gamepaddisconnected', stop);
    };
  }, []);
}
