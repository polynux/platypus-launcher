import { useEffect, useState } from 'react';
import { Effect } from 'effect';
import type { Game, StorePlugin } from '../types';
import { LauncherApi } from '../effect/runtime';
import { offers } from './capabilities';
import { addedBytes } from './options';

/** What the page can say about the size of a fresh install. */
export type InstallSize =
  | { state: 'loading' }
  | { state: 'unknown' }
  | { state: 'known'; base: number; added: number; total: number };

/** Combines the store's figure with the chosen add-ons: null base is "unknown". */
export function installSize(base: number | null, added: number): InstallSize {
  return base == null || base <= 0 ? { state: 'unknown' } : { state: 'known', base, added, total: base + added };
}

/** The size of installing a game that isn't installed yet: the store's figure
 *  (core caches it) plus the add-ons chosen for it when the store has options.
 *  `enabled` false asks nothing. A store that can't tell gives 'unknown'. */
export function useInstallSize(game: Game, plugin: StorePlugin | undefined, enabled: boolean): InstallSize {
  const [size, setSize] = useState<InstallSize>({ state: 'loading' });
  const withOptions = !!plugin && game.installable !== false && offers(plugin, 'install-options', { game });
  useEffect(() => {
    if (!enabled) return;
    let live = true;
    setSize({ state: 'loading' });
    const base = Effect.runPromise(LauncherApi.getInstallSize(game)).then((n) => n, () => null);
    // The add-ons ride on the base; losing them (the store can't list) leaves the base alone.
    const added = withOptions
      ? Effect.runPromise(LauncherApi.getGameOptions(game)).then((o) => addedBytes(o.groups, o.chosen), () => 0)
      : Promise.resolve(0);
    void Promise.all([base, added]).then(([b, a]) => { if (live) setSize(installSize(b, a)); });
    return () => { live = false; };
    // A new game or version is a new question; settings edits are not.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, game.id, game.version, game.pinned?.id, withOptions]);
  return size;
}
