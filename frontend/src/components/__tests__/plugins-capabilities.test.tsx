import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, within } from '@testing-library/react';
import { Effect } from 'effect';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    authStatus: () => Effect.succeed({ loggedIn: true } as never),
    listPlugins: () => Effect.succeed([] as never),
    rescanPlugins: () => Effect.succeed([] as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

import { PluginsView } from '../PluginsView';
import type { PluginCapability, StorePlugin } from '../../types';

const cap = (name: string, o: Partial<PluginCapability> = {}): PluginCapability =>
  ({ name, on: true, known: true, orchestrated: true, features: {}, ...o });

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

const view = (p: StorePlugin) =>
  render(<PluginsView plugins={[p]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={() => {}} />);

/** The capabilities list folds away: open it the way a user does. */
const openCapabilities = (getByRole: ReturnType<typeof view>['getByRole']) => {
  fireEvent.click(getByRole('button', { name: /capabilities · / }));
  return getByRole('list', { name: 'Capabilities' });
};

describe('Plugins view lists capabilities', () => {
  it('shows each declared capability with its state', () => {
    const { getByRole } = view(plugin({
      offers: [
        cap('library'),
        cap('verify', { on: false }),
        cap('versions', { orchestrated: false, features: { channels: true, builds: true, password: false } }),
        cap('hologram', { known: false, orchestrated: false }),
      ],
    }));
    const list = openCapabilities(getByRole);
    const rows = within(list).getAllByRole('listitem');
    const text = (name: string) => rows.find((r) => r.textContent?.startsWith(name))?.textContent ?? '';
    expect(rows).toHaveLength(4);
    expect(text('library')).toContain('on');
    expect(text('verify')).toContain('off right now');
    expect(text('versions')).toContain('not used by Platypus yet');
    expect(text('hologram')).toContain('unknown to this Platypus');
  });

  it('shows depth flags: what is on and what is off', () => {
    const { getByRole } = view(plugin({
      offers: [cap('versions', { features: { channels: true, builds: true, password: false } })],
    }));
    const row = within(openCapabilities(getByRole)).getByRole('listitem');
    expect(within(row).getByText('channels').getAttribute('data-on')).toBe('true');
    expect(within(row).getByText('builds').getAttribute('data-on')).toBe('true');
    expect(within(row).getByText('password').getAttribute('data-on')).toBe('false');
  });

  it('before the plugin has run, says the list is what it declared', () => {
    const { container } = view(plugin({ initialized: false, protocol: undefined }));
    expect(container.textContent).toContain('declared · not started yet');
  });

  it('shows warnings and the negotiated protocol', () => {
    const { container } = view(plugin({ warnings: ['offers "hologram", which this Platypus doesn\'t know; it is ignored'] }));
    expect(container.textContent).toContain('which this Platypus doesn\'t know');
    expect(container.textContent).toContain('protocol 1.1');
  });

  it('lists a plugin that needs a newer Platypus, with the reason and no way to approve it', () => {
    const { container, queryByText } = view(plugin({ status: 'needs-newer-platypus', problem: 'needs a newer Platypus (speaks protocol 2.0; Platypus speaks 1.0, 1.1)' }));
    expect(container.textContent).toContain('needs a newer Platypus');
    expect(container.textContent).toContain('speaks protocol 2.0');
    expect(queryByText('Review and approve')).toBeNull();
    expect(queryByText('Disable')).toBeNull();
  });

  it('a plugin that is too old says so', () => {
    const { container } = view(plugin({ status: 'too-old', problem: 'too old for this Platypus (speaks protocol 1.0; Platypus speaks 2.0)' }));
    expect(container.textContent).toContain('too old for this Platypus');
  });
});
