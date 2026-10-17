// i18n: converted
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { Effect } from 'effect';
import { ArrowLeft, Download, FolderCheck, FolderInput, Maximize2, Play, RefreshCw, Search, Settings2, Store as StoreIcon, Trophy } from 'lucide-react';
import type { ActivityEvent, Achievement, ArtCandidate, Collection, Game, GameDetails, GameSettings, GameJobKind, GameStatus, Job, ProtonVersion, RemoveOptions, SessionEvent, StorePlugin, Tweak, View } from '../types';
import { ActivityFeed } from './ActivityFeed';
import { PlayButton } from './PlayButton';
import { CopyPicker } from './CopyPicker';
import { Lightbox, toMediaItems, type MediaItem } from './Lightbox';
import { ExternalLink } from './ExternalLink';
import { openExternal } from '../lib/external';
import { InstallOptionsDialog } from './InstallOptionsDialog';
import { InstallDialog } from './InstallDialog';
import { GameSettingsModal } from './GameSettingsModal';
import { JobProgress } from './JobProgress';
import { CompatFacts, deckLabel, tierLabel, tierMeaning } from './CompatBadges';
import { useCompat } from '../lib/useCompat';
import { steamDeckFact } from '../lib/compat';
import { useCrashHints } from '../lib/useCrashHints';
import { CrashHintsPanel } from './CrashHints';
import { Badge } from '@/components/ds/badge';
import { Button } from '@/components/ds/button';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ds/tooltip';
import { ScrollArea } from '@/components/ds/scroll-area';
import { Cover, MonoLabel, Panel, TierBadge, cx } from './ui';
import { PluginLogo } from './PluginLogo';
import { formatBytes, heroImages, hoursPlayed, isPluginGame, needsAttention, storeLabel } from '../lib/games';
import { useNetState } from './useNetState';
import { formatCount, formatDate, formatHours, formatPercentValue } from '../lib/format';
import { formatDecimal1, recencyOf, renderRecency } from '../lib/gameLocale';
import { LauncherApi } from '../effect/runtime';
import { offers } from '../lib/capabilities';
import { ProgressAchievements, useAchievementProgress } from './ProgressAchievements';
import { pinLabel } from '../lib/versions';
import { isCustomized } from '../lib/customize';
import { useInstallSize, type InstallSize } from '../lib/installSize';
import type { GameSettingsTab } from '../lib/gameSettingsTabs';
import { ErrorNotice } from './ErrorNotice';
import { Input } from '@/components/ds/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ds/select';
import { Textarea } from '@/components/ds/textarea';

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
  /** Plugin logos by store (and plugin id): the store badge and details row. */
  sourceLogos?: Record<string, string>;
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
  /** Removes a sideloaded game from the library (custom games only). */
  onRemove?: (opts: RemoveOptions) => void;
  /** The collections (issue #45) and the dialog that changes a game's ones. */
  collections?: Collection[];
  onManageCollections?: (gameId: string) => void;
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
  const [nonce, setNonce] = useState(0);
  const [details, setDetails] = useState<GameDetails | null>(() => {
    const cached = detailsCache.get(game.id);
    return cached && cached.version === version ? cached.details : null;
  });
  const [error, setError] = useState<string | null>(null);
  // Steam, store plugins and sideloaded games (linked info, issue #38) all
  // have metadata; an unlinked sideloaded game resolves to empty details.
  const hasSource = game.source === 'steam' || game.source === 'custom' || isPluginGame(game);
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
  }, [game.id, game.version, nonce]);
  const reload = () => {
    invalidateGameDetailsCache(game.id);
    setNonce((n) => n + 1);
  };
  return { details, error, loading: !details && !error && hasSource, reload };
}

export function GamePage(p: GamePageProps) {
  const { t } = useTranslation('game');
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
  const { details, error, loading, reload } = useDetails(game);
  // ProtonDB, native, DRM and anti-cheat facts (internal/compat); null while loading.
  const { compat: info } = useCompat(game);
  // What the last launch's log says went wrong (issue #46); null while it runs.
  const crash = useCrashHints(game, !!session || p.launching);
  // Offline (issue #44): a "network" game shows a needs-connection chip while
  // offline. Play stays enabled — only the store plugin can refuse the launch.
  const { online } = useNetState();
  const needsConnection = !online && (game as Game & { offline?: string }).offline === 'network';
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

  const s = game.settings;
  // A game that never saved a MangoHud preset takes use_mangohud's old meaning.
  const mangohudOn = s.mangohud_preset ? s.mangohud_preset !== 'off' : !!s.use_mangohud;
  const autoRunner = runners.find((r) => r.is_default);
  const runner = runners.find((r) => r.id === s.runner_id || r.name === s.runner_id);
  const viaSteam = game.source === 'steam' && !game.executable;
  const plugin = p.plugin;
  const job = p.job;
  const active = job && (job.state === 'queued' || job.state === 'running');
  const fix = game.fix_info;
  const ratingBadge = info?.badges?.find((b) => b.kind === 'rating');
  const deck = steamDeckFact(info?.badges);
  const shots = details?.screenshots ?? [];
  const videos = (details as unknown as { videos?: { url: string; thumbnail?: string; title?: string; mime?: string }[] | null } | null)?.videos ?? [];
  const media: MediaItem[] = useMemo(() => toMediaItems(shots, videos ?? []), [details]);
  const ach = details?.achievements;
  const canChooseVersion = !!plugin && game.installable !== false && offers(plugin, 'versions', { game });
  const canChooseOptions = !!plugin && game.installable !== false && offers(plugin, 'install-options', { game });
  const installing = !game.is_installed && !!plugin && !!game.installable;
  const installSize = useInstallSize(game, plugin, installing);
  const events = activity.filter((e) => e.game === game.title).slice(0, 6);
  const hours = hoursPlayed(game);
  const recency = recencyOf(game.last_played_at);
  const last = recency ? renderRecency(t, 'game:recency', recency) : undefined;
  // The tooltip pairs the local record with the store's report, like before.
  const playtimeTitle =
    game.playtime_store_minutes === undefined
      ? undefined
      : game.playtime_local_minutes === undefined
        ? t('game:hero.playtimeStore', { store: formatHours(game.playtime_store_minutes) })
        : t('game:hero.playtimeBoth', { local: formatHours(game.playtime_local_minutes), store: formatHours(game.playtime_store_minutes) });
  // The store's logo: the game's own plugin first (game.source is its id),
  // then its store. A missing logo falls back to a monogram tile in PluginLogo.
  const storeLogo = p.sourceLogos?.[game.source] ?? p.sourceLogos?.[game.store];
  const storeName = game.store === 'custom' ? t('game:store.sideloaded') : storeLabel(game.store);

  const facts: [string, string][] = [
    [
      t('game:facts.runner'),
      game.native
        ? t('game:facts.nativeRunner')
        : viaSteam
          ? t('game:facts.steamRunner')
          : runner?.name ?? (!s.runner_id || s.runner_id === 'auto' ? t('game:facts.autoRunner', { name: autoRunner?.name ?? t('game:facts.noRunner') }) : t('game:facts.missingRunner', { id: s.runner_id })),
    ],
    [t('game:facts.dx'), game.native ? t('game:facts.dxUnused') : s.dx_layer === 'wined3d' ? t('game:facts.dxWined3d') : t('game:facts.dxBundled')],
    [t('game:facts.wrappers'), [s.use_gamemode && 'gamemode', mangohudOn && 'mangohud', s.use_gamescope && 'gamescope'].filter(Boolean).join(' · ') || t('game:facts.noWrappers')],
    [t('game:facts.launchVia'), viaSteam ? t('game:facts.steamClient') : plugin ? t('game:facts.viaPlatypus', { name: plugin.name }) : 'Platypus'],
    [
      t('game:facts.compatibility'),
      !info ? t('game:facts.pending') : ratingBadge ? `${ratingBadge.source} · ${ratingBadge.label}` : info.steam_app_id ? t('game:facts.protondbPending') : t('game:facts.protondbNoAppId'),
    ],
    [
      t('game:facts.deck'),
      !info
        ? t('game:facts.pending')
        : deck
          ? deckLabel(t, deck.rating)
          : info.steam_app_id
            ? t('game:facts.deckPending')
            : t('game:facts.deckNoAppId'),
    ],
  ];
  const compat = fix?.description || (ratingBadge
    ? `${ratingBadge.source} · ${ratingBadge.label}`
    : t('game:compat.noneLoaded'));

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
              {fix?.tier && <TierBadge tier={fix.tier} suffix={t('game:tierBadge.curated')} title={tierMeaning(t, fix.tier)} />}
              <Badge className="bg-black/50 text-white/75" data-testid="store-badge">
                <span className="inline-flex items-center gap-1.5">
                  <PluginLogo src={storeLogo} name={storeName} size="xs" />
                  <span>{storeName}</span>
                </span>
              </Badge>
              {formatBytes(game.size_bytes) && <Badge className="bg-black/50 text-white/75">{formatBytes(game.size_bytes)}</Badge>}
              {!game.is_installed && <Badge className="bg-black/50 text-white/55">{t('game:hero.notInstalled')}</Badge>}
              {game.update_available && <Badge variant="info" className="bg-sky-400/15 ring-1 ring-sky-300/30">{t('game:hero.updateAvailable')}</Badge>}
              {game.pinned && <Badge variant="accent" className="bg-amber-300/15 ring-1 ring-amber-300/30">{t('game:hero.pinned')}</Badge>}
              {needsConnection && (
                <Badge className="bg-amber-300/15 text-amber-100 ring-1 ring-amber-300/30" data-testid="needs-connection">
                  {t('game:hero.needsConnection', { store: storeName })}
                </Badge>
              )}
            </div>
            <GameHeading game={game} />
          </div>
          <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-3">
            {(hours != null || last) && (
              <div className="text-right font-mono text-hint leading-relaxed text-white/55">
                {hours != null && <div title={playtimeTitle}>{t('game:hero.hoursLogged', { hours: formatDecimal1(hours) })}</div>}
                {last && <div>{t('game:hero.lastPlayed', { when: last })}</div>}
              </div>
            )}
            <div className="flex flex-wrap items-center justify-end gap-2.5">
              {game.is_installed && plugin && game.update_available && !active && (
                <Button variant="neutral" size="none" onClick={() => p.onJob('update')} className="gap-2 rounded-2xl bg-black/50 px-4 py-3.5 text-[13px] text-sky-100 hover:bg-black/70">
                  <RefreshCw className="h-4 w-4" /> {t('game:hero.update')}
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
                  <Button variant="neutral" size="none" onClick={() => setSettingsTab('general')} aria-label={t('game:hero.gameSettings')} className="size-[50px] rounded-2xl bg-black/50 text-white/80 hover:bg-black/70">
                    <Settings2 className="h-[18px] w-[18px]" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t('game:hero.gameSettings')}</TooltipContent>
              </Tooltip>
              {plugin?.storefront_url && details?.store_url && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="neutral"
                      size="none"
                      onClick={() => p.onNavigate({ name: 'store', plugin: plugin.id, url: details.store_url })}
                      aria-label={t('game:hero.openInStore', { title: game.title, store: plugin.name })}
                      className="size-[50px] rounded-2xl bg-black/50 text-white/80 hover:bg-black/70"
                    >
                      <StoreIcon className="h-[18px] w-[18px]" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('game:hero.openInStoreTip')}</TooltipContent>
                </Tooltip>
              )}
            </div>
          </div>
        </div>
      </section>

      {game.is_installed && game.outside_folder && plugin && (
        <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-amber-300/[0.07] px-5 py-4 ring-1 ring-amber-300/25">
          <div className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
            <p className="font-bold text-amber-200">{t('game:outside.title', { name: plugin.name })}</p>
            <p>
              <Trans
                i18nKey="game:outside.body"
                values={{ name: plugin.name, folder: plugin.install_folder, dir: game.install_dir }}
                components={{
                  folder: <span className="font-mono text-[12px]" />,
                  dir: <span className="font-mono text-[12px] text-white/90" />,
                }}
              />
            </p>
          </div>
          <Button onClick={() => p.onAllowFolder(game.install_dir)} className="flex-none">
            <FolderCheck className="h-4 w-4" /> {t('game:outside.allow')}
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
                {canChooseVersion ? (
                  <Trans
                    i18nKey="game:install.noteWithVersion"
                    values={{ folder: plugin.install_folder }}
                    components={{
                      v: <NoteLink onClick={() => setSettingsTab('versions')} />,
                      d: <NoteLink onClick={() => p.onNavigate({ name: 'plugins' })} />,
                    }}
                  />
                ) : (
                  <Trans
                    i18nKey="game:install.notePlain"
                    values={{ folder: plugin.install_folder }}
                    components={{ d: <NoteLink onClick={() => p.onNavigate({ name: 'plugins' })} /> }}
                  />
                )}
              </p>
            </>
          )}
          {!game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback={t('game:install.lastAttempt')} />}
        </div>
      )}

      {p.refreshing && (
        <p className="mb-4 font-mono text-micro uppercase tracking-widest text-white/55">{t('game:refreshing')}</p>
      )}

      <div className="mb-6 grid gap-5 lg:grid-cols-[1.6fr_1fr]">
        <div className="min-w-0 space-y-5">
          {media.length > 0 && (
            <MediaStrip game={game} media={media} index={shot} onIndex={setShot} onOpen={() => setLightbox(true)} stripRef={strip} />
          )}
          {lightbox && media.length > 0 && <Lightbox items={media} index={shot} alt={game.title} onIndex={setShot} onClose={() => setLightbox(false)} />}

          <AboutSection
            game={game}
            details={details}
            loading={loading}
            error={error}
            compatUrls={{ protondb: ratingBadge?.link, wiki: info?.wiki?.url }}
            onOpenStore={plugin?.storefront_url && details?.store_url ? () => p.onNavigate({ name: 'store', plugin: plugin.id, url: details.store_url }) : undefined}
            onReload={reload}
          />
        </div>

        <aside className="min-w-0 space-y-5">
          <Panel className="bg-white/[0.05]">
            <h2 className="mb-2 text-[15px] font-bold">{t('game:compat.heading')}</h2>
            <p className="text-[13px] leading-relaxed text-white/65">{compat}</p>
            {fix?.winetricks && fix.winetricks.length > 0 && (
              <p className="mt-2 font-mono text-hint text-sky-200/80">{t('game:compat.recipe', { mods: fix.winetricks.join(' ') })}</p>
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

          {crash && <CrashHintsPanel game={game} hints={crash} onNotify={p.onNotify} onRunner={() => setSettingsTab('general')} />}

          <StatusNotesSection game={game} onChanged={p.onCustomized} />

          <CollectionsSection game={game} collections={p.collections} onManage={p.onManageCollections} />

          {isPluginGame(game) && game.is_installed && plugin && (
            <InstallationPanel game={game} plugin={plugin} job={job} onJobAction={p.onJobAction} sourceLogos={p.sourceLogos} />
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
          <MonoLabel className="mb-2">{t('game:session')}</MonoLabel>
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
          onCopied={() => p.onNotify?.(t('game:copied'))}
          onRemove={p.onRemove}
          running={!!session || p.launching}
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

/** The store's HTML (description, requirements): core sanitizes it to the
 *  allowlist, so rendering it as markup is safe. Links open in the browser. */
function SafeHtml({ html, testid }: { html: string; testid?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const onClick = (e: MouseEvent) => {
      const a = (e.target as HTMLElement).closest?.('a');
      if (!a) return;
      const href = a.getAttribute('href');
      if (!href) return;
      e.preventDefault();
      openExternal(href);
    };
    el.addEventListener('click', onClick);
    return () => el.removeEventListener('click', onClick);
  }, []);
  return (
    <div
      ref={ref}
      data-testid={testid}
      dangerouslySetInnerHTML={{ __html: html }}
      className="space-y-2 text-[13px] leading-relaxed text-white/70 [&_a]:text-amber-200/90 [&_a]:underline [&_a]:decoration-amber-200/40 [&_a]:underline-offset-2 [&_h2]:text-[14px] [&_h2]:font-bold [&_h2]:text-white/85 [&_h3]:text-[13px] [&_h3]:font-bold [&_h3]:text-white/85 [&_h4]:text-[13px] [&_h4]:font-bold [&_h4]:text-white/85 [&_li]:ml-4 [&_li]:list-disc [&_ul]:space-y-1"
    />
  );
}

/** Screenshots + video thumbnails: lazy, nothing heavy on first paint. The
 *  hero shows the selected item; videos show their thumbnail with a play
 *  badge and open click-to-play in the lightbox (never autoplay with sound). */
function MediaStrip({ game, media, index, onIndex, onOpen, stripRef }: {
  game: Game;
  media: MediaItem[];
  index: number;
  onIndex: (i: number) => void;
  onOpen: () => void;
  stripRef: React.RefObject<HTMLDivElement | null>;
}) {
  const { t } = useTranslation('game');
  const cur = media[index] ?? media[0];
  const thumbLabel = (m: MediaItem, i: number) =>
    m.kind === 'video'
      ? t('game:media.trailerLabel', { suffix: m.title ? `"${m.title}" ` : '', index: i + 1, total: media.length })
      : t('game:media.screenshotLabel', { index: i + 1 });
  return (
    <div className="space-y-3">
      <Tooltip>
      <TooltipTrigger asChild>
      <button
        onClick={onOpen}
        aria-label={cur?.kind === 'video' ? t('game:media.openTrailer') : t('game:media.openShots')}
        className="group relative block w-full cursor-zoom-in overflow-hidden rounded-3xl bg-black/40 ring-1 ring-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300"
      >
        {cur?.kind === 'video' ? (
          cur.thumbnail ? (
            <img key={cur.src} src={cur.thumbnail} alt={`${game.title} trailer thumbnail`} loading="lazy" className="aspect-video w-full object-cover" />
          ) : (
            <span key={cur.src} className="grid aspect-video w-full place-items-center bg-black/60">
              <Play className="h-10 w-10 text-white/80" />
            </span>
          )
        ) : (
          cur && <img key={cur.src} src={cur.src} alt={`${game.title} screenshot ${index + 1}`} className="aspect-video w-full object-cover" />
        )}
        {cur?.kind === 'video' && (
          <span aria-hidden className="absolute inset-0 grid place-items-center">
            <span className="grid h-14 w-14 place-items-center rounded-full bg-black/65 ring-1 ring-white/20 transition group-hover:scale-105">
              <Play className="h-6 w-6 fill-white text-white" />
            </span>
          </span>
        )}
        <span aria-hidden className="absolute right-3 top-3 grid h-8 w-8 place-items-center rounded-lg bg-black/60 text-white/80 opacity-0 transition group-hover:opacity-100 group-focus-visible:opacity-100">
          <Maximize2 size={15} />
        </span>
      </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{t('game:media.openViewer')}</TooltipContent>
      </Tooltip>
      <ScrollArea orientation="horizontal" type="always" viewportRef={stripRef} data-testid="shot-strip">
        {/* Padding keeps the active ring and rounded corners inside the viewport; pb leaves room for the overlay bar. */}
        <div data-testid="shot-strip-inner" className="flex w-max gap-2.5 px-1.5 pb-3.5 pt-1.5">
          {media.map((m, i) => (
            <button
              key={`${m.kind}-${m.src}`}
              data-thumb
              onClick={() => onIndex(i)}
              onDoubleClick={onOpen}
              aria-label={thumbLabel(m, i)}
              aria-pressed={i === index}
              className={cx('relative aspect-video h-[84px] flex-none overflow-hidden rounded-xl bg-black/50 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300', i === index ? 'ring-2 ring-amber-300/80' : 'opacity-55 ring-1 ring-white/[0.08] hover:opacity-90')}
            >
              {m.kind === 'video' ? (
                m.thumbnail ? (
                  <img src={m.thumbnail} alt="" loading="lazy" className="h-full w-full object-cover" />
                ) : (
                  <span className="grid h-full w-full place-items-center"><Play className="h-5 w-5 text-white/80" /></span>
                )
              ) : (
                <img src={m.src} alt="" loading="lazy" className="h-full w-full object-contain" />
              )}
              {m.kind === 'video' && (
                <span aria-hidden className="absolute inset-0 grid place-items-center">
                  <span className="grid h-7 w-7 place-items-center rounded-full bg-black/65 ring-1 ring-white/20">
                    <Play className="h-3.5 w-3.5 fill-white text-white" />
                  </span>
                </span>
              )}
            </button>
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}

function controllerLabel(t: (key: string) => string, c?: string): string | null {
  if (c === 'full') return t('game:about.controllerFull');
  if (c === 'partial') return t('game:about.controllerPartial');
  return null;
}

/** The About section: formatted description (collapsed + Show more), details
 *  grid, requirements tabs and links. Calm by default: secondary details live
 *  here, not in the hero. Sideloaded games (issue #38) get a "Find info
 *  online" action when nothing is linked, and an attribution + Unlink control
 *  once linked. */
function AboutSection({ game, details, loading, error, compatUrls, onOpenStore, onReload }: {
  game: Game;
  details: GameDetails | null;
  loading: boolean;
  error: string | null;
  compatUrls: { protondb?: string; wiki?: string };
  /** Opens the store page in the embedded store view, when the plugin has one. */
  onOpenStore?: () => void;
  /** The info link changed: details were evicted and refetched. */
  onReload?: () => void;
}) {
  const { t } = useTranslation('game');
  const [expanded, setExpanded] = useState(false);
  const [reqTab, setReqTab] = useState<'minimum' | 'recommended'>('minimum');
  useEffect(() => { setExpanded(false); setReqTab('minimum'); }, [game.id]);
  const d = details as unknown as (GameDetails & {
    description_html?: string; website?: string; store_url?: string;
    info_source?: string; info_source_name?: string; info_stale?: boolean;
    languages?: string[]; controller?: string;
    requirements?: { minimum?: string; recommended?: string } | null;
    age_rating?: string; tags?: string[]; videos?: unknown[];
  }) | null;
  const html = d?.description_html || '';
  const summary = details?.summary || '';
  const isCustom = game.store === 'custom' || game.source === 'custom';
  const linked = !!((d?.info_source || d?.info_source_name) && isCustom);
  const genres = [...(details?.genres ?? []), ...((d?.tags ?? []).filter((t) => !(details?.genres ?? []).includes(t)) ?? [])];
  const controller = controllerLabel(t, d?.controller);
  const rows: [string, string][] = [
    ...(details?.developers?.length ? [[t('game:about.developer'), details.developers.join(', ')] as [string, string]] : []),
    ...(details?.publishers?.length ? [[t('game:about.publisher'), details.publishers.join(', ')] as [string, string]] : []),
    // The user's own year wins over the source's release date (issue #38):
    // the source's title is never written anywhere.
    ...(isCustom && game.release_year
      ? [[t('game:about.release'), String(game.release_year)] as [string, string]]
      : details?.release_date
        ? [[t('game:about.release'), details.release_date] as [string, string]]
        : []),
    ...(genres.length ? [[t('game:about.genres'), genres.join(' · ')] as [string, string]] : []),
    ...((d?.languages?.length ?? 0) ? [[t('game:about.languages'), (d?.languages ?? []).join(', ')] as [string, string]] : []),
    ...(controller ? [[t('game:about.controller'), controller] as [string, string]] : []),
    ...(d?.age_rating ? [[t('game:about.ageRating'), d.age_rating] as [string, string]] : []),
  ];
  const req = d?.requirements;
  const links: [string, string][] = [
    ...(d?.website ? [[t('game:about.linkWebsite'), d.website] as [string, string]] : []),
    ...(d?.store_url ? [[t('game:about.linkStore'), d.store_url] as [string, string]] : []),
    ...(compatUrls.protondb ? [[t('game:about.linkProtondb'), compatUrls.protondb] as [string, string]] : []),
    ...(compatUrls.wiki ? [[t('game:about.linkPcgw'), compatUrls.wiki] as [string, string]] : []),
  ];
  const infoName = d?.info_source_name || infoSourceText(t, d?.info_source);
  const storeLinkLabel = t('game:about.linkStore');
  return (
    <Panel className="bg-white/[0.05]">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:about.heading')}</h2>
      {loading ? (
        <div className="space-y-2">
          <div className="h-3 w-full animate-pulse rounded bg-white/[0.06]" />
          <div className="h-3 w-4/5 animate-pulse rounded bg-white/[0.06]" />
        </div>
      ) : html ? (
        <>
          <div className={expanded ? '' : 'max-h-[104px] overflow-hidden'} data-testid="about-html-wrap">
            <SafeHtml html={html} testid="about-html" />
          </div>
          <Button variant="ghost" size="none" onClick={() => setExpanded((v) => !v)} data-testid="about-toggle" className="mt-1.5 rounded px-0 font-mono text-[12px] underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/85">
            {expanded ? t('game:about.showLess') : t('game:about.showMore')}
          </Button>
        </>
      ) : summary ? (
        <p className="text-[13px] leading-relaxed text-white/70">{summary}</p>
      ) : (
        <p className="text-[13px] text-white/70">{error ? t('game:about.storeUnavailable', { error }) : t('game:about.noDescription')}</p>
      )}
      {isCustom && !linked && !loading && (
        <InfoFinder game={game} onLinked={() => onReload?.()} />
      )}
      {rows.length > 0 && (
        <dl data-testid="details-grid" className="mt-4 grid grid-cols-2 gap-x-6 gap-y-2 border-t border-white/[0.06] pt-3">
          {rows.map(([k, v]) => (
            <div key={k} className="min-w-0">
              <dt className="font-mono text-hint uppercase tracking-widest text-white/45">{k}</dt>
              <dd className="truncate text-[12.5px] text-white/80" title={v}>{v}</dd>
            </div>
          ))}
        </dl>
      )}
      {req && (req.minimum || req.recommended) && (
        <div className="mt-4 border-t border-white/[0.06] pt-3" data-testid="requirements">
          <div role="tablist" aria-label={t('game:about.reqLabel')} className="mb-2 flex gap-1.5">
            {(['minimum', 'recommended'] as const).filter((k) => req[k]).map((k) => (
              <Button
                key={k}
                role="tab"
                aria-selected={reqTab === k}
                data-testid={`req-tab-${k}`}
                variant="ghost"
                size="none"
                onClick={() => setReqTab(k)}
                className={reqTab === k ? 'rounded-lg bg-white/[0.08] px-2.5 py-1 font-mono text-[12px] text-white' : 'rounded-lg px-2.5 py-1 font-mono text-[12px] text-white/55 hover:bg-transparent hover:text-white/80'}
              >
                {k === 'minimum' ? t('game:about.reqMinimum') : t('game:about.reqRecommended')}
              </Button>
            ))}
          </div>
          <div role="tabpanel" data-testid="req-panel">
            <SafeHtml html={req[reqTab] ?? req.minimum ?? req.recommended ?? ''} />
          </div>
        </div>
      )}
      {links.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-x-4 gap-y-1.5 border-t border-white/[0.06] pt-3 font-mono text-[12px]" data-testid="about-links">
          {links.map(([label, href]) =>
            label === storeLinkLabel && onOpenStore ? (
              <button key={label} type="button" onClick={onOpenStore} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
                {label}
              </button>
            ) : (
            <ExternalLink key={label} href={href} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
              {label} ↗
            </ExternalLink>
            ),
          )}
        </div>
      )}
      {linked && (
        <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-white/[0.06] pt-3 font-mono text-[12px] text-white/55" data-testid="info-attribution">
          <span>
            {t('game:about.infoFrom')} {d?.store_url ? (
              <ExternalLink href={d.store_url} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
                {infoName} ↗
              </ExternalLink>
            ) : infoName}
          </span>
          <InfoUnlink game={game} onUnlinked={() => onReload?.()} />
        </div>
      )}
      {d?.info_stale && (
        <p className="mt-2 font-mono text-[12px] text-amber-200/70" data-testid="info-stale">
          {t('game:about.infoStale')}
        </p>
      )}
    </Panel>
  );
}

/** The small source label of one info candidate row. */
function infoSourceText(t: (key: string) => string, source?: string): string {
  if (source === 'gog') return t('game:source.gog');
  if (source === 'lutris') return t('game:source.lutris');
  if (source === 'steam') return t('game:source.steam');
  return '';
}

/** One candidate's identity across sources. */
function infoCandidateKey(c: ArtCandidate): string {
  return `${c.source || 'steam'}:${c.id || c.app_id}`;
}

/** The small id detail of one candidate row. */
function infoCandidateDetail(t: (key: string, vars?: Record<string, unknown>) => string, c: ArtCandidate): string {
  if ((c.source || 'steam') === 'steam') return t('game:source.steamApp', { id: c.app_id || c.id });
  return c.id;
}

/** The user's own status and plain-text notes for a game (issue #51): the
 *  select saves at once, the notes autosave on a debounce and on blur, one
 *  write at a time so a slow save can't overwrite a newer one. */
function StatusNotesSection({ game, onChanged }: { game: Game; onChanged?: () => void }) {
  const { t } = useTranslation('game');
  const [status, setStatus] = useState(game.status ?? '');
  const [statusError, setStatusError] = useState<string | null>(null);
  const [notes, setNotes] = useState(game.user_notes ?? '');
  const [notesState, setNotesState] = useState<'idle' | 'saving' | 'saved' | 'failed'>('idle');
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Every notes write runs through one promise chain, so writes are strictly
  // ordered and a slow save can't overwrite a newer one. The game id is
  // captured when a write is enqueued, so a save of the previous game never
  // touches this game's state.
  const chain = useRef<Promise<void>>(Promise.resolve());
  // The text waiting on the debounce, not yet enqueued: { id, text } for the
  // game it was typed on. null when nothing is pending.
  const pending = useRef<{ id: string; text: string } | null>(null);
  // The last text enqueued for the game on screen: a blur with nothing
  // changed enqueues nothing.
  const lastText = useRef(game.user_notes ?? '');
  const shownId = useRef(game.id);

  const enqueueNotes = (id: string, text: string) => {
    lastText.current = text;
    setNotesState('saving');
    chain.current = chain.current.then(() =>
      Effect.runPromise(LauncherApi.setGameNotes(id, text)).then(
        () => { if (shownId.current === id) setNotesState('saved'); },
        () => { if (shownId.current === id) setNotesState('failed'); },
      ),
    );
  };

  useEffect(() => {
    // A text typed inside the debounce is flushed for the old game before it
    // is dropped, so navigating away never loses it.
    if (pending.current) {
      clearTimeout(timer.current);
      enqueueNotes(pending.current.id, pending.current.text);
      pending.current = null;
    }
    shownId.current = game.id;
    lastText.current = game.user_notes ?? '';
    setStatus(game.status ?? '');
    setNotes(game.user_notes ?? '');
    setStatusError(null);
    // The label is about the game on screen: an in-flight save's "Saving…"
    // belongs to the game we just left.
    setNotesState('idle');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [game.id]);

  const saveStatus = (next: GameStatus) => {
    setStatusError(null);
    const at = game.id;
    Effect.runPromise(LauncherApi.setGameStatus(at, next)).then(
      (info) => { if (shownId.current === at) setStatus(info.status); onChanged?.(); },
      (e) => { if (shownId.current === at) setStatusError(e instanceof Error ? e.message : String(e)); },
    );
  };

  // The debounce timer: it enqueues the pending text for its own game.
  const requestNotesSave = (text: string, delay: number) => {
    if (text === lastText.current) { // nothing changed since the last enqueued text
      pending.current = null;
      clearTimeout(timer.current);
      return;
    }
    const id = game.id;
    pending.current = { id, text };
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      pending.current = null;
      enqueueNotes(id, text);
    }, delay);
  };

  return (
    <Panel className="bg-white/[0.05]" data-testid="status-notes">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:status.heading')}</h2>
      <Select value={status || 'none'} onValueChange={(v) => saveStatus((v === 'none' ? '' : v) as GameStatus)}>
        <SelectTrigger aria-label={t('game:status.aria')} className="w-full" data-testid="status-select">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {(['none', 'backlog', 'playing', 'finished', 'abandoned'] as const).map((v) => (
            <SelectItem key={v} value={v}>
              {t(`game:status.${v}`)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {statusError && (
        <p role="alert" className="mt-1.5 text-[12px] text-red-300/80">
          {statusError}
        </p>
      )}
      <MonoLabel className="mb-1.5 mt-4">{t('game:status.notesLabel')}</MonoLabel>
      <Textarea
        value={notes}
        rows={4}
        aria-label={t('game:status.notesLabel')}
        placeholder={t('game:status.notesPlaceholder')}
        data-testid="notes-input"
        onChange={(e) => {
          setNotes(e.target.value);
          requestNotesSave(e.target.value, 500);
        }}
        onBlur={() => requestNotesSave(notes, 0)}
      />
      <MonoLabel className={cx('mt-1.5 text-right', notesState === 'failed' && 'text-red-300/80')}>
        {notesState === 'saving' ? t('game:status.saving') : notesState === 'saved' ? t('game:status.saved') : notesState === 'failed' ? t('game:status.failed') : ''}
      </MonoLabel>
    </Panel>
  );
}

/** The manual collections a game is in (issue #45), and the way to change them. */
function CollectionsSection({ game, collections = [], onManage }: { game: Game; collections?: Collection[]; onManage?: (gameId: string) => void }) {
  const { t } = useTranslation('collections');
  if (!onManage) return null;
  const mine = collections.filter((c) => !c.smart && c.games?.includes(game.id));
  return (
    <Panel className="bg-white/[0.05]" data-testid="game-collections">
      <h2 className="mb-2 text-[15px] font-bold">{t('collections:gameHeading')}</h2>
      <p className="text-[13px] text-white/75">
        {mine.length > 0 ? mine.map((c) => (c.builtin ? t(`builtin.${c.id}`) : c.name)).join(', ') : t('collections:notInCollections')}
      </p>
      <Button variant="neutral" className="mt-3 w-full" onClick={() => onManage(game.id)}>
        {t('collections:addToCollection')}
      </Button>
    </Panel>
  );
}

/** The Unlink control next to a linked game's attribution. */
function InfoUnlink({ game, onUnlinked }: { game: Game; onUnlinked: () => void }) {
  const { t } = useTranslation('game');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const unlink = () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    Effect.runPromise(LauncherApi.linkGameInfo(game.id, '', '')).then(
      () => { setBusy(false); onUnlinked(); },
      (e) => { setBusy(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };
  return (
    <span className="inline-flex items-center gap-2">
      <Button variant="ghost" size="none" onClick={unlink} disabled={busy} data-testid="info-unlink" className="rounded px-0 underline decoration-white/25 underline-offset-2 hover:bg-transparent hover:text-white/85">
        {busy ? t('game:about.unlinking') : t('game:about.unlink')}
      </Button>
      {error && <span className="text-red-300/80">{error}</span>}
    </span>
  );
}

/** Find a sideloaded game's info online (issue #38): the same candidate
 *  search as Find art online (SearchArtOnline), the same list (source label,
 *  year), and a pick links via LinkGameInfo. */
function InfoFinder({ game, onLinked }: { game: Game; onLinked: () => void }) {
  const { t } = useTranslation('game');
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState(game.title);
  const [candidates, setCandidates] = useState<ArtCandidate[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [linking, setLinking] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const search = () => {
    const q = query.trim();
    if (!q || searching || linking != null) return;
    setSearching(true);
    setError(null);
    Effect.runPromise(LauncherApi.searchArtOnline(q)).then(
      (cs) => { setSearching(false); setCandidates(cs); },
      (e) => { setSearching(false); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  const link = (c: ArtCandidate) => {
    if (searching || linking != null) return;
    const source = c.source || 'steam';
    const id = source === 'steam' ? (c.id || String(c.app_id)) : c.id;
    setLinking(infoCandidateKey(c));
    setError(null);
    Effect.runPromise(LauncherApi.linkGameInfo(game.id, source, id)).then(
      () => { setLinking(null); onLinked(); },
      (e) => { setLinking(null); setError(e instanceof Error ? e.message : String(e)); },
    );
  };

  if (!open) {
    return (
      <div className="mt-3">
        <Button variant="neutral" size="sm" onClick={() => setOpen(true)} data-testid="find-info-online">
          <Search className="h-3.5 w-3.5" /> {t('game:info.find')}
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-3 border-t border-white/[0.06] pt-3">
      <MonoLabel className="mb-1.5">{t('game:info.heading')}</MonoLabel>
      <p className="mb-2 text-[12px] leading-relaxed text-white/55">
        {t('game:info.help', { title: game.title })}
      </p>
      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); search(); }}
      >
        <Input
          aria-label={t('game:info.searchAria')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={game.title}
          className="min-w-0 max-w-xs flex-1"
        />
        <Button type="submit" variant="neutral" size="sm" disabled={!query.trim() || searching || linking != null}>
          <Search className="h-3.5 w-3.5" /> {searching ? t('game:info.searching') : t('game:info.search')}
        </Button>
      </form>
      {candidates != null && candidates.length === 0 && (
        <p className="mt-2 text-[12px] text-white/55">{t('game:info.noneFound')}</p>
      )}
      {candidates != null && candidates.length > 0 && (
        <ul className="mt-2 space-y-2">
          {candidates.map((c) => (
            <li key={infoCandidateKey(c)} className="flex items-center gap-3 rounded-xl bg-white/[0.03] px-3 py-2.5 ring-1 ring-white/[0.06]">
              <div className="min-w-0 flex-1">
                <div className="truncate text-[12.5px] font-semibold text-white/90">{c.name}</div>
                <div className="font-mono text-hint text-white/55">
                  <span data-testid="candidate-source">{infoSourceText(t, c.source)}</span>
                  {c.year ? ` · ${c.year}` : ''} · {infoCandidateDetail(t, c)}
                </div>
              </div>
              <Button variant="primary" size="sm" disabled={searching || linking != null} onClick={() => link(c)}>
                {linking === infoCandidateKey(c) ? t('game:info.linking') : t('game:info.link')}
              </Button>
            </li>
          ))}
        </ul>
      )}
      {error && <ErrorNotice message={error} fallback={t('game:info.linkFailed')} />}
    </div>
  );
}

function InstallButton({
  game, job, onInstall, onInstallTo, onCancel,
}: { game: Game; job?: Job; onInstall: () => void; onInstallTo?: () => void; onCancel: (id: string) => void }) {
  const { t } = useTranslation('game');
  if (job && (job.state === 'queued' || job.state === 'running')) return <JobProgress job={job} onCancel={onCancel} />;
  if (!game.installable) {
    return (
      <span className="rounded-2xl bg-white/[0.08] px-6 py-3.5 text-[13px] font-bold text-white/55" title={game.note}>
        {t('game:install.cantInstall')}
      </span>
    );
  }
  return (
    <>
      {onInstallTo && (
        <Button variant="neutral" size="none" onClick={onInstallTo} className="gap-2 rounded-2xl bg-black/50 px-4 py-3.5 text-[13px] text-white/80 hover:bg-black/70">
          <FolderInput className="h-4 w-4" /> {t('game:install.installTo')}
        </Button>
      )}
      <Button
        variant="primary"
        size="none"
        onClick={onInstall}
        className="gap-2 rounded-2xl px-8 py-3.5 text-[15px] shadow-[0_12px_30px_rgba(252,211,77,.25)]"
      >
        <Download className="h-4 w-4" strokeWidth={3} />
        {job?.state === 'failed' ? t('game:install.retryInstall') : t('game:install.install')}
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
  const { t } = useTranslation('game');
  return (
    <p className="mb-1 font-mono text-[12px] text-white/75" data-testid="install-size-line">
      {size.state === 'loading' && <span className="text-white/55">{t('game:installSize.checking')}</span>}
      {size.state === 'unknown' && <span title={t('game:installSize.unknownTitle')}>{t('game:installSize.unknown')}</span>}
      {size.state === 'known' && (
        <>
          {t('game:installSize.known', { size: formatBytes(size.total) })}
          {size.added > 0 && <span className="text-white/55">{t('game:installSize.addons', { size: formatBytes(size.added) })}</span>}
        </>
      )}
    </p>
  );
}

function NoteLink({ onClick, children }: { onClick: () => void; children?: React.ReactNode }) {
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
  sourceLogos,
}: {
  game: Game;
  plugin: StorePlugin;
  job?: Job;
  onJobAction: (action: 'cancel' | 'retry', id: string) => void;
  sourceLogos?: Record<string, string>;
}) {
  const { t } = useTranslation('game');
  const active = job && (job.state === 'queued' || job.state === 'running');
  const storeLogo = sourceLogos?.[game.source] ?? sourceLogos?.[game.store];
  const storeName = storeLabel(game.store);
  const storeText = `${storeName} · ${plugin.name}`;
  const storeValue = (
    <span className="inline-flex items-center gap-1.5" data-testid="store-row">
      <PluginLogo src={storeLogo} name={storeName} size="xs" />
      <span>{storeText}</span>
    </span>
  );
  const rows: [string, React.ReactNode, string][] = [
    [t('game:installation.store'), storeValue, storeText],
    // A game the store lists as installed may have no folder Platypus can name: say so.
    [t('game:installation.folder'), game.install_dir || (game.is_installed ? t('game:installation.unknownFolder') : '—'), game.install_dir || ''],
    [t('game:installation.version'), game.version || '—', game.version || ''],
    ...(game.pinned ? [[t('game:installation.pinnedTo'), pinLabel(game.pinned), pinLabel(game.pinned)] as [string, React.ReactNode, string]] : []),
    [t('game:installation.size'), formatBytes(game.size_bytes) || '—', formatBytes(game.size_bytes) || ''],
  ];
  return (
    <Panel className="bg-white/[0.05]">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:installation.heading')}</h2>
      <div className="space-y-2 font-mono text-hint">
        {rows.map(([k, v, title]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-white/[0.05] pt-2">
            <span className="text-white/55">{k}</span>
            <span className="truncate text-right text-white/85 select-text" title={title}>{v}</span>
          </div>
        ))}
      </div>
      {active && <div className="mt-4"><JobProgress job={job} onCancel={(id) => onJobAction('cancel', id)} /></div>}
      {job?.state === 'failed' && !active && (
        <ErrorNotice className="mt-2" message={job.error ?? ''} fallback={t('game:installation.jobFailed', { kind: job.kind })} onRetry={() => onJobAction('retry', job.id)} />
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
  const { t } = useTranslation('game');
  const pct = known && ach.total > 0 ? Math.round((ach.unlocked / ach.total) * 100) : 0;
  // Backend may send list:null for uncached progress; normalized at the
  // edge in runtime.ts, guarded here too for safety (FE-10).
  const list = ach.list ?? [];
  return (
    <section className="pb-8">
      <div className="mb-3.5 flex items-baseline justify-between">
        <h2 className="text-[16px] font-bold tracking-tight">{t('game:achievements.heading')}</h2>
        <span className="font-mono text-[12px] tabular-nums text-white/55">
          {known ? t('game:achievements.countKnown', { unlocked: formatCount(ach.unlocked), total: formatCount(ach.total), percent: formatCount(pct) }) : t('game:achievements.countTotal', { total: formatCount(ach.total) })}
        </span>
      </div>
      {known ? (
        <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
          <div className="h-full rounded-full bg-amber-300" style={{ width: `${pct}%` }} />
        </div>
      ) : (
        <p className="mb-4 text-[12px] text-white/70">
          {viaSteam
            ? t('game:achievements.steamHint')
            : t('game:achievements.storeHint')}
        </p>
      )}
      <div className="ach-grid">
        {list.map((a) => (
          <AchievementCard key={a.id || a.name} a={a} known={known} />
        ))}
      </div>
      {list.length < ach.total && (
        <p className="mt-3 font-mono text-[12px] text-white/55">
          {t('game:achievements.showing', { shown: formatCount(list.length), total: formatCount(ach.total), reason: known ? t('game:achievements.reasonSample') : t('game:achievements.reasonHighlight') })}
        </p>
      )}
    </section>
  );
}

function AchievementCard({ a, known }: { a: Achievement; known: boolean }) {
  const { t } = useTranslation('game');
  const dim = known && !a.unlocked;
  const meta = a.unlocked && a.unlocked_at
    ? formatDate(a.unlocked_at)
    : a.global_percent
      ? t('game:achievements.ofPlayers', { share: formatPercentValue(a.global_percent) })
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
