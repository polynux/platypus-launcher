import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

// The runner's choices come from runner.options: one group per block, each
// field a control of its kind. The form shows them without folds.
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    previewCommand: () => Effect.succeed('cmd' as never),
    getRunnerEnv: () => Effect.succeed({ env: {}, wrappers: [] } as never),
    getRunnerOptions: () => Effect.succeed([
      { id: 'runner', label: 'Runner', fields: [{ id: 'runner_id', kind: 'select', label: 'Runner', scope: 'game', default: 'auto', options: [{ id: 'auto', label: 'Automatic' }] }] },
      { id: 'wrappers', label: 'Wrappers', fields: [
        { id: 'use_gamemode', kind: 'bool', label: 'GameMode', scope: 'game', default: true },
        { id: 'use_gamescope', kind: 'bool', label: 'Gamescope', scope: 'game', default: false },
      ] },
      { id: 'graphics', label: 'Graphics', fields: [{ id: 'dx_layer', kind: 'select', label: 'DX layer', scope: 'game', default: 'dxvk', options: [{ id: 'dxvk', label: 'DXVK (bundled)' }, { id: 'wined3d', label: 'WineD3D' }] }] },
      { id: 'sync', label: 'Sync', fields: [{ id: 'sync_mode', kind: 'select', label: 'Sync', scope: 'game', default: 'fsync', options: [{ id: 'fsync', label: 'fsync' }, { id: 'esync', label: 'esync' }] }] },
      { id: 'gamescope', label: 'Gamescope', fields: [{ id: 'gamescope_rate', kind: 'select', label: 'Refresh rate', scope: 'game', default: '', options: [{ id: '', label: 'Game default' }, { id: '144', label: '144 Hz' }] }] },
      { id: 'anticheat', label: 'Anti-cheat', fields: [{ id: 'eac_runtime', kind: 'bool', label: 'EAC runtime', scope: 'game', default: false }] },
    ] as never),
    getLauncherSettings: () => Effect.succeed({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' } as never),
    prefixUsage: () => Effect.succeed([] as never),
    listPrefixPresets: () => Effect.succeed([] as never),
    detectGameExecutable: () => Effect.succeed({ native: false, kind: 'windows-exe' } as never),
    isGameExecutable: () => Effect.succeed(true as never),
    pickGameExecutable: () => Effect.succeed('' as never),
    updateCustomGameExecutable: () => Effect.fail(new Error('not mocked') as never),
    makeGameExecutable: () => Effect.fail(new Error('not mocked') as never),
  },
}));

import { SettingsModal } from '../SettingsModal';

const game = (settings: object) =>
  ({ id: 'g1', title: 'G', store: 'custom', source: 'custom', executable: '/x.exe', settings: { env_vars: {}, ...settings } }) as unknown as Game;

const open = (g: Game) => render(<SettingsModal game={g} runners={[]} onClose={() => {}} onSave={async () => {}} onCopied={() => {}} />);

describe('general settings groups', () => {
  it('shows the runner choices in their groups, with no folds', async () => {
    open(game({ sync_mode: 'esync' }));
    const runner = screen.getByRole('region', { name: 'runner' });
    expect(await within(runner).findByLabelText('Sync')).toBeTruthy();
    expect(within(runner).queryByRole('button', { name: /Advanced/ })).toBeNull();
    expect(within(screen.getByRole('region', { name: 'graphics' })).getByLabelText('DX layer')).toBeTruthy();
    expect(screen.getByLabelText('Sync').textContent).toContain('esync');
  });

  it('keeps a saved value the schema does not list visible', async () => {
    open(game({ use_gamescope: true, gamescope_rate: '100' }));
    await screen.findByLabelText('Refresh rate');
    expect(screen.getByLabelText('Refresh rate').textContent).toContain('100');
  });

  it('shows the gamescope choices only while gamescope is on', async () => {
    const off = open(game({ use_gamescope: false }));
    await screen.findByLabelText('GameMode');
    expect(screen.queryByLabelText('Refresh rate')).toBeNull();
    off.unmount();
    open(game({ use_gamescope: true }));
    expect(await screen.findByLabelText('Refresh rate')).toBeTruthy();
  });

  it('keeps the environment rows, the preview and the save action outside the groups', async () => {
    open(game({ env_vars: { FOO: '1' } }));
    expect(screen.getByText('$ preview')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Save config' })).toBeTruthy();
    expect((screen.getByLabelText('Variable name') as HTMLInputElement).value).toBe('FOO');
    await screen.findByLabelText('Sync');
  });
});
