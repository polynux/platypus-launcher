import React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import type { Game } from '../../types';

const net = vi.hoisted(() => ({ online: true }));
const cbs = vi.hoisted(() => ({ list: [] as Array<(s: { online: boolean }) => void> }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getNetState: () => Effect.succeed({ online: net.online, since: 0 }),
  },
}));
vi.mock('../../effect/bridge', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../effect/bridge')>();
  return {
    ...actual,
    onEvent: (event: string, cb: (s: { online: boolean }) => void) => {
      if (event === 'netstate') cbs.list.push(cb);
      return () => {};
    },
  };
});

import { LibraryView } from '../LibraryView';
import { useNetState } from '../useNetState';

const game = (o: Partial<Game> = {}): Game =>
  ({
    id: 'p.echo.tiny',
    source: 'echo',
    store: 'echo',
    store_app_id: 'tiny',
    title: 'Echo Tiny',
    is_installed: true,
    features: null,
    settings: { runner_id: 'auto', env_vars: {} },
    ...o,
  }) as Game;

function library(onlineGames: Game[] = [game()]) {
  render(
    <LibraryView
      games={onlineGames}
      runners={[]}
      activity={[]}
      search=""
      onSearch={() => {}}
      source="all"
      onSource={() => {}}
      mode="cards"
      onMode={() => {}}
      ownership="installed"
      onOwnership={() => {}}
      warnings={[]}
      plugins={[]}
      jobs={[]}
      lastScan={null}
      onOpenGame={() => {}}
      onAddGame={() => {}}
      onNavigate={() => {}}
    />,
  );
}

function Probe() {
  const { online } = useNetState();
  return <span data-testid="probe">{online ? 'online' : 'offline'}</span>;
}

describe('offline indicator', () => {
  afterEach(() => {
    cleanup();
    net.online = true;
    cbs.list.length = 0;
  });

  it('shows nothing while online', async () => {
    library();
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  });

  it('shows the offline indicator with the last-known-library note', async () => {
    net.online = false;
    library();
    expect(await screen.findByRole('status')).toHaveTextContent('Offline — showing the last known library');
  });

  it('follows the netstate event without a reload', async () => {
    library();
    await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
    render(<Probe />);
    for (const cb of cbs.list) cb({ online: false });
    expect(await screen.findByRole('status')).toHaveTextContent('Offline');
    expect(screen.getByTestId('probe')).toHaveTextContent('offline');
    await i18n.changeLanguage('fr');
    expect(screen.getByRole('status')).toHaveTextContent('Hors ligne');
    await i18n.changeLanguage('en');
  });

  it('marks a network game while its state is unknown by default', () => {
    // Absent offline means unknown: nothing shown for ok/unknown games.
    expect((game() as Game & { offline?: string }).offline).toBeUndefined();
    expect((game({ offline: 'network' }) as Game & { offline?: string }).offline).toBe('network');
  });
});
