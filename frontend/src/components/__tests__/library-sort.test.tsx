import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, fireEvent, within } from '@testing-library/react';
import { LibraryView, sortGames } from '../LibraryView';
import type { Game } from '../../types';

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
    fireEvent.click(getByRole('button', { name: 'tier' }));
    expect((getByLabelText('Sort by') as HTMLSelectElement).value).toBe('tier');
    const order = () => getAllByRole('row').slice(1).map((r) => within(r).getAllByRole('cell')[1].textContent);
    expect(order()[0]).toBe('plat');
    expect(order()[5]).toBe('none');
  });
  it('card mode sorts via the toolbar and drops the last-played strip', () => {
    const played = [mk('b', { last_played_at: 5, playtime_minutes: 1 }), mk('a', { last_played_at: 9, playtime_minutes: 99 }), mk('c', { playtime_minutes: 50 })];
    const { container, getByLabelText, queryByText } = lib(played, 'cards');
    expect(queryByText('Last played')).toBeTruthy();
    fireEvent.change(getByLabelText('Sort by'), { target: { value: 'hours' } });
    expect(queryByText('Last played')).toBeNull();
    const titles = () => [...container.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);
    expect(titles()).toEqual(['a', 'c', 'b']);
    fireEvent.click(getByLabelText(/Descending/));
    expect(titles()).toEqual(['b', 'c', 'a']);
  });
});
