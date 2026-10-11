import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Effect } from 'effect';
import { Sidebar } from './components/Sidebar';
import { offers } from './lib/capabilities';
import { Dashboard } from './components/Dashboard';
import { LibraryView, type LibraryMode, type Ownership } from './components/LibraryView';
import { GamePage, invalidateGameDetailsCache } from './components/GamePage';
import { AddGameModal } from './components/AddGameModal';
import { ActivityView } from './components/ActivityView';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { SystemView } from './components/SystemView';
import { LauncherSettingsView } from './components/LauncherSettingsView';
import { DownloadsView } from './components/DownloadsView';
import { CloudSavesHost } from './components/CloudSavesHost';
import { FriendsModal } from './components/FriendsModal';
import { RedeemDialog } from './components/RedeemDialog';
import { ChatPanel } from './components/ChatPanel';
import { FamilyWarningHost } from './components/FamilyWarningHost';
import { TermsHost } from './components/TermsHost';
import { MigrationNotice as MigrationBanner } from './components/MigrationNotice';
import { cx, EmptyState } from './components/ui';
import { Button } from './components/ds/button';
import { TooltipProvider } from './components/ds/tooltip';
import { LauncherApi, onJobs, onLibraryPartial, onPlaytime, onSession, onSocial } from './effect/runtime';
import { reconcileViews, upsertView } from './lib/social';
import { ensureTerms } from './lib/termsRequest';
import { friendNameOf, useChat } from './lib/useChat';
import { isLiveState } from './lib/sessions';
import { optimisticGamesAfterJob, withoutFinishedJobs } from './lib/jobs';
import { needsAttention, mergeCopies, mergeCustomization, mergePlaytime, mergeRescannedGames } from './lib/games';
import type { ActivityEvent, Game, GameSettings, Job, MigrationNotice, ProtonVersion, SessionEvent, SocialStatus, SocialView, SourceWarning, StorePlugin, StoreType, Tweak, View } from './types';
import { RawDetails } from './components/ErrorNotice';
import { LibrarySkeleton } from './components/LibrarySkeleton';
import { hasDetails, plainError } from './lib/plainError';

type NoticeKind = 'ok' | 'err' | 'busy';
interface Notice { msg: string; kind: NoticeKind; raw?: string }

/** What an error toast says when the message is developer text. */
const TOAST_ERROR_FALLBACK = 'Something went wrong. Try again.';

const NOTICE_STYLE: Record<NoticeKind, string> = {
  ok: 'border-emerald-300/50 text-emerald-100',
  err: 'border-red-400/50 text-red-100',
  busy: 'border-amber-300/50 text-amber-100',
};
const NOTICE_DOT: Record<NoticeKind, string> = { ok: 'bg-emerald-400', err: 'bg-red-400', busy: 'bg-amber-300' };

const errMsg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export function App() {
  const [games, setGames] = useState<Game[]>([]);
  const [runners, setRunners] = useState<ProtonVersion[]>([]);
  const [plugins, setPlugins] = useState<StorePlugin[]>([]);
  const [social, setSocial] = useState<SocialView[]>([]);
  const [friendsOpen, setFriendsOpen] = useState(false);
  const [redeemOpen, setRedeemOpen] = useState(false);
  // Sources that failed to list (a logged-out plugin) on the last scan.
  const [warnings, setWarnings] = useState<SourceWarning[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [lastScan, setLastScan] = useState<number | null>(null);
  const [scanning, setScanning] = useState(false);

  const [view, setView] = useState<View>({ name: 'dashboard' });
  const [search, setSearch] = useState('');
  const [source, setSource] = useState<StoreType | 'all'>('all');
  const [libraryMode, setLibraryMode] = useState<LibraryMode>('cards');
  const [ownership, setOwnership] = useState<Ownership>('installed');

  const [addOpen, setAddOpen] = useState(false);
  const [launchingId, setLaunchingId] = useState<string | null>(null);
  // Games Platypus launched that are starting or running, keyed by game id.
  const [sessions, setSessions] = useState<Record<string, SessionEvent>>({});
  const [tweaks, setTweaks] = useState<Tweak[]>([]);
  const [dismissedHero, setDismissedHero] = useState<ReadonlySet<string>>(() => new Set());

  // Activity is real session events only (scans, launches, setting changes).
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const eventId = useRef(0);
  const log = useCallback((e: Omit<ActivityEvent, 'id' | 'ts'>) => {
    setActivity((prev) => [{ ...e, id: ++eventId.current, ts: Date.now() }, ...prev].slice(0, 200));
  }, []);

  const [notice, setNotice] = useState<Notice | null>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // The one-time Hydra→Platypus migration notice (issue #14). Null when there
  // is nothing to show; dismissing persists it in the core and hides it here.
  const [migration, setMigration] = useState<MigrationNotice | null>(null);
  const notify = useCallback((msg: string, kind: NoticeKind = 'ok') => {
    // An error toast says a plain sentence; the raw text waits behind Details,
    // and the toast stays long enough to open it.
    const isErr = kind === 'err';
    const raw = isErr && hasDetails(msg, TOAST_ERROR_FALLBACK) ? msg : undefined;
    setNotice({ msg: isErr ? plainError(msg, TOAST_ERROR_FALLBACK) : msg, kind, raw });
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), isErr ? 8000 : 3500);
  }, []);
  useEffect(() => () => { if (noticeTimer.current) clearTimeout(noticeTimer.current); }, []);

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
          if (!savedR.ok) failures.push(`Settings: ${errMsg((savedR as { ok: false; error: Error }).error)}`);
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
              detail: `Library scan · ${installed} installed${installed < g.length ? `, ${g.length} owned` : ''}`,
              meta: `${sources} source${sources === 1 ? '' : 's'}`,
            });
            for (const w of lib.warnings) log({ type: 'update', detail: `${w.source}: ${w.needs_auth ? 'logged out' : w.unavailable ? 'unavailable right now' : 'library unavailable'}`, meta: w.message });
          } else {
            failures.push(`Library: ${errMsg((libR as { ok: false; error: Error }).error)}`);
          }
          if (rR.ok) {
            setRunners((rR as { ok: true; value: ProtonVersion[] }).value);
          } else {
            failures.push(`Runners: ${errMsg((rR as { ok: false; error: Error }).error)}`);
          }
          if (pR.ok) {
            setPlugins((pR as { ok: true; value: StorePlugin[] }).value);
            void refreshSocial();
          } else {
            failures.push(`Plugins: ${errMsg((pR as { ok: false; error: Error }).error)}`);
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
            log({ type: 'update', detail: 'Library scan had errors', meta: msg });
            notify(msg, 'err');
          } else {
            setLoadError(null);
            if (reason === 'rescan') notify('Library rescanned');
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
          const msg = errMsg(err);
          setLoadError(msg);
          notify(msg, 'err');
        })
        .finally(() => {
          startupScan.current = false;
          setLoaded(true);
          setScanning(false);
        });
    },
    [log, notify, refreshSocial]
  );

  // StrictMode mounts effects twice in dev; scan only once on startup.
  const started = useRef(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getTweaks()).then(setTweaks).catch(() => {});
    Effect.runPromise(LauncherApi.migrationNotice()).then(setMigration).catch(() => {});
  }, []);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    loadData('startup');
  }, [loadData]);

  // Enrich with fix/tier data (curated first, then the ProtonDB tier) in the background.
  // A game is asked at most once per session, except when it had no answer
  // yet: the first ProtonDB fetch can outlast the wait, and it fills the cache
  // meanwhile, so those are asked again (three tries at most).
  const fixFetched = useRef(new Set<string>());
  const fixTries = useRef(new Map<string, number>());
  const [fixRetry, setFixRetry] = useState(0);
  useEffect(() => {
    const todo = games.filter((g) => g.is_installed && !g.fix_info && g.store !== 'custom' && !fixFetched.current.has(g.id));
    if (todo.length === 0) return;
    todo.forEach((g) => fixFetched.current.add(g.id));
    const program = Effect.forEach(
      todo,
      (g) => LauncherApi.getGameFix(g).pipe(Effect.map((fix) => [g.id, fix] as const), Effect.orElseSucceed(() => [g.id, null] as const)),
      { concurrency: 6 }
    );
    Effect.runPromise(program).then((results) => {
      const found = new Map(results.filter(([, f]) => f).map(([id, f]) => [id, f!]));
      let again = false;
      for (const [id] of results.filter(([, f]) => !f)) {
        const tries = (fixTries.current.get(id) ?? 0) + 1;
        fixTries.current.set(id, tries);
        if (tries < 3) {
          fixFetched.current.delete(id);
          again = true;
        }
      }
      if (again) setTimeout(() => setFixRetry((n) => n + 1), 20000);
      if (found.size === 0) return;
      setGames((prev) => prev.map((g) => (found.has(g.id) && !g.fix_info ? { ...g, fix_info: found.get(g.id) } : g)));
      const curated = [...found.values()].filter((f) => f.source !== 'protondb').length;
      log({ type: 'fix', detail: `Compatibility data for ${found.size} titles`, meta: curated ? `${curated} curated fixes` : 'protondb' });
    });
  }, [games, log, fixRetry]);

  const launch = (game: Game) => {
    if (launchingId || sessions[game.id]) return;
    setLaunchingId(game.id);
    Effect.runPromise(LauncherApi.launchGame(game))
      .then((res) => notify(res || `Launching ${game.title}`, 'busy'))
      .catch((err) => notify(errMsg(err), 'err'))
      .finally(() => setLaunchingId(null));
  };

  const stop = (gameId: string, force: boolean) => {
    Effect.runPromise(force ? LauncherApi.forceStopGame(gameId) : LauncherApi.stopGame(gameId)).catch((err) => notify(errMsg(err), 'err'));
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
          setSessions((prev) => ({ ...prev, [e.game_id]: e }));
          if (e.state === 'running') {
            sessionStart.current[e.game_id] = Date.now();
            notify(`${title} is running`);
            log({ type: 'session', game: title, detail: `Started ${title}`, meta: e.pid ? `pid ${e.pid}` : undefined });
          }
          return;
        }
        setSessions((prev) => {
          const { [e.game_id]: _, ...rest } = prev;
          return rest;
        });
        if (e.state === 'failed') {
          notify(`${title}: ${e.message ?? 'launch failed'}`, 'err');
          log({ type: 'session', game: title, detail: `${title} failed to start`, meta: e.message?.slice(0, 80) });
        } else {
          const started = sessionStart.current[e.game_id];
          delete sessionStart.current[e.game_id];
          const mins = started ? Math.round((Date.now() - started) / 60000) : 0;
          log({ type: 'session', game: title, detail: `Stopped ${title}`, meta: started ? `${mins} min session` : undefined });
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

  const saveSettings = (id: string, settings: GameSettings, what = 'Launcher settings changed') => {
    const g = games.find((x) => x.id === id);
    setGames((prev) => prev.map((x) => (x.id === id ? { ...x, settings } : x)));
    if (!g) return;
    Effect.runPromise(LauncherApi.saveGameSettings(id, settings))
      .then(() => {
        notify('Settings saved');
        log({ type: 'tweak', game: g.title, detail: `${what} · ${g.title}` });
      })
      .catch((err) => {
        notify(`Not saved to disk, applies this session: ${errMsg(err)}`, 'err');
        log({ type: 'tweak', game: g.title, detail: `${what} · ${g.title}`, meta: 'session only' });
      });
  };

  // The install queue. A finished install, update, uninstall or move changes
  // the library, so it refreshes it: the backend dropped that source from its
  // cache when the job finished, so the refresh re-lists only it.
  const jobStates = useRef(new Map<string, string>());
  const loadRef = useRef(loadData);
  loadRef.current = loadData;
  useEffect(() => {
    const apply = (list: Job[], initial = false) => {
      setJobs(list);
      let refresh = false;
      for (const j of list) {
        const prev = jobStates.current.get(j.id);
        jobStates.current.set(j.id, j.state);
        if (initial || prev === j.state) continue;
        const verb = j.kind === 'uninstall' ? 'Uninstalled' : j.kind === 'update' ? 'Updated' : 'Installed';
        if (j.state === 'done') {
          refresh = true;
          // Optimistic flip so the game page updates before the refresh
          // round-trip (FE-4); the refresh reconciles afterwards.
          setGames((prev) => optimisticGamesAfterJob(prev, j));
          notify(`${verb} ${j.title}`);
          log({ type: 'download', game: j.title, detail: `${verb} ${j.title}`, meta: j.source });
        } else if (j.state === 'failed') {
          notify(`${j.title}: ${j.error ?? `${j.kind} failed`}`, 'err');
          log({ type: 'download', game: j.title, detail: `${j.kind} of ${j.title} failed`, meta: j.error });
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

  const runJob = (kind: 'install' | 'update' | 'uninstall', game: Game) => {
    const op = kind === 'install' ? LauncherApi.installGame : kind === 'update' ? LauncherApi.updateGame : LauncherApi.uninstallGame;
    const queue = kind === 'uninstall' ? Promise.resolve(true) : ensureTerms(game);
    queue
      .then((ok) => ok && Effect.runPromise(op(game))
        .then(() => notify(`${kind === 'install' ? 'Install' : kind === 'update' ? 'Update' : 'Uninstall'} of ${game.title} queued`, 'busy')))
      .catch((err) => notify(errMsg(err), 'err'));
  };

  const jobAction = (action: 'cancel' | 'retry' | 'pause' | 'resume' | 'discard', id: string) => {
    const call =
      action === 'cancel' ? LauncherApi.cancelJob
      : action === 'retry' ? LauncherApi.retryJob
      : action === 'pause' ? LauncherApi.pauseJob
      : action === 'resume' ? LauncherApi.resumeJob
      : LauncherApi.discardJob;
    Effect.runPromise(call(id)).catch((err) => notify(errMsg(err), 'err'));
  };

  const allowFolder = (game: Game, folder: string) => {
    Effect.runPromise(LauncherApi.allowGameFolder(game.source, folder))
      .then(() => {
        notify(`Games in ${folder} can be launched now`);
        loadData('update');
      })
      .catch((err) => notify(errMsg(err), 'err'));
  };

  const addGame = (title: string, executable: string) => {
    Effect.runPromise(LauncherApi.addCustomGame(title, executable))
      .then((game) => {
        // Sideloaded games lead the library, like before, with their saved settings.
        setGames((prev) => [game, ...prev.filter((x) => x.id !== game.id)]);
        setAddOpen(false);
        notify(`Added "${title}"`);
        log({ type: 'add', game: title, detail: `Sideloaded ${title}`, meta: game.executable });
      })
      .catch((err) => notify(errMsg(err), 'err'));
  };

  const installedGames = useMemo(() => games.filter((g) => g.is_installed), [games]);
  const storeCounts = useMemo(
    () => installedGames.reduce<Record<string, number>>((acc, g) => ((acc[g.store] = (acc[g.store] ?? 0) + 1), acc), {}),
    [installedGames]
  );
  const activeJobs = jobs.filter((j) => j.state === 'queued' || j.state === 'running');
  const jobFor = (id: string) => activeJobs.find((j) => j.game_id === id) ?? [...jobs].reverse().find((j) => j.game_id === id && j.state === 'failed');

  const socialLabels = useMemo(() => Object.fromEntries(plugins.map((p) => [p.id, p.name])), [plugins]);
  // Conversations with friends (chat capability): unread badges, the open chat, alerts.
  const chat = useChat(social, notify, socialLabels);
  // Redeem is one entry for every ready store that offers it (none: no entry).
  const redeemStores = useMemo(() => plugins.filter((p) => p.status === 'ready' && offers(p, 'redeem')), [plugins]);
  const libraryIds = useMemo(() => new Set(games.map((g) => g.id)), [games]);

  // Stable, so the library's memoized cards are not re-rendered by every App update.
  const openGame = useCallback(
    (id: string) => setView((v) => ({ name: 'game', id, from: v.name === 'library' || v.name === 'fixes' ? v.name : 'dashboard' })),
    [],
  );
  // Closing the friends modal ends any conversation in it: the chat only shows inside the modal.
  const closeFriends = () => { chat.close(); setFriendsOpen(false); };

  // Dismissing the migration notice persists it in the core (it never
  // returns) and hides the banner at once, without waiting for a relaunch.
  const dismissMigration = () => {
    setMigration(null);
    Effect.runPromise(LauncherApi.dismissMigrationNotice()).catch((err) => notify(errMsg(err), 'err'));
  };

  const currentGame = view.name === 'game' ? games.find((g) => g.id === view.id) : undefined;

  const renderView = () => {
    if (!loaded) return <LibrarySkeleton />;
    if (loadError && games.length === 0 && (view.name === 'dashboard' || view.name === 'library')) {
      return (
        <div className="p-7">
          <EmptyState
            label="backend unreachable"
            action={<Button variant="primary" size="sm" onClick={() => loadData('rescan')}>Retry</Button>}
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
        return (
          <LibraryView
            games={games}
            runners={runners}
            activity={activity}
            search={search}
            onSearch={setSearch}
            source={source}
            onSource={setSource}
            mode={libraryMode}
            onMode={setLibraryMode}
            ownership={ownership}
            onOwnership={setOwnership}
            warnings={warnings}
            plugins={plugins}
            jobs={activeJobs}
            lastScan={lastScan}
            onOpenGame={openGame}
            onAddGame={() => setAddOpen(true)}
            onNavigate={setView}
          />
        );
      case 'game':
        return currentGame ? (
          <GamePage
            game={currentGame}
            runners={runners}
            activity={activity}
            backLabel={view.from}
            onBack={() => setView({ name: view.from } as View)}
            onPlay={() => launch(currentGame)}
            onStop={() => stop(currentGame.id, false)}
            onForceStop={() => stop(currentGame.id, true)}
            launching={launchingId === currentGame.id}
            session={sessions[currentGame.id]}
            refreshing={scanning}
            tweaks={tweaks}
            onChangeSettings={(st, what) => saveSettings(currentGame.id, st, what)}
            onSaveSettings={(st) => saveSettings(currentGame.id, st)}
            onCustomized={() => refreshLibraryQuietly(mergeCustomization)}
            plugin={plugins.find((x) => x.id === currentGame.source)}
            job={jobFor(currentGame.id)}
            jobs={jobs}
            onJob={(kind) => runJob(kind, currentGame)}
            onJobAction={jobAction}
            onAllowFolder={(folder) => allowFolder(currentGame, folder)}
            onNavigate={setView}
            onNotify={notify}
            onVersionsChanged={() => loadData('update')}
            onCopyChanged={() => refreshLibraryQuietly(mergeCopies)}
          />
        ) : (
          <div className="p-7"><EmptyState label="not found">This game is no longer in the library. A rescan may have removed it.</EmptyState></div>
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
                  notify(errMsg(err), 'err');
                  Effect.runPromise(LauncherApi.listJobs()).then(setJobs).catch(() => {});
                });
            }}
            onOpenGame={(id) => games.some((g) => g.id === id) && openGame(id)}
          />
        );
      case 'settings':
        return <LauncherSettingsView onNotify={notify} plugins={plugins} />;
    }
  };

  return (
    <TooltipProvider delayDuration={300}>
    <div className={cx('flex h-screen overflow-hidden text-[#ece6da]', view.name === 'game' ? 'app-wash-game' : 'app-wash')}>
      <Sidebar
        view={view}
        onNavigate={setView}
        search={search}
        onSearch={setSearch}
        storeCounts={storeCounts}
        onPickSource={(s) => { setSource(s); setView({ name: 'library' }); }}
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
      />

      <main ref={main} data-scroll-root className="min-w-0 flex-1 overflow-y-auto">
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

      {addOpen && <AddGameModal onClose={() => setAddOpen(false)} onAdd={addGame} />}

      <CloudSavesHost games={games} onNotify={notify} onLog={log} />
      <FamilyWarningHost games={games} />
      <TermsHost />

      {redeemOpen && redeemStores.length > 0 && <RedeemDialog stores={redeemStores} onClose={() => setRedeemOpen(false)} />}

      {friendsOpen && (
        <FriendsModal
          views={social}
          labels={socialLabels}
          plugins={plugins}
          inLibrary={(id) => libraryIds.has(id)}
          onClose={closeFriends}
          onSetEnabled={(plugin, enabled) =>
            Effect.runPromise(LauncherApi.setSocialEnabled(plugin, enabled)).then(refreshSocial).catch((err) => notify(errMsg(err), 'err'))}
          onSetStatus={(plugin, state: SocialStatus) =>
            Effect.runPromise(LauncherApi.setSocialStatus(plugin, state)).catch((err) => notify(errMsg(err), 'err'))}
          onSetPlaying={(plugin, show) =>
            Effect.runPromise(LauncherApi.setSocialPlaying(plugin, show)).then(refreshSocial).catch((err) => notify(errMsg(err), 'err'))}
          onReconnect={(plugin) =>
            Effect.runPromise(LauncherApi.restartSocial(plugin)).then(refreshSocial).catch((err) => notify(errMsg(err), 'err'))}
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

      {notice && (
        <div
          role="status"
          title={notice.msg}
          className={cx(
            // z-[60]: above the modal overlay (z-50) — a toast (e.g. a save
            // or scan failure) must stay visible over an open dialog and not
            // rely on DOM order. Shape unchanged (DESIGN.md §6.6).
            'fixed bottom-4 right-4 z-[60] flex max-w-md items-center gap-2.5 rounded-[3px] border bg-[#191410] px-3 py-2 font-mono text-[12px]',
            NOTICE_STYLE[notice.kind]
          )}
          style={{ boxShadow: '0 10px 30px rgba(0,0,0,.5)' }}
        >
          <span className={cx('h-1.5 w-1.5 flex-none rounded-full', NOTICE_DOT[notice.kind])} />
          <div className="min-w-0 space-y-1">
            <span title={notice.msg} className="block break-words line-clamp-3">{notice.msg}</span>
            {notice.raw && <RawDetails text={notice.raw} />}
          </div>
        </div>
      )}
    </div>
    </TooltipProvider>
  );
}

export default App;
