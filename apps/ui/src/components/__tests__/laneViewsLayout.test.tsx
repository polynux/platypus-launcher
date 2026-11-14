// Atlas layout views (issue #116): each Atlas page renders through
// ThemeLayout, the built-in arrangement keeps today's structure, and a
// custom layout reorders or hides an optional block.
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { render as rtlRender, cleanup, screen, within } from '@testing-library/react';
import { axe } from 'vitest-axe';
import type { AxeCore } from 'vitest-axe';
import 'vitest-axe/extend-expect';
import * as matchers from 'vitest-axe/matchers';
import '../../lib/i18n';
import { TooltipProvider } from '../ds/tooltip';
import type { ActivityEvent, Game, Job, ProtonVersion, ThemeLayout } from '../../types';
import type { types as gen } from '../../gen/models';

const layoutState = vi.hoisted(() => ({ layouts: {} as Record<string, ThemeLayout | null> }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return { ...mod, useThemeLayout: (view: string) => layoutState.layouts[view] ?? null };
});

import { AtlasActivity } from '../AtlasActivity';
import { AtlasAttention } from '../AtlasAttention';
import { AtlasRunners } from '../AtlasRunners';
import { AtlasDownloads } from '../AtlasDownloads';

expect.extend(matchers);

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });
afterEach(() => { cleanup(); layoutState.layouts = {}; });

// vitest-axe registers its matcher at runtime; the static type doesn't know
// it, so name the assertion shape once instead of scattering casts.
const expectAxe = (result: AxeCore.AxeResults) =>
  expect(result) as unknown as { toHaveNoViolations(): void };

const mkGame = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: `/g/${id}`, is_installed: true, settings: {}, features: [], ...o }) as Game;

const mkJob = (o: Partial<Job> = {}): Job =>
  ({ id: 'j', game_id: 'g', title: 'Game', kind: 'install', state: 'failed', percent: 0, bytes: 0, total: 0, speed: 0, phase: '', updated_at: 1, attempts: 1, ...o }) as unknown as Job;

// The regions in DOM order.
function regions(container: HTMLElement): string[] {
  return [...container.querySelectorAll('[data-region]')].map((el) => el.getAttribute('data-region')!);
}

describe('activity layouts', () => {
  const events: ActivityEvent[] = [
    { id: 1, ts: Date.now(), type: 'session', game: 'alpha', detail: 'Played 20m' },
  ];
  const view = () => render(<AtlasActivity events={events} games={[mkGame('alpha', { title: 'Alpha' })]} onOpenGame={() => {}} />);

  it('renders the built-in arrangement: header, filter, day groups', async () => {
    const { container } = view();
    expect(regions(container)).toEqual(['top', 'main']);
    expect(container.firstElementChild?.className).toMatch(/max-w-\[1240px\]/);
    const order = [...container.querySelectorAll('h1, [role="group"], section[aria-label]')].map((el) => el.tagName);
    expect(order).toEqual(['H1', 'DIV', 'SECTION']);
    expect(screen.getByText('Activity')).toBeTruthy();
    expect(screen.getByRole('group', { name: 'Filter by kind' })).toBeTruthy();
    expect(screen.getByLabelText('Today')).toBeTruthy();
    expect(screen.getByText('Played 20m')).toBeTruthy();
    expectAxe(await axe(container));
  });

  it('hides the filter and reorders days first in a custom layout', () => {
    layoutState.layouts = {
      activity: { view: 'activity', regions: [{ name: 'main', nodes: [{ block: 'days' }, { block: 'filter' }] }] },
    };
    const { container } = render(<AtlasActivity events={events} games={[mkGame('alpha', { title: 'Alpha' })]} />);
    // The days block renders without the header region; the filter follows it.
    expect(regions(container)).toEqual(['main']);
    expect(screen.queryByRole('group', { name: 'Filter by kind' })).toBeTruthy();
    const main = container.querySelector('[data-region="main"]')!;
    const order = [...main.querySelectorAll('[role="group"], section[aria-label]')].map((el) => el.tagName);
    expect(order).toEqual(['SECTION', 'DIV']);
  });
});

describe('fixes layouts', () => {
  const g = () => mkGame('fail', { title: 'Failed game' });
  const view = () => render(
    <AtlasAttention
      games={[g()]} jobs={[mkJob({ id: 'j1', game_id: 'fail', kind: 'install', state: 'failed' })]}
      onOpenGame={() => {}} />,
  );

  it('renders the built-in arrangement: header, then cards', async () => {
    const { container } = view();
    expect(regions(container)).toEqual(['top', 'main']);
    expect(container.firstElementChild?.className).toMatch(/max-w-\[1240px\]/);
    const h1 = container.querySelector('h1')!;
    const card = container.querySelector('li')!;
    within(card).getByText('The last attempt failed.');
    expect(h1.textContent).toBe('Needs attention');
    expect(h1.compareDocumentPosition(card)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(screen.getByText('The last attempt failed.')).toBeTruthy();
    expectAxe(await axe(container));
  });

  it('hides the optional header in a custom layout', () => {
    layoutState.layouts = {
      fixes: { view: 'fixes', regions: [{ name: 'main', nodes: [{ block: 'cards' }] }] },
    };
    const { container } = view();
    expect(container.querySelector('h1')).toBeNull();
    expect(within(container.querySelector('li')!).getByText('The last attempt failed.')).toBeTruthy();
  });
});

describe('runners layouts', () => {
  const runners: ProtonVersion[] = [
    { id: 'compat:ge', name: 'GE-Proton10-4', path: '/r/ge', is_system: false, is_custom: false, is_default: true },
  ];
  const available: gen.AvailableRunner[] = [{ tag: 'ge-10-5', name: 'GE-Proton10-5', url: 'https://example.com', size_bytes: 1 }];
  const props = {
    runners, games: [], onRefresh: () => {}, scanning: false, picked: null,
    available, installing: null, removing: null, onInstall: () => {}, onRemove: () => {}, sizeLabel: () => '1 GB',
  };
  const view = () => render(<AtlasRunners {...props} />);

  it('renders the built-in arrangement: header, default, installed, available', async () => {
    const { container } = view();
    expect(regions(container)).toEqual(['top', 'main']);
    expect(container.firstElementChild?.className).toMatch(/max-w-\[1240px\]/);
    expect(screen.getByText('Runners')).toBeTruthy();
    expect(screen.getByRole('combobox')).toBeTruthy();
    const text = container.textContent ?? '';
    expect(text.indexOf('Default for Automatic games')).toBeLessThan(text.indexOf('Installed'));
    expect(text.indexOf('Installed')).toBeLessThan(text.indexOf('Available'));
    within(container.querySelector('section[aria-label="Installed"]') as HTMLElement).getByText('GE-Proton10-4');
    within(container.querySelector('section[aria-label="Available"]') as HTMLElement).getByText('GE-Proton10-5');
    expectAxe(await axe(container));
  });

  it('hides the optional default picker in a custom layout', () => {
    layoutState.layouts = {
      runners: {
        view: 'runners',
        regions: [
          { name: 'top', nodes: [{ block: 'header' }] },
          { name: 'main', nodes: [{ block: 'installed' }, { block: 'available' }] },
        ],
      },
    };
    const { container } = view();
    expect(container.querySelector('h1')!.textContent).toBe('Runners');
    expect(screen.queryByRole('combobox')).toBeNull();
    expect(screen.getByText('GE-Proton10-4')).toBeTruthy();
  });
});

describe('downloads layouts', () => {
  const jobs = () => [
    mkJob({ id: 'a', game_id: 'alpha', title: 'Alpha', kind: 'install', state: 'running', percent: 50 }),
    mkJob({ id: 'b', game_id: 'alpha', title: 'Alpha', kind: 'install', state: 'done', updated_at: 2 }),
  ];
  const view = (over: Partial<Job>[] = []) => render(
    <AtlasDownloads jobs={over.length ? over.map((o) => mkJob(o)) : jobs()} games={[mkGame('alpha', { title: 'Alpha' })]} onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
  );

  it('renders the built-in arrangement: header, queue, finished', async () => {
    const { container } = view();
    expect(regions(container)).toEqual(['top', 'main']);
    expect(container.firstElementChild?.className).toMatch(/max-w-\[1240px\]/);
    expect(screen.getByText('Downloads')).toBeTruthy();
    expect(screen.getByRole('progressbar')).toBeTruthy();
    const text = container.textContent ?? '';
    expect(text.indexOf('Queue')).toBeLessThan(text.indexOf('Recent'));
    expect(screen.getByText('Installed')).toBeTruthy();
    expectAxe(await axe(container));
  });

  it('hides the optional finished list in a custom layout', () => {
    layoutState.layouts = {
      downloads: {
        view: 'downloads',
        regions: [
          { name: 'top', nodes: [{ block: 'header' }] },
          { name: 'main', nodes: [{ block: 'queue' }] },
        ],
      },
    };
    const { container } = view();
    expect(screen.getByRole('progressbar')).toBeTruthy();
    expect(container.textContent).not.toMatch(/Recent/);
  });

  it('renders the empty state through the built-in layout', () => {
    const { container } = render(
      <AtlasDownloads jobs={[]} games={[]} onAction={() => {}} onClear={() => {}} onOpenGame={() => {}} />,
    );
    expect(regions(container)).toEqual(['top', 'main']);
    expect(screen.getByText('Nothing downloading')).toBeTruthy();
  });
});
