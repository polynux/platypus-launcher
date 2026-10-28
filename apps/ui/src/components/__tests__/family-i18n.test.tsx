import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { act, render, screen } from '@testing-library/react';
import { Effect } from 'effect';
import i18n from '../../lib/i18n';
import { FamiliesPanel } from '../FamiliesPanel';
import { FamilyWarningHost } from '../FamilyWarningHost';
import type { FamilyMember, FamilyWarning, Game, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({ getFamilyMembers: vi.fn() }));
const hooks = vi.hoisted(() => ({ onFamily: null as null | ((w: FamilyWarning) => void) }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getFamilyMembers: (source: string) => Effect.promise(() => api.getFamilyMembers(source)),
  },
  onFamily: (fn: (w: FamilyWarning) => void) => {
    hooks.onFamily = fn;
    return () => {};
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
}));

const plugin = { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['family'], declared: ['family'] } as unknown as StorePlugin;

const game = { id: 'p.echo.borrowed', title: 'Echo Borrowed Quest' } as Game;

const warning: FamilyWarning = {
  source: 'echo',
  game_id: 'p.echo.borrowed',
  message: 'Ana wants to play this game.',
  seconds_left: 42,
};

describe('family live language switching', () => {
  it('re-renders the families panel in French without a reload', async () => {
    const members: FamilyMember[] = [{ id: 'ana', name: 'Ana', role: 'adult', state: 'online' } as FamilyMember];
    api.getFamilyMembers.mockResolvedValue(members);
    render(<FamiliesPanel plugin={plugin} />);
    expect(await screen.findByText('families')).toBeInTheDocument();
    expect(await screen.findByText('adult')).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(await screen.findByText('familles')).toBeInTheDocument();
    expect(await screen.findByText('adulte')).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(await screen.findByText('families')).toBeInTheDocument();
  });

  it('re-renders the family warning toast in French without a reload', async () => {
    render(<FamilyWarningHost games={[game]} />);
    expect(screen.queryByRole('alert')).toBeNull();
    await act(async () => {
      hooks.onFamily?.(warning);
    });
    expect(screen.getByRole('alert')).toHaveTextContent('Ana wants to play this game.');
    expect(screen.getByText('42 s left')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dismiss' })).toBeInTheDocument();

    await i18n.changeLanguage('fr');
    expect(screen.getByText('42 s restantes')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ignorer' })).toBeInTheDocument();

    await i18n.changeLanguage('en');
    expect(screen.getByText('42 s left')).toBeInTheDocument();
  });
});
