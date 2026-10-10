import React, { useEffect, useState } from 'react';
import { Effect } from 'effect';
import { Trophy } from 'lucide-react';
import type { AchievementProgress, Game, ProgressAchievement } from '../types';
import { LauncherApi } from '../effect/runtime';
import { achievementMeta } from '../lib/stats';
import { cx } from './ui';

/** The user's own progress for a plugin game (protocol 1.7), or null when the
 *  plugin doesn't offer it (or the read fails): the page then shows the catalogue. */
export function useAchievementProgress(game: Game, enabled: boolean): AchievementProgress | null {
  const [progress, setProgress] = useState<AchievementProgress | null>(null);
  useEffect(() => {
    setProgress(null);
    if (!enabled) return;
    let live = true;
    Effect.runPromise(LauncherApi.getGameAchievements(game)).then(
      (p) => live && setProgress(p),
      () => live && setProgress(null),
    );
    return () => {
      live = false;
    };
  }, [game.id, enabled]);
  return progress;
}

/** Progress bar, count and the list. Locked hidden achievements arrive masked
 *  from core; this only shows what it is given. */
export function ProgressAchievements({ progress }: { progress: AchievementProgress }) {
  const pct = progress.total > 0 ? Math.round((progress.unlocked / progress.total) * 100) : 0;
  return (
    <section className="pb-8" aria-label="Your achievements">
      <div className="mb-3.5 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">Achievements</h2>
        <span className="font-mono text-[12px] tabular-nums text-white/55">
          {progress.unlocked}/{progress.total} · {pct}%
        </span>
      </div>
      <div
        role="progressbar"
        aria-label="Achievements unlocked"
        aria-valuemin={0}
        aria-valuemax={progress.total}
        aria-valuenow={progress.unlocked}
        className="mb-4 h-1.5 rounded-full bg-white/[0.08]"
      >
        <div className="h-full rounded-full bg-amber-300" style={{ width: `${pct}%` }} />
      </div>
      <div className="ach-grid">
        {progress.list.map((a) => (
          <ProgressCard key={a.id} a={a} />
        ))}
      </div>
      {progress.list.length < progress.total && (
        <p className="mt-3 font-mono text-[12px] text-white/55">showing {progress.list.length} of {progress.total}</p>
      )}
    </section>
  );
}

function ProgressCard({ a }: { a: ProgressAchievement }) {
  const meta = achievementMeta(a);
  return (
    <div className={cx('flex gap-3 rounded-2xl p-3 ring-1', a.unlocked ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'ring-white/[0.06]')}>
      {a.icon ? (
        <img src={a.icon} alt="" loading="lazy" className={cx('h-11 w-11 flex-none rounded-lg', !a.unlocked && 'opacity-40 grayscale')} />
      ) : (
        <span className="grid h-11 w-11 flex-none place-items-center rounded-lg bg-white/[0.05]"><Trophy className="h-4 w-4 text-white/55" /></span>
      )}
      <div className={cx('min-w-0', !a.unlocked && 'opacity-60')}>
        <div className="truncate text-[12px] font-bold">{a.name}</div>
        {a.description && <div className="line-clamp-2 text-hint leading-snug text-white/55">{a.description}</div>}
        <div className="mt-0.5 font-mono text-hint text-white/55">{a.unlocked ? (meta || 'unlocked') : meta || (a.hidden ? 'hidden' : 'locked')}</div>
      </div>
    </div>
  );
}
