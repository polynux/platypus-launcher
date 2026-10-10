import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, GameTerm, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({ getGameTerms: vi.fn() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));
vi.mock('../../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: vi.fn() }));

import { LegalSection } from '../LegalSection';
import { notifyTermsAccepted } from '../../lib/terms';

const game = { id: 'p.echo.licensed', source: 'echo', title: 'Echo Licensed' } as Game;
const plugin = { id: 'echo', capabilities: ['terms'], declared: ['terms'] } as unknown as StorePlugin;
const term = { id: 'eula', title: 'Echo end user licence', url: 'https://example.com/echo/eula', version: '2026.1' };

beforeEach(() => {
  vi.clearAllMocks();
});

describe('LegalSection', () => {
  it('reads the acceptance again after the user accepts terms elsewhere', async () => {
    const notAccepted: GameTerm[] = [{ ...term }];
    const accepted: GameTerm[] = [{ ...term, accepted_at: 1_700_000_000 }];
    api.getGameTerms.mockResolvedValueOnce(notAccepted).mockResolvedValue(accepted);
    render(<LegalSection game={game} plugin={plugin} />);
    expect(await screen.findByText('not accepted yet')).toBeInTheDocument();

    notifyTermsAccepted();

    await waitFor(() => expect(screen.getByText(/^accepted /)).toBeInTheDocument());
    expect(screen.queryByText('not accepted yet')).not.toBeInTheDocument();
    expect(api.getGameTerms).toHaveBeenCalledTimes(2);
  });
});
