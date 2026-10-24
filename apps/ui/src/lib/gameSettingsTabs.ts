// i18n: converted
import type { TFunction } from 'i18next';
import type { Game, Job, ProtonVersion, StorePlugin, Tweak } from '../types';
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

/** The accordion's two groups (issue #129): what a player changes often, and
 *  what they only touch to fix something. Every tab is in exactly one. */
export type GameSettingsLevel = 'everyday' | 'advanced';

export function tabLevel(tab: GameSettingsTab): GameSettingsLevel {
  switch (tab) {
    // Fixes, channels, per-store options and file care are advanced: a player
    // only opens them to solve something (the /21 SETTINGS_CATS grouping).
    case 'versions':
    case 'options':
    case 'plugin':
    case 'tweaks':
    case 'maintenance':
      return 'advanced';
    default:
      return 'everyday';
  }
}

/** The tabs the accordion shows, split into Everyday and Advanced. A group with
 *  no tab is left out, so a game never gets an empty heading. */
export function settingsGroups(tabs: GameSettingsTab[]): { level: GameSettingsLevel; tabs: GameSettingsTab[] }[] {
  const everyday = tabs.filter((tab) => tabLevel(tab) === 'everyday');
  const advanced = tabs.filter((tab) => tabLevel(tab) === 'advanced');
  return [
    ...(everyday.length ? [{ level: 'everyday' as const, tabs: everyday }] : []),
    ...(advanced.length ? [{ level: 'advanced' as const, tabs: advanced }] : []),
  ];
}

export interface TabSummaryContext {
  game: Game;
  plugin?: StorePlugin;
  runners: ProtonVersion[];
  tweaks: Tweak[];
  /** The game's active job, or its last failed one. */
  job?: Job;
}

/** The one-line summary the accordion shows under a category's label: the
 *  values a player would see inside it, from what the dialog already holds.
 *  Nothing new is fetched for it. */
export function tabSummary(t: TFunction, tab: GameSettingsTab, { game, plugin, runners, tweaks, job }: TabSummaryContext): string {
  switch (tab) {
    case 'general': {
      const wanted = game.settings.runner_id;
      const runner = game.native
        ? t('gameSettings:summary.native')
        : !wanted || wanted === 'auto'
          ? t('gameSettings:summary.auto')
          // runner_id is the runner's own name or id, depending on the schema.
          : runners.find((r) => r.id === wanted || r.name === wanted)?.name ?? wanted;
      const args = game.settings.launch_arguments?.trim();
      return args ? t('gameSettings:summary.args', { runner, args }) : runner;
    }
    case 'cloud':
      return game.cloud_sync ? t('gameSettings:summary.cloudStore') : t('gameSettings:summary.cloudPlatypus');
    case 'versions':
      if (game.pinned) return t('gameSettings:summary.pinned', { version: game.pinned.id });
      return game.update_available ? t('gameSettings:summary.update') : t('gameSettings:summary.latest');
    case 'options':
      return t('gameSettings:summary.options');
    case 'plugin':
      return plugin?.name ?? t('gameSettings:summary.plugin');
    case 'tweaks':
      return t('gameSettings:summary.tweaks', { count: tweaks.length });
    case 'customization':
      return game.customized?.length
        ? t('gameSettings:summary.customized', { count: game.customized.length })
        : t('gameSettings:summary.customization');
    default: // maintenance
      return job
        ? t('gameSettings:summary.job', { kind: job.kind, percent: Math.round(job.percent) })
        : t('gameSettings:summary.maintenance');
  }
}
