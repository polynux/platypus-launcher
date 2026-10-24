// Dashboard layouts (issue #96): the built-in arrangement and a cool-style
// themed one render the same blocks, and both pass axe.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { Dashboard } from '../Dashboard';
import { TooltipProvider } from '../ds/tooltip';
import type { ActivityEvent, Game, ThemeLayout } from '../../types';

const layoutState = vi.hoisted(() => ({ layout: null as ThemeLayout | null }));
vi.mock('../../effect/runtime', async () => {
  const { Effect } = await import('effect');
  return { LauncherApi: { getThemeLayout: () => Effect.succeed(layoutState.layout) } };
});

expect.extend(matchers);
afterEach(() => { cleanup(); layoutState.layout = null; });

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const mkEvents = (n: number): ActivityEvent[] =>
  Array.from({ length: n }, (_, i) => ({ id: i + 1, ts: Date.now(), type: 'download', detail: `event ${i + 1}` }) as ActivityEvent);

const dash = (games: Game[], events: ActivityEvent[] = []) =>
  render(<Dashboard games={games} runners={[]} activity={events} dismissedHero={new Set()} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} />);

// The cool theme's dashboard (internal/themes/builtin/cool/layouts/dashboard.xml):
// no welcome banner; continue and waiting share a row; activity moves to a
// side column with fewer events.
const coolLayout: ThemeLayout = {
  view: 'dashboard',
  regions: [
    {
      name: 'main',
      nodes: [
        { block: 'hero' },
        { block: 'empty' },
        { row: [{ block: 'continue' }, { block: 'waiting' }] },
        { block: 'recent' },
      ],
    },
    { name: 'side', nodes: [{ block: 'activity', attrs: { limit: '6' } }] },
  ],
};

const fix = { source: 'curated', tier: 'gold', description: 'Needs this fix' } as Game['fix_info'];

describe('dashboard layouts', () => {
  it('renders the built-in arrangement without a layout, with axe clean', async () => {
    const { container } = dash([mk('a'), mk('c', { store: 'custom', title: 'Gem' })], mkEvents(8));
    expect(await screen.findByText('Your games')).toBeTruthy();
    expect(screen.getByText('Waiting on you')).toBeTruthy();
    expect(screen.getByText('Recently added')).toBeTruthy();
    expect(screen.getByText('Activity')).toBeTruthy();
    // Eight events, the built-in limit.
    const activity = screen.getByText('Activity').closest('section');
    expect(activity?.querySelectorAll('ul li')).toHaveLength(8);
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the cool layout: no welcome banner, a row, activity in the side column', async () => {
    layoutState.layout = coolLayout;
    const { container } = dash([mk('a', { fix_info: fix }), mk('c', { store: 'custom', title: 'Gem' })], mkEvents(8));
    // The hero takes the welcome banner's place, in the main region.
    expect(await screen.findByRole('heading', { level: 1 })).toBeTruthy();
    expect(container.textContent).not.toContain('installed ·');
    const main = container.querySelector('[data-region="main"]');
    const side = container.querySelector('[data-region="side"]');
    expect(main).toBeTruthy();
    expect(side).toBeTruthy();
    // Continue and waiting share one row; recent stays below it.
    const row = main!.querySelector(':scope > div');
    expect(row?.getAttribute('style')).toContain('repeat(2,');
    expect(within(main as HTMLElement).getByText('Your games')).toBeTruthy();
    expect(within(main as HTMLElement).getByText('Waiting on you')).toBeTruthy();
    expect(within(main as HTMLElement).getByText('Recently added')).toBeTruthy();
    // Activity moved to the side column, with the layout's limit of six.
    expect(within(side as HTMLElement).getByText('Activity')).toBeTruthy();
    expect(side!.querySelectorAll('ul li')).toHaveLength(6);
    expect(main!.querySelectorAll('ul li')).toHaveLength(1);
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });

  it('renders the empty library without the layout, even when a theme ships one', async () => {
    layoutState.layout = coolLayout;
    const { container } = dash([]);
    expect(await screen.findByText('no titles found')).toBeTruthy();
    expect(container.querySelector('[data-region]')).toBeNull();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});
