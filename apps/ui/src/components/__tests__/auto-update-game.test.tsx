// Per-game automatic updates (issue #209): the Maintenance row's
// "Don't update automatically" box reads and saves the game's override, and
// says so when the global switch is off.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, LauncherSettings } from '../../types';

const getGameAutoUpdate = vi.fn();
const setGameAutoUpdate = vi.fn();
const getLauncherSettings = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameAutoUpdate: (id: string) => getGameAutoUpdate(id),
    setGameAutoUpdate: (id: string, on: boolean) => setGameAutoUpdate(id, on),
    getLauncherSettings: () => getLauncherSettings(),
  },
}));

import { AutoUpdateRow } from '../MaintenanceTab';

const game = { id: 'p.echo.tiny', title: 'Echo Tiny' } as unknown as Game;
const settings = (auto: boolean) => ({ auto_update: auto }) as LauncherSettings;

describe('AutoUpdateRow', () => {
  beforeEach(() => {
    getGameAutoUpdate.mockReset();
    setGameAutoUpdate.mockReset();
    getLauncherSettings.mockReset();
    getLauncherSettings.mockReturnValue(Effect.succeed(settings(true)));
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

  it('says automatic updates are off when the global switch is off', async () => {
    getGameAutoUpdate.mockReturnValue(Effect.succeed(true));
    getLauncherSettings.mockReturnValue(Effect.succeed(settings(false)));
    const { getByText } = render(<AutoUpdateRow game={game} onNotify={() => {}} />);
    await waitFor(() => expect(getByText(/Automatic updates are off in Settings/)).toBeTruthy());
  });
});
