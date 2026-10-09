import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const LONG = `Library scan failed: plugin epic-legendary needs login: run \`legendary auth --code XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX\` and then rescan to restore 42 owned titles. `.padEnd(200, 'x').slice(0, 200);

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLibrary: () => Effect.fail(new Error(LONG) as never),
    getProtonVersions: () => Effect.succeed([] as never),
    listPlugins: () => Effect.succeed([] as never),
    getGameSettings: () => Effect.succeed({} as never),
    getSessions: () => Effect.succeed([] as never),
    getGameFix: () => Effect.succeed(null as never),
    getTweaks: () => Effect.succeed([] as never),
    listJobs: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onPlaytime: () => () => {},
  onCloud: () => () => {},
}));

import { App } from '../../App';

describe('FE-13 toast keeps the full error', () => {
  it('exposes a 200-char message in full (title) instead of a 90-char slice', async () => {
    const { getByRole } = render(<App />);
    const toast = await waitFor(() => getByRole('status'), { timeout: 5000 });
    const inner = toast.querySelector('span:last-child') as HTMLElement | null;
    const title = toast.getAttribute('title') ?? inner?.getAttribute('title') ?? '';
    // Full text must be in the title for copy/hover (may carry a source prefix).
    expect(title.includes(LONG)).toBe(true);
    // And visible text must wrap, not single-line truncate.
    expect(inner?.className.includes('truncate')).toBe(false);
  });
});
