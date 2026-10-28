import { describe, expect, it } from 'vitest';
import type { Game, Job, ProtonVersion } from '../types';
import { attentionIssues, attentionReason, needsAttention } from './games';

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
    expect(issues[0]).toMatchObject({ game: games[0], kind: 'install', error: 'Disk is full' });
    expect(issues[2]).toMatchObject({ game: games[2], kind: 'runner', runnerId: 'removed' });
  });

  it('uses only the latest job and keeps the save-conflict/prefix signals out of the frontend list', () => {
    const jobs = [
      { id: 'old', kind: 'install', game_id: 'g', state: 'failed', updated_at: 1 } as Job,
      { id: 'new', kind: 'install', game_id: 'g', state: 'complete', updated_at: 2 } as Job,
    ];
    expect(attentionIssues([game('g')], jobs, [])).toEqual([]);
  });

  // Issue #144: the core flags land on the game; the frontend only lists them.
  it('lists save conflicts and damaged prefixes from the core flags', () => {
    const games = [
      game('conflict', { has_save_conflict: true }),
      game('prefix', { prefix_damaged: true }),
    ];
    const issues = attentionIssues(games, [], []);
    expect(issues.map((issue) => issue.kind)).toEqual(['conflict', 'prefix']);
    expect(issues[0].game.id).toBe('conflict');
    expect(issues[1].game.id).toBe('prefix');
  });

  it('needsAttention covers the core signals and the reason names them', () => {
    expect(needsAttention(game('c', { has_save_conflict: true }))).toBe(true);
    expect(needsAttention(game('p', { prefix_damaged: true }))).toBe(true);
    expect(needsAttention(game('ok'))).toBe(false);
    expect(attentionReason(game('c', { has_save_conflict: true }))).toBe(
      'Saves changed on this device and in the cloud. Compare them to keep playing with the right one.');
    expect(attentionReason(game('p', { prefix_damaged: true }))).toBe(
      'The Wine prefix looks damaged. Repair it or the game may not start.');
  });

  it('keeps the rating text on a rating row when the game also has a signal', () => {
    const g = game('both', {
      has_save_conflict: true,
      fix_info: { description: 'Needs tweaks', source: 'Test', tier: 'bronze' },
    } as Partial<Game>);
    expect(attentionReason(g)).toContain('cloud');
    expect(attentionReason(g, 'rating')).toBe('Needs tweaks');
  });
});
