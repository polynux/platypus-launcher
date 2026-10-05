import React, { useState } from 'react';
import { Search, Wrench, Zap, ExternalLink } from 'lucide-react';
import type { Game } from '../types';

interface FixesViewProps {
  games: Game[];
  onOpenSettings: (game: Game) => void;
}

export const FixesView: React.FC<FixesViewProps> = ({ games, onOpenSettings }) => {
  const [searchTerm, setSearchTerm] = useState('');

  const filteredGames = games.filter((g) =>
    g.title.toLowerCase().includes(searchTerm.toLowerCase())
  );

  return (
    <div className="px-8 py-6 max-w-6xl mx-auto space-y-5 animate-in fade-in duration-150">
      <div className="flex items-center justify-between border-b border-[#2a2d31] pb-3 mb-1">
        <div>
          <h2 className="text-[15px] font-bold tracking-[0.14em] text-[#f2f1ee] uppercase flex items-center gap-2.5" style={{ fontFamily: 'Hack, monospace' }}>
            <Wrench className="w-4 h-4 text-[#ffb000]" strokeWidth={1.75} />
            <span>Fixes &amp; Tweaks</span>
          </h2>
          <p className="text-[13px] text-[#9aa0a6] mt-1">
            Automated recipes, DLL overrides, and winetricks workarounds sourced from ProtonDB and Lutris.
          </p>
        </div>

        {/* Search */}
        <div className="relative w-72">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-[#767c82]" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search game fixes..."
            className="w-full bg-[#1d2024] border border-[#2e3238] rounded-[3px] pl-8 pr-3 py-1.5 text-[13px] text-[#e8e6e3] placeholder-[#767c82] focus:outline-none focus:border-[#ffb000]/70 font-mono"
          />
        </div>
      </div>

      {/* Fix Cards */}
      <div className="space-y-4">
        {filteredGames.map((game) => (
          <div
            key={game.id}
            className="p-4 bg-[#16181b] border border-[#2e3238] hover:border-[#3a3f45] transition-colors flex items-start justify-between gap-6"
          >
            <div className="flex gap-4">
              <img
                src={game.cover_url}
                alt={game.title}
                className="w-20 h-28 object-cover border border-[#2e3238] flex-shrink-0"
              />
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <h3 className="text-[15px] font-semibold text-[#f2f1ee]" style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}>{game.title}</h3>
                  <span className="font-mono text-[9px] uppercase tracking-[0.12em] px-1.5 py-0.5 bg-[#1d2024] text-[#c9cdd1] border border-[#3a3f45] rounded-[3px]">
                    {game.store}
                  </span>
                  {game.fix_info?.tier && (
                    <span className="font-mono text-[9px] uppercase tracking-[0.1em] px-1.5 py-0.5 bg-[#7dd48a]/10 text-[#7dd48a] border border-[#7dd48a]/40 rounded-[3px]">
                      Tier: {game.fix_info.tier}
                    </span>
                  )}
                </div>

                <p className="text-xs text-[#9aa0a6] leading-relaxed max-w-2xl">
                  {game.fix_info?.description || 'Compatible out of the box with standard Proton/Wine compatibility layer.'}
                </p>

                {game.fix_info?.winetricks && (
                  <div className="flex items-center gap-2 pt-1">
                    <span className="font-mono text-[9px] text-[#767c82] uppercase tracking-[0.12em]">Winetricks verbs:</span>
                    {game.fix_info.winetricks.map((trick) => (
                      <span key={trick} className="font-mono text-[10px] px-1.5 py-0.5 bg-[#1d2024] border border-[#3a3f45] text-[#e8e6e3] rounded-[3px]">
                        {trick}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="flex flex-col items-end gap-2 flex-shrink-0">
              <button
                onClick={() => onOpenSettings(game)}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-[#1d2024] hover:border-[#ffb000]/70 hover:text-[#ffb000] text-[11px] font-medium text-[#e8e6e3] border border-[#3a3f45] transition-colors rounded-[3px] font-mono"
              >
                <Zap className="w-3 h-3 text-[#ffb000]" />
                <span>Configure Fix</span>
              </button>
              {game.store === 'steam' && (
                <a
                  href={`https://www.protondb.com/app/${game.store_app_id}`}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1 font-mono text-[10px] text-[#7dd48a] hover:underline"
                >
                  <span>View on ProtonDB</span>
                  <ExternalLink className="w-3 h-3" />
                </a>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
