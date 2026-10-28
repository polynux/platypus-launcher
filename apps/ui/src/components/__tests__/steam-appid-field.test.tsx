import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Compat, Game } from '../../types';

const getGameCompat = vi.fn();
const setGameSteamAppID = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getGameCompat: (g: Game) => getGameCompat(g),
    setGameSteamAppID: (g: Game, id: string) => setGameSteamAppID(g, id),
  },
}));

import { SteamAppIdField } from '../SteamAppIdField';
import { clearCompatCache } from '../../lib/compat';

const game = { id: 'g1', source: 'echo', store: 'gog', store_app_id: 'x', title: 'Echo Tiny', is_installed: false, settings: {} } as unknown as Game;
const answer = (o: Partial<Compat> = {}): Compat => ({ native: false, ratings_online: true, protection_online: true, badges: [], ...o });

describe('SteamAppIdField', () => {
  beforeEach(() => {
    clearCompatCache();
    getGameCompat.mockReset();
    setGameSteamAppID.mockReset();
  });

  it('links to the game on Steam when there is an AppID, and to a search when there is none', async () => {
    getGameCompat.mockReturnValue(Effect.succeed(answer({ steam_app_id: '620', steam_app_id_source: 'plugin' })));
    const { findByRole, rerender } = render(<SteamAppIdField game={game} />);
    const link = await findByRole('link', { name: /its Steam page/ });
    expect(link.getAttribute('href')).toBe('https://store.steampowered.com/app/620');
    expect(link.textContent).toContain('its Steam page');

    getGameCompat.mockReturnValue(Effect.succeed(answer()));
    clearCompatCache();
    rerender(<SteamAppIdField game={{ ...game, id: 'g2' } as Game} />);
    const search = await findByRole('link', { name: /Steam search/ });
    expect(search.getAttribute('href')).toBe('https://store.steampowered.com/search/?term=Echo%20Tiny');
  });

  it('refuses something that is not a number and never calls the backend', async () => {
    getGameCompat.mockReturnValue(Effect.succeed(answer()));
    const { getByLabelText, findByRole, getByRole } = render(<SteamAppIdField game={game} />);
    await findByRole('link');
    fireEvent.change(getByLabelText('Steam AppID for ratings'), { target: { value: 'portal-2' } });
    fireEvent.click(getByRole('button', { name: 'Save' }));
    expect((await findByRole('alert')).textContent).toMatch(/is a number/);
    expect(setGameSteamAppID).not.toHaveBeenCalled();
  });

  it('saves a number the user typed and shows the answer', async () => {
    getGameCompat.mockReturnValue(Effect.succeed(answer()));
    setGameSteamAppID.mockReturnValue(Effect.succeed(answer({ steam_app_id: '730', steam_app_id_source: 'user' })));
    const { getByLabelText, findByRole, getByRole, findByText } = render(<SteamAppIdField game={game} />);
    await findByRole('link');
    fireEvent.change(getByLabelText('Steam AppID for ratings'), { target: { value: ' 730 ' } });
    fireEvent.click(getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(setGameSteamAppID).toHaveBeenCalledTimes(1));
    expect(setGameSteamAppID.mock.calls[0][1]).toBe('730');
    expect(await findByText(/Currently 730 \(set by you\)/)).toBeTruthy();
  });
});
