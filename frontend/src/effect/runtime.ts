import { Effect } from 'effect';
import type {
  AuthPrompt,
  AuthStatus,
  Game,
  GameDetails,
  GameFixInfo,
  GameSettings,
  Job,
  LauncherSettings,
  Library,
  ProtonVersion,
  SessionEvent,
  StorePlugin,
  Tweak,
} from '../types';
import { toBackendGame } from '../lib/settings';
import * as App from '../../wailsjs/go/main/App';
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

/** One bound Go call as an Effect. Go errors reject the Wails promise. */
function call<T>(op: string, fn: () => Promise<unknown>): Effect.Effect<T, Error> {
  return Effect.tryPromise({
    try: async () => {
      if (!window.go?.main?.App) throw wailsUnavailable(op);
      return (await fn()) as T;
    },
    catch: (err) => (err instanceof Error ? err : new Error(String(err))),
  });
}

// Games cross to Go with their structured settings folded into env_vars.
const backend = (g: Game) => toBackendGame(g) as never;

export const LauncherApi = {
  // Every source merged; plugin failures arrive as warnings, not errors.
  getLibrary: () =>
    call<Library>('GetLibrary', async () => {
      const lib = (await App.GetLibrary()) as unknown as Library;
      if (!lib || !Array.isArray(lib.games)) throw new Error('GetLibrary returned unexpected payload');
      return { games: lib.games, warnings: lib.warnings ?? [] };
    }),

  getProtonVersions: () => call<ProtonVersion[]>('GetProtonVersions', async () => (await App.GetProtonVersions()) ?? []),

  launchGame: (game: Game) => call<string>('LaunchGame', () => App.LaunchGame(backend(game))),

  previewCommand: (game: Game) => call<string>('PreviewLaunchCommand', () => App.PreviewLaunchCommand(backend(game))),

  // Curated fix first, then ProtonDB summary (Steam ids only). Resolves null
  // when neither has data.
  getGameFix: (game: Game) =>
    call<GameFixInfo | null>('GetGameFix', async () => {
      const fix = (await App.GetGameFix(game.source === 'steam' ? game.store_app_id : '', game.title)) as unknown as GameFixInfo | null;
      return fix && (fix.tier || fix.description) ? fix : null;
    }),

  // Summary, screenshots and achievements for the game page (Steam only).
  // Go marshals nil slices as null; normalize null lists to [] at the edge
  // so consumers never crash on list.map (FE-10).
  getGameDetails: (game: Game) =>
    call<GameDetails>('GetGameDetails', async () => {
      const d = (await App.GetGameDetails(backend(game))) as unknown as GameDetails;
      return {
        ...d,
        screenshots: d.screenshots ?? [],
        genres: d.genres ?? [],
        developers: d.developers ?? [],
        publishers: d.publishers ?? [],
        achievements: d.achievements ? { ...d.achievements, list: d.achievements.list ?? [] } : d.achievements,
      };
    }),

  // Hardware-based tweaks offered for every game.
  getTweaks: () => call<Tweak[]>('GetTweaks', async () => (await App.GetTweaks()) ?? []),

  // Games the user added by hand, persisted in settings.json.
  addCustomGame: (title: string, executable: string) =>
    call<Game>('AddCustomGame', () => App.AddCustomGame(title, executable)),
  removeCustomGame: (id: string) => call<void>('RemoveCustomGame', () => App.RemoveCustomGame(id)),

  getLauncherSettings: () => call<LauncherSettings>('GetLauncherSettings', () => App.GetLauncherSettings()),

  // Empty path resets to the default (~/Games). Rejects relative paths and files.
  setGamesFolder: (path: string) => call<LauncherSettings>('SetGamesFolder', () => App.SetGamesFolder(path)),

  // Native folder picker. Resolves '' when the user cancels.
  chooseGamesFolder: () => call<string>('ChooseGamesFolder', () => App.ChooseGamesFolder()),

  // Saved per-game settings, keyed by game id, exactly as saveGameSettings stored them.
  getGameSettings: () => call<Record<string, GameSettings>>('GetGameSettings', async () => (await App.GetGameSettings()) ?? {}),

  // Currently tracked sessions so late windows hydrate (CORE-1/FE-2).
  getSessions: () => call<SessionEvent[]>('ListSessions', async () => (await App.ListSessions()) ?? []),

  saveGameSettings: (id: string, settings: GameSettings) =>
    call<void>('SaveGameSettings', () => App.SaveGameSettings(id, settings as unknown as Record<string, unknown>)),

  // Store plugins.
  listPlugins: () => call<StorePlugin[]>('ListStorePlugins', async () => (await App.ListStorePlugins()) ?? []),
  rescanPlugins: () => call<StorePlugin[]>('RescanPlugins', async () => (await App.RescanPlugins()) ?? []),
  approvePlugin: (id: string) => call<void>('ApprovePlugin', () => App.ApprovePlugin(id)),
  setPluginEnabled: (id: string, enabled: boolean) => call<void>('SetPluginEnabled', () => App.SetPluginEnabled(id, enabled)),
  // Clears a gave-up plugin's crash count so the next call restarts it (CORE-10).
  retryPlugin: (id: string) => call<void>('RetryPlugin', () => App.RetryPlugin(id)),
  openPluginsFolder: () => call<void>('OpenPluginsFolder', () => App.OpenPluginsFolder()),
  setPluginInstallFolder: (id: string, path: string) => call<void>('SetPluginInstallFolder', () => App.SetPluginInstallFolder(id, path)),
  choosePluginInstallFolder: (id: string) => call<string>('ChoosePluginInstallFolder', () => App.ChoosePluginInstallFolder(id)),
  allowGameFolder: (pluginId: string, path: string) => call<void>('AllowGameFolder', () => App.AllowGameFolder(pluginId, path)),

  authStatus: (id: string) => call<AuthStatus>('PluginAuthStatus', () => App.PluginAuthStatus(id)),
  authBegin: (id: string) => call<AuthPrompt>('PluginAuthBegin', () => App.PluginAuthBegin(id)),
  authComplete: (id: string, fields: Record<string, string>) => call<AuthStatus>('PluginAuthComplete', () => App.PluginAuthComplete(id, fields)),
  logout: (id: string) => call<AuthStatus>('PluginLogout', () => App.PluginLogout(id)),

  // Install queue.
  listJobs: () => call<Job[]>('ListJobs', async () => (await App.ListJobs()) ?? []),
  installGame: (game: Game) => call<Job>('InstallGame', () => App.InstallGame(backend(game))),
  updateGame: (game: Game) => call<Job>('UpdateGame', () => App.UpdateGame(backend(game))),
  uninstallGame: (game: Game) => call<Job>('UninstallGame', () => App.UninstallGame(backend(game))),
  cancelJob: (id: string) => call<void>('CancelJob', () => App.CancelJob(id)),
  retryJob: (id: string) => call<void>('RetryJob', () => App.RetryJob(id)),
  clearFinishedJobs: () => call<void>('ClearFinishedJobs', () => App.ClearFinishedJobs()),
  // Pause keeps everything downloaded so far; resume continues with the same job id.
  pauseJob: (id: string) => call<void>('PauseJob', () => App.PauseJob(id)),
  resumeJob: (id: string) => call<void>('ResumeJob', () => App.ResumeJob(id)),
  // Deletes the downloaded data of a canceled download right now.
  discardJob: (id: string) => call<void>('DiscardJob', () => App.DiscardJob(id)),
  pauseAllJobs: () => call<void>('PauseAllJobs', () => App.PauseAllJobs()),
  resumeAllJobs: () => call<void>('ResumeAllJobs', () => App.ResumeAllJobs()),
  // How long a canceled download's data is kept before Hydra deletes it (0 = right away).
  setCancelCleanupDays: (days: number) =>
    call<LauncherSettings>('SetCancelCleanupDays', () => App.SetCancelCleanupDays(days)),
};

/** Subscribe to launched-game state changes. Returns the unsubscribe function. */
export function onSession(cb: (e: SessionEvent) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('session', cb);
}

/** Subscribe to the install queue; every event carries the whole list. */
export function onJobs(cb: (jobs: Job[]) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('jobs', cb);
}
