// i18n: converted
// Library list-detail selection and keys (issue #126).
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render as rtlRender, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LibraryListDetail } from '../LibraryListDetail';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/theme')>();
  return { ...actual, useVariant: (axis: string) => axis === 'shell.navigation' ? 'sourcebar' : actual.useVariant(axis as never) };
});

afterEach(() => cleanup());
const render = (ui: React.ReactElement) => rtlRender(ui, { wrapper: TooltipProvider });

const mk = (id: string, o: Partial<Game> = {}): Game =>
  ({ id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '', install_dir: '', executable: '', is_installed: true, settings: {}, last_played_at: 1700000000, ...o }) as Game;

const games = () => [
  mk('alpha', { title: 'Alpha', last_played_at: 1700000300 }),
  mk('beta', { title: 'Beta', last_played_at: 1700000200, is_installed: false }),
  mk('gamma', { title: 'Gamma', last_played_at: 1700000100 }),
];

function list(props: Partial<React.ComponentProps<typeof LibraryListDetail>> = {}) {
  const onPlay = vi.fn();
  const r = render(
    <LibraryListDetail
      games={games()}
      ownership="owned"
      source="all"
      onPlay={onPlay}
      renderStage={(g) => <div data-testid="stage">{g.title}</div>}
      {...props}
    />,
  );
  return { ...r, onPlay };
}

describe('library list-detail', () => {
  it('selects the first game and shows it on the stage', () => {
    list();
    expect(screen.getByTestId('stage')).toHaveTextContent('Alpha');
    expect(screen.getByRole('button', { name: /Alpha/ }).getAttribute('aria-current')).toBe('true');
  });

  it('keeps the selection across filter changes when possible', async () => {
    const user = userEvent.setup();
    list();
    await user.click(screen.getByRole('button', { name: /Gamma/ }));
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    const filter = screen.getByPlaceholderText('Filter');
    await user.type(filter, 'a');
    // Gamma still matches "a", so the stage keeps it.
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    await user.clear(filter);
    await user.type(filter, 'beta');
    // Only Beta matches now, so the stage follows it.
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
  });

  it('moves with j/k and the arrow keys, Enter plays', async () => {
    const user = userEvent.setup();
    const { onPlay } = list();
    const alpha = screen.getByRole('button', { name: /Alpha/ });
    alpha.focus();
    await user.keyboard('j');
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
    await user.keyboard('{ArrowDown}');
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    await user.keyboard('k');
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
    await user.keyboard('{Enter}');
    expect(onPlay).toHaveBeenCalledWith('beta');
  });

  it('launches the selected row exactly once on Enter', async () => {
    const user = userEvent.setup();
    const { onPlay } = list();
    await user.click(screen.getByRole('button', { name: /Gamma/ }));
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    onPlay.mockClear();
    // Focus is on the Gamma row; Enter used to play twice (the row handler
    // plus the list handler).
    await user.keyboard('{Enter}');
    expect(onPlay).toHaveBeenCalledTimes(1);
    expect(onPlay).toHaveBeenCalledWith('gamma');
  });

  it('moves and launches from the page body, and slash focuses the filter', () => {
    const { onPlay } = list();
    fireEvent.keyDown(document.body, { key: 'j' });
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
    fireEvent.keyDown(document.body, { key: '/' });
    expect(document.activeElement).toBe(screen.getByPlaceholderText('Filter'));
    document.body.focus();
    fireEvent.keyDown(document.body, { key: 'Enter' });
    expect(onPlay).toHaveBeenCalledWith('beta');
  });

  it('marks only the selected row current', () => {
    list();
    expect(screen.getByRole('button', { name: /Alpha/ }).getAttribute('aria-current')).toBe('true');
    expect(screen.getByRole('button', { name: /Beta/ }).getAttribute('aria-current')).toBeNull();
  });

  it('shows Not installed greyed and the empty filter result', async () => {
    const user = userEvent.setup();
    list();
    expect(screen.getByText('Not installed')).toBeInTheDocument();
    const filter = screen.getByPlaceholderText('Filter');
    await user.type(filter, 'zzz-no-match');
    expect(screen.getByText('Nothing here.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Show all games' }));
    expect(screen.getByPlaceholderText('Filter')).toHaveValue('');
  });

  it('Esc clears the filter and Down moves into the list', async () => {
    const user = userEvent.setup();
    list();
    const filter = screen.getByPlaceholderText('Filter') as HTMLInputElement;
    await user.type(filter, 'alpha');
    expect(filter.value).toBe('alpha');
    await user.keyboard('{Escape}');
    expect(filter.value).toBe('');
    await user.type(filter, 'a');
    await user.keyboard('{ArrowDown}');
    // Focus left the filter for a game row.
    expect(document.activeElement?.getAttribute('data-game-row')).toBeTruthy();
  });

  it('selects the game opened from elsewhere', () => {
    list({ initialSelectedId: 'gamma' });
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
    expect(screen.getByRole('button', { name: /Gamma/ }).getAttribute('aria-current')).toBe('true');
  });
});

describe('atlas parity (issue #132)', () => {
  it('adds a game from the list header', async () => {
    const user = userEvent.setup();
    const onAddGame = vi.fn();
    list({ onAddGame });
    await user.click(screen.getByRole('button', { name: 'Add a game' }));
    expect(onAddGame).toHaveBeenCalledTimes(1);
  });

  it('filters by tier with a dot badge and resets with Show all games', async () => {
    const user = userEvent.setup();
    list();
    expect(screen.getByTestId('stage')).toHaveTextContent('Alpha');
    // None of the fixture games has a tier, so platinum empties the list.
    await user.click(screen.getByRole('button', { name: 'Library filters' }));
    fireEvent.pointerDown(screen.getByLabelText('Filter by tier'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Platinum on ProtonDB' }));
    expect(screen.getByText('Nothing here.')).toBeInTheDocument();
    // The filter button carries a dot while a filter is active.
    expect(screen.getByRole('button', { name: 'Library filters' }).querySelector('[aria-label="needs attention"]')).toBeTruthy();
    await user.click(screen.getByRole('button', { name: 'Show all games' }));
    expect(screen.getByTestId('stage')).toHaveTextContent('Alpha');
  });

  it('shows source warnings with a link to Plugins', async () => {
    const user = userEvent.setup();
    const onOpenPlugins = vi.fn();
    list({
      warnings: [
        { source: 'steam', needs_auth: false, message: 'boom' },
        { source: 'gog', needs_auth: false, message: 'bam' },
      ],
      onOpenPlugins,
    });
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByText('2 sources need attention')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Plugins' }));
    expect(onOpenPlugins).toHaveBeenCalledTimes(1);
  });

  it('shows the login wording for logged-out sources', async () => {
    const user = userEvent.setup();
    const onOpenPlugins = vi.fn();
    list({
      warnings: [{ source: 'steam', needs_auth: true, message: 'logged out' }],
      pluginName: () => 'Steam',
      onOpenPlugins,
    });
    expect(screen.getByText(/is logged out/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Log in' }));
    expect(onOpenPlugins).toHaveBeenCalledTimes(1);
  });
});

describe('atlas parity round 2 (issue #132)', () => {
  it('opens the filters as a popover with human values and a reset entry', async () => {
    const user = userEvent.setup();
    list();
    expect(screen.getByTestId('stage')).toHaveTextContent('Alpha');
    await user.click(screen.getByRole('button', { name: 'Library filters' }));
    // A real popover: portaled to the body, so the list never moves.
    const menu = document.body.querySelector('[role="menu"]');
    expect(menu).toBeTruthy();
    expect(document.body.contains(screen.getByText('Compatibility'))).toBe(true);
    // Human values in Albert Sans, not uppercase mono.
    fireEvent.pointerDown(screen.getByLabelText('Filter by tier'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    const any = screen.getByRole('option', { name: 'Any' });
    expect(any.className).toMatch(/font-sans/);
    expect(any.className).not.toMatch(/uppercase/);
    fireEvent.click(screen.getByRole('option', { name: 'Platinum on ProtonDB' }));
    expect(screen.getByText('Nothing here.')).toBeInTheDocument();
    // The popover stays open for the next filter; Reset clears without leaving.
    expect(document.body.querySelector('[role="menu"]')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Reset filters' }));
    expect(screen.getByTestId('stage')).toHaveTextContent('Alpha');
  });

  it('opens a row menu on right-click with play and collections', async () => {
    const user = userEvent.setup();
    const onPlay = vi.fn();
    const onManage = vi.fn();
    const onSettings = vi.fn();
    const onInstall = vi.fn();
    render(
      <LibraryListDetail
        games={[games()[0], { ...games()[1], installable: true }]}
        ownership="owned"
        source="all"
        onPlay={onPlay}
        renderStage={(g) => <div data-testid="stage">{g.title}</div>}
        onManageCollections={onManage}
        onOpenSettings={onSettings}
        onInstall={onInstall}
      />,
    );
    const alpha = screen.getByRole('button', { name: /Alpha/ });
    fireEvent.contextMenu(alpha);
    expect(screen.getByRole('menuitem', { name: 'Play' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: 'Game settings' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: 'Collections…' })).toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: 'Collections…' }));
    expect(onManage).toHaveBeenCalledWith('alpha');
    fireEvent.contextMenu(screen.getByRole('button', { name: /Beta/ }));
    // An uninstalled but installable game offers Install instead of Play.
    expect(screen.queryByRole('menuitem', { name: 'Play' })).toBeNull();
    await user.click(screen.getByRole('menuitem', { name: 'Install' }));
    expect(onInstall).toHaveBeenCalledWith(expect.objectContaining({ id: 'beta' }));
  });

  it('opens the row menu with the keyboard and skips it without handlers', async () => {
    const onManage = vi.fn();
    const { unmount } = render(
      <LibraryListDetail
        games={games()}
        ownership="owned"
        source="all"
        onPlay={() => {}}
        renderStage={(g) => <div data-testid="stage">{g.title}</div>}
        onManageCollections={onManage}
      />,
    );
    const alpha = screen.getByRole('button', { name: /Alpha/ });
    alpha.focus();
    fireEvent.keyDown(alpha, { key: 'ContextMenu' });
    expect(screen.getByRole('menuitem', { name: 'Collections…' })).toBeInTheDocument();
    unmount();
    cleanup();
    list();
    fireEvent.contextMenu(screen.getByRole('button', { name: /Alpha/ }));
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('notes the active collection titles missing from the library', () => {
    list({
      collections: [{ id: 'couch', name: 'Couch co-op', games: ['alpha', 'gone-id'], titles: { 'gone-id': 'Gone Game' } }],
      collection: 'couch',
    });
    const note = screen.getByText('1 not in the library');
    expect(note.title).toBe('Gone Game');
  });
});

describe('atlas parity round 3 (issue #132)', () => {
  it('sorts by title, year, size and tier in both directions', async () => {
    const user = userEvent.setup();
    list();
    const order = () => [...document.querySelectorAll('[data-game-row]')].map((el) => el.getAttribute('data-game-row'));
    expect(order()).toEqual(['alpha', 'beta', 'gamma']);
    fireEvent.pointerDown(screen.getByLabelText('Sort games'), { button: 0, ctrlKey: false, pointerType: 'mouse' });
    // The classic keys sit beside the compact ones, in the same select.
    expect(screen.getByRole('option', { name: 'Title' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Year' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Size' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Tier' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('option', { name: 'Title' }));
    expect(order()).toEqual(['alpha', 'beta', 'gamma']);
    await user.click(screen.getByRole('button', { name: 'Ascending, switch to descending' }));
    expect(order()).toEqual(['gamma', 'beta', 'alpha']);
    expect(screen.getByRole('button', { name: 'Descending, switch to ascending' })).toBeInTheDocument();
  });

  it('keeps native row focus so arrows and j move', async () => {
    const user = userEvent.setup();
    list();
    await user.click(screen.getByRole('button', { name: /Alpha/ }));
    expect(document.activeElement?.getAttribute('data-game-row')).toBe('alpha');
    await user.keyboard('j');
    expect(screen.getByTestId('stage')).toHaveTextContent('Beta');
    await user.keyboard('{ArrowDown}');
    expect(screen.getByTestId('stage')).toHaveTextContent('Gamma');
  });
});
