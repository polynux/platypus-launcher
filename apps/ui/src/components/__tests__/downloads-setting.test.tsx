// Downloads settings (issue #207): the speed cap is edited in KiB/s and sent
// in bytes per second. The update window stays hidden until Platypus has
// automatic updates.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const getLauncherSettings = vi.fn();
const setDownloadLimit = vi.fn();
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () => getLauncherSettings(),
    setDownloadLimit: (n: number) => setDownloadLimit(n),
  },
}));
vi.mock('../../lib/theme', () => ({ useVariant: () => 'srow' }));

import { DownloadsSetting } from '../DownloadsSetting';

const settings = {
  games_folder: '', default_games_folder: '', cancel_cleanup_days: 7,
  download_limit: 0,
};

describe('Downloads settings', () => {
  beforeEach(() => {
    getLauncherSettings.mockReset();
    setDownloadLimit.mockReset();
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

  it('shows a saved cap in KiB/s and hides the update window', async () => {
    getLauncherSettings.mockReturnValue(Effect.succeed({ ...settings, download_limit: 2 * 1024 * 1024 }));
    const { getByRole, queryByRole } = render(<DownloadsSetting onNotify={() => {}} />);
    await waitFor(() => expect((getByRole('spinbutton', { name: 'Speed limit' }) as HTMLInputElement).value).toBe('2048'));
    expect(queryByRole('switch', { name: 'Only start updates in the window' })).toBeNull();
  });
});
