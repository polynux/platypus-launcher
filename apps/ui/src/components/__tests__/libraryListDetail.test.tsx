// i18n: converted
// Library list-detail selection and keys (issue #126).
import React from 'react';
import { describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, render as rtlRender, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LibraryListDetail } from '../LibraryListDetail';
import { TooltipProvider } from '../ds/tooltip';
import type { Game } from '../../types';

vi.mock('../../effect/runtime', () => ({ LauncherApi: {} }));

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
