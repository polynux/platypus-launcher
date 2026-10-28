// Issue #163: the classic Fixes page (warm, cool, slate) receives
// onResetPrefix and now offers it for a damaged prefix, behind the same
// confirm the Atlas page uses.
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
    // The classic themes: no source bar, so the rows below.
    useVariant: () => 'sidebar',
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
  ({ id, source: 'echo', store: 'echo', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], fix_info: { tier: 'gold', source: 'ProtonDB', description: 'Runs well' }, ...o }) as Game;

describe('classic fixes reset prefix', () => {
  it('offers Reset prefix for a damaged prefix, but not for native or store-client games', () => {
    const onResetPrefix = vi.fn();
    render(<FixesView games={[
      mkGame('broken', { title: 'Broken', prefix_damaged: true }),
      mkGame('native', { title: 'Native', prefix_damaged: true, native: true }),
      mkGame('client', { title: 'Client', prefix_damaged: true, client_launch: true }),
    ]} onOpenGame={() => {}} onResetPrefix={onResetPrefix} />);
    const resets = screen.getAllByRole('button', { name: 'Reset prefix' });
    expect(resets).toHaveLength(1);
    fireEvent.click(resets[0]);
    expect(screen.getByText('Reset the Wine prefix?')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Reset prefix' }).at(-1)!);
    expect(onResetPrefix).toHaveBeenCalledWith(expect.objectContaining({ id: 'broken' }));
  });
});
