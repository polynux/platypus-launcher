// The existing key handlers keep working with the shared capture handler
// installed (issue #53, findings 1 and 4): the library list owns its keys,
// the palette owns its list, and Tune / Maintenance / friends keep their
// Escape behaviour while arrows move spatially inside them.
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render as rtlRender, screen, waitFor, within } from '@testing-library/react';
import { Effect } from 'effect';
import { installControllerKeyboard } from '../../lib/spatialNav';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

vi.mock('../../lib/theme', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/theme')>();
  return { ...actual, useVariant: (axis: string) => (axis === 'shell.navigation' ? 'sourcebar' : actual.useVariant(axis as never)) };
});

const tuneApi = vi.hoisted(() => ({ getRunnerOptions: vi.fn(), saveGameSettings: vi.fn() }));

import { LibraryListDetail } from '../LibraryListDetail';
import { CommandPalette } from '../CommandPalette';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

let uninstall: (() => void) | null = null;

afterEach(() => {
  cleanup();
  uninstall?.();
  uninstall = null;
  vi.restoreAllMocks();
});

function rect(el: Element, x: number, y: number, w = 60, h = 24): void {
  (el as HTMLElement).getBoundingClientRect = () =>
    ({ left: x, top: y, width: w, height: h, right: x + w, bottom: y + h, x, y, toJSON: () => ({}) }) as DOMRect;
}

function stackVertical(container: Element): void {
  const btns = within(container as HTMLElement).getAllByRole('button');
  btns.forEach((b, i) => rect(b, 0, i * 40));
}

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, last_played_at: 1700000000, ...o }) as Game;

const listGames = () => [
  mk('alpha', { title: 'Alpha', last_played_at: 1700000300 }),
  mk('beta', { title: 'Beta', last_played_at: 1700000200 }),
  mk('gamma', { title: 'Gamma', last_played_at: 1700000100 }),
];

describe('library list keeps its keys', () => {
  function list() {
    const onPlay = vi.fn();
    const r = rtlRender(
      <LibraryListDetail
        games={listGames()}
        ownership="owned"
        source="all"
        onPlay={onPlay}
        renderStage={(g) => <div data-testid="stage">{g.title}</div>}
      />,
      { wrapper: TooltipProvider },
    );
    return { ...r, onPlay };
  }

  it('one ArrowDown advances the selection exactly once and keeps row focus', () => {
    uninstall = installControllerKeyboard();
    list();
    // Stub only the rows: the shared handler can only land on a row, so any
    // steal would show up here.
    const rows = screen.getAllByRole('button', { name: /Alpha|Beta|Gamma/ });
    rows.forEach((r, i) => rect(r, 0, i * 40));
    fireEvent.click(screen.getByRole('button', { name: /Gamma/ }));
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    // The click re-registers the list's window handler after the shared one
    // (the old double-handle order): exactly one of them may act.
    (document.activeElement as HTMLElement | null)?.blur?.();
    fireEvent.keyDown(document.body, { key: 'ArrowDown' });
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    expect(document.activeElement).toBe(screen.getByRole('button', { name: /Gamma/ }));
  });

  it('j/k and arrows on a row move the selection, Enter plays', async () => {
    uninstall = installControllerKeyboard();
    const { onPlay } = list();
    const alpha = screen.getByRole('button', { name: /Alpha/ });
    alpha.focus();
    fireEvent.keyDown(alpha, { key: 'j' });
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
    const beta = screen.getByRole('button', { name: /Beta/ });
    fireEvent.keyDown(beta, { key: 'ArrowDown' });
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    const gamma = screen.getByRole('button', { name: /Gamma/ });
    fireEvent.keyDown(gamma, { key: 'Enter' });
    expect(onPlay).toHaveBeenCalledWith('gamma');
  });
});

describe('command palette keeps its keys', () => {
  function setup() {
    const store = { id: 'steam', name: 'Steam', status: 'ready', storefront_url: 'https://example.test' };
    const calls = { openGame: vi.fn(), navigate: vi.fn() };
    rtlRender(
      <CommandPalette
        games={[mk('celeste', { title: 'Celeste' })]}
        stores={[store] as never}
        onOpenGame={calls.openGame}
        onNavigate={calls.navigate}
        onOpenStore={vi.fn()}
        onOpenSettings={vi.fn()}
        onPlay={vi.fn()}
        onStop={vi.fn()}
        onInstall={vi.fn()}
        onOpenGameSettings={vi.fn()}
      />,
    );
    return calls;
  }

  it('arrows move the highlight and keep focus in the input', async () => {
    uninstall = installControllerKeyboard();
    setup();
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true });
    const input = await screen.findByRole('combobox', { name: 'Command' });
    // A second focusable inside the dialog: without the yield the shared
    // capture handler would steal focus to it.
    const dialog = screen.getByRole('dialog');
    const extra = document.createElement('button');
    extra.type = 'button';
    extra.textContent = 'extra';
    dialog.append(extra);
    rect(input, 0, 0);
    rect(extra, 0, 100);
    const options = () => screen.getAllByRole('option');
    expect(options().length).toBeGreaterThan(1);
    expect(options()[0]).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    expect(options()[1]).toHaveAttribute('aria-selected', 'true');
    expect(document.activeElement).toBe(input);
    fireEvent.keyDown(input, { key: 'ArrowUp' });
    expect(options()[0]).toHaveAttribute('aria-selected', 'true');
    expect(document.activeElement).toBe(input);
    extra.remove();
  });
});

describe('tune flyout keeps its behaviour', () => {
  it('arrows move between its controls and Escape closes it', async () => {
    const { TuneFlyout } = (await import('../TuneFlyout')) as typeof import('../TuneFlyout');
    const { LauncherApi } = (await import('../../effect/runtime')) as { LauncherApi: Record<string, (...a: never[]) => unknown> };
    (LauncherApi as unknown as Record<string, unknown>).getRunnerOptions = (...a: never[]) => Effect.succeed(tuneApi.getRunnerOptions(...a));
    (LauncherApi as unknown as Record<string, unknown>).saveGameSettings = (...a: never[]) => Effect.succeed(tuneApi.saveGameSettings(...a));
    tuneApi.getRunnerOptions.mockReturnValue([]);
    const onClose = vi.fn();
    const game = mk('tuned', { title: 'Tuned Game' });
    uninstall = installControllerKeyboard();
    rtlRender(<TuneFlyout game={game} open onClose={onClose} onAllSettings={vi.fn()} />);
    const dialog = await screen.findByRole('dialog');
    stackVertical(dialog);
    const buttons = within(dialog).getAllByRole('button');
    expect(buttons.length).toBeGreaterThan(1);
    buttons[0].focus();
    fireEvent.keyDown(buttons[0], { key: 'ArrowDown' });
    expect(dialog.contains(document.activeElement)).toBe(true);
    expect(document.activeElement).not.toBe(buttons[0]);
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});

describe('maintenance tab keeps its behaviour', () => {
  it('arrows move between its buttons', async () => {
    const { MaintenanceTab } = await import('../MaintenanceTab');
    const { LauncherApi } = (await import('../../effect/runtime')) as { LauncherApi: Record<string, (...a: never[]) => unknown> };
    (LauncherApi as unknown as Record<string, unknown>).listSaveBackups = () => Effect.succeed([]);
    (LauncherApi as unknown as Record<string, unknown>).getSavePaths = () => Effect.succeed([]);
    (LauncherApi as unknown as Record<string, unknown>).getGameSaveBackup = () => Effect.succeed('');
    (LauncherApi as unknown as Record<string, unknown>).getAutoBackupState = () => Effect.succeed(null);
    const game = mk('tiny', { title: 'Tiny' });
    uninstall = installControllerKeyboard();
    rtlRender(
      <MaintenanceTab
        game={game}
        jobs={[]}
        onJob={vi.fn()}
        onJobAction={vi.fn()}
        onNotify={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const buttons = screen.getAllByRole('button');
    expect(buttons.length).toBeGreaterThan(1);
    stackVertical(document.body);
    buttons[0].focus();
    fireEvent.keyDown(buttons[0], { key: 'ArrowDown' });
    expect(document.activeElement).not.toBe(buttons[0]);
    expect(document.activeElement?.tagName).toBe('BUTTON');
  });
});

describe('friends panel keeps its behaviour', () => {
  it('arrows move between tabs and Escape still closes', async () => {
    const { AtlasFriendsPanel } = await import('../AtlasFriendsPanel');
    const onClose = vi.fn();
    uninstall = installControllerKeyboard();
    rtlRender(
      <AtlasFriendsPanel views={[]} labels={{}} plugins={[]} inLibrary={() => false} onClose={onClose} onSetEnabled={vi.fn()} onSetStatus={vi.fn()} onSetPlaying={vi.fn()} onReconnect={vi.fn()} onOpenGame={vi.fn()} />,
    );
    const tabs = screen.getAllByRole('tab');
    expect(tabs.length).toBeGreaterThan(1);
    tabs.forEach((t, i) => rect(t, i * 100, 0));
    tabs[0].focus();
    fireEvent.keyDown(tabs[0], { key: 'ArrowRight' });
    expect(document.activeElement).toBe(tabs[1]);
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' });
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
