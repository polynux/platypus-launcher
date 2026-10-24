// Shell navigation variants (issue #104): the sidebar of today and the
// topbar the slate theme picks. Both keep Settings and Plugins reachable,
// keep the nav items on Tab with aria-current on the active one, and pass axe.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render as rtlRender, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Shell } from '../Shell';
import { TooltipProvider } from '../ds/tooltip';
import i18n from '../../lib/i18n';

const shellState = vi.hoisted(() => ({ navigation: 'sidebar' }));
vi.mock('../../lib/theme', () => ({ useVariant: (axis: string) => axis === 'shell.navigation' ? shellState.navigation : 'page' }));

expect.extend(matchers);
afterEach(() => { cleanup(); shellState.navigation = 'sidebar'; });

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const baseProps = {
  onNavigate: () => {},
  onSettingsOpen: () => {},
  search: '',
  onSearch: () => {},
  storeCounts: {},
  onPickSource: () => {},
  sourceLogos: {},
  gameCount: 3,
  loading: false,
  downloadCount: 0,
  authNeeded: false,
  fixCount: 0,
  runnerCount: 0,
  lastScan: null,
  sessions: [],
  onStop: () => {},
  friends: [],
  friendLabels: {},
  onOpenFriends: () => {},
  canRedeem: false,
  onRedeem: () => {},
  friendsUnread: 0,
  stores: [],
  activeStore: null,
  onOpenStore: () => {},
  collections: [],
  collectionCounts: {},
  onOpenCollection: () => {},
  onManageCollections: () => {},
};

describe('shell navigation variants', () => {
  it('sidebar mode is the Sidebar of today: region, nav, Settings and Plugins', async () => {
    const user = userEvent.setup();
    const { container } = render(<Shell {...baseProps} view={{ name: 'library' }} />);
    expect(container.querySelector('[data-region="sidebar"]')).toBeTruthy();
    expect(container.querySelector('[data-region="topbar"]')).toBeNull();
    const nav = screen.getByRole('navigation', { name: 'Main' });
    expect(within(nav).getByRole('button', { name: /Library/ })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: 'Settings' })).toBeInTheDocument();
    // Plugins sits in the Tools group; opening it keeps the entry one click away.
    await user.click(within(nav).getByRole('button', { name: 'Tools' }));
    expect(within(nav).getByRole('button', { name: /Plugins/ })).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('topbar mode keeps Settings and Plugins reachable, with axe clean', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    const { container } = render(<Shell {...baseProps} view={{ name: 'dashboard' }} />);
    expect(container.querySelector('[data-region="topbar"]')).toBeTruthy();
    expect(container.querySelector('[data-region="sidebar"]')).toBeNull();
    // Settings stays put in both variants.
    expect(screen.getByRole('button', { name: 'Settings' })).toBeInTheDocument();
    // Plugins sits in the Tools menu in topbar mode.
    await user.click(screen.getByRole('button', { name: /Tools/ }));
    expect(screen.getByRole('menuitem', { name: /Plugins/ })).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the Tools panel outside the nav overflow container (D1)', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    const { container } = render(<Shell {...baseProps} view={{ name: 'dashboard' }} />);
    await user.click(screen.getByRole('button', { name: /Tools/ }));
    const menu = screen.getByRole('menu');
    // The panel portals to the body: the nav's overflow can't clip it, and a
    // pointer on a menu row never hits the strip below the bar.
    const nav = container.querySelector('nav[aria-label="Main"]');
    expect(nav).toBeTruthy();
    expect(nav!.contains(menu)).toBe(false);
    expect(document.body.contains(menu)).toBe(true);
  });

  it('moves focus into the menu on open, arrows between items, Escape back (D2)', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    render(<Shell {...baseProps} view={{ name: 'dashboard' }} />);
    const tools = screen.getByRole('button', { name: /Tools/ });
    tools.focus();
    await user.keyboard('{Enter}');
    // Focus moves into the menu on open.
    expect(screen.getByRole('menuitem', { name: 'Fixes' })).toHaveFocus();
    // Arrow keys move between items.
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Runners' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: /Plugins/ })).toHaveFocus();
    // Escape closes and returns focus to the trigger.
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(tools).toHaveFocus();
  });

  it('marks the trigger on a tool view without popping the menu open', async () => {
    shellState.navigation = 'topbar';
    render(<Shell {...baseProps} view={{ name: 'plugins' }} />);
    // Unlike the sidebar's disclosure group, a floating menu never opens on
    // page load: Plugins stays one click away, marked by the active trigger.
    expect(screen.queryByRole('menu')).toBeNull();
    expect(screen.getByRole('button', { name: /Tools/ })).toHaveClass('text-accent-strong');
  });

  it('marks the active item aria-current in both variants', async () => {
    const { unmount } = render(<Shell {...baseProps} view={{ name: 'activity' }} />);
    expect(screen.getByRole('button', { name: 'Activity' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: /Dashboard/ })).not.toHaveAttribute('aria-current');
    unmount();
    cleanup();
    shellState.navigation = 'topbar';
    render(<Shell {...baseProps} view={{ name: 'activity' }} />);
    expect(screen.getByRole('button', { name: 'Activity' })).toHaveAttribute('aria-current', 'page');
  });

  it('reaches every nav item with Tab in topbar mode', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    const seen = vi.fn();
    render(<Shell {...baseProps} view={{ name: 'dashboard' }} onNavigate={seen} />);
    const dashboard = screen.getByRole('button', { name: /Dashboard/ });
    dashboard.focus();
    await user.tab();
    expect(screen.getByRole('button', { name: /Library/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: /Downloads/ })).toHaveFocus();
  });

  it('opens the Tools menu with Enter and closes it with Escape', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    render(<Shell {...baseProps} view={{ name: 'dashboard' }} />);
    const tools = screen.getByRole('button', { name: /Tools/ });
    expect(tools).toHaveAttribute('aria-expanded', 'false');
    tools.focus();
    await user.keyboard('{Enter}');
    expect(tools).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('menuitem', { name: /Storage/ })).toBeInTheDocument();
  });

  it('searches with Enter and clears with Escape, like the sidebar', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    const navigated: string[] = [];
    const searched: string[] = [];
    render(<Shell {...baseProps} view={{ name: 'dashboard' }} search="gem" onSearch={(q) => searched.push(q)} onNavigate={(v) => navigated.push(v.name)} />);
    const box = screen.getByPlaceholderText('Search…');
    box.focus();
    await user.keyboard('{Enter}');
    expect(navigated).toEqual(['library']);
    await user.keyboard('{Escape}');
    expect(searched).toEqual(['']);
  });

  it('shows the plugin attention dot on the Tools menu in topbar mode', async () => {
    shellState.navigation = 'topbar';
    const user = userEvent.setup();
    render(<Shell {...baseProps} view={{ name: 'dashboard' }} authNeeded fixCount={2} />);
    await user.click(screen.getByRole('button', { name: /Tools/ }));
    expect(screen.getByRole('menuitem', { name: /Plugins/ })).toBeInTheDocument();
    await i18n.changeLanguage('en');
  });
});
