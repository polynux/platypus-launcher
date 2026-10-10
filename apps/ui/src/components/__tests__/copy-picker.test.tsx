import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, GameCopy, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({ setPreferredCopy: vi.fn() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { CopyPicker } from '../CopyPicker';

const plugin = (o: Partial<StorePlugin> = {}): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['family'], declared: ['family'], ...o }) as StorePlugin;

const ana: GameCopy = { owner_id: 'ana', owner_name: 'Ana', dlc: [{ id: 'soundtrack', name: 'Soundtrack', kind: 'dlc' }], note: "Ana's copy has the soundtrack", available: true };
const ben: GameCopy = { owner_id: 'ben', owner_name: 'Ben', dlc: [{ id: 'soundtrack', name: 'Soundtrack', kind: 'dlc' }, { id: 'track-pack', name: 'Track pack', kind: 'dlc' }], available: true, in_use: true };
const carl: GameCopy = { owner_id: 'carl', owner_name: 'Carl', dlc: [], available: false };

const borrowed = (o: Partial<Game> = {}): Game =>
  ({
    id: 'p.echo.shared', source: 'echo', store: 'echo', store_app_id: 'shared', title: 'Echo Shared Racer', is_installed: true,
    settings: {}, features: ['family'], shared_by: { id: 'ana', name: 'Ana' }, copies: [ana, ben, carl], preferred_copy: '', ...o,
  }) as Game;

beforeEach(() => {
  vi.clearAllMocks();
  api.setPreferredCopy.mockResolvedValue(undefined);
});

describe('CopyPicker', () => {
  it('shows nothing for the user\'s own game', () => {
    const { container } = render(<CopyPicker game={borrowed({ shared_by: null, copies: null })} plugin={plugin()} onChanged={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows nothing when the family capability is off for the plugin', () => {
    const { container } = render(<CopyPicker game={borrowed()} plugin={plugin({ capabilities: [], declared: ['family'] })} onChanged={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('names the copy the game is played from, and lists the copies', () => {
    render(<CopyPicker game={borrowed()} plugin={plugin()} onChanged={() => {}} />);
    expect(screen.getByText('Copy: from Ana')).toBeInTheDocument();
    const radios = screen.getAllByRole('radio');
    expect(radios).toHaveLength(3);
    expect(radios[0]).toHaveAttribute('aria-checked', 'true');
    expect(radios[1]).toHaveTextContent('Ben');
    expect(radios[1]).toHaveTextContent('Soundtrack, Track pack');
    expect(radios[1]).toHaveTextContent('in use');
    expect(radios[2]).toBeDisabled();
    expect(radios[2]).toHaveTextContent('unavailable');
  });

  it('follows the pick the user made', () => {
    render(<CopyPicker game={borrowed({ preferred_copy: 'ben' })} plugin={plugin()} onChanged={() => {}} />);
    expect(screen.getByText('Copy: from Ben')).toBeInTheDocument();
    expect(screen.getAllByRole('radio')[1]).toHaveAttribute('aria-checked', 'true');
  });

  it('sends the pick, then asks the library to reload', async () => {
    const onChanged = vi.fn();
    render(<CopyPicker game={borrowed()} plugin={plugin()} onChanged={onChanged} />);
    fireEvent.click(screen.getAllByRole('radio')[1]);
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(api.setPreferredCopy).toHaveBeenCalledWith(expect.objectContaining({ id: 'p.echo.shared' }), 'ben');
  });

  it('keeps the old copy and says why when the store refuses the pick', async () => {
    api.setPreferredCopy.mockRejectedValue(new Error('the copy from Ben isn\'t available right now'));
    const onChanged = vi.fn();
    render(<CopyPicker game={borrowed()} plugin={plugin()} onChanged={onChanged} />);
    fireEvent.click(screen.getAllByRole('radio')[1]);
    expect(await screen.findByRole('alert')).toHaveTextContent('the copy from Ben isn\'t available right now');
    expect(onChanged).not.toHaveBeenCalled();
    expect(screen.getByText('Copy: from Ana')).toBeInTheDocument();
  });

  it('shows no choice when only one copy is lent', () => {
    render(<CopyPicker game={borrowed({ copies: [ana] })} plugin={plugin()} onChanged={() => {}} />);
    expect(screen.getByText('Copy: from Ana')).toBeInTheDocument();
    expect(screen.queryByRole('radiogroup')).toBeNull();
  });
});
