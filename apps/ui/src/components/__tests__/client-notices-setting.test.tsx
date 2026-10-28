import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setClientNotices = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setClientNotices: (on: boolean) => setClientNotices(on),
  },
}));

import { ClientNoticesSetting } from '../LauncherSettingsView';

const settings = { games_folder: '', default_games_folder: '', cancel_cleanup_days: 7 };

describe('Store client notices', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setClientNotices.mockReset();
  });

  it('is on by default and turns off through the backend', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed(settings));
    setClientNotices.mockReturnValue(Effect.succeed({ ...settings, client_notices: false }));
    const onNotify = vi.fn();
    const { getByRole } = render(<ClientNoticesSetting onNotify={onNotify} />);
    const sw = getByRole('switch', { name: 'Show store client notices' });
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('checked'));
    fireEvent.click(sw);
    await waitFor(() => expect(setClientNotices).toHaveBeenCalledWith(false));
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('unchecked'));
    expect(onNotify).toHaveBeenCalledWith('Store client launches stay quiet');
  });

  it('shows a saved off state as off', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ ...settings, client_notices: false }));
    const { getByRole } = render(<ClientNoticesSetting onNotify={() => {}} />);
    const sw = getByRole('switch', { name: 'Show store client notices' });
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('unchecked'));
  });
});
