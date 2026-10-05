import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Effect } from 'effect';
import { Sidebar } from './components/Sidebar';
import { Dashboard } from './components/Dashboard';
import { LibraryView, type LibraryMode } from './components/LibraryView';
import { GamePage } from './components/GamePage';
import { SettingsModal } from './components/SettingsModal';
import { AddGameModal } from './components/AddGameModal';
import { ActivityView } from './components/ActivityView';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { SystemView } from './components/SystemView';
import { LauncherSettingsView } from './components/LauncherSettingsView';
import { cx, EmptyState, btnPrimary } from './components/ui';
import { LauncherApi, onSession } from './effect/runtime';
import { needsAttention } from './lib/games';
import type { ActivityEvent, Game, GameSettings, PluginInfo, ProtonVersion, SessionEvent, StoreType, Tweak, View } from './types';

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
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [lastScan, setLastScan] = useState<number | null>(null);
  const [scanning, setScanning] = useState(false);

  const [view, setView] = useState<View>({ name: 'dashboard' });
  const [search, setSearch] = useState('');
  const [source, setSource] = useState<StoreType | 'all'>('all');
  const [libraryMode, setLibraryMode] = useState<LibraryMode>('cards');

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

  const loadData = useCallback(
    (reason: 'startup' | 'rescan') => {
      setScanning(true);
      const program = Effect.all([
        LauncherApi.getGames(),
        LauncherApi.getProtonVersions(),
        LauncherApi.getPlugins(),
        // Unreadable saved settings shouldn't block the library.
        LauncherApi.getGameSettings().pipe(Effect.orElseSucceed(() => ({}) as Record<string, GameSettings>)),
      ]);
      return Effect.runPromise(program)
        .then(([g, r, p, saved]) => {
          // Keep session-only additions and edited settings across rescans;
          // on startup, settings come from disk.
          setGames((prev) => {
            const edited = new Map(prev.map((x) => [x.id, x.settings]));
            const custom = prev.filter((x) => x.store === 'custom');
            return [
              ...custom,
              ...g.map((x) => {
                const s = edited.get(x.id) ?? (saved[x.id] ? { ...x.settings, ...saved[x.id] } : undefined);
                return s ? { ...x, settings: s } : x;
              }),
            ];
          });
          setRunners(r);
          setPlugins(p);
          setLoadError(null);
          setLastScan(Date.now());
          const stores = new Set(g.map((x) => x.store)).size;
          log({ type: 'update', detail: `Library scan · ${g.length} titles`, meta: `${stores} source${stores === 1 ? '' : 's'} · ${r.length} runners` });
          if (reason === 'rescan') notify('Library rescanned');
        })
        .catch((err) => {
          const msg = errMsg(err);
          setLoadError(msg);
          notify(msg.slice(0, 90), 'err');
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
    const todo = games.filter((g) => !g.fix_info && g.store !== 'custom' && !fixFetched.current.has(g.id));
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
  const sessionStart = useRef<Record<string, number>>({});
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

  const saveSettings = (id: string, settings: GameSettings, what = 'Launcher settings changed') => {
    const g = games.find((x) => x.id === id);
    setGames((prev) => prev.map((x) => (x.id === id ? { ...x, settings } : x)));
    setSettingsFor(null);
    // Sideloaded games aren't saved yet, so neither are their settings.
    if (!g || g.store === 'custom') {
      notify('Settings saved for this session');
      if (g) log({ type: 'tweak', game: g.title, detail: `${what} · ${g.title}`, meta: 'session only' });
      return;
    }
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

  const togglePlugin = (id: string, enabled: boolean) => {
    const program = Effect.gen(function* () {
      yield* LauncherApi.togglePlugin(id, enabled);
      return yield* LauncherApi.getPlugins();
    });
    Effect.runPromise(program)
      .then((p) => { setPlugins(p); notify('Plugin updated'); })
      .catch((err) => notify(`Plugin toggle failed: ${errMsg(err)}`, 'err'));
  };

  const addGame = (title: string, executable: string) => {
    const game: Game = {
      id: 'custom_' + Date.now(),
      store: 'custom',
      store_app_id: 'custom',
      title,
      cover_url: '',
      banner_url: '',
      install_dir: '',
      executable,
      is_installed: true,
      settings: {
        runner_id: 'auto',
        wine_prefix: '',
        launch_arguments: '',
        env_vars: {},
        use_gamemode: true,
        use_mangohud: false,
        use_gamescope: false,
        gamescope_w: 1920,
        gamescope_h: 1080,
        gamescope_r: 144,
        gamescope_args: '',
        eac_runtime: false,
        battleye_runtime: false,
        working_dir: '',
      },
    };
    setGames((prev) => [game, ...prev]);
    setAddOpen(false);
    notify(`Added "${title}" (session only)`);
    log({ type: 'add', game: title, detail: `Sideloaded ${title}`, meta: 'session only' });
  };

  const storeCounts = useMemo(
    () => games.reduce<Record<string, number>>((acc, g) => ((acc[g.store] = (acc[g.store] ?? 0) + 1), acc), {}),
    [games]
  );

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
            games={games}
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
            tweaks={tweaks}
            onChangeSettings={(st, what) => saveSettings(currentGame.id, st, what)}
            onOpenSettings={() => setSettingsFor(currentGame.id)}
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
        return <PluginsView plugins={plugins} onToggle={togglePlugin} onRefresh={() => loadData('rescan')} />;
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
        gameCount={games.length}
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
          className={cx(
            'fixed bottom-4 right-4 z-[60] flex max-w-md items-center gap-2.5 rounded-xl border bg-[#191410] px-3 py-2 font-mono text-[11px]',
            NOTICE_STYLE[notice.kind]
          )}
          style={{ boxShadow: '0 10px 30px rgba(0,0,0,.5)' }}
        >
          <span className={cx('h-1.5 w-1.5 flex-none rounded-full', NOTICE_DOT[notice.kind])} />
          <span className="truncate">{notice.msg}</span>
        </div>
      )}
    </div>
  );
}

export default App;
