import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setRatingsOnline = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setRatingsOnline: (on: boolean) => setRatingsOnline(on),
  },
}));

import { RatingsSetting } from '../LauncherSettingsView';

describe('Fetch game info online', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setRatingsOnline.mockReset();
  });

  it('is on by default and turns off through the backend', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ games_folder: '', default_games_folder: '', cancel_cleanup_days: 7 }));
    setRatingsOnline.mockReturnValue(Effect.succeed({ games_folder: '', default_games_folder: '', cancel_cleanup_days: 7, ratings_online: false }));
    const onNotify = vi.fn();
    const { getByRole, getByText } = render(<RatingsSetting onNotify={onNotify} />);
    const sw = getByRole('switch', { name: 'Fetch game info online' });
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('checked'));
    fireEvent.click(sw);
    await waitFor(() => expect(setRatingsOnline).toHaveBeenCalledWith(false));
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('unchecked'));
    expect(onNotify).toHaveBeenCalledWith('Game info stays offline');
    expect(getByText(/ProtonDB community reports/)).toBeTruthy();
    expect(getByText(/PCGamingWiki contributors, CC BY-NC-SA 3.0/)).toBeTruthy();
    expect(getByText(/AreWeAntiCheatYet, MIT licence/)).toBeTruthy();
  });
});
