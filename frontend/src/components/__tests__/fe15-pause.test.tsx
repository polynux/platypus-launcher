import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import { DownloadsView } from '../DownloadsView';
import type { Job } from '../../types';

function job(over: Partial<Job> = {}): Job {
  return {
    id: Math.random().toString(36).slice(2),
    kind: 'install',
    source: 's',
    game_id: 'g',
    store_game_id: 'a',
    title: 'Goose',
    state: 'done',
    bytes: 0,
    total: 0,
    speed: 0,
    percent: -1,
    attempts: 1,
    created_at: 0,
    updated_at: 0,
    ...over,
  };
}

const noop = () => {};

describe('FE-15 pause/resume/discard buttons and states', () => {
  it('offers Pause on a running download and Resume on a paused one', () => {
    const actions: Array<[string, string]> = [];
    const onAction = (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => actions.push([action, id]);
    const { getByLabelText, getByText, rerender } = render(
      <DownloadsView jobs={[job({ id: 'j1', state: 'running', percent: 42 })]} onAction={onAction} onClear={noop} onOpenGame={noop} />
    );
    fireEvent.click(getByLabelText('Pause Goose'));
    expect(actions).toEqual([['pause', 'j1']]);

    rerender(<DownloadsView jobs={[job({ id: 'j1', state: 'paused', percent: 42 })]} onAction={onAction} onClear={noop} onOpenGame={noop} />);
    expect(getByText(/resume continues where it stopped/)).toBeTruthy();
    fireEvent.click(getByLabelText('Resume Goose'));
    expect(actions).toEqual([['pause', 'j1'], ['resume', 'j1']]);
  });

  it('keeps a paused download in the queue section and offers Resume all', () => {
    const onAction = vi.fn();
    const { getByText } = render(
      <DownloadsView
        jobs={[job({ id: 'run', state: 'running' }), job({ id: 'pa', state: 'paused' }), job({ id: 'done', state: 'done' })]}
        onAction={onAction}
        onClear={noop}
        onOpenGame={noop}
      />
    );
    // "paused" appears in the queue section, next to the running one.
    const queue = getByText('queue').closest('section')!;
    expect(queue.textContent).toContain('paused');
    expect(queue.textContent).not.toContain('done');
    fireEvent.click(getByText('Resume all'));
    expect(onAction).toHaveBeenCalledWith('resume', 'pa');
  });

  it('offers Resume and Discard data on a canceled download until it is cleaned', () => {
    const actions: Array<[string, string]> = [];
    const onAction = (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => actions.push([action, id]);
    const { getByLabelText, queryByLabelText, getByText, rerender } = render(
      <DownloadsView jobs={[job({ id: 'c1', state: 'canceled', canceled_at: 1 })]} onAction={onAction} onClear={noop} onOpenGame={noop} />
    );
    fireEvent.click(getByLabelText('Resume Goose'));
    fireEvent.click(getByLabelText('Discard downloaded data of Goose'));
    expect(actions).toEqual([['resume', 'c1'], ['discard', 'c1']]);

    // Once the leftovers are gone, neither action makes sense.
    rerender(<DownloadsView jobs={[job({ id: 'c1', state: 'canceled', canceled_at: 1, cleaned_at: 2, leftover: 'no discard capability' })]} onAction={onAction} onClear={noop} onOpenGame={noop} />);
    expect(queryByLabelText('Discard downloaded data of Goose')).toBeNull();
    expect(queryByLabelText('Resume Goose')).toBeNull();
    expect(getByText('Retry')).toBeTruthy();
  });

  it('shows what may remain on disk when the plugin could not clean up', () => {
    const { getByText } = render(
      <DownloadsView
        jobs={[job({ id: 'c2', state: 'canceled', cleaned_at: 3, leftover: 'may remain: partial install folder' })]}
        onAction={noop}
        onClear={noop}
        onOpenGame={noop}
      />
    );
    expect(getByText('may remain: partial install folder')).toBeTruthy();
  });
});

// LauncherSettingsView talks to Wails through its own module; stub the calls
// it makes. The mocked calls return Effects: the view runs them through
// Effect.runPromise, like every other view here.
vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getLauncherSettings: () =>
      Effect.succeed({ games_folder: '/g', default_games_folder: '/g', cancel_cleanup_days: 7 } as never),
    setGamesFolder: () => Effect.succeed({ games_folder: '/g', default_games_folder: '/g', cancel_cleanup_days: 7 } as never),
    chooseGamesFolder: () => Effect.succeed('' as never),
    setCancelCleanupDays: (days: number) =>
      Effect.succeed({ games_folder: '/g', default_games_folder: '/g', cancel_cleanup_days: days } as never),
  },
}));

import { LauncherSettingsView } from '../LauncherSettingsView';

describe('FE-15 cleanup-days setting', () => {
  it('saves a changed threshold and shows it coming back from the backend', async () => {
    const { findByLabelText, findAllByRole, findByDisplayValue } = render(<LauncherSettingsView onNotify={noop} />);
    // The input starts at the saved threshold once settings arrive.
    const input = await findByLabelText("Days to keep canceled downloads' data");
    await waitFor(() => expect((input as HTMLInputElement).value).toBe('7'));
    fireEvent.change(input, { target: { value: '3' } });
    fireEvent.blur(input);
    // Two Save buttons: the games-folder form's and the cleanup form's.
    fireEvent.click((await findAllByRole('button', { name: 'Save' }))[1]);
    expect(await findByDisplayValue('3')).toBeTruthy();
  });

  it('offers 0 (delete right away) and clamps anything above 365', async () => {
    const { findByLabelText, findAllByRole, findByDisplayValue } = render(<LauncherSettingsView onNotify={noop} />);
    const input = await findByLabelText("Days to keep canceled downloads' data");
    await waitFor(() => expect((input as HTMLInputElement).value).toBe('7'));
    fireEvent.change(input, { target: { value: '0' } });
    fireEvent.blur(input);
    fireEvent.click((await findAllByRole('button', { name: 'Save' }))[1]);
    expect(await findByDisplayValue('0')).toBeTruthy();
    // The field clamps anything past the max (on blur) before it can reach the backend.
    fireEvent.change(input, { target: { value: '9999' } });
    fireEvent.blur(input);
    expect((input as HTMLInputElement).value).toBe('365');
  });
});
