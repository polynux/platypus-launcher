import { describe, it, expect, vi, afterEach } from 'vitest';
import { cleanup, render, screen, waitFor, fireEvent } from '@testing-library/react';

const state = vi.hoisted(() => ({ variants: { 'settings.presentation': 'page', 'settings.navigation': 'scroll', 'section.chrome': 'hairline' } as Record<string, string> }));
vi.mock('effect', () => ({ Effect: { runPromise: (value: unknown) => Promise.resolve(value) } }));
vi.mock('../../lib/theme', () => ({
  DEFAULT_THEME_ID: 'warm',
  loadTheme: vi.fn(async () => {
    // Picking cool restyles Settings itself: page/scroll -> sheet/tabs.
    state.variants['settings.presentation'] = 'sheet';
    state.variants['settings.navigation'] = 'tabs';
  }),
  useVariant: (axis: string) => state.variants[axis] ?? (axis === 'color.game-tone' ? 'off' : 'page'),
  useThemeSchemes: () => ['dark'],
  useColorScheme: () => 'dark',
  saveColorSchemeChoice: vi.fn(async () => {}),
}));
vi.mock('../../lib/gameTone', () => ({ useGameToneChoice: () => ({ supported: 'off', choice: 'accent', setChoice: vi.fn() }) }));
vi.mock('../../lib/i18n', () => ({ PSEUDO_LOCALE: 'pseudo', applyLanguage: vi.fn(), loadSessionInfo: async () => {}, pseudoAvailable: () => false }));
const api = vi.hoisted(() => ({
  listThemes: vi.fn(),
  getLauncherSettings: vi.fn(),
  setTheme: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
}));
vi.mock('../GameFoldersSettings', () => ({ GameFoldersSettings: () => <section><h2>Game folders fixture</h2></section> }));
vi.mock('../BackupsSetting', () => ({ BackupsSetting: () => <section><h2>Save backups fixture</h2></section> }));
vi.mock('../CloudSavesSetting', () => ({ CloudSavesDefaults: () => <section><h2>Cloud saves fixture</h2></section> }));

import { LauncherSettingsView } from '../LauncherSettingsView';

afterEach(() => { cleanup(); });

describe('Theme switch keeps Settings where it was (issue #111)', () => {
  it('page/scroll -> sheet/tabs keeps the Theme section and picker focus, with no uncontrolled warning', async () => {
    const errors: string[] = [];
    const origError = console.error;
    console.error = (...args: unknown[]) => { errors.push(args.map(String).join(' ')); };
    try {
      state.variants['settings.presentation'] = 'page';
      state.variants['settings.navigation'] = 'scroll';
      let currentTheme = 'warm';
      api.listThemes.mockReturnValue([
        { id: 'warm', name: 'Warm', schema: 1, supports: ['tokens'], status: 'ok', warnings: [] },
        { id: 'cool', name: 'Cool dark', schema: 1, supports: ['tokens'], status: 'ok', warnings: [] },
      ]);
      api.getLauncherSettings.mockImplementation(() => ({ theme: currentTheme }));
      api.setTheme.mockImplementation((id: unknown) => {
        currentTheme = id as string;
        return { theme: id as string };
      });

      // Page, scroll: every section is on screen, Theme picker among them.
      const first = render(<LauncherSettingsView onNotify={() => {}} plugins={[]} />);
      const picker = await screen.findByRole('combobox', { name: 'Theme' });
      await waitFor(() => expect(picker.textContent).toContain('Warm'));
      // Focusing the picker marks Theme as the section the user is on.
      (picker as HTMLElement).focus();
      expect(picker).toHaveFocus();

      // Pick cool: the theme apply flips presentation/navigation (mocked loadTheme).
      fireEvent.click(picker);
      fireEvent.click(await screen.findByRole('option', { name: 'Cool dark' }));
      await waitFor(() => expect(api.setTheme).toHaveBeenCalledWith('cool'));
      expect(state.variants['settings.navigation']).toBe('tabs');

      // App re-mounts Settings on the new presentation: fresh instance, tabs.
      first.unmount();
      cleanup();
      render(<LauncherSettingsView onNotify={() => {}} plugins={[]} />);

      // The Theme tab stays shown (not Language) and the picker keeps focus.
      const tabs = await screen.findAllByRole('tab');
      expect(tabs.map((b) => b.textContent)).toContain('Theme');
      const themeTab = screen.getByRole('tab', { name: 'Theme' });
      expect(themeTab).toHaveAttribute('aria-selected', 'true');
      expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', 'settings-tab-theme');
      const picker2 = await screen.findByRole('combobox', { name: 'Theme' });
      await waitFor(() => expect(picker2.textContent).toContain('Cool dark'));
      await waitFor(() => expect(picker2).toHaveFocus(), { timeout: 2000 });

      const uncontrolled = errors.filter((m) => m.includes('uncontrolled') && m.includes('controlled'));
      expect(uncontrolled).toEqual([]);
    } finally {
      console.error = origError;
    }
  });
});
