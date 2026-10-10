import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render as rtlRender, fireEvent, within } from '@testing-library/react';
import { Dashboard } from '../Dashboard';
import { LibraryView, sourceChips } from '../LibraryView';
import type { Game } from '../../types';
import { TooltipProvider } from '../ds/tooltip';
const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const dash = (games: Game[], extra: Record<string, unknown> = {}) =>
  render(<Dashboard games={games} runners={[]} activity={[]} dismissedHero={new Set()} onDismissHero={() => {}} onOpenGame={() => {}} onNavigate={() => {}} {...extra} />);

describe('Dashboard truth', () => {
  it('D1: mixed library says installed and owned, never "N titles · N ready"', () => {
    const { container } = dash([mk('a'), mk('b')], { ownedCount: 5 });
    expect(container.textContent).toContain('2 installed · 5 owned');
    expect(container.textContent).not.toMatch(/ready|titles/);
  });
  it('D1: no owned figure when everything owned is installed', () => {
    const { container } = dash([mk('a'), mk('b')], { ownedCount: 2 });
    expect(container.textContent).toContain('2 installed ·');
    expect(container.textContent).not.toContain('owned');
  });
  it('D2: hides Recently added when empty, shows it with sideloaded games', () => {
    expect(dash([mk('a')]).queryByText('Recently added')).toBeNull();
    const { getByText } = dash([mk('a'), mk('c', { store: 'custom' })]);
    expect(getByText('Recently added')).toBeTruthy();
  });
  it('D3: Later dismisses ids, Hide all dismisses every waiting game', () => {
    const fix = { source: 'curated', tier: 'gold' } as Game['fix_info'];
    const onDismissHero = vi.fn();
    const { getByText } = dash([mk('a', { fix_info: fix }), mk('b', { fix_info: fix })], { onDismissHero });
    fireEvent.click(getByText('Later'));
    expect(onDismissHero).toHaveBeenLastCalledWith(['a']);
    fireEvent.click(getByText('Hide all 2'));
    expect(onDismissHero).toHaveBeenLastCalledWith(['a', 'b']);
  });
  it('D4: without play data, falls back to title order', () => {
    const { container } = dash([mk('zeta'), mk('alpha'), mk('mid')]);
    const titles = [...container.querySelectorAll('.cover-grid .truncate')].map((e) => e.textContent);
    expect(titles).toEqual(['alpha', 'mid', 'zeta']);
  });
});

const lib = (games: Game[], over: Record<string, unknown> = {}) =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode="table" onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} {...over} />
  );

describe('Library', () => {
  it('L1: chips only for stores that have games', () => {
    expect(sourceChips([mk('a'), mk('b', { store: 'custom' })])).toEqual(['all', 'steam']);
    expect(sourceChips([mk('a'), mk('g', { store: 'gog' }), mk('x', { store: 'itch' as Game['store'] })])).toEqual(['all', 'steam', 'gog', 'itch']);
    // One store has nothing to choose between: no store chips at all, two or more show them.
    expect(lib([mk('a')]).queryByRole('radio', { name: 'steam' })).toBeNull();
    const { queryByRole } = lib([mk('a'), mk('g', { store: 'gog' })]);
    expect(queryByRole('radio', { name: 'gog' })).toBeTruthy();
    expect(queryByRole('radio', { name: 'steam' })).toBeTruthy();
  });
  it('L2: sorts by title and hours with aria-sort', () => {
    const games = [mk('b', { playtime_minutes: 60 }), mk('a', { playtime_minutes: 600 }), mk('c', { playtime_minutes: 6 })];
    const { getAllByRole, getByRole } = lib(games);
    const order = () => getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].textContent);
    fireEvent.click(getByRole('button', { name: 'title' }));
    expect(order()).toEqual(['a', 'b', 'c']);
    expect(getByRole('columnheader', { name: 'title' }).getAttribute('aria-sort')).toBe('ascending');
    fireEvent.click(getByRole('button', { name: 'title' }));
    expect(order()).toEqual(['c', 'b', 'a']);
    fireEvent.click(getByRole('button', { name: 'hrs' }));
    expect(order()).toEqual(['a', 'b', 'c']);
    expect(getByRole('columnheader', { name: 'hrs' }).getAttribute('aria-sort')).toBe('descending');
  });
  it('L3: clear button only with text', () => {
    expect(lib([mk('a')]).queryByLabelText('Clear search')).toBeNull();
    const onSearch = vi.fn();
    const { getByLabelText } = lib([mk('a')], { search: 'x', onSearch });
    fireEvent.click(getByLabelText('Clear search'));
    expect(onSearch).toHaveBeenCalledWith('');
  });
});
