import React, { useState } from 'react';
import { Compass, Search, ShieldCheck, Zap, ExternalLink, Check, AlertTriangle } from 'lucide-react';
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
    <div className="p-8 max-w-6xl mx-auto space-y-6 animate-in fade-in duration-200">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
            <Compass className="w-6 h-6 text-emerald-400" />
            <span>Community Fixes & ProtonDB Feeds</span>
          </h2>
          <p className="text-sm text-zinc-400 mt-1">
            Automated recipes, DLL overrides, and winetricks workarounds sourced from ProtonDB and Lutris.
          </p>
        </div>

        {/* Search */}
        <div className="relative w-72">
          <Search className="w-4 h-4 absolute left-3 top-3 text-zinc-500" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search game fixes..."
            className="w-full bg-zinc-900 border border-zinc-800 rounded-lg pl-9 pr-4 py-2 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-emerald-500"
          />
        </div>
      </div>

      {/* Fix Cards */}
      <div className="space-y-4">
        {filteredGames.map((game) => (
          <div
            key={game.id}
            className="p-5 rounded-2xl bg-zinc-900 border border-zinc-800 hover:border-zinc-700 transition-all flex items-start justify-between gap-6"
          >
            <div className="flex gap-4">
              <img
                src={game.cover_url}
                alt={game.title}
                className="w-20 h-28 object-cover rounded-xl border border-zinc-800 flex-shrink-0"
              />
              <div className="space-y-2">
                <div className="flex items-center gap-3">
                  <h3 className="text-lg font-semibold text-white">{game.title}</h3>
                  <span className="text-xs uppercase px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 font-mono">
                    {game.store}
                  </span>
                  {game.fix_info?.tier && (
                    <span className="text-xs uppercase font-bold px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                      Tier: {game.fix_info.tier}
                    </span>
                  )}
                </div>

                <p className="text-xs text-zinc-400 leading-relaxed max-w-2xl">
                  {game.fix_info?.description || 'Compatible out of the box with standard Proton/Wine compatibility layer.'}
                </p>

                {game.fix_info?.winetricks && (
                  <div className="flex items-center gap-2 pt-1">
                    <span className="text-[11px] text-zinc-500 uppercase font-semibold">Winetricks verbs:</span>
                    {game.fix_info.winetricks.map((trick) => (
                      <span key={trick} className="text-xs font-mono px-2 py-0.5 rounded bg-zinc-800 border border-zinc-700 text-zinc-300">
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
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 border border-zinc-700 transition-colors"
              >
                <Zap className="w-3.5 h-3.5 text-yellow-400" />
                <span>Configure Fix</span>
              </button>
              {game.store === 'steam' && (
                <a
                  href={`https://www.protondb.com/app/${game.store_app_id}`}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1 text-[11px] text-emerald-400 hover:underline"
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
