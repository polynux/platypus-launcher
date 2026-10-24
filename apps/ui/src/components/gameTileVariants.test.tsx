import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup as rtlCleanup, render as rtlRender, screen as rtlScreen } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';

const state = vi.hoisted(() => ({ variants: {
  'game-tile.aspect': '3:4',
  'game-tile.compat': 'on',
  'game-tile.overlay': 'on',
  'game-tile.density': 'comfortable',
  'game-row.columns': 'full',
  'game-row.mode': 'full',
} as Record<string, string> }));
vi.mock('../lib/theme', () => ({ useVariant: (axis: string) => state.variants[axis] }));

import { GameRow, GameTile } from './GameTile';
import { WindowedGrid } from './WindowedList';
import type { Game, Job } from '../types';

expect.extend(matchers);
afterEach(() => { rtlCleanup(); });

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'steam-1', source: 'steam', store: 'steam', store_app_id: '1', title: 'Doom',
    cover_url: '', banner_url: '', install_dir: '/g/doom', executable: '/g/doom/doom.exe',
    is_installed: true, features: null, status: 'playing',
    settings: { runner_id: 'auto', env_vars: {}, use_gamemode: true },
    compat_badges: [{ kind: 'rating', label: 'Gold', tone: 'gold', source: 'ProtonDB', checked_at: 1 }],
    ...over,
  } as Game;
}

const job: Job = { id: 'j1', game_id: 'steam-1', kind: 'install', state: 'running', phase: 'download', percent: 42 } as Job;

describe('GameTile catalog variants', () => {
  for (const aspect of ['2:3', '3:4', '16:9']) {
    for (const compat of ['on', 'off']) {
      for (const overlay of ['on', 'off']) {
        for (const density of ['comfortable', 'compact']) {
          it(`renders tile ${aspect} / compat ${compat} / overlay ${overlay} / ${density} without axe violations`, async () => {
            state.variants = { ...state.variants, 'game-tile.aspect': aspect, 'game-tile.compat': compat, 'game-tile.overlay': overlay, 'game-tile.density': density };
            const { container } = rtlRender(<GameTile game={game()} job={job} meta="10h · test" coverBadges onOpen={() => {}} />);
            expect(rtlScreen.getByRole('button', { name: /Doom/ })).toBeInTheDocument();
            (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
          });
        }
      }
    }
  }

  it('renders the dashboard tile without badges or job without axe violations', async () => {
    state.variants = { ...state.variants, 'game-tile.aspect': '3:4', 'game-tile.compat': 'on', 'game-tile.overlay': 'on', 'game-tile.density': 'comfortable' };
    const { container } = rtlRender(<GameTile game={game({ status: '', is_installed: true })} meta="1h · auto" hideNative onOpen={() => {}} />);
    expect(rtlScreen.getByRole('button', { name: /Doom/ })).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});

describe('GameRow catalog variants', () => {
  for (const columns of ['full', 'minimal']) {
    for (const mode of ['full', 'compact']) {
      it(`renders row columns ${columns} / ${mode} without axe violations`, async () => {
        state.variants = { ...state.variants, 'game-row.columns': columns, 'game-row.mode': mode };
        const { container } = rtlRender(
          <div role="table" aria-label="Games">
            <GameRow game={game()} index={0} runners={[]} recency="today" onOpen={() => {}} />
          </div>,
        );
        expect(rtlScreen.getByRole('row', { name: /Doom/ })).toBeInTheDocument();
        (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
      });
    }
  }
});

describe('Virtualized game tiles', () => {
  it('keeps the windowed grid working with tiles', async () => {
    state.variants = { ...state.variants, 'game-tile.aspect': '3:4', 'game-tile.compat': 'on', 'game-tile.overlay': 'on', 'game-tile.density': 'comfortable' };
    const games = Array.from({ length: 40 }, (_, i) => game({ id: `steam-${i}`, title: `Game ${i}` }));
    const { container } = rtlRender(
      <WindowedGrid items={games} itemKey={(g) => g.id} minColumn={140} gap={12} rowHeight={(w) => w * 1.5 + 112} className="cover-grid-sm" renderItem={(g) => <GameTile game={g} meta="test" coverBadges onOpen={() => {}} />} />,
    );
    // jsdom has no layout: the fallback renders the first screenful.
    expect(rtlScreen.getByRole('button', { name: /Game 0/ })).toBeInTheDocument();
    (expect(await (await import('vitest-axe')).axe(container)) as any).toHaveNoViolations();
  });
});
