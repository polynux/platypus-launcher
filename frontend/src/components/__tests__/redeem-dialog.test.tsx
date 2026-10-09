import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { StorePlugin } from '../../types';

const mocks = vi.hoisted(() => ({
  redeemCode: vi.fn(),
  authStatus: vi.fn(),
  listPlugins: vi.fn(),
}));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    redeemCode: (source: string, code: string) => mocks.redeemCode(source, code),
    authStatus: () => mocks.authStatus(),
    listPlugins: () => mocks.listPlugins(),
    rescanPlugins: () => Effect.succeed([] as never),
    openPluginsFolder: () => Effect.succeed(undefined as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

import { RedeemDialog } from '../RedeemDialog';
import { PluginsView } from '../PluginsView';

const echo = {
  id: 'echo', name: 'Echo', version: '1', author: '', description: '', store: 'echo', kind: 'store', status: 'ready',
  missing: [], requires: [], capabilities: ['library', 'redeem'], declared: ['library', 'redeem'],
  offers: [{ name: 'redeem', on: true, known: true, orchestrated: true, features: {} }], needs: [], warnings: [],
  initialized: true, protocol: '1.7', protocol_versions: '1.7', transport: 'jsonrpc-stdio', dir: '/x', exec: 'run.sh', hash: 'h',
  network: [], read: [], write: [], running: true, install_folder: '/g', default_install_folder: '/g', extra_folders: [],
} as unknown as StorePlugin;

beforeEach(() => {
  mocks.redeemCode.mockReset();
  mocks.authStatus.mockReset().mockReturnValue(Effect.succeed({ loggedIn: true } as never));
  mocks.listPlugins.mockReset().mockReturnValue(Effect.succeed([] as never));
});

describe('redeem a code', () => {
  it('stays disabled until something is typed, and caps the length at the core limit', () => {
    render(<RedeemDialog plugin={echo} onClose={() => {}} />);
    const submit = screen.getByRole('button', { name: 'Redeem' }) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    const input = screen.getByLabelText(/Code/) as HTMLInputElement;
    expect(input.maxLength).toBe(128);
    fireEvent.change(input, { target: { value: '   ' } });
    expect(submit.disabled).toBe(true);
  });

  it('sends the trimmed code to that store only and lists what it granted', async () => {
    mocks.redeemCode.mockReturnValue(Effect.succeed({ message: 'Redeemed.', titles: ['Echo Bonus Pack'] } as never));
    render(<RedeemDialog plugin={echo} onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText(/Code/), { target: { value: '  ECHO-OK ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Redeem' }));
    await waitFor(() => expect(screen.getByText('Redeemed.')).toBeTruthy());
    expect(mocks.redeemCode).toHaveBeenCalledWith('echo', 'ECHO-OK');
    expect(screen.getByText('Echo Bonus Pack')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Done' })).toBeTruthy();
  });

  it('shows the store\'s readable failure and keeps the code for another try', async () => {
    mocks.redeemCode.mockReturnValue(Effect.fail(new Error("Echo doesn't know that code. Try ECHO-OK.")));
    render(<RedeemDialog plugin={echo} onClose={() => {}} />);
    const input = screen.getByLabelText(/Code/) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'nope' } });
    fireEvent.click(screen.getByRole('button', { name: 'Redeem' }));
    expect(await screen.findByRole('alert')).toHaveTextContent("Echo doesn't know that code. Try ECHO-OK.");
    expect(input.value).toBe('nope');
  });

  it('refuses a second submit while the first one is still running', async () => {
    let finish: (v: unknown) => void = () => {};
    mocks.redeemCode.mockReturnValue(Effect.promise(() => new Promise((r) => (finish = r)) as never));
    render(<RedeemDialog plugin={echo} onClose={() => {}} />);
    fireEvent.change(screen.getByLabelText(/Code/), { target: { value: 'ECHO-OK' } });
    fireEvent.click(screen.getByRole('button', { name: 'Redeem' }));
    fireEvent.click(screen.getByRole('button', { name: 'Redeeming…' }));
    expect(mocks.redeemCode).toHaveBeenCalledTimes(1);
    finish({ message: 'ok', titles: [] });
    await waitFor(() => expect(screen.getByText('ok')).toBeTruthy());
  });
});

describe('plugins view offers redeem only where the store does', () => {
  const view = (p: StorePlugin) =>
    render(<PluginsView plugins={[p]} warnings={[]} onPlugins={() => {}} onLibraryChanged={() => {}} onNotify={() => {}} />);

  it('shows Redeem a code for a ready store that offers it', async () => {
    const { findByRole } = view(echo);
    expect(await findByRole('button', { name: /Redeem a code/ })).toBeTruthy();
  });

  it('hides it when the store does not offer redeem', () => {
    const { queryByRole } = view({ ...echo, offers: [], capabilities: ['library'] } as unknown as StorePlugin);
    expect(queryByRole('button', { name: /Redeem a code/ })).toBeNull();
  });
});
