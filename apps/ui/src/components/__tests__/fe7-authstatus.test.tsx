import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const statusCalls: string[] = [];

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: (id: string) => {
      statusCalls.push(id);
      return Effect.succeed({ loggedIn: false } as never);
    },
    authBegin: (_id: string) => Effect.succeed({ fields: [] } as never),
    authComplete: (_id: string, _f: unknown) => Effect.succeed({ loggedIn: true } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    approvePlugin: () => Effect.succeed(undefined as never),
    setPluginEnabled: () => Effect.succeed(undefined as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    setPluginInstallFolder: () => Effect.succeed(undefined as never),
    choosePluginInstallFolder: () => Effect.succeed('' as never),
    allowGameFolder: () => Effect.succeed(undefined as never),
    logout: () => Effect.succeed({ loggedIn: false } as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: () => () => {},
}));


import { PluginsView } from '../PluginsView';
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

describe('FE-7 authStatus single-flight', () => {
  beforeEach(() => {
    statusCalls.length = 0;
  });

  it('fetches authStatus once per ready plugin under StrictMode', async () => {
    render(
      <React.StrictMode>
        <PluginsView
          plugins={[plugin('epic'), plugin('gog')]}
          warnings={[]}
          onPlugins={() => {}}
          onLibraryChanged={() => {}}
          onNotify={() => {}}
        />
      </React.StrictMode>
    );
    await waitFor(() => expect(statusCalls.length).toBeGreaterThan(0), { timeout: 3000 });
    await new Promise((r) => setTimeout(r, 150));
    expect(statusCalls.filter((x) => x === 'epic').length).toBe(1);
    expect(statusCalls.filter((x) => x === 'gog').length).toBe(1);
  });
});
