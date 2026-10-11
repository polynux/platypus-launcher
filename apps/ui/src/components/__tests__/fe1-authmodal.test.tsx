import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const authBeginCalls: string[] = [];

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authBegin: (id: string) => {
      authBeginCalls.push(id);
      return Effect.succeed({ fields: [] } as never);
    },
    authStatus: (_id: string) => Effect.succeed({ loggedIn: false } as never),
    authComplete: (_id: string, _f: unknown) => Effect.succeed({ loggedIn: true } as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: () => () => {},
}));


// Import after mocks
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

describe('FE-1 AuthModal auth.begin idempotence', () => {
  beforeEach(() => {
    authBeginCalls.length = 0;
  });

  it('opens AuthModal via Log in and fires auth.begin exactly once under StrictMode', async () => {
    const { getByRole } = render(
      <React.StrictMode>
        <PluginsView plugins={[plugin('epic')]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={() => {}} />
      </React.StrictMode>
    );
    // Click Log in to open AuthModal
    const loginBtn = await waitFor(() => getByRole('button', { name: /log in/i }));
    loginBtn.click();
    await waitFor(() => expect(authBeginCalls.length).toBeGreaterThan(0), { timeout: 2000 });
    // Give StrictMode double-effect a tick to (incorrectly) fire again
    await new Promise((r) => setTimeout(r, 100));
    expect(authBeginCalls.filter((x) => x === 'epic').length).toBe(1);
  });
});
