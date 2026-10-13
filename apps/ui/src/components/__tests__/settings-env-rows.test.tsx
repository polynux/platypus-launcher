import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    previewCommand: () => Effect.succeed('cmd' as never),
    getLauncherSettings: () => Effect.succeed({ prefixes_folder: '', default_prefixes_folder: '/home/you/.local/share/platypus/prefixes' } as never),
    detectGameExecutable: () => Effect.succeed({ native: false, kind: 'windows-exe' } as never),
    isGameExecutable: () => Effect.succeed(true as never),
    pickGameExecutable: () => Effect.succeed('' as never),
    updateCustomGameExecutable: () => Effect.fail(new Error('not mocked') as never),
    makeGameExecutable: () => Effect.fail(new Error('not mocked') as never),
  },
}));

import { SettingsModal } from '../SettingsModal';

const game = { id: 'g1', title: 'G', store: 'custom', source: 'custom', executable: '/x.exe', settings: { env_vars: {} } } as unknown as Game;

describe('env var rows', () => {
  it('adds two rows, deletes one, and saves only the remaining one', () => {
    const onSave = vi.fn();
    render(<SettingsModal game={game} runners={[]} onClose={() => {}} onSave={onSave} onCopied={() => {}} />);
    // The env vars sit in the launch group's folded Advanced section.
    fireEvent.click(within(screen.getByRole('region', { name: 'launch' })).getByRole('button', { name: /Advanced/ }));
    const add = screen.getByRole('button', { name: '+ Add variable' });
    fireEvent.click(add);
    fireEvent.click(add);
    const keys = screen.getAllByLabelText('Variable name');
    expect(keys).toHaveLength(2);
    fireEvent.change(keys[0], { target: { value: 'FOO' } });
    fireEvent.change(keys[1], { target: { value: 'BAR' } });
    fireEvent.change(screen.getByLabelText('Value of FOO'), { target: { value: '1' } });
    fireEvent.change(screen.getByLabelText('Value of BAR'), { target: { value: '2' } });

    fireEvent.click(screen.getByRole('button', { name: 'Remove FOO' }));
    expect(screen.getAllByLabelText('Variable name')).toHaveLength(1);
    expect((screen.getByLabelText('Variable name') as HTMLInputElement).value).toBe('BAR');
    expect((screen.getByLabelText('Value of BAR') as HTMLInputElement).value).toBe('2');

    fireEvent.click(screen.getByRole('button', { name: 'Save config' }));
    expect(onSave.mock.calls[0][0].env_vars).toEqual({ BAR: '2' });
  });
});
