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
  SmartFilter,
  CloudInfo,
  CloudMode,
  CloudPrompt,
  CloudSettings,
  ArtKind,
  BackupSettings,
  Customization,
  ArtCandidate,
  DiskUsage,
  GameFolder,
  InstallFolderCheck,
  InstallLocation,
  AuthStatus,
  ChatEvent,
  ChatView,
  Collection,
  Game,
  GameDetails,
  RedeemResult,
  GameFixInfo,
  Compat,
  CrashHints,
  GameSettings,
  GameStatusInfo,
  GameOptions,
  GameTerm,
  InstallChoices,
  Job,
  LauncherSettings,
  Library,
  MigrationNotice,
  MoveCheck,
  ProtonVersion,
  SaveBackup,
  SavePath,
  RestorePreview,
  AutoBackupState,
  PrefixApplyResult,
  PrefixResetResult,
  PrefixPreset,
  PrefixShare,
  RemoveCheck,
  RemoveOptions,
  SessionEvent,
  SocialStatus,
  SocialView,
  StorePlugin,
  TermRef,
  RunnerEnv,
  SettingGroup,
  ThemeInfo,
  ThemeLayout,
  ThemeTokens,
  Tweak,
  VersionInfo,
  VersionPin,
  VersionTarget,
} from '../types';
import type { types as gen } from '../gen/models';
import { App, backendAvailable, onEvent } from './bridge';

// The backend is the desktop shell's bridge (bridge.ts). If the frontend is
// served outside it (plain browser dev), every call below fails with an
// explicit error instead of silently substituting fake data.
function backendUnavailable(op: string): Error {
  return new Error(
    `Platypus backend not available (${op}). The UI must run inside the app — see docs/electron.md, not a plain vite server.`
  );
}

/** One bound Go call as an Effect. Go errors reject the bridge promise. */
function call<T>(op: string, fn: () => Promise<unknown>): Effect.Effect<T, Error> {
  return Effect.tryPromise({
    try: async () => {
      if (!backendAvailable()) throw backendUnavailable(op);
      return (await fn()) as T;
    },
    catch: (err) => (err instanceof Error ? err : new Error(String(err))),
  });
}

// Games cross to Go as they are: the runner's choices are typed settings, and
// the backend turns them into env vars. The UI holds features as null when the
// backend listed none; Go reads that as an empty list, so the handoff is a
// direct assertion, not a conversion.
const backend = (g: Game) => g as gen.Game;

/** A compat answer with its lists present, whatever Go sent (nil slices arrive as null). */
function normalizeCompat(c: Compat): Compat {
  return { ...c, badges: c.badges ?? [], notes: c.notes ?? [], links: c.links ?? [] };
}

/** A customization with every field present, whatever Go sent. */
function normalizeCustomization(c: unknown): Customization {
  const x = (c ?? {}) as Partial<Customization>;
  return { title: x.title ?? '', sort_title: x.sort_title ?? '', art: x.art ?? {}, art_source: x.art_source ?? '', art_source_name: x.art_source_name ?? '' };
}

export const LauncherApi = {
  // Every source merged; plugin failures arrive as warnings, not errors.
  // Sources the backend listed recently come from its cache: a quiet read
  // costs no store call. A manual rescan lists everything again.
  getLibrary: () =>
    call<Library>('GetLibrary', async () => {
      const lib = await App.GetLibrary();
      if (!lib || !Array.isArray(lib.games)) throw new Error('GetLibrary returned unexpected payload');
      return { games: lib.games, warnings: lib.warnings ?? [] };
    }),

  // The manual "Rescan": every source is listed again, bypassing the cache.
  rescanLibrary: () =>
    call<Library>('RescanLibrary', async () => {
      const lib = await App.RescanLibrary();
      if (!lib || !Array.isArray(lib.games)) throw new Error('RescanLibrary returned unexpected payload');
      return { games: lib.games, warnings: lib.warnings ?? [] };
    }),

  // Drops one source's cached listing so the next read lists it again. The
  // store view calls it when it closes (issue #17): what was bought in the
  // embedded store appears in the library.
  invalidateLibrary: (source: string) => call<void>('InvalidateLibrary', () => App.InvalidateLibrary(source)),

  getProtonVersions: () => call<ProtonVersion[]>('GetProtonVersions', async () => (await App.GetProtonVersions()) ?? []),

  // Whether the machine looks online (issue #44): background work skips its
  // fetches while offline, and the library shows its last known listing.
  getNetState: () => call<gen.NetState>('GetNetState', async () => (await App.GetNetState()) ?? { online: true, since: 0 }),

  launchGame: (game: Game) => call<string>('LaunchGame', () => App.LaunchGame(backend(game))),

  // Graceful stop (SIGTERM, then SIGKILL after a few seconds); progress arrives
  // as session events ("stopping", then "stopped").
  stopGame: (gameId: string) => call<void>('StopGame', () => App.StopGame(gameId)),

  forceStopGame: (gameId: string) => call<void>('ForceStopGame', () => App.ForceStopGame(gameId)),

  previewCommand: (game: Game) => call<string>('PreviewLaunchCommand', () => App.PreviewLaunchCommand(backend(game))),

  // Curated fix recipe and Deck verdict. Compatibility provider badges arrive
  // on the library game and in getGameCompat.
  getGameFix: (game: Game) =>
    call<GameFixInfo | null>('GetGameFix', async () => {
      const fix = await App.GetGameFix(backend(game));
      return fix && (fix.tier || fix.description) ? fix : null;
    }),

  // Compatibility and protection facts from providers, DRM and anti-cheat.
  getGameCompat: (game: Game) =>
    call<Compat>('GetGameCompat', async () => normalizeCompat(await App.GetGameCompat(backend(game)))),

  // Suggestions for the game's last abnormal run (issue #46); null when there are none.
  getCrashHints: (game: Game) => call<CrashHints | null>('GameCrashHints', async () => (await App.GameCrashHints(backend(game))) ?? null),

  // Installs a winetricks verb into the game's prefix, in the background (issue #46).
  runWinetricksVerb: (game: Game, verb: string) => call<void>('RunWinetricksVerb', () => App.RunWinetricksVerb(backend(game), verb)),

  openCrashLog: (game: Game) => call<void>('OpenCrashLog', () => App.OpenCrashLog(backend(game))),

  // Saves the Steam AppID the user chose for a game's ratings ("" clears it).
  setGameSteamAppID: (game: Game, appId: string) =>
    call<Compat>('SetGameSteamAppID', async () => normalizeCompat(await App.SetGameSteamAppID(backend(game), appId))),

  // The "Fetch ratings online" switch.
  setRatingsOnline: (on: boolean) => call<LauncherSettings>('SetRatingsOnline', () => App.SetRatingsOnline(on)),

  // Summary, screenshots and achievements for the game page (every source;
  // sideloaded games resolve theirs from the linked online info, issue #38).
  // Go marshals nil slices as null; normalize null lists to [] at the edge
  // so consumers never crash on list.map (FE-10).
  getGameDetails: (game: Game) =>
    call<GameDetails>('GetGameDetails', async () => {
      const d = await App.GetGameDetails(backend(game));
      return {
        ...d,
        screenshots: d.screenshots ?? [],
        genres: d.genres ?? [],
        developers: d.developers ?? [],
        publishers: d.publishers ?? [],
        languages: (d as { languages?: string[] | null }).languages ?? [],
        tags: (d as { tags?: string[] | null }).tags ?? [],
        videos: (d as { videos?: GameDetails['videos'] | null }).videos ?? [],
        achievements: d.achievements ? { ...d.achievements, list: d.achievements.list ?? [] } : d.achievements,
      };
    }),

  // Hardware-based tweaks offered for every game.
  getTweaks: () => call<Tweak[]>('GetTweaks', async () => (await App.GetTweaks()) ?? []),
  /** The runner's per-game choices, for the game settings form (runner.options).
   *  A game the store client launches gets only the choices the Steam launch
   *  options carry (issue #88): the client picks the runner and owns the prefix. */
  getRunnerOptions: (game: Game) => call<SettingGroup[]>('GetRunnerOptions', async () => (await App.GetRunnerOptions(backend(game))).groups ?? []),
  /** The env and wrappers the runner's choices add, for the Steam launch options. */
  getRunnerEnv: (game: Game) => call<RunnerEnv>('GetRunnerEnv', () => App.GetRunnerEnv(backend(game))),

  // Games the user added by hand, persisted in settings.json.
  addCustomGame: (title: string, executable: string) =>
    call<Game>('AddCustomGame', () => App.AddCustomGame(title, executable)),
  removeCustomGame: (id: string) => call<void>('RemoveCustomGame', () => App.RemoveCustomGame(id)),
  // Removing a sideloaded game with its Wine prefix and/or game files
  // (issue #43): CheckRemove says what would go, RemoveGame queues the
  // deletion as a job. Library-only stays removeCustomGame.
  checkRemove: (gameId: string, opts: RemoveOptions) =>
    call<RemoveCheck>('CheckRemove', () => App.CheckRemove(gameId, opts)),
  removeGame: (gameId: string, opts: RemoveOptions) =>
    call<Job>('RemoveGame', () => App.RemoveGame(gameId, opts)),
  // A sideloaded game's release year (0 clears it: unknown).
  setCustomGameYear: (id: string, year: number) =>
    call<Game>('SetCustomGameYear', () => App.SetCustomGameYear(id, year)),
  // Native games (issue #47): how the executable runs by default (content
  // detection), pointing it at a new file (re-detected), and the execute bit.
  detectGameExecutable: (path: string) =>
    call<{ native: boolean; kind: string }>('DetectGameExecutable', () => App.DetectGameExecutable(path)),
  updateCustomGameExecutable: (id: string, executable: string) =>
    call<Game>('UpdateCustomGameExecutable', () => App.UpdateCustomGameExecutable(id, executable)),
  isGameExecutable: (gameId: string) =>
    call<boolean>('IsGameExecutable', () => App.IsGameExecutable(gameId)),
  makeGameExecutable: (gameId: string) =>
    call<Game>('MakeGameExecutable', () => App.MakeGameExecutable(gameId)),

  getLauncherSettings: () => call<LauncherSettings>('GetLauncherSettings', () => App.GetLauncherSettings()),
  /** The runner "auto" resolves to (issue #146): the runner new games without a
   *  runner_id use. "" goes back to the automatic pick. */
  setDefaultRunner: (id: string) => call<LauncherSettings>('SetDefaultRunner', () => App.SetDefaultRunner(id)),

  // Themes (issue #93): the picker's list, a theme's resolved tokens ("" is the
  // picked theme, what the UI applies at boot), and the pick itself.
  listThemes: () => call<ThemeInfo[]>('ListThemes', () => App.ListThemes()),
  getTheme: (id: string) => call<ThemeTokens>('GetTheme', () => App.GetTheme(id)),
  setTheme: (id: string) => call<LauncherSettings>('SetTheme', () => App.SetTheme(id)),
  // The game colour from the art (issues #123, #134): "off", "accent" or
  // "whole", "" for accent. The picked theme's color.game-tone axis caps how
  // far it applies.
  setGameTone: (tone: string) => call<LauncherSettings>('SetGameTone', () => App.SetGameTone(tone)),
  // The colour scheme (issue #134): "dark", "light" or "black", "" for dark.
  // Only themes that declare the scheme offer it; the rest stay dark.
  setColorScheme: (scheme: string) => call<LauncherSettings>('SetColorScheme', () => App.SetColorScheme(scheme)),
  // Layouts (issue #96): one view's validated layout, or null when the theme
  // ships none. Go marshals the nil layout as null, like MigrationNotice.
  getThemeLayout: (view: string) =>
    call<ThemeLayout | null>('GetThemeLayout', async () => (await App.GetThemeLayout(view)) ?? null),

  // The UI language (issue #39): "" is System, else a BCP 47 tag. Switching
  // applies live through applyLanguage (lib/i18n), without a restart.
  setLanguage: (lang: string) => call<LauncherSettings>('SetLanguage', () => App.SetLanguage(lang)),

  // The one-time Hydra→Platypus migration notice (issue #14). Resolves null
  // when there is nothing to show. Go marshals nil slices as null; normalize
  // null lists to [] at the edge so consumers never crash on list.map.
  migrationNotice: () =>
    call<MigrationNotice | null>('MigrationNotice', async () => {
      const n = await App.MigrationNotice();
      return n ? { ...n, set_aside: n.set_aside ?? [], warnings: n.warnings ?? [] } : null;
    }),
  dismissMigrationNotice: () => call<void>('DismissMigrationNotice', () => App.DismissMigrationNotice()),

  // Saved per-game settings, keyed by game id, exactly as saveGameSettings stored them.
  getGameSettings: () => call<Record<string, GameSettings>>('GetGameSettings', async () => (await App.GetGameSettings()) ?? {}),

  // Currently tracked sessions so late windows hydrate (CORE-1/FE-2).
  getSessions: () => call<SessionEvent[]>('ListSessions', async () => (await App.ListSessions()) ?? []),

  // The persisted activity feed (issues #145, #151): stored rows plus the
  // playlog's sessions, newest first. since is unix ms (0 = all).
  listActivity: (since: number, limit: number) =>
    call<gen.ActivityEvent[]>('ListActivity', async () => (await App.ListActivity(since, limit)) ?? []),

  saveGameSettings: (id: string, settings: GameSettings) =>
    call<void>('SaveGameSettings', () => App.SaveGameSettings(id, { ...settings })),

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
    const s = await App.GetCloudSettings();
    return { plugins: s.plugins ?? {}, games: s.games ?? {} };
  }),
  // '' restores the default (plugin: ask; game: follow its plugin).
  setCloudPluginMode: (pluginId: string, mode: CloudMode | '') => call<void>('SetCloudPluginMode', () => App.SetCloudPluginMode(pluginId, mode)),
  setCloudGameMode: (gameId: string, mode: CloudMode | '') => call<void>('SetCloudGameMode', () => App.SetCloudGameMode(gameId, mode)),
  // Game versions (the `versions` capability). The plugin lists channels and
  // builds; Platypus adds the install history and its pin.
  getGameVersions: (game: Game) =>
    call<VersionInfo>('GetGameVersions', async () => {
      const v = await App.GetGameVersions(backend(game));
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
    call<void>('PinGameVersion', () => App.PinGameVersion(backend(game), pin)),
  unpinGameVersion: (gameId: string) => call<void>('UnpinGameVersion', () => App.UnpinGameVersion(gameId)),
  installGameVersion: (game: Game, target: VersionTarget) =>
    call<Job>('InstallGameVersion', () => App.InstallGameVersion(backend(game), target)),
  updateGameVersion: (game: Game, target: VersionTarget) =>
    call<Job>('UpdateGameVersion', () => App.UpdateGameVersion(backend(game), target)),

  // Achievements and redeem (protocol 1.7). Null when the plugin doesn't offer
  // the user's progress for this game; the game page then shows the catalogue.
  getGameAchievements: (game: Game) =>
    call<AchievementProgress | null>('GetGameAchievements', async () => {
      const a = await App.GetGameAchievements(backend(game));
      return a ? { ...a, list: a.list ?? [] } : null;
    }),
  // The code goes to the store's plugin only; one submission at a time (Go refuses a second).
  redeemCode: (source: string, code: string) =>
    call<RedeemResult>('RedeemCode', async () => {
      const r = await App.RedeemCode(source, code);
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
  // groups; Platypus adds the choices it would use now (saved, or the defaults).
  getGameOptions: (game: Game) =>
    call<GameOptions>('GetGameOptions', async () => {
      const o = (await App.GetGameOptions(backend(game))) as GameOptions;
      return {
        groups: (o.groups ?? []).map((g) => ({ ...g, values: g.values ?? [] })),
        chosen: o.chosen ?? {},
      };
    }),
  // Terms (protocol 1.11): the agreements to accept before an install or an update.
  getGameTerms: (game: Game) =>
    call<GameTerm[]>('GetGameTerms', async () => (await App.GetGameTerms(backend(game))) ?? []),
  acceptGameTerms: (game: Game, terms: TermRef[]) =>
    call<void>('AcceptGameTerms', () => App.AcceptGameTerms(backend(game), terms)),
  installGameWithOptions: (game: Game, choices: InstallChoices) =>
    call<Job>('InstallGameWithOptions', () => App.InstallGameWithOptions(backend(game), choices)),
  updateGameWithOptions: (game: Game, choices: InstallChoices) =>
    call<Job>('UpdateGameWithOptions', () => App.UpdateGameWithOptions(backend(game), choices)),
  /** Free bytes where the game would be installed. */
  getInstallFreeSpace: (game: Game) => call<number>('GetInstallFreeSpace', () => App.GetInstallFreeSpace(backend(game))),
  /** Bytes a fresh install of the game needs with the store's defaults. */
  getInstallSize: (game: Game) => call<number>('GetInstallSize', () => App.GetInstallSize(backend(game))),

  // Install location: where a fresh install goes. "" is the store's own folder.
  getInstallLocations: (game: Game) =>
    call<InstallLocation[]>('GetInstallLocations', async () => (await App.GetInstallLocations(backend(game))) ?? []),
  checkInstallFolder: (folder: string) => call<InstallFolderCheck>('CheckInstallFolder', () => App.CheckInstallFolder(folder)),
  pickInstallFolder: (start: string) => call<string>('PickInstallFolder', () => App.PickInstallFolder(start)),
  installGameTo: (game: Game, folder: string) => call<Job>('InstallGameTo', () => App.InstallGameTo(backend(game), folder)),
  installGameWithOptionsTo: (game: Game, choices: InstallChoices, folder: string) =>
    call<Job>('InstallGameWithOptionsTo', () => App.InstallGameWithOptionsTo(backend(game), choices, folder)),
  // Settings → Game folders: the default folder (where installs go) first, then the libraries.
  getGameFolders: () => call<GameFolder[]>('GetGameFolders', async () => (await App.GetGameFolders()) ?? []),
  addGameFolder: (path: string) => call<GameFolder[]>('AddGameFolder', async () => (await App.AddGameFolder(path)) ?? []),
  removeGameFolder: (path: string) => call<GameFolder[]>('RemoveGameFolder', async () => (await App.RemoveGameFolder(path)) ?? []),
  makeDefaultGameFolder: (path: string) => call<GameFolder[]>('MakeDefaultGameFolder', async () => (await App.MakeDefaultGameFolder(path)) ?? []),
  chooseGameFolder: () => call<string>('ChooseGameFolder', () => App.ChooseGameFolder()),

  // Storage view (issue #50): the last snapshot, and a forced re-measure of
  // every size. Both return immediately; a finished pass arrives as an event.
  getDiskUsage: () => call<DiskUsage>('GetDiskUsage', () => App.GetDiskUsage()),
  refreshDiskUsage: () => call<DiskUsage>('RefreshDiskUsage', () => App.RefreshDiskUsage()),
  scanGameSize: (game: Game) => call<DiskUsage>('ScanGameSize', () => App.ScanGameSize(backend(game))),
  openGameFolder: (game: Game) => call<void>('OpenGameFolder', () => App.OpenGameFolder(backend(game))),

  // Customization: the user's title and art for a game. Core only; a game's
  // plugin never sees it.
  getCustomization: (gameId: string) => call<Customization>('GetCustomization', async () => normalizeCustomization(await App.GetCustomization(gameId))),
  setCustomTitle: (gameId: string, title: string, sortTitle: string) =>
    call<Customization>('SetCustomTitle', async () => normalizeCustomization(await App.SetCustomTitle(gameId, title, sortTitle))),
  setCustomArt: (gameId: string, kind: ArtKind, path: string) =>
    call<Customization>('SetCustomArt', async () => normalizeCustomization(await App.SetCustomArt(gameId, kind, path))),
  resetCustomArt: (gameId: string, kind: ArtKind) =>
    call<Customization>('ResetCustomArt', async () => normalizeCustomization(await App.ResetCustomArt(gameId, kind))),
  resetCustomization: (gameId: string) =>
    call<Customization>('ResetCustomization', async () => normalizeCustomization(await App.ResetCustomization(gameId))),
  pickImage: (title: string) => call<string>('PickImage', () => App.PickImage(title)),

  // Per-game status and notes (issue #51): the setters answer the saved pair
  // (status, user_notes), so the game page updates without a rescan.
  setGameStatus: (gameId: string, status: string) =>
    call<GameStatusInfo>('SetGameStatus', () => App.SetGameStatus(gameId, status)),
  setGameNotes: (gameId: string, notes: string) =>
    call<GameStatusInfo>('SetGameNotes', () => App.SetGameNotes(gameId, notes)),

  // Collections (issue #45): every change answers the whole list, in order.
  listCollections: () => call<Collection[]>('ListCollections', () => App.ListCollections()),
  createCollection: (name: string, smart: SmartFilter) =>
    call<Collection[]>('CreateCollection', () => App.CreateCollection(name, smart)),
  updateCollection: (id: string, name: string, smart: SmartFilter) =>
    call<Collection[]>('UpdateCollection', () => App.UpdateCollection(id, name, smart)),
  pinCollection: (id: string, pinned: boolean) =>
    call<Collection[]>('PinCollection', () => App.PinCollection(id, pinned)),
  reorderCollections: (ids: string[]) => call<Collection[]>('ReorderCollections', () => App.ReorderCollections(ids)),
  deleteCollection: (id: string) => call<Collection[]>('DeleteCollection', () => App.DeleteCollection(id)),
  addCollectionGame: (id: string, gameId: string) =>
    call<Collection[]>('AddCollectionGame', () => App.AddCollectionGame(id, gameId)),
  removeCollectionGame: (id: string, gameId: string) =>
    call<Collection[]>('RemoveCollectionGame', () => App.RemoveCollectionGame(id, gameId)),
  rememberCollectionTitles: (titles: Record<string, string>) =>
    call<Collection[]>('RememberCollectionTitles', () => App.RememberCollectionTitles(titles)),

  // Online artwork for sideloaded games (issues #36, art fallbacks): Steam's
  // public store search first, GOG and Lutris (IGDB) as keyless fallbacks
  // when Steam is thin, then the chosen candidate's cover, banner and logo
  // (Lutris: cover only) into the game's custom art. The search refuses
  // while the artwork switch is off.
  searchArtOnline: (title: string) =>
    call<ArtCandidate[]>('SearchArtOnline', async () => (await App.SearchArtOnline(title)) ?? []),
  applyOnlineArt: (gameId: string, source: string, id: string) =>
    call<Customization>('ApplyOnlineArt', async () => normalizeCustomization(await App.ApplyOnlineArt(gameId, source, id))),

  // The "Find artwork online" switch.
  setArtworkOnline: (on: boolean) => call<LauncherSettings>('SetArtworkOnline', () => App.SetArtworkOnline(on)),

  // Online info for sideloaded games (issue #38): link a candidate's source
  // ("steam"/app id, "gog"/product id, "lutris"/slug) or unlink with ("","").
  linkGameInfo: (gameId: string, source: string, id: string) =>
    call<string>('LinkGameInfo', () => App.LinkGameInfo(gameId, source, id)),

  // The "Find game info online" switch.
  setGameInfoOnline: (on: boolean) => call<LauncherSettings>('SetGameInfoOnline', () => App.SetGameInfoOnline(on)),

  // The "Desktop notifications for chat" switch (issue #12).
  setChatDesktopNotify: (on: boolean) => call<LauncherSettings>('SetChatDesktopNotify', () => App.SetChatDesktopNotify(on)),

  // The "Minimize to tray while a game runs" switch (issue #49), off by default.
  setMinimizeToTray: (on: boolean) => call<LauncherSettings>('SetMinimizeToTray', () => App.SetMinimizeToTray(on)),

  // Sideload dialog (issue #32): the native file picker for the game's
  // executable. "" means the user cancelled; a rejection means there is
  // no picker (the text field still works).
  pickGameExecutable: (start: string) => call<string>('PickGameExecutable', () => App.PickGameExecutable(start)),

  // Folder picker for the sideload working directory, Wine prefix and
  // global prefix folder (issues #33, #34, #37). "" means cancelled.
  pickFolder: (title: string, start: string) => call<string>('PickFolder', () => App.PickFolder(title, start)),

  // Family (the `family` capability, protocol 1.8): the store's family group.
  // Shown only; Platypus enforces none of its rules.
  getFamilyMembers: (source: string) =>
    call<FamilyMember[]>('GetFamilyMembers', async () => (await App.GetFamilyMembers(source)) ?? []),
  // The lender whose copy a borrowed game is played from (family.setPreferredCopy).
  setPreferredCopy: (game: Game, ownerId: string) =>
    call<void>('SetPreferredCopy', () => App.SetPreferredCopy(backend(game), ownerId)),

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
  uninstallGameWithoutBackup: (game: Game) =>
    call<Job>('UninstallGameWithoutBackup', () => App.UninstallGameWithoutBackup(backend(game))),
  // Save backups (issue #42): paths, versions, manual backup, restore, delete.
  getSavePaths: (game: Game) =>
    call<SavePath[]>('GetSavePaths', async () => (await App.GetSavePaths(backend(game))) ?? []),
  listSaveBackups: (gameId: string) =>
    call<SaveBackup[]>('ListSaveBackups', async () => (await App.ListSaveBackups(gameId)) ?? []),
  createSaveBackup: (game: Game) => call<Job>('CreateSaveBackup', () => App.CreateSaveBackup(backend(game))),
  restoreSaveBackup: (game: Game, backupId: string) =>
    call<Job>('RestoreSaveBackup', () => App.RestoreSaveBackup(backend(game), backupId)),
  previewRestoreSaveBackup: (game: Game, backupId: string) =>
    call<RestorePreview>('PreviewRestoreSaveBackup', () => App.PreviewRestoreSaveBackup(backend(game), backupId)),
  deleteSaveBackup: (gameId: string, backupId: string) =>
    call<void>('DeleteSaveBackup', () => App.DeleteSaveBackup(gameId, backupId)),
  getBackupSettings: () => call<BackupSettings>('GetBackupSettings', () => App.GetBackupSettings()),
  setBackupSettings: (folder: string, keep: number, mode: string, afterSession: boolean) =>
    call<BackupSettings>('SetBackupSettings', () => App.SetBackupSettings(folder, keep, mode, afterSession)),
  setGameSavePaths: (gameId: string, paths: string[]) =>
    call<void>('SetGameSavePaths', () => App.SetGameSavePaths(gameId, paths)),
  setGameSaveBackup: (gameId: string, mode: string) =>
    call<void>('SetGameSaveBackup', () => App.SetGameSaveBackup(gameId, mode)),
  getGameSaveBackup: (gameId: string) =>
    call<string>('GetGameSaveBackup', () => App.GetGameSaveBackup(gameId)),
  getAutoBackupState: (game: Game) =>
    call<AutoBackupState>('GetAutoBackupState', () => App.GetAutoBackupState(backend(game))),
  refreshSaveManifest: () => call<void>('RefreshSaveManifest', () => App.RefreshSaveManifest()),
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
  // How long a canceled download's data is kept before Platypus deletes it (0 = right away).
  setCancelCleanupDays: (days: number) =>
    call<LauncherSettings>('SetCancelCleanupDays', () => App.SetCancelCleanupDays(days)),

  // Where Wine prefixes are created (issue #37). "" resets it to the
  // default; changing it never moves or deletes existing prefixes.
  setPrefixesFolder: (path: string) =>
    call<LauncherSettings>('SetPrefixesFolder', () => App.SetPrefixesFolder(path)),

  // Every Wine prefix Platypus resolves and the games using it (issue #52):
  // the "use an existing prefix" picker and the "Shared with" lines.
  prefixUsage: () => call<PrefixShare[]>('PrefixUsage', async () => (await App.PrefixUsage()) ?? []),

  // Wine prefix presets (issue #52): the list (built-ins then the user's),
  // the user's own saves and deletes, and applying one to a game.
  listPrefixPresets: () => call<PrefixPreset[]>('ListPrefixPresets', async () => (await App.ListPrefixPresets()) ?? []),
  savePrefixPreset: (p: PrefixPreset) => call<PrefixPreset>('SavePrefixPreset', () => App.SavePrefixPreset(p)),
  deletePrefixPreset: (id: string) => call<void>('DeletePrefixPreset', () => App.DeletePrefixPreset(id)),
  applyPrefixPreset: (gameID: string, presetID: string) => call<PrefixApplyResult>('ApplyPrefixPreset', () => App.ApplyPrefixPreset(gameID, presetID)),

  // Resetting a damaged Wine prefix (issue #144): the broken prefix moves
  // aside (kept, never deleted) and a fresh one takes its place. The core
  // refuses user-chosen folders outside the prefixes root, prefix-less games
  // and running games; the UI confirms first.
  resetPrefix: (gameId: string) => call<PrefixResetResult>('ResetPrefix', () => App.ResetPrefix(gameId)),

  // Friends lists of the stores that offer them (protocol 1.6).
  socialViews: () => call<SocialView[]>('SocialViews', async () => (await App.SocialViews()) ?? []),
  setSocialEnabled: (plugin: string, enabled: boolean) =>
    call<void>('SetSocialEnabled', () => App.SetSocialEnabled(plugin, enabled)),
  setSocialStatus: (plugin: string, state: SocialStatus) =>
    call<void>('SetSocialStatus', () => App.SetSocialStatus(plugin, state)),
  setSocialPlaying: (plugin: string, show: boolean) =>
    call<void>('SetSocialPlaying', () => App.SetSocialPlaying(plugin, show)),
  restartSocial: (plugin: string) => call<void>('RestartSocial', () => App.RestartSocial(plugin)),

  // Conversations with friends (protocol 1.9, docs/plugins/protocol.md "Chat").
  chatOpen: (plugin: string, friendId: string) => call<ChatView>('ChatOpen', () => App.ChatOpen(plugin, friendId)),
  chatClose: (plugin: string, friendId: string) => call<void>('ChatClose', () => App.ChatClose(plugin, friendId)),
  chatOlder: (plugin: string, friendId: string) => call<ChatView>('ChatOlder', () => App.ChatOlder(plugin, friendId)),
  chatSend: (plugin: string, friendId: string, text: string) => call<void>('ChatSend', () => App.ChatSend(plugin, friendId, text)),
  chatResend: (plugin: string, friendId: string, clientId: string) =>
    call<void>('ChatResend', () => App.ChatResend(plugin, friendId, clientId)),
  chatUnread: () => call<ChatView[]>('ChatUnread', async () => (await App.ChatUnread()) ?? []),
  chatAlerts: () => call<Record<string, boolean>>('ChatAlerts', async () => (await App.ChatAlerts()) ?? {}),
  setChatAlerts: (plugin: string, on: boolean) => call<void>('SetChatAlerts', () => App.SetChatAlerts(plugin, on)),
};

/** Subscribe to launched-game state changes. Returns the unsubscribe function. */
export function onSession(cb: (e: SessionEvent) => void): () => void {
  return onEvent('session', cb);
}

/** Subscribe to new activity feed rows (issues #145, #151). */
export function onActivity(cb: (e: gen.ActivityEvent) => void): () => void {
  return onEvent('activity', cb);
}

/** Subscribe to the install queue; every event carries the whole list. */
export function onJobs(cb: (jobs: Job[]) => void): () => void {
  return onEvent('jobs', cb);
}

/** Subscribe to the partial results of a library scan: the games of the sources that have answered while others are still out. */
export function onLibraryPartial(cb: (lib: Library) => void): () => void {
  return onEvent('library-partial', cb);
}

/** Subscribe to disk usage snapshots: a measuring pass finished (issue #50). */
export function onDiskUsage(cb: (d: DiskUsage) => void): () => void {
  return onEvent('diskusage', cb);
}

/** Subscribe to background library changes: an empty id broadcasts a batch change. */
export function onLibrary(cb: (gameId: string) => void): () => void {
  return onEvent('library', cb);
}

/** Whether the machine looks online, and when the state began (issue #44). */
export function onNetState(cb: (s: gen.NetState) => void): () => void {
  return onEvent('netstate', cb);
}

/** Subscribe to playtime changes: a plugin's report changed a library value. */
export function onPlaytime(cb: () => void): () => void {
  return onEvent('playtime', cb);
}

/** Subscribe to family warnings: a borrowed game is about to be taken back. */
export function onFamily(cb: (w: FamilyWarning) => void): () => void {
  return onEvent('family', cb);
}

/** Subscribe to cloud sync events: progress, new statuses, prompts, results. */
export function onCloud(cb: (e: CloudEvent) => void): () => void {
  return onEvent('cloud', cb);
}

/** Subscribe to a store's friends list: every change carries that store's whole view. */
export function onSocial(cb: (v: SocialView) => void): () => void {
  return onEvent('social', cb);
}

/** Subscribe to chat changes: a conversation's messages, unread count or typing. Returns the unsubscribe function. */
export function onChat(cb: (e: ChatEvent) => void): () => void {
  return onEvent('chat', cb);
}

/** A plugin's login state after a background refresh found it changed. */
export interface AuthEvent {
  plugin: string;
  status: AuthStatus;
}

/** Subscribe to login-state changes: every background refresh that found a new state. Returns the unsubscribe function. */
export function onAuth(cb: (e: AuthEvent) => void): () => void {
  return onEvent('auth', cb);
}
