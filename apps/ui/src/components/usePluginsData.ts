// Plugins data (issue #102): the view's core reads, auth cache and row
// actions, owned by the view. Blocks take the returned state and callbacks
// as props only, so a layout can move them without touching the core.
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import type { AuthStatus, StorePlugin } from '../types';
import { LauncherApi, onAuth } from '../effect/runtime';
import { offers } from '../lib/capabilities';
import { coreErrMsg } from '../lib/coreError';

export type PluginsAuthState = AuthStatus | { error: string };

// Login states outlive one visit: the core serves its cached state instantly
// and refreshes it in the background, so a revisit shows the last known state
// at once instead of flashing "checking login…" while the refresh runs.
const authCache = new Map<string, PluginsAuthState>();

/** Forgets every cached login state (tests). */
export function resetPluginsAuthCache(): void {
  authCache.clear();
}

export const hasPluginAuth = (p: StorePlugin) => offers(p, 'auth');

interface UsePluginsDataProps {
  plugins: StorePlugin[];
  onPlugins: (p: StorePlugin[]) => void;
  /** Approval, login and folder changes change what the library lists. */
  onLibraryChanged: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

export function usePluginsData({ plugins, onPlugins, onLibraryChanged, onNotify }: UsePluginsDataProps) {
  const { t } = useTranslation('plugins');
  const [auth, setAuth] = useState<Record<string, PluginsAuthState>>(() => Object.fromEntries(authCache));
  const [reviewing, setReviewing] = useState<StorePlugin | null>(null);
  const [loggingIn, setLoggingIn] = useState<StorePlugin | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const storeAuth = useCallback((id: string, s: PluginsAuthState) => {
    authCache.set(id, s);
    setAuth((a) => ({ ...a, [id]: s }));
  }, []);

  const refresh = useCallback(
    (rescan = false) =>
      Effect.runPromise(rescan ? LauncherApi.rescanPlugins() : LauncherApi.listPlugins())
        .then(onPlugins)
        .catch((e) => onNotify(coreErrMsg(e), 'err')),
    [onPlugins, onNotify]
  );

  // Login state of every approved plugin with a login. Asking starts the plugin.
  const readyIds = plugins.filter((p) => p.status === 'ready' && hasPluginAuth(p)).map((p) => p.id).join(',');
  // StrictMode mounts effects twice: only ask once per plugin id (FE-7).
  // The core answers a known state from its cache at once and refreshes it
  // in the background; the answer below only seeds the state, whose update
  // (or the auth event) re-renders.
  const statusFetched = useRef(new Set<string>());
  useEffect(() => {
    for (const id of readyIds ? readyIds.split(',') : []) {
      if (statusFetched.current.has(id)) continue;
      statusFetched.current.add(id);
      Effect.runPromise(LauncherApi.authStatus(id))
        .then((s) => storeAuth(id, s))
        .catch((e) => storeAuth(id, { error: coreErrMsg(e) }));
    }
  }, [readyIds, storeAuth]);

  // A background refresh that finds a changed state arrives as an event.
  useEffect(() => onAuth((e) => storeAuth(e.plugin, e.status)), [storeAuth]);

  const act = (id: string, what: string, program: Effect.Effect<unknown, Error>, done: string) => {
    setBusy(id + what);
    Effect.runPromise(program)
      .then(() => {
        onNotify(done);
        return refresh().then(onLibraryChanged);
      })
      .catch((e) => onNotify(coreErrMsg(e), 'err'))
      .finally(() => setBusy(null));
  };

  const logout = (p: StorePlugin) =>
    act(
      p.id,
      'logout',
      LauncherApi.logout(p.id).pipe(Effect.tap((s: AuthStatus) => Effect.sync(() => storeAuth(p.id, s)))),
      t('plugins:notify.loggedOutOf', { name: p.name })
    );

  const chooseFolder = (p: StorePlugin) =>
    Effect.runPromise(LauncherApi.choosePluginInstallFolder(p.id))
      .then((path) => path && act(p.id, 'folder', LauncherApi.setPluginInstallFolder(p.id, path), t('plugins:notify.installsInto', { name: p.name, path })))
      .catch((e) => onNotify(coreErrMsg(e), 'err'));

  return { auth, reviewing, setReviewing, loggingIn, setLoggingIn, busy, storeAuth, refresh, act, logout, chooseFolder };
}
