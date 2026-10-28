// Issue #138 (/22 pages under Atlas): every page renders with its data and
// keeps its actions. Atlas is shell.navigation = sourcebar; other themes keep
// their views (only the Atlas branch renders here).
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, act, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { ActivityEvent, DiskUsage, Game, Job, ProtonVersion, StorePlugin } from '../../types';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'settings.navigation') return 'scroll';
      if (axis === 'label.style') return 'plain';
      if (axis === 'section.chrome') return 'hairline';
      return 'page';
    },
    useThemeLayout: () => null,
    useColorScheme: () => 'dark',
    useThemeSchemes: () => ['dark'],
    loadTheme: () => Promise.resolve(),
  };
});

const mock = vi.hoisted(() => ({
  getDiskUsage: vi.fn(),
  getGameFolders: vi.fn(),
  listActivity: () => Effect.succeed([] as never),
  listJobs: vi.fn(),
  discardJob: vi.fn(),
  refreshDiskUsage: vi.fn(),
  openGameFolder: vi.fn(),
  scanGameSize: vi.fn(),
  listPlugins: vi.fn(),
  rescanPlugins: vi.fn(),
  authStatus: vi.fn(),
  approvePlugin: vi.fn(),
  getLauncherSettings: vi.fn(),
  listThemes: vi.fn(),
  socialViews: vi.fn(),
  subs: [] as ((d: unknown) => void)[],
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getDiskUsage: (...a: unknown[]) => mock.getDiskUsage(...a),
    getGameFolders: (...a: unknown[]) => mock.getGameFolders(...a),
    listJobs: (...a: unknown[]) => mock.listJobs(...a),
    discardJob: (...a: unknown[]) => mock.discardJob(...a),
    refreshDiskUsage: (...a: unknown[]) => mock.refreshDiskUsage(...a),
    openGameFolder: (...a: unknown[]) => mock.openGameFolder(...a),
    scanGameSize: (...a: unknown[]) => mock.scanGameSize(...a),
    listPlugins: (...a: unknown[]) => mock.listPlugins(...a),
    rescanPlugins: (...a: unknown[]) => mock.rescanPlugins(...a),
    authStatus: (...a: unknown[]) => mock.authStatus(...a),
    approvePlugin: (...a: unknown[]) => mock.approvePlugin(...a),
    getLauncherSettings: (...a: unknown[]) => mock.getLauncherSettings(...a),
    listThemes: (...a: unknown[]) => mock.listThemes(...a),
    socialViews: (...a: unknown[]) => mock.socialViews(...a),
  },
  onDiskUsage: (cb: (d: unknown) => void) => {
    mock.subs.push(cb);
    return () => {};
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onSocial: () => () => {},
  onChat: () => () => {},
  onAuth: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onLibrary: () => () => {},
  onLibraryPartial: () => () => {},
  onPlaytime: () => () => {},
}));

import { Dashboard } from '../Dashboard';
import { DownloadsView } from '../DownloadsView';
import { ActivityView } from '../ActivityView';
import { FixesView } from '../FixesView';
import { SystemView } from '../SystemView';
import { StorageView } from '../StorageView';
import { PluginsView } from '../PluginsView';
import { LauncherSettingsView } from '../LauncherSettingsView';
import { StoresPage } from '../StoresPage';
import { Shell } from '../Shell';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);
beforeEach(() => {
  mock.subs.length = 0;
  Object.values(mock).forEach((v) => { if (typeof v === 'function' && 'mockReset' in v) (v as unknown as { mockReset(): void }).mockReset(); });
  mock.getLauncherSettings.mockReturnValue(Effect.succeed({} as never));
  mock.listThemes.mockReturnValue(Effect.succeed([] as never));
  mock.getGameFolders.mockReturnValue(Effect.succeed([] as never));
  mock.listJobs.mockReturnValue(Effect.succeed([] as never));
  mock.discardJob.mockReturnValue(Effect.succeed(undefined as never));
  mock.socialViews.mockReturnValue(Effect.succeed([] as never));
});

const noop = () => {};

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const mkJob = (o: Partial<Job> = {}): Job =>
  ({ id: 'j', game_id: 'g', title: 'Game', kind: 'install', state: 'running', percent: 36, bytes: 100, total: 1000, speed: 0, phase: '', updated_at: Date.now(), attempts: 1, ...o }) as unknown as Job;

const GB = 1e9;
const usage = (over: Partial<DiskUsage> = {}): DiskUsage => ({
  drives: [{ id: '/mnt/a', mount: '/mnt/a', folders: ['/mnt/a/Games'], fs_type: '', device_kind: '', total_bytes: 100 * GB, free_bytes: 40 * GB, games_bytes: 30 * GB, other_bytes: 30 * GB }],
  games: [], scanning: false, generated_at: 0, ...over,
});
const sizeRow = (over: Partial<DiskUsage['games'][number]> = {}): DiskUsage['games'][number] => ({
  game_id: 'g', title: 'Alpha', source: 'steam', store: 'steam', install_dir: '/mnt/a/Games/Alpha', size_bytes: 5 * GB, prefix_bytes: 0, from_plugin: false, measured: true, ...over,
});

const mkPlugin = (o: Partial<StorePlugin> = {}): StorePlugin => ({
  id: 'steam', name: 'Steam', version: '1', author: '', description: '', store: 'steam', kind: 'store', status: 'ready',
  missing: [], requires: [], capabilities: [], declared: [], offers: [], needs: [], warnings: [], initialized: true,
  protocol: '1.1', protocol_versions: '1.0, 1.1', transport: 'jsonrpc-stdio', dir: '/x', exec: 'run.sh', hash: 'h',
  network: [], read: [], write: [], running: false, install_folder: '/g', default_install_folder: '/g', extra_folders: [], ...o,
}) as StorePlugin;

describe('a22 home (dashboard)', () => {
  it('uses install on the suggestion and shows Stop for a live game on Continue playing', () => {
    const installed = mkGame('live', { title: 'Live game', last_played_at: 10 });
    const backlog = mkGame('backlog', { is_installed: false, title: 'Backlog', installable: true });
    const onOpenGame = vi.fn();
    const onStop = vi.fn();
    const onInstall = vi.fn();
    render(
      <Dashboard
        games={[installed]} allGames={[installed, backlog]} runners={[]} activity={[]}
        dismissedHero={new Set()} onDismissHero={noop} onOpenGame={onOpenGame} onNavigate={noop}
        onInstall={onInstall} onStop={onStop}
        sessions={{ live: { game_id: 'live', state: 'running' } as never }}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /Install/ }));
    // Install goes through the install intent (the game page opens its dialog).
    expect(onInstall).toHaveBeenCalledWith(backlog);
    expect(screen.getByText('Live game is running.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Stop/ }));
    expect(onStop).toHaveBeenCalledWith('live');
  });

  it('renders the greeting, suggestion, continue cards, recent strip and waiting lists, and keeps dismiss/open', async () => {
    const attention = mkGame('needy', { fix_info: { description: 'Needs tweaks', source: 'Test', tier: 'bronze' }, playtime_minutes: 120, last_played_at: Date.now() - 3600e3 } as Partial<Game>);
    const other = mkGame('other', { playtime_minutes: 60 });
    const added = mkGame('sideloaded', { store: 'custom', source: 'custom' });
    const backlog = mkGame('backlog', { is_installed: false, source: '', store: 'steam' });
    const onDismissHero = vi.fn();
    const onOpenGame = vi.fn();
    render(
      <Dashboard
        games={[attention, other, added]}
        allGames={[attention, other, added, backlog]}
        runners={[]}
        activity={[]}
        dismissedHero={new Set()}
        onDismissHero={onDismissHero}
        onOpenGame={onOpenGame}
        onNavigate={noop}
      />,
    );
    expect(screen.getByRole('heading', { name: /Good morning|Good afternoon|Good evening|Up late/ })).toBeInTheDocument();
    // The suggestion is the first owned, not-installed game (the lab's
    // backlog pick), not the game needing attention.
    expect(screen.getByText('From your backlog')).toBeInTheDocument();
    expect(screen.getAllByText('backlog').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('Continue playing')).toBeInTheDocument();
    expect(screen.getByText('Recently added')).toBeInTheDocument();
    expect(screen.getByText('Waiting for you')).toBeInTheDocument();
    expect(screen.getByText('To finish')).toBeInTheDocument();
    expect(screen.getByText('To install')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: "Don't suggest this" }));
    expect(onDismissHero).toHaveBeenCalledWith(['backlog']);
    const plays = screen.getAllByRole('button', { name: /^Play: / });
    expect(plays.length).toBeGreaterThan(0);
    fireEvent.click(plays[0]);
    expect(onOpenGame).toHaveBeenCalled();
  });
});

describe('a22 downloads', () => {
  it('renders the queue cards and recent table, and keeps pause/cancel/retry', async () => {
    const onAction = vi.fn();
    render(
      <DownloadsView
        jobs={[
          mkJob({ id: 'a', game_id: 'civ', state: 'running', title: 'Civ VII' }),
          mkJob({ id: 'b', game_id: 'del', state: 'done', title: 'Deltarune', percent: 100 }),
          // The job title is the store title; the rows show the library title (R4-14).
          mkJob({ id: 'c', game_id: 'sub', state: 'failed', title: 'Store Subnautica', error: 'No space left' }),
        ]}
        games={[
          mkGame('civ', { title: 'Civ VII', hero_url: 'https://example.com/hero.jpg' }),
          mkGame('del', { title: 'Deltarune' }),
          mkGame('sub', { title: 'Subnautica' }),
        ]}
        onAction={onAction}
        onClear={noop}
        onOpenGame={noop}
      />,
    );
    expect(screen.getByText('Queue')).toBeInTheDocument();
    expect(screen.getByText('Recent')).toBeInTheDocument();
    expect(screen.getByText('Civ VII')).toBeInTheDocument();
    expect(screen.queryByText('Store Subnautica')).toBeNull();
    expect(screen.getByText('No space left')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Pause Civ VII' }));
    expect(onAction).toHaveBeenCalledWith('pause', 'a');
    fireEvent.click(screen.getByRole('button', { name: 'Cancel Civ VII' }));
    expect(onAction).toHaveBeenCalledWith('cancel', 'a');
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onAction).toHaveBeenCalledWith('retry', 'c');
  });
});

describe('a22 activity', () => {
  const events = (now: number): ActivityEvent[] => [
    { id: 1, ts: now - 1000, type: 'session', game: 'g', detail: 'Played for 1 h', meta: '' },
    { id: 2, ts: now - 2000, type: 'download', game: 'g', detail: 'Installed from Steam', meta: '' },
    { id: 3, ts: now - 3000, type: 'unlock', game: 'g', detail: 'Unlocked an achievement', meta: '' },
  ];
  it('renders day groups with kind chips and a game filter', async () => {
    render(<ActivityView events={events(Date.now())} games={[mkGame('g', { title: 'Hades' })]} />);
    expect(screen.getByText('Sessions')).toBeInTheDocument();
    expect(screen.getByText('Installs')).toBeInTheDocument();
    expect(screen.getAllByText('Hades').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Sessions', pressed: false } as never) as HTMLElement);
    expect(screen.getByText('Played for 1 h')).toBeInTheDocument();
    expect(screen.queryByText('Installed from Steam')).not.toBeInTheDocument();
  });
});

describe('a22 attention', () => {
  it('renders issue cards that open the game', () => {
    const onOpenGame = vi.fn();
    render(<FixesView games={[mkGame('needy', { fix_info: { description: 'Needs tweaks', source: 'Test', tier: 'bronze' } } as Partial<Game>)]} onOpenGame={onOpenGame} />);
    expect(screen.getByText('Needs attention')).toBeInTheDocument();
    expect(screen.getByText('Needs tweaks')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Open game' }));
    expect(onOpenGame).toHaveBeenCalledWith('needy');
  });
});

describe('a22 runners', () => {
  it('lists runners with the default badge and keeps rescan', async () => {
    const onRefresh = vi.fn();
    const runners: ProtonVersion[] = [{ id: 'ge', name: 'GE-Proton10-4', path: '/runners/ge', is_system: false, is_custom: false, is_default: true }];
    render(<SystemView runners={runners} onRefresh={onRefresh} scanning={false} />);
    expect(screen.getByText('Runners')).toBeInTheDocument();
    // No pick is stored (the settings mock answers {}): the picker says the
    // choice is automatic, and the row names the runner that renders.
    expect(await screen.findByText('Automatic (best installed)')).toBeInTheDocument();
    expect(screen.getAllByText('GE-Proton10-4')).toHaveLength(1);
    expect(screen.getByText('Default')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Rescan' }));
    expect(onRefresh).toHaveBeenCalled();
  });
});

describe('a22 storage', () => {
  it('renders drive cards and game rows, and keeps the uninstall flow', async () => {
    mock.getDiskUsage.mockReturnValue(Effect.succeed(usage({ games: [sizeRow()] })));
    const onUninstall = vi.fn();
    render(<StorageView games={[mkGame('g', { title: 'Alpha' })]} onNotify={noop} onRemove={noop} onUninstall={onUninstall} />);
    expect(await screen.findByRole('region', { name: '/mnt/a' })).toBeInTheDocument();
    expect(screen.getAllByText('Alpha').length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: 'Uninstall Alpha' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Uninstall' }));
    expect(onUninstall).toHaveBeenCalled();
  });
});

describe('a22 plugins', () => {
  it('lists store plugins with the selected detail, and keeps approve', async () => {
    mock.authStatus.mockReturnValue(Effect.succeed({ loggedIn: false } as never));
    mock.approvePlugin.mockReturnValue(Effect.succeed(undefined as never));
    render(
      <PluginsView
        plugins={[mkPlugin({ id: 'steam', name: 'Steam', status: 'unapproved' })]}
        warnings={[]}
        onPlugins={noop}
        onLibraryChanged={noop}
        onNotify={noop}
      />,
    );
    expect(await screen.findByText('Plugins')).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /Steam/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Review and approve/ }));
    fireEvent.click(await screen.findByRole('button', { name: 'Approve and enable' }));
    await waitFor(() => expect(mock.approvePlugin).toHaveBeenCalledWith('steam'));
  });
});

describe('a22 settings', () => {
  it('renders the lab category nav and switches bodies', async () => {
    render(<LauncherSettingsView onNotify={noop} plugins={[]} />);
    expect(await screen.findByText('General', { selector: 'button' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Appearance' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Game folders' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save backups' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ratings and game info' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Friends and chat' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Store clients' })).toBeInTheDocument();
    expect(screen.getByText('Language')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Friends and chat' }));
    expect(await screen.findByText('Chat notifications')).toBeInTheDocument();
    // Issues #150 and #149: ratings and protection are separate switches, and
    // the lab's Store clients category now has its client notices switch.
    fireEvent.click(screen.getByRole('button', { name: 'Ratings and game info' }));
    expect(await screen.findByRole('switch', { name: /Compatibility ratings/ })).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: /Anti-cheat and protection data/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Store clients' }));
    expect(await screen.findByRole('switch', { name: /Store client notices/ })).toBeInTheDocument();
  });
});

describe('a22 store page', () => {
  it('lists storefronts with tabs, opens the embedded view, and keeps redeem', () => {
    const onSelect = vi.fn();
    const onRedeem = vi.fn();
    const steam = mkPlugin({ id: 'steam', name: 'Steam', storefront_url: 'https://store.steampowered.com' });
    const gog = mkPlugin({ id: 'gog', name: 'GOG', store: 'gog', storefront_url: 'https://gog.com' });
    render(<StoresPage plugins={[steam, gog]} selected={steam} onSelect={onSelect} canRedeem onRedeem={onRedeem} onClose={noop} onNotify={noop} />);
    expect(screen.getByRole('tab', { name: 'Steam' })).toHaveAttribute('aria-selected', 'true');
    fireEvent.click(screen.getByRole('tab', { name: 'GOG' }));
    expect(onSelect).toHaveBeenCalledWith('gog');
    fireEvent.click(screen.getByRole('button', { name: /Redeem a code/ }));
    expect(onRedeem).toHaveBeenCalled();
  });
});

describe('a22 top bar', () => {
  it('labels the dashboard entry Home', () => {
    const props = {
      view: { name: 'dashboard' },
      onNavigate: noop,
      search: '',
      onSearch: noop,
      gameCount: 0,
      loading: false,
      downloadCount: 0,
      authNeeded: false,
      fixCount: 0,
      runnerCount: 0,
      sessions: [],
      friends: [],
      stores: [],
    } as unknown as Parameters<typeof Shell>[0];
    render(<Shell {...props} />);
    expect(screen.getByRole('button', { name: 'Home' })).toBeInTheDocument();
  });
});
