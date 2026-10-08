import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Effect } from 'effect';
import { Sidebar } from './components/Sidebar';
import { Dashboard } from './components/Dashboard';
import { LibraryView, type LibraryMode, type Ownership } from './components/LibraryView';
import { GamePage, invalidateGameDetailsCache } from './components/GamePage';
import { SettingsModal } from './components/SettingsModal';
import { AddGameModal } from './components/AddGameModal';
import { ActivityView } from './components/ActivityView';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { SystemView } from './components/SystemView';
import { LauncherSettingsView } from './components/LauncherSettingsView';
import { DownloadsView } from './components/DownloadsView';
import { cx, EmptyState, btnPrimary } from './components/ui';
import { LauncherApi, onJobs, onSession } from './effect/runtime';
import { optimisticGamesAfterJob, withoutFinishedJobs } from './lib/jobs';
import { needsAttention, mergeRescannedGames } from './lib/games';
import type { ActivityEvent, Game, GameSettings, Job, ProtonVersion, SessionEvent, SourceWarning, StorePlugin, StoreType, Tweak, View } from './types';

type NoticeKind = 'ok' | 'err' | 'busy';
interface Notice { msg: string; kind: NoticeKind }

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

  const [settingsFor, setSettingsFor] = useState<string | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [launchingId, setLaunchingId] = useState<string | null>(null);
  // Games Hydra launched that are starting or running, keyed by game id.
  const [sessions, setSessions] = useState<Record<string, SessionEvent>>({});
  const [tweaks, setTweaks] = useState<Tweak[]>([]);
  const [dismissedHero, setDismissedHero] = useState<string | null>(null);

  // Activity is real session events only (scans, launches, setting changes).
  const [activity, setActivity] = useState<ActivityEvent[]>([]);
  const eventId = useRef(0);
  const log = useCallback((e: Omit<ActivityEvent, 'id' | 'ts'>) => {
    setActivity((prev) => [{ ...e, id: ++eventId.current, ts: Date.now() }, ...prev].slice(0, 200));
  }, []);

  const [notice, setNotice] = useState<Notice | null>(null);
  const noticeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const notify = useCallback((msg: string, kind: NoticeKind = 'ok') => {
    setNotice({ msg, kind });
    if (noticeTimer.current) clearTimeout(noticeTimer.current);
    noticeTimer.current = setTimeout(() => setNotice(null), 3500);
  }, []);
  useEffect(() => () => { if (noticeTimer.current) clearTimeout(noticeTimer.current); }, []);

  const main = useRef<HTMLElement>(null);
  useEffect(() => { main.current?.scrollTo({ top: 0 }); }, [view]);

  // When a session started before this window mounted (CORE-1).
  const sessionStart = useRef<Record<string, number>>({});
  const loadData = useCallback(
    (reason: 'startup' | 'rescan') => {
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
      return Promise.all([
        settle(LauncherApi.getLibrary()),
        settle(LauncherApi.getProtonVersions()),
        settle(LauncherApi.listPlugins()),
        // Unreadable saved settings shouldn't block the library.
        settle(LauncherApi.getGameSettings()),
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
            for (const w of lib.warnings) log({ type: 'update', detail: `${w.source}: ${w.needs_auth ? 'logged out' : 'library unavailable'}`, meta: w.message });
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
          } else {
            failures.push(`Plugins: ${errMsg((pR as { ok: false; error: Error }).error)}`);
          }
          // Hydrate sessions so a window opened after a launch shows Playing (CORE-1/FE-2).
          const live = sessR.ok ? (sessR as { ok: true; value: SessionEvent[] }).value : [];
          if (live.length > 0) {
            const seeded: Record<string, SessionEvent> = {};
            const now = Date.now();
            for (const e of live) {
              if (e.state === 'starting' || e.state === 'running') {
                seeded[e.game_id] = e;
                if (e.state === 'running' && sessionStart.current[e.game_id] === undefined) {
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
          // Rescans can change store metadata (login, reinstall): evict the
          // details cache so the next visit refetches (FE-6).
          if (reason === 'rescan') invalidateGameDetailsCache();
          setLastScan(Date.now());
        })
        .catch((err) => {
          // Unreachable: every call is guarded. Kept as a safety net.
          const msg = errMsg(err);
          setLoadError(msg);
          notify(msg, 'err');
        })
        .finally(() => {
          setLoaded(true);
          setScanning(false);
        });
    },
    [log, notify]
  );

  // StrictMode mounts effects twice in dev; scan only once on startup.
  const started = useRef(false);
  useEffect(() => {
    Effect.runPromise(LauncherApi.getTweaks()).then(setTweaks).catch(() => {});
  }, []);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    loadData('startup');
  }, [loadData]);

  // Enrich with fix/tier data (curated first, then ProtonDB) in the background.
  // Each game is fetched at most once per session.
  const fixFetched = useRef(new Set<string>());
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
      if (found.size === 0) return;
      setGames((prev) => prev.map((g) => (found.has(g.id) && !g.fix_info ? { ...g, fix_info: found.get(g.id) } : g)));
      const curated = [...found.values()].filter((f) => f.source !== 'protondb').length;
      log({ type: 'fix', detail: `Compatibility data for ${found.size} titles`, meta: curated ? `${curated} curated fixes` : 'protondb' });
    });
  }, [games, log]);

  const launch = (game: Game) => {
    if (launchingId || sessions[game.id]) return;
    setLaunchingId(game.id);
    Effect.runPromise(LauncherApi.launchGame(game))
      .then((res) => notify(res || `Launching ${game.title}`, 'busy'))
      .catch((err) => notify(errMsg(err), 'err'))
      .finally(() => setLaunchingId(null));
  };

  // Real session state comes from the backend: for Steam games it watches the
  // game process under Steam's reaper, not the short-lived `steam` command.
  const gamesRef = useRef(games);
  gamesRef.current = games;
  useEffect(
    () =>
      onSession((e) => {
        const title = gamesRef.current.find((g) => g.id === e.game_id)?.title ?? e.game_id;
        if (e.state === 'starting' || e.state === 'running') {
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
            if (e.state === 'starting' || e.state === 'running') {
              seeded[e.game_id] = e;
              if (e.state === 'running' && sessionStart.current[e.game_id] === undefined) {
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
    setSettingsFor(null);
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

  // The install queue. A finished install or uninstall changes the library,
  // so it triggers a rescan.
  const jobStates = useRef(new Map<string, string>());
  const loadRef = useRef(loadData);
  loadRef.current = loadData;
  useEffect(() => {
    const apply = (list: Job[], initial = false) => {
      setJobs(list);
      let rescan = false;
      for (const j of list) {
        const prev = jobStates.current.get(j.id);
        jobStates.current.set(j.id, j.state);
        if (initial || prev === j.state) continue;
        const verb = j.kind === 'uninstall' ? 'Uninstalled' : j.kind === 'update' ? 'Updated' : 'Installed';
        if (j.state === 'done') {
          rescan = true;
          // Optimistic flip so the game page updates before the rescan
          // round-trip (FE-4); the rescan reconciles afterwards.
          setGames((prev) => optimisticGamesAfterJob(prev, j));
          notify(`${verb} ${j.title}`);
          log({ type: 'download', game: j.title, detail: `${verb} ${j.title}`, meta: j.source });
        } else if (j.state === 'failed') {
          notify(`${j.title}: ${j.error ?? `${j.kind} failed`}`, 'err');
          log({ type: 'download', game: j.title, detail: `${j.kind} of ${j.title} failed`, meta: j.error });
        }
      }
      if (rescan) loadRef.current('rescan');
    };
    Effect.runPromise(LauncherApi.listJobs()).then((l) => apply(l, true)).catch(() => {});
    return onJobs((l) => apply(l));
  }, [log, notify]);

  const runJob = (kind: 'install' | 'update' | 'uninstall', game: Game) => {
    const op = kind === 'install' ? LauncherApi.installGame : kind === 'update' ? LauncherApi.updateGame : LauncherApi.uninstallGame;
    Effect.runPromise(op(game))
      .then(() => notify(`${kind === 'install' ? 'Install' : kind === 'update' ? 'Update' : 'Uninstall'} of ${game.title} queued`, 'busy'))
      .catch((err) => notify(errMsg(err), 'err'));
  };

  const jobAction = (action: 'cancel' | 'retry', id: string) => {
    Effect.runPromise(action === 'cancel' ? LauncherApi.cancelJob(id) : LauncherApi.retryJob(id)).catch((err) => notify(errMsg(err), 'err'));
  };

  const allowFolder = (game: Game, folder: string) => {
    Effect.runPromise(LauncherApi.allowGameFolder(game.source, folder))
      .then(() => {
        notify(`Games in ${folder} can be launched now`);
        loadData('rescan');
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

  const openGame = (id: string) =>
    setView((v) => ({ name: 'game', id, from: v.name === 'library' || v.name === 'fixes' ? v.name : 'dashboard' }));

  const currentGame = view.name === 'game' ? games.find((g) => g.id === view.id) : undefined;
  const settingsGame = settingsFor ? games.find((g) => g.id === settingsFor) : undefined;

  const renderView = () => {
    if (!loaded) {
      return <div className="p-7 font-mono text-[11px] uppercase tracking-widest text-white/40">scanning library…</div>;
    }
    if (loadError && games.length === 0 && (view.name === 'dashboard' || view.name === 'library')) {
      return (
        <div className="p-7">
          <EmptyState
            label="backend unreachable"
            action={<button onClick={() => loadData('rescan')} className={cx(btnPrimary, 'px-4 py-2 text-[12.5px]')}>Retry</button>}
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
            runners={runners}
            activity={activity}
            dismissedHero={dismissedHero}
            onDismissHero={setDismissedHero}
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
            launching={launchingId === currentGame.id}
            session={sessions[currentGame.id]}
            refreshing={scanning}
            tweaks={tweaks}
            onChangeSettings={(st, what) => saveSettings(currentGame.id, st, what)}
            onOpenSettings={() => setSettingsFor(currentGame.id)}
            plugin={plugins.find((x) => x.id === currentGame.source)}
            job={jobFor(currentGame.id)}
            onJob={(kind) => runJob(kind, currentGame)}
            onJobAction={jobAction}
            onAllowFolder={(folder) => allowFolder(currentGame, folder)}
            onNavigate={setView}
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
            onPlugins={setPlugins}
            onLibraryChanged={() => loadData('rescan')}
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
        return <LauncherSettingsView onNotify={notify} />;
    }
  };

  return (
    <div className={cx('flex h-screen overflow-hidden text-[#ece6da]', view.name === 'game' ? 'app-wash-game' : 'app-wash')}>
      <Sidebar
        view={view}
        onNavigate={setView}
        search={search}
        onSearch={setSearch}
        storeCounts={storeCounts}
        onPickSource={(s) => { setSource(s); setView({ name: 'library' }); }}
        gameCount={installedGames.length}
        downloadCount={activeJobs.length}
        authNeeded={warnings.some((w) => w.needs_auth) || plugins.some((x) => x.status === 'unapproved' || x.status === 'changed')}
        fixCount={games.filter(needsAttention).length}
        runnerCount={runners.length}
        lastScan={lastScan}
        sessions={Object.values(sessions).map((e) => ({ ...e, title: games.find((g) => g.id === e.game_id)?.title ?? e.game_id }))}
      />

      <main ref={main} className="min-w-0 flex-1 overflow-y-auto">
        {renderView()}
      </main>

      {settingsGame && (
        <SettingsModal
          game={settingsGame}
          runners={runners}
          onClose={() => setSettingsFor(null)}
          onSave={(s) => saveSettings(settingsGame.id, s)}
          onCopied={() => notify('Copied to clipboard')}
        />
      )}

      {addOpen && <AddGameModal onClose={() => setAddOpen(false)} onAdd={addGame} />}

      {notice && (
        <div
          role="status"
          title={notice.msg}
          className={cx(
            // z-[60]: above the modal overlay (z-50) — a toast (e.g. a save
            // or scan failure) must stay visible over an open dialog and not
            // rely on DOM order. Shape unchanged (DESIGN.md §6.6).
            'fixed bottom-4 right-4 z-[60] flex max-w-md items-center gap-2.5 rounded-[3px] border bg-[#191410] px-3 py-2 font-mono text-[11px]',
            NOTICE_STYLE[notice.kind]
          )}
          style={{ boxShadow: '0 10px 30px rgba(0,0,0,.5)' }}
        >
          <span className={cx('h-1.5 w-1.5 flex-none rounded-full', NOTICE_DOT[notice.kind])} />
          <span title={notice.msg} className="break-words line-clamp-3">{notice.msg}</span>
        </div>
      )}
    </div>
  );
}

export default App;
