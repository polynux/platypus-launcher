import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render as rtlRender, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import { LibraryView } from '../LibraryView';
import type { Game, StorePlugin } from '../../types';
import { TooltipProvider } from '../ds/tooltip';

const renderLib = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const STEAM_LOGO = '/platypus-art/plugin/steam?v=1';
const ECHO_LOGO = '/platypus-art/plugin/echo?v=2';

function library(games: Game[], sourceLogos?: Record<string, string>) {
  return renderLib(
    <LibraryView
      games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode="table" onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} sourceLogos={sourceLogos}
    />,
  );
}

describe('Library store selector logos', () => {
  const games = [mk('a'), mk('b', { source: 'echo', store: 'echo' as Game['store'], id: 'b' })];

  it('shows each store logo next to its label, none on all', () => {
    const { getByRole } = library(games, { steam: STEAM_LOGO, echo: ECHO_LOGO });
    const steam = getByRole('radio', { name: 'steam' });
    const echo = getByRole('radio', { name: 'echo' });
    const all = getByRole('radio', { name: 'all' });
    expect(steam.querySelector('img')).toHaveAttribute('src', STEAM_LOGO);
    expect(echo.querySelector('img')).toHaveAttribute('src', ECHO_LOGO);
    expect(all.querySelector('img')).toBeNull();
  });

  it('falls back to a monogram tile when a store has no logo', () => {
    const { getByRole } = library(games, {});
    const steam = getByRole('radio', { name: 'steam' });
    expect(steam.querySelector('img')).toBeNull();
    // Monogram tile with the store's initial (aria-hidden, so the radio name stays "steam").
    expect(within(steam).getByText('S')).toBeInTheDocument();
    const echo = getByRole('radio', { name: 'echo' });
    expect(echo.querySelector('img')).toBeNull();
    expect(within(echo).getByText('E')).toBeInTheDocument();
  });

  it('keeps the logo at ~16px so the chips stay the same height', () => {
    const { getByRole } = library(games, { steam: STEAM_LOGO, echo: ECHO_LOGO });
    for (const name of ['steam', 'echo'] as const) {
      const img = getByRole('radio', { name }).querySelector('img')!;
      expect(img.className).toMatch(/h-4/);
      expect(img.className).toMatch(/w-4/);
    }
  });
});

// ---- Game page store logos ----

const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getInstallSize: vi.fn(),
  getGameOptions: vi.fn(),
  getInstallLocations: vi.fn(),
  checkInstallFolder: vi.fn(),
  pickInstallFolder: vi.fn(),
  installGameTo: vi.fn(),
  getGameVersions: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const plugin = (): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: [], declared: [], install_folder: '/games/echo' }) as StorePlugin;
const ggame = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/tiny', executable: '', is_installed: true, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' }, ...o,
}) as Game;

function page(g: Game, sourceLogos?: Record<string, string>) {
  return rtlRender(
    <GamePage
      game={g} plugin={plugin()} runners={[]} activity={[]} backLabel="Library" onBack={() => {}} onPlay={() => {}} onStop={() => {}} onForceStop={() => {}}
      launching={false} tweaks={[]} onChangeSettings={() => {}} onSaveSettings={async () => {}} onJob={vi.fn()} onJobAction={() => {}} onAllowFolder={() => {}}
      onNavigate={() => {}} jobs={[]} onNotify={() => {}} sourceLogos={sourceLogos}
    />,
  );
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getInstallSize.mockResolvedValue(0);
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getInstallLocations.mockResolvedValue([]);
});

describe('Game page store logos', () => {
  it('shows the store logo in the hero badge and the Store row', () => {
    page(ggame(), { echo: ECHO_LOGO });
    const badge = screen.getByTestId('store-badge');
    expect(badge).toHaveTextContent('Echo');
    expect(badge.querySelector('img')).toHaveAttribute('src', ECHO_LOGO);
    const row = screen.getByTestId('store-row');
    expect(row).toHaveTextContent(/Echo · Echo/);
    expect(row.querySelector('img')).toHaveAttribute('src', ECHO_LOGO);
  });

  it('prefers the source plugin logo over the store logo', () => {
    const g = ggame({ source: 'echo-b', store: 'echo' as Game['store'] });
    page(g, { 'echo-b': '/logo-b', echo: ECHO_LOGO });
    expect(screen.getByTestId('store-badge').querySelector('img')).toHaveAttribute('src', '/logo-b');
    expect(screen.getByTestId('store-row').querySelector('img')).toHaveAttribute('src', '/logo-b');
  });

  it('falls back to the store logo when the source plugin has none', () => {
    const g = ggame({ source: 'echo-b', store: 'echo' as Game['store'] });
    page(g, { echo: ECHO_LOGO });
    expect(screen.getByTestId('store-badge').querySelector('img')).toHaveAttribute('src', ECHO_LOGO);
  });

  it('shows a monogram tile in both spots when there is no logo', () => {
    page(ggame(), {});
    const badge = screen.getByTestId('store-badge');
    expect(badge.querySelector('img')).toBeNull();
    expect(within(badge).getByText('E')).toBeInTheDocument();
    const row = screen.getByTestId('store-row');
    expect(row.querySelector('img')).toBeNull();
    expect(within(row).getByText('E')).toBeInTheDocument();
  });
});
