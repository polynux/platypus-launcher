import { describe, it, expect } from 'vitest';
import i18n from './i18n';
import { settingsTabs, pickTab, settingsGroups, tabLevel, tabSummary, type GameSettingsTab } from './gameSettingsTabs';
import type { Game, Job, ProtonVersion, StorePlugin, Tweak } from '../types';

const t = i18n.t.bind(i18n);

const plugin = (caps: string[], declared: string[] = caps): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared }) as unknown as StorePlugin;
const game = (o: Partial<Game> = {}): Game =>
  ({ id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', is_installed: true, installable: true, features: null, ...o }) as Game;
const ALL = ['install', 'cloud-saves', 'versions', 'verify', 'install-options', 'settings', 'move'];
const tweak = { id: 't', kind: 'recommended', title: 'T', reason: 'r' } as unknown as Tweak;

describe('settingsTabs', () => {
  it('gives an installed game of a full plugin every tab', () => {
    expect(settingsTabs({ game: game(), plugin: plugin(ALL), tweaks: [tweak] })).toEqual([
      'general', 'cloud', 'versions', 'options', 'plugin', 'tweaks', 'customization', 'maintenance',
    ]);
  });

  it('leaves out what the plugin does not offer', () => {
    expect(settingsTabs({ game: game(), plugin: plugin(['install']), tweaks: [] })).toEqual(['general', 'customization', 'maintenance']);
  });

  it('leaves out a capability that is declared but switched off right now', () => {
    expect(settingsTabs({ game: game(), plugin: plugin(['install'], ALL), tweaks: [] })).not.toContain('cloud');
  });

  it("follows the game's own features", () => {
    const tabs = settingsTabs({ game: game({ features: ['install', 'versions'] }), plugin: plugin(ALL), tweaks: [] });
    expect(tabs).toContain('versions');
    expect(tabs).not.toContain('cloud');
    expect(tabs).not.toContain('options');
    expect(tabs).not.toContain('plugin');
    expect(settingsTabs({ game: game({ features: [] }), plugin: plugin(ALL), tweaks: [] })).toEqual(['general', 'customization', 'maintenance']);
  });

  it('keeps file-acting tabs for an installed game; versions stay for choosing before the install', () => {
    const tabs = settingsTabs({ game: game({ is_installed: false }), plugin: plugin(ALL), tweaks: [tweak] });
    expect(tabs).toEqual(['general', 'versions', 'plugin', 'customization', 'maintenance']);
  });

  it("has no versions tab for a game Platypus can't install", () => {
    expect(settingsTabs({ game: game({ is_installed: false, installable: false }), plugin: plugin(ALL), tweaks: [] })).not.toContain('versions');
  });

  it('gives every game Maintenance for save backups; sideloaded games get it for removing too', () => {
    expect(settingsTabs({ game: game({ source: 'steam', store: 'steam' as never }), tweaks: [] })).toEqual(['general', 'customization', 'maintenance']);
    expect(settingsTabs({ game: game({ source: 'custom', store: 'custom' as never }), tweaks: [tweak] })).toEqual(['general', 'tweaks', 'customization', 'maintenance']);
    expect(settingsTabs({ game: game({ source: 'custom', store: 'custom' as never }), tweaks: [] })).toEqual(['general', 'customization', 'maintenance']);
  });

  it('opens the tab asked for only if the game has it', () => {
    const tabs = settingsTabs({ game: game(), plugin: plugin(['install']), tweaks: [] });
    expect(pickTab(tabs, 'maintenance')).toBe('maintenance');
    expect(pickTab(tabs, 'cloud')).toBe('general');
    expect(pickTab(tabs)).toBe('general');
  });
});

describe('settingsGroups (issue #129)', () => {
  it('splits the game settings tabs into Everyday and Advanced, keeping their order', () => {
    const tabs = settingsTabs({ game: game(), plugin: plugin(ALL), tweaks: [tweak] });
    expect(settingsGroups(tabs)).toEqual([
      { level: 'everyday', tabs: ['general', 'cloud', 'customization'] },
      { level: 'advanced', tabs: ['versions', 'options', 'plugin', 'tweaks', 'maintenance'] },
    ]);
  });

  it('leaves out a group with no tab', () => {
    expect(settingsGroups(['maintenance'])).toEqual([{ level: 'advanced', tabs: ['maintenance'] }]);
    expect(settingsGroups(['general'])).toEqual([{ level: 'everyday', tabs: ['general'] }]);
    expect(settingsGroups([])).toEqual([]);
  });

  it('puts every tab in exactly one group', () => {
    const all: GameSettingsTab[] = ['general', 'cloud', 'versions', 'options', 'plugin', 'tweaks', 'customization', 'maintenance'];
    expect(all.filter((tab) => tabLevel(tab) === 'everyday')).toEqual(['general', 'cloud', 'customization']);
    expect(all.filter((tab) => tabLevel(tab) === 'advanced')).toEqual(['versions', 'options', 'plugin', 'tweaks', 'maintenance']);
    expect(settingsGroups(all).flatMap((g) => g.tabs).sort()).toEqual([...all].sort());
  });
});

describe('tabSummary', () => {
  const runners = [{ id: 'Proton - Experimental', name: 'Proton Experimental' }] as ProtonVersion[];
  const ctx = (o: Partial<Parameters<typeof tabSummary>[2]> = {}) => ({
    game: game(), runners, tweaks: [], plugin: plugin(ALL), ...o,
  });
  const settings = { runner_id: 'auto', env_vars: {}, launch_arguments: '' };

  it('shows the runner and the launch arguments in General', () => {
    expect(tabSummary(t, 'general', ctx({ game: game({ settings: { ...settings } as Game['settings'] }) }))).toBe('Automatic runner');
    expect(tabSummary(t, 'general', ctx({ game: game({ native: true, settings: { ...settings } as Game['settings'] }) }))).toBe('Native Linux');
    expect(tabSummary(t, 'general', ctx({ game: game({ settings: { ...settings, runner_id: 'Proton - Experimental' } as Game['settings'] }) }))).toBe('Proton Experimental');
    expect(tabSummary(t, 'general', ctx({ game: game({ settings: { ...settings, runner_id: 'Proton - Experimental', launch_arguments: ' -novid ' } as Game['settings'] }) }))).toBe('Proton Experimental · -novid');
  });

  it('reports the store sync, the version choice, the tweak count and the rest', () => {
    expect(tabSummary(t, 'cloud', ctx())).toBe('Synced by Platypus');
    expect(tabSummary(t, 'cloud', ctx({ game: game({ cloud_sync: true }) }))).toBe('Synced by the store');
    expect(tabSummary(t, 'versions', ctx())).toBe('Latest version');
    expect(tabSummary(t, 'versions', ctx({ game: game({ update_available: true }) }))).toBe('Update available');
    expect(tabSummary(t, 'versions', ctx({ game: game({ pinned: { kind: 'build', id: 'b42' } }) }))).toBe('Pinned to b42');
    expect(tabSummary(t, 'tweaks', ctx({ tweaks: [tweak, { ...tweak, id: 't2' }] }))).toBe('2 suggested');
    expect(tabSummary(t, 'customization', ctx({ game: game({ customized: ['logo'] }) }))).toBe('1 custom');
    expect(tabSummary(t, 'customization', ctx())).toBe('Art, title and sort');
    expect(tabSummary(t, 'plugin', ctx())).toBe('Echo');
    expect(tabSummary(t, 'plugin', ctx({ plugin: undefined }))).toBe('Store plugin');
    expect(tabSummary(t, 'options', ctx())).toBe('Add-ons and install options');
    expect(tabSummary(t, 'maintenance', ctx())).toBe('Verify, move, backups');
    expect(tabSummary(t, 'maintenance', ctx({ job: { kind: 'install', percent: 42.4 } as Job }))).toBe('install 42%');
  });
});
