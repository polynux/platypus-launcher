import React from 'react';
import { describe, it, expect } from 'vitest';
import { render as rtlRender, fireEvent, within } from '@testing-library/react';
import { LibraryView, sortGames } from '../LibraryView';
import type { Game } from '../../types';
import { TooltipProvider } from '../ds/tooltip';
const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;
const tier = (t: string) => ({ source: 'curated', tier: t }) as Game['fix_info'];

const lib = (games: Game[], mode: 'cards' | 'table') =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode={mode} onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );

const games = [
  mk('none'), mk('bronze', { fix_info: tier('bronze') }), mk('plat', { fix_info: tier('platinum') }),
  mk('borked', { fix_info: tier('borked') }), mk('gold', { fix_info: tier('gold') }), mk('silver', { fix_info: tier('silver') }),
];

describe('Library sort', () => {
  it('orders tiers by quality, untiered last in both directions', () => {
    const ids = (d: 'asc' | 'desc') => sortGames(games, { key: 'tier', dir: d }).map((g) => g.id);
    expect(ids('desc')).toEqual(['plat', 'gold', 'silver', 'bronze', 'borked', 'none']);
    expect(ids('asc')).toEqual(['borked', 'bronze', 'silver', 'gold', 'plat', 'none']);
  });
  it('table tier header sorts and the toolbar control stays in sync', () => {
    const { getAllByRole, getByRole, getByLabelText } = lib(games, 'table');
    fireEvent.click(getByRole('button', { name: /^Filters/ }));
    fireEvent.click(getByRole('button', { name: 'tier' }));
    expect(getByLabelText('Sort by').textContent).toMatch(/tier/i);
    const order = () => getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].textContent);
    expect(order()[0]).toBe('plat');
    expect(order()[5]).toBe('none');
  });
  it('card mode sorts via the toolbar and drops the last-played strip', () => {
    const played = [mk('b', { last_played_at: 5, playtime_minutes: 1 }), mk('a', { last_played_at: 9, playtime_minutes: 99 }), mk('c', { playtime_minutes: 50 })];
    const { container, getByLabelText, getByRole, queryByText } = lib(played, 'cards');
    fireEvent.click(getByRole('button', { name: /^Filters/ }));
    expect(queryByText('Last played')).toBeTruthy();
    fireEvent.pointerDown(getByLabelText('Sort by'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(getByRole('option', { name: 'hours played' }));
    expect(queryByText('Last played')).toBeNull();
    const titles = () => [...container.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);
    expect(titles()).toEqual(['a', 'c', 'b']);
    fireEvent.click(getByLabelText(/Descending/));
    expect(titles()).toEqual(['b', 'c', 'a']);
  });
  it('sorts by year, unknown years last in both directions', () => {
    const ys = [mk('y2011', { release_year: 2011 }), mk('unknown'), mk('y1998', { release_year: 1998 })];
    const ids = (d: 'asc' | 'desc') => sortGames(ys, { key: 'year', dir: d }).map((g) => g.id);
    expect(ids('desc')).toEqual(['y2011', 'y1998', 'unknown']);
    expect(ids('asc')).toEqual(['y1998', 'y2011', 'unknown']);
  });
  it('table year header sorts, unknown years last', () => {
    const ys = [mk('unknown'), mk('y2011', { release_year: 2011 }), mk('y1998', { release_year: 1998 })];
    const { getAllByRole, getByRole } = lib(ys, 'table');
    fireEvent.click(getByRole('button', { name: 'year' }));
    const order = () => getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].textContent);
    // Year defaults to newest first; the unknown year stays last.
    expect(order()).toEqual(['y2011', 'y1998', 'unknown']);
  });
  it('card meta shows the year when set', () => {
    const { container } = lib([mk('y2011', { release_year: 2011, title: 'Old Game' })], 'cards');
    expect(container.textContent).toMatch(/2011/);
  });
});
