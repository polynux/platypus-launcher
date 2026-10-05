import React from 'react';
import { Play, Settings2, Copy, Check } from 'lucide-react';
import type { Game } from '../types';

interface GameCardProps {
  game: Game;
  onSelect: (game: Game) => void;
  onOpenSettings: (game: Game) => void;
  onQuickLaunch: (game: Game, e: React.MouseEvent) => void;
  copiedId: string | null;
  onCopyCommand: (game: Game, e: React.MouseEvent) => void;
}

const STORE_LABEL: Record<string, string> = {
  steam: 'STEAM',
  epic: 'EPIC',
  gog: 'GOG',
  ubisoft: 'UBI',
  custom: 'SIDELOAD',
};

const TIER_COLOR: Record<string, string> = {
  platinum: 'text-[#7dd48a] border-[#7dd48a]/40',
  gold: 'text-[#ffb000] border-[#ffb000]/40',
  silver: 'text-[#b8bcc0] border-[#b8bcc0]/40',
  bronze: 'text-[#c98a4b] border-[#c98a4b]/40',
  borked: 'text-[#e05d5d] border-[#e05d5d]/40',
};

const runnerLabel = (game: Game): string => {
  if (game.settings.runner_id) {
    return game.settings.runner_id.replace('steam:', '').replace('compat:', '');
  }
  if (game.store === 'steam') return 'steam client';
  return 'default runner';
};

export const GameCard: React.FC<GameCardProps> = ({
  game,
  onSelect,
  onOpenSettings,
  onQuickLaunch,
  copiedId,
  onCopyCommand,
}) => {
  const tier = game.fix_info?.tier ? TIER_COLOR[game.fix_info.tier.toLowerCase()] : null;
  const copied = copiedId === game.id;

  return (
    <div
      onClick={() => onSelect(game)}
      className="group relative flex flex-col aspect-[2/3] bg-[#1d2024] border border-[#2e3238] hover:border-[#ffb000]/70 transition-colors duration-150 cursor-pointer"
      title={game.title}
    >
      {/* Cover art */}
      <div className="absolute inset-0 overflow-hidden">
        <img
          src={game.cover_url}
          alt=""
          loading="lazy"
          draggable={false}
          onError={(e) => {
            (e.target as HTMLElement).style.opacity = '0';
          }}
          className="w-full h-full object-cover opacity-85 group-hover:opacity-100 transition-opacity duration-200"
        />
        {/* readability scrim, bottom anchored */}
        <div className="absolute inset-x-0 bottom-0 h-2/5" style={{ background: 'linear-gradient(to top, rgba(13,14,16,.96), rgba(13,14,16,0))' }} />
        <div className="absolute inset-x-0 top-0 h-10" style={{ background: 'linear-gradient(to bottom, rgba(13,14,16,.55), transparent)' }} />
      </div>

      {/* Top meta row: store + tier */}
      <div className="relative z-10 flex items-center justify-between px-2 pt-2">
        <span
          className="font-mono text-[9px] tracking-[0.12em] text-[#c9cdd1] px-1 py-0.5 bg-[#101214]/80 border border-[#3a3f45]"
          style={{ fontFamily: 'Hack, monospace' }}
        >
          {STORE_LABEL[game.store] || game.store.toUpperCase()}
        </span>
        {tier && (
          <span
            className={`font-mono text-[9px] uppercase tracking-[0.08em] px-1 py-0.5 bg-[#101214]/80 border ${tier}`}
            style={{ fontFamily: 'Hack, monospace' }}
          >
            {game.fix_info?.tier}
          </span>
        )}
      </div>

      {/* Hover action bar — instrument strip, not floating orbs */}
      <div className="absolute inset-x-0 top-1/2 -translate-y-1/2 z-20 flex justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-150">
        <div className="flex items-stretch border border-[#ffb000]/50 bg-[#101214]/95 divide-x divide-[#3a3f45]">
          <button
            onClick={(e) => {
              e.stopPropagation();
              onQuickLaunch(game, e);
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] font-mono text-[#ffb000] hover:bg-[#ffb000] hover:text-[#101214] transition-colors"
            title={`Launch ${game.title}`}
            style={{ fontFamily: 'Hack, monospace' }}
          >
            <Play className="w-3 h-3 fill-current" />
            RUN
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onCopyCommand(game, e);
            }}
            className="flex items-center px-2.5 py-1.5 text-[#9aa0a6] hover:text-[#e8e6e3] hover:bg-[#232327] transition-colors"
            title="Copy launch command"
          >
            {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onOpenSettings(game);
            }}
            className="flex items-center px-2.5 py-1.5 text-[#9aa0a6] hover:text-[#e8e6e3] hover:bg-[#232327] transition-colors"
            title="Configure runner & fixes"
          >
            <Settings2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Bottom readout */}
      <div className="relative z-10 mt-auto px-2.5 pb-2">
        <h3
          className="text-[13px] font-semibold text-[#f2f1ee] truncate leading-tight"
          style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}
        >
          {game.title}
        </h3>
        <p
          className="font-mono text-[9px] tracking-[0.08em] text-[#9aa0a6] truncate mt-0.5 uppercase"
          style={{ fontFamily: 'Hack, monospace' }}
        >
          {runnerLabel(game)}
        </p>
      </div>
    </div>
  );
};