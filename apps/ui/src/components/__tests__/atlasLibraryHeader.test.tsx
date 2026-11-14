// Atlas library header from granular blocks (issue #118): the built-in
// layout reproduces today's grid/table header exactly, and a custom layout
// can hide the chips or move search to the end.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render as rtlRender, screen } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import type { Game, ThemeLayout } from '../../types';
import { VARIANT_DEFAULTS } from '../../gen/catalog';
import { BUILTIN_LAYOUTS } from '../../gen/builtinLayouts';

const layoutState = vi.hoisted(() => ({ layout: null as ThemeLayout | null }));
vi.mock('../../lib/theme', () => ({
  useVariant: (axis: string) => (VARIANT_DEFAULTS as unknown as Record<string, string>)[axis],
  useThemeLayout: (view: string) => layoutState.layout ?? (BUILTIN_LAYOUTS as Record<string, ThemeLayout>)[view],
}));
vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

import { AtlasLibraryPage } from '../AtlasLibraryViews';

afterEach(() => { cleanup(); layoutState.layout = null; });
const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({
    id, source: 'steam', store: 'steam', store_app_id: id, title: id,
    cover_url: '', banner_url: '', hero_url: '', install_dir: '/games',
    executable: 'game.exe', is_installed: true, settings: {},
    last_played_at: 1700000000, shared_by: null, ...o,
  }) as Game;

const pageProps = (extra: Record<string, unknown> = {}) => ({
  games: [mk('alpha', { title: 'Alpha' }), mk('beta', { title: 'Beta' })],
  ownership: 'owned' as const,
  source: 'all' as const,
  storeSelection: [] as string[],
  tier: 'all' as const,
  onTier: () => {},
  status: 'all' as const,
  onStatus: () => {},
  lending: 'all' as const,
  onLending: () => {},
  query: '',
  onQuery: () => {},
  warnings: [] as { source: string; needs_auth?: boolean; message?: string }[],
  pluginName: (id: string) => id,
  onOpenPlugins: () => {},
  jobs: [],
  runners: [],
  onOpen: () => {},
  ...extra,
});

describe('atlas library header (issue #118)', () => {
  it('built-in: search and sort share the bar row, warnings and chips stack below', () => {
    render(<AtlasLibraryPage {...pageProps({ query: 'alp', warnings: [{ source: 'epic', needs_auth: true }] })} view="grid" />);
    // Today's lib-bar row, kept by the bar region's frame.
    expect(document.querySelector('[data-region="bar"]')?.className).toBe('flex items-center gap-2');
    const input = screen.getByPlaceholderText('Filter the library');
    const sort = screen.getByRole('combobox', { name: 'Sort games' });
    expect(input.compareDocumentPosition(sort) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    // Warnings keep their status role, chips stack second, in order.
    const sub = [...(document.querySelector('[data-region="sub"]')?.children ?? [])];
    expect(sub).toHaveLength(2);
    expect(sub[0].getAttribute('role')).toBe('status');
    // No title, view-mode switch or filters popover: the shell carries them.
    expect(screen.queryByRole('heading', { name: 'Library' })).not.toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Library view' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Filters' })).not.toBeInTheDocument();
  });

  it('a custom layout hides the chips and keeps search, sort and warnings', () => {
    layoutState.layout = {
      view: 'library-header',
      regions: [
        { name: 'bar', nodes: [{ block: 'search' }, { block: 'sort' }] },
        { name: 'sub', nodes: [{ block: 'warnings' }] },
      ],
    };
    render(<AtlasLibraryPage {...pageProps({ query: 'alp', warnings: [{ source: 'epic', needs_auth: true }] })} view="grid" />);
    expect(screen.getByPlaceholderText('Filter the library')).toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
    // The active query would show chips in the built-in layout; hidden here.
    expect(document.querySelector('[data-region="sub"]')?.children).toHaveLength(1);
  });

  it('a custom layout moves search to the end of the bar', () => {
    layoutState.layout = {
      view: 'library-header',
      regions: [{ name: 'bar', nodes: [{ block: 'sort' }, { block: 'search' }] }],
    };
    render(<AtlasLibraryPage {...pageProps()} view="grid" />);
    const sort = screen.getByRole('combobox', { name: 'Sort games' });
    const input = screen.getByPlaceholderText('Filter the library');
    expect(sort.compareDocumentPosition(input) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('a custom layout adds the title and a working view-mode switch', () => {
    const onViewChange = vi.fn();
    layoutState.layout = {
      view: 'library-header',
      regions: [{ name: 'bar', nodes: [{ block: 'title' }, { block: 'mode' }, { block: 'search' }, { block: 'sort' }] }],
    };
    render(<AtlasLibraryPage {...pageProps({ onViewChange, lastScan: 1700000000 })} view="grid" />);
    expect(screen.getByRole('heading', { name: 'Library' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Covers' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'List' }));
    expect(onViewChange).toHaveBeenCalledWith('list');
  });
});
