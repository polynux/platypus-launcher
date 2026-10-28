import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

// Plugin lifecycle UI (issues #147, #161): an "Add a store" header action
// installs a zip and opens the approval sheet, a replace answer asks before
// overwriting, and every card offers Uninstall behind a confirmation that
// lists what goes and what stays. Bundled plugins only disable.
const calls = {
  chooseZip: 0,
  install: [] as Array<{ path: string; confirm: boolean }>,
  uninstall: [] as Array<{ id: string; removeGames: boolean }>,
  rescan: 0,
};
let installAnswer: unknown = { id: 'zippy', version: '1.0.0' };
const notifyCalls: Array<{ msg: string; kind?: 'ok' | 'err' | 'busy' }> = [];
let listed: StorePlugin[] = [];

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: () => Effect.succeed({ loggedIn: false } as never),
    listPlugins: () => Effect.succeed(listed as never),
    rescanPlugins: () => {
      calls.rescan += 1;
      return Effect.succeed([] as never);
    },
    approvePlugin: () => Effect.succeed(undefined as never),
    setPluginEnabled: () => Effect.succeed(undefined as never),
    retryPlugin: () => Effect.succeed(undefined as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    choosePluginZip: () => {
      calls.chooseZip += 1;
      return Effect.succeed('/tmp/p.zip' as never);
    },
    installPluginZip: (zipPath: string, confirmReplace: boolean) => {
      calls.install.push({ path: zipPath, confirm: confirmReplace });
      return Effect.succeed(installAnswer as never);
    },
    uninstallPlugin: (id: string, removeGames: boolean) => {
      calls.uninstall.push({ id, removeGames });
      return Effect.succeed(undefined as never);
    },
    setPluginInstallFolder: () => Effect.succeed(undefined as never),
    choosePluginInstallFolder: () => Effect.succeed('' as never),
    logout: () => Effect.succeed({ loggedIn: false } as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
  onAuth: () => () => {},
}));

import { PluginsView } from '../PluginsView';
import type { StorePlugin } from '../../types';

function plugin(over: Partial<StorePlugin> = {}): StorePlugin {
  return {
    id: 'echo',
    name: 'Echo',
    version: '1',
    author: '',
    description: '',
    store: 'echo',
    kind: 'store',
    status: 'ready',
    missing: [],
    requires: [],
    capabilities: [],
    declared: [],
    offers: [],
    needs: [],
    warnings: [],
    initialized: false,
    dir: '/x',
    exec: 'run.sh',
    hash: 'h',
    network: [],
    read: [],
    write: [],
    running: false,
    install_folder: '/g',
    default_install_folder: '/g',
    extra_folders: [],
    ...over,
  };
}

function zippy(): StorePlugin {
  return plugin({ id: 'zippy', name: 'Zippy', version: '1.0.0', status: 'unapproved' });
}

describe('plugin lifecycle', () => {
  beforeEach(() => {
    calls.chooseZip = 0;
    calls.install.length = 0;
    calls.uninstall.length = 0;
    calls.rescan = 0;
    notifyCalls.length = 0;
    listed = [];
    installAnswer = { id: 'zippy', version: '1.0.0' };
  });

  it('shows Uninstall on a card, but not on a bundled one', () => {
    const { rerender } = render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    expect(screen.getByRole('button', { name: 'Uninstall' })).toBeTruthy();
    rerender(
      <PluginsView plugins={[plugin({ bundled: true })]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    expect(screen.queryByRole('button', { name: 'Uninstall' })).toBeNull();
  });

  it('uninstall asks what goes and what stays, then calls UninstallPlugin', async () => {
    render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    expect(screen.getByText(/installed game files stay on disk/i)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /^Uninstall$/ }));
    await waitFor(() => expect(calls.uninstall).toEqual([{ id: 'echo', removeGames: false }]));
    await waitFor(() => expect(notifyCalls.some((n) => /uninstalled/i.test(n.msg))).toBe(true));
  });

  it('the uninstall checkbox also forgets per-game settings', async () => {
    render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    fireEvent.click(screen.getByRole('checkbox'));
    fireEvent.click(screen.getByRole('button', { name: /^Uninstall$/ }));
    await waitFor(() => expect(calls.uninstall).toEqual([{ id: 'echo', removeGames: true }]));
  });

  it('Add a store installs the zip and opens the approval sheet', async () => {
    listed = [zippy()];
    render(
      <PluginsView plugins={[]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Add a store' }));
    await waitFor(() => expect(calls.install).toEqual([{ path: '/tmp/p.zip', confirm: false }]));
    await waitFor(() => expect(screen.getByText('Approve Zippy')).toBeTruthy());
    await waitFor(() => expect(notifyCalls.some((n) => /installed/i.test(n.msg))).toBe(true));
  });

  it('an installed id asks before replacing, then replaces on confirm', async () => {
    installAnswer = { id: 'echo', version: '1.0.0', needsConfirm: true, currentVersion: '1.0.0', newVersion: '2.0.0' };
    render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Add a store' }));
    await waitFor(() => expect(screen.getByText('Replace Echo?')).toBeTruthy());
    expect(screen.getByText(/never merged/i)).toBeTruthy();
    installAnswer = { id: 'echo', version: '2.0.0', replaced: true };
    fireEvent.click(screen.getByRole('button', { name: 'Replace' }));
    await waitFor(() => expect(calls.install).toEqual([
      { path: '/tmp/p.zip', confirm: false },
      { path: '/tmp/p.zip', confirm: true },
    ]));
  });
});
