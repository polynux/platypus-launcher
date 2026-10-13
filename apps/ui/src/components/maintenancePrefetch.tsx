import React, { createContext, useContext, useMemo } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { AutoBackupState, Game, SaveBackup, SavePath } from '../types';

/** Maintenance data, fetched once when the settings dialog opens (issue #56),
 *  so it is usually ready before the Maintenance tab is clicked. The tab
 *  awaits these same promises instead of firing its own calls: one fetch per
 *  dialog open, shared by the tab. (The sideload removal check already fetches
 *  with the tab, ahead of the confirm click, so it stays where it is.) */
export interface MaintenancePrefetch {
  gameId: string;
  backups: Promise<SaveBackup[]>;
  paths: Promise<SavePath[]>;
  mode: Promise<string>;
  /** Never rejects: a failure answers null, like the tab's own fetch. */
  auto: Promise<AutoBackupState | null>;
  /** Fresh answers written back by the tab's own reloads (a folder added, a
   *  mode changed), so later reads of the same dialog open — the uninstall
   *  confirm — reuse them instead of refetching. */
  latest: {
    backups?: SaveBackup[];
    paths?: SavePath[];
    mode?: string;
    auto?: AutoBackupState | null;
  };
}

/** A core call as a shared promise: never throws synchronously (a missing or
 *  misbehaving mock in tests answers as a rejection, not a render crash), and
 *  never an unhandled rejection when the tab never opens. */
function shared<T>(fn: () => Effect.Effect<T, Error>, onValue?: (v: T) => void): Promise<T> {
  let p: Promise<T>;
  try {
    p = Effect.runPromise(fn());
  } catch (e) {
    p = Promise.reject(e instanceof Error ? e : new Error(String(e)));
  }
  p.then((v) => onValue?.(v), () => {});
  return p;
}

/** Starts the Maintenance core calls for this dialog open. Runs once per game
 *  (see useMaintenancePrefetchSource); awaiting the same promise twice fires
 *  only one call. */
export function createMaintenancePrefetch(game: Game): MaintenancePrefetch {
  const pre: MaintenancePrefetch = {
    gameId: game.id,
    backups: shared(() => LauncherApi.listSaveBackups(game.id), (v) => { pre.latest.backups = v; }),
    paths: shared(() => LauncherApi.getSavePaths(game), (v) => { pre.latest.paths = v; }),
    mode: shared(() => LauncherApi.getGameSaveBackup(game.id), (v) => { pre.latest.mode = v; }),
    auto: shared(
      () => LauncherApi.getAutoBackupState(game).pipe(Effect.orElseSucceed(() => null)),
      (v) => { pre.latest.auto = v; },
    ),
    latest: {},
  };
  return pre;
}

const MaintenancePrefetchContext = createContext<MaintenancePrefetch | null>(null);

/** The dialog's shared Maintenance fetch, or null outside the dialog (the tab
 *  then fetches for itself, as before). */
export function useMaintenancePrefetch(): MaintenancePrefetch | null {
  return useContext(MaintenancePrefetchContext);
}

/** Starts the Maintenance calls for this dialog open and shares them below.
 *  One fetch per game: a new game id starts a new set of calls. */
export function useMaintenancePrefetchSource(game: Game): MaintenancePrefetch {
  // eslint-disable-next-line react-hooks/exhaustive-deps
  return useMemo(() => createMaintenancePrefetch(game), [game.id]);
}

export function MaintenancePrefetchProvider({ pre, children }: { pre: MaintenancePrefetch; children: React.ReactNode }) {
  return <MaintenancePrefetchContext.Provider value={pre}>{children}</MaintenancePrefetchContext.Provider>;
}
