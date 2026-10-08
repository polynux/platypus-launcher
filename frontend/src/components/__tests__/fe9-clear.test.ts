import { describe, it, expect } from 'vitest';
import { withoutFinishedJobs } from '../../lib/jobs';
import type { Job } from '../../types';

function job(over: Partial<Job> = {}): Job {
  return {
    id: Math.random().toString(36).slice(2),
    kind: 'install',
    source: 's',
    game_id: 'g',
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

describe('FE-9 clear finished is optimistic', () => {
  it('drops done/failed/canceled and keeps queued/running', () => {
    const jobs = [
      job({ id: 'q', state: 'queued' }),
      job({ id: 'r', state: 'running' }),
      job({ id: 'd', state: 'done' }),
      job({ id: 'f', state: 'failed' }),
      job({ id: 'c', state: 'canceled' }),
    ];
    expect(withoutFinishedJobs(jobs).map((j) => j.id)).toEqual(['q', 'r']);
  });
});
