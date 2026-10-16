import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const details = {
  summary: 'A short summary',
  screenshots: ['s1.png', 's2.png'],
  genres: ['Adventure'],
  developers: ['Echo Software'],
  publishers: ['Echo Software'],
  release_date: '2024-01-01',
  achievements: null,
  description_html: '<p>A long description with <b>markup</b>.</p>',
  website: 'https://example.com/echo-tiny',
  store_url: 'https://example.com/store/echo-tiny',
  languages: ['English', 'French'],
  controller: 'full',
  requirements: { minimum: '<p>1 GHz CPU</p>', recommended: '<p>2 GHz CPU</p>' },
  age_rating: '12+',
  tags: ['Test', 'Single-player'],
  videos: [{ url: 'https://example.com/t.mp4', thumbnail: 't.jpg', title: 'Trailer', mime: 'video/mp4' }],
};

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getCrashHints: () => Effect.succeed(null),
    getGameDetails: () => Effect.succeed(details as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
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
import type { Game } from '../../types';

let n = 0;
function game(): Game {
  n += 1;
  return {
    id: `media-${n}`, source: 'steam', store: 'steam', store_app_id: `${n}`, title: 'Media Game',
    cover_url: '', banner_url: '', hero_url: '', install_dir: '/g', executable: '', is_installed: true,
    features: [],
    settings: {
      runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false,
      use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0,
      gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '',
    },
  } as unknown as Game;
}

const props = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {},
  launching: false, tweaks: [], onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {},
  onAllowFolder() {}, onNavigate() {},
};

beforeEach(() => invalidateGameDetailsCache());

describe('game page media + richer info (#24/#25)', () => {
  it('collapses the formatted description behind Show more', async () => {
    render(<GamePage game={game()} {...props} />);
    const html = await screen.findByTestId('about-html');
    expect(html.innerHTML).toContain('<b>markup</b>');
    const wrap = screen.getByTestId('about-html-wrap');
    expect(wrap.className).toMatch(/max-h-/);
    fireEvent.click(screen.getByTestId('about-toggle'));
    expect(screen.getByTestId('about-toggle').textContent).toBe('Show less');
    expect(screen.getByTestId('about-html-wrap').className).not.toMatch(/max-h-/);
  });

  it('shows the details grid, requirements tabs and links', async () => {
    render(<GamePage game={game()} {...props} />);
    const grid = await screen.findByTestId('details-grid');
    for (const label of ['Developer', 'Release', 'Genres', 'Languages', 'Controller', 'Age rating']) {
      expect(grid.textContent).toContain(label);
    }
    expect(grid.textContent).toContain('Full controller support');
    // Requirements default to minimum, switch to recommended.
    expect(screen.getByTestId('req-panel').textContent).toContain('1 GHz CPU');
    fireEvent.click(screen.getByTestId('req-tab-recommended'));
    expect(screen.getByTestId('req-panel').textContent).toContain('2 GHz CPU');
    const links = screen.getByTestId('about-links');
    expect(links.textContent).toContain('Website');
    expect(links.textContent).toContain('Store page');
  });

  it('opens the trailer click-to-play in the lightbox, muted with no autoplay', async () => {
    render(<GamePage game={game()} {...props} />);
    const thumb = await screen.findByLabelText(/Play trailer/);
    fireEvent.click(thumb);
    fireEvent.click(screen.getByLabelText('Open trailer in viewer'));
    const video = await screen.findByTestId('lightbox-video') as HTMLVideoElement;
    expect(video.muted).toBe(true);
    expect(video.autoplay).toBe(false);
    expect(video.hasAttribute('autoplay')).toBe(false);
    expect(video.getAttribute('controls')).not.toBeNull();
    // Keyboard still works with a video selected: Esc closes.
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
  });
});
