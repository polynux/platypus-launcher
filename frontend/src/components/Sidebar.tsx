import React from 'react';
import {
  Gamepad2,
  Puzzle,
  FolderPlus,
  Compass,
  Cpu,
  Wrench,
} from 'lucide-react';
import type { StoreType } from '../types';

interface SidebarProps {
  currentTab: 'library' | 'plugins' | 'system' | 'fixes';
  setCurrentTab: (tab: 'library' | 'plugins' | 'system' | 'fixes') => void;
  selectedStore: StoreType | 'all';
  setSelectedStore: (store: StoreType | 'all') => void;
  storeCounts: Record<string, number>;
  onAddCustomGame: () => void;
  sessionPid: number | null;
  gameCount: number;
}

const NAV_ITEMS = [
  { id: 'library', label: 'Library', icon: Gamepad2 },
  { id: 'fixes', label: 'Fixes & Tweaks', icon: Wrench },
  { id: 'plugins', label: 'Source Plugins', icon: Puzzle },
  { id: 'system', label: 'Runners & Proton', icon: Cpu },
] as const;

const SOURCES: Array<{ id: StoreType; label: string }> = [
  { id: 'steam', label: 'Steam' },
  { id: 'epic', label: 'Epic Games' },
  { id: 'gog', label: 'GOG Galaxy' },
  { id: 'custom', label: 'Sideloaded' },
];

export const Sidebar: React.FC<SidebarProps> = ({
  currentTab,
  setCurrentTab,
  selectedStore,
  setSelectedStore,
  storeCounts,
  onAddCustomGame,
  sessionPid,
  gameCount,
}) => {
  return (
    <aside
      className="w-60 flex-shrink-0 flex flex-col h-screen select-none bg-[#16181b] border-r border-[#2a2d31]"
    >
      {/* Wordmark */}
      <div className="px-4 pt-4 pb-3 border-b border-[#2a2d31] relative overflow-hidden">
        <div
          aria-hidden
          className="absolute inset-0 opacity-[0.05] pointer-events-none"
          style={{
            backgroundImage:
              'repeating-linear-gradient(0deg, #ffb000 0px, #ffb000 1px, transparent 1px, transparent 3px)',
          }}
        />
        <div className="relative flex items-center gap-2.5">
          <span className="w-2.5 h-2.5 bg-[#ffb000]" style={{ boxShadow: '0 0 6px rgba(255,176,0,.5)' }} />
          <h1
            className="font-mono font-bold text-[15px] tracking-[0.18em] text-[#e8e6e3]"
            style={{ fontFamily: 'Hack, monospace' }}
          >
            HYDRA
          </h1>
          <span className="font-mono text-[9px] text-[#8a8f94] tracking-wider border border-[#33373c] px-1 py-px">
            LINUX
          </span>
        </div>
        <p className="relative font-mono text-[10px] text-[#8a8f94] tracking-wide mt-1">
          launch control · linux
        </p>
      </div>

      {/* Navigation */}
      <nav className="px-2 pt-3 flex-none">
        <ul className="space-y-px">
          {NAV_ITEMS.map(({ id, label, icon: Icon }) => {
            const active = currentTab === id;
            return (
              <li key={id}>
                <button
                  onClick={() => setCurrentTab(id)}
                  className={`w-full flex items-center gap-2.5 px-3 py-[7px] text-[13px] border-l-2 transition-colors ${
                    active
                      ? 'border-[#ffb000] bg-[#1f2226] text-[#f2f1ee]'
                      : 'border-transparent text-[#9aa0a6] hover:text-[#d7d5d1] hover:bg-[#1c1f22]'
                  }`}
                >
                  <Icon className="w-3.5 h-3.5 flex-none" strokeWidth={1.75} />
                  <span className="flex-1 text-left">{label}</span>
                  {id === 'library' && (
                    <span className="font-mono text-[10px] text-[#8a8f94]">
                      {String(gameCount).padStart(3, '0')}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Store filter */}
      {currentTab === 'library' && (
        <div className="px-2 pt-4 pb-2 border-t border-[#2a2d31] mt-3 overflow-y-auto flex-1 min-h-0">
          <div className="px-3 mb-1.5 font-mono text-[10px] tracking-[0.14em] text-[#767c82] uppercase">
            Sources
          </div>
          <ul className="space-y-px">
            <li>
              <button
                onClick={() => setSelectedStore('all')}
                className={`w-full flex items-center justify-between px-3 py-1 text-[12px] transition-colors ${
                  selectedStore === 'all'
                    ? 'text-[#f2f1ee] bg-[#1f2226]'
                    : 'text-[#9aa0a6] hover:text-[#d7d5d1] hover:bg-[#1c1f22]'
                }`}
              >
                <span>All sources</span>
                <span className="font-mono text-[10px] text-[#8a8f94]">{String(storeCounts['all'] || 0).padStart(3, '0')}</span>
              </button>
            </li>
            {SOURCES.map(({ id, label }) => (
              <li key={id}>
                <button
                  onClick={() => setSelectedStore(id)}
                  className={`w-full flex items-center justify-between px-3 py-1 text-[12px] transition-colors ${
                    selectedStore === id
                      ? 'text-[#f2f1ee] bg-[#1f2226]'
                      : 'text-[#9aa0a6] hover:text-[#d7d5d1] hover:bg-[#1c1f22]'
                  }`}
                >
                  <span className="flex items-center gap-2">
                    <span
                      className={`w-1 h-2.5 flex-none ${
                        selectedStore === id ? 'bg-[#ffb000]' : 'bg-[#3a3f45]'
                      }`}
                    />
                    {label}
                  </span>
                  <span className="font-mono text-[10px] text-[#8a8f94]">{String(storeCounts[id] || 0).padStart(3, '0')}</span>
                </button>
              </li>
            ))}
          </ul>

          <button
            onClick={onAddCustomGame}
            className="mt-3 w-full flex items-center gap-2 px-3 py-1.5 text-[12px] text-[#9aa0a6] border border-[#2e3238] hover:border-[#ffb000]/60 hover:text-[#e8e6e3] bg-[#1a1c1f] transition-colors"
          >
            <FolderPlus className="w-3.5 h-3.5" strokeWidth={1.75} />
            Sideload a game
          </button>
        </div>
      )}

      <div className="flex-1 min-h-0" />

      {/* Session status line */}
      <div
        className={`mx-2 mb-2 px-3 py-2 border font-mono text-[10px] flex items-center justify-between ${
          sessionPid
            ? 'border-[#ffb000]/50 bg-[#ffb000]/[0.07] text-[#ffb000]'
            : 'border-[#2a2d31] bg-[#1a1c1f] text-[#767c82]'
        }`}
      >
        <span className="flex items-center gap-2">
          <span
            className={`w-1.5 h-1.5 rounded-full ${
              sessionPid ? 'bg-[#ffb000]' : 'bg-[#4a9a5f]'
            }`}
            style={sessionPid ? { boxShadow: '0 0 5px rgba(255,176,0,.6)' } : undefined}
          />
          {sessionPid ? 'IN SESSION' : 'IDLE'}
        </span>
        <span>{sessionPid ? `PID ${sessionPid}` : `v0.1 · x86_64`}</span>
      </div>
    </aside>
  );
};