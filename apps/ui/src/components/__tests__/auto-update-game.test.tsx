// Per-game automatic updates (issue #209): the Maintenance row's
// "Don't update automatically" box reads and saves the game's override.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

const getGameAutoUpdate = vi.fn();
const setGameAutoUpdate = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameAutoUpdate: (id: string) => getGameAutoUpdate(id),
    setGameAutoUpdate: (id: string, on: boolean) => setGameAutoUpdate(id, on),
  },
}));

import { AutoUpdateRow } from '../MaintenanceTab';

const game = { id: 'p.echo.tiny', title: 'Echo Tiny' } as unknown as Game;

describe('AutoUpdateRow', () => {
  beforeEach(() => {
    getGameAutoUpdate.mockReset();
    setGameAutoUpdate.mockReset();
  });

  it('excludes the game when the box is ticked', async () => {
    getGameAutoUpdate.mockReturnValue(Effect.succeed(true));
    setGameAutoUpdate.mockReturnValue(Effect.succeed(undefined));
    const onNotify = vi.fn();
    const { getByRole } = render(<AutoUpdateRow game={game} onNotify={onNotify} />);
    const box = getByRole('checkbox', { name: "Don't update automatically" }) as HTMLInputElement;
    await waitFor(() => expect(box.disabled).toBe(false));
    expect(box.checked).toBe(false);
    fireEvent.click(box);
    await waitFor(() => expect(setGameAutoUpdate).toHaveBeenCalledWith(game.id, false));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Automatic updates saved for this game.'));
    expect(box.checked).toBe(true);
  });

  it('returns an excluded game to the global switch', async () => {
    getGameAutoUpdate.mockReturnValue(Effect.succeed(false));
    setGameAutoUpdate.mockReturnValue(Effect.succeed(undefined));
    const { getByRole } = render(<AutoUpdateRow game={game} onNotify={() => {}} />);
    const box = getByRole('checkbox', { name: "Don't update automatically" }) as HTMLInputElement;
    await waitFor(() => expect(box.checked).toBe(true));
    fireEvent.click(box);
    await waitFor(() => expect(setGameAutoUpdate).toHaveBeenCalledWith(game.id, true));
    expect(box.checked).toBe(false);
  });
});
