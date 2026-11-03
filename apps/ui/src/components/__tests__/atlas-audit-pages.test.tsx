// Atlas pages audit fixes (Refs #171, lane A): findings 1, 2, 5, 7, 8, 11,
// 12, 13, 15, 16, 17. Each test fails without its fix. Pure-styling findings
// (6, 10, 14, 18) and the store-bar layout need no jsdom test.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, within } from '@testing-library/react';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { DiskUsage, Game, Job, ProtonVersion, StorePlugin } from '../../types';

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

vi.mock('../../lib/notify', () => ({ notify: { success: vi.fn() } }));

import { AtlasHome } from '../AtlasHome';
import { AtlasDownloads } from '../AtlasDownloads';
import { AtlasRunners } from '../AtlasRunners';
import { AtlasAttention } from '../AtlasAttention';
import { AtlasDriveCard } from '../AtlasStorage';
import { AtlasPlugins } from '../AtlasPlugins';
import { FixesView } from '../FixesView';
import { Cover } from '../ui';
import { PluginLogo } from '../PluginLogo';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const mkJob = (o: Partial<Job> = {}): Job =>
  ({ id: 'j', game_id: 'g', title: 'Game', kind: 'install', state: 'failed', percent: 0, bytes: 0, total: 0, speed: 0, phase: '', updated_at: 1, attempts: 1, ...o }) as unknown as Job;

const homeProps = (over: Record<string, unknown> = {}) => ({
  games: [], allGames: [], activity: [], runners: [] as ProtonVersion[],
  dismissedHero: new Set<string>(), onDismissHero: () => {}, onOpenGame: () => {}, onNavigate: () => {}, ...over,
});

describe('finding 1: hero art fallback', () => {
  it('hides the hero image and keeps a dark backdrop when the game has no art', () => {
    const installed = mkGame('inst', { title: 'Installed', last_played_at: 10 });
    const suggestion = mkGame('backlog', { title: 'Backlog game', is_installed: false });
    const { container } = render(<AtlasHome {...homeProps({ games: [installed], allGames: [installed, suggestion] })} />);
    const hero = container.querySelector('section[aria-label="Backlog game"]')!;
    expect(hero).toBeTruthy();
    expect(hero.querySelector('img')).toBeNull();
    expect(hero.className).toMatch(/bg-hero-backdrop/);
  });

  it('renders the art when there is some', () => {
    const installed = mkGame('inst', { title: 'Installed', last_played_at: 10 });
    const suggestion = mkGame('backlog', { title: 'Backlog game', is_installed: false, hero_url: 'https://example.com/hero.jpg' });
    const { container } = render(<AtlasHome {...homeProps({ games: [installed], allGames: [installed, suggestion] })} />);
    const hero = container.querySelector('section[aria-label="Backlog game"]')!;
    expect(hero.querySelector('img')).toHaveAttribute('src', 'https://example.com/hero.jpg');
  });

  it('falls back to the dark backdrop when the art fails to load', () => {
    const installed = mkGame('inst', { title: 'Installed', last_played_at: 10 });
    const suggestion = mkGame('backlog', { title: 'Backlog game', is_installed: false, hero_url: 'https://example.com/404.jpg' });
    const { container } = render(<AtlasHome {...homeProps({ games: [installed], allGames: [installed, suggestion] })} />);
    const hero = container.querySelector('section[aria-label="Backlog game"]')!;
    fireEvent.error(hero.querySelector('img')!);
    // No fallback tile in the hero (no duplicated title): the dark section shows through.
    expect(hero.querySelector('.cover-fallback')).toBeNull();
    expect(within(hero as HTMLElement).getAllByText('Backlog game')).toHaveLength(1);
  });
});

describe('finding 2: home attention card follows the issue list', () => {
  it('lists an update-ready game the old filter missed, with its action and count', () => {
    const onJob = vi.fn();
    const g = mkGame('upd', { title: 'Updatable', update_available: true });
    render(<AtlasHome {...homeProps({ games: [g], allGames: [g], onJob })} />);
    expect(screen.queryByText('Everything works.')).toBeNull();
    expect(screen.getByText('See all 1')).toBeInTheDocument();
    expect(screen.getByText('An update is available.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Update now' }));
    expect(onJob).toHaveBeenCalledWith('update', expect.objectContaining({ id: 'upd' }));
  });

  it('retries a failed install from the card', () => {
    const onRetryJob = vi.fn();
    const g = mkGame('fail', { title: 'Failed game' });
    const job = mkJob({ id: 'job-1', game_id: 'fail', kind: 'install', state: 'failed', updated_at: 5 });
    render(<AtlasHome {...homeProps({ games: [g], allGames: [g], jobs: [job], onRetryJob })} />);
    expect(screen.getByText('See all 1')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry install' }));
    expect(onRetryJob).toHaveBeenCalledWith('job-1');
  });

  it('asks before resetting a prefix', () => {
    const onResetPrefix = vi.fn();
    const g = mkGame('pre', { title: 'Prefix game', prefix_damaged: true });
    render(<AtlasHome {...homeProps({ games: [g], allGames: [g], onResetPrefix })} />);
    fireEvent.click(screen.getByRole('button', { name: 'Reset prefix' }));
    expect(screen.getByText('Reset the Wine prefix?')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Reset prefix' }).at(-1)!);
    expect(onResetPrefix).toHaveBeenCalledWith(expect.objectContaining({ id: 'pre' }));
  });

  it('asks before allowing a game folder', () => {
    const onAllowFolder = vi.fn();
    const g = mkGame('out', { title: 'Outside', outside_folder: true, install_dir: '/tmp/out' });
    render(<AtlasHome {...homeProps({ games: [g], allGames: [g], onAllowFolder })} />);
    fireEvent.click(screen.getByRole('button', { name: 'Allow this folder' }));
    expect(screen.getByText('Allow this game folder?')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: 'Allow this folder' }).at(-1)!);
    expect(onAllowFolder).toHaveBeenCalledWith(expect.objectContaining({ id: 'out' }));
  });
});

describe('finding 5: card Play buttons match the hero button', () => {
  it('renders both as the accent fill, like /22 and the game page', () => {
    const played = mkGame('played', { title: 'Played', last_played_at: 10 });
    const backlog = mkGame('backlog', { title: 'Backlog', is_installed: false });
    render(<AtlasHome {...homeProps({ games: [played], allGames: [played, backlog] })} />);
    const heroInstall = screen.getByRole('button', { name: /Install/ });
    const cardPlay = screen.getByRole('button', { name: 'Play: Played' });
    for (const b of [heroInstall, cardPlay]) {
      expect(b.className).toMatch(/bg-accent(?!-strong)/);
      expect(b.className).not.toMatch(/bg-accent-strong/);
      expect(b.className).toMatch(/rounded-xl/);
    }
  });
});

describe('finding 7: downloads recent rows', () => {
  const dlProps = (jobs: Job[]) => ({
    jobs, games: [mkGame('civ', { title: 'Civ VII' }), mkGame('sub', { title: 'Subnautica' })],
    onAction: () => {}, onClear: () => {}, onOpenGame: () => {},
  });

  it('shows an empty track for a queued or paused job with no progress yet, a full pulse only while running', () => {
    const { container } = render(
      <AtlasDownloads {...dlProps([
        mkJob({ id: 'q', game_id: 'civ', state: 'queued', percent: -1 }),
        mkJob({ id: 'p', game_id: 'sub', state: 'paused', percent: -1 }),
        mkJob({ id: 'r', game_id: 'civ', state: 'running', percent: -1 }),
      ])} />,
    );
    const fills = [...container.querySelectorAll<HTMLElement>('[style*="width"]')].filter((el) => el.className.includes('rounded-[3px]'));
    expect(fills.map((el) => el.style.width)).toEqual(['0%', '0%', '100%']);
    expect(fills[2]!.className).toContain('animate-pulse');
    expect(fills[0]!.className).not.toContain('animate-pulse');
  });

  it('labels finished jobs by verb instead of "done"', () => {
    render(
      <AtlasDownloads {...dlProps([
        mkJob({ id: 'a', game_id: 'civ', kind: 'install', state: 'done' }),
        mkJob({ id: 'b', game_id: 'civ', kind: 'update', state: 'done' }),
        mkJob({ id: 'c', game_id: 'civ', kind: 'move', state: 'done' }),
        mkJob({ id: 'd', game_id: 'civ', kind: 'verify', state: 'done' }),
      ])} />,
    );
    expect(screen.getByText('Installed')).toBeInTheDocument();
    expect(screen.getByText('Updated')).toBeInTheDocument();
    expect(screen.getByText('Moved')).toBeInTheDocument();
    expect(screen.getByText('Verified')).toBeInTheDocument();
    expect(screen.queryByText('done')).toBeNull();
  });

  it('shows the failed label for the screenshot error, with the raw text in a tooltip', () => {
    const raw = 'size check: unknown game "bulk-0001"';
    const { container } = render(
      <AtlasDownloads {...dlProps([mkJob({ id: 'c', game_id: 'sub', kind: 'install', state: 'failed', error: raw })])} />,
    );
    // The engine string never shows in the narrow status column.
    expect(screen.queryByText(raw)).toBeNull();
    const status = screen.getByText('failed');
    expect(status.getAttribute('title')).toBe(raw);
  });

  it('aligns recent columns on one grid shared by every row', () => {
    const { container } = render(
      <AtlasDownloads {...dlProps([
        mkJob({ id: 'a', game_id: 'civ', kind: 'install', state: 'done' }),
        mkJob({ id: 'c', game_id: 'sub', kind: 'install', state: 'failed', error: 'No space left' }),
      ])} />,
    );
    const recent = screen.getByText('Recent').closest('section')!;
    // One template on the list, every row spans it: columns can't drift.
    expect(recent.querySelector('ul')?.className).toMatch(/grid-cols-\[/);
    for (const li of recent.querySelectorAll(':scope ul > li')) {
      expect(li.className).toMatch(/grid-cols-subgrid/);
    }
    expect(container.querySelector('li.grid-cols-\\[')).toBeNull();
  });
});

describe('finding 8: downloads empty states', () => {
  it('says something else than the empty title when there is nothing at all', () => {
    render(<AtlasDownloads jobs={[]} games={[]} onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />);
    expect(screen.getByText('Installs, updates and moves show up here.')).toBeInTheDocument();
    expect(screen.getAllByText('Nothing downloading')).toHaveLength(1);
  });

  it('shows a single muted line for an empty queue with history', () => {
    render(
      <AtlasDownloads
        jobs={[mkJob({ id: 'a', game_id: 'civ', kind: 'install', state: 'done' })]}
        games={[mkGame('civ', { title: 'Civ VII' })]}
        onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    expect(screen.getByText('Nothing in the queue.')).toBeInTheDocument();
    expect(screen.queryByText('Nothing downloading')).toBeNull();
    expect(screen.getByText('Recent')).toBeInTheDocument();
  });
});

describe('finding 11: plugin monograms', () => {
  it('renders the bare letter without a tint tile when there is no logo', () => {
    const { container } = render(<PluginLogo src="" name="Linux Runner" size="xs" bare />);
    expect(screen.getByText('L')).toBeInTheDocument();
    expect(container.querySelector('[class*="bg-"]')).toBeNull();
  });

  it('keeps the tinted tile by default', () => {
    render(<PluginLogo src="" name="Linux Runner" size="xs" />);
    expect(screen.getByText('L').className).toMatch(/bg-/);
  });

  it('shows a single tile in the Atlas plugin list', () => {
    const plugin = {
      id: 'facts', name: 'Linux Facts', version: '1', kind: 'store', status: 'disabled',
      capabilities: [], declared: [], install_folder: '/g',
    } as unknown as StorePlugin;
    const { container } = render(
      <AtlasPlugins
        plugins={[plugin]} games={[]} warnings={[]} auth={{}} busy={null}
        data={{ refresh: () => {}, setUninstalling: () => {} } as never}
        onNotify={() => {}} onAddStore={() => {}} />,
    );
    const option = screen.getByRole('option', { name: /Linux Facts/ });
    // The wrapper tile only: no nested monogram tile of its own.
    expect(option.querySelectorAll('.place-items-center')).toHaveLength(1);
    expect(container.querySelector('img')).toBeNull();
  });
});

describe('finding 12: compact cover fallback', () => {
  it('renders an initial instead of the full title', () => {
    const { container } = render(<Cover src={[]} title="Echo Slow" compact className="h-5 w-11" />);
    expect(screen.getByText('E')).toBeInTheDocument();
    expect(screen.queryByText('Echo Slow')).toBeNull();
    expect(container.firstChild?.textContent).toBe('E');
  });
});

describe('finding 13: plugins header order', () => {
  it('ends with the primary Add a store button with a Plus icon', () => {
    const plugin = {
      id: 'steam', name: 'Steam', version: '1', kind: 'store', status: 'disabled',
      capabilities: [], declared: [], install_folder: '/g',
    } as unknown as StorePlugin;
    render(
      <AtlasPlugins
        plugins={[plugin]} games={[]} warnings={[]} auth={{}} busy={null}
        data={{ refresh: () => {}, setUninstalling: () => {} } as never}
        onNotify={() => {}} onAddStore={() => {}} />,
    );
    const names = screen.getAllByRole('button').map((b) => b.textContent?.trim());
    const folder = names.findIndex((n) => n?.includes('Plugins folder'));
    const rescan = names.findIndex((n) => n === 'Rescan');
    const add = names.findIndex((n) => n?.includes('Add a store'));
    expect(folder).toBeGreaterThanOrEqual(0);
    expect(rescan).toBeGreaterThan(folder);
    expect(add).toBeGreaterThan(rescan);
    const addBtn = screen.getByRole('button', { name: 'Add a store' });
    expect(addBtn.className).toMatch(/bg-accent-strong/);
    expect(addBtn.querySelector('svg')).toBeTruthy();
  });
});

describe('finding 15: storage drive card', () => {
  const driveCard = (n: number) => {
    const GB = 1e9;
    const rows = Array.from({ length: n }, (_, i) => ({
      game_id: `g${i}`, title: `Game ${i}`, source: 'steam', store: 'steam',
      install_dir: `/mnt/a/Games/g${i}`, size_bytes: (n - i) * GB, prefix_bytes: 0,
      from_plugin: false, measured: true,
    })) as DiskUsage['games'];
    const drive = {
      id: '/mnt/a', mount: '/mnt/a', folders: ['/mnt/a/Games'], fs_type: 'ext4',
      device_kind: 'ssd', total_bytes: 200 * GB, free_bytes: 40 * GB, games_bytes: 100 * GB, other_bytes: 60 * GB,
    } as DiskUsage['drives'][number];
    const data = {
      byId: new Map(rows.map((r) => [r.game_id, mkGame(r.game_id, { title: r.title })])),
      moveId: null, removeId: null, uninstallId: null,
      setMoveId: () => {}, setRemoveId: () => {}, setUninstallId: () => {},
      measuring: new Set(), openFolder: () => {}, scan: () => {}, onNotify: () => {},
    } as never;
    return { drive, rows, data };
  };

  it('collapses to six rows and expands to reach every row action', () => {
    const { drive, rows, data } = driveCard(8);
    render(<AtlasDriveCard drive={drive} rows={rows} data={data} />);
    expect(screen.getByText('Show all 8')).toBeInTheDocument();
    expect(screen.queryByText('Game 6')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Show all 8' }));
    // Row 7's actions are reachable again after expanding.
    expect(screen.getByText('Game 6')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Uninstall Game 6' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Show less' }));
    expect(screen.queryByText('Game 6')).toBeNull();
  });
});

describe('finding 16: runners available rows', () => {
  it('leaves the middle column of available rows empty', () => {
    const runners: ProtonVersion[] = [
      { id: 'ge', name: 'GE-Proton10-4', path: '/runners/ge', is_system: false, is_custom: false, is_default: true },
    ];
    render(
      <AtlasRunners
        runners={runners} games={[]} onRefresh={() => {}} scanning={false} picked={null}
        available={[{ tag: 'ge-10-5', name: 'GE-Proton10-5', url: 'https://example.com', size_bytes: 1 }]}
        installing={null} removing={null} onInstall={() => {}} onRemove={() => {}} sizeLabel={() => '1 GB'} />,
    );
    // The installed runner with no games keeps its note; the available one says nothing.
    expect(screen.getAllByText('No game uses it')).toHaveLength(1);
  });
});

describe('finding 17: attention cards', () => {
  it('shows a friendly install sentence with the raw error behind Details', () => {
    const g = mkGame('fail', { title: 'Failed game' });
    render(
      <FixesView
        games={[g]}
        jobs={[mkJob({ id: 'j1', game_id: 'fail', kind: 'install', state: 'failed', error: 'size check: unknown game "bulk-0001"' })]}
        onOpenGame={() => {}} />,
    );
    expect(screen.getByText('The last attempt failed.')).toBeInTheDocument();
    expect(screen.queryByText('size check: unknown game "bulk-0001"')).toBeInTheDocument();
    expect(screen.getByText('Details')).toBeInTheDocument();
  });

  it('uses the wrench for a damaged prefix and 13px muted text', () => {
    const { container } = render(<FixesView games={[mkGame('broken', { title: 'Broken', prefix_damaged: true })]} onOpenGame={() => {}} />);
    expect(screen.getByText('Wine prefix damaged')).toBeInTheDocument();
    const desc = screen.getByText('The Wine prefix has no Windows side. The game may not start.');
    expect(desc.className).toMatch(/text-\[13px\]/);
    expect(desc.className).toMatch(/text-text-muted/);
    expect(container.querySelector('svg')).toBeTruthy();
  });

  it('AtlasAttention still opens the game from its card', () => {
    const onOpenGame = vi.fn();
    render(<AtlasAttention games={[mkGame('needy', { fix_info: { description: 'Needs tweaks' } } as Partial<Game>)]} onOpenGame={onOpenGame} />);
    fireEvent.click(screen.getByRole('button', { name: 'Open game' }));
    expect(onOpenGame).toHaveBeenCalledWith('needy');
  });
});
