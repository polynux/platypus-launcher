import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender } from '@testing-library/react';
import { Dashboard, RecentList } from '../Dashboard';
import { CompatChip } from '../CompatBadges';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const pdb = (tier: string) => ({ source: 'protondb', tier, description: '' }) as Game['fix_info'];
const fix = (tier: string) => ({ source: 'curated', tier, description: 'needs this fix' }) as Game['fix_info'];

const dash = (games: Game[]) =>
  render(<Dashboard games={games} runners={[]} activity={[]} dismissedHero={new Set()} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} />);

describe('Dashboard ProtonDB chips (#20)', () => {
  it('shares the library chip: a tier shows it, native and unrated games show nothing', () => {
    const { getByText, queryByText } = render(<CompatChip game={mk('a', { fix_info: pdb('gold') })} />);
    expect(getByText('ProtonDB · Gold')).toBeTruthy();
    expect(queryByText('Native')).toBeNull();
  });

  it('shows the tier chip on grid cards, hidden for native and unrated games', () => {
    const games = [
      mk('rated', { fix_info: pdb('gold') }),
      mk('native', { native: true, fix_info: pdb('gold') }),
      mk('plain'),
    ];
    const { getByText, queryByText, container } = dash(games);
    expect(getByText('ProtonDB · Gold')).toBeTruthy();
    // The native game keeps its tier hidden: no Native badge on the Dashboard.
    expect(queryByText('Native')).toBeNull();
    // Exactly one chip for three cards.
    expect(container.textContent?.match(/ProtonDB · Gold/g)?.length).toBe(1);
  });

  it('shows a curated fix with the same chip, not a new style', () => {
    const { getByText } = dash([mk('fixed', { fix_info: fix('gold') })]);
    expect(getByText('Fix · Gold')).toBeTruthy();
  });

  it('lays badges below the title, not beside it, so they cannot cover it (#72)', () => {
    const g = mk('r-both', { store: 'custom', fix_info: { source: 'protondb', tier: 'platinum', description: '', deck: 'playable' } as Game['fix_info'] });
    const { getByText } = render(<RecentList games={[g]} onOpenGame={() => {}} />);
    const title = getByText('r-both');
    // Both badges share the title's own container, on lines below it…
    expect(title.parentElement?.textContent).toContain('ProtonDB · Platinum');
    expect(title.parentElement?.textContent).toContain('Deck · Playable');
    // …and the row holds only the cover and that container, so nothing
    // competes with the title for width.
    expect(title.closest('button')?.children.length).toBe(2);
  });

  it('shows the chip in compact Recently-added rows, hidden for native and unrated', () => {
    const games = [
      mk('r-rated', { store: 'custom', fix_info: pdb('platinum') }),
      mk('r-native', { store: 'custom', native: true, fix_info: pdb('platinum') }),
      mk('r-plain', { store: 'custom' }),
    ];
    const { getByText, queryByText, container } = render(<RecentList games={games} onOpenGame={() => {}} />);
    expect(getByText('ProtonDB · Platinum')).toBeTruthy();
    expect(queryByText('Native')).toBeNull();
    expect(container.textContent?.match(/ProtonDB · Platinum/g)?.length).toBe(1);
  });
});
