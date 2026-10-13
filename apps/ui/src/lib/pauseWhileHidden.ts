// Use less while hidden or unfocused (issue #49): the renderer pauses work
// the user can't see. Paused means the page is hidden (document.hidden —
// the window is minimized, hidden to the tray, or on another desktop), or
// the window lost focus (blur) while a game runs. The shell keeps
// backgroundThrottling on, so Chromium already throttles timers and frames;
// this stops ours too: CSS animations and transitions (the platypus-paused
// class), video and trailer playback (paused elements), and polling loops
// (they check document.hidden or shouldPause before scheduling).
import { useEffect, useState } from 'react';

/** Whether the renderer should pause: hidden always, blurred only mid-game. */
export function shouldPause(opts: { hidden: boolean; blurred: boolean; gameRunning: boolean }): boolean {
  return opts.hidden || (opts.blurred && opts.gameRunning);
}

/** The class App toggles on <html> while paused (see style.css). */
export const PAUSED_CLASS = 'platypus-paused';

/** True while the page is hidden (minimized, tray-hidden, other desktop). */
export function usePageHidden(): boolean {
  const [hidden, setHidden] = useState(() => typeof document !== 'undefined' && document.hidden);
  useEffect(() => {
    const onChange = () => setHidden(document.hidden);
    document.addEventListener('visibilitychange', onChange);
    return () => document.removeEventListener('visibilitychange', onChange);
  }, []);
  return hidden;
}

/** True while the window lost focus (blur) — only meaningful with a game running. */
export function useWindowBlurred(): boolean {
  const [blurred, setBlurred] = useState(false);
  useEffect(() => {
    const onBlur = () => setBlurred(true);
    const onFocus = () => setBlurred(false);
    window.addEventListener('blur', onBlur);
    window.addEventListener('focus', onFocus);
    return () => {
      window.removeEventListener('blur', onBlur);
      window.removeEventListener('focus', onFocus);
    };
  }, []);
  return blurred;
}

/** Pause every media element in the page (trailers, videos). */
export function pauseAllVideos(root: Document | HTMLElement = document): void {
  for (const v of root.querySelectorAll('video')) {
    try {
      (v as HTMLVideoElement).pause();
    } catch {
      // A video that can't pause (already removed) is fine.
    }
  }
}

/**
 * Pause the renderer while hidden, or while blurred with a game running.
 * Toggles the platypus-paused class on <html> (CSS stops animations and
 * transitions) and pauses playing videos. Returns whether paused.
 */
export function usePauseWhileGameRuns(gameRunning: boolean): boolean {
  const hidden = usePageHidden();
  const blurred = useWindowBlurred();
  const paused = shouldPause({ hidden, blurred, gameRunning });
  useEffect(() => {
    document.documentElement.classList.toggle(PAUSED_CLASS, paused);
    if (paused) pauseAllVideos();
    return () => document.documentElement.classList.remove(PAUSED_CLASS);
  }, [paused]);
  return paused;
}
