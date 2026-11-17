// i18n: converted
import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
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
  onActivity: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { ClientSetupAction } from '../ClientSetupAction';

const props = {
  client: 'Echo client',
  source: 'echo',
  onNotify: vi.fn(),
};

function codedError(code: string, params: Record<string, string>, message: string) {
  return Object.assign(new Error(message), { data: { code, params } });
}

beforeEach(() => {
  api.setupClient.mockReset();
  api.setupClient.mockResolvedValue({ id: 'job-1' });
});

describe('ClientSetupAction', () => {
  it('renders nothing without a prompt and no running job', () => {
    const { container } = render(<ClientSetupAction {...props} message={null} />);
    expect(container.textContent).toBe('');
  });

  it('shows the failure with a Set up button that queues unconfirmed', async () => {
    render(<ClientSetupAction {...props} message="Echo client isn't set up yet" />);
    fireEvent.click(screen.getByRole('button', { name: /Set up Echo client/ }));
    await waitFor(() => expect(api.setupClient).toHaveBeenCalledWith('echo', false));
  });

  it('asks for download confirmation when the client.setup answer requires it', async () => {
    api.setupClient.mockRejectedValueOnce(codedError('client.confirmDownload', { message: 'This downloads 800 MB.' }, 'This downloads 800 MB.'));
    render(<ClientSetupAction {...props} message="Echo client isn't set up yet" />);
    fireEvent.click(screen.getByRole('button', { name: /Set up Echo client/ }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog.textContent).toMatch(/800 MB/);
    fireEvent.click(screen.getByRole('button', { name: /Download/ }));
    await waitFor(() => expect(api.setupClient).toHaveBeenLastCalledWith('echo', true));
  });

  it('shows Setting up instead of Play while the setup job runs', () => {
    const job = { id: 'job-1', game_id: 'p.echo.client', kind: 'client-setup', state: 'running', title: 'Echo client' };
    render(<ClientSetupAction {...props} message={null} job={job as never} />);
    expect(screen.getByText(/Setting up Echo client/)).toBeDefined();
    expect(screen.queryByRole('button', { name: /Set up Echo client/ })).toBeNull();
  });
});
