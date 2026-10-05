import { Effect } from 'effect';
import type { Game, ProtonVersion, PluginInfo } from '../types';
import {
  GetGames,
  GetProtonVersions,
  LaunchGame,
  ListPlugins,
  TogglePlugin,
  PreviewLaunchCommand,
} from '../../wailsjs/go/main/App';

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
        return (await LaunchGame(game as never)) as string;
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),

  previewCommand: (game: Game): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        if (!window.go?.main?.App) throw wailsUnavailable('PreviewLaunchCommand');
        const res = (await PreviewLaunchCommand(game as never)) as string | Record<string, string>;
        return typeof res === 'string' ? res : JSON.stringify(res);
      },
      catch: (err) => (err instanceof Error ? err : new Error(String(err))),
    }),
};