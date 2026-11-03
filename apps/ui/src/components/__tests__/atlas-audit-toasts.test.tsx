// @ts-nocheck — node fs/path guard test, outside the app's DOM tsconfig.
// Atlas audit pass 2, lane C (Refs #171): toasts, errors, small page items.
// Findings F1, F4, F5, F7, F8, F9, F12, F13, F14, F15, F16. Bug tests render
// DOM or unit-test pure helpers; CSS-only rules assert file content.
import React from 'react';
import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
import { render as rtlRender, cleanup, screen, fireEvent, act, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
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

const api = vi.hoisted(() => ({
  getGameFolders: vi.fn(),
  pickMoveFolder: vi.fn(),
  checkMove: vi.fn(),
  moveGame: vi.fn(),
}));

vi.mock('../../effect/runtime', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../effect/runtime')>();
  return {
    ...mod,
    LauncherApi: new Proxy({}, {
      get: (_t, k: string) => (...a: unknown[]) => {
        const fn = (api as Record<string, (...x: unknown[]) => unknown>)[k];
        if (fn) return fn(...a);
        throw new Error(`unmocked LauncherApi.${k}`);
      },
    }),
  };
});

import { Toaster } from '../ds/sonner';
import { notify as libNotify, __resetNotifyForTests } from '../../lib/notify';
import { AtlasDownloads } from '../AtlasDownloads';
import { AtlasRunners } from '../AtlasRunners';
import { AtlasAttention } from '../AtlasAttention';
import { AtlasDriveCard } from '../AtlasStorage';
import { StripJob } from '../GameTabs';
import { GameOutsideAlert } from '../game/blocks';
import { friendlyCoreError, errorToastParts } from '../../lib/plainError';
import { isSystemFolder, SYSTEM_DIRS, cleanPosix } from '../../lib/games';

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(cleanup);

const src = (p: string) => fs.readFileSync(path.join(__dirname, '../..', p), 'utf8');

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'echo', store: 'gog', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const mkJob = (o: Partial<Job> = {}): Job =>
  ({ id: 'j', game_id: 'g', title: 'Game', kind: 'install', state: 'failed', percent: 0, bytes: 0, total: 0, speed: 0, phase: '', updated_at: 1, attempts: 1, ...o }) as unknown as Job;

const tEn = (key: string, opts?: Record<string, unknown>): string => i18n.t(key, opts as never) as unknown as string;

beforeEach(() => {
  vi.useRealTimers();
  __resetNotifyForTests();
  Object.values(api).forEach((f) => (f as { mockReset: () => void }).mockReset());
  api.getGameFolders.mockReturnValue(Effect.succeed([{ path: '/home/games', is_default: true, exists: true, free_bytes: 30e9 }]) as never);
});

describe('F1: long toasts wrap instead of cutting to one line', () => {
  it('pill toast renders wrapped with full text in title=', async () => {
    render(<Toaster storeOpen={false} />);
    const long = `Echo Escape has a fresh prefix. The old one is kept at /tmp/atlas-kit/sbx-cap3-lw/home/prefixes/p.echo.escape.bak-1 which is the point of the message.`;
    act(() => {
      libNotify.success(long);
    });
    const msg = await screen.findByText(long.slice(0, 20), { exact: false });
    const toast = msg.closest('[data-sonner-toast]')!;
    // Computed classes: wrapping pill, not a one-line nowrap pill.
    expect(toast.className).toContain('whitespace-normal');
    expect(toast.className).toContain('rounded-[10px]');
    expect(toast.className).not.toContain('whitespace-nowrap');
    // Full text rides in title= so the 3-line clamp still reads on hover.
    const message = toast.querySelector('.platypus-toast-message')!;
    expect(message.getAttribute('title')).toBe(long);
  });
});

describe('F5: bottom-center pill is centred without breaking Sonner', () => {
  it('widens the list and centres with margins, no translateX override', () => {
    const css = src('style.css');
    expect(css).toContain("[data-sonner-toaster][data-x-position='center']");
    expect(css).toContain('--width: min(560px, calc(100vw - 2rem))');
    expect(css).toContain('margin-inline: auto');
    expect(css).toContain('width: fit-content');
    expect(css).not.toContain('translateX(-50%)');
  });
});

describe('F4: core errors become sentences, raw behind title/Details', () => {
  it('errorToastParts keeps the game title for multi-colon engine errors', () => {
    const raw = 'Echo Huge: not enough space in /tmp/x: needs 2 GB, 1 GB free';
    const parts = errorToastParts(raw, tEn, 'Something went wrong. Try again.');
    expect(parts.title).toBe('Echo Huge: Not enough space on /tmp/x');
    expect(parts.description).toBe(raw);
  });

  it('errorToastParts keeps the runner name for tar refusals', () => {
    const raw = 'GE-Proton11-11: refusing tar entry "x": unpacked data over 1 MB (size bomb?)';
    const parts = errorToastParts(raw, tEn, 'Something went wrong. Try again.');
    expect(parts.title).toBe('GE-Proton11-11: The download failed a safety check');
    expect(parts.description).toBe(raw);
  });

  it('captures paths with spaces up to the next colon', () => {
    const raw = 'Echo Huge: not enough space in /run/media/me/Games SSD/Echo: needs 2 GB, 1 GB free';
    const parts = errorToastParts(raw, tEn, 'Something went wrong. Try again.');
    expect(parts.title).toBe('Echo Huge: Not enough space on /run/media/me/Games SSD/Echo');
    expect(parts.description).toBe(raw);
  });

  it('skips strings that already have a catalog translation', () => {
    // English move.noSpace reads well already; only lowercase engine maps.
    const catalog = 'Not enough space in /tmp/x: the game needs 2 GB, 1 GB is free.';
    expect(friendlyCoreError(catalog, tEn)).toBeNull();
    const parts = errorToastParts(catalog, tEn, 'Something went wrong. Try again.');
    expect(parts.title).toBe(catalog);
    expect(parts.description).toBeUndefined();
    // Plugin-written sentences pass through (null): no widening.
    expect(friendlyCoreError('Echo Client is not installed', tEn)).toBeNull();
  });

  it('jobFailed keeps the title in French (space colon)', async () => {
    await act(async () => {
      await i18n.changeLanguage('fr');
    });
    try {
      const tFr = (key: string, opts?: Record<string, unknown>): string => i18n.t(key, opts as never) as unknown as string;
      const raw = 'Echo Huge : not enough space in /tmp/x: needs 2 GB, 1 GB free';
      const parts = errorToastParts(raw, tFr, 'Quelque chose a mal tourné.');
      expect(parts.title).toBe('Echo Huge : Pas assez de place sur /tmp/x');
      expect(parts.description).toBe(raw);
      const rawTar = 'GE-Proton11-11 : refusing tar entry "x"';
      const partsTar = errorToastParts(rawTar, tFr, 'Quelque chose a mal tourné.');
      expect(partsTar.title).toBe('GE-Proton11-11 : Le téléchargement a échoué à un contrôle de sécurité');
    } finally {
      await act(async () => {
        await i18n.changeLanguage('en');
      });
    }
  });

  it('status reads "Failed: …" with the raw engine string in the tooltip', () => {
    const raw = 'not enough space in /tmp/games/echo: needs 1 GB, 0 free';
    render(
      <AtlasDownloads
        jobs={[mkJob({ id: 'c', game_id: 'sub', kind: 'install', state: 'failed', error: raw })]}
        games={[mkGame('sub', { title: 'Subnautica' })]}
        onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    expect(screen.queryByText(raw)).toBeNull();
    const status = screen.getByText('Failed: Not enough space on /tmp/games/echo');
    expect(status.getAttribute('title')).toBe(raw);
  });

  it('a failed job with no error text reads "Failed", not "Failed: failed"', () => {
    render(
      <AtlasDownloads
        jobs={[mkJob({ id: 'c', game_id: 'sub', kind: 'install', state: 'failed', error: '' })]}
        games={[mkGame('sub', { title: 'Subnautica' })]}
        onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.queryByText('Failed: failed')).toBeNull();
  });
});

describe('F7: light close button is readable', () => {
  it('paints the ghost close with the muted text token and a hover lift', () => {
    const css = src('style.css');
    expect(css).toContain('.platypus-toast-close');
    expect(css).toContain('color: var(--color-text-muted) !important');
    expect(css).toContain('.platypus-toast-close:hover');
  });
});

describe('F8: error pills differ from success pills', () => {
  it('composes a danger inset shadow on the pill without dropping the ring', () => {
    const css = src('style.css');
    expect(css).toContain(".platypus-toast-pill[data-type='error']");
    expect(css).toContain('--tw-inset-shadow: inset 3px 0 0 var(--color-danger)');
    expect(css).not.toMatch(/\.platypus-toast\[data-type='error'\]\s*\{[^}]*box-shadow/);
  });
});

describe('F9: store toasts sit below the store bar, others stay put', () => {
  it('offsets only the store-open case to 112, keeping pill bottom and classic top', () => {
    const sonner = src('components/ds/sonner.tsx');
    expect(sonner).toContain('storeOpen ? { top: 112, right: 16 }');
    expect(sonner).toContain('plain ? { bottom: 40 } : { top: 64, right: 16 }');
  });
});

describe('F12: storage order follows the shown sizes', () => {
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
    expect(thumb.textContent).toBe('E');
    expect(thumb.textContent).not.toBe('Echo Slow');
  });
});

describe('F15: runner rows hover', () => {
  it('highlights rendered rows on hover without a rounding ring over the border', () => {
    const runners: ProtonVersion[] = [{ id: 'ge', name: 'GE-Proton10-4', path: '/runners/ge', is_system: false, is_custom: false, is_default: true } as ProtonVersion];
    const { container } = render(
      <AtlasRunners
        runners={runners} games={[]} onRefresh={() => {}} scanning={false} picked={null}
        available={[]} installing={null} removing={null} onInstall={() => {}} onRemove={() => {}} sizeLabel={() => '1 GB'} />,
    );
    const row = container.querySelector('li')!;
    expect(row.className).toContain('hover:bg-surface');
    expect(row.className).not.toContain('rounded-lg');
  });
});

describe('F16: no Allow for system folders', () => {
  it('mirrors the core systemDirs list and cleans POSIX paths', () => {
    const manager = src('../../../internal/plugins/manager.go');
    const m = manager.match(/systemDirs\s*=\s*\[\]string\{([^}]+)\}/);
    expect(m).not.toBeNull();
    const goList = [...m![1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
    expect([...SYSTEM_DIRS].sort()).toEqual([...goList].sort());
    expect(isSystemFolder('/usr/bin')).toBe(true);
    expect(isSystemFolder('/usr/bin/wine')).toBe(true);
    expect(isSystemFolder('/')).toBe(true);
    expect(isSystemFolder('//usr/bin')).toBe(true);
    expect(isSystemFolder('/usr//bin')).toBe(true);
    expect(isSystemFolder('/opt/../usr/bin')).toBe(true);
    expect(isSystemFolder('///')).toBe(true);
    expect(isSystemFolder('/usr/../home/me/Games/x')).toBe(false);
    expect(isSystemFolder('/tmp/out')).toBe(false);
    expect(isSystemFolder('/home/u/Games/Echo')).toBe(false);
    expect(isSystemFolder('/home/u', '/home/u')).toBe(true);
    expect(cleanPosix('/opt/../usr/bin')).toBe('/usr/bin');
  });

  it('game banner offers Move to Games for /usr/bin and says why', async () => {
    const plugin = { id: 'echo', name: 'Echo', install_folder: '/g/echo' } as never;
    const game = mkGame('esc', { title: 'Echo Escape', install_dir: '/usr/bin', outside_folder: true });
    render(<GameOutsideAlert game={game} plugin={plugin} onAllowFolder={() => {}} />);
    expect(screen.queryByRole('button', { name: 'Allow this folder' })).toBeNull();
    expect(screen.getByText(/can't be allowed/)).toHaveTextContent('/usr/bin');
    const moveBtn = screen.getByRole('button', { name: 'Move to Games' });
    fireEvent.click(moveBtn);
    await waitFor(() => expect(screen.getByText('Move Echo Escape')).toBeInTheDocument());
    cleanup();
    const okGame = mkGame('out', { title: 'Outside', install_dir: '/tmp/out', outside_folder: true });
    render(<GameOutsideAlert game={okGame} plugin={plugin} onAllowFolder={() => {}} />);
    expect(screen.getByRole('button', { name: 'Allow this folder' })).toBeInTheDocument();
  });

  it('attention card moves instead of allowing a system folder', async () => {
    const onOpenGame = vi.fn();
    const game = mkGame('esc', { title: 'Echo Escape', install_dir: '/usr/bin', outside_folder: true });
    render(<AtlasAttention games={[game]} onOpenGame={onOpenGame} />);
    expect(screen.getByText(/system folder Platypus can't allow/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Allow this folder' })).toBeNull();
    // Ghost action still opens the game.
    fireEvent.click(screen.getByRole('button', { name: 'Open game' }));
    expect(onOpenGame).toHaveBeenCalledWith('esc');
    // Primary moves via the Storage dialog.
    fireEvent.click(screen.getByRole('button', { name: 'Move to Games' }));
    await waitFor(() => expect(screen.getByText('Move Echo Escape')).toBeInTheDocument());
  });
});
