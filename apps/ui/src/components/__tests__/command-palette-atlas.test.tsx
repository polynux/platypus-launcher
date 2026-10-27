// The /22 command palette under Atlas (review 4, lane 4A): the Library,
// Running, Open and Go to game groups, the collection sub-mode, and the
// empty Running row that just closes the palette. Behaviour only; the other
// themes keep the generic palette (command-palette.test.tsx).
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import '../../lib/i18n';
import { CommandPalette, openCommandPalette, type CommandPaletteProps } from '../CommandPalette';
import type { Collection, Game, SessionEvent, StorePlugin } from '../../types';

afterEach(cleanup);

vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return { ...mod, useVariant: () => 'plain' };
});

const game = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const store = (id: string, name: string): StorePlugin =>
  ({ id, name, store: id, status: 'ready', storefront_url: 'https://example.test' }) as StorePlugin;

const collection = (id: string, name: string): Collection => ({ id, name, games: [] }) as Collection;

function setup(overrides: Partial<CommandPaletteProps> = {}) {
  const calls = {
    openGame: vi.fn(),
    navigate: vi.fn(),
    openStore: vi.fn(),
    openSettings: vi.fn(),
    play: vi.fn(),
    stop: vi.fn(),
    install: vi.fn(),
    gameSettings: vi.fn(),
    openFriends: vi.fn(),
    showCollection: vi.fn(),
    newCollection: vi.fn(),
  };
  render(
    <CommandPalette
      games={[game('celeste', { title: 'Celeste' })]}
      stores={[store('steam', 'Steam')]}
      onOpenGame={calls.openGame}
      onNavigate={calls.navigate}
      onOpenStore={calls.openStore}
      onOpenSettings={calls.openSettings}
      onPlay={calls.play}
      onStop={calls.stop}
      onInstall={calls.install}
      onOpenGameSettings={calls.gameSettings}
      onOpenFriends={calls.openFriends}
      collections={[collection('c1', 'Weekend'), collection('c2', 'Empty')]}
      collectionTotals={{ c1: 3, c2: 0 }}
      onShowCollection={calls.showCollection}
      onNewCollection={calls.newCollection}
      {...overrides}
    />,
  );
  return calls;
}

const open = async () => {
  openCommandPalette();
  await screen.findByRole('dialog', { name: 'Command palette' });
};
const input = () => screen.getByRole('combobox', { name: 'Command' });
// Label and hint are adjacent spans with no whitespace between them, so the
// accessible name concatenates them ("Weekend3 games"): match the text.
const opt = (text: string) => screen.getAllByRole('option').find((o) => o.textContent === text)!;
const option = (name: string) => screen.getByRole('option', { name });

describe('atlas command palette', () => {
  it('lists the Library, Running, Open and Go to game groups', async () => {
    setup({ onRedeem: vi.fn(), onAddGame: vi.fn() });
    await open();
    expect(option('Redeem a code…')).toBeInTheDocument();
    expect(option('Add a game…')).toBeInTheDocument();
    expect(option('New collection…')).toBeInTheDocument();
    expect(option('Show a collection…')).toBeInTheDocument();
    // Nothing runs: the row says so, and running it just closes the palette.
    expect(opt('Stop the running gamenothing running')).toBeInTheDocument();
    expect(option('Open the Steam store')).toBeInTheDocument();
    expect(option('Home')).toBeInTheDocument();
    expect(screen.getByText('Go to game')).toBeInTheDocument();
    const row = opt('Stop the running gamenothing running');
    fireEvent.click(row);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('offers Stop for each live session, from any game', async () => {
    const calls = setup({
      sessions: [{ game_id: 'celeste', state: 'running', title: 'Celeste' } as SessionEvent & { title: string }],
    });
    await open();
    fireEvent.click(option('Stop Celeste'));
    expect(calls.stop).toHaveBeenCalledWith('celeste');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('opens the collection sub-mode and runs the picked collection', async () => {
    const calls = setup({ collectionId: 'c1' });
    await open();
    fireEvent.click(option('Show a collection…'));
    // The chip, the collections with their counts, and a collection placeholder.
    expect(screen.getByTitle('Back (Backspace)')).toHaveTextContent('Collection');
    expect(input()).toHaveAttribute('placeholder', 'Search collections…');
    expect(opt('Weekend3 games')).toBeInTheDocument();
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(calls.showCollection).toHaveBeenCalledWith('c1');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('leaves the sub-mode on Esc or Backspace instead of closing', async () => {
    setup();
    await open();
    fireEvent.click(option('Show a collection…'));
    expect(screen.getByTitle('Back (Backspace)')).toBeInTheDocument();
    fireEvent.keyDown(input(), { key: 'Escape' });
    expect(screen.queryByTitle('Back (Backspace)')).toBeNull();
    expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
    // Back in the sub-mode, an empty Backspace goes back too.
    fireEvent.click(option('Show a collection…'));
    fireEvent.keyDown(input(), { key: 'Backspace' });
    expect(screen.queryByTitle('Back (Backspace)')).toBeNull();
    expect(screen.getByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
  });
});
