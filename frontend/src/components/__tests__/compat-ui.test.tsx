import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender, fireEvent, within } from '@testing-library/react';
import { LibraryView } from '../LibraryView';
import { CompatChip, CompatFacts, WikiCreditLine } from '../CompatBadges';
import { TooltipProvider } from '../ds/tooltip';
import type { Compat, Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const pdb = (tier: string) => ({ source: 'protondb', tier, description: '' }) as Game['fix_info'];

const lib = (games: Game[], mode: 'cards' | 'table' = 'cards') =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode={mode} onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );

describe('CompatChip', () => {
  it('shows Native for a Linux build, the tier otherwise, and nothing when unknown', () => {
    const { getByText, rerender, container } = render(<CompatChip game={mk('a', { native: true, fix_info: pdb('gold') })} />);
    expect(getByText('Native')).toBeTruthy();
    rerender(<CompatChip game={mk('b', { fix_info: pdb('platinum') })} />);
    expect(getByText('ProtonDB · Platinum')).toBeTruthy();
    rerender(<CompatChip game={mk('c')} />);
    expect(container.textContent).toBe('');
  });
});

describe('CompatFacts', () => {
  const full: Compat = {
    steam_app_id: '620',
    steam_app_id_source: 'plugin',
    native: true,
    ratings_online: true,
    protondb: { tier: 'platinum', confidence: 'strong', total: 343, fetched_at: 1791589136, url: 'https://www.protondb.com/app/620' },
    drm: [{ kind: 'online', label: 'Online required', detail: 'The store says it needs the network.', source: 'Epic', checked_at: 1791589136 }],
    drm_notes: 'Echo fixture notes',
    anticheat: [{ kind: 'anticheat', label: 'Easy Anti-Cheat', detail: 'Linux: supported.', source: 'AreWeAntiCheatYet (MIT)', checked_at: 1791589136 }],
  };

  it('shows the rating with its link and date, the DRM and anti-cheat badges, and the AppID', () => {
    const { getByText, getByRole, getByLabelText } = render(<CompatFacts compat={full} />);
    expect(getByText(/Platinum · 343 reports/)).toBeTruthy();
    expect(getByText(/as of 2026-10-09/)).toBeTruthy();
    expect(getByRole('link', { name: /protondb/i }).getAttribute('href')).toBe('https://www.protondb.com/app/620');
    expect(within(getByLabelText('DRM')).getByText('Online required')).toBeTruthy();
    expect(within(getByLabelText('Anti-cheat')).getByText('Easy Anti-Cheat')).toBeTruthy();
    expect(getByText('Echo fixture notes')).toBeTruthy();
    expect(getByText(/Steam AppID 620 \(from the store\)/)).toBeTruthy();
  });

  it('credits PCGamingWiki under the facts it gave: the page, the licence and the day read', () => {
    const wiki: Compat = {
      ...full,
      drm: [{ kind: 'drm-removed', label: 'Denuvo removed', detail: 'Removed.', source: 'PCGamingWiki', checked_at: 1791589136 }],
      anticheat: [],
      wiki: { title: 'Portal 2', url: 'https://www.pcgamingwiki.com/wiki/Portal_2', fetched_at: 1791589136, licence_url: 'https://creativecommons.org/licenses/by-nc-sa/3.0/' },
    };
    const { getByLabelText, getByRole, getByText } = render(<CompatFacts compat={wiki} />);
    const credit = getByLabelText('PCGamingWiki credit');
    expect(credit.textContent).toContain('Source: PCGamingWiki contributors, CC BY-NC-SA 3.0');
    expect(credit.textContent).toContain('read 2026-10-09');
    expect(getByRole('link', { name: 'Portal 2 on PCGamingWiki ↗' }).getAttribute('href')).toBe('https://www.pcgamingwiki.com/wiki/Portal_2');
    expect(getByRole('link', { name: 'licence ↗' }).getAttribute('href')).toBe('https://creativecommons.org/licenses/by-nc-sa/3.0/');
    expect(getByText('Denuvo removed')).toBeTruthy();
  });

  it('shows no PCGamingWiki credit when the wiki gave no facts', () => {
    const { queryByLabelText } = render(<CompatFacts compat={{ ...full, wiki: null }} />);
    expect(queryByLabelText('PCGamingWiki credit')).toBeNull();
    const { container } = render(<WikiCreditLine credit={{ title: 'X', url: 'u', fetched_at: 0, licence_url: 'l' }} />);
    expect(container.textContent).not.toContain('read ');
  });

  it('says so when nothing is reported, and leaves out a rating that was never asked for', () => {
    const { getByText, queryByText } = render(<CompatFacts compat={{ native: false, ratings_online: false, drm: [], anticheat: [] }} />);
    expect(getByText('No DRM or anti-cheat reported for this game.')).toBeTruthy();
    expect(queryByText(/reports/)).toBeNull();
  });
});

describe('Library tier filter', () => {
  const games = [
    mk('plat', { fix_info: pdb('platinum') }),
    mk('nat', { native: true }),
    mk('none'),
    mk('bork', { fix_info: pdb('borked') }),
  ];
  const shown = (container: HTMLElement) => [...container.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);

  it('shows native builds with their badge and narrows the grid by tier', () => {
    const { container, getByLabelText, getByRole, getAllByText } = lib(games);
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
