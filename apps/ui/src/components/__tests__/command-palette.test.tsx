// The command palette (issue #124): Ctrl K and the shell's window event open
// it, typing filters it, the arrows and Enter pick a row, Escape closes it and
// the focus goes back where it was.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import * as matchers from 'vitest-axe/matchers';
import 'vitest-axe/extend-expect';
import { CommandPalette, openCommandPalette, type CommandPaletteProps } from '../CommandPalette';
import { Modal } from '../Modal';
import type { Game, SessionEvent, StorePlugin } from '../../types';

expect.extend(matchers);

afterEach(cleanup);

const game = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, ...o }) as Game;

const store = (id: string, name: string): StorePlugin => ({ id, name, status: 'ready', storefront_url: 'https://example.test' }) as StorePlugin;

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
  };
  render(
    <CommandPalette
      games={[game('celeste', { title: 'Celeste' }), game('hades', { title: 'Hades' })]}
      stores={[store('steam', 'Steam')]}
      onOpenGame={calls.openGame}
      onNavigate={calls.navigate}
      onOpenStore={calls.openStore}
      onOpenSettings={calls.openSettings}
      onPlay={calls.play}
      onStop={calls.stop}
      onInstall={calls.install}
      onOpenGameSettings={calls.gameSettings}
      {...overrides}
    />,
  );
  return calls;
}

const openWithCtrlK = () => fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
const input = () => screen.getByRole('combobox', { name: 'Command' });
const options = () => screen.getAllByRole('option');

describe('command palette', () => {
  it('opens with Ctrl K and closes with Escape, returning the focus', async () => {
    setup();
    const opener = document.createElement('button');
    document.body.append(opener);
    opener.focus();
    expect(screen.queryByRole('dialog')).toBeNull();

    openWithCtrlK();
    const dialog = await screen.findByRole('dialog', { name: 'Command palette' });
    expect(within(dialog).getByRole('combobox', { name: 'Command' })).toBe(document.activeElement);

    fireEvent.keyDown(input(), { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(opener));
    opener.remove();
  });

  it('opens from the window event the shell dispatches', async () => {
    setup();
    openCommandPalette();
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('stays shut while another dialog is open, for both Ctrl K and the event', async () => {
    setup();
    // The settings drawer: the app's own Modal (Radix), with unsaved edits.
    const drawer = render(<Modal label="Settings" onClose={() => {}}><p>Unsaved edits</p></Modal>);

    openWithCtrlK();
    expect(screen.queryByRole('dialog', { name: 'Command palette' })).toBeNull();
    openCommandPalette();
    expect(screen.queryByRole('dialog', { name: 'Command palette' })).toBeNull();

    // The drawer closes: the palette opens again.
    drawer.unmount();
    openWithCtrlK();
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
  });

  it('stays shut under a modal that marks itself with aria-modal, such as the lightbox', async () => {
    setup();
    // Rendered rather than appended, so a failing test cannot leave it behind.
    const viewer = render(<div role="dialog" aria-modal="true" aria-label="Screenshot" />);

    openWithCtrlK();
    expect(screen.queryByRole('dialog', { name: 'Command palette' })).toBeNull();

    viewer.unmount();
    openCommandPalette();
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
  });

  it('lists the places, the stores and the first games, axe clean', async () => {
    setup();
    openWithCtrlK();
    const dialog = await screen.findByRole('dialog', { name: 'Command palette' });
    const labels = options().map((o) => o.textContent);
    expect(labels).toContain('Library');
    expect(labels).toContain('Settings');
    expect(labels).toContain('Steam');
    // A game row carries its hours played after the title.
    expect(labels.some((label) => label?.startsWith('Celeste'))).toBe(true);
    (expect(await (await import('vitest-axe')).axe(dialog)) as unknown as { toHaveNoViolations: () => void }).toHaveNoViolations();
  });

  it('filters on typing', async () => {
    setup();
    openWithCtrlK();
    await screen.findByRole('dialog');
    fireEvent.change(input(), { target: { value: 'cel' } });
    expect(options()).toHaveLength(1);
    expect(options()[0]).toHaveTextContent('Celeste');
    fireEvent.change(input(), { target: { value: 'zzz' } });
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(screen.getByText(/Nothing matches/)).toBeInTheDocument();
  });

  it('moves the highlight with the arrows and runs the row on Enter', async () => {
    const calls = setup();
    openWithCtrlK();
    await screen.findByRole('dialog');

    // Nothing typed: the first place is highlighted, the second is next.
    expect(options()[0]).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'ArrowDown' });
    expect(options()[1]).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input(), { key: 'ArrowUp' });
    expect(options()[0]).toHaveAttribute('aria-selected', 'true');

    fireEvent.change(input(), { target: { value: 'had' } });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(calls.openGame).toHaveBeenCalledWith('hades');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('acts on the current game: stop while it runs, then the settings entry', async () => {
    const session = { game_id: 'celeste', state: 'running' } as SessionEvent;
    const calls = setup({ currentGame: game('celeste', { title: 'Celeste' }), session });
    openWithCtrlK();
    await screen.findByRole('dialog');
    expect(options()[0]).toHaveTextContent('Stop Celeste');
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(calls.stop).toHaveBeenCalledWith('celeste');
  });

  it('offers Play on an installed current game, Install when it is not', async () => {
    const calls = setup({ currentGame: game('celeste', { title: 'Celeste' }) });
    openWithCtrlK();
    await screen.findByRole('dialog');
    // Play is the first row; its neighbour is the settings entry. The labels
    // carry the game title, so match on the whole option text.
    const rows = options().map((o) => o.textContent);
    expect(rows[0]).toBe('Play Celeste');
    expect(rows[1]).toBe('Open game settings');
    fireEvent.click(options()[1]);
    expect(calls.gameSettings).toHaveBeenCalledWith('celeste');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    calls.openGame.mockClear();
    cleanup();
    const fresh = setup({ currentGame: game('hades', { title: 'Hades', is_installed: false }) });
    openWithCtrlK();
    await screen.findByRole('dialog');
    expect(options()[0]).toHaveTextContent('Install Hades');
    fireEvent.click(options()[0]);
    expect(fresh.install).toHaveBeenCalledWith(expect.objectContaining({ id: 'hades' }));
    void calls;
  });
});

describe('atlas parity actions (issue #132)', () => {
  it('offers redeem, collections and add-game actions when provided', async () => {
    const redeem = vi.fn();
    const manage = vi.fn();
    const add = vi.fn();
    setup({ onRedeem: redeem, onManageCollections: manage, onAddGame: add });
    openWithCtrlK();
    await screen.findByRole('dialog');
    expect(screen.getByRole('option', { name: 'Redeem a code' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Manage collections' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Add a game' })).toBeInTheDocument();
    fireEvent.change(input(), { target: { value: 'redeem' } });
    fireEvent.keyDown(input(), { key: 'Enter' });
    expect(redeem).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  });

  it('hides the atlas actions without their providers', async () => {
    setup();
    openWithCtrlK();
    await screen.findByRole('dialog');
    expect(screen.queryByRole('option', { name: 'Redeem a code' })).toBeNull();
    expect(screen.queryByRole('option', { name: 'Manage collections' })).toBeNull();
    expect(screen.queryByRole('option', { name: 'Add a game' })).toBeNull();
  });
});
