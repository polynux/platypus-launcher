import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameDetails: (_g: unknown) =>
      Effect.succeed({
        summary: 's',
        screenshots: null,
        genres: null,
        developers: null,
        publishers: null,
        release_date: '',
        achievements: { total: 5, unlocked: -1, list: null },
      } as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
}));

import { GamePage } from '../GamePage';
import type { Game } from '../../types';

function game(): Game {
  return {
    id: 'g-ach-null',
    source: 'steam',
    store: 'steam',
    store_app_id: '10',
    title: 'Ach Game',
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
  onStop: () => {},
  onForceStop: () => {},
  launching: false,
  tweaks: [],
  onChangeSettings: () => {},
  onOpenSettings: () => {},
  onJob: () => {},
  onJobAction: () => {},
  onAllowFolder: () => {},
  onNavigate: () => {},
};

describe('FE-10 null achievements list does not crash', () => {
  it('renders the game page with list:null', async () => {
    const { getByText, getAllByText } = render(<GamePage game={game()} {...baseProps} />);
    await waitFor(() => expect(getByText('Achievements')).toBeInTheDocument(), { timeout: 3000 });
    expect(getAllByText('Ach Game').length).toBeGreaterThan(0);
  });
});
