import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getLauncherSettings: vi.fn(),
  setMinimizeToTray: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
  onCloud: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
}));

import { TraySetting } from '../LauncherSettingsView';

beforeEach(() => {
  api.getLauncherSettings.mockReset();
  api.setMinimizeToTray.mockReset();
});

describe('TraySetting (issue #49)', () => {
  it('is off by default and toggles the minimize-to-tray switch', async () => {
    api.getLauncherSettings.mockResolvedValue({});
    api.setMinimizeToTray.mockResolvedValue({ minimize_to_tray: true });
    const onNotify = vi.fn();
    render(<TraySetting onNotify={onNotify} />);
    const toggle = await screen.findByLabelText('Minimize to tray while a game runs');
    expect(toggle).toHaveAttribute('aria-checked', 'false');
    fireEvent.click(toggle);
    await waitFor(() => expect(api.setMinimizeToTray).toHaveBeenCalledWith(true));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Platypus will hide to the tray while a game runs'));
  });

  it('shows the saved on state', async () => {
    api.getLauncherSettings.mockResolvedValue({ minimize_to_tray: true });
    render(<TraySetting onNotify={vi.fn()} />);
    const toggle = await screen.findByLabelText('Minimize to tray while a game runs');
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    expect(api.setMinimizeToTray).not.toHaveBeenCalled();
  });
});
