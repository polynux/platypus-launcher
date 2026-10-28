// Storage layouts (issue #102): the built-in arrangement and a cool-style
// themed one render the same blocks, and both pass axe.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Effect } from 'effect';
import { TooltipProvider } from '../ds/tooltip';
import type { DiskUsage, Game, ThemeLayout } from '../../types';

const GB = 1e9;
const layoutState = vi.hoisted(() => ({ layout: null as ThemeLayout | null }));
const mock = vi.hoisted(() => ({
  subs: [] as ((d: unknown) => void)[],
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getDiskUsage: () => Effect.succeed(usage()),
    getGameFolders: () => Effect.succeed([{ path: '/mnt/a/Games', is_default: true, exists: true, free_bytes: 40 * GB }]),
    listActivity: () => Effect.succeed([] as never),
    listJobs: () => Effect.succeed([]),
    refreshDiskUsage: () => Effect.succeed(usage()),
    openGameFolder: () => Effect.succeed(undefined as never),
    scanGameSize: () => Effect.succeed(usage()),
    checkRemove: () => Effect.succeed({} as never),
    checkMove: () => Effect.succeed({} as never),
    pickMoveFolder: () => Effect.succeed(null as never),
    moveGame: () => Effect.succeed(undefined as never),
    getThemeLayout: () => Effect.succeed(layoutState.layout),
  },
  onDiskUsage: (cb: (d: unknown) => void) => {
    mock.subs.push(cb);
    return () => {};
  },
}));

import { StorageView } from '../StorageView';

expect.extend(matchers);
afterEach(() => { cleanup(); layoutState.layout = null; });

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

function usage(over: Partial<DiskUsage> = {}): DiskUsage {
  return {
    drives: [
      {
        id: '/mnt/a',
        mount: '/mnt/a',
        folders: ['/mnt/a/Games'],
        total_bytes: 100 * GB,
        free_bytes: 40 * GB,
        games_bytes: 30 * GB,
        other_bytes: 30 * GB,
      },
    ],
    games: [
      {
        game_id: 'g',
        title: 'Alpha',
        source: 'custom',
        store: 'custom',
        install_dir: '/mnt/a/Games/Alpha',
        size_bytes: 5 * GB,
        prefix_bytes: 1 * GB,
        from_plugin: false,
        measured: true,
      },
    ],
    scanning: false,
    generated_at: 0,
    ...over,
  };
}

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'custom', store: 'custom', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/mnt/a/Games/${id}`, executable: `/mnt/a/Games/${id}/run.sh`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const noop = () => {};
const view = (over: Partial<Parameters<typeof StorageView>[0]> = {}) =>
  render(<StorageView games={[mk('g', { title: 'Alpha' })]} onNotify={noop} onRemove={noop} onUninstall={noop} {...over} />);

// The cool theme's storage (internal/themes/builtin/cool/layouts/storage.xml):
// the header stays on top; the game list and the drives share a row, the
// drives in a narrower side column.
const coolLayout: ThemeLayout = {
  view: 'storage',
  regions: [
    { name: 'main', nodes: [{ block: 'header' }, { row: [{ block: 'games' }, { block: 'drives' }] }] },
  ],
};

describe('storage layouts', () => {
  it('renders the built-in arrangement without a layout, with axe clean', async () => {
    const { container } = view();
    expect(await screen.findByRole('progressbar')).toBeTruthy();
    expect(screen.getByText('games by size')).toBeTruthy();
    expect(screen.getByText('Alpha')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the cool layout: games and drives share a row', async () => {
    layoutState.layout = coolLayout;
    const { container } = view();
    expect(await screen.findByRole('progressbar')).toBeTruthy();
    const main = container.querySelector('[data-region="main"]');
    expect(main).toBeTruthy();
    // Games and drives share one row, the drives in the narrower column.
    const row = main!.querySelector(':scope > div:last-child');
    expect(row?.className).toContain('grid');
    expect(row?.className).toContain('xl:grid-cols-[1.75fr_1fr]');
    expect(within(main as HTMLElement).getByText('games by size')).toBeTruthy();
    expect(within(main as HTMLElement).getByText('Alpha')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the empty library through the built-in layout', async () => {
    layoutState.layout = null;
    const { container } = view({ games: [] });
    expect(await screen.findByText('no games to measure')).toBeTruthy();
    const main = container.querySelector('[data-region="main"]');
    expect(main).toBeTruthy();
    expect(within(main as HTMLElement).getByText('games by size')).toBeTruthy();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});
