// Atlas settings wiring (issue #117): the real launcher sections reach the
// Atlas page as layout blocks, in the same category grouping as today's
// static page.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import type { ThemeLayout } from '../../types';
import { BUILTIN_LAYOUTS } from '../../gen/builtinLayouts';

const state = vi.hoisted(() => ({
  variants: {
    'settings.presentation': 'page',
    'settings.navigation': 'tabs',
    'section.chrome': 'hairline',
    'settings-row.style': 'srow',
  } as Record<string, string>,
  layout: null as ThemeLayout | null,
}));
vi.mock('effect', () => ({ Effect: { runPromise: (value: unknown) => Promise.resolve(value) } }));
vi.mock('../../lib/theme', () => ({
  DEFAULT_THEME_ID: 'warm',
  loadTheme: vi.fn(),
  useVariant: (axis: string) => state.variants[axis] ?? 'page',
  useThemeLayout: (view: string) => state.layout ?? (BUILTIN_LAYOUTS as Record<string, ThemeLayout>)[view],
  useThemeSchemes: () => ['dark'],
  useColorScheme: () => 'dark',
  currentThemeVariables: () => ({ '--color-accent': '#9db0ff' }),
  onThemeApplied: () => () => {},
  saveColorSchemeChoice: vi.fn(async () => {}),
}));
vi.mock('../../lib/gameTone', () => ({ useGameToneChoice: () => ({ supported: 'off', choice: 'accent', setChoice: vi.fn() }), useDefaultAccent: () => ['', vi.fn()] }));
vi.mock('../../lib/i18n', () => ({ PSEUDO_LOCALE: 'pseudo', applyLanguage: vi.fn(), loadSessionInfo: async () => {}, pseudoAvailable: () => false }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_target, name: string) => () => name === 'listThemes'
    ? Promise.resolve([{ id: 'warm', name: 'Warm', status: 'ok', warnings: [] }])
    : Promise.resolve({ cancel_cleanup_days: 7, language: '', theme: 'warm', ratings_online: true, artwork_online: true, game_info_online: true, chat_desktop_notify: true, minimize_to_tray: false }) }),
}));
vi.mock('../GameFoldersSettings', () => ({ GameFoldersSettings: () => <section><h2>Game folders fixture</h2></section> }));
vi.mock('../BackupsSetting', () => ({ BackupsSetting: () => <section><h2>Save backups fixture</h2></section> }));
vi.mock('../CloudSavesSetting', () => ({ CloudSavesDefaults: () => <section><h2>Cloud saves fixture</h2></section> }));

import { LauncherSettingsView, SETTINGS_SECTION_NAMES } from '../LauncherSettingsView';
import { SETTINGS_BLOCK_CATS } from '../AtlasLauncherSettings';
describe('atlas settings wiring (issue #117)', () => {
  it('shows the real Language and tray sections under General', () => {
    render(<LauncherSettingsView onNotify={() => {}} plugins={[]} />);
    expect(screen.getAllByRole('button').map((b) => b.textContent)).toEqual(
      ['General', 'Appearance', 'Game folders', 'Save backups', 'Ratings and game info', 'Friends and chat', 'Store clients']);
    // Atlas renders sections as srow rows (labels, not h2s): the Language
    // picker and the tray switch land under General, in layout order.
    const body = screen.getByRole('region', { name: 'General' });
    const language = within(body).getByRole('combobox', { name: 'Language' });
    // AtlasSSwitch names the switch by its label and description together.
    const tray = within(body).getByRole('switch');
    expect(tray.getAttribute('aria-label') ?? tray.textContent).toMatch(/^Minimize to tray/);
    expect(language.compareDocumentPosition(tray) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Ratings and game info' }));
    const data = screen.getByRole('region', { name: 'Ratings and game info' });
    const switches = within(data).getAllByRole('switch');
    const names = switches.map((s) => s.getAttribute('aria-label') ?? s.textContent ?? '');
    expect(names.some((n) => n.startsWith('Compatibility ratings'))).toBe(true);
    // The game info section is block settings.gameinfo (lowercase): it must
    // reach the page, not drop out on a key mismatch.
    expect(names.some((n) => n.startsWith('Sideloaded game info lookup'))).toBe(true);
  });

  it('the blocks record keys, SETTINGS_BLOCK_CATS and the built-in layout agree', () => {
    // The blocks record is keyed by section name lowercased, so its keys
    // are SETTINGS_SECTION_NAMES lowercased by construction. A block the
    // catalog adds but SETTINGS_BLOCK_CATS misses would silently disappear
    // in tabs mode, and a renamed layout block would drop its section.
    const layout = BUILTIN_LAYOUTS['settings'];
    const layoutNames = layout.regions.flatMap((r) => r.nodes.flatMap((n) => n.row ? n.row.map((c) => c.block) : [n.block]));
    const keys = SETTINGS_SECTION_NAMES.map((n) => n.toLowerCase()).sort().join(',');
    expect(keys).toBe([...Object.keys(SETTINGS_BLOCK_CATS)].sort().join(','));
    expect(keys).toBe([...(layoutNames as string[])].sort().join(','));
  });
});
