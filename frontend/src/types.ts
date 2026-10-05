export type StoreType = 'steam' | 'epic' | 'gog' | 'ubisoft' | 'custom';

export interface ProtonVersion {
  id: string;
  name: string;
  path: string;
  is_system: boolean;
  is_custom: boolean;
  is_default: boolean;
}

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
}

export interface GameFixInfo {
  source: string;
  tier: string; // 'platinum' | 'gold' | 'silver' | 'bronze' | 'borked'
  description: string;
  winetricks?: string[];
  dll_overrides?: Record<string, string>;
  launch_args?: string;
}

export interface Game {
  id: string;
  store: StoreType;
  store_app_id: string;
  title: string;
  cover_url: string;
  banner_url: string;
  install_dir: string;
  executable: string;
  is_installed: boolean;
  settings: GameSettings;
  fix_info?: GameFixInfo;
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
