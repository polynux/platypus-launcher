import React, { useState } from 'react';
import { ArrowLeft, Loader2, Play, Settings2 } from 'lucide-react';
import type { ActivityEvent, Game, ProtonVersion } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { Cover, MonoLabel, Panel, TierBadge, btnNeutral, cx } from './ui';
import { attentionReason, heroImages, needsAttention, storeLabel } from '../lib/games';
import { normalizeSettings } from '../lib/settings';

interface GamePageProps {
  game: Game;
  runners: ProtonVersion[];
  activity: ActivityEvent[];
  backLabel: string;
  onBack: () => void;
  onPlay: () => void;
  launching: boolean;
  onOpenSettings: () => void;
}

export function GamePage({ game, runners, activity, backLabel, onBack, onPlay, launching, onOpenSettings }: GamePageProps) {
  const [shot, setShot] = useState(0);
  const s = normalizeSettings(game.settings);
  const runner = runners.find((r) => r.id === s.runner_id || r.name === s.runner_id);
  const fix = game.fix_info;
  const shots = game.screenshots ?? [];
  const ach = game.achievements;
  const events = activity.filter((e) => e.game === game.title).slice(0, 6);

  const facts: [string, string][] = [
    ['Runner', runner?.name ?? (s.runner_id === 'auto' || !s.runner_id ? 'auto (first available)' : `${s.runner_id} (missing)`)],
    ['DX layer', s.dx_layer === 'wined3d' ? 'WineD3D' : 'DXVK (bundled)'],
    ['Wrappers', [s.use_gamemode && 'gamemode', s.mangohud_preset !== 'off' && 'mangohud', s.use_gamescope && 'gamescope'].filter(Boolean).join(' · ') || 'none'],
    ['Launch via', game.store === 'steam' && !game.executable ? 'Steam client' : 'Hydra'],
  ];
  if (game.playtime_hours != null) facts.push(['Played', `${game.playtime_hours}h`]);

  const summary = fix
    ? needsAttention(game) ? attentionReason(game) : fix.description
    : 'No compatibility reports loaded for this title yet. It runs with the default Proton setup.';

  return (
    <div className="px-10 py-6">
      <button onClick={onBack} className="flex items-center gap-1.5 rounded text-[12px] font-semibold text-white/50 transition hover:text-white">
        <ArrowLeft className="h-3.5 w-3.5" /> {backLabel}
      </button>

      <section className="relative mb-6 mt-4 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
        <Cover src={heroImages(game)} title={game.title} className="h-[280px] w-full" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.98) 10%, rgba(19,15,10,.35) 62%, rgba(19,15,10,.25) 100%)' }} />
        <div className="absolute bottom-6 left-8 right-8 flex items-end justify-between gap-6">
          <div className="min-w-0">
            <div className="mb-2.5 flex flex-wrap gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
              {fix?.tier && <TierBadge tier={fix.tier} suffix={fix.source === 'protondb' ? '· protondb' : '· curated'} />}
              <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{storeLabel(game.store)}</span>
              {game.size_on_disk && <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{game.size_on_disk}</span>}
            </div>
            <h1 className="truncate text-[36px] font-extrabold leading-none tracking-tight">{game.title}</h1>
          </div>
          <div className="flex flex-none items-center gap-4">
            {(game.playtime_hours != null || game.last_played) && (
              <div className="text-right font-mono text-[10px] leading-relaxed text-white/50">
                {game.playtime_hours != null && <div>{game.playtime_hours}h logged</div>}
                {game.last_played && <div>last {game.last_played}</div>}
              </div>
            )}
            <button
              onClick={onPlay}
              disabled={launching}
              className="flex items-center gap-2 rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#130f0a] shadow-[0_12px_30px_rgba(252,211,77,.25)] transition hover:bg-amber-200 disabled:cursor-wait disabled:opacity-70"
            >
              {launching ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4 fill-current" />}
              {launching ? 'Launching' : 'Play'}
            </button>
          </div>
        </div>
      </section>

      <div className={cx('mb-6 grid gap-5', shots.length > 0 && 'lg:grid-cols-[1.6fr_1fr]')}>
        {shots.length > 0 && (
          <div className="min-w-0 space-y-3">
            <div className="overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
              <img key={shot} src={shots[shot]} alt={`${game.title} screenshot ${shot + 1}`} className="aspect-video w-full object-cover" />
            </div>
            <div className="no-scrollbar flex gap-2.5 overflow-x-auto pb-1">
              {shots.map((src, i) => (
                <button
                  key={src}
                  onClick={() => setShot(i)}
                  aria-label={`Show screenshot ${i + 1}`}
                  aria-pressed={i === shot}
                  className={cx('aspect-video w-[150px] flex-none overflow-hidden rounded-xl transition', i === shot ? 'ring-2 ring-amber-300/80' : 'opacity-55 ring-1 ring-white/[0.08] hover:opacity-90')}
                >
                  <img src={src} alt="" className="h-full w-full object-cover" />
                </button>
              ))}
            </div>
          </div>
        )}

        <aside className={cx('space-y-5', shots.length === 0 && 'grid gap-5 space-y-0 lg:grid-cols-[1.6fr_1fr]')}>
          <Panel className="bg-white/[0.05]">
            <h2 className="mb-2 text-[15px] font-bold">Summary</h2>
            <p className="text-[13px] leading-relaxed text-white/65">{summary}</p>
            {fix?.winetricks && fix.winetricks.length > 0 && (
              <p className="mt-2 font-mono text-[10.5px] text-sky-200/80">recipe: winetricks {fix.winetricks.join(' ')}</p>
            )}
            <div className="mt-4 space-y-2 font-mono text-[10.5px]">
              {facts.map(([k, v]) => (
                <div key={k} className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
                  <span className="text-white/45">{k}</span>
                  <span className="truncate text-right text-white/85">{v}</span>
                </div>
              ))}
            </div>
          </Panel>
          <div className="space-y-3">
            <button onClick={onOpenSettings} className={cx(btnNeutral, 'flex w-full items-center justify-center gap-2 rounded-2xl border border-white/[0.1] px-5 py-3 text-[13px]')}>
              <Settings2 className="h-4 w-4" /> Launcher settings
            </button>
            {game.store === 'steam' && (
              <a
                href={`https://www.protondb.com/app/${game.store_app_id}`}
                target="_blank"
                rel="noreferrer"
                className="block text-center font-mono text-[10px] uppercase tracking-widest text-white/40 transition hover:text-white/80"
              >
                protondb reports ↗
              </a>
            )}
          </div>
        </aside>
      </div>

      {ach && ach.total > 0 && (
        <section className="pb-8">
          <div className="mb-3.5 flex items-baseline justify-between">
            <h2 className="text-[16px] font-bold tracking-tight">Achievements</h2>
            <span className="font-mono text-[11px] tabular-nums text-white/45">{ach.unlockedCount}/{ach.total} · {ach.score}%</span>
          </div>
          <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
            <div className="h-full rounded-full bg-amber-300" style={{ width: `${ach.score}%` }} />
          </div>
          <div className="ach-grid">
            {ach.list.map((a) => (
              <div key={a.name} title={a.desc} className={cx('rounded-2xl p-3.5 ring-1', a.unlocked ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'opacity-40 ring-white/[0.06]')}>
                <div className={cx('grid h-10 w-10 place-items-center rounded-xl text-[15px]', a.unlocked ? 'bg-amber-300/15 ring-1 ring-amber-300/40' : 'bg-white/[0.05]')}>★</div>
                <div className="mt-2 truncate text-[12px] font-bold">{a.name}</div>
                <div className="font-mono text-[9px] text-white/40">{a.unlocked ? a.date : a.rarity}</div>
              </div>
            ))}
          </div>
        </section>
      )}

      {events.length > 0 && (
        <section className="pb-10">
          <MonoLabel className="mb-2">this session</MonoLabel>
          <ActivityFeed events={events} />
        </section>
      )}
    </div>
  );
}
