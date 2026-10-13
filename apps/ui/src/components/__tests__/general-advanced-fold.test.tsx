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

const game = (settings: object) =>
  ({ id: 'g1', title: 'G', store: 'custom', source: 'custom', executable: '/x.exe', settings: { env_vars: {}, ...settings } }) as unknown as Game;

const open = (g: Game) => render(<SettingsModal game={g} runners={[]} onClose={() => {}} onSave={() => {}} onCopied={() => {}} />);

describe('general settings groups', () => {
  it('folds the advanced settings, counting the ones that are set', () => {
    open(game({ sync_mode: 'esync', env_vars: { FOO: '1' } }));
    const runner = screen.getByRole('region', { name: 'runner' });
    const launch = screen.getByRole('region', { name: 'launch' });
    expect(within(runner).getByRole('button', { name: /Advanced.*1 set/ }).getAttribute('aria-expanded')).toBe('false');
    expect(within(launch).getByRole('button', { name: /Advanced.*1 set/ })).toBeTruthy();
    // Folded: the controls stay mounted but hidden until the fold opens.
    expect(within(launch).getByLabelText('Variable name').closest('[hidden]')).not.toBeNull();
    fireEvent.click(within(launch).getByRole('button', { name: /Advanced/ }));
    expect(within(launch).getByRole('button', { name: /Advanced/ }).getAttribute('aria-expanded')).toBe('true');
    expect((within(launch).getByLabelText('Variable name') as HTMLInputElement).value).toBe('FOO');
  });

  it('keeps the launch preview and the save action outside the groups', () => {
    open(game({}));
    expect(screen.getByText('$ preview')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Save config' })).toBeTruthy();
    expect(screen.getByRole('region', { name: 'graphics' })).toBeTruthy();
  });
});
