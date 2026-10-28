// Plugins layouts (issue #102): the built-in arrangement and a cool-style
// themed one render the same blocks, and both pass axe. The store block keeps
// its required parts (the login row, the approve/enable actions) inside the
// card, so a layout can't hide them on their own.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';
import { TooltipProvider } from '../ds/tooltip';
import type { PluginCapability, StorePlugin, ThemeLayout } from '../../types';

const layoutState = vi.hoisted(() => ({ layout: null as ThemeLayout | null }));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: () => Effect.succeed({ loggedIn: true } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
    getGameSettingsFields: () => Effect.succeed({ groups: [], values: {} } as never),
    saveGameSettingsFields: () => Effect.succeed({ applied: true } as never),
    getThemeLayout: () => Effect.succeed(layoutState.layout),
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

expect.extend(matchers);
afterEach(() => { cleanup(); layoutState.layout = null; });

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const cap = (name: string): PluginCapability =>
  ({ name, on: true, known: true, orchestrated: true, features: {} });

function plugin(o: Partial<StorePlugin> = {}): StorePlugin {
  const offers = o.offers ?? [cap('library'), cap('auth'), cap('install')];
  return {
    id: 'echo', name: 'Echo', version: '1', author: '', description: 'Echo store', store: 'echo', kind: 'store', status: 'ready',
    missing: [], requires: [], capabilities: offers.filter((c) => c.on).map((c) => c.name), declared: offers.map((c) => c.name),
    offers, needs: [], warnings: [], initialized: true, protocol: '1.1', protocol_versions: '1.0, 1.1', transport: 'jsonrpc-stdio',
    dir: '/x', exec: 'run.sh', hash: 'h', network: [], read: [], write: [], running: true,
    install_folder: '/g', default_install_folder: '/g', extra_folders: [], ...o,
  };
}

const dataPlugin = (): StorePlugin => plugin({ id: 'theme-pack', name: 'Theme pack', kind: 'data', store: 'themes', description: 'A theme pack' });

const noop = () => {};
const view = (plugins: StorePlugin[]) =>
  render(<PluginsView plugins={plugins} warnings={[]} onPlugins={noop} onLibraryChanged={noop} onNotify={noop} />);

// The cool theme's plugins (internal/themes/builtin/cool/layouts/plugins.xml):
// the header and the store cards stay full width; the data cards and the
// built-in list share a row below.
const coolLayout: ThemeLayout = {
  view: 'plugins',
  regions: [
    {
      name: 'main',
      nodes: [{ block: 'header' }, { block: 'store' }, { row: [{ block: 'data' }, { block: 'builtin' }] }],
    },
  ],
};

describe('plugins layouts', () => {
  it('renders the built-in arrangement without a layout, with axe clean', async () => {
    const { container } = view([plugin(), dataPlugin()]);
    expect(await screen.findByText('Echo')).toBeTruthy();
    expect(screen.getByText('Theme pack')).toBeTruthy();
    expect(screen.getByText('ProtonDB')).toBeTruthy();
    // The required parts render inside the store card: the login row and
    // the install-folder action of a ready plugin.
    expect(screen.getByText('Echo store')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the cool layout: data and built-in share a row below the store cards', async () => {
    layoutState.layout = coolLayout;
    const { container } = view([plugin(), dataPlugin()]);
    expect(await screen.findByText('Echo')).toBeTruthy();
    const main = container.querySelector('[data-region="main"]');
    expect(main).toBeTruthy();
    const row = main!.querySelector(':scope > div:last-child');
    expect(row?.getAttribute('style')).toContain('repeat(2,');
    expect(within(row as HTMLElement).getByText('Theme pack')).toBeTruthy();
    expect(within(row as HTMLElement).getByText('ProtonDB')).toBeTruthy();
    // The store block keeps its required parts in the themed layout too.
    expect(within(main as HTMLElement).getByText('Echo')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('keeps the approve action and login row inside the store card', async () => {
    const { container } = view([plugin({ status: 'unapproved', offers: [cap('library')] })]);
    expect(await screen.findByRole('button', { name: /Review and approve/ })).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});
