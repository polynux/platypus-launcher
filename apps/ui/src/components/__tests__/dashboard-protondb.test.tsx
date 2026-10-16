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
const pdb = (tier: string) => [{ kind: 'rating', label: tier[0].toUpperCase() + tier.slice(1), tone: tier, source: 'ProtonDB', checked_at: 0 }] as Game['compat_badges'];
const fix = (tier: string) => ({ source: 'curated', tier, description: 'needs this fix' }) as Game['fix_info'];

const dash = (games: Game[]) =>
  render(<Dashboard games={games} runners={[]} activity={[]} dismissedHero={new Set()} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} />);

describe('Dashboard ProtonDB chips (#20)', () => {
  it('shares the library chip: a tier shows it, native and unrated games show nothing', () => {
    const { getByText, queryByText } = render(<CompatChip game={mk('a', { compat_badges: pdb('gold') })} />);
    expect(getByText('ProtonDB · Gold')).toBeTruthy();
    expect(queryByText('Native')).toBeNull();
  });

  it('shows the tier chip on grid cards, hidden for native and unrated games', () => {
    const games = [
      mk('rated', { compat_badges: pdb('gold') }),
      mk('native', { native: true, compat_badges: pdb('gold') }),
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
    expect(getByText('curated · Gold')).toBeTruthy();
  });

  it('lays badges below the title, not beside it, so they cannot cover it (#72)', () => {
    const g = mk('r-both', { store: 'custom', compat_badges: pdb('platinum'), fix_info: { source: 'steamdeck', tier: '', description: '', deck: 'playable' } as Game['fix_info'] });
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
      mk('r-rated', { store: 'custom', compat_badges: pdb('platinum') }),
      mk('r-native', { store: 'custom', native: true, compat_badges: pdb('platinum') }),
      mk('r-plain', { store: 'custom' }),
    ];
    const { getByText, queryByText, container } = render(<RecentList games={games} onOpenGame={() => {}} />);
    expect(getByText('ProtonDB · Platinum')).toBeTruthy();
    expect(queryByText('Native')).toBeNull();
    expect(container.textContent?.match(/ProtonDB · Platinum/g)?.length).toBe(1);
  });
});
