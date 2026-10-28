// Issue #146: the Runners page's default picker is the core's setting, in both
// presentations (the Atlas /22 page and the classic rows), and both can go back
// to the automatic pick.
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { ProtonVersion } from '../../types';
import { SystemView } from '../SystemView';

// shell.navigation decides the presentation; every other axis keeps its own.
const variant = vi.hoisted(() => ({ nav: 'sourcebar' }));
vi.mock('../../lib/theme', () => ({ useVariant: (axis: string) => (axis === 'shell.navigation' ? variant.nav : 'page') }));

// The stored default: what the page reads to tell a pick from the automatic one.
const core = vi.hoisted(() => ({ settings: { default_runner_id: '' } }));
vi.mock('../../effect/runtime', async () => {
  const { Effect } = await import('effect');
  return { LauncherApi: { getLauncherSettings: () => Effect.succeed(core.settings as never) } };
});

const runners = [
  { id: 'ge', name: 'GE-Proton10-4', path: '/runners/ge', is_system: false, is_custom: false, is_default: true },
  { id: 'proton9', name: 'Proton 9.0', path: '/runners/p9', is_system: false, is_custom: false, is_default: false },
] as ProtonVersion[];

const open = () => screen.getByRole('combobox');

describe('the default runner of Automatic games', () => {
  it('picks another runner from the Atlas page', () => {
    variant.nav = 'sourcebar';
    core.settings.default_runner_id = '';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    fireEvent.pointerDown(open(), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Proton 9.0' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('proton9');
  });

  it('stays read-only when the page has no way to save the pick', () => {
    variant.nav = 'sourcebar';
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} />);
    expect(open()).toBeDisabled();
  });

  it('offers the pick on the classic rows too', () => {
    variant.nav = 'page';
    core.settings.default_runner_id = '';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    // The row that is already the default says so and offers no button; the
    // way back is for a stored pick, and nothing is stored here.
    expect(screen.getByText('default')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Use the automatic pick' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Make default' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('proton9');
  });

  it('goes back to the automatic pick from the Atlas page', async () => {
    variant.nav = 'sourcebar';
    core.settings.default_runner_id = 'proton9';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    // The picker shows the stored pick, next to the automatic choice.
    await waitFor(() => expect(open()).toHaveTextContent('Proton 9.0'));
    fireEvent.pointerDown(open(), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Automatic (best installed)' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('');
  });

  it('goes back to the automatic pick from the classic page', async () => {
    variant.nav = 'page';
    core.settings.default_runner_id = 'proton9';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    fireEvent.click(await screen.findByRole('button', { name: 'Use the automatic pick' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('');
  });
});
