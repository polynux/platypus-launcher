// i18n: converted
// Game page tabs variant (issue #127): the hero with the logo, the action
// strip, sticky tabs and the store page, with real data. Sections keep
// today's path (see gameLayout.test.tsx).
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';

const themeState = vi.hoisted(() => ({ structure: 'tabs', logo: 'logo' }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'game.structure') return themeState.structure;
      if (axis === 'game-hero.logo') return themeState.logo;
      if (axis === 'game-settings.presentation') return 'dialog';
      if (axis === 'game-hero.chips') return 'solid';
      return 'page';
    },
    useThemeLayout: () => null,
  };
});

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: () => Effect.succeed(details as never),
    getCrashHints: () => Effect.succeed(null),
    getInstallSize: () => Effect.succeed({ state: 'known', total: 1500000000, added: 0 } as never),
    getGameOptions: () => Effect.succeed({ groups: [], chosen: {} } as never),
    getGameAchievements: () => Effect.fail(new Error('no achievements')),
    getCloudInfo: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: null, running: false } as never),
    checkCloudSaves: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: null, running: false } as never),
    listSaveBackups: () => Effect.succeed([] as never),
    getThemeLayout: () => Effect.succeed(null as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
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
import { TooltipProvider } from '../ds/tooltip';
import type { Game, StorePlugin } from '../../types';

expect.extend(matchers);
afterEach(() => { cleanup(); themeState.structure = 'tabs'; themeState.logo = 'logo'; });
beforeEach(() => invalidateGameDetailsCache());

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const details = {
  summary: 'A short summary',
  screenshots: ['s1.png', 's2.png', 's3.png'],
  genres: ['Adventure'],
  developers: ['Echo Software'],
  publishers: ['Echo Software'],
  release_date: '2024-01-01',
  hero_url: 'hero.png',
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
  runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_mangohud: false,
  use_mangohud_preset: '', use_gamemode: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0,
  gamescope_r: 0, gamescope_args: '', eac_runner: false, battleye_runtime: false, working_dir: '',
} as unknown as Game['settings'];

const plugin = (): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['install'], declared: ['install'], install_folder: '/games' }) as StorePlugin;

let n = 0;
function installedGame(): Game {
  n += 1;
  return {
    id: `tabs-installed-${n}`, source: 'echo', store: 'echo', store_app_id: `${n}`, title: 'Tabs Installed',
    cover_url: 'cover.png', banner_url: '', hero_url: '', logo_url: 'logo.png',
    install_dir: '/games/echo-tiny', executable: 'tiny', is_installed: true,
    installable: true, features: [], settings, playtime_minutes: 120, last_played_at: 1700000000,
  } as unknown as Game;
}

const baseProps = {
  runners: [{ id: 'proton', name: 'Proton 9', path: '/proton', is_system: true, is_custom: false, is_default: true }],
  backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
  launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {},
  onAllowFolder() {}, onNavigate() {}, jobs: [], onNotify() {},
};

function page(game: Game) {
  return render(
    <GamePage
      game={game}
      plugin={plugin()}
      runners={baseProps.runners as never}
      activity={[]}
      {...baseProps}
      onManageCollections={() => {}}
      collections={[]}
    />,
  );
}

describe('game page tabs variant', () => {
  it('renders the hero logo, action strip with Tune, and sticky tabs, axe clean', async () => {
    const user = userEvent.setup();
    const { container } = page(installedGame());
    // Hero falls back to the logo art with the title as its name.
    expect(await screen.findByAltText('Tabs Installed')).toBeTruthy();
    // Action strip: Play, Tune with chevron, the store label, the save state.
    expect(screen.getByRole('button', { name: /^Play/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Tune/ })).toBeInTheDocument();
    expect(screen.getByText('Echo')).toBeInTheDocument();
    // Sticky tabs with counts; achievements only when present.
    const tablist = screen.getByRole('tablist', { name: 'Game page' });
    expect(within(tablist).getByRole('tab', { name: /Store page/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Achievements.*1\/2/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Activity/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Saves/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Compatibility/ })).toBeInTheDocument();
    // Store page: the description, pictures, the 2x2 grid, the summary, details.
    expect(screen.getByText('A long description.')).toBeInTheDocument();
    expect(screen.getByText('Pictures')).toBeInTheDocument();
    expect(screen.getByText('Played')).toBeInTheDocument();
    expect(screen.getByText('Details')).toBeInTheDocument();
    // The achievements summary opens the Achievements tab.
    await user.click(screen.getByRole('button', { name: /See all 2 achievements/ }));
    expect(within(tablist).getByRole('tab', { name: /Achievements/ })).toHaveAttribute('aria-selected', 'true');
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('hides the back link on the list-detail stage but keeps it outside', async () => {
    const props = (hideBack?: boolean) => (
      <GamePage
        game={installedGame()}
        plugin={plugin()}
        runners={baseProps.runners as never}
        activity={[]}
        {...baseProps}
        hideBack={hideBack}
        onManageCollections={() => {}}
        collections={[]}
      />
    );
    const first = render(props(true));
    expect(await screen.findByRole('button', { name: /^Play/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Library' })).toBeNull();
    first.unmount();
    cleanup();
    render(props());
    expect(await screen.findByRole('button', { name: 'Library' })).toBeInTheDocument();
  });

  it('keeps the required blocks: back, actions with Update, and the outside alert', async () => {
    const g = installedGame();
    (g as Game & { outside_folder?: boolean }).outside_folder = true;
    (g as Game & { update_available?: boolean }).update_available = true;
    page(g);
    expect(await screen.findByRole('button', { name: 'Library' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('sections stay untouched when the theme keeps them', async () => {
    themeState.structure = 'sections';
    const { container } = page(installedGame());
    expect(await screen.findByRole('heading', { name: 'Tabs Installed' })).toBeTruthy();
    // Sections have no tablist; tabs do.
    expect(screen.queryByRole('tablist', { name: 'Game page' })).toBeNull();
    expect(container.querySelector('[data-region="main"]')).toBeTruthy();
  });
});
