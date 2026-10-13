import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: () => Effect.succeed({ loggedIn: true } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    getGameSettingsFields: () => Effect.succeed({ groups: [], values: {} } as never),
    saveGameSettingsFields: () => Effect.succeed({ applied: true } as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: () => () => {},
}));

import { PluginsView } from '../PluginsView';
import { GameSettingsPanel } from '../PluginSettings';
import type { Game, PluginCapability, StorePlugin } from '../../types';

const cap = (name: string): PluginCapability =>
  ({ name, on: true, known: true, orchestrated: true, features: {} });

function plugin(o: Partial<StorePlugin> = {}): StorePlugin {
  const offers = o.offers ?? [cap('library'), cap('install')];
  return {
    id: 'echo', name: 'Echo', version: '1', author: '', description: '', store: 'echo', kind: 'store', status: 'ready',
    missing: [], requires: [], capabilities: offers.filter((c) => c.on).map((c) => c.name), declared: offers.map((c) => c.name),
    offers, needs: [], warnings: [], initialized: true, protocol: '1.1', protocol_versions: '1.0, 1.1', transport: 'jsonrpc-stdio',
    dir: '/x', exec: 'run.sh', hash: 'h', network: [], read: [], write: [], running: true,
    install_folder: '/g', default_install_folder: '/g', extra_folders: [], ...o,
  };
}

describe('plugins view live language switching', () => {
  it('re-renders French without a reload', async () => {
    render(<PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={() => {}} />);
    expect(screen.getByText(/bring in libraries/)).toBeTruthy();
    expect(screen.getByText('approved')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Rescan' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /capabilities · 2/ }));
    expect(screen.getByRole('list', { name: 'Capabilities' })).toBeTruthy();

    await i18n.changeLanguage('fr');
    expect(screen.getByText(/importent les bibliothèques/)).toBeTruthy();
    expect(screen.getByText('approuvé')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Réanalyser' })).toBeTruthy();
    expect(screen.getByRole('list', { name: 'Capacités' })).toBeTruthy();

    await i18n.changeLanguage('en');
    expect(screen.getByText('approved')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Rescan' })).toBeTruthy();
  });

  it('translates the empty game-settings notice', async () => {
    const game = { id: 'g', title: 'Tiny', features: null } as Game;
    render(<GameSettingsPanel game={game} plugin={plugin({ capabilities: ['settings'], declared: ['settings'] })} framed={false} />);
    await waitFor(() => expect(screen.getByText('Echo has no settings for Tiny.')).toBeTruthy());
    await i18n.changeLanguage('fr');
    expect(screen.getByText("Echo n'a aucun réglage pour Tiny.")).toBeTruthy();
    await i18n.changeLanguage('en');
    expect(screen.getByText('Echo has no settings for Tiny.')).toBeTruthy();
  });
});
