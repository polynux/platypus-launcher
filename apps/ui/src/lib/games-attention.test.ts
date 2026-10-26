import { describe, expect, it } from 'vitest';
import type { Game, Job, ProtonVersion } from '../types';
import { attentionIssues } from './games';

const game = (id: string, fields: Partial<Game> = {}) => ({
  id, source: 'steam', store: 'steam', store_app_id: id, title: id, cover_url: '', banner_url: '', hero_url: '',
  install_dir: '/games/test', executable: '', is_installed: false, settings: { runner_id: 'auto' }, ...fields,
}) as Game;

describe('attentionIssues', () => {
  it('includes failed installs, outside-folder installs, missing runners and available updates', () => {
    const games = [
      game('failed'),
      game('outside', { is_installed: true, outside_folder: true }),
      game('runner', { is_installed: true, native: false, settings: { runner_id: 'removed' } as Game['settings'] }),
      game('update', { is_installed: true, update_available: true }),
    ];
    const jobs = [{ id: 'job', kind: 'install', game_id: 'failed', state: 'failed', error: 'Disk is full', updated_at: 4 } as Job];
    const runners = [{ id: 'default', name: 'Default', is_default: true } as ProtonVersion];
    const issues = attentionIssues(games, jobs, runners);
    expect(issues.map((issue) => issue.kind)).toEqual(['install', 'outside', 'runner', 'update']);
    expect(issues[0]).toMatchObject({ title: 'Install failed', reason: 'Disk is full', fix: 'Retry install' });
    expect(issues[2]).toMatchObject({ title: 'Runner missing', fix: 'Use Default' });
  });

  it('uses only the latest job and keeps the save-conflict/prefix signals out of the frontend list', () => {
    const jobs = [
      { id: 'old', kind: 'install', game_id: 'g', state: 'failed', updated_at: 1 } as Job,
      { id: 'new', kind: 'install', game_id: 'g', state: 'complete', updated_at: 2 } as Job,
    ];
    expect(attentionIssues([game('g')], jobs, [])).toEqual([]);
  });
});
