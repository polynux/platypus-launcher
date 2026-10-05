import React from 'react';
import { 
  Gamepad2, 
  Layers, 
  Settings2, 
  Puzzle, 
  Flame, 
  FolderPlus,
  Compass,
  Cpu
} from 'lucide-react';
import type { StoreType } from '../types';

interface SidebarProps {
  currentTab: 'library' | 'plugins' | 'system' | 'fixes';
  setCurrentTab: (tab: 'library' | 'plugins' | 'system' | 'fixes') => void;
  selectedStore: StoreType | 'all';
  setSelectedStore: (store: StoreType | 'all') => void;
  storeCounts: Record<string, number>;
  onAddCustomGame: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  setCurrentTab,
  selectedStore,
  setSelectedStore,
  storeCounts,
  onAddCustomGame,
}) => {
  return (
    <aside className="w-64 bg-zinc-900/90 backdrop-blur-md border-r border-zinc-800/80 flex flex-col justify-between p-4 h-screen select-none">
      <div className="space-y-6">
        {/* Launcher Title / Branding */}
        <div className="flex items-center gap-3 px-2 py-1">
          <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-purple-600 via-indigo-600 to-pink-500 flex items-center justify-center shadow-lg shadow-purple-500/25">
            <Flame className="w-6 h-6 text-white" />
          </div>
          <div>
            <h1 className="font-bold text-lg text-white tracking-wide flex items-center gap-1.5">
              HYDRA <span className="text-[10px] uppercase font-semibold px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30">Linux</span>
            </h1>
            <p className="text-xs text-zinc-400">Universal Game Center</p>
          </div>
        </div>

        {/* Quick Add Custom Game Action */}
        <button
          onClick={onAddCustomGame}
          className="w-full flex items-center justify-center gap-2 px-3 py-2.5 rounded-lg bg-zinc-800/80 hover:bg-zinc-700/80 border border-zinc-700/60 text-sm font-medium text-zinc-200 transition-all hover:border-purple-500/50 shadow-sm"
        >
          <FolderPlus className="w-4 h-4 text-purple-400" />
          <span>Add Custom Game</span>
        </button>

        {/* Navigation Tabs */}
        <nav className="space-y-1">
          <div className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider px-3 mb-2">Navigation</div>
          
          <button
            onClick={() => setCurrentTab('library')}
            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              currentTab === 'library'
                ? 'bg-purple-600/15 text-purple-300 border border-purple-500/30'
                : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
            }`}
          >
            <div className="flex items-center gap-2.5">
              <Gamepad2 className="w-4 h-4" />
              <span>Library</span>
            </div>
            <span className="text-xs px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-400 font-mono">
              {storeCounts['all'] || 0}
            </span>
          </button>

          <button
            onClick={() => setCurrentTab('fixes')}
            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              currentTab === 'fixes'
                ? 'bg-purple-600/15 text-purple-300 border border-purple-500/30'
                : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
            }`}
          >
            <div className="flex items-center gap-2.5">
              <Compass className="w-4 h-4 text-emerald-400" />
              <span>Fixes & Tweaks</span>
            </div>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-semibold uppercase">
              ProtonDB
            </span>
          </button>

          <button
            onClick={() => setCurrentTab('plugins')}
            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              currentTab === 'plugins'
                ? 'bg-purple-600/15 text-purple-300 border border-purple-500/30'
                : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
            }`}
          >
            <div className="flex items-center gap-2.5">
              <Puzzle className="w-4 h-4 text-blue-400" />
              <span>Source Plugins</span>
            </div>
            <span className="text-xs px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-400">
              6
            </span>
          </button>

          <button
            onClick={() => setCurrentTab('system')}
            className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              currentTab === 'system'
                ? 'bg-purple-600/15 text-purple-300 border border-purple-500/30'
                : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800/50'
            }`}
          >
            <div className="flex items-center gap-2.5">
              <Cpu className="w-4 h-4 text-amber-400" />
              <span>Runners & Proton</span>
            </div>
          </button>
        </nav>

        {/* Store Filter List (Only visible in library tab) */}
        {currentTab === 'library' && (
          <div className="space-y-1 pt-2 border-t border-zinc-800/60">
            <div className="text-[11px] font-semibold text-zinc-500 uppercase tracking-wider px-3 mb-2">Sources</div>
            
            <button
              onClick={() => setSelectedStore('all')}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs transition-colors ${
                selectedStore === 'all'
                  ? 'bg-zinc-800 text-white font-medium'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40'
              }`}
            >
              <span>All Sources</span>
              <span className="text-[11px] opacity-60 font-mono">{storeCounts['all'] || 0}</span>
            </button>

            <button
              onClick={() => setSelectedStore('steam')}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs transition-colors ${
                selectedStore === 'steam'
                  ? 'bg-zinc-800 text-white font-medium'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40'
              }`}
            >
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-blue-500"></span>
                Steam
              </span>
              <span className="text-[11px] opacity-60 font-mono">{storeCounts['steam'] || 0}</span>
            </button>

            <button
              onClick={() => setSelectedStore('epic')}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs transition-colors ${
                selectedStore === 'epic'
                  ? 'bg-zinc-800 text-white font-medium'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40'
              }`}
            >
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-zinc-300"></span>
                Epic Games
              </span>
              <span className="text-[11px] opacity-60 font-mono">{storeCounts['epic'] || 0}</span>
            </button>

            <button
              onClick={() => setSelectedStore('gog')}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs transition-colors ${
                selectedStore === 'gog'
                  ? 'bg-zinc-800 text-white font-medium'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40'
              }`}
            >
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-purple-400"></span>
                GOG Galaxy
              </span>
              <span className="text-[11px] opacity-60 font-mono">{storeCounts['gog'] || 0}</span>
            </button>

            <button
              onClick={() => setSelectedStore('custom')}
              className={`w-full flex items-center justify-between px-3 py-1.5 rounded-md text-xs transition-colors ${
                selectedStore === 'custom'
                  ? 'bg-zinc-800 text-white font-medium'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40'
              }`}
            >
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-amber-400"></span>
                Sideloaded / Custom
              </span>
              <span className="text-[11px] opacity-60 font-mono">{storeCounts['custom'] || 0}</span>
            </button>
          </div>
        )}
      </div>

      {/* Footer Info */}
      <div className="pt-4 border-t border-zinc-800/60 text-xs text-zinc-500 flex items-center justify-between">
        <span className="font-mono">Linux x86_64</span>
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
          Wails + Vite
        </span>
      </div>
    </aside>
  );
};
