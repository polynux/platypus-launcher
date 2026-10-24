import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';
import React, { useState } from 'react';
import type { Game, StorePlugin, Tweak } from '../types';

const state = vi.hoisted(() => ({
  variants: {
    'game-settings.presentation': 'dialog',
    'game-settings.tabs': 'vertical',
    'section.chrome': 'hairline',
    'dialog.presentation': 'dialog',
    'dialog.size': 'auto',
  } as Record<string, string>,
}));
const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getCloudInfo: vi.fn(),
  checkCloudSaves: vi.fn(),
  getGameSettingsFields: vi.fn(),
  getGameVersions: vi.fn(),
  getCustomization: vi.fn(),
  previewCommand: vi.fn(),
  getLauncherSettings: vi.fn(),
}));

vi.mock('../lib/theme', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../lib/theme')>()),
  useVariant: (axis: string) => state.variants[axis],
}));
vi.mock('../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
}));
vi.mock('./SettingsModal', () => ({
  GeneralSettings: () => <div>General fixture</div>,
  useGameSettingsDraft: (game: Game) => ({ current: game.settings, applySaved: () => {} }),
}));
vi.mock('./LegalSection', () => ({ LegalSection: () => null }));
vi.mock('./maintenancePrefetch', () => ({
  MaintenancePrefetchProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMaintenancePrefetchSource: () => ({}),
}));

import { GameSettingsModal } from './GameSettingsModal';

expect.extend(matchers);
afterEach(() => { cleanup(); });
beforeEach(() => {
  state.variants['game-settings.presentation'] = 'dialog';
  state.variants['game-settings.tabs'] = 'vertical';
  state.variants['dialog.size'] = 'auto';
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getInstallSize.mockResolvedValue(0);
  api.getCloudInfo.mockResolvedValue({ mode: 'ask', plugin_mode: 'ask', game_mode: '', running: false, status: { state: 'synced', checkedAt: 1, counts: { same: 1, download: 0, upload: 0, conflict: 0 } } });
  api.getGameSettingsFields.mockResolvedValue({ groups: [], values: {} });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getCustomization.mockResolvedValue({ title: '', sort_title: '', art: {} });
  api.previewCommand.mockResolvedValue('wine game.exe');
  api.getLauncherSettings.mockResolvedValue({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' });
});

const ALL = ['install', 'cloud-saves', 'versions', 'verify', 'install-options', 'settings', 'move'];
const testPlugin = (): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ALL, declared: ALL, install_folder: '/games' }) as StorePlugin;
const testGame = (): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' },
}) as Game;
const testTweak = { id: 't1', kind: 'recommended', title: 'Use fsync', reason: 'faster', env: { DXVK_ASYNC: '1' } } as unknown as Tweak;

function modalProps(over: Partial<React.ComponentProps<typeof GameSettingsModal>> = {}) {
  return {
    game: testGame(), plugin: testPlugin(), runners: [], tweaks: [testTweak], jobs: [],
    onClose: () => {}, onSaveSettings: () => {}, onChangeSettings: () => {}, onJob: () => {}, onJobAction: () => {},
    onNotify: () => {}, onChanged: () => {}, onCustomized: () => {}, onCopied: () => {},
    ...over,
  };
}

describe('Game settings catalog variants', () => {
  for (const presentation of ['dialog', 'sheet', 'page']) {
    for (const tabs of ['vertical', 'horizontal']) {
      it(`renders game settings as ${presentation} with ${tabs} tabs without axe violations`, async () => {
        state.variants['game-settings.presentation'] = presentation;
        state.variants['game-settings.tabs'] = tabs;
        // A page lives in the app's main landmark (GamePage renders inside
        // App's <main>); a dialog brings its own dialog role.
        const ui = <GameSettingsModal {...modalProps()} />;
        render(presentation === 'page' ? <main>{ui}</main> : ui);
        if (presentation === 'page') {
          expect(screen.queryByRole('dialog')).toBeNull();
          expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
          expect(screen.getByRole('heading', { name: 'Game settings' })).toBeInTheDocument();
        } else {
          expect(screen.getByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeInTheDocument();
        }
        expect(screen.getByRole('tablist', { name: 'Game settings sections' }))
          .toHaveAttribute('aria-orientation', tabs);
        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(
          ['General', 'Cloud saves', 'Versions', 'Add-ons & options', 'Plugin settings', 'Suggested tweaks', 'Customization', 'Maintenance'],
        );
        // The dialog portals outside the render container, so axe runs on the
        // body: the dialog content is what must be violation-free.
        (expect(await (await import('vitest-axe')).axe(document.body)) as any).toHaveNoViolations();
      });
    }
  }

  it('traps focus, closes with Escape and restores focus to the settings gear', async () => {
    const user = userEvent.setup();
    const closed = vi.fn();
    function Host() {
      const [open, setOpen] = useState(false);
      return <><button onClick={() => setOpen(true)}>Game settings</button>{open && <GameSettingsModal {...modalProps({ onClose: () => { closed(); setOpen(false); } })} />}</>;
    }
    render(<Host />);
    const trigger = screen.getByRole('button', { name: 'Game settings' });
    await user.click(trigger);
    expect(screen.getByRole('dialog', { name: 'Settings for Echo Tiny' })).toBeInTheDocument();
    await user.tab();
    expect(trigger).not.toHaveFocus();
    await user.keyboard('{Escape}');
    expect(closed).toHaveBeenCalledOnce();
    expect(trigger).toHaveFocus();
  });

  it.each(['vertical', 'horizontal'] as const)('moves focus between tabs with arrow keys (%s)', (tabs) => {
    state.variants['game-settings.tabs'] = tabs;
    render(<GameSettingsModal {...modalProps()} />);
    const general = screen.getByRole('tab', { name: 'General' });
    fireEvent.keyDown(general, { key: tabs === 'vertical' ? 'ArrowDown' : 'ArrowRight' });
    expect(screen.getByRole('tab', { name: 'Cloud saves', selected: true })).toBeTruthy();
    expect(document.getElementById('gs-tab-cloud')).toHaveFocus();
  });

  it('honours an explicit dialog size and keeps xl for auto', () => {
    state.variants['dialog.size'] = 'lg';
    const { unmount } = render(<GameSettingsModal {...modalProps()} />);
    expect(screen.getByRole('dialog')).toHaveClass('max-w-[720px]');
    unmount();
    state.variants['dialog.size'] = 'auto';
    render(<GameSettingsModal {...modalProps()} />);
    expect(screen.getByRole('dialog')).toHaveClass('max-w-[920px]');
  });

  it('goes back to the game from the page presentation', async () => {
    state.variants['game-settings.presentation'] = 'page';
    const user = userEvent.setup();
    const closed = vi.fn();
    render(<GameSettingsModal {...modalProps({ onClose: closed })} />);
    await user.click(screen.getByRole('button', { name: 'Back' }));
    expect(closed).toHaveBeenCalledOnce();
  });
});
