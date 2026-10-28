// Atlas parity scroll guard (issue #132, gap 1): App's main clips only on the
// library/game views in list-detail mode; every other view scrolls like warm.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';

vi.mock('../../effect/runtime', () => {
  const ok = <T,>(v: T) => Effect.succeed(v);
  const empties: Record<string, unknown> = {
    getLibrary: { games: [], warnings: [] },
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
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'library.presentation') return 'list-detail';
      return 'page';
    },
    useThemeLayout: () => null,
    loadTheme: () => Promise.resolve(),
  };
});

import { App } from '../../App';

afterEach(cleanup);

const main = () => document.querySelector('[data-scroll-root]');

describe('atlas main scroll (issue #132)', () => {
  it('scrolls on non-library views under list-detail', async () => {
    render(<App />);
    // The dashboard starts with no games: the empty library notice shows, and
    // the main scrolls instead of clipping like the list-detail stage does.
    expect(await screen.findByText('no titles found')).toBeInTheDocument();
    expect(main()?.className).toMatch(/overflow-y-auto/);
    expect(main()?.className).not.toMatch(/overflow-hidden/);
  });

  it('clips on the library view under list-detail', async () => {
    const user = userEvent.setup();
    render(<App />);
    expect(await screen.findByText('no titles found')).toBeInTheDocument();
    // The /22 library bar only shows on the library and game views (#135).
    await user.click(screen.getByRole('button', { name: 'Library' }));
    await user.click(screen.getByRole('button', { name: /^Installed/ }));
    await screen.findByLabelText('Games');
    expect(main()?.className).toMatch(/overflow-hidden/);
  });
});
