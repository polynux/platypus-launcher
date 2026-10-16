import { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { LauncherApi } from '../effect/runtime';
import type { CrashHints, Game } from '../types';

/** The suggestions for a game's last abnormal run (issue #46). Asked again each
 *  time the game stops running; null while it runs, and when the last run was fine. */
export function useCrashHints(game: Game, running: boolean): CrashHints | null {
  const [hints, setHints] = useState<CrashHints | null>(null);
  useEffect(() => {
    if (running) {
      setHints(null);
      return;
    }
    let live = true;
    Effect.runPromise(LauncherApi.getCrashHints(game))
      .then((h) => live && setHints(h))
      .catch(() => live && setHints(null));
    return () => {
      live = false;
    };
    // Ask once per stop, not on other edits to the game.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id, running]);
  return hints;
}
