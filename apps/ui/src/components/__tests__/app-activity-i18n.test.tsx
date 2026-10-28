import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';

const h = vi.hoisted(() => ({
  api: {} as Record<string, ReturnType<typeof vi.fn>>,
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, name: string) => h.api[name] ?? (() => Effect.succeed(null)) }),
  onJobs: () => () => {},
  onFamily: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onLibrary: () => () => {},
  onCloud: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
}));

import { App } from '../../App';

const ok = <T,>(v: T) => vi.fn(() => Effect.succeed(v));

const game = {
  id: 'g1', source: 'steam', store: 'steam', store_app_id: '10', title: 'Test Game',
  cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true, settings: {},
};

function baseApi() {
  Object.assign(h.api, {
    socialViews: ok([]),
    chatUnread: ok([]),
    chatAlerts: ok({}),
    getLibrary: ok({ games: [game], warnings: [] }),
    getProtonVersions: ok([]),
    listPlugins: ok([]),
    getGameSettings: ok({}),
    getSessions: ok([]),
    getGameFix: ok(null),
    getTweaks: ok([]),
    migrationNotice: ok(null),
    listJobs: ok([]),
  });
}

beforeEach(() => {
  for (const k of Object.keys(h.api)) delete h.api[k];
  baseApi();
});

describe('activity log details follow the UI language', () => {
  it('logs the library scan in French when French is active', async () => {
    await i18n.changeLanguage('fr');
    render(<App />);
    // The scan ran while French was active, so its activity row is French.
    await waitFor(() => expect(screen.getByText(/Analyse de la bibliothèque/)).toBeTruthy(), { timeout: 5000 });
    expect(screen.getByText(/1 jeu installé/)).toBeTruthy();
    await i18n.changeLanguage('en');
  });
});
