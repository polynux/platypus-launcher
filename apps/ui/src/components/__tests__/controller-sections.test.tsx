// LB/RB section switching through the whole App (issue #53), in both
// presentations: the classic sidebar (the default theme) and the Atlas
// sourcebar. The pad is stubbed; without the gamepad wiring the view never
// changes, so the hash assertions fail.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const nav = vi.hoisted(() => ({ shell: 'sidebar' }));

const { games } = vi.hoisted(() => {
  const mk = (id: string, title: string) => ({
    id, source: 'steam', store: 'steam', store_app_id: id, title,
    cover_url: '', banner_url: '', hero_url: '', install_dir: '/games',
    executable: 'game.exe', is_installed: true, settings: {}, last_played_at: 1700000000, shared_by: null,
  });
  return { games: [mk('alpha', 'Alpha Quest'), mk('beta', 'Beta Run')] };
});

vi.mock('../../effect/runtime', () => {
  const ok = <T,>(v: T) => Effect.succeed(v);
  const empties: Record<string, unknown> = {
    getLibrary: { games, warnings: [] },
    rescanLibrary: { games: [], warnings: [] },
    getProtonVersions: [],
    listPlugins: [],
    getGameSettings: {},
    getSessions: [],
    getTweaks: [],
    listCollections: [],
    migrationNotice: null,
    socialViews: [],
    chatUnread: [],
    listJobs: [],
    getLauncherSettings: null,
  };
  return {
    LauncherApi: new Proxy(
      {},
      { get: (_t, k: string) => () => (k in empties ? ok(empties[k]) : ok(undefined)) },
    ),
    onJobs: () => () => {},
    onLibrary: () => () => {},
    onLibraryPartial: () => () => {},
    onPlaytime: () => () => {},
    onActivity: () => () => {},
    onSession: () => () => {},
    onSocial: () => () => {},
    onChat: () => () => {},
    onCloud: () => () => {},
    onFamily: () => () => {},
  };
});

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return nav.shell;
      if (axis === 'dashboard.presentation') return 'home';
      if (axis === 'fixes.presentation') return 'cards';
      if (axis === 'activity.presentation') return 'days';
      if (axis === 'runners.presentation') return 'table';
      if (axis === 'downloads.presentation') return 'cards';
      if (axis === 'storage.presentation') return 'cards';
      if (axis === 'plugins.presentation') return 'directory';
      if (axis === 'friends.presentation') return 'panel';
      if (axis === 'store.presentation') return 'storefronts';
      if (axis === 'settings-row.style') return 'srow';
      if (axis === 'toast.style') return 'pill';
      if (axis === 'library.presentation') return 'list-detail';
      return 'page';
    },
    useThemeLayout: () => null,
    loadTheme: () => Promise.resolve(),
  };
});

import { App } from '../../App';

function stubPad(pressedIndex: number): void {
  const buttons = Array.from({ length: 17 }, (_, i) => ({ pressed: i === pressedIndex }));
  Object.defineProperty(window.navigator, 'getGamepads', {
    value: () => [{ connected: true, buttons, axes: [0, 0] }, null, null, null],
    configurable: true,
  });
}

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  window.location.hash = '';
  if ('getGamepads' in window.navigator) {
    delete (window.navigator as unknown as Record<string, unknown>).getGamepads;
  }
});

describe('gamepad section switching', () => {
  it('RB/LB move between sections with the classic sidebar', async () => {
    nav.shell = 'sidebar';
    stubPad(5);
    render(<App />);
    // Settle on the dashboard first.
    await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
    window.dispatchEvent(new Event('gamepadconnected'));
    await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
  }, 30_000);

  it('RB moves between sections with the Atlas sourcebar', async () => {
    nav.shell = 'sourcebar';
    stubPad(5);
    render(<App />);
    await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
    window.dispatchEvent(new Event('gamepadconnected'));
    await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
  }, 30_000);
});
