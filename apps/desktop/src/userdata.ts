// Where the shell keeps its own state (issue #41): window-state.json and
// the Chromium profile live in Electron's userData. With no variables set
// that stays Electron's default (it already follows XDG_CONFIG_HOME). When
// PLATYPUS_CONFIG_DIR (else PLATYPUS_DATA_DIR) points at an absolute path,
// userData moves to a `platypus-shell` subfolder of it, so a packager's
// redirected config/data carries the window state too. Call before app.ready.
import path from 'node:path';

function absDir(v: string | undefined): string | undefined {
  return v && path.isAbsolute(v) ? v : undefined;
}

/** userData for env: the override subfolder, or def (Electron's default). */
export function resolveUserData(env: NodeJS.ProcessEnv, def: string): string {
  const base = absDir(env.PLATYPUS_CONFIG_DIR) ?? absDir(env.PLATYPUS_DATA_DIR);
  if (!base) return def;
  return path.join(base, 'platypus-shell');
}

/** Environment the shell spawns the core with: everything it has, so the
 * PLATYPUS_* variables it was started with reach the core. Explicit (rather
 * than relying on spawn's default inherit) so tests can assert the
 * pass-through. */
export function coreEnv(env: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  return { ...env };
}
