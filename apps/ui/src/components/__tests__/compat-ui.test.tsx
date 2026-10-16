import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender, fireEvent, within } from '@testing-library/react';
import { LibraryView } from '../LibraryView';
import { CompatChip, CompatFacts, DeckBadge, WikiCreditLine } from '../CompatBadges';
import { TooltipProvider } from '../ds/tooltip';
import type { Compat, Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const pdb = (tier: string) => [{ kind: 'rating', label: tier[0].toUpperCase() + tier.slice(1), tone: tier, source: 'ProtonDB', checked_at: 1791589136, link: 'https://www.protondb.com/app/620' }] as Game['compat_badges'];

const lib = (games: Game[], mode: 'cards' | 'table' = 'cards') =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode={mode} onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );

describe('CompatChip', () => {
  it('shows Native for a Linux build, the tier otherwise, and nothing when unknown', () => {
    const { getByText, rerender, container } = render(<CompatChip game={mk('a', { native: true, compat_badges: pdb('gold') })} />);
    expect(getByText('Native')).toBeTruthy();
    rerender(<CompatChip game={mk('b', { compat_badges: pdb('platinum') })} />);
    expect(getByText('ProtonDB · Platinum')).toBeTruthy();
    rerender(<CompatChip game={mk('c')} />);
    expect(container.textContent).toBe('');
  });
});

describe('DeckBadge', () => {
  it('shows a badge per verdict and nothing when Steam has no verdict', () => {
    const { getByText, rerender, container } = render(<DeckBadge rating="verified" />);
    expect(getByText('Deck · Verified')).toBeTruthy();
    rerender(<DeckBadge rating="playable" notes={['Some text is too small to read on the Deck.']} />);
    expect(getByText('Deck · Playable')).toBeTruthy();
    rerender(<DeckBadge rating="unsupported" />);
    expect(getByText('Deck · Unsupported')).toBeTruthy();
    rerender(<DeckBadge rating="" />);
    expect(container.textContent).toBe('');
  });

  it('shows the Deck verdict beside the tier on a card, even with no tier', () => {
    const deckOnly = mk('d', { compat_badges: [{ kind: 'verdict', label: 'Playable', tone: 'playable', source: 'Steam Deck', checked_at: 1 }] });
    const { getByText, queryByText } = render(<CompatChip game={deckOnly} />);
    expect(getByText('Deck · Playable')).toBeTruthy();
    expect(queryByText(/ProtonDB/)).toBeNull();
  });
});

describe('CompatFacts', () => {
  const full: Compat = {
    steam_app_id: '620',
    steam_app_id_source: 'plugin',
    native: true,
    ratings_online: true,
    badges: [...pdb('platinum')!, { kind: 'online', label: 'Online required', detail: 'The store says it needs the network.', source: 'Epic', checked_at: 1791589136 }, { kind: 'anticheat', label: 'Easy Anti-Cheat', detail: 'Linux: supported.', source: 'AreWeAntiCheatYet (MIT)', checked_at: 1791589136 }],
    notes: ['Echo fixture notes'],
  };

  it('shows the rating with its link and date, the DRM and anti-cheat badges, and the AppID', () => {
    const { getByText, getByRole, getByLabelText } = render(<CompatFacts compat={full} />);
    expect(getByText('Platinum').getAttribute('title')).toContain('checked 2026-10-09');
    expect(getByRole('link', { name: 'Platinum' }).getAttribute('href')).toBe('https://www.protondb.com/app/620');
    expect(getByRole('link', { name: 'Platinum' }).getAttribute('href')).toBe('https://www.protondb.com/app/620');
    expect(within(getByLabelText('DRM')).getByText('Online required')).toBeTruthy();
    expect(within(getByLabelText('Anti-cheat')).getByText('Easy Anti-Cheat')).toBeTruthy();
    expect(getByText('Echo fixture notes')).toBeTruthy();
    expect(getByText(/Steam AppID 620 \(from the store\)/)).toBeTruthy();
  });

  it('shows report counts and the no/few report messages from generic provider facts', () => {
    const { getByText, rerender } = render(<CompatFacts compat={{ ...full, report: { state: 'rated', count: 343, tier: 'platinum' } }} />);
    expect(getByText('Platinum · 343 reports')).toBeTruthy();
    rerender(<CompatFacts compat={{ ...full, report: { state: 'few', count: 2 } }} />);
    expect(getByText('Few ProtonDB reports (2)')).toBeTruthy();
    rerender(<CompatFacts compat={{ ...full, report: { state: 'none', count: 0 } }} />);
    expect(getByText('No ProtonDB reports')).toBeTruthy();
  });

  it('shows the Deck verdict with its link and date beside the ProtonDB rating', () => {
    const decked: Compat = {
      ...full,
      steam_app_id: '620', badges: [{ kind: 'verdict', label: 'Playable', tone: 'playable', source: 'Steam Deck', checked_at: 1791589136, link: 'https://store.steampowered.com/app/620' }, { kind: 'note', label: 'Some text is too small to read on the Deck.', source: 'Steam Deck', checked_at: 1791589136 }],
    };
    const { getByText, getAllByText, getByRole } = render(<CompatFacts compat={decked} />);
    expect(getByText('Deck · Playable')).toBeTruthy();
    expect(getAllByText(/2026-10-09/)).toHaveLength(1);
    expect(getByRole('link', { name: /steam/i }).getAttribute('href')).toBe('https://store.steampowered.com/app/620');
  });

  it('credits PCGamingWiki under the facts it gave: the page, the licence and the day read', () => {
    const wiki: Compat = {
      ...full,
      badges: [{ kind: 'drm-removed', label: 'Denuvo removed', detail: 'Removed.', source: 'PCGamingWiki', checked_at: 1791589136 }],
      wiki: { title: 'Portal 2', url: 'https://www.pcgamingwiki.com/wiki/Portal_2', fetched_at: 1791589136, licence_url: 'https://creativecommons.org/licenses/by-nc-sa/3.0/' },
    };
    const { getByLabelText, getByRole, getByText } = render(<CompatFacts compat={wiki} />);
    const credit = getByLabelText('PCGamingWiki credit');
    expect(credit.textContent).toContain('Source: PCGamingWiki contributors, CC BY-NC-SA 3.0');
    expect(credit.textContent).toContain('read 2026-10-09');
    expect(getByRole('link', { name: 'Portal 2 on PCGamingWiki ↗' }).getAttribute('href')).toBe('https://www.pcgamingwiki.com/wiki/Portal_2');
    expect(getByRole('link', { name: 'licence ↗' }).getAttribute('href')).toBe('https://creativecommons.org/licenses/by-nc-sa/3.0/');
    expect(within(getByLabelText('DRM')).getByText('Denuvo removed')).toBeTruthy();
  });

  it('shows no PCGamingWiki credit when the wiki gave no facts', () => {
    const { queryByLabelText } = render(<CompatFacts compat={{ ...full, wiki: null }} />);
    expect(queryByLabelText('PCGamingWiki credit')).toBeNull();
    const { container } = render(<WikiCreditLine credit={{ title: 'X', url: 'u', fetched_at: 0, licence_url: 'l' }} />);
    expect(container.textContent).not.toContain('read ');
  });

  it('says so when nothing is reported, and leaves out a rating that was never asked for', () => {
    const { getByText, queryByText } = render(<CompatFacts compat={{ native: false, ratings_online: false, badges: [] }} />);
    expect(getByText('No DRM or anti-cheat reported for this game.')).toBeTruthy();
    expect(queryByText(/reports/)).toBeNull();
  });
});

describe('Library tier filter', () => {
  const games = [
    mk('plat', { compat_badges: pdb('platinum') }),
    mk('nat', { native: true }),
    mk('none'),
    mk('bork', { compat_badges: pdb('borked') }),
  ];
  const shown = (container: HTMLElement) => [...container.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);

  it('shows native builds with their badge and narrows the grid by tier', () => {
    const { container, getByLabelText, getByRole, getAllByText } = lib(games);
    fireEvent.click(getByRole('button', { name: /^Filters/ }));
    expect(shown(container)).toEqual(['plat', 'nat', 'none', 'bork']);
    expect(getAllByText('Native').length).toBe(1);
    fireEvent.pointerDown(getByLabelText('Filter by tier'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(getByRole('option', { name: 'native' }));
    expect(shown(container)).toEqual(['nat']);
    fireEvent.pointerDown(getByLabelText('Filter by tier'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(getByRole('option', { name: 'no rating' }));
    expect(shown(container)).toEqual(['none']);
  });

  it('shows the native build as native in the table', () => {
    const { getByRole } = lib(games, 'table');
    expect(within(getByRole('table', { name: 'Library' })).getByText('native')).toBeTruthy();
  });
});
