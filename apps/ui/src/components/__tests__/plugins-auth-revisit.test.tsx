import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const statusCalls: string[] = [];
let resolvers: Array<(s: { loggedIn: boolean; user?: string }) => void> = [];
let authCb: ((e: { plugin: string; status: { loggedIn: boolean; user?: string } }) => void) | null = null;

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: (id: string) => {
      statusCalls.push(id);
      return Effect.promise(() => new Promise<{ loggedIn: boolean }>((resolve) => {
        resolvers.push(resolve as (s: { loggedIn: boolean; user?: string }) => void);
      }));
    },
    authBegin: (_id: string) => Effect.succeed({ fields: [] } as never),
    authComplete: (_id: string, _f: unknown) => Effect.succeed({ loggedIn: true } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    approvePlugin: () => Effect.succeed(undefined as never),
    setPluginEnabled: () => Effect.succeed(undefined as never),
    retryPlugin: () => Effect.succeed(undefined as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    setPluginInstallFolder: () => Effect.succeed(undefined as never),
    choosePluginInstallFolder: () => Effect.succeed('' as never),
    allowGameFolder: () => Effect.succeed(undefined as never),
    logout: () => Effect.succeed({ loggedIn: false } as never),
    getPluginSettings: () => Effect.succeed({ fields: [], values: {} } as never),
    savePluginSettings: () => Effect.succeed({ applied: true } as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: (cb: (e: { plugin: string; status: { loggedIn: boolean; user?: string } }) => void) => {
    authCb = cb;
    return () => {
      authCb = null;
    };
  },
}));

// Import after mocks
import { PluginsView, resetPluginsAuthCache } from '../PluginsView';
import type { StorePlugin } from '../../types';

function plugin(id: string): StorePlugin {
  return {
    id,
    name: id,
    version: '1',
    author: '',
    description: '',
    store: id,
    kind: 'store',
    status: 'ready',
    missing: [],
    requires: [],
    capabilities: ['auth'],
    declared: ['auth'],
    offers: [],
    needs: [],
    warnings: [],
    initialized: false,
    dir: '/x',
    exec: 'run.sh',
    hash: 'h',
    network: [],
    read: [],
    write: [],
    running: false,
    install_folder: '/g',
    default_install_folder: '/g',
    extra_folders: [],
  };
}

function propsFor(p: StorePlugin) {
  return {
    plugins: [p],
    warnings: [],
    onPlugins: () => {},
    onLibraryChanged: () => {},
    onNotify: () => {},
  };
}

function resolveStatuses(s: { loggedIn: boolean; user?: string }) {
  resolvers.splice(0).forEach((r) => r(s));
}

describe('auth status cache: no flash on revisit', () => {
  beforeEach(() => {
    statusCalls.length = 0;
    resolvers = [];
    authCb = null;
    resetPluginsAuthCache();
  });

  it('shows the cached login state at once when the view is revisited', async () => {
    const props = propsFor(plugin('epic'));
    const first = render(<PluginsView {...props} />);
    // First visit: nothing cached yet, the view checks the login.
    expect(first.queryByText(/checking login/i)).not.toBeNull();
    resolveStatuses({ loggedIn: false });
    await waitFor(() => expect(first.queryByText(/logged out/i)).not.toBeNull(), { timeout: 3000 });
    expect(statusCalls.filter((x) => x === 'epic').length).toBe(1);
    first.unmount();

    statusCalls.length = 0;
    const second = render(<PluginsView {...props} />);
    // Revisit: the last known state shows immediately, with no flash…
    expect(second.queryByText(/checking login/i)).toBeNull();
    expect(second.queryByText(/logged out/i)).not.toBeNull();
    // …while the view still re-checks in the background.
    await waitFor(() => expect(statusCalls.filter((x) => x === 'epic').length).toBe(1), { timeout: 3000 });
    resolveStatuses({ loggedIn: false });
    await waitFor(() => expect(second.queryByText(/logged out/i)).not.toBeNull(), { timeout: 3000 });
    second.unmount();
  });

  it('updates the login state when the core reports a change', async () => {
    const view = render(<PluginsView {...propsFor(plugin('gog'))} />);
    resolveStatuses({ loggedIn: false });
    await waitFor(() => expect(view.queryByText(/logged out/i)).not.toBeNull(), { timeout: 3000 });
    expect(authCb).not.toBeNull();
    authCb?.({ plugin: 'gog', status: { loggedIn: true, user: 'echo-user' } });
    await waitFor(() => expect(view.queryByText(/echo-user/i)).not.toBeNull(), { timeout: 3000 });
    expect(view.queryByText(/checking login/i)).toBeNull();
    view.unmount();
  });
});
