import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Effect } from 'effect';
import { ArrowLeft, Download, FolderCheck, FolderInput, Maximize2, RefreshCw, Settings2, Trophy } from 'lucide-react';
import type { ActivityEvent, Achievement, Game, GameDetails, GameSettings, GameJobKind, Job, ProtonVersion, SessionEvent, StorePlugin, Tweak, View } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { PlayButton } from './PlayButton';
import { CopyPicker } from './CopyPicker';
import { Lightbox } from './Lightbox';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { InstallDialog } from './InstallDialog';
import { GameSettingsModal } from './GameSettingsModal';
import { JobProgress } from './JobProgress';
import { CompatFacts } from './CompatBadges';
import { useCompat } from '../lib/useCompat';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ds/tooltip';
import { ScrollArea } from '@/components/ds/scroll-area';
import { Cover, MonoLabel, Panel, TierBadge, cx } from './ui';
import { attentionReason, formatBytes, heroImages, hoursPlayed, isPluginGame, lastPlayedLabel, needsAttention, storeLabel } from '../lib/games';
import { TIER_LABEL, ratingLine } from '../lib/compat';
import { normalizeSettings } from '../lib/settings';
import { LauncherApi } from '../effect/runtime';
import { offers } from '../lib/capabilities';
import { playtimeTooltip } from '../lib/stats';
import { ProgressAchievements, useAchievementProgress } from './ProgressAchievements';
import { pinLabel } from '../lib/versions';
import { isCustomized } from '../lib/customize';
import { useInstallSize, type InstallSize } from '../lib/installSize';
import type { GameSettingsTab } from '../lib/gameSettingsTabs';
import { ErrorNotice } from './ErrorNotice';

/** Wide art for the hero banner: the details' hero first, then the library
 *  entry's images. Kept per game id alongside the details cache. */
function useHero(game: Game, details: GameDetails | null): string[] {
  // The user's own banner wins over the store's hero.
  const hero = isCustomized(game, 'banner') ? undefined : details?.hero_url;
  return useMemo(() => {
    const list = [];
    if (hero) list.push(hero);
    return [...list, ...heroImages(game)];
    // Only recompute when the game or its fetched hero changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id, hero, game.hero_url, game.banner_url, game.cover_url]);
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
  /** A tweak was switched on or off. */
  onChangeSettings: (s: GameSettings, what: string) => void;
  /** The general settings form was saved. */
  onSaveSettings: (s: GameSettings) => void;
  /** The game's customization changed: the library should reload to carry it. */
  onCustomized?: () => void;
  plugin?: StorePlugin; // plugin games only
  job?: Job; // the game's active job, or its last failed one
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  /** Every job in the queue: the verify dialog follows its own. */
  jobs?: Job[];
  onAllowFolder: (folder: string) => void;
  onNavigate: (v: View) => void;
  /** Toast, for the version dialog. */
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  /** A version job was queued or a pin changed: the library should reload. */
  onVersionsChanged?: () => void;
  /** The family copy a borrowed game is played from changed: the library should reload. */
  onCopyChanged?: () => void;
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
  const [lightbox, setLightbox] = useState(false);
  // The game settings dialog, and the tab it opens on.
  const [settingsTab, setSettingsTab] = useState<GameSettingsTab | null>(null);
  // "Install to…": the install dialog for a game without install options.
  const [installTo, setInstallTo] = useState(false);
  // The install-options dialog before the first install; an installed game's add-ons are a settings tab.
  const [optionsOpen, setOptionsOpen] = useState(false);
  const strip = useRef<HTMLDivElement>(null);
  const { details, error, loading } = useDetails(game);
  // ProtonDB, native, DRM and anti-cheat facts (internal/compat); null while loading.
  const { compat: info } = useCompat(game);
  const progress = useAchievementProgress(game, !!p.plugin && offers(p.plugin, 'achievements', { game }));
  const hero = useHero(game, details);
  useEffect(() => { setShot(0); setLightbox(false); setSettingsTab(null); setInstallTo(false); setOptionsOpen(false); }, [game.id]);
  useEffect(() => {
    const el = strip.current?.querySelectorAll<HTMLElement>('[data-thumb]')[shot];
    el?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' });
  }, [shot, details]);
  // The thumbnail strip scrolls sideways: a mouse wheel's vertical turns scroll it too. Native, not
  // React's onWheel, because React's listener is passive and can't stop the page from scrolling.
  useEffect(() => {
    const el = strip.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (Math.abs(e.deltaY) <= Math.abs(e.deltaX) || el.scrollWidth <= el.clientWidth) return;
      e.preventDefault();
      el.scrollLeft += e.deltaY;
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
  }, [details]);

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
  const canChooseVersion = !!plugin && game.installable !== false && offers(plugin, 'versions', { game });
  const canChooseOptions = !!plugin && game.installable !== false && offers(plugin, 'install-options', { game });
  const installing = !game.is_installed && !!plugin && !!game.installable;
  const installSize = useInstallSize(game, plugin, installing);
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
    ['Launch via', viaSteam ? 'Steam client' : plugin ? `Platypus · ${plugin.name}` : 'Platypus'],
    [
      'ProtonDB',
      !info
        ? '…'
        : info.protondb
          ? ratingLine(info.protondb)
          : info.native
            ? 'not rated · native build'
            : info.steam_app_id
              ? 'not fetched yet'
              : 'no Steam AppID',
    ],
  ];
  const compat = fix
    ? needsAttention(game) ? attentionReason(game) : fix.description
    : info?.protondb?.tier
      ? `ProtonDB rates this title ${TIER_LABEL[info.protondb.tier] ?? info.protondb.tier}, from community reports.`
      : info?.protondb?.no_reports
        ? 'ProtonDB has no reports for this title.'
        : 'No compatibility reports loaded for this title yet.';

  const about = [
    details?.genres?.length ? details.genres.slice(0, 3).join(' · ') : null,
    details?.developers?.length ? details.developers.join(', ') : null,
    details?.publishers?.length ? details.publishers.join(', ') : null,
    details?.release_date || null,
  ].filter(Boolean) as string[];

  return (
    <TooltipProvider>
    <div className="relative isolate p-7">
      {game.background_url && (
        <div
          aria-hidden
          data-testid="game-background"
          className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[620px] bg-cover bg-top opacity-[.2]"
          style={{ backgroundImage: `url("${game.background_url}")`, maskImage: 'linear-gradient(180deg, #000 30%, transparent)', WebkitMaskImage: 'linear-gradient(180deg, #000 30%, transparent)' }}
        />
      )}
      <Button variant="ghost" size="none" onClick={p.onBack} className="gap-1.5 rounded px-0 text-[12px] font-semibold text-white/55 hover:bg-transparent hover:text-white">
        <ArrowLeft className="h-3.5 w-3.5" /> {p.backLabel}
      </Button>

      <section className="relative mb-6 mt-4 flex min-h-[280px] flex-col justify-end overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
        <Cover src={hero} title={game.title} className="absolute inset-0 h-full w-full" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.98) 10%, rgba(19,15,10,.35) 62%, rgba(19,15,10,.25) 100%)' }} />
        <div className="relative flex flex-wrap items-end justify-between gap-x-6 gap-y-4 px-8 pb-6 pt-16">
          <div className="min-w-0 flex-1 basis-72">
            <div className="mb-2.5 flex flex-wrap gap-2">
              {fix?.tier && <TierBadge tier={fix.tier} suffix={fix.source === 'protondb' ? '· protondb' : '· curated'} />}
              <Badge className="bg-black/50 text-white/75">{storeLabel(game.store)}</Badge>
              {formatBytes(game.size_bytes) && <Badge className="bg-black/50 text-white/75">{formatBytes(game.size_bytes)}</Badge>}
              {!game.is_installed && <Badge className="bg-black/50 text-white/55">not installed</Badge>}
              {game.update_available && <Badge variant="info" className="bg-sky-400/15 ring-1 ring-sky-300/30">update available</Badge>}
              {game.pinned && <Badge variant="accent" className="bg-amber-300/15 ring-1 ring-amber-300/30">pinned</Badge>}
            </div>
            <GameHeading game={game} />
          </div>
          <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-3">
            {(hours != null || last) && (
              <div className="text-right font-mono text-hint leading-relaxed text-white/55">
                {hours != null && <div title={playtimeTooltip(game)}>{hours}h logged</div>}
                {last && <div>last played {last}</div>}
              </div>
            )}
            <div className="flex flex-wrap items-center justify-end gap-2.5">
              {game.is_installed && plugin && game.update_available && !active && (
                <Button variant="neutral" size="none" onClick={() => p.onJob('update')} className="gap-2 rounded-2xl bg-black/50 px-4 py-3.5 text-[13px] text-sky-100 hover:bg-black/70">
                  <RefreshCw className="h-4 w-4" /> Update
                </Button>
              )}
              {!game.is_installed ? (
                <InstallButton
                  game={game}
                  job={job}
                  onInstall={() => (canChooseOptions ? setOptionsOpen(true) : p.onJob('install'))}
                  onInstallTo={plugin && !canChooseOptions ? () => setInstallTo(true) : undefined}
                  onCancel={(id) => p.onJobAction('cancel', id)}
                />
              ) : (
                <PlayButton session={session} launching={p.launching} disabled={game.outside_folder} onPlay={p.onPlay} onStop={p.onStop} onForceStop={p.onForceStop} />
              )}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button variant="neutral" size="none" onClick={() => setSettingsTab('general')} aria-label="Game settings" className="size-[50px] rounded-2xl bg-black/50 text-white/80 hover:bg-black/70">
                    <Settings2 className="h-[18px] w-[18px]" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Game settings</TooltipContent>
              </Tooltip>
            </div>
          </div>
        </div>
      </section>

      {game.is_installed && game.outside_folder && plugin && (
        <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-5 py-4 ring-1 ring-amber-300/25">
          <div className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
            <p className="font-bold text-amber-200">Installed outside {plugin.name}'s folder</p>
            <p>
              Platypus only launches {plugin.name} games from <span className="font-mono text-[12px]">{plugin.install_folder}</span> or folders you allow.
              This one is in <span className="font-mono text-[12px] text-white/90">{game.install_dir}</span>, probably from before Platypus managed it.
            </p>
          </div>
          <Button onClick={() => p.onAllowFolder(game.install_dir)} className="flex-none">
            <FolderCheck className="h-4 w-4" /> Allow this folder
          </Button>
        </div>
      )}

      {!active && (game.note || installing) && (
        <div className="mb-6 rounded-2xl bg-white/[0.04] px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-white/[0.07]">
          {game.note && <p>{game.note}</p>}
          {installing && plugin && (
            <>
              <InstallSizeLine size={installSize} />
              <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[12px] text-white/55">
                <span>installs into {plugin.install_folder}</span>
                {canChooseVersion && <><span aria-hidden>·</span><NoteLink onClick={() => setSettingsTab('versions')}>Choose version</NoteLink></>}
                <span aria-hidden>·</span><NoteLink onClick={() => p.onNavigate({ name: 'plugins' })}>change default</NoteLink>
              </p>
            </>
          )}
          {!game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback="The last attempt didn't finish." />}
        </div>
      )}

      {p.refreshing && (
        <p className="mb-4 font-mono text-micro uppercase tracking-widest text-white/55">refreshing library…</p>
      )}

      <div className="mb-6 grid gap-5 lg:grid-cols-[1.6fr_1fr]">
        <div className="min-w-0 space-y-5">
          {shots.length > 0 && (
            <div className="space-y-3">
              <Tooltip>
              <TooltipTrigger asChild>
              <button
                onClick={() => setLightbox(true)}
                aria-label="Open screenshots in slideshow"
                className="group relative block w-full cursor-zoom-in overflow-hidden rounded-3xl bg-black/40 ring-1 ring-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300"
              >
                <img key={shot} src={shots[shot]} alt={`${game.title} screenshot ${shot + 1}`} className="aspect-video w-full object-cover" />
                <span aria-hidden className="absolute right-3 top-3 grid h-8 w-8 place-items-center rounded-lg bg-black/60 text-white/80 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
                  <Maximize2 size={15} />
                </span>
              </button>
              </TooltipTrigger>
              <TooltipContent side="bottom">Open slideshow</TooltipContent>
              </Tooltip>
              <ScrollArea orientation="horizontal" type="always" viewportRef={strip} data-testid="shot-strip">
                {/* Padding keeps the active ring and rounded corners inside the viewport; pb leaves room for the overlay bar. */}
                <div data-testid="shot-strip-inner" className="flex w-max gap-2.5 px-1.5 pb-3.5 pt-1.5">
                  {shots.map((src, i) => (
                    <button
                      key={src}
                      data-thumb
                      onClick={() => setShot(i)}
                      onDoubleClick={() => setLightbox(true)}
                      aria-label={`Show screenshot ${i + 1}`}
                      aria-pressed={i === shot}
                      className={cx('aspect-video h-[84px] flex-none overflow-hidden rounded-xl bg-black/50 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300', i === shot ? 'ring-2 ring-amber-300/80' : 'opacity-55 ring-1 ring-white/[0.08] hover:opacity-90')}
                    >
                      <img src={src} alt="" loading="lazy" className="h-full w-full object-contain" />
                    </button>
                  ))}
                </div>
              </ScrollArea>
              {lightbox && <Lightbox images={shots} index={shot} alt={game.title} onIndex={setShot} onClose={() => setLightbox(false)} />}
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
              <p className="text-[13px] text-white/70">{error ? `Store data unavailable: ${error}` : 'No store description for this title.'}</p>
            )}
            {about.length > 0 && <p className="mt-3 font-mono text-[12px] text-white/55">{about.join('  ·  ')}</p>}
          </Panel>
        </div>

        <aside className="min-w-0 space-y-5">
          <Panel className="bg-white/[0.05]">
            <h2 className="mb-2 text-[15px] font-bold">Compatibility</h2>
            <p className="text-[13px] leading-relaxed text-white/65">{compat}</p>
            {fix?.winetricks && fix.winetricks.length > 0 && (
              <p className="mt-2 font-mono text-hint text-sky-200/80">recipe: winetricks {fix.winetricks.join(' ')}</p>
            )}
            <div className="mt-4 space-y-2 font-mono text-hint">
              {facts.map(([k, v]) => (
                <div key={k} className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
                  <span className="text-white/55">{k}</span>
                  <span className="truncate text-right text-white/85">{v}</span>
                </div>
              ))}
            </div>
            {info && <CompatFacts compat={info} />}
          </Panel>

          {isPluginGame(game) && game.is_installed && plugin && (
            <InstallationPanel game={game} plugin={plugin} job={job} onJobAction={p.onJobAction} />
          )}

          {isPluginGame(game) && plugin && <CopyPicker game={game} plugin={plugin} onChanged={() => p.onCopyChanged?.()} />}
        </aside>
      </div>

      {progress ? (
        <ProgressAchievements progress={progress} />
      ) : ach && ach.total > 0 ? (
        <AchievementsSection ach={ach} known={ach.unlocked >= 0} viaSteam={game.source === 'steam'} />
      ) : null}

      {events.length > 0 && (
        <section className="pb-10">
          <MonoLabel className="mb-2">this session</MonoLabel>
          <ActivityFeed events={events} />
        </section>
      )}
      {settingsTab && (
        <GameSettingsModal
          game={game}
          plugin={plugin}
          runners={runners}
          tweaks={tweaks}
          jobs={p.jobs ?? []}
          job={job}
          initialTab={settingsTab}
          onClose={() => setSettingsTab(null)}
          onSaveSettings={p.onSaveSettings}
          onChangeSettings={p.onChangeSettings}
          onJob={p.onJob}
          onJobAction={p.onJobAction}
          onNotify={(msg, kind) => p.onNotify?.(msg, kind)}
          onChanged={() => p.onVersionsChanged?.()}
          onCustomized={() => p.onCustomized?.()}
          onCopied={() => p.onNotify?.('Copied to clipboard')}
        />
      )}
      {installTo && plugin && (
        <InstallDialog
          game={game}
          plugin={plugin}
          onClose={() => setInstallTo(false)}
          onChanged={() => p.onVersionsChanged?.()}
          onNotify={(msg, kind) => p.onNotify?.(msg, kind)}
        />
      )}
      {optionsOpen && plugin && (
        <InstallOptionsDialog
          game={game}
          plugin={plugin}
          mode="install"
          onClose={() => setOptionsOpen(false)}
          onChanged={() => p.onVersionsChanged?.()}
          onNotify={(msg, kind) => p.onNotify?.(msg, kind)}
        />
      )}
    </div>
    </TooltipProvider>
  );
}

function InstallButton({
  game, job, onInstall, onInstallTo, onCancel,
}: { game: Game; job?: Job; onInstall: () => void; onInstallTo?: () => void; onCancel: (id: string) => void }) {
  if (job && (job.state === 'queued' || job.state === 'running')) return <JobProgress job={job} onCancel={onCancel} />;
  if (!game.installable) {
    return (
      <span className="rounded-2xl bg-white/[0.08] px-6 py-3.5 text-[13px] font-bold text-white/55" title={game.note}>
        Can't install here
      </span>
    );
  }
  return (
    <>
      {onInstallTo && (
        <Button variant="neutral" size="none" onClick={onInstallTo} className="gap-2 rounded-2xl bg-black/50 px-4 py-3.5 text-[13px] text-white/80 hover:bg-black/70">
          <FolderInput className="h-4 w-4" /> Install to…
        </Button>
      )}
      <Button
        variant="primary"
        size="none"
        onClick={onInstall}
        className="gap-2 rounded-2xl px-8 py-3.5 text-[15px] shadow-[0_12px_30px_rgba(252,211,77,.25)]"
      >
        <Download className="h-4 w-4" strokeWidth={3} />
        {job?.state === 'failed' ? 'Retry install' : 'Install'}
      </Button>
    </>
  );
}

/** The game's name over the banner: the user's logo when they set one (the
 *  title stays as its accessible name), else the title in type. */
function GameHeading({ game }: { game: Game }) {
  const [broken, setBroken] = useState<string | null>(null);
  const logo = game.logo_url && game.logo_url !== broken ? game.logo_url : null;
  return (
    <h1 className={cx(logo ? 'flex' : 'truncate text-[36px] font-extrabold leading-none tracking-tight')}>
      {logo ? (
        <img src={logo} alt={game.title} onError={() => setBroken(logo)} className="max-h-[96px] max-w-full object-contain object-left" />
      ) : (
        game.title
      )}
    </h1>
  );
}

/** The size of a fresh install, said before it is queued. */
function InstallSizeLine({ size }: { size: InstallSize }) {
  return (
    <p className="mb-1 font-mono text-[12px] text-white/75" data-testid="install-size-line">
      {size.state === 'loading' && <span className="text-white/55">checking the install size…</span>}
      {size.state === 'unknown' && <span title="The store can't tell how big this is">Size unknown</span>}
      {size.state === 'known' && (
        <>
          Install size {formatBytes(size.total)}
          {size.added > 0 && <span className="text-white/55"> · includes {formatBytes(size.added)} of add-ons</span>}
        </>
      )}
    </p>
  );
}

function NoteLink({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <Button variant="ghost" size="none" onClick={onClick} className="inline-flex rounded-none px-0 font-mono text-[12px] font-normal underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/80">
      {children}
    </Button>
  );
}

/** Where a plugin game lives: key facts only. What can be done with it is in the game settings. */
function InstallationPanel({
  game,
  plugin,
  job,
  onJobAction,
}: {
  game: Game;
  plugin: StorePlugin;
  job?: Job;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
}) {
  const active = job && (job.state === 'queued' || job.state === 'running');
  return (
    <Panel className="bg-white/[0.05]">
      <h2 className="mb-2 text-[15px] font-bold">Installation</h2>
      <div className="space-y-2 font-mono text-hint">
        {[
          ['Store', `${storeLabel(game.store)} · ${plugin.name}`],
          // A game the store lists as installed may have no folder Platypus can name: say so.
          ['Folder', game.install_dir || (game.is_installed ? 'not known to Platypus' : '—')],
          ['Version', game.version || '—'],
          ...(game.pinned ? [['Pinned to', pinLabel(game.pinned)]] : []),
          ['Size', formatBytes(game.size_bytes) || '—'],
        ].map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
            <span className="text-white/55">{k}</span>
            <span className="truncate text-right text-white/85 select-text" title={v}>{v}</span>
          </div>
        ))}
      </div>
      {active && <div className="mt-4"><JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} /></div>}
      {job?.state === 'failed' && !active && (
        <ErrorNotice className="mt-2" message={job.error ?? ''} fallback={`The ${job.kind} didn't finish.`} onRetry={() => onJobAction('retry', job.id)} />
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
        <span className="font-mono text-[12px] tabular-nums text-white/55">
          {known ? `${ach.unlocked}/${ach.total} · ${pct}%` : `${ach.total} total`}
        </span>
      </div>
      {known ? (
        <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
          <div className="h-full rounded-full bg-amber-300" style={{ width: `${pct}%` }} />
        </div>
      ) : (
        <p className="mb-4 text-[12px] text-white/70">
          {viaSteam
            ? "Your progress isn't cached yet. Open this game once in Steam's library and it shows up here."
            : 'The store lists these achievements; your progress there isn\'t available to Platypus.'}
        </p>
      )}
      <div className="ach-grid">
        {list.map((a) => (
          <AchievementCard key={a.id || a.name} a={a} known={known} />
        ))}
      </div>
      {list.length < ach.total && (
        <p className="mt-3 font-mono text-[12px] text-white/55">
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
        <span className="grid h-11 w-11 flex-none place-items-center rounded-lg bg-white/[0.05]"><Trophy className="h-4 w-4 text-white/55" /></span>
      )}
      <div className={cx('min-w-0', dim && 'opacity-60')}>
        <div className="truncate text-[12px] font-bold">{a.name}</div>
        {a.description && <div className="line-clamp-2 text-hint leading-snug text-white/55">{a.description}</div>}
        {meta && <div className="mt-0.5 font-mono text-hint text-white/55">{meta}</div>}
      </div>
    </div>
  );
}
