import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const retryCalls: string[] = [];
const notifyCalls: Array<{ msg: string; kind?: 'ok' | 'err' | 'busy' }> = [];

// CORE-10 UI: a plugin that gave up after repeated crashes (gave_up) gets a
// Retry action on its card; invoking it calls RetryPlugin and reports back.
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: (_id: string) => Effect.succeed({ loggedIn: false } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    retryPlugin: (id: string) => {
      retryCalls.push(id);
      return Effect.succeed(undefined as never);
    },
    setPluginEnabled: () => Effect.succeed(undefined as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: () => () => {},
}));

import { PluginsView } from '../PluginsView';
import type { StorePlugin } from '../../types';

function plugin(over: Partial<StorePlugin> = {}): StorePlugin {
  return {
    id: 'echo',
    name: 'Echo',
    version: '1',
    author: '',
    description: '',
    store: 'echo',
    kind: 'store',
    status: 'ready',
    missing: [],
    requires: [],
    capabilities: [],
    declared: [],
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
    ...over,
  };
}

describe('CORE-10 retry gave-up plugin from its card', () => {
  beforeEach(() => {
    retryCalls.length = 0;
    notifyCalls.length = 0;
  });

  it('shows no Retry action while the plugin is healthy', () => {
    const { queryByRole } = render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    expect(queryByRole('button', { name: /retry/i })).toBeNull();
  });

  it('offers Retry on a gave-up card and calls RetryPlugin', async () => {
    const { getByRole } = render(
      <PluginsView
        plugins={[plugin({ gave_up: true, problem: 'plugin echo crashed 3 times in a row, last: killed' })]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={(msg, kind) => notifyCalls.push({ msg, kind })}
      />
    );
    const btn = getByRole('button', { name: /retry/i });
    fireEvent.click(btn);
    await waitFor(() => expect(retryCalls).toEqual(['echo']));
    // act() toasts the done message (kind defaults to 'ok' in App.tsx).
    await waitFor(() => expect(notifyCalls.some((n) => n.msg.includes('Echo'))).toBe(true));
  });
});