// Minimize to tray while a game runs (issue #49): the pure logic behind the
// shell's tray icon, kept free of Electron imports so node:test can drive it.
//
// Lifecycle decision: the tray icon exists only while the window is hidden
// for a running game — not permanently while the switch is on. A permanent
// status icon would clutter panels and leave GNOME users without a tray
// host with a dead icon; creating it only for the hidden window means there
// is nothing stale to maintain, and the window can never strand itself: the
// fallback below plus the single-instance restore always bring it back.
//
// Reachability fallback: on GNOME without the AppIndicator extension (and on
// any host where `new Tray()` throws) Electron shows no icon, so hiding the
// window would strand it. The shell then minimizes instead of hiding: the
// taskbar entry stays reachable. Launching Platypus again (the
// single-instance lock) shows the window either way.

/** One running game as the tray menu shows it. */
export interface TrayGame {
  id: string;
  title: string;
}

/** A tray menu row the shell turns into a real MenuItem. */
export type TrayMenuItem =
  | { kind: 'show'; label: string }
  | { kind: 'stop'; label: string; gameId: string }
  | { kind: 'quit'; label: string }
  | { kind: 'separator' };

/** Window actions the shell performs; TrayState decides, main.ts executes. */
export type TrayWindowAction = 'hide' | 'minimize' | 'show-quiet' | 'none';

/** Whether a session state counts as "a game runs now". */
export function isRunningState(state: string): boolean {
  return state === 'starting' || state === 'running' || state === 'stopping';
}

/**
 * What the window does when a session event says a game runs: hide (tray
 * available), minimize (no tray host), or nothing (switch off or a state
 * that isn't a running game).
 */
export function hideActionForSessionStart(
  settingOn: boolean,
  state: string,
  trayAvailable: boolean,
): TrayWindowAction {
  if (!settingOn || !isRunningState(state)) return 'none';
  return trayAvailable ? 'hide' : 'minimize';
}

/** Showing happens only when the last running game is gone. */
export function shouldShowOnSessionEnd(remaining: number): boolean {
  return remaining <= 0;
}

/** The tray menu model: Show Platypus, the running game(s) each with Stop, Quit. */
export function buildMenuModel(
  games: readonly TrayGame[],
  text: { show: string; stop: (title: string) => string; quit: string },
): TrayMenuItem[] {
  const items: TrayMenuItem[] = [{ kind: 'show', label: text.show }];
  if (games.length > 0) items.push({ kind: 'separator' });
  for (const g of games) items.push({ kind: 'stop', label: text.stop(g.title || g.id), gameId: g.id });
  items.push({ kind: 'separator' });
  items.push({ kind: 'quit', label: text.quit });
  return items;
}

/** A minimal window surface: the real BrowserWindow in main.ts, a fake in tests. */
export interface TrayWindowLike {
  isMinimized(): boolean;
  isVisible(): boolean;
  restore(): void;
  show(): void;
  focus(): void;
}

/**
 * Bring the window back for an explicit user action (tray click, "Show
 * Platypus", second launch): a hidden window shows, a minimized one
 * restores, then it focuses. A hidden window's isMinimized() is false, so
 * show() must come first — restore() alone never unhides it.
 */
export function restoreWindow(w: TrayWindowLike): void {
  if (!w.isVisible()) w.show();
  if (w.isMinimized()) w.restore();
  w.focus();
}

/** Tracks the running games and decides what the window does. */
export class TrayState {
  /** The launcher switch; the shell refreshes it from the core. */
  settingOn = false;
  /** Whether `new Tray()` worked on this host (false: minimize, never hide). */
  trayAvailable = true;
  private running = new Map<string, string>();

  /** Game ids currently running, for the menu. */
  games(): TrayGame[] {
    return [...this.running.entries()].map(([id, title]) => ({ id, title }));
  }

  runningCount(): number {
    return this.running.size;
  }

  /** A session event arrived: remember the game, decide what the window does. */
  onSessionEvent(gameId: string, title: string, state: string): TrayWindowAction {
    if (!gameId) return 'none';
    if (isRunningState(state)) {
      if (!this.running.has(gameId)) this.running.set(gameId, title || gameId);
      // Only the transition from "nothing runs" to "a game runs" hides: a
      // second game's start while hidden must not re-hide (flicker) or
      // re-minimize (taskbar flash) the window.
      if (this.running.size !== 1) return 'none';
      return hideActionForSessionStart(this.settingOn, state, this.trayAvailable);
    }
    this.running.delete(gameId);
    return 'none';
  }

  /** A session ended (stopped/failed): forget it; the caller shows the window. */
  onSessionEnd(gameId: string): boolean {
    if (!gameId) return false;
    this.running.delete(gameId);
    return shouldShowOnSessionEnd(this.running.size);
  }

  menu(text: { show: string; stop: (title: string) => string; quit: string }): TrayMenuItem[] {
    return buildMenuModel(this.games(), text);
  }
}
