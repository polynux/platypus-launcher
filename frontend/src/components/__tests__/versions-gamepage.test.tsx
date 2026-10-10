import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getGameVersions = vi.hoisted(() => vi.fn());
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: () => Effect.succeed({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null } as never),
    getGameVersions: (g: unknown) => Effect.promise(() => getGameVersions(g)),
    getInstallSize: () => Effect.succeed(0),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

import { GamePage } from '../GamePage';
import type { Game, StorePlugin } from '../../types';

const base = { runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {} };
const plugin = (caps: string[]): StorePlugin => ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: ['install', 'versions'], install_folder: '/g' }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '', install_dir: '/g/tiny', executable: '',
  is_installed: false, installable: true, features: ['install', 'versions'],
  settings: { runner_id: 'auto', env_vars: {} },
  ...o,
}) as Game;

describe('Version controls on the game page', () => {
  it('opens the version choice from a link under the Install button when the plugin offers versions', async () => {
    getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
    render(<GamePage game={game()} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.getByRole('button', { name: /^Install$/ })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /^Version$/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Choose version' }));
    // It lands on the Versions tab of the game settings.
    expect(await screen.findByRole('tab', { name: 'Versions', selected: true })).toBeTruthy();
    expect(getGameVersions).toHaveBeenCalledOnce();
  });

  it('shows nothing without the capability', () => {
    render(<GamePage game={game()} plugin={plugin(['install'])} {...base} />);
    expect(screen.queryByRole('button', { name: 'Choose version' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
    expect(screen.queryByRole('tab', { name: 'Versions' })).toBeNull();
  });

  it('shows nothing for a game whose features leave versions out', () => {
    render(<GamePage game={game({ features: ['install'] })} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.queryByRole('button', { name: 'Choose version' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
    expect(screen.queryByRole('tab', { name: 'Versions' })).toBeNull();
  });

  it('shows nothing for Steam and sideloaded games', () => {
    render(<GamePage game={game({ source: 'steam', features: null })} {...base} />);
    expect(screen.queryByRole('button', { name: 'Choose version' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
    expect(screen.queryByRole('tab', { name: 'Versions' })).toBeNull();
  });

  it('shows the pin on the page and holds the version choice in the settings tab', async () => {
    getGameVersions.mockResolvedValue({ channels: [], builds: [], manual: null, links: [], updateAvailable: false, pinned: null, ledger: [] });
    render(<GamePage game={game({ is_installed: true, version: '2.0.0', pinned: { kind: 'build', id: 'b200' } })} plugin={plugin(['install', 'versions'])} {...base} />);
    expect(screen.getByText('pinned')).toBeTruthy();
    expect(screen.getByText('Pinned to')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /^Version$/ })).toBeNull(); // no longer a button on the page
    fireEvent.click(screen.getByRole('button', { name: 'Game settings' }));
    fireEvent.click(screen.getByRole('tab', { name: 'Versions' }));
    expect(await screen.findByRole('heading', { name: 'Versions' })).toBeTruthy();
    await waitFor(() => expect(getGameVersions).toHaveBeenCalledOnce());
  });
});
