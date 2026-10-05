import React from 'react';
import { Play, Settings, ShieldCheck, Flame } from 'lucide-react';
import type { Game } from '../types';

interface GameCardProps {
  game: Game;
  onSelect: (game: Game) => void;
  onOpenSettings: (game: Game) => void;
  onQuickLaunch: (game: Game, e: React.MouseEvent) => void;
}

export const GameCard: React.FC<GameCardProps> = ({
  game,
  onSelect,
  onOpenSettings,
  onQuickLaunch,
}) => {
  const storeColor = {
    steam: 'bg-blue-600/80 border-blue-400/40 text-blue-200',
    epic: 'bg-zinc-800/80 border-zinc-500/40 text-zinc-200',
    gog: 'bg-purple-700/80 border-purple-400/40 text-purple-200',
    ubisoft: 'bg-indigo-700/80 border-indigo-400/40 text-indigo-200',
    custom: 'bg-amber-600/80 border-amber-400/40 text-amber-200',
  }[game.store] || 'bg-zinc-800';

  const tierBadge = game.fix_info?.tier ? (
    <div className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded shadow backdrop-blur-sm border ${
      game.fix_info.tier.toLowerCase() === 'platinum' ? 'bg-emerald-500/30 text-emerald-300 border-emerald-500/50' :
      game.fix_info.tier.toLowerCase() === 'gold' ? 'bg-yellow-500/30 text-yellow-300 border-yellow-500/50' :
      game.fix_info.tier.toLowerCase() === 'silver' ? 'bg-slate-400/30 text-slate-200 border-slate-400/50' :
      'bg-red-500/30 text-red-300 border-red-500/50'
    }`}>
      {game.fix_info.tier}
    </div>
  ) : null;

  return (
    <div
      onClick={() => onSelect(game)}
      className="group relative rounded-xl overflow-hidden bg-zinc-900 border border-zinc-800/80 hover:border-purple-500/60 transition-all duration-300 hover:shadow-xl hover:shadow-purple-500/10 cursor-pointer flex flex-col aspect-[2/3]"
    >
      {/* Background Poster Image */}
      <div className="absolute inset-0 bg-zinc-950">
        <img
          src={game.cover_url}
          alt={game.title}
          loading="lazy"
          onError={(e) => {
            // Fallback placeholder
            (e.target as HTMLElement).style.display = 'none';
          }}
          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500 opacity-90 group-hover:opacity-100"
        />
        <div className="absolute inset-0 bg-gradient-to-t from-zinc-950 via-zinc-950/40 to-transparent"></div>
      </div>

      {/* Top Bar Badges */}
      <div className="relative p-2.5 flex items-center justify-between z-10">
        <span className={`text-[10px] font-semibold uppercase px-2 py-0.5 rounded-md backdrop-blur-md border ${storeColor}`}>
          {game.store}
        </span>
        {tierBadge}
      </div>

      {/* Hover Action Overlay */}
      <div className="absolute inset-0 flex items-center justify-center gap-3 opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-20 bg-zinc-950/40 backdrop-blur-[2px]">
        <button
          onClick={(e) => onQuickLaunch(game, e)}
          className="w-12 h-12 rounded-full bg-purple-600 hover:bg-purple-500 text-white flex items-center justify-center shadow-lg shadow-purple-600/50 hover:scale-110 transition-transform"
          title="Play Game"
        >
          <Play className="w-5 h-5 fill-white ml-0.5" />
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation();
            onOpenSettings(game);
          }}
          className="w-10 h-10 rounded-full bg-zinc-800/90 hover:bg-zinc-700 text-zinc-200 flex items-center justify-center border border-zinc-600/50 shadow-md hover:scale-110 transition-transform"
          title="Game Settings"
        >
          <Settings className="w-4 h-4" />
        </button>
      </div>

      {/* Bottom Title Area */}
      <div className="relative mt-auto p-3 z-10">
        <h3 className="text-sm font-semibold text-white truncate drop-shadow-md">
          {game.title}
        </h3>
        <p className="text-[11px] text-zinc-400 truncate mt-0.5 flex items-center gap-1.5">
          {game.settings.runner_id ? (
            <span className="text-purple-300 font-mono text-[10px]">{game.settings.runner_id.replace('steam:', '').replace('compat:', '')}</span>
          ) : (
            <span>Default Proton</span>
          )}
        </p>
      </div>
    </div>
  );
};
