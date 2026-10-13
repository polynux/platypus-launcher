// i18n: converted
import type { TFunction } from 'i18next';
import type { Game, StorePlugin, Tweak } from '../types';
import { offers } from './capabilities';
import { isPluginGame } from './games';

/** The tabs of the game settings dialog. */
export type GameSettingsTab =
  | 'general' | 'cloud' | 'versions' | 'options' | 'plugin' | 'tweaks' | 'customization' | 'maintenance';

/** The label of a game settings tab, for the tab list and its heading. */
export function tabLabel(t: TFunction, tab: GameSettingsTab): string {
  switch (tab) {
    case 'general': return t('gameSettings:tabs.general');
    case 'cloud': return t('gameSettings:tabs.cloud');
    case 'versions': return t('gameSettings:tabs.versions');
    case 'options': return t('gameSettings:tabs.options');
    case 'plugin': return t('gameSettings:tabs.plugin');
    case 'tweaks': return t('gameSettings:tabs.tweaks');
    case 'customization': return t('gameSettings:tabs.customization');
    default: return t('gameSettings:tabs.maintenance');
  }
}

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
  // Save backups live in Maintenance for every game (backups for all games):
  // plugin-only rows (verify, move, uninstall) still need the plugin, but the
  // tab itself always shows so backups stay reachable. A game with no other
  // maintenance action shows just the Save backups section.
  tabs.push('maintenance');
  return tabs;
}

/** The tab to open: the one asked for if the game has it, else General. */
export function pickTab(tabs: GameSettingsTab[], want?: GameSettingsTab): GameSettingsTab {
  return want && tabs.includes(want) ? want : 'general';
}
