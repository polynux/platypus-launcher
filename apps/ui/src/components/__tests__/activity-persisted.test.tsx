import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender } from '@testing-library/react';
import { Dashboard, recentByAddedAt } from '../Dashboard';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../lib/theme', async () => {
  const actual = await vi.importActual<typeof import('../../lib/theme')>('../../lib/theme');
  return { ...actual, useVariant: () => 'cards' };
});

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const dash = (games: Game[]) =>
  render(<Dashboard games={games} runners={[]} activity={[]} dismissedHero={new Set()} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} />);

describe('persisted activity (issues #145, #151)', () => {
  it('sorts Recently added on added_at, newest first', () => {
    const games = [mk('old', { added_at: 1000, title: 'Old' }), mk('new', { added_at: 3000, title: 'New' }), mk('mid', { added_at: 2000, title: 'Mid' })];
    expect(recentByAddedAt(games, 5).map((g) => g.id)).toEqual(['new', 'mid', 'old']);
  });

  it('fills Recently added from added_at, not only sideloaded games', () => {
    // Without the change the classic strip only shows custom games, so a
    // steam-only library with added_at would hide Recently added.
    const { queryByText } = dash([mk('a', { added_at: 2000, title: 'Alpha' }), mk('b', { added_at: 1000, title: 'Beta' })]);
    expect(queryByText('Recently added')).toBeTruthy();
  });

  it('orders the classic strip newest first', () => {
    const { container } = dash([mk('a', { added_at: 1000, title: 'Alpha', store: 'custom' }), mk('b', { added_at: 3000, title: 'Beta', store: 'custom' })]);
    const titles = [...container.querySelectorAll('li button')].map((e) => e.textContent);
    const beta = titles.findIndex((t) => t?.includes('Beta'));
    const alpha = titles.findIndex((t) => t?.includes('Alpha'));
    expect(beta).toBeGreaterThanOrEqual(0);
    expect(alpha).toBeGreaterThanOrEqual(0);
    expect(beta).toBeLessThan(alpha);
  });
});
