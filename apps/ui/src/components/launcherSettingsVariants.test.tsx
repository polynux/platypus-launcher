import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Modal } from './Modal';

const state = vi.hoisted(() => ({ variants: { 'settings.presentation': 'page', 'settings.navigation': 'scroll', 'section.chrome': 'hairline' } as Record<string, string> }));
vi.mock('effect', () => ({ Effect: { runPromise: (value: unknown) => Promise.resolve(value) } }));
vi.mock('../lib/theme', () => ({ DEFAULT_THEME_ID: 'warm', loadTheme: vi.fn(), useVariant: (axis: string) => state.variants[axis] ?? (axis === 'color.game-tone' ? 'off' : 'page'), useThemeSchemes: () => ['dark'], useColorScheme: () => 'dark', saveColorSchemeChoice: vi.fn(async () => {}) }));
vi.mock('../lib/gameTone', () => ({ useGameToneChoice: () => ({ supported: 'off', choice: 'accent', setChoice: vi.fn() }) }));
vi.mock('../lib/i18n', () => ({ PSEUDO_LOCALE: 'pseudo', applyLanguage: vi.fn(), loadSessionInfo: async () => {}, pseudoAvailable: () => false }));
vi.mock('../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_target, name: string) => () => name === 'listThemes'
    ? Promise.resolve([{ id: 'warm', name: 'Warm', status: 'ok', warnings: [] }])
    : Promise.resolve({ cancel_cleanup_days: 7, language: '', theme: 'warm', ratings_online: true, artwork_online: true, game_info_online: true, chat_desktop_notify: true, minimize_to_tray: false }) }),
}));
vi.mock('./GameFoldersSettings', () => ({ GameFoldersSettings: () => <section><h2>Game folders fixture</h2></section> }));
vi.mock('./BackupsSetting', () => ({ BackupsSetting: () => <section><h2>Save backups fixture</h2></section> }));
vi.mock('./CloudSavesSetting', () => ({ CloudSavesDefaults: () => <section><h2>Cloud saves fixture</h2></section> }));

import { LauncherSettingsView } from './LauncherSettingsView';

expect.extend(matchers);
afterEach(() => { cleanup(); });

function SettingsFixture({ presentation }: { presentation: string }) {
  const settings = <LauncherSettingsView onNotify={() => {}} plugins={[]} />;
  return presentation === 'page' ? settings : <Modal label="Settings" presentation={presentation === 'sheet' ? 'sheet' : 'dialog'} onClose={() => {}}>{settings}</Modal>;
}

describe('Launcher Settings catalog variants', () => {
  const cases = [
    ['page', 'scroll', 'hairline'], ['page', 'scroll', 'framed'], ['page', 'scroll', 'plain'],
    ['page', 'tabs', 'hairline'], ['page', 'tabs', 'framed'], ['page', 'tabs', 'plain'],
    ['dialog', 'scroll', 'hairline'], ['dialog', 'scroll', 'framed'], ['dialog', 'scroll', 'plain'],
    ['dialog', 'tabs', 'hairline'], ['dialog', 'tabs', 'framed'], ['dialog', 'tabs', 'plain'],
    ['sheet', 'scroll', 'hairline'], ['sheet', 'scroll', 'framed'], ['sheet', 'scroll', 'plain'],
    ['sheet', 'tabs', 'hairline'], ['sheet', 'tabs', 'framed'], ['sheet', 'tabs', 'plain'],
  ];
  for (const [presentation, navigation, chrome] of cases) {
    it(`renders Settings with ${presentation} / ${navigation} / ${chrome} without axe violations`, async () => {
      state.variants = { 'settings.presentation': presentation, 'settings.navigation': navigation, 'section.chrome': chrome };
      const { container } = render(<SettingsFixture presentation={presentation} />);
      expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
      if (navigation === 'tabs') expect(screen.getByRole('tablist', { name: 'Settings sections' })).toBeInTheDocument();
      (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
    });
  }
});
