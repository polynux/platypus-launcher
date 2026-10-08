import React, { useEffect, useMemo, useState } from 'react';
import { Effect } from 'effect';
import { ArrowLeft, Download, FolderCheck, Loader2, Play, RefreshCw, Settings2, Trash2, Trophy, X } from 'lucide-react';
import type { ActivityEvent, Achievement, Game, GameDetails, GameSettings, Job, JobKind, ProtonVersion, SessionEvent, StorePlugin, Tweak, View } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { PlayButton } from './PlayButton';
import { Cover, MonoLabel, Panel, TierBadge, TogglePill, btnNeutral, cx } from './ui';
import { attentionReason, formatBytes, heroImages, hoursPlayed, isPluginGame, lastPlayedLabel, needsAttention, storeLabel } from '../lib/games';
import { normalizeSettings, setTweak, tweakApplied } from '../lib/settings';
import { LauncherApi } from '../effect/runtime';

/** Wide art for the hero banner: the details' hero first, then the library
 *  entry's images. Kept per game id alongside the details cache. */
function useHero(game: Game, details: GameDetails | null): string[] {
  const hero = details?.hero_url;
  return useMemo(() => {
    const list = [];
    if (hero) list.push(hero);
    return [...list, ...heroImages(game)];
    // Only recompute when the game or its fetched hero changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id, hero]);
}

interface GamePageProps {
  game: Game;
  runners: ProtonVersion[];
  activity: ActivityEvent[];
  backLabel: string;
  onBack: () => void;
  onPlay: () => void;
  onStop: () => void;
  onForceStop: () => void;
  launching: boolean;
  session?: SessionEvent;
  refreshing?: boolean;
  tweaks: Tweak[];
  onChangeSettings: (s: GameSettings, what: string) => void;
  onOpenSettings: () => void;
  plugin?: StorePlugin; // plugin games only
  job?: Job; // the game's active job, or its last failed one
  onJob: (kind: JobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  onAllowFolder: (folder: string) => void;
  onNavigate: (v: View) => void;
}

// Store metadata is cached across navigations, keyed by game id + version.
// A module-level in-flight map keeps StrictMode's double effect (and quick
// remounts) from firing two concurrent GetGameDetails (FE-6).
const detailsCache = new Map<string, { details: GameDetails; version: string }>();
const detailsInflight = new Map<string, Promise<GameDetails>>();

/** Evict cached game details (all, or one game). Called on library rescan
 *  so login/logout/reinstall metadata can't stay frozen (FE-6). */
export function invalidateGameDetailsCache(id?: string) {
  if (id) {
    detailsCache.delete(id);
    detailsInflight.delete(id);
  } else {
    detailsCache.clear();
    detailsInflight.clear();
  }
}

function useDetails(game: Game) {
  const version = game.version ?? '';
  const [details, setDetails] = useState<GameDetails | null>(() => {
    const cached = detailsCache.get(game.id);
    return cached && cached.version === version ? cached.details : null;
  });
  const [error, setError] = useState<string | null>(null);
  // Steam and store plugins both have metadata; sideloaded games have none.
  const hasSource = game.source === 'steam' || isPluginGame(game);
  useEffect(() => {
    const v = game.version ?? '';
    const cached = detailsCache.get(game.id);
    if (cached && cached.version === v) {
      setDetails(cached.details);
      return;
    }
    setDetails(null);
    setError(null);
    if (!hasSource) return;
    // Attach to an in-flight fetch instead of starting a second one.
    const inflight = detailsInflight.get(game.id);
    if (inflight) {
      let live = true;
      inflight.then(
        (d) => { if (live) setDetails(d); },
        (e) => { if (live) setError(e instanceof Error ? e.message : String(e)); },
      );
      return () => { live = false; };
    }
    let live = true;
    const p = Effect.runPromise(LauncherApi.getGameDetails(game));
    detailsInflight.set(game.id, p);
    p.then(
      (d) => {
        detailsCache.set(game.id, { details: d, version: v });
        detailsInflight.delete(game.id);
        if (live) setDetails(d);
      },
      (e) => {
        detailsInflight.delete(game.id);
        if (live) setError(e instanceof Error ? e.message : String(e));
      },
    );
    return () => { live = false; };
    // Refetch when the game (or its version) changes, not on settings edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id, game.version]);
  return { details, error, loading: !details && !error && hasSource };
}

export function GamePage(p: GamePageProps) {
  const { game, runners, activity, session, tweaks } = p;
  const [shot, setShot] = useState(0);
  const { details, error, loading } = useDetails(game);
  const hero = useHero(game, details);
  useEffect(() => setShot(0), [game.id]);

  const s = normalizeSettings(game.settings);
  const autoRunner = runners.find((r) => r.is_default);
  const runner = runners.find((r) => r.id === s.runner_id || r.name === s.runner_id);
  const viaSteam = game.source === 'steam' && !game.executable;
  const plugin = p.plugin;
  const job = p.job;
  const active = job && (job.state === 'queued' || job.state === 'running');
  const fix = game.fix_info;
  const shots = details?.screenshots ?? [];
  const ach = details?.achievements;
  const events = activity.filter((e) => e.game === game.title).slice(0, 6);
  const hours = hoursPlayed(game);
  const last = lastPlayedLabel(game);

  const facts: [string, string][] = [
    [
      'Runner',
      game.native
        ? 'none · native Linux build'
        : viaSteam
          ? 'chosen in Steam'
          : runner?.name ?? (!s.runner_id || s.runner_id === 'auto' ? `auto · ${autoRunner?.name ?? 'none installed'}` : `${s.runner_id} (missing)`),
    ],
    ['DX layer', game.native ? 'not used' : s.dx_layer === 'wined3d' ? 'WineD3D' : 'DXVK (bundled)'],
    ['Wrappers', [s.use_gamemode && 'gamemode', s.mangohud_preset !== 'off' && 'mangohud', s.use_gamescope && 'gamescope'].filter(Boolean).join(' · ') || 'none'],
    ['Launch via', viaSteam ? 'Steam client' : plugin ? `Hydra · ${plugin.name}` : 'Hydra'],
  ];
  const compat = fix
    ? needsAttention(game) ? attentionReason(game) : fix.description
    : 'No compatibility reports loaded for this title yet.';

  const about = [
    details?.genres?.length ? details.genres.slice(0, 3).join(' · ') : null,
    details?.developers?.length ? details.developers.join(', ') : null,
    details?.publishers?.length ? details.publishers.join(', ') : null,
    details?.release_date || null,
  ].filter(Boolean) as string[];

  return (
    <div className="p-7">
      <button onClick={p.onBack} className="flex items-center gap-1.5 rounded text-[12px] font-semibold text-white/50 transition hover:text-white">
        <ArrowLeft className="h-3.5 w-3.5" /> {p.backLabel}
      </button>

      <section className="relative mb-6 mt-4 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
        <Cover src={hero} title={game.title} className="h-[280px] w-full" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.98) 10%, rgba(19,15,10,.35) 62%, rgba(19,15,10,.25) 100%)' }} />
        <div className="absolute bottom-6 left-8 right-8 flex items-end justify-between gap-6">
          <div className="min-w-0">
            <div className="mb-2.5 flex flex-wrap gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
              {fix?.tier && <TierBadge tier={fix.tier} suffix={fix.source === 'protondb' ? '· protondb' : '· curated'} />}
              <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{storeLabel(game.store)}</span>
              {formatBytes(game.size_bytes) && (
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{formatBytes(game.size_bytes)}</span>
              )}
              {!game.is_installed && <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/55">not installed</span>}
              {game.update_available && <span className="rounded-lg bg-sky-400/15 px-2.5 py-1 text-sky-200 ring-1 ring-sky-300/30">update available</span>}
            </div>
            <h1 className="truncate text-[36px] font-extrabold leading-none tracking-tight">{game.title}</h1>
          </div>
          <div className="flex flex-none items-center gap-4">
            {(hours != null || last) && (
              <div className="text-right font-mono text-[10px] leading-relaxed text-white/50">
                {hours != null && <div>{hours}h logged</div>}
                {last && <div>last played {last}</div>}
              </div>
            )}
            {!game.is_installed ? (
              <InstallButton game={game} job={job} onInstall={() => p.onJob('install')} onCancel={(id) => p.onJobAction('cancel', id)} />
            ) : (
            <PlayButton session={session} launching={p.launching} disabled={game.outside_folder} onPlay={p.onPlay} onStop={p.onStop} onForceStop={p.onForceStop} />
            )}
          </div>
        </div>
      </section>

      {game.is_installed && game.outside_folder && plugin && (
        <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-5 py-4 ring-1 ring-amber-300/25">
          <div className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
            <p className="font-bold text-amber-200">Installed outside {plugin.name}'s folder</p>
            <p>
              Hydra only launches {plugin.name} games from <span className="font-mono text-[11px]">{plugin.install_folder}</span> or folders you allow.
              This one is in <span className="font-mono text-[11px] text-white/90">{game.install_dir}</span>, probably from before Hydra managed it.
            </p>
          </div>
          <button onClick={() => p.onAllowFolder(game.install_dir)} className={cx(btnNeutral, 'flex flex-none items-center gap-2 px-4 py-2.5 text-[12.5px]')}>
            <FolderCheck className="h-4 w-4" /> Allow this folder
          </button>
        </div>
      )}

      {!active && (game.note || (!game.is_installed && plugin && game.installable)) && (
        <div className="mb-6 rounded-2xl bg-white/[0.04] px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-white/[0.07]">
          {game.note && <p>{game.note}</p>}
          {!game.is_installed && plugin && game.installable && (
            <p className="font-mono text-[10.5px] text-white/45">
              installs into {plugin.install_folder} ·{' '}
              <button onClick={() => p.onNavigate({ name: 'plugins' })} className="underline decoration-white/25 underline-offset-2 hover:text-white/80">change</button>
            </p>
          )}
          {!game.is_installed && job?.state === 'failed' && <p className="mt-1 font-mono text-[10.5px] text-red-300">last attempt failed: {job.error}</p>}
        </div>
      )}

      {p.refreshing && (
        <p className="mb-4 font-mono text-[10px] uppercase tracking-widest text-white/40">refreshing library…</p>
      )}

      <div className="mb-6 grid gap-5 lg:grid-cols-[1.6fr_1fr]">
        <div className="min-w-0 space-y-5">
          {shots.length > 0 && (
            <div className="space-y-3">
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
                    <img src={src} alt="" loading="lazy" className="h-full w-full object-cover" />
                  </button>
                ))}
              </div>
            </div>
          )}

          <Panel className="bg-white/[0.05]">
            <h2 className="mb-2 text-[15px] font-bold">About</h2>
            {loading ? (
              <div className="space-y-2">
                <div className="h-3 w-full animate-pulse rounded bg-white/[0.06]" />
                <div className="h-3 w-4/5 animate-pulse rounded bg-white/[0.06]" />
              </div>
            ) : details?.summary ? (
              <p className="text-[13px] leading-relaxed text-white/70">{details.summary}</p>
            ) : (
              <p className="text-[13px] text-white/45">{error ? `Store data unavailable: ${error}` : 'No store description for this title.'}</p>
            )}
            {about.length > 0 && <p className="mt-3 font-mono text-[10.5px] text-white/45">{about.join('  ·  ')}</p>}
          </Panel>
        </div>

        <aside className="min-w-0 space-y-5">
          <Panel className="bg-white/[0.05]">
            <h2 className="mb-2 text-[15px] font-bold">Compatibility</h2>
            <p className="text-[13px] leading-relaxed text-white/65">{compat}</p>
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
            <button onClick={p.onOpenSettings} className={cx(btnNeutral, 'mt-4 flex w-full items-center justify-center gap-2 rounded-2xl border border-white/[0.1] px-5 py-2.5 text-[13px]')}>
              <Settings2 className="h-4 w-4" /> Launcher settings
            </button>
            {game.source === 'steam' && (
              <a
                href={`https://www.protondb.com/app/${game.store_app_id}`}
                target="_blank"
                rel="noreferrer"
                className="mt-3 block text-center font-mono text-[10px] uppercase tracking-widest text-white/40 transition hover:text-white/80"
              >
                protondb reports ↗
              </a>
            )}
          </Panel>

          {isPluginGame(game) && game.is_installed && plugin && (
            <ManagePanel game={game} plugin={plugin} job={job} onJob={p.onJob} onJobAction={p.onJobAction} />
          )}

          {tweaks.length > 0 && game.is_installed && (
            <TweaksPanel
              game={game}
              // Proton-only env tweaks do nothing for a native build.
              tweaks={game.native ? tweaks.filter((t) => !Object.keys(t.env ?? {}).some((k) => /^(PROTON_|WINE_)/.test(k))) : tweaks}
              viaSteam={viaSteam}
              onChange={p.onChangeSettings}
              onOpenSettings={p.onOpenSettings}
            />
          )}
        </aside>
      </div>

      {ach && ach.total > 0 && (
        <AchievementsSection ach={ach} known={ach.unlocked >= 0} viaSteam={game.source === 'steam'} />
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

function JobProgress({ job, onCancel }: { job: Job; onCancel: (id: string) => void }) {
  const pct = job.percent >= 0 ? Math.round(job.percent) : null;
  const what = job.state === 'queued' ? 'queued' : job.phase || job.kind;
  return (
    <div className="w-72 rounded-2xl bg-black/70 px-4 py-3 ring-1 ring-white/10">
      <div className="flex items-center justify-between gap-3 font-mono text-[10px] uppercase tracking-wider">
        <span className="truncate text-amber-200">{what}{pct != null && job.state === 'running' ? ` · ${pct}%` : ''}</span>
        <button onClick={() => onCancel(job.id)} aria-label={`Cancel ${job.kind}`} className="rounded p-0.5 text-white/50 transition hover:text-white">
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="mt-2 h-1.5 rounded-full bg-white/15" role="progressbar" aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
        <div className={cx('h-full rounded-full bg-amber-300 transition-[width]', pct == null && 'animate-pulse')} style={{ width: `${pct ?? 100}%` }} />
      </div>
      {job.state === 'running' && job.total > 0 && (
        <div className="mt-1.5 font-mono text-[9.5px] tabular-nums text-white/45">
          {formatBytes(job.bytes) || '0 B'} of {formatBytes(job.total)}
          {job.speed > 0 && ` · ${formatBytes(job.speed)}/s`}
        </div>
      )}
    </div>
  );
}

function InstallButton({ game, job, onInstall, onCancel }: { game: Game; job?: Job; onInstall: () => void; onCancel: (id: string) => void }) {
  if (job && (job.state === 'queued' || job.state === 'running')) return <JobProgress job={job} onCancel={onCancel} />;
  if (!game.installable) {
    return (
      <span className="rounded-2xl bg-white/[0.08] px-6 py-3.5 text-[13px] font-bold text-white/55" title={game.note}>
        Can't install here
      </span>
    );
  }
  return (
    <button
      onClick={onInstall}
      className="flex items-center gap-2 rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#130f0a] shadow-[0_12px_30px_rgba(252,211,77,.25)] transition hover:bg-amber-200"
    >
      <Download className="h-4 w-4" strokeWidth={3} />
      {job?.state === 'failed' ? 'Retry install' : 'Install'}
    </button>
  );
}

/** Where a plugin game lives and what can be done with it. */
function ManagePanel({
  game,
  plugin,
  job,
  onJob,
  onJobAction,
}: {
  game: Game;
  plugin: StorePlugin;
  job?: Job;
  onJob: (kind: JobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
}) {
  const [confirm, setConfirm] = useState(false);
  useEffect(() => setConfirm(false), [game.id]);
  const active = job && (job.state === 'queued' || job.state === 'running');
  return (
    <Panel className="bg-white/[0.05]">
      <h2 className="mb-2 text-[15px] font-bold">Installation</h2>
      <div className="space-y-2 font-mono text-[10.5px]">
        {[
          ['Store', `${storeLabel(game.store)} · ${plugin.name}`],
          ['Folder', game.install_dir || '—'],
          ['Version', game.version || '—'],
          ['Size', formatBytes(game.size_bytes) || '—'],
        ].map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
            <span className="text-white/45">{k}</span>
            <span className="truncate text-right text-white/85 select-text" title={v}>{v}</span>
          </div>
        ))}
      </div>
      {active ? (
        <div className="mt-4"><JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} /></div>
      ) : (
        <div className="mt-4 flex gap-2">
          {game.update_available && (
            <button onClick={() => onJob('update')} className={cx(btnNeutral, 'flex flex-1 items-center justify-center gap-2 py-2.5 text-[12.5px] text-sky-100')}>
              <RefreshCw className="h-3.5 w-3.5" /> Update
            </button>
          )}
          {plugin.capabilities.includes('install') && (
            <button
              onClick={() => (confirm ? (setConfirm(false), onJob('uninstall')) : setConfirm(true))}
              onBlur={() => setConfirm(false)}
              onKeyDown={(e) => { if (e.key === 'Escape') setConfirm(false); }}
              className={cx(
                'flex flex-1 items-center justify-center gap-2 rounded-xl py-2.5 text-[12.5px] font-bold transition',
                confirm ? 'bg-red-500/80 text-white hover:bg-red-500' : 'bg-white/[0.06] text-white/70 hover:bg-white/[0.1]'
              )}
            >
              <Trash2 className="h-3.5 w-3.5" /> {confirm ? 'Delete game files?' : 'Uninstall'}
            </button>
          )}
        </div>
      )}
      {job?.state === 'failed' && !active && (
        <p className="mt-2 font-mono text-[10px] text-red-300">
          {job.kind} failed: {job.error}{' '}
          <button onClick={() => onJobAction('retry', job.id)} className="underline underline-offset-2 hover:text-red-200">retry</button>
        </p>
      )}
    </Panel>
  );
}

function TweaksPanel({
  game,
  tweaks,
  viaSteam,
  onChange,
  onOpenSettings,
}: {
  game: Game;
  tweaks: Tweak[];
  viaSteam: boolean;
  onChange: (s: GameSettings, what: string) => void;
  onOpenSettings: () => void;
}) {
  const groups: [string, Tweak[]][] = [
    ['recommended', tweaks.filter((t) => t.kind === 'recommended')],
    ['if something breaks', tweaks.filter((t) => t.kind === 'troubleshoot')],
  ];
  return (
    <Panel className="bg-white/[0.05]">
      <h2 className="text-[15px] font-bold">Suggested tweaks</h2>
      <p className="mt-1 text-[11.5px] leading-relaxed text-white/45">
        Based on this machine, not the game's rating: a platinum game can still need them on your GPU.
        {viaSteam && (
          <>
            {' '}Steam applies these only through the game's{' '}
            <button onClick={onOpenSettings} className="underline decoration-white/30 underline-offset-2 hover:text-white/80">launch options</button>.
          </>
        )}
      </p>
      {groups.map(([label, list]) =>
        list.length === 0 ? null : (
          <div key={label} className="mt-3">
            <MonoLabel className="mb-1.5">{label}</MonoLabel>
            <div className="space-y-1.5">
              {list.map((t) => {
                const on = tweakApplied(game.settings, t);
                return (
                  <button
                    key={t.id}
                    role="switch"
                    aria-checked={on}
                    onClick={() => onChange(setTweak(game.settings, t, !on), `${on ? 'Removed' : 'Applied'} "${t.title}"`)}
                    className={cx('flex w-full items-start gap-3 rounded-xl px-3 py-2.5 text-left ring-1 transition', on ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'ring-white/[0.06] hover:bg-white/[0.04]')}
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block text-[12.5px] font-semibold text-white/90">{t.title}</span>
                      <span className="block text-[11px] leading-snug text-white/50">{t.reason}</span>
                      {t.env && Object.keys(t.env).length > 0 && (
                        <span className="mt-1 block truncate font-mono text-[9.5px] text-sky-200/60">
                          {Object.entries(t.env).map(([k, v]) => `${k}=${v}`).join(' ')}
                        </span>
                      )}
                    </span>
                    <span className="flex flex-none pt-0.5"><TogglePill on={on} /></span>
                  </button>
                );
              })}
            </div>
          </div>
        )
      )}
    </Panel>
  );
}

function AchievementsSection({
  ach,
  known,
  viaSteam,
}: {
  ach: NonNullable<GameDetails['achievements']>;
  known: boolean;
  viaSteam: boolean;
}) {
  const pct = known && ach.total > 0 ? Math.round((ach.unlocked / ach.total) * 100) : 0;
  // Backend may send list:null for uncached progress; normalized at the
  // edge in runtime.ts, guarded here too for safety (FE-10).
  const list = ach.list ?? [];
  return (
    <section className="pb-8">
      <div className="mb-3.5 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">Achievements</h2>
        <span className="font-mono text-[11px] tabular-nums text-white/45">
          {known ? `${ach.unlocked}/${ach.total} · ${pct}%` : `${ach.total} total`}
        </span>
      </div>
      {known ? (
        <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
          <div className="h-full rounded-full bg-amber-300" style={{ width: `${pct}%` }} />
        </div>
      ) : (
        <p className="mb-4 text-[11.5px] text-white/45">
          {viaSteam
            ? "Your progress isn't cached yet. Open this game once in Steam's library and it shows up here."
            : 'The store lists these achievements; your progress there isn\'t available to Hydra.'}
        </p>
      )}
      <div className="ach-grid">
        {list.map((a) => (
          <AchievementCard key={a.id || a.name} a={a} known={known} />
        ))}
      </div>
      {list.length < ach.total && (
        <p className="mt-3 font-mono text-[10px] text-white/35">
          showing {list.length} of {ach.total} · {known ? 'Steam only caches a sample locally' : 'highlighted by the store'}
        </p>
      )}
    </section>
  );
}

function AchievementCard({ a, known }: { a: Achievement; known: boolean }) {
  const dim = known && !a.unlocked;
  const meta = a.unlocked && a.unlocked_at
    ? new Date(a.unlocked_at * 1000).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' })
    : a.global_percent
      ? `${a.global_percent.toFixed(1)}% of players`
      : '';
  return (
    <div title={a.description} className={cx('flex gap-3 rounded-2xl p-3 ring-1', a.unlocked ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'ring-white/[0.06]')}>
      {a.icon ? (
        <img src={a.icon} alt="" loading="lazy" className={cx('h-11 w-11 flex-none rounded-lg', dim && 'opacity-40 grayscale')} />
      ) : (
        <span className="grid h-11 w-11 flex-none place-items-center rounded-lg bg-white/[0.05]"><Trophy className="h-4 w-4 text-white/40" /></span>
      )}
      <div className={cx('min-w-0', dim && 'opacity-60')}>
        <div className="truncate text-[12px] font-bold">{a.name}</div>
        {a.description && <div className="line-clamp-2 text-[10.5px] leading-snug text-white/50">{a.description}</div>}
        {meta && <div className="mt-0.5 font-mono text-[9px] text-white/40">{meta}</div>}
      </div>
    </div>
  );
}
