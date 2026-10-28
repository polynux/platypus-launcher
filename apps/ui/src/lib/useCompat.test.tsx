import { describe, it, expect, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { Effect } from 'effect';
import type { Compat, Game } from '../types';

const getGameCompat = vi.fn();
vi.mock('../effect/runtime', () => ({
  LauncherApi: { getGameCompat: (g: Game) => getGameCompat(g) },
}));

import { useCompat } from './useCompat';
import { clearCompatCache, setGameCompat } from './compat';

const game = { id: 'g1', store_app_id: 'x', native: false, is_installed: true } as unknown as Game;
const compat = (o: Partial<Compat> = {}): Compat => ({ native: false, ratings_online: true, protection_online: true, badges: [], ...o });

describe('useCompat', () => {
  it('takes a new answer saved by another view of the same game', async () => {
    clearCompatCache();
    getGameCompat.mockReturnValue(Effect.succeed(compat()));
    const { result } = renderHook(() => useCompat(game));
    await waitFor(() => expect(result.current.compat).not.toBeNull());
    expect(result.current.compat?.steam_app_id).toBeUndefined();

    act(() => setGameCompat('g1', compat({ steam_app_id: '1196590' })));
    expect(result.current.compat?.steam_app_id).toBe('1196590');
  });

  it('ignores an answer saved for another game', async () => {
    clearCompatCache();
    getGameCompat.mockReturnValue(Effect.succeed(compat()));
    const { result } = renderHook(() => useCompat(game));
    await waitFor(() => expect(result.current.compat).not.toBeNull());
    act(() => setGameCompat('other', compat({ steam_app_id: '620' })));
    expect(result.current.compat?.steam_app_id).toBeUndefined();
  });
});
