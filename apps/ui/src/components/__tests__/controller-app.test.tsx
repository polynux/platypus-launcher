// Round-2 App-level tests (issue #53): the bugs live in App wiring, so the
// App renders for real — permanent listbox scope, friends panel arrows,
// B through the real gamepad path, and LB/RB refocus. The pad is stubbed;
// without the wiring the hashes and focus never move.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { SocialView } from '../../types';

const nav = vi.hoisted(() => ({ shell: 'sidebar' }));

const { games } = vi.hoisted(() => {
  const mk = (id: string, title: string) => ({
    id, source: 'steam', store: 'steam', store_app_id: id, title,
    cover_url: '', banner_url: '', hero_url: '', install_dir: '/games',
    executable: 'game.exe', is_installed: true, settings: {}, last_played_at: 1700000000, shared_by: null,
  });
  return { games: [mk('alpha', 'Alpha Quest'), mk('beta', 'Beta Run')] };
});

const echoView = vi.hoisted((): SocialView => ({
  seq: 1, plugin: 'echo', enabled: true, showPlaying: true, running: true,
  connection: { state: 'online' },
  self: { name: 'Echo Player', state: 'online' },
  friends: [{ id: '1', name: 'Ana Ruiz', state: 'online' }],
}));

vi.mock('../../effect/runtime', () => {
  const ok = <T,>(v: T) => Effect.succeed(v);
  const empties: Record<string, unknown> = {
    getLibrary: { games, warnings: [] },
    rescanLibrary: { games: [], warnings: [] },
    getProtonVersions: [],
    getGameSettings: {},
    getSessions: [],
    getTweaks: [],
    listCollections: [],
    migrationNotice: null,
    chatUnread: [],
    listJobs: [],
    getLauncherSettings: null,
  };
  return {
    LauncherApi: new Proxy(
      {},
      {
        get: (_t, k: string) => {
          if (k === 'listPlugins') {
            return () => ok([
              { id: 'echo', name: 'Echo', status: 'ready', kind: 'store', capabilities: ['social'], declared: ['social'] },
              { id: 'gog', name: 'GOG', status: 'ready', kind: 'store', capabilities: [], declared: [] },
            ]);
          }
          if (k === 'socialViews') return () => ok([echoView]);
          return () => (k in empties ? ok(empties[k]) : ok(undefined));
        },
      },
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
    onAuth: () => () => {},
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

/** jsdom lays out at 0x0, so nav/main/aside buttons get stacked rects in
 *  DOM order; anything else keeps its (empty) rect and is skipped. */
function stubLayout(): () => void {
  const orig = HTMLElement.prototype.getBoundingClientRect;
  const spy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    const els = Array.from(
      document.querySelectorAll('nav button, nav input, main button, main input, main a, main [tabindex], aside button, aside input, aside [tabindex]'),
    );
    const i = els.indexOf(this);
    if (i < 0) return orig.call(this);
    const top = 10 + i * 40;
    return { left: 10, top, width: 120, height: 24, right: 130, bottom: top + 24, x: 10, y: top, toJSON: () => ({}) } as DOMRect;
  });
  return () => spy.mockRestore();
}

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  window.location.hash = '';
  if ('getGamepads' in window.navigator) {
    delete (window.navigator as unknown as Record<string, unknown>).getGamepads;
  }
});

describe('plugins permanent listbox', () => {
  it('does not trap arrows or B (N1)', async () => {
    nav.shell = 'sidebar';
    stubPad([]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      window.location.hash = '#/plugins';
      const listbox = await screen.findByRole('listbox', { name: 'Plugins' }, { timeout: 10_000 });
      const options = within(listbox).getAllByRole('option');
      expect(options.length).toBeGreaterThan(1);
      // Down moves within the option list instead of sticking on Echo.
      options[0].focus();
      fireEvent.keyDown(options[0], { key: 'ArrowDown' });
      await waitFor(() => expect(document.activeElement).toBe(options[1]));
      // Down past the last option leaves the listbox for the detail panel.
      fireEvent.keyDown(options[1], { key: 'ArrowDown' });
      await waitFor(() => expect(document.activeElement).not.toBe(options[1]));
      expect(listbox.contains(document.activeElement)).toBe(false);
      // B with no layer open goes back instead of sending Escape nowhere.
      stubPad([1]);
      window.dispatchEvent(new Event('gamepadconnected'));
      await waitFor(() => expect(window.location.hash).not.toContain('plugins'), { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);
});

describe('friends panel in App', () => {
  it('arrows move between tabs and Escape still closes (N3)', async () => {
    nav.shell = 'sidebar';
    stubPad([]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      fireEvent.click(screen.getByRole('button', { name: /Friends/ }));
      const tabs = await screen.findAllByRole('tab', undefined, { timeout: 10_000 });
      expect(tabs.length).toBeGreaterThan(1);
      tabs[0].focus();
      fireEvent.keyDown(tabs[0], { key: 'ArrowRight' });
      await waitFor(() => expect(document.activeElement).toBe(tabs[1]));
      fireEvent.keyDown(tabs[1], { key: 'Escape' });
      await waitFor(() => expect(screen.queryAllByRole('tab').length).toBe(0), { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);
});

describe('B through the real gamepad path', () => {
  it('backs out of the library to the dashboard', async () => {
    nav.shell = 'sidebar';
    stubPad([]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      window.location.hash = '#/library';
      await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
      stubPad([1]);
      window.dispatchEvent(new Event('gamepadconnected'));
      await waitFor(() => expect(window.location.hash).not.toContain('library'), { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);
});

describe('LB/RB always refocuses the new view', () => {
  it('moves focus off the stale nav button (N8)', async () => {
    nav.shell = 'sidebar';
    stubPad([5]);
    const restoreLayout = stubLayout();
    try {
      render(<App />);
      await waitFor(() => expect(document.body.textContent ?? '').not.toBe(''), { timeout: 10_000 });
      const dash = screen.getByRole('button', { name: 'Dashboard' });
      dash.focus();
      expect(document.activeElement).toBe(dash);
      window.dispatchEvent(new Event('gamepadconnected'));
      await waitFor(() => expect(window.location.hash).toContain('library'), { timeout: 10_000 });
      await waitFor(() => {
        expect(document.querySelector('main')?.contains(document.activeElement)).toBe(true);
      }, { timeout: 10_000 });
    } finally {
      restoreLayout();
    }
  }, 30_000);
});
