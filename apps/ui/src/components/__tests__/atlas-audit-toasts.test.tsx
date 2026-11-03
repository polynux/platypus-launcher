// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
// Atlas audit pass 2, lane C (Refs #171): toasts, errors, small page items.
// Findings F1, F4, F5, F7, F8, F9, F12, F13, F14, F15, F16. Each bug test
// fails without its fix. Pure-styling findings need no jsdom layout (0x0),
// so they assert classes and file content, not positions.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within, fireEvent } from '@testing-library/react';
import '../../lib/i18n';
import i18n from '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { DiskUsage, Game, Job, ProtonVersion } from '../../types';
import fs from 'node:fs';
import path from 'node:path';

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'toast.style') return 'pill';
      if (axis === 'game.structure') return 'tabs';
      if (axis === 'shell.navigation') return 'sourcebar';
      if (axis === 'fixes.presentation') return 'cards';
      if (axis === 'storage.presentation') return 'cards';
      if (axis === 'downloads.presentation') return 'cards';
      return 'page';
    },
    useThemeLayout: () => null,
    useColorScheme: () => 'dark',
    useThemeSchemes: () => ['dark'],
    loadTheme: () => Promise.resolve(),
  };
});

vi.mock('../../lib/notify', () => ({ notify: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import { AtlasDownloads } from '../AtlasDownloads';
import { AtlasRunners } from '../AtlasRunners';
import { AtlasAttention } from '../AtlasAttention';
import { AtlasStorage, AtlasDriveCard } from '../AtlasStorage';
import { StripJob } from '../GameTabs';
import { GameOutsideAlert } from '../game/blocks';
import { Cover } from '../ui';
import { friendlyCoreError } from '../../lib/plainError';
import { isSystemFolder } from '../../lib/games';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'echo', store: 'gog', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const mkJob = (o: Partial<Job> = {}): Job =>
  ({ id: 'j', game_id: 'g', title: 'Game', kind: 'install', state: 'failed', percent: 0, bytes: 0, total: 0, speed: 0, phase: '', updated_at: 1, attempts: 1, ...o }) as unknown as Job;

const t = (key: string, opts?: Record<string, unknown>): string => i18n.t(key, opts as never) as unknown as string;

describe('F1: long toasts wrap instead of cutting to one line', () => {
  it('pill wraps up to 3 lines with a 10px radius, not nowrap+truncate', () => {
    const sonner = src('components/ds/sonner.tsx');
    // The pill variant (plain) is the one that truncated.
    expect(sonner).toContain('whitespace-normal');
    expect(sonner).toContain('line-clamp-3');
    expect(sonner).toContain('rounded-[10px]');
    expect(sonner).not.toMatch(/plain \? '[^']*whitespace-nowrap/);
    expect(sonner).not.toContain('[&_[data-title]]:truncate');
  });

  it('full text rides in title= for the message and string descriptions', () => {
    const notifySrc = src('lib/notify.ts');
    expect(notifySrc).toContain('title: message');
    expect(notifySrc).toContain('title: desc');
    const css = src('style.css');
    // The message itself clamps to 3 lines instead of truncating one.
    expect(css).toContain('.platypus-toast-message');
    expect(css).toContain('-webkit-line-clamp: 3');
  });
});

describe('F4: core errors become sentences, raw behind title/Details', () => {
  it('maps the three screenshot kinds, keeping plugin sentences readable', () => {
    expect(friendlyCoreError('refusing to allow system folder /usr/bin', t)).toBe(
      "Platypus can't launch games from system folders like /usr/bin",
    );
    expect(friendlyCoreError('not enough space in /tmp/atlas: needs 1 GB, 0 free', t)).toBe(
      'Not enough space on /tmp/atlas',
    );
    expect(friendlyCoreError('GE-Proton11-11: refusing tar entry "x" (size bomb?)', t)).toBe(
      'The download failed a safety check',
    );
    // Plugin-written sentences pass through (null): no widening.
    expect(friendlyCoreError('Echo Client is not installed', t)).toBeNull();
    expect(friendlyCoreError('size check: unknown game "bulk-0001"', t)).toBeNull();
  });

  it('status reads "Failed: …" with the raw engine string in the tooltip', () => {
    const raw = 'not enough space in /tmp/games/echo: needs 1 GB, 0 free';
    render(
      <AtlasDownloads
        jobs={[mkJob({ id: 'c', game_id: 'sub', kind: 'install', state: 'failed', error: raw })]}
        games={[mkGame('sub', { title: 'Subnautica' })]}
        onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    // Plugin copy never shows raw in the narrow column.
    expect(screen.queryByText(raw)).toBeNull();
    const status = screen.getByText('Failed: Not enough space on /tmp/games/echo');
    expect(status.getAttribute('title')).toBe(raw);
  });

  it('uses a wider status column and a 1fr name column', () => {
    const sonner = src('components/AtlasDownloads.tsx');
    expect(sonner).toContain('minmax(0,1fr)_minmax(0,1.2fr)');
    expect(sonner).not.toContain('minmax(0,1fr)_140px');
  });
});

describe('F7: light close button is readable', () => {
  it('paints the ghost close with the muted text token', () => {
    const css = src('style.css');
    expect(css).toContain('.platypus-toast-close');
    expect(css).toContain('color: var(--color-text-muted) !important');
  });
});

describe('F8: error pills differ from success pills', () => {
  it('marks error toasts with a danger edge in theme tokens', () => {
    const css = src('style.css');
    expect(css).toContain(".platypus-toast[data-type='error']");
    expect(css).toContain('inset 3px 0 0 var(--color-danger)');
  });
});

describe('F9: store toasts sit below the store bar', () => {
  it('offsets top-right toasts below the 54px bar when the store is open', () => {
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('top: 112');
    expect(sonner).not.toMatch(/plain && !storeOpen \? \{[^}]*\} : \{ top: 64/);
  });
});

describe('F12: storage order follows the shown sizes', () => {
  it('shows the sorted total (files + prefix), like Atlas', () => {
    const classic = src('components/StorageView.tsx');
    expect(classic).toContain('figure(row.size_bytes + row.prefix_bytes)');
  });

  it('orders drive rows biggest first even when the snapshot is not', () => {
    const GB = 1e9;
    const rows = [
      { game_id: 's', title: 'Small', source: 'steam', store: 'steam', install_dir: '/mnt/a/Games/Small', size_bytes: 1 * GB, prefix_bytes: 0, from_plugin: false, measured: true },
      { game_id: 'b', title: 'Big', source: 'steam', store: 'steam', install_dir: '/mnt/a/Games/Big', size_bytes: 7.8 * GB, prefix_bytes: 0, from_plugin: false, measured: true },
    ] as DiskUsage['games'];
    const drive = {
      id: '/mnt/a', mount: '/mnt/a', folders: ['/mnt/a/Games'], fs_type: '', device_kind: '',
      total_bytes: 100 * GB, free_bytes: 40 * GB, games_bytes: 30 * GB, other_bytes: 30 * GB,
    } as DiskUsage['drives'][number];
    const data = {
      byId: new Map([
        ['s', mkGame('s', { title: 'Small' })],
        ['b', mkGame('b', { title: 'Big' })],
      ]),
      moveId: null, removeId: null, uninstallId: null,
      setMoveId: () => {}, setRemoveId: () => {}, setUninstallId: () => {},
      measuring: new Set(), openFolder: () => {}, scan: () => {}, onNotify: () => {},
    } as never;
    render(<AtlasDriveCard drive={drive} rows={rows} data={data} />);
    const titles = screen.getAllByText(/Small|Big/).map((el) => el.textContent);
    // Big (7.8 GB) must come before Small even though the input listed Small first.
    expect(titles.indexOf('Big') < titles.indexOf('Small')).toBe(true);
  });
});

describe('F13: queued strip shows a static clock, not a spinner', () => {
  it('queued has no spin and an empty track; running without a figure pulses', () => {
    const onJobAction = () => {};
    const { container, rerender } = render(
      <TooltipProvider><StripJob job={mkJob({ id: 'q', state: 'queued', percent: -1 }) as never} onJobAction={onJobAction} /></TooltipProvider>,
    );
    expect(container.querySelector('.animate-spin')).toBeNull();
    const fills = [...container.querySelectorAll<HTMLElement>('[style*="width"]')];
    expect(fills.map((el) => el.style.width)).toContain('0%');
    // Running without a figure still pulses full width.
    rerender(<TooltipProvider><StripJob job={mkJob({ id: 'r', state: 'running', percent: -1 }) as never} onJobAction={onJobAction} /></TooltipProvider>);
    expect(container.querySelector('.animate-spin')).not.toBeNull();
  });
});

describe('F14: queue thumbnails use the compact monogram', () => {
  it('art-less queue rows show an initial, not the repeated title tile', () => {
    const game = mkGame('slow', { title: 'Echo Slow', cover_url: '', banner_url: '', hero_url: '' });
    render(
      <AtlasDownloads
        jobs={[mkJob({ id: 'q', game_id: 'slow', kind: 'install', state: 'queued', percent: -1, title: 'Echo Slow' })]}
        games={[game]}
        onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    const thumb = screen.getByRole('button', { name: 'Echo Slow' });
    // Compact monogram: a single initial in the 120x56 tile.
    expect(thumb.textContent).toBe('E');
    expect(thumb.textContent).not.toBe('Echo Slow');
  });
});

describe('F15: runner rows hover', () => {
  it('highlights rows on hover like activity rows', () => {
    const runnersSrc = src('components/AtlasRunners.tsx');
    expect(runnersSrc).toContain('hover:bg-surface');
    expect(runnersSrc).toContain('rounded-lg');
  });
});

describe('F16: no Allow for system folders', () => {
  it('knows the core system folders', () => {
    expect(isSystemFolder('/usr/bin')).toBe(true);
    expect(isSystemFolder('/usr/bin/wine')).toBe(true);
    expect(isSystemFolder('/')).toBe(true);
    expect(isSystemFolder('/tmp/out')).toBe(false);
    expect(isSystemFolder('/home/u/Games/Echo')).toBe(false);
  });

  it('game banner drops Allow for /usr/bin and says why', () => {
    const plugin = { id: 'echo', name: 'Echo', install_folder: '/g/echo' } as never;
    const game = mkGame('esc', { title: 'Echo Escape', install_dir: '/usr/bin', outside_folder: true });
    render(<GameOutsideAlert game={game} plugin={plugin} onAllowFolder={() => {}} />);
    expect(screen.queryByRole('button', { name: 'Allow this folder' })).toBeNull();
    expect(screen.getByText(/can't be allowed/)).toHaveTextContent('/usr/bin');
    // A normal outside folder still offers Allow.
    cleanup();
    const okGame = mkGame('out', { title: 'Outside', install_dir: '/tmp/out', outside_folder: true });
    render(<GameOutsideAlert game={okGame} plugin={plugin} onAllowFolder={() => {}} />);
    expect(screen.getByRole('button', { name: 'Allow this folder' })).toBeInTheDocument();
  });

  it('attention card opens the game instead of allowing a system folder', () => {
    const onOpenGame = vi.fn();
    const game = mkGame('esc', { title: 'Echo Escape', install_dir: '/usr/bin', outside_folder: true });
    render(<AtlasAttention games={[game]} onOpenGame={onOpenGame} />);
    expect(screen.getByText(/system folder Platypus can't allow/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Allow this folder' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Open game' }));
    expect(onOpenGame).toHaveBeenCalledWith('esc');
  });
});
