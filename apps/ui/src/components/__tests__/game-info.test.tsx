import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { ArtCandidate, Game } from '../../types';

const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  searchArtOnline: vi.fn(),
  linkGameInfo: vi.fn(),
  getLauncherSettings: vi.fn(),
  setGameInfoOnline: vi.fn(),
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
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
  onCloud: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
}));

import { GamePage, invalidateGameDetailsCache } from '../GamePage';
import { GameInfoSetting } from '../LauncherSettingsView';

const game = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom', title: 'Rivage',
  cover_url: '', banner_url: '', install_dir: '/g/rivage', executable: '/g/rivage/rivage.exe',
  is_installed: true, features: null, release_year: 0,
  settings: { runner_id: 'auto', env_vars: {} }, ...o,
}) as Game;

const emptyDetails = {
  summary: '', screenshots: [], genres: [], developers: [], publishers: [],
  release_date: '', achievements: null,
};

const baseProps = {
  runners: [], activity: [], backLabel: 'Library',
  onBack: () => {}, onPlay: () => {}, onStop: () => {}, onForceStop: () => {},
  launching: false, tweaks: [], onChangeSettings: () => {}, onSaveSettings: () => {},
  onJob: () => {}, onJobAction: () => {}, onAllowFolder: () => {}, onNavigate: () => {},
};

const cands: ArtCandidate[] = [
  { source: 'steam', id: '367520', app_id: 367520, name: 'Hollow Knight', thumb_url: 'https://cdn/367520.jpg' },
  { source: 'gog', id: '1308320804', app_id: 0, name: 'Hollow Knight', year: 2017, thumb_url: 'https://images.gog-statics.com/h.jpg' },
  { source: 'lutris', id: 'rivage', app_id: 0, name: 'Rivage', year: 2026, thumb_url: 'https://images.igdb.com/x.jpg' },
];

function page(g: Game = game()) {
  return render(<GamePage game={g} {...baseProps} />);
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  invalidateGameDetailsCache();
});

describe('GameInfo About (issue #38)', () => {
  it('shows Find info online for a sideloaded game with no linked info', async () => {
    api.getGameDetails.mockResolvedValue(emptyDetails);
    page();
    await waitFor(() => expect(api.getGameDetails).toHaveBeenCalled());
    expect(await screen.findByTestId('find-info-online')).toBeInTheDocument();
    expect(screen.queryByTestId('info-attribution')).toBeNull();
  });

  it('hides the finder for store games', async () => {
    api.getGameDetails.mockResolvedValue(emptyDetails);
    page(game({ id: 'steam-1', source: 'steam', store: 'steam', store_app_id: '123', executable: '' }));
    await waitFor(() => expect(api.getGameDetails).toHaveBeenCalled());
    expect(screen.queryByTestId('find-info-online')).toBeNull();
  });

  it('searches the same candidates and links a pick', async () => {
    api.getGameDetails.mockResolvedValue(emptyDetails);
    api.searchArtOnline.mockResolvedValue(cands);
    api.linkGameInfo.mockResolvedValue('gog:1308320804');
    page();
    fireEvent.click(await screen.findByTestId('find-info-online'));
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    await waitFor(() => expect(api.searchArtOnline).toHaveBeenCalledWith('Rivage'));
    const sources = screen.getAllByTestId('candidate-source').map((el) => el.textContent);
    expect(sources).toEqual(['Steam', 'GOG', 'Lutris']);
    fireEvent.click(screen.getAllByRole('button', { name: 'Link' })[1]);
    await waitFor(() => expect(api.linkGameInfo).toHaveBeenCalledWith('custom-1', 'gog', '1308320804'));
  });

  it('links a Steam candidate with its app id', async () => {
    api.getGameDetails.mockResolvedValue(emptyDetails);
    api.searchArtOnline.mockResolvedValue(cands);
    api.linkGameInfo.mockResolvedValue('steam:367520');
    page();
    fireEvent.click(await screen.findByTestId('find-info-online'));
    fireEvent.click(screen.getByRole('button', { name: 'Search' }));
    await screen.findAllByText('Hollow Knight');
    fireEvent.click(screen.getAllByRole('button', { name: 'Link' })[0]);
    await waitFor(() => expect(api.linkGameInfo).toHaveBeenCalledWith('custom-1', 'steam', '367520'));
  });

  it('shows the attribution with a store link and unlinks', async () => {
    api.getGameDetails.mockResolvedValue({
      ...emptyDetails, summary: 'Lead intro', release_date: '24 Feb, 2017',
      store_url: 'https://www.gog.com/game/hollow_knight',
      info_source: 'gog', info_source_name: 'GOG',
    });
    api.linkGameInfo.mockResolvedValue('');
    page();
    const attr = await screen.findByTestId('info-attribution');
    expect(attr.textContent).toMatch(/Info from/);
    expect(attr.textContent).toMatch(/GOG/);
    expect(attr.querySelector('a[href="https://www.gog.com/game/hollow_knight"]')).toBeTruthy();
    fireEvent.click(screen.getByTestId('info-unlink'));
    await waitFor(() => expect(api.linkGameInfo).toHaveBeenCalledWith('custom-1', '', ''));
  });

  it("shows the user's year over the source's release date", async () => {
    api.getGameDetails.mockResolvedValue({
      ...emptyDetails, summary: 'Lead intro', release_date: '24 Feb, 2017',
      developers: ['Team Cherry'], info_source: 'gog', info_source_name: 'GOG',
    });
    page(game({ release_year: 2026 }));
    const grid = await screen.findByTestId('details-grid');
    expect(grid.textContent).toMatch(/2026/);
    expect(grid.textContent).not.toMatch(/24 Feb, 2017/);
  });
});

describe('GameInfoSetting (issue #38)', () => {
  it('toggles the game-info switch', async () => {
    api.getLauncherSettings.mockResolvedValue({ game_info_online: true });
    api.setGameInfoOnline.mockResolvedValue({ game_info_online: false });
    const onNotify = vi.fn();
    render(<GameInfoSetting onNotify={onNotify} />);
    const toggle = await screen.findByLabelText('Find game info online');
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(toggle);
    await waitFor(() => expect(api.setGameInfoOnline).toHaveBeenCalledWith(false));
  });
});
