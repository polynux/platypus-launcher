// Issue #146: the Runners page's default picker is the core's setting, in both
// presentations (the Atlas /22 page and the classic rows).
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import type { ProtonVersion } from '../../types';
import { SystemView } from '../SystemView';

// shell.navigation decides the presentation; every other axis keeps its own.
const variant = vi.hoisted(() => ({ nav: 'sourcebar' }));
vi.mock('../../lib/theme', () => ({ useVariant: (axis: string) => (axis === 'shell.navigation' ? variant.nav : 'page') }));

const runners = [
  { id: 'ge', name: 'GE-Proton10-4', path: '/runners/ge', is_system: false, is_custom: false, is_default: true },
  { id: 'proton9', name: 'Proton 9.0', path: '/runners/p9', is_system: false, is_custom: false, is_default: false },
] as ProtonVersion[];

describe('the default runner of new games', () => {
  it('picks another runner from the Atlas page', () => {
    variant.nav = 'sourcebar';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    fireEvent.pointerDown(screen.getByRole('combobox'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Proton 9.0' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('proton9');
  });

  it('stays read-only when the page has no way to save the pick', () => {
    variant.nav = 'sourcebar';
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} />);
    expect(screen.getByRole('combobox')).toBeDisabled();
  });

  it('offers the pick on the classic rows too', () => {
    variant.nav = 'page';
    const onDefaultRunner = vi.fn();
    render(<SystemView runners={runners} onRefresh={() => {}} scanning={false} onDefaultRunner={onDefaultRunner} />);
    // The row that is already the default says so and offers no button.
    expect(screen.getByText('default')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Make default' }));
    expect(onDefaultRunner).toHaveBeenCalledWith('proton9');
  });
});
