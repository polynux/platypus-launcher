import { describe, it, expect } from 'vitest';
import { settingsTabs, pickTab } from './gameSettingsTabs';
import type { Game, StorePlugin, Tweak } from '../types';

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
    expect(tabs).toEqual(['general', 'versions', 'plugin', 'customization']);
  });

  it("has no versions tab for a game Platypus can't install", () => {
    expect(settingsTabs({ game: game({ is_installed: false, installable: false }), plugin: plugin(ALL), tweaks: [] })).not.toContain('versions');
  });

  it('gives Steam only the core tabs; sideloaded games get Maintenance for removing', () => {
    expect(settingsTabs({ game: game({ source: 'steam', store: 'steam' as never }), tweaks: [] })).toEqual(['general', 'customization']);
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
