import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';

const api = vi.hoisted(() => ({
  clientState: vi.fn(),
  openClient: vi.fn(),
  setupClient: vi.fn(),
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
  onActivity: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { ClientCardActions } from '../ClientCardActions';
import type { PluginCapability, StorePlugin } from '../../types';

function codedError(code: string, params: Record<string, string>, message: string) {
  return Object.assign(new Error(message), { data: { code, params } });
}

function state(o = {}) {
  return {
    supported: true, setUp: false, running: false, setupJob: null, clientRunBusy: false, ...o,
  };
}

const notify = vi.fn();
const cap = (name: string): PluginCapability =>
  ({ name, on: true, known: true, orchestrated: true, features: {} });

function plugin(): StorePlugin {
  const offers = [cap('library'), cap('launch')];
  return {
    id: 'ubisoft-connect', name: 'Ubisoft Connect', version: '0.2.0', author: '', description: '', store: 'ubisoft',
    kind: 'store', status: 'ready', missing: [], requires: [],
    capabilities: ['library', 'launch'], declared: ['library', 'launch'], offers,
    needs: [], warnings: [], initialized: true, protocol: '1.25', protocol_versions: '1.20, 1.23, 1.25',
    transport: 'jsonrpc-stdio', dir: '/x', exec: 'run.sh', hash: 'h', network: [], read: [], write: [],
    running: false, install_folder: '/g', default_install_folder: '/g', extra_folders: [],
  };
}

beforeEach(() => {
  api.clientState.mockReset();
  api.openClient.mockReset();
  api.setupClient.mockReset();
  notify.mockReset();
  api.openClient.mockResolvedValue('Opened Ubisoft Connect (PID 1)');
  api.setupClient.mockResolvedValue({ id: 'job-1' });
});

afterEach(async () => {
  await i18n.changeLanguage('en');
});

describe('ClientCardActions', () => {
  it('shows Set up when the client prefix is missing', async () => {
    api.clientState.mockResolvedValue(state({ setUp: false }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    expect(await screen.findByRole('button', { name: 'Set up Ubisoft Connect' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Open Ubisoft Connect/ })).toBeNull();
  });

  it('shows Open with the sign-in hint once set up', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    expect(await screen.findByRole('button', { name: 'Open Ubisoft Connect' })).toBeTruthy();
    expect(screen.getByText('Sign in to Ubisoft Connect here; Platypus reads your games once you have.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Set up Ubisoft Connect/ })).toBeNull();
  });

  it('says the client is already running instead of hiding it', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true, running: true }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    await screen.findByRole('button', { name: 'Open Ubisoft Connect' });
    expect(screen.getByText('Ubisoft Connect is already running.')).toBeTruthy();
  });

  it('opens the client on click and reports the result', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Open Ubisoft Connect' }));
    await waitFor(() => expect(api.openClient).toHaveBeenCalledWith('ubisoft-connect'));
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Opened Ubisoft Connect (PID 1)'));
  });

  it('reports a refused open as an error', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true }));
    api.openClient.mockRejectedValueOnce(codedError('client.running', { client: 'Ubisoft Connect' }, 'Ubisoft Connect is already running.'));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Open Ubisoft Connect' }));
    await waitFor(() => expect(notify).toHaveBeenCalledWith('Ubisoft Connect is already running.', 'err'));
  });

  it('shows Setting up while the setup job holds the client', async () => {
    api.clientState.mockResolvedValue(state({
      setUp: false,
      setupJob: { id: 'job-1', game_id: 'p.ubisoft-connect.client', kind: 'client-setup', state: 'running', title: 'Ubisoft Connect client' } as never,
    }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    expect(await screen.findByText('Setting up Ubisoft Connect…')).toBeTruthy();
  });

  it('disables Open and explains while a client command runs', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true, clientRunBusy: true }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    const button = await screen.findByRole('button', { name: 'Open Ubisoft Connect' });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('Ubisoft Connect is busy; try again in a moment.')).toBeTruthy();
  });

  it('renders nothing for a store without a wine client', async () => {
    api.clientState.mockResolvedValue(state({ supported: false }));
    const { container } = render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    await waitFor(() => expect(api.clientState).toHaveBeenCalled());
    expect(container.textContent).toBe('');
  });

  it('copies read in French', async () => {
    api.clientState.mockResolvedValue(state({ setUp: true }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    await screen.findByRole('button', { name: 'Open Ubisoft Connect' });
    await i18n.changeLanguage('fr');
    expect(await screen.findByRole('button', { name: 'Ouvrir Ubisoft Connect' })).toBeTruthy();
    expect(screen.getByText('Connectez-vous à Ubisoft Connect ici ; Platypus lira vos jeux une fois fait.')).toBeTruthy();
    await i18n.changeLanguage('en');
    expect(await screen.findByRole('button', { name: 'Open Ubisoft Connect' })).toBeTruthy();
  });

  it('set-up button reads in French', async () => {
    api.clientState.mockResolvedValue(state({ setUp: false }));
    render(<ClientCardActions plugin={plugin()} onNotify={notify} />);
    await screen.findByRole('button', { name: 'Set up Ubisoft Connect' });
    await i18n.changeLanguage('fr');
    expect(await screen.findByRole('button', { name: 'Configurer Ubisoft Connect' })).toBeTruthy();
    await i18n.changeLanguage('en');
  });
});
