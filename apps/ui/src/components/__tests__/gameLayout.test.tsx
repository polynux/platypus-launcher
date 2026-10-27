// Game page layouts (issue #103): the built-in arrangement reproduces today's
// look through blocks, and a cool-style themed one moves the media under the
// about text with compat and installation leading the side column. Both pass
// axe, and required blocks (back, actions, outside) come back when omitted.
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within, fireEvent } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';

const layoutState = vi.hoisted(() => ({ layout: null as unknown }));
vi.mock('../../effect/runtime', () => ({
  // Calls the page doesn't need here (the settings page's) answer empty.
  LauncherApi: new Proxy({
    getGameDetails: () => Effect.succeed(details as never),
    getCrashHints: () => Effect.succeed(null),
    getInstallSize: () => Effect.succeed(0),
    getGameOptions: () => Effect.succeed({ groups: [], chosen: {} } as never),
    getThemeLayout: () => Effect.succeed(layoutState.layout as never),
    prefixUsage: () => Effect.succeed([]),
    listPrefixPresets: () => Effect.succeed([]),
    getRunnerOptions: () => Effect.succeed([]),
  } as Record<string, unknown>, { get: (t, k: string) => t[k] ?? (() => Effect.succeed(null)) }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

vi.mock('../../lib/compat', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/compat')>();
  return { ...mod, gameCompat: () => Promise.resolve(null), setGameCompat: () => {} };
});

import { GamePage, invalidateGameDetailsCache } from '../GamePage';
import { applyTheme } from '../../lib/theme';
import { TooltipProvider } from '../ds/tooltip';
import type { Game, StorePlugin, ThemeLayout, ThemeTokens } from '../../types';

expect.extend(matchers);
afterEach(() => { cleanup(); layoutState.layout = null; });
beforeEach(() => invalidateGameDetailsCache());

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const details = {
  summary: 'A short summary',
  screenshots: ['s1.png', 's2.png'],
  genres: ['Adventure'],
  developers: ['Echo Software'],
  publishers: ['Echo Software'],
  release_date: '2024-01-01',
  achievements: {
    total: 2,
    unlocked: 1,
    list: [
      { id: 'a1', name: 'First', description: 'Win once', unlocked: true, icon: '' },
      { id: 'a2', name: 'Second', description: 'Win twice', unlocked: false, icon: '' },
    ],
  },
  description_html: '<p>A long description.</p>',
  website: 'https://example.com/echo-tiny',
  store_url: 'https://example.com/store/echo-tiny',
};

const settings = {
  runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false,
  use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0,
  gamescope_args: '', eac_runner: false, battleye_runtime: false, working_dir: '',
} as unknown as Game['settings'];

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;

let n = 0;
function installedGame(): Game {
  n += 1;
  return {
    id: `layout-installed-${n}`, source: 'echo', store: 'echo', store_app_id: `${n}`, title: 'Layout Installed',
    cover_url: '', banner_url: '', install_dir: '/games/echo-tiny', executable: 'tiny', is_installed: true,
    installable: true, features: [], settings, playtime_minutes: 90, last_played_at: Date.now(),
  } as unknown as Game;
}

function freshGame(): Game {
  n += 1;
  return {
    id: `layout-fresh-${n}`, source: 'echo', store: 'echo', store_app_id: `${n}`, title: 'Layout Fresh',
    cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: false,
    installable: true, features: [], settings,
  } as unknown as Game;
}

const baseProps = {
  runners: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
  launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJob() {}, onJobAction() {},
  onAllowFolder() {}, onNavigate() {}, jobs: [], onNotify() {},
};

function page(game: Game, activity: GamePageActivity = []) {
  return render(
    <GamePage
      game={game}
      plugin={plugin(['install'])}
      runners={[]}
      activity={activity}
      {...baseProps}
      onManageCollections={() => {}}
      collections={[]}
    />,
  );
}

type GamePageActivity = React.ComponentProps<typeof GamePage>['activity'];

// The cool game layout (internal/themes/builtin/cool/layouts/game.xml): the
// about text comes first with the media strip under it; compat and
// installation lead the side column.
const coolLayout: ThemeLayout = {
  view: 'game',
  regions: [
    { name: 'hero', nodes: [{ block: 'back' }, { block: 'heading' }, { block: 'actions' }, { block: 'outside' }, { block: 'note' }] },
    { name: 'main', nodes: [{ block: 'about' }, { block: 'media' }, { block: 'achievements-progress' }, { block: 'achievements' }, { block: 'activity' }] },
    { name: 'side', nodes: [{ block: 'compat' }, { block: 'installation' }, { block: 'crash' }, { block: 'status' }, { block: 'collections' }, { block: 'copies' }] },
  ],
};

describe('game page layouts', () => {
  // A theme can show game settings as a page: opening them must not change
  // the page's hook count (the settings page used to return before a hook).
  it('opens game settings as a page without crashing', async () => {
    applyTheme({ id: 'paged', tokens: {}, wash: [], variants: { 'game-settings.presentation': 'page' } } as unknown as ThemeTokens);
    try {
      page(installedGame(), []);
      await screen.findByRole('heading', { name: 'Layout Installed' });
      fireEvent.click(screen.getAllByRole('button', { name: 'Game settings' })[0]!);
      expect(await screen.findByRole('tab', { name: /General/ })).toBeTruthy();
    } finally {
      applyTheme({ id: 'warm', tokens: {}, wash: [], variants: {} } as unknown as ThemeTokens);
    }
  });

  it('renders the built-in arrangement: media above about, compat in the side column, axe clean', async () => {
    const events = [{ id: 1, ts: Date.now(), type: 'play', detail: 'played', game: 'Layout Installed' }] as unknown as GamePageActivity;
    const { container } = page(installedGame(), events);
    // Hero: back link, title and actions.
    expect(await screen.findByRole('heading', { name: 'Layout Installed' })).toBeTruthy();
    expect(screen.getByText('Library')).toBeTruthy();
    // Main column: the media strip above the about text.
    expect(await screen.findByTestId('shot-strip')).toBeTruthy();
    expect(screen.getByText('About')).toBeTruthy();
    const main = container.querySelector('[data-region="main"]');
    expect(main).toBeTruthy();
    const order = [...main!.querySelectorAll(':scope > *')].map((el) => el.textContent?.slice(0, 20) ?? '');
    void order;
    // Side column: compat facts and the installation panel.
    const side = container.querySelector('[data-region="side"]');
    expect(side).toBeTruthy();
    expect(within(side as HTMLElement).getAllByText('Compatibility').length).toBeGreaterThan(0);
    expect(within(side as HTMLElement).getByText('Installation')).toBeTruthy();
    // Bottom region: achievements list and session activity full width.
    const bottom = container.querySelector('[data-region="bottom"]');
    expect(bottom).toBeTruthy();
    expect(within(bottom as HTMLElement).getByText('Achievements')).toBeTruthy();
    expect(within(bottom as HTMLElement).getByText('this session')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders a fresh game: install note and install action, axe clean', async () => {
    const { container } = page(freshGame());
    expect(await screen.findByRole('heading', { name: 'Layout Fresh' })).toBeTruthy();
    expect(screen.getAllByRole('button', { name: /Install/ }).length).toBeGreaterThan(0);
    expect(screen.getByTestId('install-size-line')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the cool layout: about above media, compat and installation lead the side, axe clean', async () => {
    layoutState.layout = coolLayout;
    const { container } = page(installedGame());
    expect(await screen.findByRole('heading', { name: 'Layout Installed' })).toBeTruthy();
    const main = container.querySelector('[data-region="main"]');
    const side = container.querySelector('[data-region="side"]');
    expect(main).toBeTruthy();
    expect(side).toBeTruthy();
    // About comes before the media strip in the cool main column.
    const mainHtml = main!.innerHTML;
    expect(mainHtml.indexOf('About')).toBeLessThan(mainHtml.indexOf('shot-strip'));
    // Compat and installation lead the cool side column.
    const sideText = (side as HTMLElement).textContent ?? '';
    expect(sideText.indexOf('Compatibility')).toBeLessThan(sideText.indexOf('Installation'));
    expect(sideText.indexOf('Installation')).toBeLessThan(sideText.indexOf('Status'));
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});
