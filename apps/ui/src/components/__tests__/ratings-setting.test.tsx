import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setRatingsOnline = vi.fn();
const setProtectionOnline = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setRatingsOnline: (on: boolean) => setRatingsOnline(on),
    setProtectionOnline: (on: boolean) => setProtectionOnline(on),
  },
}));

import { RatingsSetting } from '../LauncherSettingsView';

const settings = { games_folder: '', default_games_folder: '', cancel_cleanup_days: 7 };

describe('Online compatibility data', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setRatingsOnline.mockReset();
    setProtectionOnline.mockReset();
  });

  it('has its own ratings and protection switches, both on by default, and saves them separately', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed(settings));
    setRatingsOnline.mockReturnValue(Effect.succeed({ ...settings, ratings_online: false }));
    setProtectionOnline.mockReturnValue(Effect.succeed({ ...settings, protection_online: false }));
    const onNotify = vi.fn();
    const { getByRole, getByText } = render(<RatingsSetting onNotify={onNotify} />);

    const ratings = getByRole('switch', { name: 'Fetch ProtonDB ratings online' });
    const protection = getByRole('switch', { name: 'Fetch protection data online' });
    await waitFor(() => expect(ratings.getAttribute('data-state')).toBe('checked'));
    await waitFor(() => expect(protection.getAttribute('data-state')).toBe('checked'));

    fireEvent.click(ratings);
    await waitFor(() => expect(setRatingsOnline).toHaveBeenCalledWith(false));
    await waitFor(() => expect(ratings.getAttribute('data-state')).toBe('unchecked'));
    expect(setProtectionOnline).not.toHaveBeenCalled();
    expect(onNotify).toHaveBeenCalledWith('Ratings stay offline');

    fireEvent.click(protection);
    await waitFor(() => expect(setProtectionOnline).toHaveBeenCalledWith(false));
    await waitFor(() => expect(protection.getAttribute('data-state')).toBe('unchecked'));
    expect(setRatingsOnline).toHaveBeenCalledTimes(1);
    expect(onNotify).toHaveBeenCalledWith('Protection data stays offline');

    expect(getByText(/ProtonDB community reports/)).toBeTruthy();
    expect(getByText(/PCGamingWiki contributors, CC BY-NC-SA 3.0/)).toBeTruthy();
    expect(getByText(/AreWeAntiCheatYet, MIT licence/)).toBeTruthy();
  });

  it('loads the protection switch from the saved settings, not the ratings one', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ ...settings, ratings_online: true, protection_online: false }));
    const { getByRole } = render(<RatingsSetting onNotify={() => {}} />);
    const protection = getByRole('switch', { name: 'Fetch protection data online' });
    await waitFor(() => expect(protection.getAttribute('data-state')).toBe('unchecked'));
    expect(getByRole('switch', { name: 'Fetch ProtonDB ratings online' }).getAttribute('data-state')).toBe('checked');
  });
});
