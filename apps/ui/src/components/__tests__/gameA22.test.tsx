// Issue #137 (/22 game page + settings drawer): the strip renders per game
// state (installed, not installed, running, failed, conflict, multi-copy),
// the tabs carry counts, the hero back overlays the art, the launch notes row
// shows under the strip, and the drawer categories map to real settings.
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';

const cloudState = vi.hoisted(() => ({ status: null as null | { state: string; conflicts: unknown[]; checkedAt: number } }));

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'dashboard.presentation') return 'home';
      if (axis === 'fixes.presentation') return 'cards';
      if (axis === 'activity.presentation') return 'days';
      if (axis === 'runners.presentation') return 'table';
      if (axis === 'downloads.presentation') return 'cards';
      if (axis === 'storage.presentation') return 'cards';
      if (axis === 'plugins.presentation') return 'directory';
      if (axis === 'friends.presentation') return 'panel';
      if (axis === 'store.presentation') return 'storefronts';
      if (axis === 'settings-row.style') return 'srow';
      if (axis === 'toast.style') return 'pill';
      if (axis === 'game.structure') return 'tabs';
      if (axis === 'game-hero.logo') return 'title';
      if (axis === 'game-settings.presentation') return 'dialog';
      if (axis === 'game-hero.chips') return 'solid';
      if (axis === 'label.style') return 'plain';
      return 'page';
    },
    useThemeLayout: () => null,
  };
});

const details = {
  summary: 'A short summary',
  screenshots: ['s1.png', 's2.png'],
  genres: ['Adventure'],
  developers: ['Echo Software'],
  publishers: ['Echo Software'],
  release_date: '2024-01-01',
  hero_url: '',
  achievements: null,
  description_html: '<p>A long description.</p>',
  website: 'https://example.com/echo-tiny',
  store_url: 'https://example.com/store/echo-tiny',
};

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: () => Effect.succeed(details as never),
    getCrashHints: () => Effect.succeed(null),
    getInstallSize: () => Effect.succeed({ state: 'known', total: 1500000000, added: 0 } as never),
    getGameOptions: () => Effect.succeed({ groups: [], chosen: {} } as never),
    getGameAchievements: () => Effect.fail(new Error('no achievements')),
    getCloudInfo: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: cloudState.status, running: false } as never),
    checkCloudSaves: () => Effect.succeed({ mode: 'auto', plugin_mode: 'auto', game_mode: '', status: cloudState.status, running: false } as never),
    listSaveBackups: () => Effect.succeed([] as never),
    setGameStatus: () => Effect.succeed({ status: 'backlog', user_notes: '' } as never),
    setGameNotes: () => Effect.succeed(undefined as never),
    setPreferredCopy: () => Effect.succeed(undefined as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
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
import { atlasCategoryForTab, atlasGroups } from '../AtlasSettings';
import { TooltipProvider } from '../ds/tooltip';
import type { Game, StorePlugin } from '../../types';

afterEach(() => { cleanup(); cloudState.status = null; });
beforeEach(() => invalidateGameDetailsCache());

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const settings = {
  runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_mangohud: false,
  use_gamemode: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0,
  gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '',
} as unknown as Game['settings'];

const plugin = (): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['install'], declared: ['install'], install_folder: '/games' }) as StorePlugin;

let n = 0;
function game(o: Record<string, unknown> = {}): Game {
  n += 1;
  return {
    id: `a22-${n}`, source: 'echo', store: 'echo', store_app_id: `${n}`, title: `A22 Game ${n}`,
    cover_url: '', banner_url: '', hero_url: '', install_dir: '/games/echo', executable: 'game',
    is_installed: true, installable: true, features: [], settings,
    ...o,
  } as unknown as Game;
}

const base = {
  runners: [{ id: 'proton', name: 'Proton 9', path: '/proton', is_system: true, is_custom: false, is_default: true }],
  backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
  launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJob() {}, onJobAction() {},
  onAllowFolder() {}, onNavigate() {}, jobs: [], onNotify() {},
};

function page(g: Game, extra: Record<string, unknown> = {}) {
  return render(
    <GamePage
      game={g}
      plugin={plugin()}
      runners={base.runners as never}
      activity={[]}
      {...base}
      collections={[]}
      onManageCollections={() => {}}
      {...extra}
    />,
  );
}

describe('/22 game strip states (issue #137)', () => {
  it('installed: Play + Tune + single store chip + sync state', async () => {
    page(game());
    expect(await screen.findByRole('button', { name: /^Play/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Tune/ })).toBeInTheDocument();
    expect(screen.getByText('Echo')).toBeInTheDocument();
    expect(await screen.findByText('Saves not checked')).toBeInTheDocument();
  });

  it('not installed: Install with size, no Play', async () => {
    page(game({ is_installed: false, installable: true }));
    expect(await screen.findByRole('button', { name: /Install/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Play/ })).toBeNull();
  });

  it('running session: Stop with live elapsed, no Play', async () => {
    page(game(), { session: { game_id: 'x', state: 'running' }, sessionStartedAt: Date.now() - 65000 });
    expect(await screen.findByRole('button', { name: /Stop/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^Play/ })).toBeNull();
    // Live elapsed mm:ss or hh:mm:ss.
    expect(screen.getByText(/\d+:\d\d(:\d\d)?/)).toBeInTheDocument();
  });

  it('paused job: Paused strip button with resume, cancel one click away, no second Install (R4-11)', async () => {
    const user = userEvent.setup();
    const onJobAction = vi.fn();
    const g = game({ is_installed: false, installable: true });
    const job = {
      id: 'j-paused', kind: 'install', source: 'echo', game_id: g.id, store_game_id: '1', title: g.title,
      state: 'paused', bytes: 20950000, total: 41900000, speed: 0, percent: 50, attempts: 1, created_at: 0, updated_at: 0,
    };
    page(g, { job, onJobAction });
    const btn = await screen.findByRole('progressbar', { name: /Paused/ });
    expect(btn).toHaveTextContent('50%');
    expect(btn).toHaveAttribute('title', 'Resume');
    // The paused job stays offered: no duplicate "Install" queued from the page.
    expect(screen.queryByRole('button', { name: /^Install/ })).toBeNull();
    expect(screen.getByRole('button', { name: /Cancel install/ })).toBeInTheDocument();
    await user.click(btn);
    expect(onJobAction).toHaveBeenCalledWith('resume', 'j-paused');
  });

  it('failed job on an uninstalled game: Retry + last attempt', async () => {
    const g = game({ is_installed: false, installable: true });
    const job = {
      id: 'j1', kind: 'install', source: 'echo', game_id: g.id, store_game_id: '1', title: g.title,
      state: 'failed', bytes: 0, total: 0, speed: 0, percent: 0, attempts: 1, created_at: 0, updated_at: 0, error: '',
    };
    page(g, { job });
    expect(await screen.findByRole('button', { name: /Retry/ })).toBeInTheDocument();
    expect(await screen.findByText(/didn't finish/)).toBeInTheDocument();
  });

  it('cloud conflict: a button that opens the conflict dialog', async () => {
    const user = userEvent.setup();
    cloudState.status = { state: 'conflict', conflicts: [{ path: 'save1.dat', local: { mtime: 2, size: 10 }, remote: { mtime: 1, size: 10 } }], checkedAt: 0 };
    page(game());
    const btn = await screen.findByRole('button', { name: /Save conflict/ });
    await user.click(btn);
    expect(await screen.findByText('Saves changed on both sides')).toBeInTheDocument();
  });

  it('multi-copy: segmented chips + an icon compare button that opens the dialog', async () => {
    const user = userEvent.setup();
    const g = game({
      shared_by: { id: 'ana', name: 'Ana' },
      preferred_copy: 'ana',
      copies: [
        { owner_id: 'ana', owner_name: 'Ana', available: true },
        { owner_id: 'ben', owner_name: 'Ben', available: true },
      ],
      copy_compare: {
        baseline: 'ana',
        diffs: [{ owner_id: 'ben', only_here: 1, only_baseline: 0, shared: 3, soundtracks_only_here: 0, soundtracks_only_baseline: 0, soundtracks_shared: 0 }],
      },
    });
    page(g);
    expect(await screen.findByRole('radiogroup', { name: 'Launch from' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Compare copies' }));
    expect(await screen.findByRole('dialog', { name: /Compare copies of/ })).toBeInTheDocument();
  });

  it('update waiting: Update button in the strip and a launch note with Choose version', async () => {
    const g = game({ update_available: true });
    page(g);
    expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument();
    expect(await screen.findByText('An update is waiting.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Choose a version' })).toBeInTheDocument();
  });

  it('client launch: the launch notes name the store client', async () => {
    page(game({ client_launch: true }));
    expect(await screen.findByText(/Launches through Echo/)).toBeInTheDocument();
  });
});

describe('/22 hero, tabs and side (issue #137)', () => {
  it('the back button overlays the art, and hides on the list-detail stage', async () => {
    const first = page(game());
    const back = await screen.findByRole('button', { name: 'Library' });
    // Over the art: absolute positioning inside the hero frame.
    expect(back.className).toMatch(/absolute/);
    first.unmount();
    cleanup();
    page(game(), { hideBack: true });
    expect(await screen.findByRole('button', { name: /^Play/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Library' })).toBeNull();
  });

  it('tabs carry counts: achievements, activity, saves, compat', async () => {
    const g = game();
    render(
      <GamePage
        game={g}
        plugin={plugin()}
        runners={base.runners as never}
        activity={[{ id: 1, ts: Date.now(), type: 'session', game: g.id, detail: 'played' }]}
        {...base}
        collections={[]}
        onManageCollections={() => {}}
      />,
    );
    const tablist = await screen.findByRole('tablist', { name: 'Game page' });
    expect(within(tablist).getByRole('tab', { name: /Activity.*1/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Saves/ })).toBeInTheDocument();
    expect(within(tablist).getByRole('tab', { name: /Compatibility/ })).toBeInTheDocument();
  });

  it('store side: Pictures, You facts, Details headings all render', async () => {
    page(game());
    expect(await screen.findByText('Pictures')).toBeInTheDocument();
    // The You block carries no heading of its own (the lab's YouPanel): the
    // facts, Status row, notes and collections read as one block.
    expect(screen.getByText('Last played')).toBeInTheDocument();
    expect(screen.getByText('Details')).toBeInTheDocument();
  });
});

describe('/22 drawer category mapping (issue #137)', () => {
  it('routes every game-settings tab into the drawer, defaulting to compat', () => {
    expect(atlasCategoryForTab('versions')).toBe('versions');
    expect(atlasCategoryForTab('options')).toBe('options');
    expect(atlasCategoryForTab('plugin')).toBe('plugin');
    expect(atlasCategoryForTab('tweaks')).toBe('fixes');
    expect(atlasCategoryForTab('customization')).toBe('customization');
    expect(atlasCategoryForTab('maintenance')).toBe('maintenance');
    expect(atlasCategoryForTab('cloud')).toBe('saves');
    expect(atlasCategoryForTab(undefined)).toBe('compat');
    expect(atlasCategoryForTab(null)).toBe('compat');
  });

  it('groups Everyday, Advanced and Game in drawer order', () => {
    const groups = atlasGroups(['general', 'cloud', 'versions', 'options', 'plugin', 'tweaks', 'customization', 'maintenance'] as never);
    expect(groups.map((g) => g.level)).toEqual(['everyday', 'advanced', 'game']);
    expect(groups[2].cats).toEqual(['versions', 'options', 'plugin', 'customization', 'maintenance']);
  });
});
