// The game page's Workshop section (protocol 1.22): subscribed items, their
// disk state, and the Sync action that queues the plugin's job. The data hook
// goes through the same LauncherApi call as the real page.
import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getGameWorkshop: vi.fn(),
  getCrashHints: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';

const plugin = (caps: string[]): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: caps, declared: caps, install_folder: '/games' }) as StorePlugin;
const game = (): Game => ({
  id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Echo Tiny', cover_url: '', banner_url: '',
  install_dir: '/games/echo/Tiny', executable: '', is_installed: true, installable: true, features: null,
  settings: { runner_id: 'auto', env_vars: {}, launch_arguments: '' },
}) as Game;

function page(p: StorePlugin, onSyncWorkshop = vi.fn()) {
  render(
    <GamePage
      game={game()} plugin={p} runners={[]} activity={[]} backLabel="Library" onBack={() => {}} onPlay={() => {}} onStop={() => {}} onForceStop={() => {}}
      launching={false} tweaks={[]} onChangeSettings={() => {}} onSaveSettings={async () => {}} onJob={vi.fn()} onJobAction={() => {}} onAllowFolder={() => {}}
      onNavigate={() => {}} jobs={[]} onNotify={() => {}} onSyncWorkshop={onSyncWorkshop}
    />,
  );
  return onSyncWorkshop;
}

const items = [
  { id: '1', title: 'Echo Extra Level', preview_url: 'https://example.com/1.jpg', size_bytes: 3 << 20, updated_at: 1791000000, installed: true, needs_update: false },
  { id: '2', title: 'Echo Skin Pack', preview_url: '', size_bytes: 1 << 20, updated_at: 1791100000, installed: true, needs_update: true },
];

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockResolvedValue({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null });
  api.getCrashHints.mockResolvedValue(null);
  api.getGameWorkshop.mockResolvedValue(items);
});

describe('Workshop section (protocol 1.22)', () => {
  it('lists the subscribed items with their state and syncs on demand', async () => {
    const onSync = page(plugin(['library', 'install', 'launch', 'workshop']));
    expect(await screen.findByText('Echo Extra Level')).toBeInTheDocument();
    expect(screen.getByText('Echo Skin Pack')).toBeInTheDocument();
    expect(screen.getByText('Update available')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Sync' }));
    expect(onSync).toHaveBeenCalledTimes(1);
  });

  it('shows a failed listing with Retry instead of hiding the section', async () => {
    api.getGameWorkshop.mockRejectedValueOnce(new Error('Steam CM connection dropped: reset'));
    page(plugin(['library', 'install', 'launch', 'workshop']));
    expect(await screen.findByText(/Steam CM connection dropped/)).toBeInTheDocument();
    const retry = screen.getByRole('alert').querySelector('button');
    if (!retry) throw new Error('the error row has no Retry button');
    const before = api.getGameWorkshop.mock.calls.length;
    fireEvent.click(retry);
    await waitFor(() => expect(api.getGameWorkshop.mock.calls.length).toBe(before + 1));
    expect(await screen.findByText('Echo Extra Level')).toBeInTheDocument();
  });

  it('renders nothing without the workshop capability', async () => {
    api.getGameWorkshop.mockResolvedValue([]);
    page(plugin(['library', 'install', 'launch']));
    await waitFor(() => expect(api.getGameWorkshop).not.toHaveBeenCalled());
    expect(screen.queryByText('Echo Extra Level')).not.toBeInTheDocument();
  });
});
