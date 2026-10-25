// i18n: converted
import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Effect } from 'effect';
import { Shell, SourceFooter } from './components/Shell';
import { offers } from './lib/capabilities';
import { Dashboard } from './components/Dashboard';
import { LibraryView, type LibraryMode, type Ownership } from './components/LibraryView';
import { LibraryListDetail } from './components/LibraryListDetail';
import { GamePage, invalidateGameDetailsCache } from './components/GamePage';
import { StoreView } from './components/StoreView';
import { AddGameModal, type AddGameOptions } from './components/AddGameModal';
import { addGameFlow } from './lib/addGame';
import { ActivityView } from './components/ActivityView';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { StorageView } from './components/StorageView';
import { SystemView } from './components/SystemView';
import { LauncherSettingsView } from './components/LauncherSettingsView';
import { DownloadsView, jobKindKey } from './components/DownloadsView';
import { CloudSavesHost } from './components/CloudSavesHost';
import { FriendsModal } from './components/FriendsModal';
import { RedeemDialog } from './components/RedeemDialog';
import { CollectionsDialog } from './components/CollectionsDialog';
import { CommandPalette } from './components/CommandPalette';
import { collectionCounts, staleTitles } from './lib/collections';
import { ChatPanel } from './components/ChatPanel';
import { FamilyWarningHost } from './components/FamilyWarningHost';
import { TermsHost } from './components/TermsHost';
import { MigrationNotice as MigrationBanner } from './components/MigrationNotice';
import { cx, EmptyState } from './components/ui';
import { Button } from './components/ds/button';
import { TooltipProvider } from './components/ds/tooltip';
import { LauncherApi, onJobs, onLibrary, onLibraryPartial, onPlaytime, onSession, onSocial } from './effect/runtime';
import { reconcileViews, upsertView } from './lib/social';
import { ensureTerms } from './lib/termsRequest';
import { friendNameOf, useChat } from './lib/useChat';
import { isLiveState } from './lib/sessions';
import { usePauseWhileGameRuns } from './lib/pauseWhileHidden';
import { optimisticGamesAfterJob, withoutFinishedJobs } from './lib/jobs';
import { heroImages, needsAttention, mergeCopies, mergeCustomization, mergePlaytime, mergeRescannedGames } from './lib/games';
import type { ActivityEvent, Collection, Game, GameSettings, Job, MigrationNotice, ProtonVersion, RemoveOptions, SessionEvent, SocialStatus, SocialView, SourceWarning, StorePlugin, StoreType, Tweak, View } from './types';
import { useTranslation } from 'react-i18next';
import { applyLanguage, loadSessionInfo } from './lib/i18n';
import { loadTheme } from './lib/theme';
import { LibrarySkeleton } from './components/LibrarySkeleton';
import { hasDetails, plainError } from './lib/plainError';
import { coreErrMsg } from './lib/coreError';
import { Toaster } from './components/ds/sonner';
import { Modal } from './components/Modal';
import { useVariant } from './lib/theme';
import { useGameTone } from './lib/gameTone';
import type { GameSettingsTab } from './lib/gameSettingsTabs';
import { notify as toastNotify, type LegacyKind as NoticeKind } from './lib/notify';
import { X } from 'lucide-react';

export function App() {
  const { t } = useTranslation(['nav', 'common', 'activity']);
  const [games, setGames] = useState<Game[]>([]);
  const [runners, setRunners] = useState<ProtonVersion[]>([]);
  const [plugins, setPlugins] = useState<StorePlugin[]>([]);
  const [social, setSocial] = useState<SocialView[]>([]);
  const [friendsOpen, setFriendsOpen] = useState(false);
  const [redeemOpen, setRedeemOpen] = useState(false);
  // Collections (issue #45): the list from settings, and the dialog (for one game, or the manager).
  const [collections, setCollections] = useState<Collection[]>([]);
  const [collectionsDialog, setCollectionsDialog] = useState<{ gameId?: string } | null>(null);
  // Sources that failed to list (a logged-out plugin) on the last scan.
  const [warnings, setWarnings] = useState<SourceWarning[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [lastScan, setLastScan] = useState<number | null>(null);
  const [scanning, setScanning] = useState(false);

  const [view, setView] = useState<View>({ name: 'dashboard' });
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsReturnView, setSettingsReturnView] = useState<View>({ name: 'dashboard' });
  const settingsPresentation = useVariant('settings.presentation');
  // The shell is a view (issue #104): its regions depend on shell.navigation.
  // Sidebar mode keeps today's row layout; topbar mode stacks the bar above
  // the main column.
  const shellNavigation = useVariant('shell.navigation');
  useEffect(() => {
    if (settingsPresentation !== 'page' && view.name === 'settings') {
      setSettingsOpen(true);
      setView(settingsReturnView.name === 'settings' ? { name: 'dashboard' } : settingsReturnView);
    } else if (settingsPresentation === 'page' && settingsOpen) {
      setSettingsOpen(false);
      setView({ name: 'settings' });
    }
  }, [settingsPresentation, settingsOpen, settingsReturnView, view.name]);
  const [search, setSearch] = useState('');
  const [source, setSource] = useState<StoreType | 'all'>('all');
  // The sourcebar's multi-store selection (issue #125, D21 empty = every
  // store): kept in sync with the single source the sidebar and topbar use.
  const [selectedStores, setSelectedStores] = useState<StoreType[]>([]);
  const [libraryMode, setLibraryMode] = useState<LibraryMode>('cards');
  const [ownership, setOwnership] = useState<Ownership>('installed');

  const [addOpen, setAddOpen] = useState(false);
  const [launchingId, setLaunchingId] = useState<string | null>(null);
  // Games Platypus launched that are starting or running, keyed by game id.
  const [sessions, setSessions] = useState<Record<string, SessionEvent>>({});
  const [sessionFailures, setSessionFailures] = useState<Record<string, SessionEvent>>({});
  // Use less while hidden or unfocused (issue #49): animations, transitions
  // and videos pause while the page is hidden, or while blurred with a game
  // running. Polling loops check document.hidden before scheduling (see
  // PluginsView's QR poll); backgroundThrottling stays on in the shell.
  usePauseWhileGameRuns(Object.keys(sessions).length > 0);
  const [tweaks, setTweaks] = useState<Tweak[]>([]);
  const [dismissedHero, setDismissedHero] = useState<ReadonlySet<string>>(() => new Set());

  // Activity is real session events only (scans, launches, setting changes).
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const eventId = useRef(0);
  const log = useCallback((e: Omit<ActivityEvent, 'id' | 'ts'>) => {
    setActivity((prev) => [{ ...e, id: ++eventId.current, ts: Date.now() }, ...prev].slice(0, 200));
  }, []);

  // The one-time Hydra→Platypus migration notice (issue #14). Null when there
  // is nothing to show; dismissing persists it in the core and hides it here.
  const [migration, setMigration] = useState<MigrationNotice | null>(null);
  // Every in-app notice goes through Sonner (issue #64): stacked top-right
  // toasts, newest on top. onNotify props stay (least churn) as a thin
  // wrapper: ok → success, err → error, busy → info.
  const notify = useCallback((msg: string, kind: NoticeKind = 'ok') => {
    // An error toast says a plain sentence; the raw text rides as the
    // toast description, and the toast stays long enough to read it.
    const fallback = t('errors:fallback');
    if (kind === 'err') {
      const raw = hasDetails(msg, fallback) ? msg : undefined;
      toastNotify.error(plainError(msg, fallback), raw ? { description: raw } : undefined);
      return;
    }
    if (kind === 'busy') {
      toastNotify.info(msg);
      return;
    }
    toastNotify.success(msg);
    // t is a dep: toasts follow a live language switch.
  }, [t]);

  const main = useRef<HTMLElement>(null);
  useEffect(() => { main.current?.scrollTo({ top: 0 }); }, [view]);

  // Friends lists: the snapshot of every store that offers one, then each push.
  // A snapshot is merged with what the pushes already brought in, and it drops
  // the stores that are no longer ready (refreshSocial runs on a plugin change).
  const refreshSocial = useCallback(
    () => Effect.runPromise(LauncherApi.socialViews()).then((vs) => setSocial((list) => reconcileViews(list, vs)), () => {}),
    [],
  );
  const onPluginsChanged = useCallback((list: StorePlugin[]) => {
    setPlugins(list);
    void refreshSocial();
  }, [refreshSocial]);
  useEffect(() => {
    const unsubscribe = onSocial((v) => setSocial((list) => upsertView(list, v)));
    void refreshSocial();
    return unsubscribe;
  }, [refreshSocial]);

  // The startup scan shows the library of the sources that answered first (Steam
  // reads local files; a store plugin may wait on its store) instead of a
  // skeleton until the slowest one does. Later rescans keep what is shown.
  const startupScan = useRef(false);
  const savedSettings = useRef<Promise<Record<string, GameSettings>>>(Promise.resolve({}));
  useEffect(
    () =>
      onLibraryPartial((lib) => {
        if (!startupScan.current) return;
        void savedSettings.current.then((saved) => {
          if (!startupScan.current) return;
          setGames((prev) => mergeRescannedGames(prev, lib.games, saved));
          setLoaded(true);
        });
      }),
    [],
  );

  // When a session started before this window mounted (CORE-1).
  const sessionStart = useRef<Record<string, number>>({});
  // reason is what the read is for: 'startup' fills an empty window,
  // 'rescan' is the manual Rescan (every source is listed again), 'update'
  // refreshes after something changed (a job, a pin, a plugin). An update
  // reads through the backend cache, so it re-lists only the source the
  // change invalidated; a rescan bypasses the cache.
  const loadData = useCallback(
    (reason: 'startup' | 'rescan' | 'update') => {
      setScanning(true);
      // Each source resolves independently: one rejection must not discard
      // the other three successes (FE-3). Settle each call on its own
      // promise so a single failure can't fail the batch.
      type Settled<A> = { ok: true; value: A } | { ok: false; error: Error };
      const settle = <A,>(eff: Effect.Effect<A, Error>): Promise<Settled<A>> =>
        Effect.runPromise(eff).then(
          (value): Settled<A> => ({ ok: true, value }),
          (error): Settled<A> => ({ ok: false, error: error instanceof Error ? error : new Error(String(error)) }),
        );
      startupScan.current = reason === 'startup';
      // Unreadable saved settings shouldn't block the library.
      const savedCall = settle(LauncherApi.getGameSettings());
      savedSettings.current = savedCall.then((r) => (r.ok ? r.value : {}));
      const libraryCall = settle(reason === 'rescan' ? LauncherApi.rescanLibrary() : LauncherApi.getLibrary());
      return Promise.all([
        libraryCall,
        settle(LauncherApi.getProtonVersions()),
        settle(LauncherApi.listPlugins()),
        savedCall,
        // A missing session snapshot (old backend) shouldn't block the library either.
        settle(LauncherApi.getSessions()),
      ])
        .then(([libR, rR, pR, savedR, sessR]) => {
          const failures: string[] = [];
          const saved: Record<string, GameSettings> = savedR.ok ? savedR.value : {};
          if (!savedR.ok) failures.push(`${t('nav:title.settings')}: ${coreErrMsg((savedR as { ok: false; error: Error }).error)}`);
          if (libR.ok) {
            const lib = (libR as { ok: true; value: { games: Game[]; warnings: SourceWarning[] } }).value;
            const g = lib.games;
            setWarnings(lib.warnings);
            // Keep edited settings and frontend-enriched fix_info across
            // rescans (FE-5); on startup, settings come from disk.
            // Sideloaded games arrive from the backend now.
            setGames((prev) => mergeRescannedGames(prev, g, saved));
            const sources = new Set(g.map((x) => x.source)).size;
            const installed = g.filter((x) => x.is_installed).length;
            // Runner count uses whatever the runners call returned (possibly stale).
            log({
              type: 'update',
              detail: installed < g.length
                ? t('activity:log.scanOwned', { count: installed, owned: g.length })
                : t('activity:log.scan', { count: installed }),
              meta: t('activity:log.sources', { count: sources }),
            });
            for (const w of lib.warnings) log({
              type: 'update',
              detail: w.needs_auth
                ? t('activity:log.warnLoggedOut', { source: w.source })
                : w.unavailable
                  ? t('activity:log.warnUnavailable', { source: w.source })
                  : t('activity:log.warnLibrary', { source: w.source }),
              meta: w.message,
            });
          } else {
            failures.push(`${t('nav:title.library')}: ${coreErrMsg((libR as { ok: false; error: Error }).error)}`);
          }
          if (rR.ok) {
            setRunners((rR as { ok: true; value: ProtonVersion[] }).value);
          } else {
            failures.push(`${t('nav:title.runners')}: ${coreErrMsg((rR as { ok: false; error: Error }).error)}`);
          }
          if (pR.ok) {
            setPlugins((pR as { ok: true; value: StorePlugin[] }).value);
            void refreshSocial();
          } else {
            failures.push(`${t('nav:title.plugins')}: ${coreErrMsg((pR as { ok: false; error: Error }).error)}`);
          }
          // Hydrate sessions so a window opened after a launch shows Playing (CORE-1/FE-2).
          const live = sessR.ok ? (sessR as { ok: true; value: SessionEvent[] }).value : [];
          if (live.length > 0) {
            const seeded: Record<string, SessionEvent> = {};
            const now = Date.now();
            for (const e of live) {
              if (isLiveState(e.state)) {
                seeded[e.game_id] = e;
                if (e.state !== 'starting' && sessionStart.current[e.game_id] === undefined) {
                  sessionStart.current[e.game_id] = now;
                }
              }
            }
            setSessions(seeded);
          }
          if (failures.length > 0) {
            const msg = failures.join(' · ');
            setLoadError(msg);
            log({ type: 'update', detail: t('activity:log.scanErrors'), meta: msg });
            notify(msg, 'err');
          } else {
            setLoadError(null);
            if (reason === 'rescan') notify(t('common:libraryRescanned'));
          }
          // Updates can change store metadata (login, reinstall, a new pin):
          // evict the details cache so the next visit refetches (FE-6). Only
          // the manual rescan says so out loud; an update already notified
          // what changed (the finished job, the pin, the plugin).
          if (reason !== 'startup') invalidateGameDetailsCache();
          setLastScan(Date.now());
        })
        .catch((err) => {
          // Unreachable: every call is guarded. Kept as a safety net.
          const msg = coreErrMsg(err);
          setLoadError(msg);
          notify(msg, 'err');
        })
        .finally(() => {
          startupScan.current = false;
          setLoaded(true);
          setScanning(false);
        });
    },
    // t is a dep: rescan notices follow a live language switch (the startup
    // scan still runs once: `started` guards it).
    [log, notify, refreshSocial, t]
  );

  // The launcher language (issue #39): the shell's OS list behind System,
  // the core's setting for the explicit choice. Applies live, no restart.
  useEffect(() => {
    // A missing or failing settings read keeps English: the language is a
    // preference, never a reason to break startup (or tests with a partial
    // core mock).
    void loadSessionInfo()
      .then(() => {
        try {
          return Effect.runPromise(LauncherApi.getLauncherSettings());
        } catch {
          return null;
        }
      })
      .then(
        (st) => applyLanguage(st?.language ?? ''),
        () => applyLanguage(''),
      );
  }, []);

  // The picked theme (issue #93) replaces the one applied from the cache at boot.
  useEffect(() => { void loadTheme(); }, []);

  // StrictMode mounts effects twice in dev; scan only once on startup.
  const started = useRef(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getTweaks()).then(setTweaks).catch(() => {});
    // A null list (a Go nil slice over the bridge) reads as none.
    Effect.runPromise(LauncherApi.listCollections()).then((l) => setCollections(l ?? [])).catch(() => {});
    Effect.runPromise(LauncherApi.migrationNotice()).then(setMigration).catch(() => {});
  }, []);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    loadData('startup');
  }, [loadData]);


  const launch = (game: Game) => {
    if (launchingId || sessions[game.id]) return;
    setLaunchingId(game.id);
    Effect.runPromise(LauncherApi.launchGame(game))
      .then((res) => notify(res || t('game:launching', { title: game.title }), 'busy'))
      .catch((err) => notify(coreErrMsg(err), 'err'))
      .finally(() => setLaunchingId(null));
  };

  const stop = (gameId: string, force: boolean) => {
    Effect.runPromise(force ? LauncherApi.forceStopGame(gameId) : LauncherApi.stopGame(gameId)).catch((err) => notify(coreErrMsg(err), 'err'));
  };

  // Real session state comes from the backend: for Steam games it watches the
  // game process under Steam's reaper, not the short-lived `steam` command.
  const gamesRef = useRef(games);
  gamesRef.current = games;
  useEffect(
    () =>
      onSession((e) => {
        const title = gamesRef.current.find((g) => g.id === e.game_id)?.title ?? e.game_id;
        if (isLiveState(e.state)) {
          setSessionFailures((prev) => { const { [e.game_id]: _, ...rest } = prev; return rest; });
          setSessions((prev) => ({ ...prev, [e.game_id]: e }));
          if (e.state === 'running') {
            sessionStart.current[e.game_id] = Date.now();
            notify(t('game:nowRunning', { title }));
            log({ type: 'session', game: title, detail: t('activity:log.sessionStarted', { title }), meta: e.pid ? t('activity:log.sessionPid', { pid: e.pid }) : undefined });
          }
          return;
        }
        setSessions((prev) => {
          const { [e.game_id]: _, ...rest } = prev;
          return rest;
        });
        if (e.state === 'failed') {
          setSessionFailures((prev) => ({ ...prev, [e.game_id]: e }));
          notify(t('errors:gameFailed', { title, error: e.message ?? t('errors:launchFailedText') }), 'err');
          log({ type: 'session', game: title, detail: t('activity:log.sessionFailed', { title }), meta: e.message?.slice(0, 80) });
        } else {
          const started = sessionStart.current[e.game_id];
          delete sessionStart.current[e.game_id];
          const mins = started ? Math.round((Date.now() - started) / 60000) : 0;
          log({ type: 'session', game: title, detail: t('activity:log.sessionStopped', { title }), meta: started ? t('activity:log.sessionMinutes', { count: mins }) : undefined });
        }
      }),
    [log, notify]
  );

  // A hidden window can miss nothing (events stay live) but re-checks on
  // focus anyway, so it never shows a stale Play button. Silent: the event
  // stream remains the source of notifications and activity.
  useEffect(() => {
    const onFocus = () => {
      Effect.runPromise(LauncherApi.getSessions())
        .then((live) => {
          const seeded: Record<string, SessionEvent> = {};
          const now = Date.now();
          for (const e of live) {
            if (isLiveState(e.state)) {
              seeded[e.game_id] = e;
              if (e.state !== 'starting' && sessionStart.current[e.game_id] === undefined) {
                sessionStart.current[e.game_id] = now;
              }
            }
          }
          for (const id of Object.keys(sessionStart.current)) {
            if (!(id in seeded)) delete sessionStart.current[id];
          }
          setSessions(seeded);
        })
        .catch(() => {});
    };
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, []);

  const saveSettings = (id: string, settings: GameSettings, what = t('activity:log.tweakDefault')) => {
    const g = games.find((x) => x.id === id);
    setGames((prev) => prev.map((x) => (x.id === id ? { ...x, settings } : x)));
    if (!g) return Promise.resolve();
    // Returns the write: the settings drawer clears its dirty state only
    // once this resolves, so a failed save keeps its footer.
    return Effect.runPromise(LauncherApi.saveGameSettings(id, settings))
      .then(() => {
        notify(t('common:settingsSaved'));
        log({ type: 'tweak', game: g.title, detail: t('activity:log.tweakDetail', { what, title: g.title }) });
      })
      .catch((err) => {
        notify(t('errors:notPersisted', { error: coreErrMsg(err) }), 'err');
        log({ type: 'tweak', game: g.title, detail: t('activity:log.tweakDetail', { what, title: g.title }), meta: t('activity:log.tweakSessionOnly') });
        throw err;
      });
  };

  // The install queue. A finished install, update, uninstall or move changes
  // the library, so it refreshes it: the backend dropped that source from its
  // cache when the job finished, so the refresh re-lists only it.
  const jobStates = useRef(new Map<string, string>());
  const loadRef = useRef(loadData);
  loadRef.current = loadData;
  // Leaving the embedded store view: what was bought there appears in the
  // library (the backend dropped the source's cached listing, so the update
  // re-lists only it).
  const closeStore = useCallback((pluginId: string) => {
    setView({ name: 'library' });
    Effect.runPromise(LauncherApi.invalidateLibrary(pluginId))
      .then(() => loadRef.current('update'))
      .catch((err) => notify(coreErrMsg(err), 'err'));
  }, [notify]);
  useEffect(() => {
    const apply = (list: Job[], initial = false) => {
      setJobs(list);
      let refresh = false;
      for (const j of list) {
        const prev = jobStates.current.get(j.id);
        jobStates.current.set(j.id, j.state);
        if (initial || prev === j.state) continue;
        const verbKey = j.kind === 'uninstall' ? 'uninstalled' : j.kind === 'update' ? 'updated' : j.kind === 'backup' ? 'backedUp' : j.kind === 'restore' ? 'restored' : 'installed';
        const action = t(j.kind === 'uninstall' ? 'activity:log.jobVerbUninstalled' : j.kind === 'update' ? 'activity:log.jobVerbUpdated' : j.kind === 'backup' ? 'activity:log.jobVerbBackedUp' : j.kind === 'restore' ? 'activity:log.jobVerbRestored' : 'activity:log.jobVerbInstalled');
        if (j.state === 'done') {
          refresh = true;
          // Optimistic flip so the game page updates before the refresh
          // round-trip (FE-4); the refresh reconciles afterwards.
          setGames((prev) => optimisticGamesAfterJob(prev, j));
          notify(t(`downloads:${verbKey}`, { title: j.title }));
          log({ type: 'download', game: j.title, detail: t('activity:log.jobDone', { action, title: j.title }), meta: j.source });
        } else if (j.state === 'failed') {
          notify(t('downloads:jobFailed', { title: j.title, error: j.error ?? t(`downloads:actionFailed.${j.kind}`) }), 'err');
          log({ type: 'download', game: j.title, detail: t('activity:log.jobFailed', { kind: t(jobKindKey(j.kind)), title: j.title }), meta: j.error });
        }
      }
      if (refresh) loadRef.current('update');
    };
    Effect.runPromise(LauncherApi.listJobs()).then((l) => apply(l, true)).catch(() => {});
    return onJobs((l) => apply(l));
  }, [log, notify]);

  // A silent library read: no rescan notice and no details-cache eviction.
  // Only the fields `merge` names are taken from the fresh read. A failed read
  // keeps what is shown; the next rescan reconciles it. Reads go through the
  // backend cache, so asks that come while one is out share the next single
  // read instead of each starting their own.
  const quietRead = useRef<{ running: boolean; waiting: Set<(prev: Game[], fresh: Game[]) => Game[]> }>({ running: false, waiting: new Set() });
  const refreshLibraryQuietly = useCallback((merge: (prev: Game[], fresh: Game[]) => Game[]) => {
    const q = quietRead.current;
    q.waiting.add(merge);
    if (q.running) return;
    const read = () => {
      const merges = [...q.waiting];
      q.waiting.clear();
      q.running = true;
      const done = () => {
        q.running = false;
        if (q.waiting.size > 0) read();
      };
      Effect.runPromise(LauncherApi.getLibrary()).then(
        (lib) => {
          setGames((prev) => merges.reduce((acc, m) => m(acc, lib.games), prev));
          done();
        },
        done,
      );
    };
    read();
  }, []);

  // A plugin's playtime report changed a library value (protocol 1.7).
  useEffect(() => onPlaytime(() => refreshLibraryQuietly(mergePlaytime)), [refreshLibraryQuietly]);

  // The background artwork lookup (issue #36) applied art to a sideloaded
  // game: take the new art from a quiet read, like a manual customization.
  useEffect(() => onLibrary(() => refreshLibraryQuietly(mergeCustomization)), [refreshLibraryQuietly]);

  const runJob = (kind: 'install' | 'update' | 'uninstall', game: Game) => {
    const op = kind === 'install' ? LauncherApi.installGame : kind === 'update' ? LauncherApi.updateGame : LauncherApi.uninstallGame;
    const queue = kind === 'uninstall' ? Promise.resolve(true) : ensureTerms(game);
    queue
      .then((ok) => ok && Effect.runPromise(op(game))
        .then(() => notify(t('downloads:queued', { action: t(`downloads:action.${kind}`), title: game.title }), 'busy')))
      .catch((err) => notify(coreErrMsg(err), 'err'));
  };

  const jobAction = (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => {
    const call =
      action === 'cancel' ? LauncherApi.cancelJob
      : action === 'retry' ? LauncherApi.retryJob
      : action === 'pause' ? LauncherApi.pauseJob
      : action === 'resume' ? LauncherApi.resumeJob
      : LauncherApi.discardJob;
    Effect.runPromise(call(id)).catch((err) => notify(coreErrMsg(err), 'err'));
  };

  const allowFolder = (game: Game, folder: string) => {
    Effect.runPromise(LauncherApi.allowGameFolder(game.source, folder))
      .then(() => {
        notify(t('game:folderAllowed', { folder }));
        loadData('update');
      })
      .catch((err) => notify(coreErrMsg(err), 'err'));
  };

  // Adding is all-or-nothing: the modal stays open until the returned promise
  // settles. The game only joins the library once its advanced options are
  // saved too; a failed options save removes the new game again and the error
  // surfaces in the modal.
  const addGame = (title: string, executable: string, opts?: AddGameOptions) => {
    return addGameFlow(
      {
        addCustomGame: (t, e) => Effect.runPromise(LauncherApi.addCustomGame(t, e)),
        saveGameSettings: (id, s) => Effect.runPromise(LauncherApi.saveGameSettings(id, s)),
        removeCustomGame: (id) => Effect.runPromise(LauncherApi.removeCustomGame(id)),
        setCustomGameYear: (id, y) => Effect.runPromise(LauncherApi.setCustomGameYear(id, y)),
      },
      title,
      executable,
      opts,
    ).then(({ game }) => {
      // Sideloaded games lead the library, like before, with their saved settings.
      setGames((prev) => [game, ...prev.filter((x) => x.id !== game.id)]);
      notify(t('game:added', { title }));
      log({ type: 'add', game: title, detail: t('activity:log.sideloaded', { title }), meta: game.executable });
    });
  };

  // Forgetting a sideloaded game: library only drops it at once (files stay
  // on disk); with a delete flag the backend queues a removal job instead.
  // A running game can't leave the library until it stops, or it could no
  // longer be stopped from the launcher. Leaving the page only happens when
  // the user is still on this game's page: a navigation meanwhile wins.
  const removeGame = (game: Game, opts: RemoveOptions) => {
    if (sessions[game.id] || launchingId === game.id) {
      notify(t('game:stopBeforeRemove', { title: game.title }), 'err');
      return;
    }
    if (!opts.delete_prefix && !opts.delete_files) {
      Effect.runPromise(LauncherApi.removeCustomGame(game.id))
        .then(() => {
          setGames((prev) => prev.filter((x) => x.id !== game.id));
          invalidateGameDetailsCache(game.id);
          setView((v) => (v.name === 'game' && v.id === game.id ? { name: v.from } : v));
          notify(t('game:removed', { title: game.title }));
          log({ type: 'remove', game: game.title, detail: t('activity:log.removed', { title: game.title }), meta: game.install_dir || undefined });
        })
        .catch((err) => notify(coreErrMsg(err), 'err'));
      return;
    }
    Effect.runPromise(LauncherApi.removeGame(game.id, opts))
      .then(() => {
        notify(t('downloads:queued', { action: t('downloads:action.remove'), title: game.title }), 'busy');
      })
      .catch((err) => notify(coreErrMsg(err), 'err'));
  };

  const installedGames = useMemo(() => games.filter((g) => g.is_installed), [games]);
  const storeCounts = useMemo(
    () => installedGames.reduce<Record<string, number>>((acc, g) => ((acc[g.store] = (acc[g.store] ?? 0) + 1), acc), {}),
    [installedGames]
  );
  const activeJobs = jobs.filter((j) => j.state === 'queued' || j.state === 'running');
  const jobFor = (id: string) => activeJobs.find((j) => j.game_id === id) ?? [...jobs].reverse().find((j) => j.game_id === id && j.state === 'failed');

  const collectionTotals = useMemo(() => collectionCounts(collections, games, Math.floor(Date.now() / 1000)), [collections, games]);
  // Manual collections remember each game's title (issue #77), so a game that
  // leaves the library still has a name. Titles are sent only when out of date.
  useEffect(() => {
    const stale = staleTitles(collections, games);
    if (Object.keys(stale).length === 0) return;
    Effect.runPromise(LauncherApi.rememberCollectionTitles(stale)).then((l) => setCollections(l ?? [])).catch(() => {});
  }, [collections, games]);
  const socialLabels = useMemo(() => Object.fromEntries(plugins.map((p) => [p.id, p.name])), [plugins]);
  // Enabled plugins that declare a storefront: the sidebar's Store section
  // and the embedded store view (issue #17).
  const storePlugins = useMemo(
    () => plugins.filter((p) => p.status === 'ready' && p.storefront_url).sort((a, b) => a.name.localeCompare(b.name)),
    [plugins],
  );
  // Plugin logos by store (and id) for the sidebar Sources rows: the store's
  // own plugin first, so a branded row never takes another plugin's mark.
  const sourceLogos = useMemo(() => {
    const logos: Record<string, string> = {};
    const sorted = [...plugins].sort((a, b) => (a.store === a.id ? 0 : 1) - (b.store === b.id ? 0 : 1));
    for (const p of sorted) {
      if (!p.icon_url) continue;
      logos[p.store] ??= p.icon_url;
      logos[p.id] ??= p.icon_url;
    }
    return logos;
  }, [plugins]);
  // Conversations with friends (chat capability): unread badges, the open chat, alerts.
  const chat = useChat(social, notify, socialLabels);
  // Redeem is one entry for every ready store that offers it (none: no entry).
  const redeemStores = useMemo(() => plugins.filter((p) => p.status === 'ready' && offers(p, 'redeem')), [plugins]);
  const libraryIds = useMemo(() => new Set(games.map((g) => g.id)), [games]);

  // Stable, so the library's memoized cards are not re-rendered by every App update.
  const openGame = useCallback(
    (id: string) =>
      setView((v) =>
        // Opening the game already shown keeps its back target.
        v.name === 'game' && v.id === id
          ? v
          : { name: 'game', id, from: v.name === 'library' || v.name === 'fixes' ? v.name : 'dashboard' },
      ),
    [],
  );
  // The Settings entry (the shell's and the palette's): the theme's presentation
  // decides whether Settings replaces the view or opens over it.
  const openSettings = useCallback(() => {
    setSettingsReturnView(view);
    if (settingsPresentation === 'page') setView({ name: 'settings' });
    else setSettingsOpen(true);
  }, [settingsPresentation, view]);
  // Game settings asked for from the palette (issue #124): open the game page
  // on that tab, as the page's own Settings button does.
  const [gameSettingsTab, setGameSettingsTab] = useState<{ gameId: string; tab: GameSettingsTab } | null>(null);
  const openGameSettings = useCallback(
    (id: string) => {
      openGame(id);
      setGameSettingsTab({ gameId: id, tab: 'general' });
    },
    [openGame],
  );
  // Closing the friends modal ends any conversation in it: the chat only shows inside the modal.
  const closeFriends = () => { chat.close(); setFriendsOpen(false); };
  // A clicked desktop notification opens its conversation here (issue #12):
  // the chat only shows inside the modal, so the modal opens with it.
  useEffect(() => { if (chat.target) setFriendsOpen(true); }, [chat.target]);

  // Dismissing the migration notice persists it in the core (it never
  // returns) and hides the banner at once, without waiting for a relaunch.
  const dismissMigration = () => {
    setMigration(null);
    Effect.runPromise(LauncherApi.dismissMigrationNotice()).catch((err) => notify(coreErrMsg(err), 'err'));
  };

  // Atlas list-detail (issue #126): the library is the list plus the stage.
  const libraryPresentation = useVariant('library.presentation');
  const isListDetail = libraryPresentation === 'list-detail';
  // The staged game in the library list-detail (for the game tone and the
  // palette's actions): tracked whenever the stage shows, whatever opened
  // it. Opening a game from the palette or dashboard lands on the stage,
  // and clicking another row re-stages, so both follow the staged game.
  const [stagedGameId, setStagedGameId] = useState<string | null>(null);
  const stagedGame = isListDetail ? games.find((g) => g.id === stagedGameId) ?? undefined : undefined;
  const gameViewGame = view.name === 'game' ? games.find((g) => g.id === view.id) : undefined;
  const currentGame = isListDetail ? (stagedGame ?? gameViewGame) : gameViewGame;
  // The hero art the game page actually shows (the details hero once it
  // loads): the tone samples that, falling back to the library chain.
  const [heroArt, setHeroArt] = useState<string | null>(null);
  const currentGameId = currentGame?.id;
  useEffect(() => { setHeroArt(null); }, [currentGameId]);
  // The game's colour (issue #123), applied here so it also works where the
  // theme shows no switch; the sourcebar shell renders the switch.
  const gameTone = useGameTone(currentGame ? (heroArt ?? heroImages(currentGame)[0]) : undefined);

  // One game page for the game view and the list-detail stage (issues #126,
  // #127): the same props, so the stage renders the game page for the
  // selected game with the real data and APIs.
  const renderGamePage = (game: Game, from: 'dashboard' | 'library' | 'fixes', hideBack = false) => (
    <GamePage
      game={game}
      runners={runners}
      activity={activity}
      backLabel={t(`nav:${from}`)}
      onBack={() => setView({ name: from } as View)}
      hideBack={hideBack}
      onHeroArt={setHeroArt}
      onTuneSaved={(st) => setGames((prev) => prev.map((x) => (x.id === game.id ? { ...x, settings: st } : x)))}
      onPlay={() => launch(game)}
      onStop={() => stop(game.id, false)}
      onForceStop={() => stop(game.id, true)}
      launching={launchingId === game.id}
      session={sessions[game.id] ?? sessionFailures[game.id]}
      sessionStartedAt={sessionStart.current[game.id]}
      refreshing={scanning}
      tweaks={tweaks}
      // saveSettings rethrows after its toast so the settings drawer can
      // keep its dirty state; the tweak switch has no such state, so a
      // failed tweak save must not leave an unhandled rejection here.
      onChangeSettings={(st, what) => { void saveSettings(game.id, st, what).catch(() => {}); }}
      onSaveSettings={(st) => saveSettings(game.id, st)}
      onCustomized={() => refreshLibraryQuietly(mergeCustomization)}
      plugin={plugins.find((x) => x.id === game.source)}
      sourceLogos={sourceLogos}
      job={jobFor(game.id)}
      jobs={jobs}
      onJob={(kind) => runJob(kind, game)}
      onJobAction={jobAction}
      onAllowFolder={(folder) => allowFolder(game, folder)}
      onNavigate={setView}
      onNotify={notify}
      onVersionsChanged={() => loadData('update')}
      onCopyChanged={() => refreshLibraryQuietly(mergeCopies)}
      onRemove={(opts) => removeGame(game, opts)}
      collections={collections}
      onManageCollections={(gameId) => setCollectionsDialog({ gameId })}
      openSettingsTab={gameSettingsTab?.gameId === game.id ? gameSettingsTab.tab : undefined}
      onSettingsOpened={() => setGameSettingsTab(null)}
    />
  );

  // The list-detail library (issue #126): the list plus the stage. Opening a
  // game from elsewhere (palette, dashboard) lands here with that game
  // selected, through the game view below.
  const renderListDetail = (initialSelectedId?: string | null) => (
    <LibraryListDetail
      games={games}
      ownership={ownership}
      source={source}
      storeSelection={shellNavigation === 'sourcebar' ? selectedStores : undefined}
      collection={view.name === 'library' ? view.collection : undefined}
      collections={collections}
      onPlay={(id) => {
        const g = games.find((x) => x.id === id);
        if (g) launch(g);
      }}
      renderStage={(g) => renderGamePage(g, view.name === 'game' && view.from ? view.from : 'library', true)}
      initialSelectedId={initialSelectedId}
      onStageChange={setStagedGameId}
      onAddGame={() => setAddOpen(true)}
      warnings={warnings}
      onOpenPlugins={() => setView({ name: 'plugins' })}
      onResetFilters={() => {
        setOwnership('owned');
        setSelectedStores([]);
        setSource('all');
        setSearch('');
        if (view.name !== 'library') setView({ name: 'library' });
        else setView({ name: 'library', collection: undefined });
      }}
    />
  );

  const renderView = () => {
    if (!loaded) return <LibrarySkeleton />;
    if (loadError && games.length === 0 && (view.name === 'dashboard' || view.name === 'library')) {
      return (
        <div className="p-7">
          <EmptyState
            label={t('nav:backendUnreachable')}
            action={<Button variant="primary" size="sm" onClick={() => loadData('rescan')}>{t('common:retry')}</Button>}
          >
            {loadError}
          </EmptyState>
        </div>
      );
    }
    switch (view.name) {
      case 'dashboard':
        return (
          <Dashboard
            games={installedGames}
            ownedCount={games.length}
            runners={runners}
            activity={activity}
            dismissedHero={dismissedHero}
            onDismissHero={(ids) => setDismissedHero((prev) => new Set([...prev, ...ids]))}
            onOpenGame={openGame}
            onNavigate={setView}
          />
        );
      case 'library':
        // Atlas list-detail is the list plus the stage; other themes keep the grid.
        if (isListDetail) return renderListDetail();
        return (
          <LibraryView
            games={games}
            runners={runners}
            activity={activity}
            search={search}
            onSearch={setSearch}
            source={source}
            onSource={setSource}
            storeSelection={shellNavigation === 'sourcebar' ? selectedStores : undefined}
            mode={libraryMode}
            onMode={setLibraryMode}
            ownership={ownership}
            onOwnership={setOwnership}
            collection={view.name === 'library' ? view.collection : undefined}
            onCollection={(id) => setView({ name: 'library', collection: id })}
            collections={collections}
            onManageCollections={(gameId) => setCollectionsDialog({ gameId })}
            warnings={warnings}
            plugins={plugins}
            sourceLogos={sourceLogos}
            jobs={activeJobs}
            lastScan={lastScan}
            onOpenGame={openGame}
            onAddGame={() => setAddOpen(true)}
            onNavigate={setView}
          />
        );
      case 'game':
        // Atlas list-detail shows the list with that game selected on the stage.
        if (isListDetail) {
          const selected = games.find((g) => g.id === view.id);
          if (!selected) {
            return (
              <div className="p-7"><EmptyState label={t('nav:notFound')}>{t('nav:goneDetail')}</EmptyState></div>
            );
          }
          return renderListDetail(view.id);
        }
        return currentGame ? (
          renderGamePage(currentGame, view.from)
        ) : (
          <div className="p-7"><EmptyState label={t('nav:notFound')}>{t('nav:goneDetail')}</EmptyState></div>
        );
      case 'activity':
        return <ActivityView events={activity} />;
      case 'fixes':
        return <FixesView games={games} onOpenGame={openGame} />;
      case 'runners':
        return <SystemView runners={runners} onRefresh={() => loadData('rescan')} scanning={scanning} />;
      case 'plugins':
        return (
          <PluginsView
            plugins={plugins}
            warnings={warnings}
            onPlugins={onPluginsChanged}
            onLibraryChanged={() => loadData('update')}
            onNotify={notify}
          />
        );
      case 'storage':
        return (
          <StorageView
            games={games}
            onNotify={notify}
            onRemove={removeGame}
            onUninstall={(game) => runJob('uninstall', game)}
          />
        );
      case 'downloads':
        return (
          <DownloadsView
            jobs={jobs}
            onAction={jobAction}
            onClear={() => {
              // Optimistic clear so the button feels instant even when the
              // next jobs event is slow (FE-9); reconcile afterwards.
              setJobs((prev) => withoutFinishedJobs(prev));
              Effect.runPromise(LauncherApi.clearFinishedJobs())
                .then(() => Effect.runPromise(LauncherApi.listJobs()).then(setJobs).catch(() => {}))
                .catch((err) => {
                  notify(coreErrMsg(err), 'err');
                  Effect.runPromise(LauncherApi.listJobs()).then(setJobs).catch(() => {});
                });
            }}
            onOpenGame={(id) => games.some((g) => g.id === id) && openGame(id)}
          />
        );
      case 'settings':
        return settingsPresentation === 'page' ? <LauncherSettingsView onNotify={notify} plugins={plugins} /> : null;
      case 'store': {
        const plugin = storePlugins.find((x) => x.id === view.plugin);
        if (!plugin) {
          return (
            <div className="p-7">
              <EmptyState label={t('store:unavailable')}>{t('store:unavailableDetail')}</EmptyState>
            </div>
          );
        }
        return (
          <StoreView
            plugin={plugin}
            initialUrl={view.url}
            onClose={() => closeStore(plugin.id)}
            onNotify={notify}
          />
        );
      }
    }
  };

  // Atlas parity (issue #132): the list and stage scroll inside themselves,
  // so App's main clips only on the library/game views in list-detail mode;
  // every other view scrolls like warm.
  const listDetailActive = isListDetail && (view.name === 'library' || view.name === 'game');

  return (
    <TooltipProvider delayDuration={300}>
    <div className={cx('flex h-screen overflow-hidden text-foreground', view.name === 'game' ? 'app-wash-game' : 'app-wash', (shellNavigation === 'topbar' || shellNavigation === 'sourcebar') && 'flex-col')}>
      <Shell
        view={view}
        onNavigate={setView}
        onSettingsOpen={openSettings}
        gameTone={gameTone}
        search={search}
        onSearch={setSearch}
        storeCounts={storeCounts}
        onPickSource={(s) => { setSource(s); setSelectedStores(s === 'all' ? [] : [s]); setView({ name: 'library' }); }}
        sourceLogos={sourceLogos}
        gameCount={installedGames.length}
        loading={!loaded}
        downloadCount={activeJobs.length}
        authNeeded={warnings.some((w) => w.needs_auth) || plugins.some((x) => x.status === 'unapproved' || x.status === 'changed')}
        fixCount={games.filter(needsAttention).length}
        runnerCount={runners.length}
        lastScan={lastScan}
        sessions={Object.values(sessions).map((e) => ({ ...e, title: games.find((g) => g.id === e.game_id)?.title ?? e.game_id }))}
        onStop={(id) => stop(id, false)}
        friends={social}
        friendLabels={socialLabels}
        onOpenFriends={() => setFriendsOpen(true)}
        canRedeem={redeemStores.length > 0}
        onRedeem={() => setRedeemOpen(true)}
        friendsUnread={chat.unread}
        stores={storePlugins}
        activeStore={view.name === 'store' ? view.plugin : null}
        onOpenStore={(id) => setView({ name: 'store', plugin: id })}
        collections={collections}
        collectionCounts={collectionTotals}
        onOpenCollection={(id) => setView({ name: 'library', collection: id })}
        onManageCollections={() => setCollectionsDialog({})}
        ownership={ownership}
        onOwnership={setOwnership}
        ownedCount={games.length}
        storeSelection={selectedStores}
        onSelectStores={(s) => { setSelectedStores(s); setSource(s.length === 1 ? s[0] : 'all'); }}
        jobs={activeJobs}
        sessionStartedAt={{ ...sessionStart.current }}
      />
      {settingsPresentation !== 'page' && settingsOpen && (
        <Modal label={t('nav:title.settings')} presentation={settingsPresentation === 'sheet' ? 'sheet' : 'dialog'} size="xl" className="h-[min(52rem,100%)]" onClose={() => setSettingsOpen(false)}>
          <button type="button" aria-label={t('common:close')} onClick={() => setSettingsOpen(false)} className="absolute right-5 top-5 z-10 rounded-lg p-2 text-text-muted hover:bg-surface-strong focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus"><X aria-hidden className="h-4 w-4" /></button>
          <LauncherSettingsView onNotify={notify} plugins={plugins} />
        </Modal>
      )}

      <main ref={main} data-scroll-root className={cx('min-h-0 min-w-0 flex-1', listDetailActive ? 'overflow-hidden' : 'overflow-y-auto overflow-x-hidden')}>
        {migration && (
          <div className="px-7 pt-7">
            <MigrationBanner
              notice={migration}
              onOpenPlugins={() => setView({ name: 'plugins' })}
              onDismiss={dismissMigration}
            />
          </div>
        )}
        {renderView()}
      </main>
      {shellNavigation === 'sourcebar' && (
        <SourceFooter
          sessions={Object.values(sessions).map((e) => ({ ...e, title: games.find((g) => g.id === e.game_id)?.title ?? e.game_id }))}
          jobs={activeJobs}
          sessionStartedAt={{ ...sessionStart.current }}
          onStop={(id) => stop(id, false)}
        />
      )}

      {addOpen && <AddGameModal onClose={() => setAddOpen(false)} onAdd={addGame} />}

      {/* The command palette (issue #124) is one dialog at the app root; the
          shell reaches it through openCommandPalette(). */}
      <CommandPalette
        games={games}
        currentGame={currentGame}
        session={currentGame ? sessions[currentGame.id] : undefined}
        stores={storePlugins}
        onOpenGame={openGame}
        onNavigate={setView}
        onOpenStore={(id) => setView({ name: 'store', plugin: id })}
        onOpenSettings={openSettings}
        onPlay={launch}
        onStop={(id) => stop(id, false)}
        onInstall={(game) => runJob('install', game)}
        onOpenGameSettings={openGameSettings}
        onRedeem={redeemStores.length > 0 ? () => setRedeemOpen(true) : undefined}
        onManageCollections={() => setCollectionsDialog({})}
        onAddGame={() => setAddOpen(true)}
      />

      <CloudSavesHost games={games} onNotify={notify} onLog={log} />
      <FamilyWarningHost games={games} />
      <TermsHost />

      {redeemOpen && redeemStores.length > 0 && <RedeemDialog stores={redeemStores} onClose={() => setRedeemOpen(false)} />}
      {collectionsDialog && (
        <CollectionsDialog
          collections={collections}
          games={games}
          gameId={collectionsDialog.gameId}
          onChanged={setCollections}
          onClose={() => setCollectionsDialog(null)}
        />
      )}

      {friendsOpen && (
        <FriendsModal
          views={social}
          labels={socialLabels}
          plugins={plugins}
          inLibrary={(id) => libraryIds.has(id)}
          onClose={closeFriends}
          onSetEnabled={(plugin, enabled) =>
            Effect.runPromise(LauncherApi.setSocialEnabled(plugin, enabled)).then(refreshSocial).catch((err) => notify(coreErrMsg(err), 'err'))}
          onSetStatus={(plugin, state: SocialStatus) =>
            Effect.runPromise(LauncherApi.setSocialStatus(plugin, state)).catch((err) => notify(coreErrMsg(err), 'err'))}
          onSetPlaying={(plugin, show) =>
            Effect.runPromise(LauncherApi.setSocialPlaying(plugin, show)).then(refreshSocial).catch((err) => notify(coreErrMsg(err), 'err'))}
          onReconnect={(plugin) =>
            Effect.runPromise(LauncherApi.restartSocial(plugin)).then(refreshSocial).catch((err) => notify(coreErrMsg(err), 'err'))}
          onOpenGame={(id) => { closeFriends(); openGame(id); }}
          unreadOf={chat.unreadOf}
          onOpenChat={(plugin, friend) => chat.open(plugin, friend.id)}
          chatAlerts={chat.alerts}
          onSetChatAlerts={chat.setAlerts}
          chat={chat.target && (
            <ChatPanel
              view={chat.conversation}
              friendName={friendNameOf(social, chat.target.plugin, chat.target.friendId)}
              storeName={socialLabels[chat.target.plugin] ?? chat.target.plugin}
              error={chat.error}
              onLoadOlder={chat.loadOlder}
              onSend={chat.send}
              onResend={chat.resend}
              onBack={chat.close}
              onClose={closeFriends}
            />
          )}
          onBackFromChat={chat.close}
        />
      )}

      <Toaster />
    </div>
    </TooltipProvider>
  );
}

export default App;
