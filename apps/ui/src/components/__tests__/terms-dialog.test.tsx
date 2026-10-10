import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@testing-library/react';
import { Effect } from 'effect';
import type { Game, GameTerm } from '../../types';

const api = vi.hoisted(() => ({ acceptGameTerms: vi.fn(), browserOpen: vi.fn() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: new Proxy(api, {
    get: (t, k: string) => (...args: unknown[]) => Effect.tryPromise({
      try: async () => (t as Record<string, (...a: unknown[]) => unknown>)[k](...args),
      catch: (e) => (e instanceof Error ? e : new Error(String(e))),
    }),
  }),
}));

import { TermsDialog } from '../TermsDialog';
import { onTermsAccepted } from '../../lib/terms';

const game = { id: 'p.echo.licensed', source: 'echo', title: 'Echo Licensed' } as Game;
const terms: GameTerm[] = [
  { id: 'eula', title: 'Echo end user licence', url: 'https://example.com/echo/eula', version: '2026.2' },
  { id: 'privacy', title: 'Echo privacy notice', url: 'javascript:alert(1)', version: '3' },
];

beforeEach(() => {
  vi.clearAllMocks();
  window.platypus!.openExternal = api.browserOpen;
  api.acceptGameTerms.mockResolvedValue(undefined);
});

describe('TermsDialog', () => {
  it('lists each term with its version, and links only the web ones', () => {
    render(<TermsDialog game={game} terms={terms} onAccepted={() => {}} onCancel={() => {}} />);
    expect(screen.getByText('Echo end user licence')).toBeInTheDocument();
    expect(screen.getByText('version 2026.2')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /Read/ })).toHaveLength(1);
  });

  it('opens a term in the browser', () => {
    render(<TermsDialog game={game} terms={terms} onAccepted={() => {}} onCancel={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: /Read/ }));
    expect(api.browserOpen).toHaveBeenCalledWith('https://example.com/echo/eula');
  });

  it('accepts exactly the versions it shows, then reports it', async () => {
    const onAccepted = vi.fn();
    const onTermsChanged = vi.fn();
    const unsubscribe = onTermsAccepted(onTermsChanged);
    render(<TermsDialog game={game} terms={terms} onAccepted={onAccepted} onCancel={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'I accept' }));
    await waitFor(() => expect(onAccepted).toHaveBeenCalled());
    expect(onTermsChanged).toHaveBeenCalledTimes(1);
    unsubscribe();
    expect(api.acceptGameTerms).toHaveBeenCalledWith(game, [{ id: 'eula', version: '2026.2' }, { id: 'privacy', version: '3' }]);
  });

  it('cancel accepts nothing', () => {
    const onCancel = vi.fn();
    render(<TermsDialog game={game} terms={terms} onAccepted={() => {}} onCancel={onCancel} />);
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalled();
    expect(api.acceptGameTerms).not.toHaveBeenCalled();
  });

  it('a refused acceptance says why and keeps the dialog open', async () => {
    api.acceptGameTerms.mockRejectedValue(new Error('the terms of Echo Licensed changed; look at them again'));
    const onAccepted = vi.fn();
    render(<TermsDialog game={game} terms={terms} onAccepted={onAccepted} onCancel={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'I accept' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('changed');
    expect(onAccepted).not.toHaveBeenCalled();
  });
});
