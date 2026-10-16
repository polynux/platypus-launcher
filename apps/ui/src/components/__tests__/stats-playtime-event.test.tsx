import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { act, render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

// A playtime report reads the library only: no full rescan, no rescan notice.
const state = vi.hoisted(() => ({
  minutes: 30,
  library: 0,
  runners: 0,
  onPlaytime: null as null | (() => void),
  // When set, a library read waits for it.
  gate: null as null | Promise<void>,
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLibrary: () => {
      state.library += 1;
      const wait = state.gate;
      const answer = {
        games: [
          {
            id: 'g1', source: 'steam', store: 'steam', store_app_id: '10', title: 'Test Game', cover_url: '', banner_url: '',
            install_dir: '/g', executable: '', is_installed: true, playtime_minutes: state.minutes,
            settings: { runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false, use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '' },
          },
        ],
        warnings: [],
      };
      return (wait ? Effect.promise(() => wait.then(() => answer)) : Effect.succeed(answer)) as never;
    },
    getProtonVersions: () => {
      state.runners += 1;
      return Effect.succeed([] as never);
    },
    listPlugins: () => Effect.succeed([] as never),
    getGameSettings: () => Effect.succeed({} as never),
    getSessions: () => Effect.succeed([] as never),
    getGameFix: () => Effect.succeed(null as never),
    getTweaks: () => Effect.succeed([] as never),
    listCollections: () => Effect.succeed([] as never),
    migrationNotice: () => Effect.succeed(null as never),
    dismissMigrationNotice: () => Effect.succeed(undefined as never),
    listJobs: () => Effect.succeed([] as never),
    socialViews: () => Effect.succeed([] as never),
    chatUnread: () => Effect.succeed([] as never),
    chatAlerts: () => Effect.succeed({} as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
  onPlaytime: (cb: () => void) => {
    state.onPlaytime = cb;
    return () => {};
  },
  onCloud: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
}));

import { App } from '../../App';

describe('playtime event merges the library silently', () => {
  it('reads the library again without a rescan notice or a full reload', async () => {
    const { queryByText } = render(<App />);
    await waitFor(() => expect(state.library).toBe(1), { timeout: 3000 });
    await waitFor(() => expect(state.onPlaytime).not.toBeNull());
    const runnersBefore = state.runners;

    state.minutes = 150;
    act(() => state.onPlaytime?.());

    await waitFor(() => expect(state.library).toBe(2), { timeout: 3000 });
    expect(state.runners).toBe(runnersBefore);
    expect(queryByText('Library rescanned')).toBeNull();
  });

  it('shares one read between the reports that arrive while it is out', async () => {
    render(<App />);
    await waitFor(() => expect(state.library).toBeGreaterThanOrEqual(1), { timeout: 3000 });
    await waitFor(() => expect(state.onPlaytime).not.toBeNull());
    await new Promise((r) => setTimeout(r, 50));
    const before = state.library;
    let open!: () => void;
    state.gate = new Promise<void>((r) => (open = r));
    act(() => state.onPlaytime?.());
    act(() => state.onPlaytime?.());
    act(() => state.onPlaytime?.());
    expect(state.library).toBe(before + 1); // the first read is out, the others wait for it
    state.gate = null;
    open();
    await waitFor(() => expect(state.library).toBe(before + 2), { timeout: 3000 }); // one more read for all of them
    await new Promise((r) => setTimeout(r, 100));
    expect(state.library).toBe(before + 2);
  });
});
