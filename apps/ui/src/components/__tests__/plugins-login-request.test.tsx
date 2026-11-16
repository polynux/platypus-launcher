import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const authBeginCalls: string[] = [];

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    authBegin: (id: string) => {
      authBeginCalls.push(id);
      return Effect.succeed({ fields: [] } as never);
    },
    authStatus: (_id: string) => Effect.succeed({ loggedIn: false } as never),
    authComplete: (_id: string, _f: unknown) => Effect.succeed({ loggedIn: true } as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
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

describe('PluginsView login request', () => {
  it('opens the store login flow for a refused install, then releases the request', async () => {
    authBeginCalls.length = 0;
    const onLoginOpened = vi.fn();
    render(
      <PluginsView
        plugins={[plugin('echo')]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={() => {}}
        loginRequestId="echo"
        onLoginOpened={onLoginOpened}
      />,
    );
    await waitFor(() => expect(authBeginCalls).toContain('echo'));
    expect(onLoginOpened).toHaveBeenCalled();
  });

  it('waits for the plugin list before opening the login', async () => {
    authBeginCalls.length = 0;
    const onLoginOpened = vi.fn();
    const props = {
      warnings: [],
      onPlugins: () => {},
      onLibraryChanged: () => {},
      onNotify: () => {},
      loginRequestId: 'echo',
      onLoginOpened,
    };
    const { rerender } = render(<PluginsView plugins={[]} {...props} />);
    // The list hasn't loaded: nothing opens and the request is kept.
    const { promise, resolve } = Promise.withResolvers<void>();
    setTimeout(resolve, 50);
    await promise;
    expect(authBeginCalls).not.toContain('echo');
    expect(onLoginOpened).not.toHaveBeenCalled();
    rerender(<PluginsView plugins={[plugin('echo')]} {...props} />);
    await waitFor(() => expect(authBeginCalls).toContain('echo'));
    expect(onLoginOpened).toHaveBeenCalled();
  });

  it('toasts when the requested store is absent after the load', async () => {
    authBeginCalls.length = 0;
    const onNotify = vi.fn();
    const onLoginOpened = vi.fn();
    render(
      <PluginsView
        plugins={[plugin('other')]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={onNotify}
        loginRequestId="echo"
        onLoginOpened={onLoginOpened}
      />,
    );
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.stringMatching(/echo/), 'err'));
    expect(authBeginCalls).not.toContain('echo');
    expect(onLoginOpened).toHaveBeenCalled();
  });
});
