import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { Game, ProtonVersion } from '../../types';
import { SystemView } from '../SystemView';

vi.mock('../../lib/theme', () => ({
  useVariant: (axis: string) => {
    if (axis === 'runners.presentation') return 'table';
    if (axis === 'shell.navigation') return 'sourcebar';
    return 'sourcebar';
  },
}));

it('counts Auto games against the default runner', () => {
  const runners = [
    { id: 'default', name: 'Default Proton', is_default: true, path: '/r/default' },
    { id: 'other', name: 'Other Proton', path: '/r/other' },
  ] as ProtonVersion[];
  const games = [
    { id: 'auto', title: 'Auto game', native: false, settings: { runner_id: 'auto' } },
    { id: 'explicit', title: 'Explicit game', native: false, settings: { runner_id: 'other' } },
  ] as Game[];
  render(<SystemView runners={runners} games={games} onRefresh={() => {}} scanning={false} />);
  expect(screen.getAllByText('1 game')).toHaveLength(2);
});
