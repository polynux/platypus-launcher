import { useCallback, useEffect, useState } from 'react';
import type { Compat, Game } from '../types';
import { COMPAT_CHANGED, gameCompat, setGameCompat } from './compat';

/** The compat facts of a game, loaded once it's shown. `update` replaces them
 *  with an answer from a save (the Steam AppID). A failed load gives null. */
export function useCompat(game: Game): { compat: Compat | null; update: (c: Compat) => void } {
  const [compat, setCompat] = useState<Compat | null>(null);
  useEffect(() => {
    let live = true;
    setCompat(null);
    gameCompat(game)
      .then((c) => live && setCompat(c))
      .catch(() => live && setCompat(null));
    return () => {
      live = false;
    };
    // Reload when the game or its Steam AppID/provider badges change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id, game.store_app_id, game.native, game.is_installed, game.compat_badges]);
  // Another view of the same game may save a new answer (the Steam AppID in
  // the settings modal); this view takes it too.
  useEffect(() => {
    const onChange = (e: Event) => {
      const d = (e as CustomEvent<{ id: string; compat: Compat }>).detail;
      if (d.id === game.id) setCompat(d.compat);
    };
    window.addEventListener(COMPAT_CHANGED, onChange);
    return () => window.removeEventListener(COMPAT_CHANGED, onChange);
  }, [game.id]);
  const update = useCallback(
    (c: Compat) => {
      setGameCompat(game.id, c);
    },
    [game.id]
  );
  return { compat, update };
}
