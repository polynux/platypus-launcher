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
export type Upscaler = 'native' | 'fsr1' | 'integer';
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

export type SessionState = 'starting' | 'running' | 'stopped' | 'failed';

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
  size_on_disk?: string;
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
}

export interface SourceWarning {
  source: string;
  message: string;
  needs_auth?: boolean;
}

export interface Library {
  games: Game[];
  warnings: SourceWarning[];
}

export type PluginStatus = 'ready' | 'invalid' | 'unapproved' | 'changed' | 'disabled' | 'missing-requirements';

export interface PluginRequirement {
  bin: string;
  hint: string;
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
  capabilities: string[];
  dir: string;
  exec: string;
  hash: string;
  network: string[];
  read: string[];
  write: string[];
  running: boolean;
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

export type JobKind = 'install' | 'update' | 'uninstall';
export type JobState = 'queued' | 'running' | 'done' | 'failed' | 'canceled';

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
}

/** Hydra-wide settings, persisted in ~/.local/share/hydra/settings.json. */
export interface LauncherSettings {
  games_folder: string;
  default_games_folder: string;
}

export type ActivityType = 'session' | 'unlock' | 'fix' | 'update' | 'download' | 'add' | 'tweak';

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
