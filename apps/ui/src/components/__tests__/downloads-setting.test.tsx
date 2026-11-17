// Downloads settings (issue #207, #209): the speed cap is edited in KiB/s and
// sent in bytes per second, automatic updates save at once, and the window
// they wait for saves with its times.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setDownloadLimit = vi.fn();
const setAutoUpdate = vi.fn();
const setUpdateWindow = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setDownloadLimit: (n: number) => setDownloadLimit(n),
    setAutoUpdate: (on: boolean) => setAutoUpdate(on),
    setUpdateWindow: (on: boolean, start: string, end: string) => setUpdateWindow(on, start, end),
  },
}));
vi.mock('../../lib/theme', () => ({ useVariant: () => 'srow' }));

import { DownloadsSetting } from '../DownloadsSetting';

const settings = {
  games_folder: '', default_games_folder: '', cancel_cleanup_days: 7,
  download_limit: 0, auto_update: true,
  update_window_enabled: false, update_window_start: '03:00', update_window_end: '06:00',
};

describe('Downloads settings', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setDownloadLimit.mockReset();
    setAutoUpdate.mockReset();
    setUpdateWindow.mockReset();
  });

  it('saves the speed cap in bytes per second', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed(settings));
    setDownloadLimit.mockReturnValue(Effect.succeed({ ...settings, download_limit: 512 * 1024 }));
    const onNotify = vi.fn();
    const { getByRole, getAllByRole } = render(<DownloadsSetting onNotify={onNotify} />);
    const field = getByRole('spinbutton', { name: 'Speed limit' }) as HTMLInputElement;
    await waitFor(() => expect(field.value).toBe('0'));
    fireEvent.change(field, { target: { value: '512' } });
    fireEvent.blur(field);
    const save = getAllByRole('button', { name: 'Save' })[0];
    await waitFor(() => expect((save as HTMLButtonElement).disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(setDownloadLimit).toHaveBeenCalledWith(512 * 1024));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Download speed limit saved'));
  });

  it('shows a saved cap and turns automatic updates off', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ ...settings, download_limit: 2 * 1024 * 1024 }));
    setAutoUpdate.mockReturnValue(Effect.succeed({ ...settings, auto_update: false }));
    const onNotify = vi.fn();
    const { getByRole } = render(<DownloadsSetting onNotify={onNotify} />);
    await waitFor(() => expect((getByRole('spinbutton', { name: 'Speed limit' }) as HTMLInputElement).value).toBe('2048'));
    const sw = getByRole('switch', { name: 'Automatic updates' });
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('checked'));
    fireEvent.click(sw);
    await waitFor(() => expect(setAutoUpdate).toHaveBeenCalledWith(false));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Automatic updates saved'));
  });

  it('saves the window with its interpolated label', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ ...settings, update_window_start: '22:00', update_window_end: '06:00' }));
    setUpdateWindow.mockReturnValue(Effect.succeed({
      ...settings, update_window_enabled: true, update_window_start: '22:00', update_window_end: '06:00',
    }));
    const { getByRole, getByText, getAllByRole } = render(<DownloadsSetting onNotify={() => {}} />);
    await waitFor(() => expect(getByText('Only auto-update between 22:00 and 06:00')).toBeTruthy());
    fireEvent.click(getByRole('switch', { name: 'Only auto-update in the window' }));
    const save = getAllByRole('button', { name: 'Save' }).find((b) => !(b as HTMLButtonElement).disabled)!;
    fireEvent.click(save);
    await waitFor(() => expect(setUpdateWindow).toHaveBeenCalledWith(true, '22:00', '06:00'));
  });
});
