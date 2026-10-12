import type { Game, GameSettings } from '../types';
import type { AddGameOptions } from '../components/AddGameModal';

/** The backend calls adding a sideloaded game needs, injectable for tests. */
export interface AddGameDeps {
  addCustomGame: (title: string, executable: string) => Promise<Game>;
  saveGameSettings: (id: string, settings: GameSettings) => Promise<void>;
  removeCustomGame: (id: string) => Promise<void>;
}

/**
 * Adds a sideloaded game with its advanced options, all-or-nothing: the game
 * is only returned once its options are saved too. When saving the options
 * fails (e.g. a missing working directory), the new game is removed again and
 * the error is rethrown, so the caller can keep the dialog open on it. The
 * removal is best-effort: a rollback failure still surfaces the original
 * error.
 */
export function addGameFlow(
  deps: AddGameDeps,
  title: string,
  executable: string,
  opts?: AddGameOptions,
): Promise<{ game: Game; settings: GameSettings | null }> {
  return deps.addCustomGame(title, executable).then((game) => {
    const work = opts?.workingDir?.trim();
    const prefix = opts?.winePrefix?.trim();
    if (!work && !prefix) return { game, settings: null };
    const next: GameSettings = {
      ...game.settings,
      ...(work ? { working_dir: work } : {}),
      ...(prefix ? { wine_prefix: prefix } : {}),
    };
    return deps.saveGameSettings(game.id, next).then(
      () => ({ game: { ...game, settings: next }, settings: next }),
      (err: unknown) =>
        deps.removeCustomGame(game.id).then(
          () => { throw err; },
          () => { throw err; },
        ),
    );
  });
}
