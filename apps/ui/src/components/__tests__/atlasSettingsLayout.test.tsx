// Atlas settings as layout blocks (issue #117): the category navigation
// follows the settings layout's order under both navigation modes, and a
// custom layout reorders the page with it.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import type { ThemeLayout } from '../../types';
import { BUILTIN_LAYOUTS } from '../../gen/builtinLayouts';

const layoutState = vi.hoisted(() => ({ navigation: 'tabs', layout: null as ThemeLayout | null }));
vi.mock('../../lib/theme', () => ({
  useVariant: (axis: string) => (axis === 'settings.navigation' ? layoutState.navigation : axis === 'settings-row.style' ? 'srow' : 'page'),
  useThemeLayout: (view: string) => layoutState.layout ?? (BUILTIN_LAYOUTS as Record<string, ThemeLayout>)[view],
}));
vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

import { AtlasSettingsPage } from '../AtlasLauncherSettings';

afterEach(() => { cleanup(); layoutState.navigation = 'tabs'; layoutState.layout = null; });

const NAMES = ['language', 'tray', 'theme', 'appearance', 'folders', 'prefixes', 'cleanup', 'backups', 'cloud', 'ratings', 'artwork', 'gameinfo', 'chat', 'notices'] as const;

const fixtureBlocks = (): Record<string, React.ReactNode> =>
  Object.fromEntries(NAMES.map((n) => [n, <section key={n}><h2>{n} heading</h2></section>]));

const navLabels = () =>
  screen.getAllByRole('button').map((b) => b.textContent);

describe('atlas settings layout (issue #117)', () => {
  it('tabs: the built-in layout shows the seven categories in Atlas order, General first', () => {
    render(<AtlasSettingsPage blocks={fixtureBlocks()} loadError={null} />);
    // The nav follows the layout's first-appearance order: today's Atlas order.
    expect(navLabels()).toEqual(['General', 'Appearance', 'Game folders', 'Save backups', 'Ratings and game info', 'Friends and chat', 'Store clients']);
    // General is the default tab: language and tray, in layout order.
    const body = screen.getByRole('region', { name: 'General' });
    expect(within(body).getByRole('heading', { name: 'General' })).toBeInTheDocument();
    const headings = within(body).getAllByRole('heading').map((h) => h.textContent);
    expect(headings).toEqual(['General', 'language heading', 'tray heading']);
    expect(screen.queryByText('theme heading')).not.toBeInTheDocument();
  });

  it('tabs: switching category shows its sections in layout order', () => {
    render(<AtlasSettingsPage blocks={fixtureBlocks()} loadError={null} />);
    fireEvent.click(screen.getByRole('button', { name: 'Ratings and game info' }));
    const body = screen.getByRole('region', { name: 'Ratings and game info' });
    const headings = within(body).getAllByRole('heading').map((h) => h.textContent);
    expect(headings).toEqual(['Ratings and game info', 'ratings heading', 'artwork heading', 'gameinfo heading']);
  });

  it('scroll: stacks every section in layout order and the nav jumps to categories', () => {
    layoutState.navigation = 'scroll';
    render(<AtlasSettingsPage blocks={fixtureBlocks()} loadError={null} />);
    const headings = screen.getAllByRole('heading').map((h) => h.textContent);
    expect(headings).toEqual(['Settings', ...NAMES.map((n) => `${n} heading`)]);
    // Same order, one nav.
    expect(navLabels()).toEqual(['General', 'Appearance', 'Game folders', 'Save backups', 'Ratings and game info', 'Friends and chat', 'Store clients']);
  });

  it('a custom layout reorders the navigation and the tabs with it', () => {
    layoutState.layout = {
      view: 'settings',
      regions: [{
        name: 'main',
        nodes: [
          { block: 'notices' },
          { row: [{ block: 'theme' }, { block: 'appearance' }] },
          ...NAMES.filter((n) => n !== 'notices' && n !== 'theme' && n !== 'appearance').map((n) => ({ block: n })),
        ],
      }],
    };
    render(<AtlasSettingsPage blocks={fixtureBlocks()} loadError={null} />);
    // Stores first (notices), then Appearance (theme), then General (language).
    expect(navLabels().slice(0, 3)).toEqual(['Store clients', 'Appearance', 'General']);
    const body = screen.getByRole('region', { name: 'Store clients' });
    expect(within(body).getByText('notices heading')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Appearance' }));
    // The theme/appearance row shares one grid frame, in layout order.
    const appearance = screen.getByRole('region', { name: 'Appearance' });
    const theme = within(appearance).getByText('theme heading');
    const appearanceHeading = within(appearance).getByText('appearance heading');
    expect(theme.compareDocumentPosition(appearanceHeading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(theme.closest('[class*="md:grid-cols-2"]')).not.toBeNull();
  });
});
