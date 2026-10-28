import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

// Plugin lifecycle under Atlas (issues #147, #161): the detail column offers
// the same Add a store, Uninstall and replace flows as the classic cards.
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'dashboard.presentation') return 'home';
      if (axis === 'fixes.presentation') return 'cards';
      if (axis === 'activity.presentation') return 'days';
      if (axis === 'runners.presentation') return 'table';
      if (axis === 'downloads.presentation') return 'cards';
      if (axis === 'storage.presentation') return 'cards';
      if (axis === 'plugins.presentation') return 'directory';
      if (axis === 'friends.presentation') return 'panel';
      if (axis === 'store.presentation') return 'storefronts';
      if (axis === 'settings-row.style') return 'srow';
      if (axis === 'toast.style') return 'pill';
      return 'page';
    },
    useThemeLayout: () => null,
  };
});

const calls = {
  install: [] as Array<{ path: string; confirm: boolean }>,
  uninstall: [] as Array<{ id: string; removeGames: boolean }>,
  approve: [] as string[],
};
let installAnswer: unknown = { id: 'zippy', version: '1.0.0' };
const notifyCalls: Array<{ msg: string; kind?: 'ok' | 'err' | 'busy' }> = [];
let listed: StorePlugin[] = [];

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: () => Effect.succeed({ loggedIn: false } as never),
    listPlugins: () => Effect.succeed(listed as never),
    rescanPlugins: () => Effect.succeed([] as never),
    approvePlugin: (id: string) => {
      calls.approve.push(id);
      return Effect.succeed(undefined as never);
    },
    setPluginEnabled: () => Effect.succeed(undefined as never),
    retryPlugin: () => Effect.succeed(undefined as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    choosePluginZip: () => Effect.succeed('/tmp/p.zip' as never),
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

describe('plugin lifecycle under Atlas', () => {
  beforeEach(() => {
    calls.install.length = 0;
    calls.uninstall.length = 0;
    calls.approve.length = 0;
    notifyCalls.length = 0;
    listed = [];
    installAnswer = { id: 'zippy', version: '1.0.0' };
  });

  it('the detail column uninstalls behind the same confirmation', async () => {
    render(
      <PluginsView plugins={[plugin()]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    expect(screen.getByText(/stay on disk and in the library/i)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /^Uninstall$/ }));
    await waitFor(() => expect(calls.uninstall).toEqual([{ id: 'echo', removeGames: false }]));
  });

  it('Add a store installs from the header and opens approval', async () => {
    listed = [plugin({ id: 'zippy', name: 'Zippy', version: '1.0.0', status: 'unapproved' })];
    render(
      <PluginsView plugins={[]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={(msg, kind) => notifyCalls.push({ msg, kind })} />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Add a store' }));
    await waitFor(() => expect(calls.install).toEqual([{ path: '/tmp/p.zip', confirm: false }]));
    await waitFor(() => expect(screen.getByText('Approve Zippy')).toBeTruthy());
  });

  it('a data plugin uninstalls from the Atlas detail', async () => {
    render(
      <PluginsView
        plugins={[plugin({ id: 'data', name: 'Data', kind: 'data', status: 'ready' })]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={(msg, kind) => notifyCalls.push({ msg, kind })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall' }));
    expect(screen.queryByRole('checkbox')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /^Uninstall$/ }));
    await waitFor(() => expect(calls.uninstall).toEqual([{ id: 'data', removeGames: false }]));
  });

  it('a pending data plugin re-opens approval from the Atlas detail', async () => {
    render(
      <PluginsView
        plugins={[plugin({ id: 'data', name: 'Data', kind: 'data', status: 'unapproved' })]}
        warnings={[]}
        onPlugins={() => {}}
        onLibraryChanged={() => {}}
        onNotify={(msg, kind) => notifyCalls.push({ msg, kind })}
      />
    );
    fireEvent.click(screen.getByRole('button', { name: 'Review and approve' }));
    expect(screen.getByText('Approve Data')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Approve and enable' }));
    await waitFor(() => expect(calls.approve).toEqual(['data']));
  });
});
