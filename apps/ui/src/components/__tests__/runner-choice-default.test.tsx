import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

// The runner's own schema: the automatic pick names the runner it runs with.
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    previewCommand: () => Effect.succeed('cmd' as never),
    getRunnerEnv: () => Effect.succeed({ env: {}, wrappers: [] } as never),
    getRunnerOptions: () => Effect.succeed([
      { id: 'runner', label: 'Runner', fields: [{ id: 'runner_id', kind: 'select', label: 'Runner', scope: 'game', default: 'auto',
        options: [{ id: 'auto', label: 'Automatic (uses FakeProton)' }, { id: 'proton-ge', label: 'GE-Proton 9' }] }] },
      { id: 'graphics', label: 'Graphics', fields: [{ id: 'dx_layer', kind: 'select', label: 'DX layer', scope: 'game', default: 'dxvk',
        options: [{ id: 'dxvk', label: 'DXVK (bundled)' }, { id: 'wined3d', label: 'WineD3D' }] }] },
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
  ({ id: 'g1', title: 'Echo sandbox-crash', store: 'custom', source: 'custom', executable: '/x.exe', settings: { env_vars: {}, ...settings } }) as unknown as Game;

const open = (g: Game) => render(<SettingsModal game={g} runners={[]} onClose={() => {}} onSave={() => {}} onCopied={() => {}} />);

describe('the runner choice of a game that never saved one', () => {
  it('shows the automatic pick, with no warning, when runner_id is unset', async () => {
    open(game({}));
    const select = await screen.findByLabelText('Runner');
    expect(select.textContent).toContain('Automatic (uses FakeProton)');
    expect(screen.queryByText(/Saved runner is not installed/)).toBeNull();
  });

  it('treats an empty runner_id the same as an unset one', async () => {
    open(game({ runner_id: '' }));
    const select = await screen.findByLabelText('Runner');
    expect(select.textContent).toContain('Automatic (uses FakeProton)');
    expect(screen.queryByText(/Saved runner is not installed/)).toBeNull();
  });

  it('warns only about a saved runner that is not installed', async () => {
    open(game({ runner_id: 'no-such-runner' }));
    expect(await screen.findByText(/Saved runner is not installed/)).toBeTruthy();
  });
});

describe('the headings of the runner panel', () => {
  it('shows each group title once: the core runner area does not repeat the runner group', async () => {
    open(game({}));
    await screen.findByLabelText('Runner');
    // The plugin's group title is the one heading; the field named "Runner" doesn't repeat it.
    expect(screen.getAllByText(/^runner$/i)).toHaveLength(1);
    expect(screen.getAllByText(/^graphics$/i)).toHaveLength(1);
    // The control keeps its name for screen readers.
    expect(within(screen.getByRole('region', { name: 'runner' })).getByLabelText('Runner')).toBeTruthy();
  });
});
