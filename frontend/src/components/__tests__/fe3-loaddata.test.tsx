import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const calls = { library: 0, plugins: 0 };

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    socialViews: () => Effect.succeed([] as never),
    chatUnread: () => Effect.succeed([] as never),
    chatAlerts: () => Effect.succeed({} as never),
    getLibrary: () => {
      calls.library += 1;
      return Effect.succeed({
        games: [
          {
            id: 'g1',
            source: 'steam',
            store: 'steam',
            store_app_id: '10',
            title: 'Test Game',
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
          },
        ],
        warnings: [],
      } as never);
    },
    getProtonVersions: () => Effect.succeed([] as never),
    listPlugins: () => {
      calls.plugins += 1;
      return Effect.fail(new Error('ListStorePlugins boom') as never);
    },
    getGameSettings: () => Effect.succeed({} as never),
    getSessions: () => Effect.succeed([] as never),
    getGameFix: () => Effect.succeed(null as never),
    getTweaks: () => Effect.succeed([] as never),
    listJobs: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onFamily: () => () => {},
  onSession: () => () => {},
  onPlaytime: () => () => {},
  onCloud: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
}));

import { App } from '../../App';

describe('FE-3 loadData partitions failures', () => {
  beforeEach(() => {
    calls.library = 0;
    calls.plugins = 0;
  });

  it('still renders the library when listPlugins rejects', async () => {
    const { queryByText, getAllByText } = render(<App />);
    await waitFor(() => expect(calls.library).toBeGreaterThan(0), { timeout: 3000 });
    // Library payload arrived: game title visible on dashboard, no backend-unreachable.
    await waitFor(() => expect(getAllByText('Test Game').length).toBeGreaterThan(0), { timeout: 3000 });
    expect(queryByText('backend unreachable')).not.toBeInTheDocument();
  });
});
