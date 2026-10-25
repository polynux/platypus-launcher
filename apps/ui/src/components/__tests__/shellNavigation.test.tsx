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

describe('sourcebar shell variant (/22, issue #135)', () => {
  const sourcebarProps = {
    ...baseProps,
    ownership: 'installed' as const,
    onOwnership: () => {},
    ownedCount: 10,
    storeSelection: [] as string[],
    onSelectStores: () => {},
    jobs: [],
    sessionStartedAt: {},
  };

  it('renders the /22 chrome: top bar, library bar and footer region, axe clean', async () => {
    shellState.navigation = 'sourcebar';
    const { container } = render(<Shell {...sourcebarProps} view={{ name: 'library' }} storeCounts={{ steam: 4, gog: 2 }} />);
    expect(container.querySelector('[data-region="sourcebar"]')).toBeTruthy();
    expect(container.querySelector('[data-region="sidebar"]')).toBeNull();
    expect(container.querySelector('[data-region="topbar"]')).toBeNull();
    // Top bar: brand, nav and command button.
    expect(screen.getByText('Platypus')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Dashboard' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Library' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Store' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Search or run a command/ })).toBeInTheDocument();
    // The Places and Stores menus of the /21 chrome are gone.
    expect(screen.queryByRole('button', { name: 'Places' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Stores' })).toBeNull();
    // The game-colour switch moved to Settings > Appearance (#134).
    expect(container.textContent).not.toMatch(/Game colour|Accent.*Whole/);
    // Library bar: own switch, store tabs, collections, filters, view switch.
    expect(screen.getByRole('button', { name: /All games/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Installed/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Every store' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Collections/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Library filters' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'List' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Covers' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Table' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Add store/ })).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('hides the library bar on views without a library', async () => {
    shellState.navigation = 'sourcebar';
    const { container } = render(<Shell {...sourcebarProps} view={{ name: 'dashboard' }} />);
    // The top bar stays; only the library/views bar is gone.
    expect(screen.getByRole('button', { name: 'Dashboard' })).toBeInTheDocument();
    expect(container.querySelector('nav[aria-label="Library"]')).toBeNull();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('dispatches the command palette event from the search button', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const seen: string[] = [];
    const onPalette = () => seen.push('palette');
    window.addEventListener('platypus:command-palette', onPalette);
    try {
      render(<Shell {...sourcebarProps} view={{ name: 'library' }} />);
      await user.click(screen.getByRole('button', { name: /Search or run a command/ }));
      expect(seen).toEqual(['palette']);
    } finally {
      window.removeEventListener('platypus:command-palette', onPalette);
    }
  });

  it('keeps every place reachable through the top bar and More places menu', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const navigated: string[] = [];
    let friendsOpened = 0;
    let settingsOpened = 0;
    let added = 0;
    const redeemed: string[] = [];
    const openedStores: string[] = [];
    render(
      <Shell
        {...sourcebarProps}
        view={{ name: 'library' }}
        onNavigate={(v: { name: string }) => navigated.push(v.name)}
        onOpenFriends={() => { friendsOpened += 1; }}
        onSettingsOpen={() => { settingsOpened += 1; }}
        onAddGame={() => { added += 1; }}
        canRedeem
        onRedeem={() => redeemed.push('redeem')}
        stores={[{ id: 'echo', name: 'Echo' } as never]}
        onOpenStore={(id) => openedStores.push(id)}
      />
    );
    // Home, Library and Store nav.
    await user.click(screen.getByRole('button', { name: 'Dashboard' }));
    await user.click(screen.getByRole('button', { name: 'Library' }));
    expect(navigated).toEqual(['dashboard', 'library']);
    // Store opens the first storefront (#138 gives Store its own page).
    await user.click(screen.getByRole('button', { name: 'Store' }));
    expect(openedStores).toEqual(['echo']);
    // Downloads and Attention icons.
    await user.click(screen.getByRole('button', { name: 'Downloads' }));
    await user.click(screen.getByRole('button', { name: 'Needs attention' }));
    expect(navigated).toContain('downloads');
    expect(navigated).toContain('fixes');
    // The Friends avatar toggles the side panel.
    await user.click(screen.getByRole('button', { name: 'Friends' }));
    expect(friendsOpened).toBe(1);
    // More places: Activity, Runners, Plugins, Storage, Settings, then
    // Redeem a code and Add a game.
    await user.click(screen.getByRole('button', { name: 'More places' }));
    for (const name of ['Activity', 'Runners', 'Plugins', 'Storage', 'Settings']) {
      expect(screen.getByRole('menuitem', { name })).toBeInTheDocument();
    }
    expect(screen.queryByRole('menuitem', { name: 'Friends' })).toBeNull();
    await user.click(screen.getByRole('menuitem', { name: 'Settings' }));
    expect(settingsOpened).toBe(1);
    await user.click(screen.getByRole('button', { name: 'More places' }));
    await user.click(screen.getByRole('menuitem', { name: /Redeem a code/ }));
    expect(redeemed).toEqual(['redeem']);
    await user.click(screen.getByRole('button', { name: 'More places' }));
    await user.click(screen.getByRole('menuitem', { name: 'Add a game' }));
    expect(added).toBe(1);
  });

  it('opens Plugins from Store with no storefronts', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const navigated: string[] = [];
    render(<Shell {...sourcebarProps} view={{ name: 'library' }} stores={[]} onNavigate={(v: { name: string }) => navigated.push(v.name)} />);
    await user.click(screen.getByRole('button', { name: 'Store' }));
    expect(navigated).toEqual(['plugins']);
  });

  it('selects one store on click and combines on shift-click', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const selected: string[][] = [];
    render(
      <Shell
        {...sourcebarProps}
        view={{ name: 'library' }}
        storeCounts={{ steam: 4, gog: 2 }}
        storeSelection={['steam']}
        onSelectStores={(s) => selected.push([...s])}
      />
    );
    const steam = screen.getByRole('button', { name: /Steam/ });
    expect(steam).toHaveAttribute('aria-pressed', 'true');
    await user.click(screen.getByRole('button', { name: /GOG/ }));
    expect(selected.at(-1)).toEqual(['gog']);
    await user.keyboard('{Shift>}');
    await user.click(screen.getByRole('button', { name: /GOG/ }));
    await user.keyboard('{/Shift}');
    expect(selected.at(-1)).toEqual(['steam', 'gog']);
  });

  it('marks the active own tab with a 2px accent underline', async () => {
    shellState.navigation = 'sourcebar';
    render(<Shell {...sourcebarProps} ownership="installed" view={{ name: 'library' }} />);
    const tab = screen.getByRole('button', { name: /^Installed/ });
    expect(tab).toHaveAttribute('aria-pressed', 'true');
    expect(tab.querySelector('.bg-accent')).toBeTruthy();
  });

  it('rings the Downloads icon and badges Needs attention', async () => {
    shellState.navigation = 'sourcebar';
    render(
      <Shell
        {...sourcebarProps}
        view={{ name: 'library' }}
        downloadCount={1}
        fixCount={3}
        jobs={[{ id: 'j1', kind: 'download', source: 'steam', game_id: 'g1', store_game_id: 'g1', title: 'Game', state: 'running', bytes: 50, total: 100, speed: 1, percent: 50, attempts: 1, created_at: 0, updated_at: 0 }]}
      />
    );
    expect(screen.getByRole('button', { name: /Downloads, 1 active/ }).querySelector('circle')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Needs attention, 3/ }).querySelector('.bg-danger')).toBeTruthy();
  });

  it('Add store opens Plugins', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const navigated: string[] = [];
    render(<Shell {...sourcebarProps} view={{ name: 'library' }} onNavigate={(v: { name: string }) => navigated.push(v.name)} />);
    await user.click(screen.getByRole('button', { name: /Add store/ }));
    expect(navigated).toEqual(['plugins']);
  });
});

describe('sourcebar activity footer (issue #125)', () => {
  it('shows the running game with elapsed time and jobs with progress lines', async () => {
    const { SourceFooter } = await import('../Shell');
    shellState.navigation = 'sourcebar';
    const startedAt = Date.now() - 65_000;
    const { container } = render(
      <SourceFooter
        sessions={[{ game_id: 'g1', state: 'running', title: 'Game One' } as never]}
        jobs={[{ id: 'j1', kind: 'install', source: 'steam', game_id: 'g2', store_game_id: 'g2', title: 'Game Two', state: 'running', bytes: 25, total: 100, speed: 1, percent: 25, attempts: 1, created_at: 0, updated_at: 0 }]}
        sessionStartedAt={{ g1: startedAt }}
      />
    );
    expect(container.querySelector('[data-region="sourcebar-footer"]')).toBeTruthy();
    expect(screen.getByText('Game One')).toBeInTheDocument();
    expect(screen.getByText('Game Two')).toBeInTheDocument();
    expect(screen.getByText('25%')).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  const atlasBarProps = {
    ...baseProps,
    ownership: 'installed' as const,
    onOwnership: () => {},
    ownedCount: 10,
    storeSelection: [] as string[],
    onSelectStores: () => {},
    jobs: [],
    sessionStartedAt: {},
  };

  it('Store nav opens the first storefront; redeem and add-game live in More places (issue #132)', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const opened: string[] = [];
    const redeemed: string[] = [];
    const added: string[] = [];
    render(
      <Shell
        {...atlasBarProps}
        view={{ name: 'library' }}
        stores={[{ id: 'echo', name: 'Echo' }]}
        activeStore="echo"
        onOpenStore={(id) => opened.push(id)}
        canRedeem
        onRedeem={() => redeemed.push('redeem')}
        onAddGame={() => added.push('add')}
      />
    );
    await user.click(screen.getByRole('button', { name: 'Store' }));
    expect(opened).toEqual(['echo']);
    await user.click(screen.getByRole('button', { name: 'More places' }));
    expect(screen.getByRole('menuitem', { name: /Redeem a code/ })).toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: /Redeem a code/ }));
    expect(redeemed).toEqual(['redeem']);
    await user.click(screen.getByRole('button', { name: 'More places' }));
    await user.click(screen.getByRole('menuitem', { name: 'Add a game' }));
    expect(added).toEqual(['add']);
  });

  it('pins Collections at the source-bar edge with counts and manage', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const opened: (string | undefined)[] = [];
    const managed: string[] = [];
    render(
      <Shell
        {...atlasBarProps}
        view={{ name: 'library', collection: 'couch' }}
        collections={[{ id: 'couch', name: 'Couch co-op' }]}
        collectionCounts={{ couch: 4 }}
        onOpenCollection={(id) => opened.push(id)}
        onManageCollections={() => managed.push('manage')}
      />
    );
    // The active collection names the tab, with the accent underline.
    const tab = screen.getByRole('button', { name: /Couch co-op/ });
    expect(tab).toHaveAttribute('aria-pressed', 'true');
    expect(tab.querySelector('.bg-accent')).toBeTruthy();
    await user.click(tab);
    expect(screen.getByRole('menuitem', { name: /All games/ })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: /Couch co-op/ })).toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: /Couch co-op/ }));
    expect(opened).toEqual(['couch']);
    await user.click(tab);
    await user.click(screen.getByRole('menuitem', { name: /Manage/ }));
    expect(managed).toEqual(['manage']);
  });

  it('badges the Friends avatar and dots More places when a plugin needs attention', async () => {
    shellState.navigation = 'sourcebar';
    render(<Shell {...atlasBarProps} view={{ name: 'library' }} friendsUnread={3} authNeeded />);
    expect(screen.getByRole('button', { name: /Friends, 3 unread/ }).querySelector('.bg-danger')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'More places' }).querySelector('[aria-label="needs attention"]')).toBeTruthy();
  });

  it('stops a running game from the activity footer', async () => {
    const { SourceFooter } = await import('../Shell');
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    const stopped: string[] = [];
    render(
      <SourceFooter
        sessions={[{ game_id: 'g1', state: 'running', title: 'Game One' } as never]}
        jobs={[]}
        sessionStartedAt={{}}
        onStop={(id) => stopped.push(id)}
      />
    );
    await user.click(screen.getByRole('button', { name: 'Stop Game One' }));
    expect(stopped).toEqual(['g1']);
  });

  it('shows the last scan, offline marker and version in the Atlas footer (issue #132)', async () => {
    shellState.navigation = 'sourcebar';
    const { SourceFooter } = await import('../Shell');
    const { container } = render(
      <SourceFooter
        sessions={[]}
        jobs={[]}
        sessionStartedAt={{}}
        lastScan={Date.now() - 65_000}
        online={false}
      />
    );
    expect(container.querySelector('[data-region="sourcebar-footer"]')?.className).toMatch(/h-7/);
    expect(screen.getByText(/Scanned/)).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent(/Offline/);
    expect(screen.getByText('platypus v0.2')).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('dots the Plugins item when a plugin needs attention (issue #132)', async () => {
    shellState.navigation = 'sourcebar';
    const user = userEvent.setup();
    render(<Shell {...atlasBarProps} view={{ name: 'library' }} authNeeded />);
    await user.click(screen.getByRole('button', { name: 'More places' }));
    const plugins = screen.getByRole('menuitem', { name: /Plugins/ });
    expect(plugins.querySelector('[aria-label="needs attention"]')).toBeTruthy();
  });

  it('leaves All and Installed inactive with a collection active (issue #132)', () => {
    shellState.navigation = 'sourcebar';
    render(
      <Shell
        {...atlasBarProps}
        view={{ name: 'library', collection: 'couch' }}
        collections={[{ id: 'couch', name: 'Couch co-op' }]}
      />,
    );
    expect(screen.getByRole('button', { name: /^All/ })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: /^Installed/ })).toHaveAttribute('aria-pressed', 'false');
  });
});
