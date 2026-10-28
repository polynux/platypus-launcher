// Issue #156: page presentations are independent axes, not the shell.
// A theme picks one page presentation without the rest. Matchers are unique
// to each presentation (not shared headings), each test asserts the other
// presentation is absent, and both directions are covered. The banner case
// is finding 1: mixed shell/friends pairs show the migration banner exactly
// once.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import { Dashboard } from '../Dashboard';
import { FixesView } from '../FixesView';
import '../../lib/i18n';

const variants = vi.hoisted(() => ({ map: {} as Record<string, string> }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => variants.map[axis] ?? 'page',
    useThemeLayout: () => null,
  };
});

const api = vi.hoisted(() => ({
  values: {} as Record<string, unknown>,
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
        return () => Effect.succeed(undefined as never);
      },
    },
  ),
  onJobs: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
  onPlaytime: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
}));

afterEach(() => {
  cleanup();
  variants.map = {};
  api.values = {};
});

const dashboardGame = {
  id: 'g',
  title: 'G',
  store: 'steam',
  source: 'steam',
  is_installed: true,
  settings: {},
} as never;

describe('page-presentation axes', () => {
  it('Atlas dashboard + classic fixes: home body without classic waiting list', () => {
    variants.map['dashboard.presentation'] = 'home';
    variants.map['fixes.presentation'] = 'rows';
    render(
      <Dashboard
        games={[dashboardGame]}
        runners={[]}
        activity={[]}
        dismissedHero={new Set()}
        onDismissHero={() => {}}
        onOpenGame={() => {}}
        onNavigate={() => {}}
      />,
    );
    // Unique to AtlasHome; classic DashboardWaiting never renders it.
    expect(screen.getByText('From your backlog')).toBeInTheDocument();
    expect(screen.queryByText('Waiting on you')).toBeNull();
    cleanup();
    render(<FixesView games={[]} onOpenGame={() => {}} />);
    // Unique to the classic rows; the Atlas cards page never renders it.
    expect(screen.getByText('no fix data yet')).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Needs attention' })).toBeNull();
  });

  it('classic dashboard + Atlas fixes: rows without the home suggestion', () => {
    variants.map['dashboard.presentation'] = 'blocks';
    variants.map['fixes.presentation'] = 'cards';
    render(
      <Dashboard
        games={[dashboardGame]}
        runners={[]}
        activity={[]}
        dismissedHero={new Set()}
        onDismissHero={() => {}}
        onOpenGame={() => {}}
        onNavigate={() => {}}
      />,
    );
    // Unique to the classic blocks; AtlasHome never renders it.
    expect(screen.getByText('Waiting on you')).toBeInTheDocument();
    expect(screen.queryByText('From your backlog')).toBeNull();
    cleanup();
    render(<FixesView games={[]} onOpenGame={() => {}} />);
    // Unique to the Atlas cards page; the classic rows never render it.
    expect(screen.getByRole('heading', { name: 'Needs attention' })).toBeInTheDocument();
    expect(screen.queryByText('no fix data yet')).toBeNull();
  });

  it.each([
    ['sidebar', 'panel'],
    ['sourcebar', 'modal'],
  ])('shows the migration banner exactly once for %s + friends=%s', async (shell, friends) => {
    variants.map['shell.navigation'] = shell;
    variants.map['friends.presentation'] = friends;
    variants.map['library.presentation'] = 'grid';
    variants.map['settings.presentation'] = 'page';
    const { App } = await import('../../App');
    api.values = {
      getLibrary: { games: [], warnings: [] },
      getProtonVersions: [],
      listPlugins: [],
      getGameSettings: {},
      getSessions: [],
      getTweaks: [],
      listCollections: [],
      migrationNotice: { moved: true, copied: false, set_aside: [], warnings: [] },
      socialViews: [],
      listJobs: [],
      getLauncherSettings: {},
      listThemes: [],
      listActivity: [],
    };
    render(<App />);
    await waitFor(() => {
      expect(screen.getAllByRole('status', { name: /hydra to platypus/i })).toHaveLength(1);
    });
  });
});
