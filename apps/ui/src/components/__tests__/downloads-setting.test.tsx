// Downloads settings (issue #207): the speed cap is edited in KiB/s and sent
// in bytes per second, and the update window saves through the backend.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setDownloadLimit = vi.fn();
const setUpdateWindow = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setDownloadLimit: (n: number) => setDownloadLimit(n),
    setUpdateWindow: (on: boolean, start: string, end: string) => setUpdateWindow(on, start, end),
  },
}));
vi.mock('../../lib/theme', () => ({ useVariant: () => 'srow' }));

import { DownloadsSetting } from '../DownloadsSetting';

const settings = {
  games_folder: '', default_games_folder: '', cancel_cleanup_days: 7,
  download_limit: 0, update_window_enabled: false,
  update_window_start: '03:00', update_window_end: '06:00',
};

describe('Downloads settings', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setDownloadLimit.mockReset();
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

  it('turns the update window on with the stored times', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed(settings));
    setUpdateWindow.mockReturnValue(Effect.succeed({
      ...settings, update_window_enabled: true, update_window_start: '22:00', update_window_end: '06:00',
    }));
    const { getByRole } = render(<DownloadsSetting onNotify={() => {}} />);
    const sw = getByRole('switch', { name: 'Only start updates in the window' });
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('unchecked'));
    fireEvent.click(sw);
    await waitFor(() => expect(setUpdateWindow).toHaveBeenCalledWith(true, '03:00', '06:00'));
    await waitFor(() => expect(sw.getAttribute('data-state')).toBe('checked'));
  });

  it('shows a saved cap and window', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({
      ...settings, download_limit: 2 * 1024 * 1024, update_window_enabled: true, update_window_start: '22:00', update_window_end: '06:00',
    }));
    const { getByRole, getByLabelText } = render(<DownloadsSetting onNotify={() => {}} />);
    await waitFor(() => expect((getByRole('spinbutton', { name: 'Speed limit' }) as HTMLInputElement).value).toBe('2048'));
    expect((getByRole('switch', { name: 'Only start updates in the window' }) as HTMLElement).getAttribute('data-state')).toBe('checked');
    expect((getByLabelText('Window start') as HTMLInputElement).value).toBe('22:00');
    expect((getByLabelText('Window end') as HTMLInputElement).value).toBe('06:00');
  });
});
