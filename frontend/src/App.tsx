import React, { useState, useEffect } from 'react';
import { Effect } from 'effect';
import { Search, Plus, Filter, RefreshCw, LayoutGrid, List } from 'lucide-react';
import { Sidebar } from './components/Sidebar';
import { GameCard } from './components/GameCard';
import { GameModal } from './components/GameModal';
import { PluginsView } from './components/PluginsView';
import { FixesView } from './components/FixesView';
import { SystemView } from './components/SystemView';
import { LauncherApi } from './effect/runtime';
import type { Game, ProtonVersion, PluginInfo, StoreType } from './types';

export function App() {
  const [games, setGames] = useState<Game[]>([]);
  const [protonVersions, setProtonVersions] = useState<ProtonVersion[]>([]);
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);
  
  const [currentTab, setCurrentTab] = useState<'library' | 'plugins' | 'system' | 'fixes'>('library');
  const [selectedStore, setSelectedStore] = useState<StoreType | 'all'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  
  const [selectedGame, setSelectedGame] = useState<Game | null>(null);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [notification, setNotification] = useState<string | null>(null);

  const showNotification = (msg: string) => {
    setNotification(msg);
    setTimeout(() => setNotification(null), 3500);
  };

  const loadData = () => {
    const program = Effect.all([
      LauncherApi.getGames(),
      LauncherApi.getProtonVersions(),
      LauncherApi.getPlugins(),
    ]);

    Effect.runPromise(program).then(([loadedGames, loadedProtons, loadedPlugins]) => {
      setGames(loadedGames);
      setProtonVersions(loadedProtons);
      setPlugins(loadedPlugins);
    }).catch((err) => {
      console.error(err);
      showNotification('Error loading launcher data');
    });
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleLaunchGame = (game: Game) => {
    Effect.runPromise(LauncherApi.launchGame(game))
      .then((res: string) => showNotification(res))
      .catch((err: any) => showNotification(String(err)));
  };

  const handleTogglePlugin = (id: string, enabled: boolean) => {
    const program = Effect.gen(function* () {
      yield* LauncherApi.togglePlugin(id, enabled);
      return yield* LauncherApi.getPlugins();
    });

    Effect.runPromise(program).then((updated: PluginInfo[]) => {
      setPlugins(updated);
      showNotification(`Plugin updated`);
    });
  };

  const handleAddCustomGame = () => {
    const title = prompt('Enter Game Title:');
    if (!title) return;
    const exe = prompt('Enter Executable Path (e.g. /home/.../game.exe):') || '';

    const newGame: Game = {
      id: 'custom_' + Date.now(),
      store: 'custom',
      store_app_id: 'custom',
      title,
      cover_url: 'https://images.unsplash.com/photo-1542751371-adc38448a05e?auto=format&fit=crop&w=600&q=80',
      banner_url: 'https://images.unsplash.com/photo-1542751371-adc38448a05e?auto=format&fit=crop&w=1200&q=80',
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
    showNotification(`Added custom game "${title}"`);
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

  return (
    <div className="flex h-screen bg-zinc-950 text-zinc-100 overflow-hidden select-none">
      {/* Sidebar */}
      <Sidebar
        currentTab={currentTab}
        setCurrentTab={setCurrentTab}
        selectedStore={selectedStore}
        setSelectedStore={setSelectedStore}
        storeCounts={storeCounts}
        onAddCustomGame={handleAddCustomGame}
      />

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col overflow-hidden bg-gradient-to-b from-zinc-900/40 to-zinc-950">
        {/* Top Header Bar */}
        <header className="h-16 border-b border-zinc-800/80 px-8 flex items-center justify-between backdrop-blur-md bg-zinc-900/30">
          <div className="relative w-96">
            <Search className="w-4 h-4 absolute left-3 top-3 text-zinc-400" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search games, launchers, runners..."
              className="w-full bg-zinc-900/90 border border-zinc-700/60 rounded-xl pl-9 pr-4 py-2 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-purple-500/80 transition-colors"
            />
          </div>

          <div className="flex items-center gap-4">
            <button
              onClick={loadData}
              className="p-2 rounded-lg bg-zinc-800/80 hover:bg-zinc-700 text-zinc-300 transition-colors"
              title="Rescan Local Libraries"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
          </div>
        </header>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto">
          {currentTab === 'library' && (
            <div className="p-8">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h2 className="text-xl font-bold text-white tracking-tight">
                    {selectedStore === 'all' ? 'All Installed Games' : `${selectedStore.toUpperCase()} Titles`}
                  </h2>
                  <p className="text-xs text-zinc-400 mt-0.5">
                    Showing {filteredGames.length} available titles ready to play on Linux
                  </p>
                </div>
              </div>

              {/* Game Cards Grid */}
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-5">
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
                    onQuickLaunch={(g, e) => {
                      e.stopPropagation();
                      handleLaunchGame(g);
                    }}
                  />
                ))}
              </div>
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

      {/* Game Config Modal */}
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

      {/* Global Notification Toast */}
      {notification && (
        <div className="fixed bottom-6 right-6 z-50 px-4 py-2.5 rounded-xl bg-purple-600 text-white text-sm font-medium shadow-xl shadow-purple-600/30 animate-in slide-in-from-bottom duration-200">
          {notification}
        </div>
      )}
    </div>
  );
}

export default App;
