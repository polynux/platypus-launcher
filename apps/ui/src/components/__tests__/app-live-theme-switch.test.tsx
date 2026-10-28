import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';
import type { ThemeTokens } from '../../types';

vi.mock('../../effect/runtime', () => {
  const values: Record<string, unknown> = {
    getLibrary: { games: [], warnings: [] },
    getProtonVersions: [], listPlugins: [], getGameSettings: {}, getSessions: [],
    getTweaks: [], listCollections: [], migrationNotice: null, socialViews: [],
    chatUnread: [], listJobs: [], getLauncherSettings: {}, listThemes: [],
  };
  return {
    LauncherApi: new Proxy({}, { get: (_target, key: string) => () => Effect.succeed(values[key]) }),
    onJobs: () => () => {}, onLibrary: () => () => {}, onLibraryPartial: () => () => {},
    onPlaytime: () => () => {}, onActivity: () => () => {},
  onSession: () => () => {}, onSocial: () => () => {},
    onChat: () => () => {}, onAuth: () => () => {}, onCloud: () => () => {},
    onFamily: () => () => {},
  };
});

import { App } from '../../App';
import { applyTheme } from '../../lib/theme';

afterEach(() => cleanup());

const theme = (id: string, navigation: 'sidebar' | 'sourcebar'): ThemeTokens => ({
  id, tokens: {}, wash: [], variants: { 'shell.navigation': navigation } as ThemeTokens['variants'],
});

describe('App live theme switch', () => {
  it('switches warm → Atlas → warm on Home and Fixes without crashing', async () => {
    const user = userEvent.setup();
    applyTheme(theme('warm', 'sidebar'));
    render(<App />);
    await screen.findByText(/no titles found/i);
    act(() => applyTheme(theme('atlas', 'sourcebar')));
    act(() => applyTheme(theme('warm', 'sidebar')));
    await user.click(screen.getByRole('button', { name: 'Tools' }));
    await user.click(screen.getByRole('button', { name: /fixes/i }));
    await screen.findByText(/no fix data yet/i);
    act(() => applyTheme(theme('atlas', 'sourcebar')));
    act(() => applyTheme(theme('warm', 'sidebar')));
    expect(screen.getByRole('button', { name: /fixes/i })).toBeInTheDocument();
  });
});
