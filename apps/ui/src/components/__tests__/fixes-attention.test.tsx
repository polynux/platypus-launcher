// Issue #144 (Atlas): save conflicts and damaged prefixes show on Needs
// attention with an action that opens the game, where each is resolved.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent } from '@testing-library/react';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      return 'page';
    },
    useThemeLayout: () => null,
    useColorScheme: () => 'dark',
    useThemeSchemes: () => ['dark'],
    loadTheme: () => Promise.resolve(),
  };
});

import { FixesView } from '../FixesView';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'echo', store: 'echo', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

describe('atlas attention signals', () => {
  it('shows a save conflict with a compare action that opens the game', () => {
    const onOpenGame = vi.fn();
    render(<FixesView games={[mkGame('conflicted', { title: 'Conflicted', has_save_conflict: true })]} onOpenGame={onOpenGame} />);
    expect(screen.getByText('Save conflict')).toBeInTheDocument();
    expect(screen.getByText('Saves changed on this device and in the cloud.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Compare saves' }));
    expect(onOpenGame).toHaveBeenCalledWith('conflicted');
  });

  it('shows a damaged prefix with a reset action behind a confirm', () => {
    const onOpenGame = vi.fn();
    const onResetPrefix = vi.fn();
    render(<FixesView games={[mkGame('broken', { title: 'Broken', prefix_damaged: true })]} onOpenGame={onOpenGame} onResetPrefix={onResetPrefix} />);
    expect(screen.getByText('Wine prefix damaged')).toBeInTheDocument();
    expect(screen.getByText('The Wine prefix has no Windows side. The game may not start.')).toBeInTheDocument();
    // The primary action asks first; the ghost still opens the game.
    fireEvent.click(screen.getByRole('button', { name: 'Open game' }));
    expect(onOpenGame).toHaveBeenCalledWith('broken');
    expect(onResetPrefix).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    expect(screen.getByText('Reset the Wine prefix?')).toBeInTheDocument();
    expect(screen.getByText('The old prefix is kept as a backup next to the new one.')).toBeInTheDocument();
    // The modal hides the card from assistive tech: its confirm is the match.
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    expect(onResetPrefix).toHaveBeenCalledWith(expect.objectContaining({ id: 'broken' }));
    expect(onOpenGame).toHaveBeenCalledTimes(1);
  });

  it('counts both signals in the header', () => {
    render(<FixesView games={[
      mkGame('a', { has_save_conflict: true }),
      mkGame('b', { prefix_damaged: true }),
    ]} onOpenGame={() => {}} />);
    expect(screen.getByText("2 games can't start or need a decision.")).toBeInTheDocument();
  });
});

