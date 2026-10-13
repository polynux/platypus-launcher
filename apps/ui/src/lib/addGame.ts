import type { Game, GameSettings } from '../types';
import type { AddGameOptions } from '../components/AddGameModal';

/** The backend calls adding a sideloaded game needs, injectable for tests. */
export interface AddGameDeps {
  addCustomGame: (title: string, executable: string) => Promise<Game>;
  saveGameSettings: (id: string, settings: GameSettings) => Promise<void>;
  removeCustomGame: (id: string) => Promise<void>;
  setCustomGameYear: (id: string, year: number) => Promise<Game>;
}

/**
 * Adds a sideloaded game with its advanced options, all-or-nothing: the game
 * is only returned once its options are saved too. When saving the options
 * fails (e.g. a missing working directory, a bad year), the new game is
 * removed again and the error is rethrown, so the caller can keep the dialog
 * open on it. The removal is best-effort: a rollback failure still surfaces
 * the original error.
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
    const runMode = opts?.runMode?.trim();
    const keepRunMode = runMode && runMode !== 'auto' ? runMode : '';
    const year = opts?.releaseYear;
    if (!work && !prefix && !keepRunMode && !year) return { game, settings: null };
    const next: GameSettings = {
      ...game.settings,
      ...(work ? { working_dir: work } : {}),
      ...(prefix ? { wine_prefix: prefix } : {}),
      ...(keepRunMode ? { run_mode: keepRunMode } : {}),
    };
    const saveSettings =
      work || prefix || keepRunMode
        ? deps.saveGameSettings(game.id, next).then(() => ({ ...game, settings: next }))
        : Promise.resolve(game);
    return saveSettings.then((g) =>
      year
        ? deps.setCustomGameYear(g.id, year).then((withYear) => ({ game: withYear, settings: work || prefix || keepRunMode ? next : null }))
        : { game: g, settings: work || prefix || keepRunMode ? next : null },
    ).then(
      (r) => r,
      (err: unknown) =>
        deps.removeCustomGame(game.id).then(
          () => { throw err; },
          () => { throw err; },
        ),
    );
  });
}
