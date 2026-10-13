// i18n: converted
import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import { Trophy } from 'lucide-react';
import type { AchievementProgress, Game, ProgressAchievement } from '../types';
import { LauncherApi } from '../effect/runtime';
import { formatCount, formatDate, formatPercentValue } from '../lib/format';
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
  const { t } = useTranslation('game');
  const pct = progress.total > 0 ? Math.round((progress.unlocked / progress.total) * 100) : 0;
  return (
    <section className="pb-8" aria-label={t('game:progress.label')}>
      <div className="mb-3.5 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">{t('game:achievements.heading')}</h2>
        <span className="font-mono text-[12px] tabular-nums text-white/55">
          {t('game:progress.count', { unlocked: formatCount(progress.unlocked), total: formatCount(progress.total), percent: formatCount(pct) })}
        </span>
      </div>
      <div
        role="progressbar"
        aria-label={t('game:progress.barLabel')}
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
        <p className="mt-3 font-mono text-[12px] text-white/55">{t('game:progress.showing', { shown: formatCount(progress.list.length), total: formatCount(progress.total) })}</p>
      )}
    </section>
  );
}

function ProgressCard({ a }: { a: ProgressAchievement }) {
  const { t } = useTranslation('game');
  const meta = progressMeta(t, a);
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
        <div className="mt-0.5 font-mono text-hint text-white/55">{a.unlocked ? (meta || t('game:progress.unlocked')) : meta || (a.hidden ? t('game:progress.hidden') : t('game:progress.locked'))}</div>
      </div>
    </div>
  );
}

/** Second line under a progress achievement: when it was unlocked, else the
 *  global share. Dates and percents go through lib/format for the locale. */
function progressMeta(t: (key: string, vars?: Record<string, unknown>) => string, a: ProgressAchievement): string {
  if (a.unlocked && a.unlocked_at) return t('game:progress.unlockedOn', { date: formatDate(a.unlocked_at) });
  if (a.global_percent !== undefined) return t('game:progress.shareOfPlayers', { share: formatPercentValue(a.global_percent) });
  return '';
}
