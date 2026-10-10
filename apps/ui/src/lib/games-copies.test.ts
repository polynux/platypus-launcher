import { describe, it, expect } from 'vitest';
import { mergeCopies } from './games';
import type { Game, GameCopy } from '../types';

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'p.echo.shared',
    source: 'echo',
    store: 'echo',
    store_app_id: 'shared',
    title: 'Echo Shared Racer',
    cover_url: '',
    banner_url: '',
    install_dir: '',
    executable: '',
    is_installed: true,
    settings: { runner_id: 'x' } as Game['settings'],
    ...over,
  };
}

const ana: GameCopy = { owner_id: 'ana', owner_name: 'Ana', dlc: [], available: true };
const ben: GameCopy = { owner_id: 'ben', owner_name: 'Ben', dlc: [], available: true };

describe('family copies merge', () => {
  it('takes only the family fields from a fresh library', () => {
    const prev = [game({ title: 'Kept', copies: [ana, ben], preferred_copy: '' })];
    const fresh = [game({ title: 'Renamed', is_installed: false, copies: [ana, ben], preferred_copy: 'ben' })];
    const [merged] = mergeCopies(prev, fresh);
    expect(merged.title).toBe('Kept');
    expect(merged.is_installed).toBe(true);
    expect(merged.settings).toEqual({ runner_id: 'x' });
    expect(merged.preferred_copy).toBe('ben');
    expect(merged.copies).toEqual([ana, ben]);
  });

  it('keeps the same list when no copy changed', () => {
    const prev = [game({ copies: [ana, ben], preferred_copy: 'ben' })];
    expect(mergeCopies(prev, [game({ copies: [ana, ben], preferred_copy: 'ben' })])).toBe(prev);
  });

  it('leaves games absent from the fresh list alone', () => {
    const prev = [game({ id: 'p.echo.other', preferred_copy: 'ana' })];
    expect(mergeCopies(prev, [game({ preferred_copy: 'ben' })])).toBe(prev);
  });
});
