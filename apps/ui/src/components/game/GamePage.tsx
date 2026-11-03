// i18n: converted
// The game page (issue #119): the page and the data it owns. Blocks and
// sections live next to it in ./blocks and ./sections; gameBlocks assembles
// them into the layout regions.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Effect } from 'effect';
import type { ActivityEvent, ArtCandidate, Collection, Game, GameDetails, GameSettings, GameJobKind, GameStatus, Job, ProtonVersion, RemoveOptions, SessionEvent, StorePlugin, Tweak, View } from '../../types';
import { CopyPicker } from '../CopyPicker';
import { Lightbox, toMediaItems, type MediaItem } from '../Lightbox';
import { InstallOptionsDialog } from '../InstallOptionsDialog';
import { InstallDialog } from '../InstallDialog';
import { GameSettingsModal } from '../GameSettingsModal';
import { deckLabel } from '../CompatBadges';
import { useCompat } from '../../lib/useCompat';
import { steamDeckFact } from '../../lib/compat';
import { useCrashHints } from '../../lib/useCrashHints';
import { CrashHintsPanel } from '../CrashHints';
import { TooltipProvider } from '@/components/ds/tooltip';
import { Hero } from '../Hero';
import { ProgressAchievements, useAchievementProgress } from '../ProgressAchievements';
import { heroImages, hoursPlayed, isPluginGame, storeLabel } from '../../lib/games';
import { useNetState } from '../useNetState';
import { formatHours } from '../../lib/format';
import { recencyOf, renderRecency } from '../../lib/gameLocale';
import { LauncherApi } from '../../effect/runtime';
import { useVariant, useThemeLayout } from '../../lib/theme';
import { ThemeLayout } from '../../lib/layout';
import { BUILTIN_LAYOUTS } from '../../gen/builtinLayouts';
import { GameTabsView } from '../GameTabs';
import type { ThemeLayout as GameLayoutData } from '../../types';
import { offers } from '../../lib/capabilities';
import { isCustomized } from '../../lib/customize';
import { useInstallSize, type InstallSize } from '../../lib/installSize';
import type { GameSettingsTab } from '../../lib/gameSettingsTabs';
import { GameActivity, GameBackLink, GameCompat, GameHeroActions, GameHeroHeading, GameInstallNote, GameOutsideAlert } from './blocks';
import { AboutSection, AchievementsSection, CollectionsSection, InstallationPanel, MediaStrip, StatusNotesSection } from './sections';

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
  installIntent?: boolean;
  onInstallIntentHandled?: () => void;
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
  onSaveSettings: (s: GameSettings) => Promise<void>; // i18n:allow: Promise return type
  /** The game's customization changed: the library should reload to carry it. */
  onCustomized?: () => void;
  plugin?: StorePlugin; // plugin games only
  job?: Job; // the game's active job, or its last failed one
  /** Plugin logos by store (and plugin id): the store badge and details row. */
  sourceLogos?: Record<string, string>;
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry' | 'pause' | 'resume', id: string) => void;
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
  /** Open the settings dialog on this tab (the command palette's entry, issue
   *  #124). Cleared through onSettingsOpened once the page has opened it. */
  openSettingsTab?: GameSettingsTab;
  onSettingsOpened?: () => void;
  /** When the session started (ms epoch), for the tabs strip elapsed time.
   *  Only the tabs variant reads it; sections keep today's PlayButton. */
  sessionStartedAt?: number;
  /** The list-detail stage shows no back link (it would go nowhere). */
  hideBack?: boolean;
  /** The hero art the page actually shows, for the game tone to sample. */
  onHeroArt?: (url: string | null) => void;
  /** A Tune flyout write the core confirmed, for App's copy of the game. */
  onTuneSaved?: (settings: GameSettings) => void;
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

/** One region's frame in a game layout (issue #103): the hero holds the top
 *  of the page full width; the main column breathes like the built-in page
 *  and the side column packs tighter; the bottom region holds the
 *  achievements and session activity full width below both columns, as the
 *  pre-layout page did. The built-in spacing is exact. */
function gameRegionClass(region: string): string {
  switch (region) {
    case 'hero': return 'min-w-0 space-y-0';
    case 'side': return 'min-w-0 space-y-5';
    case 'bottom': return 'min-w-0';
    default: return 'min-w-0 space-y-5';
  }
}

export function GamePage(p: GamePageProps) {
  const { t } = useTranslation('game');
  const { game, runners, activity, session, tweaks } = p;
  const sessionActive = session?.state === 'starting' || session?.state === 'running' || session?.state === 'stopping';
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
  const crash = useCrashHints(game, sessionActive || p.launching);
  // Offline (issue #44): a "network" game shows a needs-connection chip while
  // offline. Play stays enabled — only the store plugin can refuse the launch.
  const { online } = useNetState();
  const needsConnection = !online && (game as Game & { offline?: string }).offline === 'network';
  const progress = useAchievementProgress(game, !!p.plugin && offers(p.plugin, 'achievements', { game }));
  const hero = useHero(game, details);
  // The tone samples what the hero actually shows (the details hero once it
  // loads, else the library fallback), not heroImages(game)[0] blindly.
  const heroTop = hero[0] ?? null;
  useEffect(() => {
    p.onHeroArt?.(heroTop);
    // The callback is App's stable setter; the art is what matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [heroTop]);
  useEffect(() => { setShot(0); setLightbox(false); setSettingsTab(null); setInstallTo(false); setOptionsOpen(false); }, [game.id]);
  // The palette's "Open game settings" (issue #124): open the tab it asked for,
  // then clear the request so closing the dialog keeps it closed.
  useEffect(() => {
    if (!p.openSettingsTab) return;
    setSettingsTab(p.openSettingsTab);
    p.onSettingsOpened?.();
  }, [p.openSettingsTab, p.onSettingsOpened]);
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
  const events = activity.filter((e) => e.game === game.id).slice(0, 6);
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
    [t('game:facts.wrappers'), [s.use_gamemode && 'GameMode', mangohudOn && 'MangoHud', s.use_gamescope && 'Gamescope'].filter(Boolean).join(' · ') || t('game:facts.noWrappers')],
    [t('game:facts.launchVia'), game.client_launch ? (plugin?.name ?? storeLabel(game.store)) : viaSteam ? t('game:facts.steamClient') : game.native ? t('game:facts.nativeLaunch') : 'umu-run'],
    [
      t('game:facts.compatibility'),
      !info ? t('game:facts.pending') : ratingBadge ? `${ratingBadge.source} · ${ratingBadge.label}` : info.steam_app_id ? t('game:facts.protondbPending') : t('game:facts.unknown'),
    ],
    [
      t('game:facts.deck'),
      !info
        ? t('game:facts.pending')
        : deck
          ? deckLabel(t, deck.rating)
          : info.steam_app_id
            ? t('game:facts.deckPending')
            : t('game:facts.unknown'),
    ],
  ];
  const compat = fix?.description || (ratingBadge
    ? `${ratingBadge.source} · ${ratingBadge.label}`
    : t('game:compat.noneLoaded'));

  // The theme picks the game settings presentation (issue #99): a dialog or
  // sheet overlays the game, while a page replaces it, with its own back
  // entry to the game.
  const settingsPage = useVariant('game-settings.presentation') === 'page';
  // The theme picks the game structure (issue #127): sections is today's
  // page, tabs is the /21 hero with the action strip and sticky tabs. The
  // install flow belongs to the structure (issue #168), not the shell: tabs
  // review the location in the install dialog, sections install at once.
  const structure = useVariant('game.structure');
  const tabsInstall = structure === 'tabs';
  useEffect(() => {
    if (!p.installIntent) return;
    if (canChooseOptions) setOptionsOpen(true);
    else if (tabsInstall && plugin) setInstallTo(true);
    else p.onJob('install');
    p.onInstallIntentHandled?.();
  }, [p.installIntent, canChooseOptions, tabsInstall, plugin, p.onJob, p.onInstallIntentHandled]);
  const settingsModal = settingsTab ? (
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
      running={sessionActive || p.launching}
    />
  ) : null;
  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's game page before the core answers over RPC.
  // Above the settings-page return: hooks must run on every render.
  const layout = useThemeLayout('game') ?? BUILTIN_LAYOUTS['game'];
  if (settingsModal && settingsPage) {
    return (
      <TooltipProvider>
        {settingsModal}
      </TooltipProvider>
    );
  }

  // The page owns the data: the core calls live here, the blocks get
  // callbacks, so they are props only and can move in a layout.
  const onSearchInfo = (query: string) => Effect.runPromise(LauncherApi.searchArtOnline(query));
  const onLinkInfo = (id: string, source: string, infoId: string) =>
    Effect.runPromise(LauncherApi.linkGameInfo(id, source, infoId)).then(() => {});
  const onUnlinkInfo = (id: string) =>
    Effect.runPromise(LauncherApi.linkGameInfo(id, '', '')).then(() => {});
  const onSaveStatus = (id: string, status: GameStatus) =>
    Effect.runPromise(LauncherApi.setGameStatus(id, status));
  const onSaveNotes = (id: string, text: string) =>
    Effect.runPromise(LauncherApi.setGameNotes(id, text)).then(() => {});
  const onOpenStore = plugin?.storefront_url && details?.store_url
    ? () => p.onNavigate({ name: 'store', plugin: plugin.id, url: details.store_url })
    : undefined;

  // Tabs re-arrange the same blocks (issue #127): the page owns the data,
  // the tabs only lay it out, so required blocks stay present.
  if (structure === 'tabs') {
    return (
      <TooltipProvider>
        {/* Full-bleed like /21 `.stage`: no page padding around the hero and
            strip; the tabs view owns its own padding below them. */}
        <div className="relative isolate">
          {game.background_url && (
            <div
              aria-hidden
              data-testid="game-background"
              className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[620px] bg-cover bg-top opacity-[.2]"
              style={{ backgroundImage: `url("${game.background_url}")`, maskImage: 'linear-gradient(180deg, #000 30%, transparent)', WebkitMaskImage: 'linear-gradient(180deg, #000 30%, transparent)' }}
            />
          )}
          <GameTabsView
            game={game}
            hero={hero}
            plugin={plugin}
            runners={runners}
            activity={activity}
            backLabel={p.backLabel}
            onBack={p.onBack}
            onPlay={p.onPlay}
            onStop={p.onStop}
            onForceStop={p.onForceStop}
            launching={p.launching}
            session={session}
            sessionStartedAt={p.sessionStartedAt}
            refreshing={p.refreshing}
            job={job}
            sourceLogos={p.sourceLogos}
            onJob={p.onJob}
            onJobAction={p.onJobAction}
            onAllowFolder={p.onAllowFolder}
            onNavigate={p.onNavigate}
            onNotify={p.onNotify}
            onCopyChanged={p.onCopyChanged}
            installing={installing}
            installSize={installSize}
            canChooseOptions={canChooseOptions}
            canChooseVersion={canChooseVersion}
            onInstall={() => (canChooseOptions ? setOptionsOpen(true) : plugin ? setInstallTo(true) : p.onJob('install'))}
            onInstallTo={plugin && !canChooseOptions ? () => setInstallTo(true) : undefined}
            onOpenVersions={() => setSettingsTab('versions')}
            onOpenPlugins={() => p.onNavigate({ name: 'plugins' })}
            media={media}
            details={details}
            loading={loading}
            error={error}
            compatUrls={{ protondb: ratingBadge?.link, wiki: info?.wiki?.url }}
            onReload={reload}
            onSearchInfo={onSearchInfo}
            onLinkInfo={onLinkInfo}
            onUnlinkInfo={onUnlinkInfo}
            onOpenStore={onOpenStore}
            hours={hours}
            last={last}
            runsWith={facts[0]?.[1] ?? ''}
            protondb={ratingBadge?.label ?? ''}
            progress={progress}
            ach={ach}
            events={events}
            compatText={compat}
            facts={facts}
            onOpenRunner={() => setSettingsTab('general')}
            onOpenSettings={() => setSettingsTab('general')}
            onOpenFixes={() => setSettingsTab('tweaks')}
            tweaks={tweaks}
            onChangeSettings={p.onChangeSettings}
            hideBack={p.hideBack}
            onTuneSaved={p.onTuneSaved}
            collections={p.collections}
            onManageCollections={p.onManageCollections}
            onSaveStatus={onSaveStatus}
            onSaveNotes={onSaveNotes}
            onStatusChanged={() => p.onCustomized?.()}
          />
          {!settingsPage && settingsModal}
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
  const blocks = gameBlocks(
    {
      backLabel: p.backLabel,
      onBack: p.onBack,
      game,
      fix,
      storeLogo,
      storeName,
      needsConnection,
      plugin,
      job,
      active: !!active,
      session,
      launching: p.launching,
      hours,
      last,
      playtimeTitle,
      canChooseOptions,
      onJob: p.onJob,
      onJobAction: p.onJobAction,
      onPlay: p.onPlay,
      onStop: p.onStop,
      onForceStop: p.onForceStop,
      onInstall: () => (canChooseOptions ? setOptionsOpen(true) : p.onJob('install')),
      onInstallTo: plugin && !canChooseOptions ? () => setInstallTo(true) : undefined,
      onSettings: () => setSettingsTab('general'),
      onOpenStore,
      onAllowFolder: p.onAllowFolder,
      installing,
      installSize,
      canChooseVersion,
      refreshing: p.refreshing,
      onOpenVersions: () => setSettingsTab('versions'),
      onOpenPlugins: () => p.onNavigate({ name: 'plugins' }),
      media,
      shot,
      onShot: setShot,
      onOpenLightbox: () => setLightbox(true),
      strip,
      details,
      loading,
      error,
      compatUrls: { protondb: ratingBadge?.link, wiki: info?.wiki?.url },
      onReload: reload,
      onSearchInfo,
      onLinkInfo,
      onUnlinkInfo,
      compat,
      facts,
      info,
      crash,
      onNotify: p.onNotify,
      onOpenRunner: () => setSettingsTab('general'),
      onCustomized: p.onCustomized,
      onSaveStatus,
      onSaveNotes,
      collections: p.collections,
      onManageCollections: p.onManageCollections,
      sourceLogos: p.sourceLogos,
      onCopyChanged: p.onCopyChanged,
      progress,
      ach,
      events,
    },
    layout,
  );
  const heroRegion = layout.regions.find((r) => r.name === 'hero');
  const bottomRegion = layout.regions.find((r) => r.name === 'bottom');
  // Main and side stay today's two columns; bottom holds the achievements
  // and session activity full width below them, as the pre-layout page did.
  const gridLayout: GameLayoutData = {
    view: layout.view,
    regions: layout.regions.filter((r) => r.name === 'main' || r.name === 'side'),
  };
  const bottomLayout: GameLayoutData | null = bottomRegion
    ? { view: layout.view, regions: [bottomRegion] }
    : null;

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
      {heroRegion && (
        <GameHeroRegion region={heroRegion} blocks={blocks} art={hero} title={game.title} />
      )}
      <ThemeLayout
        layout={gridLayout}
        blocks={blocks}
        className="mb-6 grid gap-5 lg:grid-cols-[1.6fr_1fr]"
        regionClassName={gameRegionClass}
      />
      {bottomLayout && (
        <ThemeLayout
          layout={bottomLayout}
          blocks={blocks}
          regionClassName={gameRegionClass}
        />
      )}
      {lightbox && media.length > 0 && <Lightbox items={media} index={shot} alt={game.title} onIndex={setShot} onClose={() => setLightbox(false)} />}
      {!settingsPage && settingsModal}
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

/** The hero region's frame in a game layout (issue #103): the back link above
 *  the hero, the outside-folder alert and install note below it, and the
 *  heading and actions inside the Hero frame. The view wraps the hero
 *  region's heading and actions in the Hero, so a layout can move the actions
 *  out of the hero (then the hero shows the heading alone, and the actions
 *  render where the layout puts them). Order follows the layout: the hero
 *  frame sits where the first of heading/actions appears. */
function GameHeroRegion({ region, blocks, art, title }: {
  region: GameLayoutData['regions'][number];
  blocks: Record<string, React.ReactNode>;
  art: string[];
  title: string;
}) {
  const nodes = region.nodes.flatMap((n) => (n.row ? n.row : [n]));
  const names = nodes.map((n) => n.block ?? '');
  const hasHeading = names.includes('heading');
  const hasActions = names.includes('actions');
  const heroAt = names.findIndex((n) => n === 'heading' || n === 'actions');
  const out: React.ReactNode[] = [];
  let heroEmitted = false;
  const emitHero = (key: string) => {
    if (heroEmitted) return;
    heroEmitted = true;
    if (!hasHeading && !hasActions) return;
    out.push(
      <Hero key={key} site="game" art={art} title={title}>
        {hasHeading ? blocks['heading'] : null}
        {hasActions ? blocks['actions'] : null}
      </Hero>,
    );
  };
  nodes.forEach((n, i) => {
    const name = n.block ?? '';
    if (name === 'heading' || name === 'actions') {
      if (i === heroAt) emitHero(`hero-${i}`);
      return;
    }
    if (i === heroAt) emitHero(`hero-${i}`);
    if (name) out.push(<React.Fragment key={`${name}-${i}`}>{blocks[name]}</React.Fragment>);
  });
  if (!heroEmitted && (hasHeading || hasActions)) emitHero('hero-last');
  // A hero region with no heading or actions still shows the back link and
  // the alerts: the required blocks a layout left out come back here.
  return <>{out}</>;
}

interface GameData {
  backLabel: string;
  onBack: () => void;
  game: Game;
  fix: Game['fix_info'];
  storeLogo?: string;
  storeName: string;
  needsConnection: boolean;
  plugin?: StorePlugin;
  job?: Job;
  active: boolean;
  session?: SessionEvent;
  launching: boolean;
  hours: number | null;
  last?: string;
  playtimeTitle?: string;
  canChooseOptions: boolean;
  onJob: (kind: GameJobKind) => void;
  onJobAction: (action: 'cancel' | 'retry' | 'pause' | 'resume', id: string) => void;
  onPlay: () => void;
  onStop: () => void;
  onForceStop: () => void;
  onInstall: () => void;
  onInstallTo?: () => void;
  onSettings: () => void;
  onOpenStore?: () => void;
  onAllowFolder: (folder: string) => void;
  installing: boolean;
  installSize: InstallSize;
  canChooseVersion: boolean;
  refreshing?: boolean;
  onOpenVersions: () => void;
  onOpenPlugins: () => void;
  media: MediaItem[];
  shot: number;
  onShot: (i: number) => void;
  onOpenLightbox: () => void;
  strip: React.RefObject<HTMLDivElement | null>;
  details: GameDetails | null;
  loading: boolean;
  error: string | null;
  compatUrls: { protondb?: string; wiki?: string };
  onReload: () => void;
  onSearchInfo: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLinkInfo: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  onUnlinkInfo: (id: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  compat: string;
  facts: [string, string][];
  info: ReturnType<typeof useCompat>['compat'];
  crash: ReturnType<typeof useCrashHints>;
  onNotify?: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
  onOpenRunner: () => void;
  onCustomized?: () => void;
  onSaveStatus: (id: string, status: GameStatus) => Promise<{ status: GameStatus }>; // i18n:allow: Promise return type, not user-facing text
  onSaveNotes: (id: string, text: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  collections?: Collection[];
  onManageCollections?: (gameId: string) => void;
  sourceLogos?: Record<string, string>;
  onCopyChanged?: () => void;
  progress: ReturnType<typeof useAchievementProgress>;
  ach: GameDetails['achievements'];
  events: ActivityEvent[];
}

// The game page's layout blocks (issue #103): each takes its data as props
// only (data hooks stay in the page above) and renders a single root or null
// when it has nothing to show, so a layout orders blocks without conditionals
// of its own. The hero region's heading and actions render inside the Hero
// frame (see GameHeroRegion); every other block carries its own spacing, as
// the built-in page does.
function gameBlocks(d: GameData, _layout?: GameLayoutData): Record<string, React.ReactNode> {
  const { game } = d;
  return {
    back: <GameBackLink backLabel={d.backLabel} onBack={d.onBack} />,
    heading: (
      <GameHeroHeading game={game} fix={d.fix} storeLogo={d.storeLogo} storeName={d.storeName} needsConnection={d.needsConnection} />
    ),
    actions: (
      <GameHeroActions
        game={game}
        plugin={d.plugin}
        job={d.job}
        active={d.active}
        session={d.session}
        launching={d.launching}
        hours={d.hours}
        last={d.last}
        playtimeTitle={d.playtimeTitle}
        canChooseOptions={d.canChooseOptions}
        onJob={d.onJob}
        onJobAction={d.onJobAction}
        onPlay={d.onPlay}
        onStop={d.onStop}
        onForceStop={d.onForceStop}
        onInstall={d.onInstall}
        onInstallTo={d.onInstallTo}
        onSettings={d.onSettings}
        onOpenStore={d.onOpenStore}
      />
    ),
    outside: d.plugin ? (
      <GameOutsideAlert game={game} plugin={d.plugin} onAllowFolder={d.onAllowFolder} />
    ) : null,
    note: (
      <GameInstallNote
        game={game}
        plugin={d.plugin}
        installing={d.installing}
        installSize={d.installSize}
        job={d.job}
        canChooseVersion={d.canChooseVersion}
        refreshing={d.refreshing}
        onOpenVersions={d.onOpenVersions}
        onOpenPlugins={d.onOpenPlugins}
      />
    ),
    media: d.media.length > 0 ? (
      <MediaStrip game={game} media={d.media} index={d.shot} onIndex={d.onShot} onOpen={d.onOpenLightbox} stripRef={d.strip} />
    ) : null,
    about: (
      <AboutSection
        game={game}
        details={d.details}
        loading={d.loading}
        error={d.error}
        compatUrls={d.compatUrls}
        onOpenStore={d.onOpenStore}
        onReload={d.onReload}
        onSearchInfo={d.onSearchInfo}
        onLinkInfo={d.onLinkInfo}
        onUnlinkInfo={d.onUnlinkInfo}
      />
    ),
    compat: <GameCompat compat={d.compat} fix={d.fix} facts={d.facts} info={d.info} />,
    crash: d.crash ? (
      <CrashHintsPanel game={game} hints={d.crash} onNotify={d.onNotify} onRunner={d.onOpenRunner} />
    ) : null,
    status: (
      <StatusNotesSection
        game={game}
        onChanged={d.onCustomized}
        onSaveStatus={d.onSaveStatus}
        onSaveNotes={d.onSaveNotes}
      />
    ),
    collections: (
      <CollectionsSection game={game} collections={d.collections} onManage={d.onManageCollections} />
    ),
    installation: isPluginGame(game) && game.is_installed && d.plugin ? (
      <InstallationPanel game={game} plugin={d.plugin} job={d.job} onJobAction={d.onJobAction} sourceLogos={d.sourceLogos} />
    ) : null,
    copies: isPluginGame(game) && d.plugin ? (
      <CopyPicker game={game} plugin={d.plugin} onChanged={() => d.onCopyChanged?.()} />
    ) : null,
    'achievements-progress': d.progress ? <ProgressAchievements progress={d.progress} /> : null,
    achievements:
      !d.progress && d.ach && d.ach.total > 0 ? (
        <AchievementsSection ach={d.ach} known={d.ach.unlocked >= 0} viaSteam={game.source === 'steam'} />
      ) : null,
    activity: <GameActivity events={d.events} />,
  };
}
