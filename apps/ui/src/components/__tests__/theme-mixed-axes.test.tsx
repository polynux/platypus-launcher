// Issue #168: page and dialog decisions follow their own axes, not the shell.
// A theme that mixes a non-sourcebar shell with Atlas pages gets Atlas
// dialogs and game-page details. Each case renders a mixed combination both
// ways: the Atlas axis wins whatever the shell is.
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, act } from '@testing-library/react';
import { Effect } from 'effect';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { Game, StorePlugin } from '../../types';

const theme = vi.hoisted(() => ({
  shell: 'sidebar',
  structure: 'sections',
  library: 'grid',
  store: 'embedded',
}));

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return theme.shell;
      if (axis === 'game.structure') return theme.structure;
      if (axis === 'library.presentation') return theme.library;
      if (axis === 'store.presentation') return theme.store;
      if (axis === 'game-settings.presentation') return 'dialog';
      if (axis === 'game-hero.chips') return 'solid';
      if (axis === 'game-hero.logo') return 'title';
      if (axis === 'label.style') return 'mono';
      return 'page';
    },
    useThemeLayout: () => null,
  };
});

const api = vi.hoisted(() => ({
  values: {} as Record<string, unknown>,
  getInstallSize: vi.fn(),
  getInstallLocations: vi.fn(),
  getGameOptions: vi.fn(),
  sessionHandler: null as null | ((e: unknown) => void),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(
    {},
    {
      get: (_t, key: string) => {
        if (key in api.values) {
          const v = api.values[key];
          return typeof v === 'function' ? v : () => Effect.succeed(v);
        }
        if (key === 'getInstallSize') return (...a: unknown[]) => Effect.tryPromise({ try: () => api.getInstallSize(...a), catch: (e) => (e instanceof Error ? e : new Error(String(e))) });
        if (key === 'getInstallLocations') return (...a: unknown[]) => Effect.tryPromise({ try: () => api.getInstallLocations(...a), catch: (e) => (e instanceof Error ? e : new Error(String(e))) });
        if (key === 'getGameOptions') return (...a: unknown[]) => Effect.tryPromise({ try: () => api.getGameOptions(...a), catch: (e) => (e instanceof Error ? e : new Error(String(e))) });
        return (...a: unknown[]) => Effect.tryPromise({ try: () => { throw new Error(`unmocked ${key}`); }, catch: (e) => (e instanceof Error ? e : new Error(String(e))) });
      },
    },
  ),
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: (cb: (e: unknown) => void) => { api.sessionHandler = cb; return () => {}; },
  onSocial: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocialViews: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
  onPlaytime: () => () => {},
}));

vi.mock('../../lib/compat', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/compat')>();
  return { ...mod, gameCompat: () => Promise.resolve(null), setGameCompat: () => {} };
});

import { InstallDialog } from '../InstallDialog';
import { InstallOptionsDialog } from '../InstallOptionsDialog';
import { RedeemDialog } from '../RedeemDialog';
import { AddGameModal } from '../AddGameModal';
import { GamePage } from '../GamePage';
import { LibraryListDetail } from '../LibraryListDetail';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

afterEach(() => {
  cleanup();
  theme.shell = 'sidebar';
  theme.structure = 'sections';
  theme.library = 'grid';
  theme.store = 'embedded';
  api.values = {};
  api.sessionHandler = null;
});
beforeEach(() => {
  api.getInstallSize.mockReset();
  api.getInstallLocations.mockReset();
  api.getGameOptions.mockReset();
  api.getInstallSize.mockResolvedValue(30_000_000_000);
  api.getInstallLocations.mockResolvedValue([{ folder: '/games/echo', kind: 'default', free_bytes: 100_000_000_000, exists: true }]);
});

const noop = () => {};
const plugin = (caps: string[] = ['install']): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game =>
  ({
    id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny',
    cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: false,
    installable: true, features: [], settings: { runner_id: 'auto', env_vars: {} }, ...o,
  }) as Game;

describe('mixed themes: dialogs follow their flow (issue #168)', () => {
  it('install dialog follows the game structure, not the shell', async () => {
    theme.shell = 'sidebar';
    theme.structure = 'tabs';
    render(<InstallDialog game={game()} plugin={plugin()} onClose={noop} onChanged={noop} onNotify={noop} />);
    // Tabs review the size and the place: the foot names the folder.
    expect(await screen.findByRole('button', { name: 'Install to Choose folder…' })).toBeInTheDocument();
    cleanup();

    theme.shell = 'sourcebar';
    theme.structure = 'sections';
    render(<InstallDialog game={game()} plugin={plugin()} onClose={noop} onChanged={noop} onNotify={noop} />);
    // Sections install at once: the plain Install action, no folder naming.
    expect(await screen.findByRole('button', { name: /^Install$/ })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Install to Choose folder…' })).toBeNull();
  });

  it('the location rows follow the game structure too', async () => {
    theme.shell = 'sidebar';
    theme.structure = 'tabs';
    render(<InstallDialog game={game()} plugin={plugin()} onClose={noop} onChanged={noop} onNotify={noop} />);
    const group = await screen.findByRole('radiogroup', { name: 'Install location' });
    const first = group.querySelector('[role=radio]');
    expect(first?.className).toMatch(/rounded-\[10px\]/);
    cleanup();

    theme.shell = 'sourcebar';
    theme.structure = 'sections';
    render(<InstallDialog game={game()} plugin={plugin()} onClose={noop} onChanged={noop} onNotify={noop} />);
    const classic = await screen.findByRole('radiogroup', { name: 'Install location' });
    expect(classic.querySelector('[role=radio]')?.className).toMatch(/rounded-xl/);
  });

  it('install options follow the game structure, not the shell', async () => {
    api.getGameOptions.mockResolvedValue({
      groups: [{ id: 'language', label: 'Language', kind: 'select', default: 'en', values: [{ id: 'en', label: 'English', owned: true }] }],
      chosen: { language: 'en' },
    });
    theme.shell = 'sidebar';
    theme.structure = 'tabs';
    render(<InstallOptionsDialog game={game()} plugin={plugin()} mode="install" onClose={noop} onChanged={noop} onNotify={noop} />);
    const totals = await screen.findByTestId('total-size');
    expect(totals.closest('div.rounded-2xl')?.className).toMatch(/font-sans/);
    cleanup();

    theme.shell = 'sourcebar';
    theme.structure = 'sections';
    render(<InstallOptionsDialog game={game()} plugin={plugin()} mode="install" onClose={noop} onChanged={noop} onNotify={noop} />);
    const classic = await screen.findByTestId('total-size');
    expect(classic.closest('div.rounded-2xl')?.className).toMatch(/font-mono/);
  });

  it('add-game glass follows the library presentation, not the shell', () => {
    theme.shell = 'sidebar';
    theme.library = 'list-detail';
    render(<AddGameModal onClose={noop} onAdd={vi.fn()} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(document.querySelector('.bg-glass')).not.toBeNull();
    cleanup();

    theme.shell = 'sourcebar';
    theme.library = 'grid';
    render(<AddGameModal onClose={noop} onAdd={vi.fn()} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(document.querySelector('.bg-glass')).toBeNull();
  });

  it('redeem glass follows the store presentation, not the shell', () => {
    theme.shell = 'sidebar';
    theme.store = 'storefronts';
    render(<RedeemDialog stores={[plugin()]} onClose={noop} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(document.querySelector('.bg-glass')).not.toBeNull();
    cleanup();

    theme.shell = 'sourcebar';
    theme.store = 'embedded';
    render(<RedeemDialog stores={[plugin()]} onClose={noop} />);
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(document.querySelector('.bg-glass')).toBeNull();
  });
});

describe('mixed themes: the game install flow follows the structure (issue #168)', () => {
  const base = {
    runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
    launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJobAction() {},
    onAllowFolder() {}, onNavigate() {}, jobs: [], onNotify() {},
  };

  it('tabs review the location in the dialog even with a classic shell', async () => {
    theme.shell = 'sidebar';
    theme.structure = 'tabs';
    api.getInstallSize.mockResolvedValue(0);
    const onJob = vi.fn();
    render(<GamePage game={game()} plugin={plugin()} {...base} onJob={onJob} collections={[]} onManageCollections={noop} />);
    fireEvent.click(await screen.findByRole('button', { name: /^Install$/ }));
    // The dialog opens instead of queueing at once.
    expect(await screen.findByRole('dialog')).toBeInTheDocument();
    expect(onJob).not.toHaveBeenCalled();
  });

  it('sections install at once even with the Atlas shell', async () => {
    theme.shell = 'sourcebar';
    theme.structure = 'sections';
    api.getInstallSize.mockResolvedValue(0);
    const onJob = vi.fn();
    render(<GamePage game={game()} plugin={plugin()} {...base} onJob={onJob} collections={[]} onManageCollections={noop} />);
    fireEvent.click(await screen.findByRole('button', { name: /^Install$/ }));
    expect(onJob).toHaveBeenCalledWith('install');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('the failed install shows in the note whatever the shell is', async () => {
    const failed = {
      id: 'j1', kind: 'install', source: 'echo', game_id: 'p.echo.tiny', store_game_id: 'tiny', title: 'Echo Tiny',
      state: 'failed', bytes: 0, total: 0, speed: 0, percent: 0, attempts: 1, created_at: 0, updated_at: 0, error: '',
    };
    for (const shell of ['sourcebar', 'sidebar']) {
      theme.shell = shell;
      theme.structure = 'sections';
      render(<GamePage game={game()} plugin={plugin()} {...base} onJob={noop} job={failed as never} collections={[]} onManageCollections={noop} />);
      expect(await screen.findByText(/didn't finish/)).toBeInTheDocument();
      cleanup();
    }
  });
});

describe('mixed themes: the list-detail keys follow the library (issue #168)', () => {
  const mk = (id: string, o: Partial<Game> = {}): Game =>
    ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, last_played_at: 1700000000, ...o }) as Game;
  const games = () => [mk('alpha'), mk('beta'), mk('gamma')];

  it('j moves the stage with a classic shell', () => {
    theme.shell = 'sidebar';
    theme.library = 'list-detail';
    render(
      <LibraryListDetail games={games()} ownership="owned" source="all" onPlay={noop} renderStage={(g) => <div data-testid="stage">{g.title}</div>} />,
    );
    expect(screen.getByTestId('stage')).toHaveTextContent('alpha');
    fireEvent.keyDown(document.body, { key: 'j' });
    expect(screen.getByTestId('stage')).toHaveTextContent('beta');
  });

  it('the keys stay off without the list-detail presentation', () => {
    theme.shell = 'sourcebar';
    theme.library = 'grid';
    render(
      <LibraryListDetail games={games()} ownership="owned" source="all" onPlay={noop} renderStage={(g) => <div data-testid="stage">{g.title}</div>} />,
    );
    expect(screen.getByTestId('stage')).toHaveTextContent('alpha');
    fireEvent.keyDown(document.body, { key: 'j' });
    expect(screen.getByTestId('stage')).toHaveTextContent('alpha');
  });
});

describe('mixed themes: the now-running toast follows the library (issue #168)', () => {
  const doom = (): Game =>
    ({ id: 'g1', source: 'steam', store: 'steam', store_app_id: 'g1', title: 'Doom', is_installed: true, settings: {} }) as Game;

  async function openApp() {
    const { App } = await import('../../App');
    api.values = {
      getLibrary: { games: [doom()], warnings: [] },
      getProtonVersions: [],
      listPlugins: [],
      getGameSettings: {},
      getSessions: [],
      listActivity: [],
      getLauncherSettings: {},
      listCollections: [],
      migrationNotice: null,
      socialViews: [],
      listJobs: [],
      getTweaks: [],
      listThemes: [],
    };
    render(<App />);
    // The library settled: the game title is on screen.
    expect(await screen.findAllByText('Doom')).not.toHaveLength(0);
    const emit = (e: unknown) => api.sessionHandler?.(e);
    return { emit };
  }

  it('stays quiet with the list-detail stage even under a classic shell', async () => {
    theme.shell = 'sidebar';
    theme.library = 'list-detail';
    const { emit } = await openApp();
    await act(async () => { emit({ game_id: 'g1', state: 'running', pid: 4242 }); });
    // The session landed (the shell names it playing) but no toast follows.
    expect(await screen.findByText('playing')).toBeInTheDocument();
    expect(screen.queryByText('Doom is running')).toBeNull();
  });

  it('still confirms with the grid library', async () => {
    theme.shell = 'sidebar';
    theme.library = 'grid';
    const { emit } = await openApp();
    await act(async () => { emit({ game_id: 'g1', state: 'running', pid: 4242 }); });
    expect(await screen.findByText('Doom is running')).toBeInTheDocument();
  });
});
