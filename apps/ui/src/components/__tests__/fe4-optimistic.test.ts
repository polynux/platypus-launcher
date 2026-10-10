import { describe, it, expect } from 'vitest';
import { optimisticGamesAfterJob } from '../../lib/jobs';
import type { Game, Job } from '../../types';

function game(over: Partial<Game> = {}): Game {
  return {
    id: 'g1',
    source: 'epic-legendary',
    store: 'epic',
    store_app_id: 'a',
    title: 'G',
    cover_url: '',
    banner_url: '',
    install_dir: '/g',
    executable: '',
    is_installed: true,
    settings: {} as Game['settings'],
    ...over,
  };
}

function job(over: Partial<Job> = {}): Job {
  return {
    id: 'j1',
    kind: 'uninstall',
    source: 'epic-legendary',
    game_id: 'g1',
    store_game_id: 'a',
    title: 'G',
    state: 'done',
    bytes: 0,
    total: 0,
    speed: 0,
    percent: 100,
    attempts: 1,
    created_at: 0,
    updated_at: 0,
    ...over,
  };
}

describe('FE-4 optimistic install/uninstall', () => {
  it('flips is_installed=false on uninstall done before rescan', () => {
    const next = optimisticGamesAfterJob([game({ is_installed: true })], job({ kind: 'uninstall', state: 'done' }));
    expect(next[0].is_installed).toBe(false);
  });

  it('flips is_installed=true on install done before rescan', () => {
    const next = optimisticGamesAfterJob([game({ is_installed: false })], job({ kind: 'install', state: 'done' }));
    expect(next[0].is_installed).toBe(true);
  });

  it('leaves games alone for non-terminal or unrelated jobs', () => {
    const before = [game({ is_installed: true })];
    expect(optimisticGamesAfterJob(before, job({ kind: 'uninstall', state: 'running' }))).toEqual(before);
    expect(optimisticGamesAfterJob(before, job({ kind: 'uninstall', state: 'done', game_id: 'other' }))).toEqual(before);
  });
});
