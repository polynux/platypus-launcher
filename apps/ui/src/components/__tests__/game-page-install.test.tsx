import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, StorePlugin } from '../../types';

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
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const GB = 1e9;
const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '', executable: '', is_installed: false, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' }, ...o,
}) as Game;

function page(p: StorePlugin, g: Game = game()) {
  render(
    <GamePage
      game={g} plugin={p} runners={[]} activity={[]} backLabel="Library" onBack={() => {}} onPlay={() => {}} onStop={() => {}} onForceStop={() => {}}
      launching={false} tweaks={[]} onChangeSettings={() => {}} onSaveSettings={() => {}} onJob={vi.fn()} onJobAction={() => {}} onAllowFolder={() => {}}
      onNavigate={() => {}} jobs={[]} onNotify={() => {}}
    />,
  );
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getInstallSize.mockResolvedValue(30 * GB);
  api.getGameOptions.mockResolvedValue({ groups: [], chosen: {} });
  api.getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
  api.getInstallLocations.mockResolvedValue([{ folder: '/games/echo', kind: 'default', free_bytes: 100 * GB, exists: true }]);
});

describe('the install size on a game that is not installed', () => {
  it('says it is checking, then gives the size', async () => {
    let done!: (n: number) => void;
    api.getInstallSize.mockReturnValue(new Promise<number>((r) => { done = r; }));
    page(plugin(['install']));
    expect(screen.getByTestId('install-size-line')).toHaveTextContent(/checking/);
    done(30 * GB);
    await waitFor(() => expect(screen.getByTestId('install-size-line')).toHaveTextContent('Install size 30.0 GB'));
  });

  it('says "Size unknown" when the store cannot tell', async () => {
    api.getInstallSize.mockResolvedValue(0);
    page(plugin(['install']));
    await waitFor(() => expect(screen.getByTestId('install-size-line')).toHaveTextContent('Size unknown'));
  });

  it('says "Size unknown" when asking fails', async () => {
    api.getInstallSize.mockRejectedValue(new Error('offline'));
    page(plugin(['install']));
    await waitFor(() => expect(screen.getByTestId('install-size-line')).toHaveTextContent('Size unknown'));
  });

  it('adds the chosen add-ons when the store has options', async () => {
    api.getGameOptions.mockResolvedValue({
      groups: [{ id: 'dlc', kind: 'multi', dlc: true, values: [{ id: 'hd', label: 'HD pack', sizeBytes: 5 * GB }] }],
      chosen: { dlc: ['hd'] },
    });
    page(plugin(['install', 'install-options']));
    await waitFor(() => expect(screen.getByTestId('install-size-line')).toHaveTextContent('Install size 35.0 GB'));
    expect(screen.getByTestId('install-size-line')).toHaveTextContent('includes 5.0 GB of add-ons');
  });

  it('is not shown for an installed game', () => {
    page(plugin(['install']), game({ is_installed: true, install_dir: '/games/echo/tiny' }));
    expect(screen.queryByTestId('install-size-line')).toBeNull();
    expect(api.getInstallSize).not.toHaveBeenCalled();
  });
});

describe('Install to… on the page', () => {
  it('opens the install dialog for a store without options', async () => {
    page(plugin(['install']));
    fireEvent.click(screen.getByRole('button', { name: /Install to…/ }));
    expect(await screen.findByRole('dialog', { name: /Install Echo Tiny/ })).toBeInTheDocument();
    expect(await screen.findByRole('radiogroup', { name: 'Install location' })).toBeInTheDocument();
  });

  it('leaves it to the options dialog when the store has options', async () => {
    page(plugin(['install', 'install-options']));
    expect(screen.queryByRole('button', { name: /Install to…/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Install' }));
    expect(await screen.findByRole('radiogroup', { name: 'Install location' })).toBeInTheDocument();
  });
});
