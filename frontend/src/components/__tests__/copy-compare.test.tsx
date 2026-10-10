import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import type { CopyComparison, Game, GameCopy, StorePlugin } from '../../types';

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

const plugin = { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['family'], declared: ['family'] } as unknown as StorePlugin;

// Ana has an expansion and the soundtrack; Ben has the track pack, an art book and the soundtrack.
const ana: GameCopy = {
  owner_id: 'ana', owner_name: 'Ana', available: true, note: "Ana's copy has the expansion",
  dlc: [{ id: 'expansion', name: 'Expansion One', kind: 'dlc' }, { id: 'soundtrack', name: 'Soundtrack', kind: 'soundtrack' }],
};
const ben: GameCopy = {
  owner_id: 'ben', owner_name: 'Ben', available: true, exclude: 'The expansion isn\'t lent from this copy',
  dlc: [{ id: 'track-pack', name: 'Track pack', kind: 'dlc' }, { id: 'soundtrack', name: 'Soundtrack', kind: 'soundtrack' }, { id: 'art-book', name: 'Art book', kind: 'other' }],
};
/** The table row of an add-on, found by the name its first cell starts with. */
function rowFor(table: HTMLElement, name: string): HTMLElement | undefined {
  return within(table).getAllByRole('row').find((r) => r.querySelector('td')?.textContent?.startsWith(name));
}

const compare: CopyComparison = {
  baseline: 'ben',
  diffs: [{ owner_id: 'ana', only_here: 1, only_baseline: 2, shared: 0, soundtracks_only_here: 0, soundtracks_only_baseline: 0, soundtracks_shared: 1 }],
};

const borrowed = (o: Partial<Game> = {}): Game =>
  ({
    id: 'p.echo.shared', source: 'echo', store: 'echo', store_app_id: 'shared', title: 'Echo Shared Racer', is_installed: true,
    settings: {}, features: ['family'], shared_by: { id: 'ana', name: 'Ana' }, copies: [ana, ben], preferred_copy: '', copy_compare: compare, ...o,
  }) as Game;

beforeEach(() => {
  vi.clearAllMocks();
});

describe('CopyPicker summary and compare', () => {
  it('says, per copy, how it differs from the baseline', () => {
    render(<CopyPicker game={borrowed()} plugin={plugin} onChanged={() => {}} />);
    expect(screen.getByText("Ana's copy has 1 more and 2 fewer DLC than Ben's.")).toBeInTheDocument();
    expect(screen.getByText('The expansion isn\'t lent from this copy')).toBeInTheDocument();
  });

  it('a copy that matches the baseline has no summary line', () => {
    render(<CopyPicker game={borrowed({ copy_compare: { baseline: 'ben', diffs: [{ ...compare.diffs[0], only_here: 0, only_baseline: 0, soundtracks_shared: 1 }] } })} plugin={plugin} onChanged={() => {}} />);
    expect(screen.getByText("Ana's copy has the same add-ons as Ben's.")).toBeInTheDocument();
  });

  it('opens the table with the baseline first, differences only, soundtracks hidden', () => {
    render(<CopyPicker game={borrowed()} plugin={plugin} onChanged={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Compare copies' }));
    const table = screen.getByRole('table', { name: 'Add-ons by copy' });
    const headers = within(table).getAllByRole('columnheader').map((h) => h.textContent);
    expect(headers[0]).toBe('Add-on');
    expect(headers[1]).toContain('Ben');
    expect(headers[2]).toContain('Ana');
    expect(within(table).getByText('Expansion One')).toBeInTheDocument();
    expect(within(table).getByText('Track pack')).toBeInTheDocument();
    expect(rowFor(table, 'Soundtrack')).toBeUndefined();
    expect(rowFor(table, 'Art book')).toBeInTheDocument();
  });

  it('shows the shared soundtrack when soundtracks are on, and every add-on when differences-only is off', () => {
    render(<CopyPicker game={borrowed()} plugin={plugin} onChanged={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Compare copies' }));
    fireEvent.click(screen.getByRole('switch', { name: 'Show soundtracks' }));
    const table = screen.getByRole('table', { name: 'Add-ons by copy' });
    // Both copies have the soundtrack: it is not a difference, so it stays out until differences-only is off.
    expect(rowFor(table, 'Soundtrack')).toBeUndefined();
    fireEvent.click(screen.getByRole('switch', { name: 'Differences only' }));
    expect(rowFor(table, 'Soundtrack')).toBeInTheDocument();
    expect(within(table).getAllByRole('row')).toHaveLength(1 + 4); // header row + the four add-ons
  });

  it('marks who has each add-on with a check or a dash', () => {
    render(<CopyPicker game={borrowed()} plugin={plugin} onChanged={() => {}} />);
    fireEvent.click(screen.getByRole('button', { name: 'Compare copies' }));
    fireEvent.click(screen.getByRole('switch', { name: 'Show soundtracks' }));
    fireEvent.click(screen.getByRole('switch', { name: 'Differences only' }));
    const table = screen.getByRole('table', { name: 'Add-ons by copy' });
    expect(within(rowFor(table, 'Track pack')!).getByLabelText('Ana doesn\'t have it')).toBeInTheDocument();
    expect(within(rowFor(table, 'Track pack')!).getByLabelText('Ben has it')).toBeInTheDocument();
    expect(within(rowFor(table, 'Soundtrack')!).getByLabelText('Ana has it')).toBeInTheDocument();
  });

  it('shows no compare button for a single copy', () => {
    render(<CopyPicker game={borrowed({ copies: [ana], copy_compare: null })} plugin={plugin} onChanged={() => {}} />);
    expect(screen.queryByRole('button', { name: 'Compare copies' })).not.toBeInTheDocument();
  });
});
