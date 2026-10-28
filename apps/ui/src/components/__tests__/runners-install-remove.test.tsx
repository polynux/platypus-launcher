// Issue #142: the Runners page shows each runner's size, installs available
// builds as a job with progress, and removes user-installed builds (never
// system Wine). A test must fail without the change: the old page shows "—"
// for size and offers no Install/Remove.
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ProtonVersion } from '../../types';
import type { Job } from '../../types';
import { SystemView } from '../SystemView';

const variant = vi.hoisted(() => ({ nav: 'sourcebar' }));
vi.mock('../../lib/theme', () => ({ useVariant: (axis: string) => (axis === 'shell.navigation' ? variant.nav : 'page') }));

const core = vi.hoisted(() => ({
  settings: { default_runner_id: '' },
  available: [{ tag: 'GE-Proton10-4', name: 'GE-Proton10-4', url: 'http://example.com/g.tar.gz', size_bytes: 400 * 1024 * 1024 }],
  removed: null as null | string,
  installedTag: null as null | string,
  installError: null as null | string,
  availableError: null as null | string,
  online: true,
}));
vi.mock('../../effect/runtime', async () => {
  const { Effect } = await import('effect');
  return {
    LauncherApi: {
      getLauncherSettings: () => Effect.succeed(core.settings as never),
      listAvailableRunners: () =>
        core.availableError ? Effect.fail(new Error(core.availableError) as never) : Effect.succeed(core.available as never),
      installRunner: (tag: string) => {
        core.installedTag = tag as never;
        return core.installError
          ? Effect.fail(new Error(core.installError) as never)
          : Effect.succeed({ id: 'job1', kind: 'runner-install', store_game_id: tag } as never);
      },
      checkRemoveRunner: (id: string) => Effect.succeed({ id, removable: true, used_by: [], size_bytes: 100 } as never),
      removeRunner: (id: string) => {
        core.removed = id as never;
        return Effect.succeed(undefined as never);
      },
      getNetState: () => Effect.succeed({ online: core.online, since: 0 } as never),
    },
  };
});

const runners = [
  { id: 'compat:GE-Proton10-3', name: 'GE-Proton10-3', path: '/runners/ge3/proton', is_system: false, is_custom: true, is_default: true, size_bytes: 500 * 1024 * 1024 },
  { id: 'system:wine', name: 'System Wine', path: '/usr/bin/wine', is_system: true, is_custom: false, is_default: false, size_bytes: 1024 },
] as ProtonVersion[];

describe('runner install and remove (issue #142)', () => {
  it('shows each runner size on the Atlas page', async () => {
    variant.nav = 'sourcebar';
    render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} />);
    await waitFor(() => expect(screen.getByText('524 MB')).toBeInTheDocument());
  });

  it('offers Remove for user builds but never for system Wine', async () => {
    variant.nav = 'sourcebar';
    window.confirm = vi.fn(() => true) as never;
    render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} />);
    expect(await screen.findByRole('button', { name: /Remove/ })).toBeInTheDocument();
    expect(screen.queryByText('From your system')).toBeInTheDocument();
    // Only one Remove (the compat build); system Wine has none.
    expect(screen.getAllByRole('button', { name: /Remove/ })).toHaveLength(1);
  });

  it('lists available builds with Install on the classic rows', async () => {
    variant.nav = 'page';
    core.installedTag = null;
    render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} />);
    // Installed size shows; available GE-Proton10-4 (not installed) offers Install.
    expect(await screen.findByText('524 MB')).toBeInTheDocument();
    const install = await screen.findByRole('button', { name: /Install/ });
    fireEvent.click(install);
    await waitFor(() => expect(screen.getByText('419 MB')).toBeInTheDocument());
    // The click queues the tagged build (n15).
    await waitFor(() => expect(core.installedTag).toBe('GE-Proton10-4'));
  });

  it('surfaces install failures instead of swallowing them (M7)', async () => {
    variant.nav = 'page';
    core.installError = 'nope';
    const onNotify = vi.fn();
    try {
      render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} onNotify={onNotify} />);
      fireEvent.click(await screen.findByRole('button', { name: /Install/ }));
      await waitFor(() => expect(onNotify).toHaveBeenCalledWith('nope', 'err'));
    } finally {
      core.installError = null;
    }
  });

  it('shows the offline notice when the available list fails offline (M7)', async () => {
    variant.nav = 'page';
    core.availableError = 'boom';
    core.online = false;
    const onNotify = vi.fn();
    try {
      render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} onNotify={onNotify} />);
      await waitFor(() => expect(onNotify).toHaveBeenCalledWith(expect.stringContaining('offline'), 'err'));
    } finally {
      core.availableError = null;
      core.online = true;
    }
  });

  it('refreshes the list when a runner install job finishes (M7)', async () => {
    variant.nav = 'page';
    const onRefresh = vi.fn();
    const job = { id: 'j1', kind: 'runner-install', state: 'done', title: 'GE-Proton10-4', game_id: 'runner:GE-Proton10-4' } as unknown as Job;
    render(<SystemView runners={runners} games={[]} onRefresh={onRefresh} scanning={false} jobs={[job]} onNotify={() => {}} />);
    await waitFor(() => expect(onRefresh).toHaveBeenCalled());
  });

  it('surfaces a failed runner install job (M7)', async () => {
    variant.nav = 'page';
    const onNotify = vi.fn();
    const job = { id: 'j2', kind: 'runner-install', state: 'failed', error: 'disk full', title: 'GE-Proton10-4', game_id: 'runner:GE-Proton10-4' } as unknown as Job;
    render(<SystemView runners={runners} games={[]} onRefresh={() => {}} scanning={false} jobs={[job]} onNotify={onNotify} />);
    await waitFor(() => expect(onNotify).toHaveBeenCalledWith('disk full', 'err'));
  });
});
