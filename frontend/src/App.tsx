import React, { useState, useEffect, useRef, useCallback } from 'react';
import { RefreshCw, Search, Copy, Check, Terminal, Loader2 } from 'lucide-react';
import { Effect } from 'effect';
import { Sidebar } from './components/Sidebar';
import { GameCard } from './components/GameCard';
import { GameModal } from './components/GameModal';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { SystemView } from './components/SystemView';
import { LauncherApi } from './effect/runtime';
import type { Game, ProtonVersion, PluginInfo, StoreType } from './types';

type NotificationKind = 'ok' | 'err' | 'busy';
interface Notice { msg: string; kind: NotificationKind; }

const NOTIF_STYLE: Record<NotificationKind, string> = {
  ok: 'border-[#7dd48a]/60 text-[#cfe8d3]',
  err: 'border-[#e05d5d]/60 text-[#f0c9c9]',
  busy: 'border-[#ffb000]/60 text-[#ffdf9e]',
};

const DOT_STYLE: Record<NotificationKind, string> = {
  ok: 'bg-[#7dd48a]',
  err: 'bg-[#e05d5d]',
  busy: 'bg-[#ffb000]',
};

export function App() {
  const [games, setGames] = useState<Game[]>([]);
  const [protonVersions, setProtonVersions] = useState<ProtonVersion[]>([]);
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);

  const [currentTab, setCurrentTab] = useState<'library' | 'plugins' | 'system' | 'fixes'>('library');
  const [selectedStore, setSelectedStore] = useState<StoreType | 'all'>('all');
  const [searchQuery, setSearchQuery] = useState('');

  const [selectedGame, setSelectedGame] = useState<Game | null>(null);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [notification, setNotification] = useState<Notice | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [launchingId, setLaunchingId] = useState<string | null>(null);
  const [sessionPid, setSessionPid] = useState<number | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [scanPending, setScanPending] = useState(false);

  const notificationTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const showNotification = useCallback((msg: string, kind: NotificationKind = 'ok') => {
    setNotification({ msg, kind });
    if (notificationTimer.current) clearTimeout(notificationTimer.current);
    notificationTimer.current = setTimeout(() => setNotification(null), 3500);
  }, []);
  useEffect(() => () => {
    if (notificationTimer.current) clearTimeout(notificationTimer.current);
  }, []);

  const loadData = useCallback(() => {
    const program = Effect.all([
      LauncherApi.getGames(),
      LauncherApi.getProtonVersions(),
      LauncherApi.getPlugins(),
    ]);

    Effect.runPromise(program).then(([loadedGames, loadedProtons, loadedPlugins]) => {
      setGames(loadedGames);
      setProtonVersions(loadedProtons);
      setPlugins(loadedPlugins);
      setLoadError(null);
    }).catch((err) => {
      console.error(err);
      const msg = err instanceof Error ? err.message : String(err);
      setLoadError(msg);
      showNotification(msg.slice(0, 90), 'err');
    });
  }, [showNotification]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleLaunchGame = (game: Game) => {
    if (launchingId) return;
    setLaunchingId(game.id);
    Effect.runPromise(LauncherApi.launchGame(game))
      .then((res: string) => {
        showNotification(res || `Launching ${game.title}`, 'busy');
        const pidMatch = res?.match(/PID (\d+)/);
        if (pidMatch) {
          setSessionPid(parseInt(pidMatch[1], 10));
        } else {
          setSessionPid(-1); // in session, pid unknown
        }
      })
      .catch((err: unknown) => showNotification(err instanceof Error ? err.message : String(err), 'err'))
      .finally(() => setLaunchingId(null));
  };

  const handleTogglePlugin = (id: string, enabled: boolean) => {
    const program = Effect.gen(function* () {
      yield* LauncherApi.togglePlugin(id, enabled);
      return yield* LauncherApi.getPlugins();
    });

    Effect.runPromise(program).then((updated: PluginInfo[]) => {
      setPlugins(updated);
      showNotification('Plugin updated');
    }).catch((err: unknown) => {
      showNotification(`Plugin toggle failed: ${err instanceof Error ? err.message : String(err)}`, 'err');
    });
  };

  const handleAddCustomGame = () => {
    const title = prompt('Game title:');
    if (!title) return;
    const exe = prompt('Executable path (e.g. /home/.../game.exe):') || '';

    const newGame: Game = {
      id: 'custom_' + Date.now(),
      store: 'custom',
      store_app_id: 'custom',
      title,
      cover_url: '',
      banner_url: '',
      install_dir: '',
      executable: exe,
      is_installed: true,
      settings: {
        runner_id: protonVersions[0]?.id || 'auto',
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

    setGames([newGame, ...games]);
    showNotification(`Sideloaded "${title}" (session only)`);
  };

  const handleCopyCommand = (game: Game, e: React.MouseEvent) => {
    e.stopPropagation();
    Effect.runPromise(LauncherApi.previewCommand(game))
      .then((cmd) => navigator.clipboard.writeText(cmd))
      .then(() => {
        setCopiedId(game.id);
        setTimeout(() => setCopiedId(null), 1500);
        showNotification('Launch command copied');
      })
      .catch((err: unknown) => showNotification(err instanceof Error ? err.message : String(err), 'err'));
  };

  const storeCounts = games.reduce((acc, g) => {
    acc['all'] = (acc['all'] || 0) + 1;
    acc[g.store] = (acc[g.store] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  const filteredGames = games.filter((g) => {
    const matchesStore = selectedStore === 'all' || g.store === selectedStore;
    const matchesSearch = g.title.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesStore && matchesSearch;
  });

  const handleRescan = () => {
    setScanPending(true);
    setTimeout(() => {
      loadData();
      setScanPending(false);
      showNotification('Library rescanned');
    }, 400);
  };

  return (
    <div
      className="flex h-screen text-[#e8e6e3] overflow-hidden select-none bg-[#121417]"
      style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}
    >
      <Sidebar
        currentTab={currentTab}
        setCurrentTab={setCurrentTab}
        selectedStore={selectedStore}
        setSelectedStore={setSelectedStore}
        storeCounts={storeCounts}
        onAddCustomGame={handleAddCustomGame}
        sessionPid={sessionPid}
        gameCount={games.length}
      />

      {/* Main column */}
      <main className="flex-1 flex flex-col overflow-hidden bg-[#121417]">
        {/* Instrument header */}
        <header className="h-14 flex-none border-b border-[#2a2d31] bg-[#16181b] px-5 flex items-center justify-between gap-6">
          <div className="relative w-[26rem] max-w-[36vw]">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-[#767c82]" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="filter library…"
              className="w-full bg-[#1d2024] border border-[#2e3238] rounded-[3px] pl-8 pr-3 py-1.5 text-[13px] text-[#e8e6e3] placeholder-[#767c82] focus:outline-none focus:border-[#ffb000]/70 transition-colors"
              style={{ fontFamily: 'Hack, monospace' }}
            />
          </div>

          <div className="flex items-center gap-3 font-mono text-[10px] text-[#767c82] tracking-[0.1em]" style={{ fontFamily: 'Hack, monospace' }}>
            <span className="hidden lg:flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5" />
              {protonVersions.length} RUNNERS
            </span>
            <button
              onClick={handleRescan}
              disabled={scanPending}
              className="flex items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-[#c9cdd1] bg-[#1d2024] border border-[#2e3238] hover:border-[#ffb000]/70 hover:text-[#ffb000] transition-colors disabled:opacity-50 disabled:cursor-wait rounded-[3px]"
            >
              {scanPending
                ? <Loader2 className="w-3 h-3 animate-spin" />
                : <RefreshCw className="w-3 h-3" strokeWidth={1.75} />}
              {scanPending ? 'SCANNING' : 'RESCAN'}
            </button>
          </div>
        </header>

        {/* Tab content */}
        <div className="flex-1 overflow-y-auto hydra-scroll">
          {currentTab === 'library' && (
            <div className="px-6 py-5">
              <header className="flex items-baseline justify-between mb-4 border-b border-[#2a2d31] pb-3">
                <h2
                  className="text-[15px] font-bold tracking-[0.14em] text-[#f2f1ee] uppercase"
                  style={{ fontFamily: 'Hack, monospace' }}
                >
                  {selectedStore === 'all' ? 'Library' : selectedStore}
                  <span className="text-[#ffb000] ml-3 font-normal">
                    {String(filteredGames.length).padStart(3, '0')}
                  </span>
                </h2>
                <p className="font-mono text-[10px] text-[#767c82] tracking-wide" style={{ fontFamily: 'Hack, monospace' }}>
                  {searchQuery ? `filtered · "${searchQuery}"` : 'all installed titles · linux ready'}
                </p>
              </header>

              {loadError ? (
                <div className="mt-16 mx-auto max-w-md text-center border border-[#e05d5d]/40 bg-[#e05d5d]/5 px-6 py-8">
                  <p className="font-mono text-[11px] tracking-widest text-[#e05d5d] uppercase" style={{ fontFamily: 'Hack, monospace' }}>backend unreachable</p>
                  <p className="text-[13px] text-[#c9cdd1] mt-3 leading-relaxed">{loadError}</p>
                  <button
                    onClick={loadData}
                    className="mt-5 px-4 py-1.5 font-mono text-[11px] text-[#ffb000] border border-[#ffb000]/50 hover:bg-[#ffb000] hover:text-[#101214] transition-colors"
                  >
                    RETRY
                  </button>
                </div>
              ) : filteredGames.length === 0 ? (
                <div className="mt-16 mx-auto max-w-md text-center border border-[#2e3238] bg-[#16181b] px-6 py-10">
                  <p className="font-mono text-[11px] tracking-widest text-[#767c82] uppercase" style={{ fontFamily: 'Hack, monospace' }}>no titles found</p>
                  <p className="text-[13px] text-[#9aa0a6] mt-3">Adjust the filter, or rescan your library sources.</p>
                </div>
              ) : (
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-7 gap-2.5">
                  {filteredGames.map((game) => (
                    <GameCard
                      key={game.id}
                      game={game}
                      onSelect={(g) => {
                        setSelectedGame(g);
                        setIsSettingsOpen(true);
                      }}
                      onOpenSettings={(g) => {
                        setSelectedGame(g);
                        setIsSettingsOpen(true);
                      }}
                      onQuickLaunch={handleLaunchGame}
                      copiedId={copiedId}
                      onCopyCommand={handleCopyCommand}
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          {currentTab === 'fixes' && (
            <FixesView
              games={games}
              onOpenSettings={(g) => {
                setSelectedGame(g);
                setIsSettingsOpen(true);
              }}
            />
          )}

          {currentTab === 'plugins' && (
            <PluginsView
              plugins={plugins}
              onToggle={handleTogglePlugin}
              onRefresh={loadData}
            />
          )}

          {currentTab === 'system' && (
            <SystemView
              runners={protonVersions}
              onRefresh={loadData}
            />
          )}
        </div>
      </main>

      {/* Game config modal */}
      {isSettingsOpen && selectedGame && (
        <GameModal
          game={selectedGame}
          protonVersions={protonVersions}
          onClose={() => {
            setIsSettingsOpen(false);
            setSelectedGame(null);
          }}
          onSave={(updated) => {
            setGames(games.map((g) => (g.id === updated.id ? updated : g)));
            showNotification(`Updated ${updated.title} settings`);
          }}
          onLaunch={(updated) => {
            handleLaunchGame(updated);
            setIsSettingsOpen(false);
          }}
        />
      )}

      {/* Notification toast — instrument readout */}
      {notification && (
        <div
          className={`fixed bottom-4 right-4 z-50 px-3 py-2 bg-[#16181b] border rounded-[3px] font-mono text-[11px] flex items-center gap-2.5 max-w-md ${NOTIF_STYLE[notification.kind]}`}
          style={{ fontFamily: 'Hack, monospace', boxShadow: '0 6px 18px rgba(0,0,0,.45), 0 2px 6px rgba(0,0,0,.35)' }}
          role="status"
        >
          <span className={`w-1.5 h-1.5 flex-none rounded-full ${DOT_STYLE[notification.kind]}`} />
          <span className="truncate">{notification.msg}</span>
        </div>
      )}
    </div>
  );
}

export default App;