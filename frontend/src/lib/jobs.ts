import type { Game, Job } from '../types';

/** Optimistic library update applied the moment a job reaches `done`,
 *  before the slower rescan reconciles (FE-4). Install/update mark the
 *  game installed; uninstall marks it removed. Anything else (queued,
 *  running, failed, unknown game) returns the list unchanged. */
export function optimisticGamesAfterJob(games: Game[], job: Job): Game[] {
  if (job.state !== 'done') return games;
  if (job.kind === 'uninstall') {
    let touched = false;
    const next = games.map((g) => {
      if (g.id !== job.game_id || !g.is_installed) return g;
      touched = true;
      return { ...g, is_installed: false };
    });
    return touched ? next : games;
  }
  if (job.kind === 'install' || job.kind === 'update') {
    let touched = false;
    const next = games.map((g) => {
      if (g.id !== job.game_id || g.is_installed) return g;
      touched = true;
      return { ...g, is_installed: true };
    });
    return touched ? next : games;
  }
  return games;
}
