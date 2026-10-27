import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game } from '../../types';

const api = vi.hoisted(() => ({
  setCustomGameYear: vi.fn(),
  getLauncherSettings: vi.fn(),
  previewCommand: vi.fn(),
  getGameTerms: vi.fn(),
}));
vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { GameSettingsModal } from '../GameSettingsModal';
import { GamePage } from '../GamePage';

const custom = (o: Partial<Game> = {}): Game => ({
  id: 'custom-1', source: 'custom', store: 'custom', store_app_id: 'custom', title: 'Doom',
  cover_url: '', banner_url: '', install_dir: '/g/doom', executable: '/g/doom/doom.exe',
  is_installed: true, features: null,
  settings: { runner_id: 'auto', env_vars: {} },
  ...o,
}) as Game;

function modal(g: Game = custom(), over: Partial<React.ComponentProps<typeof GameSettingsModal>> = {}) {
  const onCustomized = vi.fn();
  render(
    <GameSettingsModal
      game={g} runners={[]} tweaks={[]} jobs={[]}
      onClose={() => {}} onSaveSettings={async () => {}} onChangeSettings={() => {}}
      onJob={() => {}} onJobAction={() => {}} onNotify={() => {}}
      onChanged={() => {}} onCustomized={onCustomized} onCopied={() => {}}
      {...over}
    />
  );
  return { onCustomized };
}

beforeEach(() => {
  Object.values(api).forEach((f) => f.mockReset());
  api.getLauncherSettings.mockResolvedValue({});
  api.previewCommand.mockResolvedValue('doom');
  api.getGameTerms.mockResolvedValue([]);
});

describe('CustomGameYear (issue #35)', () => {
  it('shows the saved year for a sideloaded game', () => {
    modal(custom({ release_year: 1993 }));
    expect((screen.getByLabelText('Release year') as HTMLInputElement).value).toBe('1993');
  });

  it('hides the year field for store games', () => {
    modal(custom({ source: 'steam', store: 'steam', store_app_id: '10', executable: '' }));
    expect(screen.queryByLabelText('Release year')).toBeNull();
  });

  it('saves a new year and reloads the library entry', async () => {
    api.setCustomGameYear.mockResolvedValue({});
    const { onCustomized } = modal(custom());
    fireEvent.change(screen.getByLabelText('Release year'), { target: { value: '2011' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save year' }));
    await waitFor(() => expect(api.setCustomGameYear).toHaveBeenCalledWith('custom-1', 2011));
    await waitFor(() => expect(onCustomized).toHaveBeenCalled());
  });

  it('clears the year back to unknown', async () => {
    api.setCustomGameYear.mockResolvedValue({});
    modal(custom({ release_year: 1993 }));
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }));
    await waitFor(() => expect(api.setCustomGameYear).toHaveBeenCalledWith('custom-1', 0));
  });

  it('refuses a bad year before calling the backend', async () => {
    modal(custom());
    fireEvent.change(screen.getByLabelText('Release year'), { target: { value: '1899' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save year' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/between 1970/);
    expect(api.setCustomGameYear).not.toHaveBeenCalled();
  });

  it('shows a backend refusal', async () => {
    api.setCustomGameYear.mockRejectedValue(new Error('unknown game "custom-1"'));
    modal(custom());
    fireEvent.change(screen.getByLabelText('Release year'), { target: { value: '2011' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save year' }));
    expect(await screen.findByText(/unknown game/)).toBeInTheDocument();
  });
});

describe('GamePage release row (issue #35)', () => {
  function gamePage(g: Game) {
    render(
      <GamePage
        game={g} runners={[]} activity={[]} backLabel="Library"
        onBack={() => {}} onPlay={() => {}} onStop={() => {}} onForceStop={() => {}}
        launching={false} tweaks={[]}
        onChangeSettings={() => {}} onSaveSettings={async () => {}}
        onJob={() => {}} onJobAction={() => {}} onAllowFolder={() => {}} onNavigate={() => {}} jobs={[]}
      />
    );
  }

  it('shows the year in the details for a sideloaded game', async () => {
    gamePage(custom({ release_year: 1993 }));
    const grid = await screen.findByTestId('details-grid');
    expect(grid.textContent).toMatch(/Release/);
    expect(grid.textContent).toMatch(/1993/);
  });

  it('shows no release row when the year is unknown', async () => {
    gamePage(custom());
    await waitFor(() => expect(screen.queryByTestId('details-grid')).toBeNull());
  });
});
