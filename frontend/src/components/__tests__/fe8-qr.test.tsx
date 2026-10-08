import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

let statusCalls = 0;

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authBegin: (_id: string) =>
      Effect.succeed({ qr: '████\n████', fields: [], url: undefined, instructions: 'scan' } as never),
    authStatus: (_id: string) => {
      statusCalls += 1;
      return Effect.fail(new Error('store tool crashed') as never);
    },
    authComplete: () => Effect.succeed({ loggedIn: false } as never),
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
  onSession: () => () => {},
}));

vi.mock('../../../wailsjs/runtime/runtime', () => ({
  BrowserOpenURL: vi.fn(),
}));

import { PluginsView, qrPollDelay, QR_ERROR_AFTER, QR_MAX_ATTEMPTS } from '../PluginsView';
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

describe('FE-8 QR polling surfaces errors and backs off', () => {
  beforeEach(() => {
    statusCalls = 0;
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows an error after repeated poll failures instead of looping silently', async () => {
    const { getByRole } = render(
      <PluginsView
        plugins={[plugin('epic')]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={() => {}}
      />
    );
    getByRole('button', { name: /log in/i }).click();
    // authBegin resolves, QR shows, first poll scheduled
    await vi.advanceTimersByTimeAsync(2000);
    await vi.advanceTimersByTimeAsync(4000);
    await vi.advanceTimersByTimeAsync(6000);
    await vi.advanceTimersByTimeAsync(10000);
    // Flush pending Effect promises
    await Promise.resolve();
    const alerts = document.querySelectorAll('[role="alert"]');
    const texts = [...alerts].map((e) => e.textContent ?? '');
    expect(texts.some((t) => /reach|fail|error|retry/i.test(t))).toBe(true);
  });

  it('backs off polling delays and caps attempts', () => {
    expect(qrPollDelay(0)).toBe(1500);
    expect(qrPollDelay(1)).toBe(1500);
    expect(qrPollDelay(2)).toBe(3000);
    expect(qrPollDelay(3)).toBe(5000);
    expect(qrPollDelay(10)).toBe(15000);
    expect(QR_ERROR_AFTER).toBeLessThanOrEqual(3);
    expect(QR_MAX_ATTEMPTS).toBeLessThanOrEqual(60);
  });
});
