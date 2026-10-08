import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: { previewCommand: () => Effect.succeed('cmd' as never) },
}));

import { SettingsModal } from '../SettingsModal';

const game = { id: 'g1', title: 'G', store: 'custom', source: 'custom', executable: '/x.exe', settings: { env_vars: {} } } as unknown as Game;

describe('env var rows', () => {
  it('adds two rows, deletes one, and saves only the remaining one', () => {
    const onSave = vi.fn();
    render(<SettingsModal game={game} runners={[]} onClose={() => {}} onSave={onSave} onCopied={() => {}} />);
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
