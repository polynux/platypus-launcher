import { Effect } from 'effect';
import type { Game, GameDetails, GameFixInfo, GameSettings, LauncherSettings, ProtonVersion, PluginInfo, SessionEvent, Tweak } from '../types';
import { toBackendGame } from '../lib/settings';
import {
  GetGames,
  GetProtonVersions,
  LaunchGame,
  ListPlugins,
  TogglePlugin,
  PreviewLaunchCommand,
  GetGameFix,
  GetGameDetails,
  GetTweaks,
  GetLauncherSettings,
  SetGamesFolder,
  ChooseGamesFolder,
  GetGameSettings,
  SaveGameSettings,
} from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

// Wails injects window.go bindings into the webview at runtime.
declare global {
  interface Window {
    go?: {
      main?: {
        App?: Record<string, (...args: unknown[]) => Promise<unknown>>;
      };
    };
  }
}

// The generated wailsjs model classes (with convertValues) don't match our
// structural src/types interfaces; at runtime Wails returns plain JSON objects
// that structurally satisfy src/types. Cast through unknown once at the edge.

// Generated Wails bindings resolve window.go at call time. If the frontend is
// served outside the compiled app (plain browser dev), every call below fails
// with an explicit error instead of silently substituting fake data.
function wailsUnavailable(op: string): Error {
  return new Error(
    `Hydra backend not available (${op}). The UI must run inside the Wails app — use \`wails dev -tags webkit2_41\`, not a plain vite server.`
  );
}

export const LauncherApi = {
  getGames: (): Effect.Effect<Game[], Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetGames');
        // Go returns ([]types.Game, error): on error, Wails rejects the promise.
        const games = await GetGames();
        if (!Array.isArray(games)) {
          throw new Error('GetGames returned unexpected payload');
        }
        return games as unknown as Game[];
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  getProtonVersions: (): Effect.Effect<ProtonVersion[], Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetProtonVersions');
        const runners = await GetProtonVersions();
        if (!Array.isArray(runners)) {
          throw new Error('GetProtonVersions returned unexpected payload');
        }
        return runners as unknown as ProtonVersion[];
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  getPlugins: (): Effect.Effect<PluginInfo[], Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('ListPlugins');
        const plugins = await ListPlugins();
        if (!Array.isArray(plugins)) {
          throw new Error('ListPlugins returned unexpected payload');
        }
        return plugins as unknown as PluginInfo[];
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  togglePlugin: (id: string, enabled: boolean): Effect.Effect<boolean, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('TogglePlugin');
        return await TogglePlugin(id, enabled);
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  launchGame: (game: Game): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('LaunchGame');
        return (await LaunchGame(toBackendGame(game) as never)) as string;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  previewCommand: (game: Game): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('PreviewLaunchCommand');
        const res = (await PreviewLaunchCommand(toBackendGame(game) as never)) as string | Record<string, string>;
        return typeof res === 'string' ? res : JSON.stringify(res);
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Curated fix first, then ProtonDB summary. Resolves null when neither has data.
  getGameFix: (game: Game): Effect.Effect<GameFixInfo | null, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetGameFix');
        const fix = (await GetGameFix(game.store_app_id, game.title)) as unknown as GameFixInfo | null;
        return fix && (fix.tier || fix.description) ? fix : null;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Summary, screenshots and achievements for the game page (Steam only).
  getGameDetails: (game: Game): Effect.Effect<GameDetails, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetGameDetails');
        return (await GetGameDetails(toBackendGame(game) as never)) as unknown as GameDetails;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Hardware-based tweaks offered for every game.
  getTweaks: (): Effect.Effect<Tweak[], Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetTweaks');
        return ((await GetTweaks()) ?? []) as unknown as Tweak[];
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  getLauncherSettings: (): Effect.Effect<LauncherSettings, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetLauncherSettings');
        return (await GetLauncherSettings()) as LauncherSettings;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Empty path resets to the default (~/Games). Rejects relative paths and files.
  setGamesFolder: (path: string): Effect.Effect<LauncherSettings, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('SetGamesFolder');
        return (await SetGamesFolder(path)) as LauncherSettings;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Native folder picker. Resolves '' when the user cancels.
  chooseGamesFolder: (): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('ChooseGamesFolder');
        return await ChooseGamesFolder();
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  // Saved per-game settings, keyed by game id, exactly as saveGameSettings stored them.
  getGameSettings: (): Effect.Effect<Record<string, GameSettings>, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('GetGameSettings');
        return ((await GetGameSettings()) ?? {}) as unknown as Record<string, GameSettings>;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  saveGameSettings: (id: string, settings: GameSettings): Effect.Effect<void, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('SaveGameSettings');
        await SaveGameSettings(id, settings as unknown as Record<string, unknown>);
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),
};

/** Subscribe to launched-game state changes. Returns the unsubscribe function. */
export function onSession(cb: (e: SessionEvent) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('session', cb);
}
