import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

let calls = 0;

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: (_g: unknown) => {
      calls += 1;
      return Effect.succeed({
        summary: 'A great game',
        screenshots: [],
        genres: [],
        developers: [],
        publishers: [],
        release_date: '',
        achievements: null,
      } as never);
    },
  },
  onJobs: () => () => {},
  onSession: () => () => {},
}));

import { GamePage, invalidateGameDetailsCache } from '../GamePage';
import type { Game } from '../../types';

function game(): Game {
  return {
    id: 'g-details-1',
    source: 'steam',
    store: 'steam',
    store_app_id: '10',
    title: 'Details Game',
    cover_url: '',
    banner_url: '',
    install_dir: '/g',
    executable: '',
    is_installed: true,
    settings: {
      runner_id: 'auto',
      wine_prefix: '',
      launch_arguments: '',
      env_vars: {},
      use_gamemode: false,
      use_mangohud: false,
      use_gamescope: false,
      gamescope_w: 0,
      gamescope_h: 0,
      gamescope_r: 0,
      gamescope_args: '',
      eac_runtime: false,
      battleye_runtime: false,
      working_dir: '',
    },
  };
}

const baseProps = {
  runners: [],
  activity: [],
  backLabel: 'Library',
  onBack: () => {},
  onPlay: () => {},
  launching: false,
  tweaks: [],
  onChangeSettings: () => {},
  onOpenSettings: () => {},
  onJob: () => {},
  onJobAction: () => {},
  onAllowFolder: () => {},
  onNavigate: () => {},
};

describe('FE-6 useDetails single-flight', () => {
  beforeEach(() => {
    calls = 0;
  });

  it('fires GetGameDetails once under StrictMode', async () => {
    invalidateGameDetailsCache();
    const { getByText } = render(
      <React.StrictMode>
        <GamePage game={game()} {...baseProps} />
      </React.StrictMode>
    );
    await waitFor(() => expect(getByText('A great game')).toBeInTheDocument(), { timeout: 3000 });
    await new Promise((r) => setTimeout(r, 100));
    expect(calls).toBe(1);
  });

  it('serves the cache on revisit but refetches after rescan eviction', async () => {
    invalidateGameDetailsCache();
    calls = 0;
    const g = game();
    const first = render(<GamePage game={g} {...baseProps} />);
    await waitFor(() => expect(first.getByText('A great game')).toBeInTheDocument(), { timeout: 3000 });
    expect(calls).toBe(1);
    first.unmount();
    // Revisit hits the cache: no new RPC.
    calls = 0;
    const second = render(<GamePage game={g} {...baseProps} />);
    await waitFor(() => expect(second.getByText('A great game')).toBeInTheDocument(), { timeout: 3000 });
    await new Promise((r) => setTimeout(r, 50));
    expect(calls).toBe(0);
    second.unmount();
    // Rescan evicts: next visit refetches.
    invalidateGameDetailsCache();
    calls = 0;
    const third = render(<GamePage game={g} {...baseProps} />);
    await waitFor(() => expect(third.getByText('A great game')).toBeInTheDocument(), { timeout: 3000 });
    expect(calls).toBe(1);
    third.unmount();
  });
});
