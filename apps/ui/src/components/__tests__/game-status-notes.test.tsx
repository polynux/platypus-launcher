import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

const api = vi.hoisted(() => ({
  getGameDetails: vi.fn(),
  getNetState: vi.fn(),
  getGameCompat: vi.fn(),
  setGameStatus: vi.fn(),
  setGameNotes: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy({}, { get: (_t, k: string) => (...a: unknown[]) => (api as Record<string, (...x: unknown[]) => unknown>)[k](...a) }),
  onJobs: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';
import type { Game, GameStatusInfo } from '../../types';

const ok = <T,>(v: T) => Effect.succeed(v);

const settings = { runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false, use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '' };
const game = (o: Partial<Game> = {}) =>
  ({ id: 'p.echo.tiny', source: 'echo', store: 'echo', store_app_id: 'tiny', title: 'Tiny', cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true, settings, ...o }) as unknown as Game;

const props = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() {}, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {},
};

const pair = (o: Partial<GameStatusInfo> = {}): GameStatusInfo => ({ status: 'playing', user_notes: '', ...o });

const page = (g: Game, onChanged?: () => void) => (mounted = render(<GamePage game={g} {...props} onCustomized={onChanged} />));

/** Re-render the mounted GamePage with a different game (navigation). */
let mounted: ReturnType<typeof render> | null = null;
const navigateTo = (g: Game) => mounted!.rerender(<GamePage game={g} {...props} />);

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getGameDetails.mockReturnValue(ok({ summary: '', screenshots: [], genres: [], developers: [], publishers: [], release_date: '', achievements: null } as never));
  api.getNetState.mockReturnValue(ok({ online: true, since: 0 } as never));
  api.getGameCompat.mockReturnValue(ok({} as never));
});

describe('game status select', () => {
  it('saves at once, applies the saved pair and tells the owner', async () => {
    const onChanged = vi.fn();
    api.setGameStatus.mockReturnValue(ok(pair()));
    page(game(), onChanged);
    const trigger = await screen.findByTestId('status-select');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Playing' }));
    await waitFor(() => expect(api.setGameStatus).toHaveBeenCalledWith('p.echo.tiny', 'playing'));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("shows the core's refusal inline and keeps the previous status", async () => {
    api.setGameStatus.mockReturnValue(ok(pair({ status: 'playing' })));
    page(game());
    let trigger = await screen.findByTestId('status-select');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Playing' }));
    await waitFor(() => expect(api.setGameStatus).toHaveBeenCalledTimes(1));

    api.setGameStatus.mockReturnValue(Effect.fail(new Error('game status must be backlog, playing, finished or abandoned, got "x"')));
    trigger = screen.getByTestId('status-select');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false, pointerType: 'mouse' });
    fireEvent.click(screen.getByRole('option', { name: 'Finished' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/backlog, playing, finished or abandoned/);
    // The select stays on the saved value: a refusal never lands.
    expect(screen.getByTestId('status-select')).toHaveTextContent(/playing/i);
    expect(screen.getByTestId('status-select')).not.toHaveTextContent(/finished/i);
  });
});

describe('game notes autosave', () => {
  it('saves a burst of typing once, debounced, with the save label', async () => {
    api.setGameNotes.mockReturnValue(ok(pair({ status: '', user_notes: 'abc' })));
    page(game());
    const notes = await screen.findByTestId('notes-input');
    fireEvent.change(notes, { target: { value: 'a' } });
    fireEvent.change(notes, { target: { value: 'ab' } });
    fireEvent.change(notes, { target: { value: 'abc' } });
    expect(api.setGameNotes).not.toHaveBeenCalled();
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledTimes(1), { timeout: 2000 });
    expect(api.setGameNotes).toHaveBeenCalledWith('p.echo.tiny', 'abc');
    expect(await screen.findByText('Saved')).toBeInTheDocument();
  });

  it('saves on blur right away', async () => {
    api.setGameNotes.mockReturnValue(ok(pair({ status: '', user_notes: 'typed' })));
    page(game());
    const notes = await screen.findByTestId('notes-input');
    fireEvent.change(notes, { target: { value: 'typed' } });
    fireEvent.blur(notes);
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledWith('p.echo.tiny', 'typed'), { timeout: 2000 });
  });

  it('never overlaps writes: a slow save finishes before a newer one starts', async () => {
    let releaseFirst: (v: GameStatusInfo) => void = () => {};
    api.setGameNotes
      .mockImplementationOnce(() => Effect.promise(() => new Promise((r) => { releaseFirst = r; })))
      .mockImplementation(() => ok(pair({ status: '', user_notes: 'newer' })));
    page(game());
    const notes = await screen.findByTestId('notes-input');
    fireEvent.change(notes, { target: { value: 'first' } });
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledTimes(1), { timeout: 2000 });
    expect(await screen.findByText('Saving…')).toBeInTheDocument();

    // The debounce of a newer text runs out while the first write is still in
    // flight: nothing goes out beside it.
    fireEvent.change(notes, { target: { value: 'newer' } });
    await new Promise((r) => setTimeout(r, 700));
    expect(api.setGameNotes).toHaveBeenCalledTimes(1);

    releaseFirst(pair({ status: '', user_notes: 'first' }));
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledTimes(2), { timeout: 2000 });
    expect(api.setGameNotes).toHaveBeenLastCalledWith('p.echo.tiny', 'newer');
    expect(await screen.findByText('Saved')).toBeInTheDocument();
  });

  it('labels the field Your notes, apart from the store note', async () => {
    page(game({ note: 'Needs the EA app' }));
    await screen.findByTestId('notes-input');
    expect(screen.getByText('Your notes')).toBeInTheDocument();
    expect(screen.getByLabelText('Your notes')).toBe(screen.getByTestId('notes-input'));
  });

  it('blur writes nothing when the text is unchanged', async () => {
    api.setGameNotes.mockReturnValue(ok(pair({ status: '', user_notes: 'kept' })));
    page(game({ user_notes: 'kept' }));
    const notes = await screen.findByTestId('notes-input');
    expect((notes as HTMLTextAreaElement).value).toBe('kept');
    fireEvent.blur(notes);
    await new Promise((r) => setTimeout(r, 700));
    expect(api.setGameNotes).not.toHaveBeenCalled();
  });

  it('still saves after a save of a previous game settles (never sticks on saving)', async () => {
    let releaseFirst: (v: GameStatusInfo) => void = () => {};
    api.setGameNotes
      .mockImplementationOnce(() => Effect.promise(() => new Promise((r) => { releaseFirst = r; })))
      .mockImplementation(() => ok(pair({ status: '', user_notes: 'again' })));
    page(game());
    const notes = await screen.findByTestId('notes-input');
    fireEvent.change(notes, { target: { value: 'first' } });
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledTimes(1), { timeout: 2000 });

    // Navigate away while that write is still in flight.
    navigateTo(game({ id: 'p.echo.huge', user_notes: '' }));
    const hugeNotes = await screen.findByTestId('notes-input');
    expect(api.setGameNotes).toHaveBeenCalledTimes(1);

    releaseFirst(pair({ status: '', user_notes: 'first' }));
    await waitFor(() => expect(screen.queryByText('Saving…')).not.toBeInTheDocument());

    // The new game still saves: the old write's settle freed the lock.
    fireEvent.change(hugeNotes, { target: { value: 'again' } });
    fireEvent.blur(hugeNotes);
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledTimes(2), { timeout: 2000 });
    expect(api.setGameNotes).toHaveBeenLastCalledWith('p.echo.huge', 'again');
  });

  it('flushes a debounced text for the old game when the user navigates away', async () => {
    api.setGameNotes.mockReturnValue(ok(pair({ status: '', user_notes: 'typed' })));
    page(game());
    const notes = await screen.findByTestId('notes-input');
    fireEvent.change(notes, { target: { value: 'typed' } });
    expect(api.setGameNotes).not.toHaveBeenCalled(); // still inside the debounce

    navigateTo(game({ id: 'p.echo.huge', user_notes: '' }));
    await waitFor(() => expect(api.setGameNotes).toHaveBeenCalledWith('p.echo.tiny', 'typed'), { timeout: 2000 });

    // The new game shows its own notes, not the old game's text.
    const hugeNotes = await screen.findByTestId('notes-input');
    expect((hugeNotes as HTMLTextAreaElement).value).toBe('');
  });
});
