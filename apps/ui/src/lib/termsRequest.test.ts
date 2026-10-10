import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Effect } from 'effect';
import type { Game, GameTerm } from '../types';

const api = vi.hoisted(() => ({ getGameTerms: vi.fn() }));

vi.mock('../effect/runtime', () => ({
  LauncherApi: {
    getGameTerms: (...args: unknown[]) => Effect.tryPromise({
      try: () => api.getGameTerms(...args) as Promise<GameTerm[]>,
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  },
}));

import { ensureTerms, setTermsHost, type TermsRequest } from './termsRequest';

const game = { id: 'p.echo.licensed', title: 'Echo Licensed' } as Game;
const eula: GameTerm = { id: 'eula', title: 'Licence', version: '1' };

let unregister: () => void = () => {};
beforeEach(() => { api.getGameTerms.mockReset(); });
afterEach(() => unregister());

describe('ensureTerms', () => {
  it('lets the install go on without a dialog when nothing is pending', async () => {
    api.getGameTerms.mockResolvedValue([{ ...eula, accepted_at: 5 }]);
    const host = vi.fn();
    unregister = setTermsHost(host);
    await expect(ensureTerms(game)).resolves.toBe(true);
    expect(host).not.toHaveBeenCalled();
  });

  it('asks the host for the pending terms and follows its answer', async () => {
    api.getGameTerms.mockResolvedValue([eula, { id: 'privacy', title: 'Privacy', version: '3', accepted_at: 5 }]);
    let asked: TermsRequest | undefined;
    unregister = setTermsHost((r) => { asked = r; });
    const answer = ensureTerms(game);
    await vi.waitFor(() => expect(asked).toBeDefined());
    expect(asked?.terms.map((t) => t.id)).toEqual(['eula']);
    asked!.done(true);
    await expect(answer).resolves.toBe(true);
  });

  it('a cancel stops the install', async () => {
    api.getGameTerms.mockResolvedValue([eula]);
    let asked: TermsRequest | undefined;
    unregister = setTermsHost((r) => { asked = r; });
    const answer = ensureTerms(game);
    await vi.waitFor(() => expect(asked).toBeDefined());
    asked!.done(false);
    await expect(answer).resolves.toBe(false);
  });

  it('without a host to show the dialog, nothing is accepted', async () => {
    api.getGameTerms.mockResolvedValue([eula]);
    unregister = setTermsHost(null);
    await expect(ensureTerms(game)).resolves.toBe(false);
  });

  it('a store that cannot list the terms fails the install', async () => {
    api.getGameTerms.mockRejectedValue(new Error('the store is down'));
    unregister = setTermsHost(() => {});
    await expect(ensureTerms(game)).rejects.toThrow('the store is down');
  });
});
