import { Effect } from 'effect';
import type {
  AchievementProgress,
  AuthPrompt,
  CloudEvent,
  FamilyMember,
  FamilyWarning,
  SettingsSaved,
  SettingsView,
  SettingValue,
  CloudInfo,
  CloudMode,
  CloudPrompt,
  CloudSettings,
  AuthStatus,
  Game,
  GameDetails,
  RedeemResult,
  GameFixInfo,
  GameSettings,
  GameOptions,
  InstallChoices,
  Job,
  LauncherSettings,
  Library,
  MoveCheck,
  ProtonVersion,
  SessionEvent,
  SocialStatus,
  SocialView,
  StorePlugin,
  Tweak,
  VersionInfo,
  VersionPin,
  VersionTarget,
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

  // Graceful stop (SIGTERM, then SIGKILL after a few seconds); progress arrives
  // as session events ("stopping", then "stopped").
  stopGame: (gameId: string) => call<void>('StopGame', () => App.StopGame(gameId)),

  forceStopGame: (gameId: string) => call<void>('ForceStopGame', () => App.ForceStopGame(gameId)),

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

  // Cloud saves. LaunchGame syncs first and may answer with a prompt (the "cloud"
  // event) instead of launching; the answers are the calls below or a retry.
  getCloudInfo: (game: Game) => call<CloudInfo>('GetCloudInfo', () => App.GetCloudInfo(backend(game))),
  checkCloudSaves: (game: Game) => call<CloudInfo>('CheckCloudSaves', () => App.CheckCloudSaves(backend(game))),
  // choices: file path → 'local' (keep this device's copy) or 'remote' (keep the cloud's).
  resolveCloudSaves: (game: Game, choices: Record<string, 'local' | 'remote'>) =>
    call<CloudInfo>('ResolveCloudSaves', () => App.ResolveCloudSaves(backend(game), choices)),
  cancelCloudSync: (gameId: string) => call<boolean>('CancelCloudSync', () => App.CancelCloudSync(gameId)),
  launchGameAnyway: (game: Game, prompt: CloudPrompt) => call<string>('LaunchGameAnyway', () => App.LaunchGameAnyway(backend(game), prompt)),
  launchGameViaClient: (game: Game) => call<string>('LaunchGameViaClient', () => App.LaunchGameViaClient(backend(game))),
  getCloudSettings: () => call<CloudSettings>('GetCloudSettings', async () => {
    const s = (await App.GetCloudSettings()) as unknown as Partial<CloudSettings>;
    return { plugins: s?.plugins ?? {}, games: s?.games ?? {} };
  }),
  // '' restores the default (plugin: ask; game: follow its plugin).
  setCloudPluginMode: (pluginId: string, mode: CloudMode | '') => call<void>('SetCloudPluginMode', () => App.SetCloudPluginMode(pluginId, mode)),
  setCloudGameMode: (gameId: string, mode: CloudMode | '') => call<void>('SetCloudGameMode', () => App.SetCloudGameMode(gameId, mode)),
  // Game versions (the `versions` capability). The plugin lists channels and
  // builds; Hydra adds the install history and its pin.
  getGameVersions: (game: Game) =>
    call<VersionInfo>('GetGameVersions', async () => {
      const v = (await App.GetGameVersions(backend(game))) as unknown as VersionInfo;
      return {
        ...v,
        channels: v.channels ?? [],
        builds: v.builds ?? [],
        links: v.links ?? [],
        ledger: v.ledger ?? [],
        manual: v.manual ?? null,
        pinned: v.pinned ?? null,
      };
    }),
  // The password goes to the plugin and nowhere else; resolves whether it was accepted.
  unlockVersionChannel: (game: Game, channel: string, password: string) =>
    call<boolean>('UnlockVersionChannel', () => App.UnlockVersionChannel(backend(game), channel, password)),
  pinGameVersion: (game: Game, pin: VersionPin) =>
    call<void>('PinGameVersion', () => App.PinGameVersion(backend(game), pin as never)),
  unpinGameVersion: (gameId: string) => call<void>('UnpinGameVersion', () => App.UnpinGameVersion(gameId)),
  installGameVersion: (game: Game, target: VersionTarget) =>
    call<Job>('InstallGameVersion', () => App.InstallGameVersion(backend(game), target as never)),
  updateGameVersion: (game: Game, target: VersionTarget) =>
    call<Job>('UpdateGameVersion', () => App.UpdateGameVersion(backend(game), target as never)),

  // Achievements and redeem (protocol 1.7). Null when the plugin doesn't offer
  // the user's progress for this game; the game page then shows the catalogue.
  getGameAchievements: (game: Game) =>
    call<AchievementProgress | null>('GetGameAchievements', async () => {
      const a = (await App.GetGameAchievements(backend(game))) as unknown as AchievementProgress | null;
      return a ? { ...a, list: a.list ?? [] } : null;
    }),
  // The code goes to the store's plugin only; one submission at a time (Go refuses a second).
  redeemCode: (source: string, code: string) =>
    call<RedeemResult>('RedeemCode', async () => {
      const r = (await App.RedeemCode(source, code)) as unknown as RedeemResult;
      return { message: r.message ?? '', titles: r.titles ?? [] };
    }),

  // Verify files and move (protocol 1.4). A check only reports; a repair is a
  // second run, asked for after a check found damage.
  verifyGame: (game: Game) => call<Job>('VerifyGame', () => App.VerifyGame(backend(game))),
  repairGame: (game: Game) => call<Job>('RepairGame', () => App.RepairGame(backend(game))),
  pickMoveFolder: (start: string) => call<string>('PickMoveFolder', () => App.PickMoveFolder(start)),
  checkMove: (game: Game, parent: string) => call<MoveCheck>('CheckMove', () => App.CheckMove(backend(game), parent)),
  moveGame: (game: Game, parent: string) => call<Job>('MoveGame', () => App.MoveGame(backend(game), parent)),

  // Install options (the `install-options` capability). The store lists the
  // groups; Hydra adds the choices it would use now (saved, or the defaults).
  getGameOptions: (game: Game) =>
    call<GameOptions>('GetGameOptions', async () => {
      const o = (await App.GetGameOptions(backend(game))) as unknown as GameOptions;
      return {
        groups: (o.groups ?? []).map((g) => ({ ...g, values: g.values ?? [] })),
        chosen: o.chosen ?? {},
      };
    }),
  installGameWithOptions: (game: Game, choices: InstallChoices) =>
    call<Job>('InstallGameWithOptions', () => App.InstallGameWithOptions(backend(game), choices)),
  updateGameWithOptions: (game: Game, choices: InstallChoices) =>
    call<Job>('UpdateGameWithOptions', () => App.UpdateGameWithOptions(backend(game), choices)),
  /** Free bytes where the game would be installed. */
  getInstallFreeSpace: (game: Game) => call<number>('GetInstallFreeSpace', () => App.GetInstallFreeSpace(backend(game))),
  /** Bytes a fresh install of the game needs with the store's defaults. */
  getInstallSize: (game: Game) => call<number>('GetInstallSize', () => App.GetInstallSize(backend(game))),

  // Family (the `family` capability, protocol 1.8): the store's family group.
  // Shown only; Hydra enforces none of its rules.
  getFamilyMembers: (source: string) =>
    call<FamilyMember[]>('GetFamilyMembers', async () => (await App.GetFamilyMembers(source)) ?? []),

  // Plugin settings (the `settings` capability, protocol 1.8). The plugin-wide
  // scope is the Plugins view's; the game scope is one game's page. A save sends
  // only the keys the user changed (null resets one); the backend checks them
  // against the schema and keeps them with the rest.
  getPluginSettings: (source: string) =>
    call<SettingsView>('GetPluginSettings', () => App.GetPluginSettings(source)),
  savePluginSettings: (source: string, values: Record<string, SettingValue>) =>
    call<SettingsSaved>('SavePluginSettings', () => App.SavePluginSettings(source, values)),
  getGameSettingsFields: (game: Game) =>
    call<SettingsView>('GetGameSettingsFields', () => App.GetGameSettingsFields(backend(game))),
  saveGameSettingsFields: (game: Game, values: Record<string, SettingValue>) =>
    call<SettingsSaved>('SaveGameSettingsFields', () => App.SaveGameSettingsFields(backend(game), values)),

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

  // Friends lists of the stores that offer them (protocol 1.6).
  socialViews: () => call<SocialView[]>('SocialViews', async () => (await App.SocialViews()) ?? []),
  setSocialEnabled: (plugin: string, enabled: boolean) =>
    call<void>('SetSocialEnabled', () => App.SetSocialEnabled(plugin, enabled)),
  setSocialStatus: (plugin: string, state: SocialStatus) =>
    call<void>('SetSocialStatus', () => App.SetSocialStatus(plugin, state)),
  setSocialPlaying: (plugin: string, show: boolean) =>
    call<void>('SetSocialPlaying', () => App.SetSocialPlaying(plugin, show)),
  restartSocial: (plugin: string) => call<void>('RestartSocial', () => App.RestartSocial(plugin)),
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

/** Subscribe to playtime changes: a plugin's report changed a library value. */
export function onPlaytime(cb: () => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('playtime', cb);
}

/** Subscribe to family warnings: a borrowed game is about to be taken back. */
export function onFamily(cb: (w: FamilyWarning) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('family', cb);
}

/** Subscribe to cloud sync events: progress, new statuses, prompts, results. */
export function onCloud(cb: (e: CloudEvent) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('cloud', cb);
}

/** Subscribe to a store's friends list: every change carries that store's whole view. */
export function onSocial(cb: (v: SocialView) => void): () => void {
  if (!window.go?.main?.App) return () => {};
  return EventsOn('social', cb);
}
