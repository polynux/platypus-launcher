// Issue #144: resetting a damaged Wine prefix from the game's maintenance
// settings. The first click only arms the confirm; confirming calls the core
// and reports the backup it kept.
import React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { Game, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({
  resetPrefix: vi.fn(),
  listSaveBackups: vi.fn(),
  getSavePaths: vi.fn(),
  getGameSaveBackup: vi.fn(),
  getAutoBackupState: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    resetPrefix: (...a: unknown[]) => Effect.tryPromise({
      try: () => (api.resetPrefix as (...x: unknown[]) => Promise<unknown>)(...a),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
    listSaveBackups: () => Effect.succeed([]),
    getSavePaths: () => Effect.succeed([]),
    getGameSaveBackup: () => Effect.succeed(''),
    getAutoBackupState: () => Effect.succeed(null),
  },
  onJobs: () => () => {},
}));

import { MaintenanceTab } from '../MaintenanceTab';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const game = (o: Partial<Game> = {}): Game =>
  ({ id: 'wine', source: 'echo', store: 'echo', store_app_id: 'wine', title: 'Wine', cover_url: '', install_dir: '/g/wine', executable: '/g/wine/game.exe', is_installed: true, settings: {}, features: [], ...o }) as Game;

const plugin = { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['install'], declared: ['install'] } as unknown as StorePlugin;

const props = {
  jobs: [],
  onJob: () => {},
  onJobAction: () => {},
  onNotify: vi.fn(),
  onClose: () => {},
};

describe('resetting a damaged Wine prefix', () => {
  it('offers reset for an installed wine game and calls the core after confirming', async () => {
    const onNotify = vi.fn();
    api.resetPrefix.mockResolvedValue({ prefix: '/prefixes/wine', backup: '/prefixes/wine.bak-1' });
    render(<MaintenanceTab game={game()} plugin={plugin} {...props} onNotify={onNotify} />);
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    expect(screen.getByText('Reset the Wine prefix of Wine?')).toBeInTheDocument();
    expect(api.resetPrefix).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    await waitFor(() => expect(api.resetPrefix).toHaveBeenCalledWith('wine'));
    expect(onNotify).toHaveBeenCalledWith(expect.stringContaining('/prefixes/wine.bak-1'), 'ok');
  });

  it('cancelling keeps the prefix untouched', () => {
    render(<MaintenanceTab game={game()} plugin={plugin} {...props} />);
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(api.resetPrefix).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Reset prefix' })).toBeInTheDocument();
  });

  it('hides reset for native and store-client games', () => {
    const { unmount } = render(<MaintenanceTab game={game({ native: true })} plugin={plugin} {...props} />);
    expect(screen.queryByRole('button', { name: 'Reset prefix' })).toBeNull();
    unmount();
    render(<MaintenanceTab game={game({ client_launch: true })} plugin={plugin} {...props} />);
    expect(screen.queryByRole('button', { name: 'Reset prefix' })).toBeNull();
  });

  it('a core refusal surfaces as an error toast', async () => {
    const onNotify = vi.fn();
    api.resetPrefix.mockRejectedValue(new Error('refused: custom prefix'));
    render(<MaintenanceTab game={game()} plugin={plugin} {...props} onNotify={onNotify} />);
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.stringContaining('refused'), 'err'));
  });
});
