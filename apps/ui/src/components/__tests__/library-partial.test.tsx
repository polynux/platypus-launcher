import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

// The startup scan shows what the first sources found while a slow store is
// still out; the final library then replaces it.
const state = vi.hoisted(() => ({
  release: null as null | (() => void),
  partial: null as null | ((lib: unknown) => void),
}));

const game = (id: string, title: string, source: string) => ({
  id, source, store: source, store_app_id: id, title, cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true,
  settings: { runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false, use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '' },
});

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    // Resolves when the test lets the slow store answer.
    getLibrary: () =>
      Effect.promise(
        () => new Promise((resolve) => {
          state.release = () => resolve({ games: [game('steam_1', 'Steam Game', 'steam'), game('slow_1', 'Slow Store Game', 'slow')], warnings: [] });
        }),
      ) as never,
    getProtonVersions: () => Effect.succeed([] as never),
    listPlugins: () => Effect.succeed([] as never),
    getGameSettings: () => Effect.succeed({} as never),
    getSessions: () => Effect.succeed([] as never),
    getGameFix: () => Effect.succeed(null as never),
    getTweaks: () => Effect.succeed([] as never),
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
  onPlaytime: () => () => {},
  onCloud: () => () => {},
  onLibraryPartial: (cb: (lib: unknown) => void) => {
    state.partial = cb;
    return () => {};
  },
}));

import { App } from '../../App';

describe('startup library scan', () => {
  it('shows the first sources before the slow store answers', async () => {
    const { container } = render(<App />);
    await waitFor(() => expect(state.release).not.toBeNull());
    await waitFor(() => expect(state.partial).not.toBeNull());
    expect(container.textContent).not.toContain('Steam Game');

    act(() => state.partial?.({ games: [game('steam_1', 'Steam Game', 'steam')], warnings: [] }));
    await waitFor(() => expect(screen.getAllByText('Steam Game').length).toBeGreaterThan(0));
    expect(container.textContent).not.toContain('Slow Store Game');

    act(() => state.release?.());
    await waitFor(() => expect(container.querySelectorAll('[aria-label="counting games"]').length).toBe(0));
    await waitFor(() => expect(screen.getAllByText('Steam Game').length).toBeGreaterThan(0));
  });
});
