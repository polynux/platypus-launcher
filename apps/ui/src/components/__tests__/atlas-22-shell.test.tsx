// The /22 shell (issue #135): top bar, library bar, banners, status footer
// and friends side panel. One test per control: each renders under atlas and
// calls the right handler; the panel toggles with Escape; the view switch
// persists like the classic library mode.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render as rtlRender, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Effect } from 'effect';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Shell } from '../Shell';
import { TooltipProvider } from '../ds/tooltip';
import i18n from '../../lib/i18n';
import type { Game } from '../../types';

const shellState = vi.hoisted(() => ({ navigation: 'sourcebar' }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return shellState.navigation;
      if (axis === 'library.presentation') return 'list-detail';
      return 'page';
    },
    useThemeLayout: () => null,
    loadTheme: () => Promise.resolve(),
  };
});

expect.extend(matchers);
afterEach(() => { cleanup(); shellState.navigation = 'sourcebar'; });

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
  ownership: 'installed' as const,
  onOwnership: () => {},
  ownedCount: 10,
  storeSelection: [] as string[],
  onSelectStores: () => {},
  jobs: [],
  sessionStartedAt: {},
};

describe('/22 library bar controls (issue #135)', () => {
  it('switches ownership through All/Installed and clears the store selection', async () => {
    const user = userEvent.setup();
    const owned: string[] = [];
    const selected: string[][] = [];
    render(
      <Shell
        {...baseProps}
        view={{ name: 'library' }}
        onOwnership={(o) => owned.push(o)}
        onSelectStores={(s) => selected.push([...s])}
      />
    );
    await user.click(screen.getByRole('button', { name: /All games/ }));
    expect(owned).toEqual(['owned']);
    expect(selected).toEqual([[]]);
  });

  it('Every store clears a store selection', async () => {
    const user = userEvent.setup();
    const selected: string[][] = [];
    render(
      <Shell
        {...baseProps}
        view={{ name: 'library' }}
        storeCounts={{ steam: 4 }}
        storeSelection={['steam']}
        onSelectStores={(s) => selected.push([...s])}
      />
    );
    await user.click(screen.getByRole('button', { name: 'Every store' }));
    expect(selected).toEqual([[]]);
  });

  it('dots a store tab when its plugin needs attention', () => {
    render(
      <Shell
        {...baseProps}
        view={{ name: 'library' }}
        storeCounts={{ steam: 4 }}
        stores={[{ id: 'steam', name: 'Steam', store: 'steam', status: 'changed' } as never]}
      />
    );
    expect(screen.getByRole('button', { name: /Steam/ }).querySelector('[aria-label="needs attention"]')).toBeTruthy();
  });

  it('toggles Sideloaded when custom games exist', async () => {
    const user = userEvent.setup();
    const selected: string[][] = [];
    render(
      <Shell
        {...baseProps}
        view={{ name: 'library' }}
        storeCounts={{ custom: 2 }}
        storeSelection={[]}
        onSelectStores={(s) => selected.push([...s])}
      />
    );
    await user.click(screen.getByRole('button', { name: /Sideloaded/ }));
    expect(selected).toEqual([['custom']]);
  });

  it('opens the filters popover and writes through the tier', async () => {
    const user = userEvent.setup();
    const tiers: string[] = [];
    render(
      <Shell
        {...baseProps}
        view={{ name: 'library' }}
        tier="all"
        onTier={(v) => tiers.push(v)}
        statusFilter="all"
        onStatusFilter={() => {}}
      />
    );
    await user.click(screen.getByRole('button', { name: 'Library filters' }));
    expect(document.body.textContent).toContain('Compatibility');
    fireEvent.pointerDown(screen.getByLabelText('Filter by tier'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Platinum' }));
    expect(tiers).toEqual(['platinum']);
  });

  it('shows the lent row only when a family lends a game', async () => {
    const user = userEvent.setup();
    render(<Shell {...baseProps} view={{ name: 'library' }} hasBorrowed={false} />);
    await user.click(screen.getByRole('button', { name: 'Library filters' }));
    expect(document.body.textContent).not.toContain('Lent to me');
    cleanup();
    render(<Shell {...baseProps} view={{ name: 'library' }} hasBorrowed lending="all" onLending={() => {}} />);
    await user.click(screen.getByRole('button', { name: 'Library filters' }));
    expect(document.body.textContent).toContain('Lent to me');
  });

  it('switches the library view through list, covers and table', async () => {
    const user = userEvent.setup();
    const seen: string[] = [];
    render(<Shell {...baseProps} view={{ name: 'library' }} atlasView="list" onAtlasView={(v) => seen.push(v)} />);
    expect(screen.getByRole('button', { name: 'List' })).toHaveAttribute('aria-pressed', 'true');
    await user.click(screen.getByRole('button', { name: 'Covers' }));
    await user.click(screen.getByRole('button', { name: 'Table' }));
    expect(seen).toEqual(['grid', 'table']);
  });

  it('marks the avatar active while the panel is open', () => {
    render(<Shell {...baseProps} view={{ name: 'library' }} friendsOpen friendsUnread={2} />);
    const avatar = screen.getByRole('button', { name: /Friends, 2 unread/ });
    expect(avatar).toHaveAttribute('aria-expanded', 'true');
  });
});

const h = vi.hoisted(() => ({ api: {} as Record<string, ReturnType<typeof vi.fn>> }));

vi.mock('../../effect/runtime', () => {
  const pick = (name: string) => h.api[name] ?? (() => Effect.succeed(null));
  return {
    LauncherApi: new Proxy({}, { get: (_t, name: string) => pick(name as string) }),
    onJobs: () => () => {},
    onLibrary: () => () => {},
    onLibraryPartial: () => () => {},
    onPlaytime: () => () => {},
    onSession: () => () => {},
    onSocial: () => () => {},
    onChat: () => () => {},
    onCloud: () => () => {},
    onFamily: () => () => {},
  };
});

const ok = <T,>(v: T) => vi.fn(() => Effect.succeed(v));

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({
    id, source: 'steam', store: 'steam', store_app_id: id, title: id,
    cover_url: '', banner_url: '', hero_url: '', logo_url: '',
    install_dir: '/games', executable: 'game.exe', is_installed: true,
    settings: {}, last_played_at: 1700000000, shared_by: null,
    ...o,
  }) as Game;

function mockApp(games: Game[] = []) {
  for (const k of Object.keys(h.api)) delete h.api[k];
  Object.assign(h.api, {
    socialViews: ok([]),
    chatUnread: ok([]),
    chatAlerts: ok({}),
    getLibrary: ok({ games, warnings: [] }),
    getProtonVersions: ok([]),
    listPlugins: ok([]),
    getGameSettings: ok({}),
    getSessions: ok([]),
    getTweaks: ok([]),
    listCollections: ok([]),
    listJobs: ok([]),
    getLauncherSettings: ok(null),
    migrationNotice: ok(null),
  });
}

describe('/22 friends panel and banners in App (issue #135)', () => {
  it('toggles the side panel from the avatar and closes it with Escape', async () => {
    const user = userEvent.setup();
    const { App } = await import('../../App');
    mockApp([mkGame('alpha', { title: 'Alpha' })]);
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Library' }));
    await screen.findByRole('button', { name: /Installed/ });
    expect(screen.queryByRole('complementary', { name: 'Friends' })).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Friends' }));
    const panel = await screen.findByRole('complementary', { name: 'Friends' });
    expect(panel.textContent).toMatch(/Friends/);
    // No modal for Atlas: the dialog stays shut.
    expect(screen.queryByRole('dialog')).toBeNull();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Friends' })).toBeNull());
  });

  it('shows the migration banner between the top bar and the library bar', async () => {
    const user = userEvent.setup();
    const { App } = await import('../../App');
    mockApp([mkGame('alpha', { title: 'Alpha' })]);
    h.api.migrationNotice = ok({ moved: true, copied: false, set_aside: ['echo'], warnings: [] });
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Library' }));
    const banner = await screen.findByRole('status', { name: /hydra to platypus/i });
    const bar = document.querySelector('[data-region="sourcebar"]');
    expect(bar?.contains(banner)).toBe(true);
    const libBar = bar?.querySelector('nav[aria-label="Library"]');
    expect(libBar).toBeTruthy();
    // The banner precedes the library bar in document order.
    expect(banner.compareDocumentPosition(libBar!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows the classic grid under Atlas and keeps it across navigation', async () => {
    const user = userEvent.setup();
    const { App } = await import('../../App');
    mockApp([mkGame('alpha', { title: 'Alpha' }), mkGame('beta', { title: 'Beta' })]);
    render(<App />);
    await user.click(await screen.findByRole('button', { name: 'Library' }));
    await screen.findByRole('button', { name: /Installed/ });
    // List-detail first: the filter field, not the classic grid search.
    expect(screen.getByPlaceholderText('Filter')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Covers' }));
    expect(await screen.findByPlaceholderText('filter…')).toBeInTheDocument();
    // The choice persists like the classic library mode.
    await user.click(screen.getByRole('button', { name: 'Dashboard' }));
    await user.click(screen.getByRole('button', { name: 'Library' }));
    expect(screen.getByPlaceholderText('filter…')).toBeInTheDocument();
    await i18n.changeLanguage('en');
  });
});
