import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, Job, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  verifyGame: vi.fn(),
  repairGame: vi.fn(),
  pickMoveFolder: vi.fn(),
  checkMove: vi.fn(),
  moveGame: vi.fn(),
  checkRemove: vi.fn(),
  removeGame: vi.fn(),
  getGameDetails: vi.fn(),
  getGameVersions: vi.fn(),
  getInstallSize: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
  listSaveBackups: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onLibrary: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: ['install', 'verify', 'move'],
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;

const base = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {}, jobs: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.checkRemove.mockResolvedValue({
    targets: [
      { kind: 'prefix', path: '/pfx/gem', bytes: 1_000_000, exists: true, allowed: true },
      { kind: 'files', path: '/games/sideload', bytes: 2_000_000, exists: true, allowed: true },
    ],
    saves: { backup_id: '', files: 0, bytes: 0, no_paths: false },
  });
  api.removeGame.mockResolvedValue({ id: 'remove-1' });
  api.getSavePaths.mockResolvedValue([]);
  api.getGameSaveBackup.mockResolvedValue('');
  api.getAutoBackupState.mockResolvedValue({ on: true, reason: 'on' });
  api.listSaveBackups.mockResolvedValue([]);
});

const openSettings = () => {
  fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
};

describe('Game settings dialog height (issue #57)', () => {
  it('has a fixed height clamped to the window, scoped to this dialog', () => {
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    openSettings();
    const dialog = screen.getByRole('dialog', { name: 'Settings for Echo Tiny' });
    // Fixed height about the Main tab's, clamped: min(43rem, window minus margins).
    expect(dialog.className).toMatch(/h-\[min\(43rem,calc\(100dvh-6rem\)\)\]/);
  });

  it('scrolls only the content column, not the dialog', () => {
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    openSettings();
    const dialog = screen.getByRole('dialog', { name: 'Settings for Echo Tiny' });
    const scroll = screen.getByTestId('gs-tab-scroll');
    expect(dialog.contains(scroll)).toBe(true);
    expect(scroll.className).toMatch(/overflow-y-auto/);
    // Every tab panel renders inside that same scroll container.
    for (const tab of screen.getAllByRole('tab')) fireEvent.click(tab);
    for (const panel of screen.getAllByRole('tabpanel', { hidden: false })) {
      expect(scroll.contains(panel)).toBe(true);
    }
  });

  it('keeps the same container element and classes when switching tabs', () => {
    render(<GamePage game={game()} plugin={plugin(['install', 'verify'])} {...base} />);
    openSettings();
    const dialog = screen.getByRole('dialog', { name: 'Settings for Echo Tiny' });
    const dialogClass = dialog.className;
    const scroll = screen.getByTestId('gs-tab-scroll');
    const scrollClass = scroll.className;
    const tabs = screen.getAllByRole('tab');
    expect(tabs.length).toBeGreaterThan(1);
    for (const tab of tabs) {
      fireEvent.click(tab);
      expect(screen.getByRole('dialog', { name: 'Settings for Echo Tiny' }).className).toBe(dialogClass);
      expect(screen.getByTestId('gs-tab-scroll')).toBe(scroll);
      expect(screen.getByTestId('gs-tab-scroll').className).toBe(scrollClass);
    }
  });

  it('keeps the constant height for a sideloaded game and a Steam game with no plugin', () => {
    const custom = game({
      id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom',
      title: 'Sideloaded Gem', install_dir: '/games/sideload', executable: '/games/sideload/gem', features: null,
    });
    const { unmount } = render(<GamePage game={custom} {...base} onRemove={() => {}} />);
    openSettings();
    expect(screen.getByRole('dialog', { name: 'Settings for Sideloaded Gem' }).className)
      .toMatch(/h-\[min\(43rem,calc\(100dvh-6rem\)\)\]/);
    unmount();

    const steam = game({
      id: 'steam_10', source: 'steam', store: 'steam', store_app_id: '10',
      title: 'Steam Counter', install_dir: '/home/you/.local/share/Steam/steamapps/common/Counter',
      executable: '', installable: false, features: null,
    });
    render(<GamePage game={steam} plugin={undefined} {...base} />);
    openSettings();
    expect(screen.getByRole('dialog', { name: 'Settings for Steam Counter' }).className)
      .toMatch(/h-\[min\(43rem,calc\(100dvh-6rem\)\)\]/);
  });
});
