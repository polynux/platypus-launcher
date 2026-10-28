import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';

const h = vi.hoisted(() => ({
  api: {} as Record<string, ReturnType<typeof vi.fn>>,
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, name: string) => h.api[name] ?? (() => Effect.succeed(null)) }),
  onJobs: () => () => {},
  onFamily: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
  onCloud: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
}));

import { App } from '../../App';

const ok = <T,>(v: T) => vi.fn(() => Effect.succeed(v));

function baseApi() {
  Object.assign(h.api, {
    socialViews: ok([]),
    chatUnread: ok([]),
    chatAlerts: ok({}),
    getLibrary: ok({ games: [], warnings: [] }),
    getProtonVersions: ok([]),
    listPlugins: ok([]),
    getGameSettings: ok({}),
    getSessions: ok([]),
    getGameFix: ok(null),
    getTweaks: ok([]),
    listJobs: ok([]),
  });
}

beforeEach(() => {
  for (const k of Object.keys(h.api)) delete h.api[k];
  baseApi();
});

describe('migration notice', () => {
  it('shows the one-time notice after a move: what happened and the set-aside plugins by name', async () => {
    h.api.migrationNotice = ok({ moved: true, copied: false, set_aside: ['echo', 'custom'], warnings: [] });
    h.api.dismissMigrationNotice = ok(undefined);
    render(<App />);
    const banner = await waitFor(() => screen.getByRole('status', { name: /hydra to platypus/i }), { timeout: 5000 });
    expect(banner.textContent).toMatch(/moved from the old hydra folder/i);
    expect(banner.textContent).toMatch(/echo/);
    expect(banner.textContent).toMatch(/custom/);
    expect(banner.textContent).toMatch(/reinstall/i);
    // A button leads to the Plugins view, and dismissing hides the banner.
    expect(within(banner).getByRole('button', { name: /plugins/i })).toBeTruthy();
    fireEvent.click(within(banner).getByRole('button', { name: /dismiss/i }));
    await waitFor(() => expect(screen.queryByRole('status', { name: /hydra to platypus/i })).toBeNull());
    expect(h.api.dismissMigrationNotice).toHaveBeenCalled();
  });

  it('shows nothing when there is nothing to show', async () => {
    h.api.migrationNotice = ok(null);
    render(<App />);
    // The library is empty, so the dashboard shows its welcome banner — but
    // no migration banner.
    await waitFor(() => expect(screen.queryByRole('status', { name: /hydra to platypus/i })).toBeNull(), { timeout: 5000 });
  });
});
