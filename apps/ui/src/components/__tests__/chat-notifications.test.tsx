import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getLauncherSettings: vi.fn(),
  setChatDesktopNotify: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
  onJobs: () => () => {},
  onActivity: () => () => {},
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

import { ChatNotificationsSetting } from '../LauncherSettingsView';

beforeEach(() => {
  api.getLauncherSettings.mockReset();
  api.setChatDesktopNotify.mockReset();
});

describe('ChatNotificationsSetting (issue #12)', () => {
  it('is on by default and toggles the desktop-notifications switch', async () => {
    api.getLauncherSettings.mockResolvedValue({});
    api.setChatDesktopNotify.mockResolvedValue({ chat_desktop_notify: false });
    const onNotify = vi.fn();
    render(<ChatNotificationsSetting onNotify={onNotify} />);
    const toggle = await screen.findByLabelText('Desktop notifications for chat');
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(toggle);
    await waitFor(() => expect(api.setChatDesktopNotify).toHaveBeenCalledWith(false));
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('Chat stays in the app'));
  });

  it('shows the saved off state', async () => {
    api.getLauncherSettings.mockResolvedValue({ chat_desktop_notify: false });
    render(<ChatNotificationsSetting onNotify={vi.fn()} />);
    const toggle = await screen.findByLabelText('Desktop notifications for chat');
    expect(toggle).toHaveAttribute('aria-checked', 'false');
    expect(api.setChatDesktopNotify).not.toHaveBeenCalled();
  });
});
