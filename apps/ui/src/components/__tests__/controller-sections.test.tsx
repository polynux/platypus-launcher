// LB/RB section switching through the whole App (issue #53), in both
// presentations: the classic sidebar (the default theme) and the Atlas
// sourcebar. The pad is stubbed; without the gamepad wiring the view never
// changes, so the hash assertions fail.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, waitFor } from '@testing-library/react';
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

function stubPad(pressed: number[]): void {
  const buttons = Array.from({ length: 17 }, (_, i) => ({ pressed: pressed.includes(i) }));
  Object.defineProperty(window.navigator, 'getGamepads', {
    value: () => [{ connected: true, buttons, axes: [0, 0] }, null, null, null],
    configurable: true,
  });
}

/** jsdom lays out at 0x0, so every button in nav/main gets a stacked rect in
 *  DOM order; anything else keeps its (empty) rect and is skipped. */
function stubLayout(): () => void {
  const orig = HTMLElement.prototype.getBoundingClientRect;
  const spy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    const els = Array.from(
      document.querySelectorAll('nav button, nav input, main button, main input, main a, main [tabindex]'),
    );
    const i = els.indexOf(this);
    if (i < 0) return orig.call(this);
    const top = 10 + i * 40;
    return { left: 10, top, width: 120, height: 24, right: 130, bottom: top + 24, x: 10, y: top, toJSON: () => ({}) } as DOMRect;
  });
  return () => spy.mockRestore();
}

function inMain(): boolean {
  const main = document.querySelector('main');
  return main instanceof HTMLElement && main.contains(document.activeElement);
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
  it('RB then LB move between sections with the classic sidebar', async () => {
    nav.shell = 'sidebar';
    stubPad([5]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      window.dispatchEvent(new Event('gamepadconnected'));
      await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
      // The section switch moves focus into the new view (finding 3).
      await waitFor(() => expect(inMain()).toBe(true), { timeout: 10_000 });
      stubPad([4]);
      await waitFor(() => expect(window.location.hash).not.toContain('library'), { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);

  it('RB then LB move between sections with the Atlas sourcebar', async () => {
    nav.shell = 'sourcebar';
    stubPad([5]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      window.dispatchEvent(new Event('gamepadconnected'));
      await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
      await waitFor(() => expect(inMain()).toBe(true), { timeout: 10_000 });
      stubPad([4]);
      await waitFor(() => expect(window.location.hash).not.toContain('library'), { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);

  it('the installed arrow handler moves focus into the view from body', async () => {
    nav.shell = 'sidebar';
    stubPad([]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      // Fresh load: nothing is focused. The real App-installed handler must
      // pick the first sensible element instead of doing nothing (finding 3).
      expect(document.activeElement === document.body || document.activeElement === null).toBe(true);
      fireEvent.keyDown(document.body, { key: 'ArrowDown' });
      expect(inMain()).toBe(true);
      expect(document.activeElement?.getAttribute('data-controller-focus')).toBe('true');
    } finally {
      restoreLayout();
    }
  }, 30_000);
});
