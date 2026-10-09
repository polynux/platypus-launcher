import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { AchievementProgress, Game, StorePlugin } from '../../types';

const mocks = vi.hoisted(() => ({
  getGameAchievements: vi.fn(),
  getGameDetails: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameAchievements: (g: unknown) => mocks.getGameAchievements(g),
    getGameDetails: (g: unknown) => mocks.getGameDetails(g),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));

import { ProgressAchievements } from '../ProgressAchievements';
import { GamePage } from '../GamePage';

const progress: AchievementProgress = {
  total: 4,
  unlocked: 2,
  list: [
    { id: 'installed', name: 'Get installed', description: 'Install it.', icon: '', hidden: false, unlocked: true, unlocked_at: 1700000000, global_percent: 100 },
    { id: 'played', name: 'Play it', description: 'Start once.', icon: '', hidden: false, unlocked: false, unlocked_at: 0, global_percent: 84.1 },
    // Core blanks these; the UI shows what it is given.
    { id: 'secret', name: 'Hidden achievement', description: '', icon: '', hidden: true, unlocked: false, unlocked_at: 0 },
    { id: 'ninety', name: 'Ninety seconds', description: 'Keep it running.', icon: '', hidden: true, unlocked: true, unlocked_at: 1700100000 },
  ],
};

describe('progress achievements', () => {
  it('shows the count, a bar, unlock dates, global shares and masked hidden entries', () => {
    render(<ProgressAchievements progress={progress} />);
    expect(screen.getByText('2/4 · 50%')).toBeTruthy();
    const bar = screen.getByRole('progressbar', { name: 'Achievements unlocked' });
    expect(bar.getAttribute('aria-valuenow')).toBe('2');
    expect(bar.getAttribute('aria-valuemax')).toBe('4');
    expect(screen.getAllByText(/unlocked .*2023/)).toHaveLength(2);
    expect(screen.getByText('84.1% of players')).toBeTruthy();
    expect(screen.getByText('Hidden achievement')).toBeTruthy();
    expect(screen.getByText('Ninety seconds')).toBeTruthy();
    expect(screen.getByText('hidden')).toBeTruthy();
  });

  it('says how many it shows when the list is shorter than the total', () => {
    render(<ProgressAchievements progress={{ ...progress, total: 9 }} />);
    expect(screen.getByText('showing 4 of 9')).toBeTruthy();
  });
});

const echo = {
  id: 'echo', name: 'Echo', status: 'ready', capabilities: ['library', 'achievements'], declared: ['library', 'achievements'],
  offers: [{ name: 'achievements', on: true, known: true, orchestrated: true, features: {} }],
} as unknown as StorePlugin;

const game = {
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '', executable: '', is_installed: false, installable: false, features: null, playtime_minutes: 150,
  playtime_local_minutes: 30, playtime_store_minutes: 150,
  settings: { runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false, use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '' },
} as unknown as Game;

const pageProps = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onOpenSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {},
};

describe('game page stats', () => {
  beforeEach(() => {
    mocks.getGameDetails.mockReset().mockReturnValue(Effect.succeed({
      summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '',
      achievements: { total: 1, unlocked: -1, list: [{ id: 'catalogue', name: 'Catalogue only', description: '', icon: '', unlocked: false, unlocked_at: 0, global_percent: 0 }] },
    } as never));
    mocks.getGameAchievements.mockReset();
  });

  it('shows the playtime with the local and store values in its tooltip', async () => {
    mocks.getGameAchievements.mockReturnValue(Effect.succeed(null as never));
    render(<GamePage game={game} plugin={echo} {...pageProps} />);
    const logged = await screen.findByText('2.5h logged');
    expect(logged.getAttribute('title')).toBe('local 0.5 h · store 2.5 h');
  });

  it('shows the user\'s progress over the catalogue list when the plugin offers it', async () => {
    mocks.getGameAchievements.mockReturnValue(Effect.succeed(progress as never));
    render(<GamePage game={game} plugin={echo} {...pageProps} />);
    expect(await screen.findByLabelText('Your achievements')).toBeTruthy();
    expect(screen.getByText('2/4 · 50%')).toBeTruthy();
    expect(screen.queryByText('Catalogue only')).toBeNull();
    expect(mocks.getGameAchievements).toHaveBeenCalledTimes(1);
  });

  it('shows progress with no achievements in the total rather than the catalogue', async () => {
    mocks.getGameAchievements.mockReturnValue(Effect.succeed({ total: 0, unlocked: 0, list: [] } as never));
    render(<GamePage game={game} plugin={echo} {...pageProps} />);
    expect(await screen.findByLabelText('Your achievements')).toBeTruthy();
    expect(screen.getByText('0/0 · 0%')).toBeTruthy();
    expect(screen.queryByText('Catalogue only')).toBeNull();
  });

  it('falls back to the catalogue list when progress is not offered', async () => {
    mocks.getGameAchievements.mockReturnValue(Effect.succeed(null as never));
    render(<GamePage game={game} plugin={echo} {...pageProps} />);
    await waitFor(() => expect(screen.getByText('Catalogue only')).toBeTruthy());
    expect(screen.queryByLabelText('Your achievements')).toBeNull();
  });

  it('asks for progress only when the plugin offers the capability', async () => {
    const plain = { ...echo, capabilities: ['library'], offers: [{ name: 'achievements', on: false, known: true, orchestrated: true, features: {} }] } as unknown as StorePlugin;
    render(<GamePage game={game} plugin={plain} {...pageProps} />);
    await waitFor(() => expect(screen.getByText('Catalogue only')).toBeTruthy());
    expect(mocks.getGameAchievements).not.toHaveBeenCalled();
  });
});
