// Storefronts Hydra knows by name; store plugins may report others.
export type StoreType = 'steam' | 'epic' | 'gog' | 'ubisoft' | 'custom' | (string & {});

export interface ProtonVersion {
  id: string;
  name: string;
  path: string;
  is_system: boolean;
  is_custom: boolean;
  is_default: boolean;
}

export type SyncMode = 'default' | 'esync' | 'fsync' | 'ntsync' | 'off';
export type DxLayer = 'dxvk' | 'wined3d';
export type Upscaler = 'native' | 'fsr1';
export type HudPreset = 'off' | 'fps only' | 'minimal' | 'full';

export interface GameSettings {
  runner_id: string;
  wine_prefix: string;
  launch_arguments: string;
  env_vars: Record<string, string>;
  use_gamemode: boolean;
  use_mangohud: boolean;
  use_gamescope: boolean;
  gamescope_w: number;
  gamescope_h: number;
  gamescope_r: number;
  gamescope_args: string;
  eac_runtime: boolean;
  battleye_runtime: boolean;
  working_dir: string;
  // Frontend-side Proton controls (DESIGN §5). The backend has no fields for
  // these yet; they are folded into env_vars before preview/launch
  // (see lib/settings.ts), so the Go side receives real, honest env vars.
  proton_flags?: string[];
  sync_mode?: SyncMode;
  dx_layer?: DxLayer;
  upscaler?: Upscaler;
  mangohud_preset?: HudPreset;
  shader_cache?: 'on' | 'off';
  winetricks?: string[];
}

export type Tier = 'platinum' | 'gold' | 'silver' | 'bronze' | 'borked';

export interface GameFixInfo {
  source: string;
  tier: string;
  description: string;
  winetricks?: string[];
  dll_overrides?: Record<string, string>;
  launch_args?: string;
}

export interface Achievement {
  id: string;
  name: string;
  description: string;
  icon: string;
  unlocked: boolean;
  unlocked_at: number; // unix seconds, 0 when locked or unknown
  global_percent: number;
}

export interface Achievements {
  total: number;
  unlocked: number; // -1 when the user's progress isn't cached locally
  /** May arrive as null from Go (nil slice); normalized to [] at the edge. */
  list: Achievement[]; // may be a subset of total
}

export interface GameDetails {
  summary: string;
  screenshots: string[] | null;
  genres: string[] | null;
  developers: string[] | null;
  publishers: string[] | null;
  release_date: string;
  /** Wide art from the store's metadata, when better than the library entry's. */
  hero_url?: string;
  achievements: Achievements | null;
}

export interface Tweak {
  id: string;
  title: string;
  reason: string;
  kind: 'recommended' | 'troubleshoot';
  env?: Record<string, string>;
  wrappers?: string[];
}

export type SessionState = 'starting' | 'running' | 'stopping' | 'stopped' | 'failed';

export interface SessionEvent {
  game_id: string;
  state: SessionState;
  pid?: number;
  message?: string;
}

export interface Game {
  id: string;
  /** Where the game comes from: 'steam', 'custom' or a store plugin id. */
  source: string;
  store: StoreType;
  store_app_id: string;
  title: string;
  cover_url: string;
  banner_url: string;
  hero_url?: string;
  install_dir: string;
  executable: string;
  is_installed: boolean;
  settings: GameSettings;
  fix_info?: GameFixInfo;
  last_played_at?: number; // unix seconds, from Steam's localconfig.vdf
  playtime_minutes?: number;
  size_bytes?: number;
  version?: string;
  // Plugin games only.
  update_available?: boolean;
  installable?: boolean;
  note?: string;
  /** Installed outside the folders Hydra lets its plugin launch from. */
  outside_folder?: boolean;
  /** Linux build: runs without Proton, so runner and Proton-only settings don't apply. */
  native?: boolean;
  /** Plugin capabilities on offer for this game (null for Steam and sideloaded
   *  games). An empty list means none; see lib/capabilities. */
  features?: string[] | null;
  /** The version the game is held at (the `versions` capability): updates
   *  install it and the update prompt is off. */
  pinned?: VersionPin;
}

export interface SourceWarning {
  source: string;
  message: string;
  needs_auth?: boolean;
  /** The source answered "unavailable right now" (plugin error -32002): not a login problem. */
  unavailable?: boolean;
}

export interface Library {
  games: Game[];
  warnings: SourceWarning[];
}

export type PluginStatus =
  | 'ready' | 'invalid' | 'unapproved' | 'changed' | 'disabled' | 'missing-requirements'
  | 'needs-newer-hydra' | 'too-old';

export interface PluginRequirement {
  bin: string;
  hint: string;
}

/** One declared capability as the Plugins view lists it. */
export interface PluginCapability {
  name: string;
  /** The plugin offers it right now. */
  on: boolean;
  /** This Hydra has it in its registry; unknown ones are ignored. */
  known: boolean;
  /** Core has code that uses it; known but not orchestrated = not used yet. */
  orchestrated: boolean;
  features: Record<string, boolean>;
}

/** A store plugin found in ~/.local/share/hydra/plugins. Kind "data" is a
 *  JSON-only plugin (fixes, tweaks): no code, no approval, always on. */
export interface StorePlugin {
  id: string;
  name: string;
  version: string;
  author: string;
  description: string;
  store: string;
  kind: 'store' | 'data';
  status: PluginStatus;
  problem?: string;
  missing: PluginRequirement[];
  requires: PluginRequirement[];
  /** What the plugin offers right now (its last initialize), or the declared
   *  list until it has run. UI surfaces test this through lib/capabilities. */
  capabilities: string[];
  /** The manifest's list: the promise the user approves. */
  declared: string[];
  /** Every declared capability with its state and depth flags. */
  offers: PluginCapability[];
  needs: string[];
  /** Things that don't stop the plugin, e.g. capabilities this Hydra ignores. */
  warnings: string[];
  initialized: boolean;
  /** Negotiated protocol version ("1.1"). */
  protocol?: string;
  protocol_versions?: string;
  transport?: string;
  dir: string;
  exec: string;
  hash: string;
  network: string[];
  read: string[];
  write: string[];
  running: boolean;
  /** True when the plugin gave up after repeated crashes: it fails fast
   *  until the user retries it (CORE-10). */
  gave_up?: boolean;
  install_folder: string;
  default_install_folder: string;
  extra_folders: string[];
}

export interface AuthStatus {
  loggedIn: boolean;
  user?: string;
  /** Current QR login code as block-art text; refreshes while pending. */
  qr?: string;
}

export interface AuthField {
  id: string;
  label: string;
  secret?: boolean;
  placeholder?: string;
}

export interface AuthPrompt {
  url?: string;
  instructions?: string;
  /** QR code to scan with the store's mobile app, as a data URL. */
  qr?: string;
  fields: AuthField[];
}

export type JobKind = 'install' | 'update' | 'uninstall' | 'verify' | 'move';
/** The jobs a game page starts from its install panel (the rest have their own actions). */
export type GameJobKind = 'install' | 'update' | 'uninstall';
export type JobState = 'queued' | 'running' | 'paused' | 'done' | 'failed' | 'canceled';

export interface Job {
  id: string;
  kind: JobKind;
  source: string;
  game_id: string;
  store_game_id: string;
  title: string;
  state: JobState;
  install_dir?: string;
  phase?: string;
  bytes: number;
  total: number;
  speed: number; // bytes per second
  percent: number; // -1 when unknown
  error?: string;
  attempts: number;
  created_at: number;
  updated_at: number;
  /** When the user canceled the job; its partial data is deleted after the
   *  cleanup threshold (LauncherSettings.cancel_cleanup_days), until then it
   *  can be resumed. */
  canceled_at?: number;
  /** When the canceled job's leftovers were deleted (or forgotten — see
   *  leftover); 0/absent = not yet. */
  cleaned_at?: number;
  /** What may remain on disk when the plugin couldn't delete the data, or why
   *  the last cleanup attempt failed. */
  leftover?: string;
  /** The version the install or update asked for (never a password). */
  target?: VersionTarget;
  /** A verify job's second run: it restores what the check found damaged. */
  repair?: boolean;
  /** Where a move takes the game (install_dir is where it is now). */
  destination?: string;
  /** What a verify job found, once it is done. */
  verify?: VerifyReport;
}

/** What a files check found (protocol 1.4). A repair run reports what it
 *  restored, and what is still wrong afterwards. */
export interface VerifyReport {
  checked: number;
  damaged: number;
  missing: number;
  repaired: number;
}

/** What a move would do, before it is queued. */
export interface MoveCheck {
  to: string;
  need_bytes: number;
  free_bytes: number;
  /** The plugin moves the game itself (game.move); otherwise Hydra does. */
  plugin_moves: boolean;
}

/** What game.install/game.update should install: a channel (moving), a build
 *  (immutable) or the text the user typed where the store has no list. */
export interface VersionTarget {
  channel?: string;
  build?: string;
  manual?: Record<string, string>;
}

export interface VersionChannel {
  id: string;
  name: string;
  description?: string;
  updatedAt?: string;
  buildId?: string;
  requires: 'none' | 'password';
  unlocked?: boolean;
  /** The installed game follows this channel. */
  current?: boolean;
}

export type VersionBuildSource = 'store' | 'archive' | 'manual';

export interface VersionBuild {
  id: string;
  name?: string;
  publishedAt?: string;
  channel?: string;
  source: VersionBuildSource;
  /** Hydra fills it in: false means the plugin can't fetch the build now. */
  available: boolean;
  current?: boolean;
}

export interface VersionLink {
  label: string;
  url: string;
}

export interface VersionPin {
  kind: 'build' | 'channel';
  id: string;
}

/** A row of the install history Hydra keeps for the game. */
export interface LedgerEntry {
  store: string;
  game: string;
  build: string;
  channel?: string;
  versionName?: string;
  /** RFC 3339, UTC. */
  date: string;
  archiveRef?: unknown;
}

export interface VersionInfo {
  channels: VersionChannel[];
  builds: VersionBuild[];
  /** Something the user types where the store lists nothing (Steam: manifest ids). */
  manual: { label: string; hint?: string } | null;
  links: VersionLink[];
  /** The plugin's verdict relative to the pin (see the protocol doc). */
  updateAvailable: boolean;
  /** Hydra's own pin. */
  pinned: VersionPin | null;
  ledger: LedgerEntry[];
}

/** Hydra-wide settings, persisted in ~/.local/share/hydra/settings.json. */
export interface LauncherSettings {
  games_folder: string;
  default_games_folder: string;
  /** How long a canceled download's partial data is kept before Hydra deletes
   *  it, in days; 0 deletes it right away. */
  cancel_cleanup_days: number;
}

export type ActivityType = 'session' | 'unlock' | 'fix' | 'update' | 'download' | 'add' | 'tweak' | 'cloud';

export interface ActivityEvent {
  id: number;
  ts: number; // epoch ms
  type: ActivityType;
  game?: string;
  detail: string;
  meta?: string;
}

export type View =
  | { name: 'dashboard' }
  | { name: 'library' }
  | { name: 'game'; id: string; from: 'dashboard' | 'library' | 'fixes' }
  | { name: 'activity' }
  | { name: 'fixes' }
  | { name: 'runners' }
  | { name: 'plugins' }
  | { name: 'downloads' }
  | { name: 'settings' };

// ---- Cloud saves (protocol 1.2; docs/plugins/protocol.md "Cloud saves") ----

export type CloudState = 'synced' | 'pending' | 'conflict' | 'unsupported' | 'offline' | 'error';
export type CloudMode = 'off' | 'ask' | 'auto';

export interface CloudFile {
  size: number;
  mtime: number; // unix seconds
  machine?: string;
}

export interface CloudConflict {
  path: string;
  local: CloudFile;
  remote: CloudFile;
}

export interface CloudStatus {
  state: CloudState;
  /** Written by the plugin for people; for `unsupported` it is the reason. */
  message?: string;
  counts: { same: number; download: number; upload: number; conflict: number; unmapped: number };
  conflicts?: CloudConflict[] | null;
  checkedAt?: number; // unix seconds
  backupDir?: string;
  /** The store's own client, when the plugin reports it. Hydra doesn't sync while it runs. */
  client?: { running: boolean; name: string; canLaunch?: boolean } | null;
}

/** The game page's view: modes plus the last status of this session. */
export interface CloudInfo {
  /** Effective mode. */
  mode: CloudMode;
  plugin_mode: CloudMode;
  /** The game's own choice; '' = follow the plugin. */
  game_mode: CloudMode | '';
  status: CloudStatus | null;
  /** The job running for the game ("pull", "push", "resolve"), if any. */
  busy?: string;
  /** The game is running: its saves are settled after it exits. */
  running?: boolean;
}

export interface CloudSettings {
  plugins: Record<string, CloudMode>;
  games: Record<string, CloudMode>;
}

export type CloudPrompt = 'conflict' | 'offline' | 'client-running';

/** The "cloud" Wails event. */
export interface CloudEvent {
  game_id: string;
  kind: 'progress' | 'status' | 'prompt' | 'pushed' | 'skipped' | 'failed' | 'canceled';
  prompt?: CloudPrompt;
  message?: string;
  status?: CloudStatus;
  job_id?: string;
  phase?: 'check' | 'download' | 'upload';
  done?: number;
  total?: number;
  /** The store client's name ("Steam"), for the client prompt and skips. */
  client?: string;
  /** The game's title, for a prompt about a game the library may not list. */
  title?: string;
}
