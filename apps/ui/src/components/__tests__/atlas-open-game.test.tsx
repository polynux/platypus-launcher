// Atlas covers/table open the clicked game: the list's staged game must not
// override the routed game id once the library leaves the list view.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';

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
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'library.presentation') return 'list-detail';
      return 'page';
    },
    useThemeLayout: () => null,
    loadTheme: () => Promise.resolve(),
  };
});

import { App } from '../../App';

afterEach(() => { cleanup(); window.localStorage.clear(); window.location.hash = ''; });

describe('atlas covers/table open the clicked game', () => {
  for (const mode of ['Covers', 'Table'] as const) {
    it(`${mode} opens the clicked game after the list staged another`, async () => {
      const user = userEvent.setup();
      render(<App />);
      await user.click(await screen.findByRole('button', { name: 'Library' }));
      // The list stages a game first, as it does in real use.
      await screen.findByRole('heading', { name: 'Alpha Quest' });
      await user.click(screen.getByRole('button', { name: mode }));
      const target = mode === 'Covers'
        ? await screen.findByRole('button', { name: 'Beta Run' })
        : (await screen.findAllByText('Beta Run'))[0]!;
      await user.click(target);
      expect(await screen.findByRole('heading', { name: 'Beta Run' })).toBeInTheDocument();
      expect(screen.queryByRole('heading', { name: 'Alpha Quest' })).not.toBeInTheDocument();
    });
  }
});
