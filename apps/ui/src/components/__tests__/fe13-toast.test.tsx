import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const LONG = `Library scan failed: plugin epic-legendary needs login: run \`legendary auth --code XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX\` and then rescan to restore 42 owned titles. `.padEnd(200, 'x').slice(0, 200);

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    socialViews: () => Effect.succeed([] as never),
    chatUnread: () => Effect.succeed([] as never),
    chatAlerts: () => Effect.succeed({} as never),
    getLibrary: () => Effect.fail(new Error(LONG) as never),
    getProtonVersions: () => Effect.succeed([] as never),
    listPlugins: () => Effect.succeed([] as never),
    getGameSettings: () => Effect.succeed({} as never),
    getSessions: () => Effect.succeed([] as never),
    getGameFix: () => Effect.succeed(null as never),
    getTweaks: () => Effect.succeed([] as never),
    migrationNotice: () => Effect.succeed(null as never),
    dismissMigrationNotice: () => Effect.succeed(undefined as never),
    listJobs: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onFamily: () => () => {},
  onSession: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
  onCloud: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
}));

import { App } from '../../App';

describe('FE-13 toast keeps the full error', () => {
  it('exposes a 200-char message in full instead of a 90-char slice', async () => {
    const { getByRole } = render(<App />);
    // Errors are assertive (issue #64): the load failure is a toast error.
    const toast = await waitFor(() => getByRole('alert'), { timeout: 5000 });
    expect(toast.textContent ?? '').toContain(LONG.slice(0, 60));
    // And visible text wraps (line-clamp), not single-line truncate.
    expect(toast.className.includes('truncate')).toBe(false);
    expect((toast.textContent ?? '').length).toBeGreaterThan(90);
  });
});
