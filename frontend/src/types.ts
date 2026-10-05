export type StoreType = 'steam' | 'epic' | 'gog' | 'ubisoft' | 'custom';

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
  list: Achievement[]; // may be a subset of total
}

export interface GameDetails {
  summary: string;
  screenshots: string[] | null;
  genres: string[] | null;
  developers: string[] | null;
  publishers: string[] | null;
  release_date: string;
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
}

export interface PluginInfo {
  id: string;
  name: string;
  author: string;
  version: string;
  description: string;
  enabled: boolean;
  store: string;
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
  | { name: 'plugins' };
