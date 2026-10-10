import type { Game, StorePlugin, Tweak } from '../types';
import { offers } from './capabilities';
import { isPluginGame } from './games';

/** The tabs of the game settings dialog. */
export type GameSettingsTab =
  | 'general' | 'cloud' | 'versions' | 'options' | 'plugin' | 'tweaks' | 'customization' | 'maintenance';

export const TAB_LABELS: Record<GameSettingsTab, string> = {
  general: 'General',
  cloud: 'Cloud saves',
  versions: 'Versions',
  options: 'Add-ons & options',
  plugin: 'Plugin settings',
  tweaks: 'Suggested tweaks',
  customization: 'Customization',
  maintenance: 'Maintenance',
};

export interface TabContext {
  game: Game;
  plugin?: StorePlugin;
  tweaks: Tweak[];
}

/** The tabs a game gets, in the order they show. Each tab appears only where
 *  what it holds is on offer: the capability for the plugin and the game
 *  (CapabilitySurface's rule), the game being installed where the tab acts on
 *  files. General and Customization are core's, for every game. */
export function settingsTabs({ game, plugin, tweaks }: TabContext): GameSettingsTab[] {
  const tabs: GameSettingsTab[] = ['general'];
  const installed = game.is_installed;
  const fromPlugin = isPluginGame(game) && !!plugin;
  if (fromPlugin && installed && offers(plugin, 'cloud-saves', { game })) tabs.push('cloud');
  if (fromPlugin && game.installable !== false && offers(plugin, 'versions', { game })) tabs.push('versions');
  if (fromPlugin && installed && game.installable !== false && offers(plugin, 'install-options', { game })) tabs.push('options');
  if (fromPlugin && offers(plugin, 'settings', { game })) tabs.push('plugin');
  if (installed && tweaks.length > 0) tabs.push('tweaks');
  tabs.push('customization');
  // Verify and uninstall need the plugin; a move is core's, for any installed plugin game.
  if (fromPlugin && installed) tabs.push('maintenance');
  return tabs;
}

/** The tab to open: the one asked for if the game has it, else General. */
export function pickTab(tabs: GameSettingsTab[], want?: GameSettingsTab): GameSettingsTab {
  return want && tabs.includes(want) ? want : 'general';
}
