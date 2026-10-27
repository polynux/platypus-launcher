import React from 'react';
import { describe, it, expect } from 'vitest';
import { render as rtlRender, fireEvent, screen } from '@testing-library/react';
import { LibraryView } from '../LibraryView';
import type { Game } from '../../types';
import { TooltipProvider } from '../ds/tooltip';
const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const lib = (games: Game[]) =>
  render(
    <LibraryView games={games} runners={[]} activity={[]} search="" onSearch={() => {}} source="all" onSource={() => {}} mode="cards" onMode={() => {}}
      ownership="owned" onOwnership={() => {}} warnings={[]} plugins={[]} jobs={[]} lastScan={null} onOpenGame={() => {}} onAddGame={() => {}} onNavigate={() => {}} />
  );

const games = [
  mk('playing-one', { status: 'playing' as const }),
  mk('finished-one', { status: 'finished' as const }),
  mk('plain-one'),
];

const titles = () =>
  [...document.querySelectorAll('.cover-grid-sm > button')].map((b) => b.querySelector('.truncate')?.textContent);

const pickStatusFilter = (name: string) => {
  const filters = screen.getByRole('button', { name: /^Filters/ });
  if (filters.getAttribute('aria-expanded') !== 'true') fireEvent.click(filters);
  fireEvent.pointerDown(screen.getByLabelText('Filter by status'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
  fireEvent.click(screen.getByRole('option', { name }));
};

describe('library status filter', () => {
  it('narrows the library to one status, to none, and back to all', () => {
    lib(games);
    expect(titles()).toEqual(['playing-one', 'finished-one', 'plain-one']);

    pickStatusFilter('Playing');
    expect(titles()).toEqual(['playing-one']);

    pickStatusFilter('No status');
    expect(titles()).toEqual(['plain-one']);

    pickStatusFilter('all');
    expect(titles()).toEqual(['playing-one', 'finished-one', 'plain-one']);
  });
});

describe('library status badge', () => {
  it('shows a badge whenever a status is set, and none without one', () => {
    lib(games);
    const badges = screen.getAllByTestId('status-badge');
    expect(badges).toHaveLength(2);
    expect(badges[0]).toHaveTextContent(/playing/i);
    expect(badges[1]).toHaveTextContent(/finished/i);
    const plain = [...document.querySelectorAll('.cover-grid-sm > button')].find((b) => b.textContent?.includes('plain-one'));
    expect(plain).toBeTruthy();
    expect(plain!.querySelector('[data-testid="status-badge"]')).toBeNull();
  });
});
