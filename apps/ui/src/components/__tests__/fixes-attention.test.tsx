// Issue #144 (Atlas): save conflicts and damaged prefixes show on Needs
// attention with an action that opens the game, where each is resolved.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent } from '@testing-library/react';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../lib/notify', () => ({ notify: { success: vi.fn() } }));
import { notify } from '../../lib/notify';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'dashboard.presentation') return 'home';
      if (axis === 'fixes.presentation') return 'cards';
      if (axis === 'activity.presentation') return 'days';
      if (axis === 'runners.presentation') return 'table';
      if (axis === 'downloads.presentation') return 'cards';
      if (axis === 'storage.presentation') return 'cards';
      if (axis === 'plugins.presentation') return 'directory';
      if (axis === 'friends.presentation') return 'panel';
      if (axis === 'store.presentation') return 'storefronts';
      if (axis === 'settings-row.style') return 'srow';
      if (axis === 'toast.style') return 'pill';
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

  it('asks before allowing a game folder', () => {
    const onAllowFolder = vi.fn();
    render(<FixesView games={[mkGame('outside', { title: 'Outside', outside_folder: true })]} onOpenGame={() => {}} onAllowFolder={onAllowFolder} />);
    fireEvent.click(screen.getByRole('button', { name: 'Allow this folder' }));
    expect(screen.getByText('Allow this game folder?')).toBeInTheDocument();
    expect(screen.getByText('Platypus will be allowed to launch games from /g/outside. This changes which folders can launch games.')).toBeInTheDocument();
    expect(onAllowFolder).not.toHaveBeenCalled();
    fireEvent.click(screen.getAllByRole('button', { name: 'Allow this folder' }).at(-1)!);
    expect(onAllowFolder).toHaveBeenCalledWith(expect.objectContaining({ id: 'outside' }));
  });

  it('uses a translated fallback when the outside folder is unknown', () => {
    render(<FixesView games={[mkGame('outside', { outside_folder: true, install_dir: '' })]} onOpenGame={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Allow this folder' }));
    expect(screen.getByText('Platypus will be allowed to launch games from this folder. This changes which folders can launch games.')).toBeInTheDocument();
  });

  it('offers an undo toast after choosing the default runner', async () => {
    const onSaveSettings = vi.fn(() => Promise.resolve());
    const game = mkGame('runner-game', { title: 'Runner game', settings: { runner_id: 'missing:runner' } as Game['settings'] });
    const runner = { id: 'system:proton', name: 'Proton', is_default: true } as never;
    render(<FixesView games={[game]} runners={[runner]} onOpenGame={() => {}} onSaveSettings={onSaveSettings} />);
    fireEvent.click(screen.getByRole('button', { name: 'Use Proton' }));
    await vi.waitFor(() => expect(notify.success).toHaveBeenCalledWith('Runner game now uses Proton.', expect.objectContaining({ action: expect.objectContaining({ label: 'Undo' }) })));
    expect(onSaveSettings).toHaveBeenCalledWith(game, 'system:proton', false);
    const options = vi.mocked(notify.success).mock.calls.at(-1)?.[1] as { action: { onClick: (event: React.MouseEvent<HTMLButtonElement>) => void } };
    options.action.onClick({} as React.MouseEvent<HTMLButtonElement>);
    expect(onSaveSettings).toHaveBeenNthCalledWith(2, game, 'missing:runner', false);
  });

  it('counts both signals in the header', () => {
    render(<FixesView games={[
      mkGame('a', { has_save_conflict: true }),
      mkGame('b', { prefix_damaged: true }),
    ]} onOpenGame={() => {}} />);
    expect(screen.getByText("2 games can't start or need a decision.")).toBeInTheDocument();
  });
});

