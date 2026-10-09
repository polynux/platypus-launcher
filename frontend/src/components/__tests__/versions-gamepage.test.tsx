import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const getGameVersions = vi.hoisted(() => vi.fn());
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: () => Effect.succeed({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null } as never),
    getGameVersions: (g: unknown) => Effect.promise(() => getGameVersions(g)),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

import { GamePage } from '../GamePage';
import type { Game, StorePlugin } from '../../types';

const base = { runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onOpenSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {} };
const plugin = (caps: string[]): StorePlugin => ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: ['install', 'versions'], install_folder: '/g' }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '', install_dir: '/g/tiny', executable: '',
  is_installed: false, installable: true, features: ['install', 'versions'],
  settings: { runner_id: 'auto', env_vars: {} },
  ...o,
}) as Game;

describe('Version controls on the game page', () => {
  it('offers a version choice next to Install when the plugin offers versions', () => {
    render(<GamePage game={game()} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.getByRole('button', { name: /Choose version/ })).toBeTruthy();
    expect(screen.getByRole('button', { name: /^Install$/ })).toBeTruthy();
  });

  it('shows nothing without the capability', () => {
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    expect(screen.queryByRole('button', { name: /Choose version/ })).toBeNull();
  });

  it('shows nothing for a game whose features leave versions out', () => {
    render(<GamePage game={game({ features: ['install'] })} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.queryByRole('button', { name: /Choose version/ })).toBeNull();
  });

  it('shows nothing for Steam and sideloaded games', () => {
    render(<GamePage game={game({ source: 'steam', features: null })} {...base} />);
    expect(screen.queryByRole('button', { name: /Choose version/ })).toBeNull();
  });

  it('opens the dialog from the installed game’s Installation panel, with the pin shown', async () => {
    getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
    render(<GamePage game={game({ is_installed: true, version: '2.0.0', pinned: { kind: 'build', id: 'b200' } })} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.getByText('pinned')).toBeTruthy();
    expect(screen.getByText('Pinned to')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /^Version$/ }));
    expect(await screen.findByRole('dialog', { name: 'Version of Echo Tiny' })).toBeTruthy();
    expect(getGameVersions).toHaveBeenCalledOnce();
  });
});
