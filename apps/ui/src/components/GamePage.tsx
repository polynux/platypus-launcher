// i18n: converted
import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { useTranslation, Trans } from 'react-i18next';
import { Effect } from 'effect';
import { ArrowLeft, AlertTriangle, Download, FolderCheck, FolderInput, Maximize2, Play, RefreshCw, Search, Settings2, Store as StoreIcon, Trophy } from 'lucide-react';
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
import { MonoLabel, Panel, TierBadge, cx } from './ui';
import { Hero, heroChipClass } from './Hero';
import { PluginLogo } from './PluginLogo';
import { formatBytes, heroImages, hoursPlayed, isPluginGame, needsAttention, storeLabel } from '../lib/games';
import { useNetState } from './useNetState';
import { formatCount, formatDate, formatHours, formatPercentValue } from '../lib/format';
import { formatDecimal1, recencyOf, renderRecency } from '../lib/gameLocale';
import { LauncherApi } from '../effect/runtime';
import { useVariant, useThemeLayout } from '../lib/theme';
import { ThemeLayout } from '../lib/layout';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';
import { GameTabsView } from './GameTabs';
import type { ThemeLayout as GameLayoutData } from '../types';
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

/** The back link (catalog game.back): where the game was opened from. Required. */
export function GameBackLink({ backLabel, onBack }: { backLabel: string; onBack: () => void }) {
  return (
    <Button variant="ghost" size="none" onClick={onBack} className="gap-1.5 rounded px-0 text-[12px] font-semibold text-text-muted hover:bg-transparent hover:text-white">
      <ArrowLeft className="h-3.5 w-3.5" /> {backLabel}
    </Button>
  );
}

/** The outside-folder alert (catalog game.outside): a plugin game installed
 *  outside its folder. Required: it tells the truth about the game's state.
 *  In the tabs structure (atlas) it is a slim full-width warning row under
 *  the strip: danger tint with a 2px left inset bar, never a card. */
export function GameOutsideAlert({ game, plugin, onAllowFolder }: {
  game: Game;
  plugin: StorePlugin;
  onAllowFolder: (folder: string) => void;
}) {
  const { t } = useTranslation('game');
  const tabs = useVariant('game.structure') === 'tabs';
  if (!game.is_installed || !game.outside_folder) return null;
  if (tabs) {
    return (
      <div role="alert" className="flex items-center gap-3 bg-danger/10 px-6 py-2.5 text-[13px] shadow-[inset_2px_0_0_var(--color-danger)]">
        <AlertTriangle className="h-4 w-4 flex-none text-danger" aria-hidden />
        <p className="min-w-0 flex-1 leading-relaxed text-text">
          <span className="font-bold">{t('game:outside.title', { name: plugin.name })} </span>
          <Trans
            i18nKey="game:outside.body"
            values={{ name: plugin.name, folder: plugin.install_folder, dir: game.install_dir }}
            components={{
              folder: <span className="font-mono text-[12px]" />,
              dir: <span className="font-mono text-[12px] text-text" />,
            }}
          />
        </p>
        <Button onClick={() => onAllowFolder(game.install_dir)} className="flex-none">
          <FolderCheck className="h-4 w-4" /> {t('game:outside.allow')}
        </Button>
      </div>
    );
  }
  return (
    <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-accent-strong/[0.07] px-5 py-4 ring-1 ring-accent-strong/25">
      <div className="min-w-0 text-[12.5px] leading-relaxed text-white/75">
        <p className="font-bold text-amber-200">{t('game:outside.title', { name: plugin.name })}</p>
        <p>
          <Trans
            i18nKey="game:outside.body"
            values={{ name: plugin.name, folder: plugin.install_folder, dir: game.install_dir }}
            components={{
              folder: <span className="font-mono text-[12px]" />,
              dir: <span className="font-mono text-[12px] text-text" />,
            }}
          />
        </p>
      </div>
      <Button onClick={() => onAllowFolder(game.install_dir)} className="flex-none">
        <FolderCheck className="h-4 w-4" /> {t('game:outside.allow')}
      </Button>
    </div>
  );
}

/** The install note (catalog game.note): the sideload note, the fresh-install
 *  size and folder, the failed attempt, and the refreshing line. All small
 *  full-width notices live here, so a layout moves them as one. */
export function GameInstallNote({ game, plugin, installing, installSize, job, canChooseVersion, refreshing, onOpenVersions, onOpenPlugins }: {
  game: Game;
  plugin?: StorePlugin;
  installing: boolean;
  installSize: InstallSize;
  job?: Job;
  canChooseVersion: boolean;
  refreshing?: boolean;
  onOpenVersions: () => void;
  onOpenPlugins: () => void;
}) {
  const { t } = useTranslation('game');
  const active = job && (job.state === 'queued' || job.state === 'running');
  const showNote = !active && (game.note || installing);
  const tabs = useVariant('game.structure') === 'tabs';
  const atlas = useVariant('shell.navigation') === 'sourcebar';
  if (tabs) {
    // Atlas (/21 has no row here): the note lives inline at the end of the
    // action strip (muted 12.5px, folder links in the accent), never as a
    // 41px row between the strip and the tabs. The strip owns it.
    return null;
  }
  if (!showNote && !refreshing) return null;
  if (showNote && !refreshing) {
    return (
      <div className="mb-6 rounded-2xl bg-surface px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-hairline">
        {game.note && <p>{game.note}</p>}
        {installing && plugin && (
          <>
            <InstallSizeLine size={installSize} />
            <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[12px] text-text-muted">
              {canChooseVersion ? (
                <Trans
                  i18nKey="game:install.noteWithVersion"
                  values={{ folder: plugin.install_folder }}
                  components={{
                    v: <NoteLink onClick={onOpenVersions} />,
                    d: <NoteLink onClick={onOpenPlugins} />,
                  }}
                />
              ) : (
                <Trans
                  i18nKey="game:install.notePlain"
                  values={{ folder: plugin.install_folder }}
                  components={{ d: <NoteLink onClick={onOpenPlugins} /> }}
                />
              )}
            </p>
          </>
        )}
        {!atlas && !game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback={t('game:install.lastAttempt')} />}
      </div>
    );
  }
  // Both present (a rescan landing while the note shows): one root for the
  // row count, the same siblings as today inside it.
  return (
    <div>
      {showNote && (
        <div className="mb-6 rounded-2xl bg-surface px-5 py-3.5 text-[12.5px] leading-relaxed text-white/65 ring-1 ring-hairline">
          {game.note && <p>{game.note}</p>}
          {installing && plugin && (
            <>
              <InstallSizeLine size={installSize} />
              <p className="flex flex-wrap items-center gap-x-1.5 font-mono text-[12px] text-text-muted">
                {canChooseVersion ? (
                  <Trans
                    i18nKey="game:install.noteWithVersion"
                    values={{ folder: plugin.install_folder }}
                    components={{
                      v: <NoteLink onClick={onOpenVersions} />,
                      d: <NoteLink onClick={onOpenPlugins} />,
                    }}
                  />
                ) : (
                  <Trans
                    i18nKey="game:install.notePlain"
                    values={{ folder: plugin.install_folder }}
                    components={{ d: <NoteLink onClick={onOpenPlugins} /> }}
                  />
                )}
              </p>
            </>
          )}
          {!atlas && !game.is_installed && job?.state === 'failed' && <ErrorNotice className="mt-1" message={job.error ?? ''} fallback={t('game:install.lastAttempt')} />}
        </div>
      )}
      {refreshing && (
        <p className="mb-4 font-mono text-micro uppercase tracking-widest text-text-muted">{t('game:refreshing')}</p>
      )}
    </div>
  );
}

/** Compatibility facts (catalog game.compat): the curated recipe or rating,
 *  the runner facts, and the compat badges. Props only (facts come from the
 *  page's useCompat hook). */
export function GameCompat({ compat, fix, facts, info }: {
  compat: string;
  fix: Game['fix_info'];
  facts: [string, string][];
  info: ReturnType<typeof useCompat>['compat'];
}) {
  const { t } = useTranslation('game');
  return (
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:compat.heading')}</h2>
      <p className="text-[13px] leading-relaxed text-white/65">{compat}</p>
      {fix?.winetricks && fix.winetricks.length > 0 && (
        <p className="mt-2 font-mono text-hint text-sky-200/80">{t('game:compat.recipe', { mods: fix.winetricks.join(' ') })}</p>
      )}
      <div className="mt-4 space-y-2 font-mono text-hint">
        {facts.map(([k, v]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-hairline pt-2">
            <span className="text-text-muted">{k}</span>
            <span className="truncate text-right text-white/85">{v}</span>
          </div>
        ))}
      </div>
      {info && <CompatFacts compat={info} />}
    </Panel>
  );
}

/** Session activity for one game (catalog game.activity): the last events
 *  with this game's title. Props only. */
export function GameActivity({ events }: { events: ActivityEvent[] }) {
  const { t } = useTranslation('game');
  if (events.length === 0) return null;
  return (
    <section className="pb-10">
      <MonoLabel className="mb-2">{t('game:session')}</MonoLabel>
      <ActivityFeed events={events} />
    </section>
  );
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
  // page, tabs is the /21 hero with the action strip and sticky tabs.
  const structure = useVariant('game.structure');
  const atlasShell = useVariant('shell.navigation') === 'sourcebar';
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

  // The built-in arrangement is the initial layout, so the first paint
  // already shows today's game page before the core answers over RPC.
  const layout = useThemeLayout('game') ?? BUILTIN_LAYOUTS['game'];
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
            onInstall={() => (canChooseOptions ? setOptionsOpen(true) : atlasShell && plugin ? setInstallTo(true) : p.onJob('install'))}
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
      onInstall: () => (canChooseOptions ? setOptionsOpen(true) : atlasShell && plugin ? setInstallTo(true) : p.onJob('install')),
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

/** The store's HTML (description, requirements): core sanitizes it to the
 *  allowlist, so rendering it as markup is safe. Links open in the browser. */
export function SafeHtml({ html, testid, className }: { html: string; testid?: string; className?: string }) {
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
      className={cx('space-y-2 text-[13px] leading-relaxed text-text-body [&_a]:text-amber-200/90 [&_a]:underline [&_a]:decoration-amber-200/40 [&_a]:underline-offset-2 [&_h2]:text-[14px] [&_h2]:font-bold [&_h2]:text-white/85 [&_h3]:text-[13px] [&_h3]:font-bold [&_h3]:text-white/85 [&_h4]:text-[13px] [&_h4]:font-bold [&_h4]:text-white/85 [&_li]:ml-4 [&_li]:list-disc [&_ul]:space-y-1', className)}
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
        className="group relative block w-full cursor-zoom-in overflow-hidden rounded-3xl bg-black/40 ring-1 ring-hairline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"
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
              className={cx('relative aspect-video h-[84px] flex-none overflow-hidden rounded-xl bg-black/50 transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus', i === index ? 'ring-2 ring-accent-strong/80' : 'opacity-55 ring-1 ring-hairline hover:opacity-90')}
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
export function AboutSection({ game, details, loading, error, compatUrls, onOpenStore, onReload, onSearchInfo, onLinkInfo, onUnlinkInfo, hideDetails }: {
  game: Game;
  details: GameDetails | null;
  loading: boolean;
  error: string | null;
  compatUrls: { protondb?: string; wiki?: string };
  /** Opens the store page in the embedded store view, when the plugin has one. */
  onOpenStore?: () => void;
  /** The info link changed: details were evicted and refetched. */
  onReload?: () => void;
  onSearchInfo: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLinkInfo: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  onUnlinkInfo: (id: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
  /** The tabs store page shows Details in its side column: hide the grid here. */
  hideDetails?: boolean;
}) {
  const { t } = useTranslation('game');
  const [expanded, setExpanded] = useState(false);
  const [reqTab, setReqTab] = useState<'minimum' | 'recommended'>('minimum');
  useEffect(() => { setExpanded(false); setReqTab('minimum'); }, [game.id]);
  // The tabs clamp (760px) applies only when the description actually
  // overflows it: a short store description shows with no mask and no
  // "Read the full description" toggle. Measured off scrollHeight (the full
  // content height even when clamped).
  const wrapRef = useRef<HTMLDivElement>(null);
  const [overflows, setOverflows] = useState(false);
  useEffect(() => { setOverflows(false); }, [game.id, details]);
  useLayoutEffect(() => {
    if (!hideDetails) return;
    const el = wrapRef.current;
    if (!el) return;
    const check = () => {
      if (expanded) return;
      setOverflows(el.scrollHeight > 761);
    };
    check();
    window.addEventListener('resize', check);
    let ro: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      ro = new ResizeObserver(check);
      ro.observe(el);
    }
    return () => {
      window.removeEventListener('resize', check);
      ro?.disconnect();
    };
  }, [hideDetails, details, expanded]);
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
  // The tabs store page (atlas) shows the description as plain text: 14px,
  // max 72ch, 1.62 line height, with no card and no About heading. An
  // untitled game with no description shows one muted line.
  if (hideDetails) {
    return (
      <div className="max-w-[72ch] text-[14px] leading-[1.62] text-text-body">
        {loading ? (
          <div className="space-y-2">
            <div className="h-3 w-full animate-pulse rounded bg-surface-strong" />
            <div className="h-3 w-4/5 animate-pulse rounded bg-surface-strong" />
          </div>
        ) : html ? (
          <>
            <div ref={wrapRef} className={expanded || !overflows ? '' : 'max-h-[760px] overflow-hidden [mask-image:linear-gradient(#000_70%,transparent)]'} data-testid="about-html-wrap">
              <SafeHtml html={html} testid="about-html" className="text-[14px] leading-[1.62] !space-y-3.5 [&_h2]:!text-[15px] [&_h2]:!font-[750] [&_h2]:!mt-2.5 [&_h3]:!text-[15px] [&_h3]:!font-[750] [&_h3]:!mt-2.5 [&_h4]:text-[13px] [&_li]:text-[14px] [&_li]:leading-[1.62] [&_li::marker]:!text-accent [&_p]:text-[14px] [&_p]:leading-[1.62] [&_ul]:text-[14px] [&_a]:!text-accent" />
            </div>
            {overflows && (
              <Button variant="ghost" size="none" onClick={() => setExpanded((v) => !v)} data-testid="about-toggle" className="mt-2.5 rounded px-0 font-sans text-[14px] font-semibold text-accent hover:bg-transparent hover:text-text hover:underline hover:underline-offset-[3px]">
                {expanded ? t('game:about.showLess') : t('game:about.readFull')}
              </Button>
            )}
          </>
        ) : summary ? (
          <p className="leading-[1.62]">{summary}</p>
        ) : (
          <p className="text-text-muted">{error ? t('game:about.storeUnavailable', { error }) : t('game:about.noDescription')}</p>
        )}
        {isCustom && !linked && !loading && (
          <InfoFinder game={game} onLinked={() => onReload?.()} onSearch={onSearchInfo} onLink={onLinkInfo} />
        )}
      </div>
    );
  }
  return (
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:about.heading')}</h2>
      {loading ? (
        <div className="space-y-2">
          <div className="h-3 w-full animate-pulse rounded bg-surface-strong" />
          <div className="h-3 w-4/5 animate-pulse rounded bg-surface-strong" />
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
        <p className="text-[13px] leading-relaxed text-text-body">{summary}</p>
      ) : (
        <p className="text-[13px] text-text-body">{error ? t('game:about.storeUnavailable', { error }) : t('game:about.noDescription')}</p>
      )}
      {isCustom && !linked && !loading && (
        <InfoFinder game={game} onLinked={() => onReload?.()} onSearch={onSearchInfo} onLink={onLinkInfo} />
      )}
      {!hideDetails && rows.length > 0 && (
        <dl data-testid="details-grid" className="mt-4 grid grid-cols-2 gap-x-6 gap-y-2 border-t border-hairline pt-3">
          {rows.map(([k, v]) => (
            <div key={k} className="min-w-0">
              <dt className="font-mono text-hint uppercase tracking-widest text-white/45">{k}</dt>
              <dd className="truncate text-[12.5px] text-white/80" title={v}>{v}</dd>
            </div>
          ))}
        </dl>
      )}
      {req && (req.minimum || req.recommended) && (
        <div className="mt-4 border-t border-hairline pt-3" data-testid="requirements">
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
                className={reqTab === k ? 'rounded-lg bg-white/[0.08] px-2.5 py-1 font-mono text-[12px] text-white' : 'rounded-lg px-2.5 py-1 font-mono text-[12px] text-text-muted hover:bg-transparent hover:text-white/80'}
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
        <div className="mt-4 flex flex-wrap gap-x-4 gap-y-1.5 border-t border-hairline pt-3 font-mono text-[12px]" data-testid="about-links">
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
        <div className="mt-4 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-hairline pt-3 font-mono text-[12px] text-text-muted" data-testid="info-attribution">
          <span>
            {t('game:about.infoFrom')} {d?.store_url ? (
              <ExternalLink href={d.store_url} className="text-amber-200/80 underline decoration-white/25 underline-offset-2 hover:text-amber-200">
                {infoName} ↗
              </ExternalLink>
            ) : infoName}
          </span>
          <InfoUnlink game={game} onUnlinked={() => onReload?.()} onUnlink={onUnlinkInfo} />
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
 *  write at a time so a slow save can't overwrite a newer one. The page owns
 *  the data: saves go through the callbacks it passes (the core calls live
 *  there), so this block is props only and can move in a layout. */
function StatusNotesSection({ game, onChanged, onSaveStatus, onSaveNotes }: {
  game: Game;
  onChanged?: () => void;
  /** Saves the status through the page (the core call lives there). */
  onSaveStatus: (id: string, status: GameStatus) => Promise<{ status: GameStatus }>; // i18n:allow: Promise return type, not user-facing text
  /** Saves the notes through the page (the core call lives there). */
  onSaveNotes: (id: string, text: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
}) {
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
  const chain = useRef<Promise<void>>(Promise.resolve()); // i18n:allow: Promise return type, not user-facing text
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
      onSaveNotes(id, text).then(
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
    onSaveStatus(at, next).then(
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
    <Panel className="bg-surface-strong" data-testid="status-notes">
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
    <Panel className="bg-surface-strong" data-testid="game-collections">
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

/** The Unlink control next to a linked game's attribution. The page owns the
 *  data: the unlink goes through the callback it passes, so this block is
 *  props only and can move in a layout. */
function InfoUnlink({ game, onUnlinked, onUnlink }: { game: Game; onUnlinked: () => void; onUnlink: (id: string) => Promise<void> }) { // i18n:allow: Promise return type, not user-facing text
  const { t } = useTranslation('game');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const unlink = () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    onUnlink(game.id).then(
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
 *  year), and a pick links via LinkGameInfo. The page owns the data: search
 *  and link go through the callbacks it passes, so this block is props only
 *  and can move in a layout. */
function InfoFinder({ game, onLinked, onSearch, onLink }: {
  game: Game;
  onLinked: () => void;
  onSearch: (query: string) => Promise<ArtCandidate[]>; // i18n:allow: Promise return type, not user-facing text
  onLink: (id: string, source: string, infoId: string) => Promise<void>; // i18n:allow: Promise return type, not user-facing text
}) {
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
    onSearch(q).then(
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
    onLink(game.id, source, id).then(
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
    <div className="mt-3 border-t border-hairline pt-3">
      <MonoLabel className="mb-1.5">{t('game:info.heading')}</MonoLabel>
      <p className="mb-2 text-[12px] leading-relaxed text-text-muted">
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
        <p className="mt-2 text-[12px] text-text-muted">{t('game:info.noneFound')}</p>
      )}
      {candidates != null && candidates.length > 0 && (
        <ul className="mt-2 space-y-2">
          {candidates.map((c) => (
            <li key={infoCandidateKey(c)} className="flex items-center gap-3 rounded-xl bg-surface px-3 py-2.5 ring-1 ring-hairline">
              <div className="min-w-0 flex-1">
                <div className="truncate text-[12.5px] font-semibold text-text">{c.name}</div>
                <div className="font-mono text-hint text-text-muted">
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
      <span className="rounded-2xl bg-white/[0.08] px-6 py-3.5 text-[13px] font-bold text-text-muted" title={game.note}>
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

/** Today's game page hero heading (catalog game.heading): the badges and the
 *  title. The game page shows it inside the Hero frame; #103 will place it in
 *  a layout. Positioning is structural (as today): the frame lays the heading
 *  and the actions out. */
export function GameHeroHeading({ game, fix, storeLogo, storeName, needsConnection }: {
  game: Game;
  fix: Game['fix_info'];
  storeLogo?: string;
  storeName: string;
  needsConnection: boolean;
}) {
  const { t } = useTranslation('game');
  const chips = useVariant('game-hero.chips');
  return (
    <div className="min-w-0 flex-1 basis-72">
      <div className="mb-2.5 flex flex-wrap gap-2">
        {fix?.tier && <TierBadge tier={fix.tier} suffix={t('game:tierBadge.curated')} title={tierMeaning(t, fix.tier)} />}
        <Badge className={cx(heroChipClass(chips), 'text-white/75')} data-testid="store-badge">
          <span className="inline-flex items-center gap-1.5">
            <PluginLogo src={storeLogo} name={storeName} size="xs" />
            <span>{storeName}</span>
          </span>
        </Badge>
        {formatBytes(game.size_bytes) && <Badge className={cx(heroChipClass(chips), 'text-white/75')}>{formatBytes(game.size_bytes)}</Badge>}
        {!game.is_installed && <Badge className={cx(heroChipClass(chips), 'text-text-muted')}>{t('game:hero.notInstalled')}</Badge>}
        {game.update_available && <Badge variant="info" className="bg-sky-400/15 ring-1 ring-info/30">{t('game:hero.updateAvailable')}</Badge>}
        {game.pinned && <Badge variant="accent" className="bg-accent-strong/15 ring-1 ring-accent-strong/30">{t('game:hero.pinned')}</Badge>}
        {needsConnection && (
          <Badge className="bg-accent-strong/15 text-amber-100 ring-1 ring-accent-strong/30" data-testid="needs-connection">
            {t('game:hero.needsConnection', { store: storeName })}
          </Badge>
        )}
      </div>
      <GameHeading game={game} />
    </div>
  );
}

/** Today's game page hero actions (catalog game.actions): playtime, Install /
 *  Play / Update, game settings and the store link. Required (it holds
 *  Install, Play and Update); the game page shows it inside the Hero frame
 *  until #103 places it in a layout. */
export function GameHeroActions({ game, plugin, job, active, session, launching, hours, last, playtimeTitle, canChooseOptions, onJob, onJobAction, onPlay, onStop, onForceStop, onInstall, onInstallTo, onSettings, onOpenStore }: {
  game: Game;
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
}) {
  const { t } = useTranslation('game');
  const chips = useVariant('game-hero.chips');
  return (
    <div className="flex flex-wrap items-center justify-end gap-x-4 gap-y-3">
      {(hours != null || last) && (
        <div className="text-right font-mono text-hint leading-relaxed text-text-muted">
          {hours != null && <div title={playtimeTitle}>{t('game:hero.hoursLogged', { hours: formatDecimal1(hours) })}</div>}
          {last && <div>{t('game:hero.lastPlayed', { when: last })}</div>}
        </div>
      )}
      <div className="flex flex-wrap items-center justify-end gap-2.5">
        {game.is_installed && plugin && game.update_available && !active && (
          <Button variant="neutral" size="none" onClick={() => onJob('update')} className={cx(heroChipClass(chips), 'gap-2 rounded-2xl px-4 py-3.5 text-[13px] text-sky-100 hover:bg-black/70')}>
            <RefreshCw className="h-4 w-4" /> {t('game:hero.update')}
          </Button>
        )}
        {!game.is_installed ? (
          <InstallButton
            game={game}
            job={job}
            onInstall={onInstall}
            onInstallTo={onInstallTo}
            onCancel={(id) => onJobAction('cancel', id)}
          />
        ) : (
          <PlayButton session={session} launching={launching} disabled={game.outside_folder} onPlay={onPlay} onStop={onStop} onForceStop={onForceStop} />
        )}
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="neutral" size="none" onClick={onSettings} aria-label={t('game:hero.gameSettings')} className={cx(heroChipClass(chips), 'size-[50px] rounded-2xl text-white/80 hover:bg-black/70')}>
              <Settings2 className="h-[18px] w-[18px]" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('game:hero.gameSettings')}</TooltipContent>
        </Tooltip>
        {plugin?.storefront_url && onOpenStore && (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="neutral"
                size="none"
                onClick={onOpenStore}
                aria-label={t('game:hero.openInStore', { title: game.title, store: plugin.name })}
                className={cx(heroChipClass(chips), 'size-[50px] rounded-2xl text-white/80 hover:bg-black/70')}
              >
                <StoreIcon className="h-[18px] w-[18px]" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{t('game:hero.openInStoreTip')}</TooltipContent>
          </Tooltip>
        )}
      </div>
    </div>
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
      {size.state === 'loading' && <span className="text-text-muted">{t('game:installSize.checking')}</span>}
      {size.state === 'unknown' && <span title={t('game:installSize.unknownTitle')}>{t('game:installSize.unknown')}</span>}
      {size.state === 'known' && (
        <>
          {t('game:installSize.known', { size: formatBytes(size.total) })}
          {size.added > 0 && <span className="text-text-muted">{t('game:installSize.addons', { size: formatBytes(size.added) })}</span>}
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

/** The install note's folder links in the tabs row: plain sans text in the
 *  accent, underlined on hover (the lab's link style, not the mono links the
 *  sections page keeps). Only the tabs row uses it. */
export function TabsNoteLink({ onClick, children }: { onClick: () => void; children?: React.ReactNode }) {
  return (
    <button type="button" onClick={onClick} className="inline font-sans text-accent hover:underline">
      {children}
    </button>
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
  onJobAction: (action: 'cancel' | 'retry' | 'pause' | 'resume', id: string) => void;
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
    <Panel className="bg-surface-strong">
      <h2 className="mb-2 text-[15px] font-bold">{t('game:installation.heading')}</h2>
      <div className="space-y-2 font-mono text-hint">
        {rows.map(([k, v, title]) => (
          <div key={k} className="flex justify-between gap-4 border-t border-hairline pt-2">
            <span className="text-text-muted">{k}</span>
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

export function AchievementsSection({
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
        <span className="font-mono text-[12px] tabular-nums text-text-muted">
          {known ? t('game:achievements.countKnown', { unlocked: formatCount(ach.unlocked), total: formatCount(ach.total), percent: formatCount(pct) }) : t('game:achievements.countTotal', { total: formatCount(ach.total) })}
        </span>
      </div>
      {known ? (
        <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]">
          <div className="h-full rounded-full bg-accent-strong" style={{ width: `${pct}%` }} />
        </div>
      ) : (
        <p className="mb-4 text-[12px] text-text-body">
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
        <p className="mt-3 font-mono text-[12px] text-text-muted">
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
    <div title={a.description} className={cx('flex gap-3 rounded-2xl p-3 ring-1', a.unlocked ? 'bg-accent-strong/[0.07] ring-accent-strong/30' : 'ring-hairline')}>
      {a.icon ? (
        <img src={a.icon} alt="" loading="lazy" className={cx('h-11 w-11 flex-none rounded-lg', dim && 'opacity-40 grayscale')} />
      ) : (
        <span className="grid h-11 w-11 flex-none place-items-center rounded-lg bg-surface-strong"><Trophy className="h-4 w-4 text-text-muted" /></span>
      )}
      <div className={cx('min-w-0', dim && 'opacity-60')}>
        <div className="truncate text-[12px] font-bold">{a.name}</div>
        {a.description && <div className="line-clamp-2 text-hint leading-snug text-text-muted">{a.description}</div>}
        {meta && <div className="mt-0.5 font-mono text-hint text-text-muted">{meta}</div>}
      </div>
    </div>
  );
}
