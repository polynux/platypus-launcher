import React from 'react';
import { Cpu, HardDrive, CheckCircle2, RefreshCw, Terminal, Layers } from 'lucide-react';
import type { ProtonVersion } from '../types';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
}

export const SystemView: React.FC<SystemViewProps> = ({ runners, onRefresh }) => {
  return (
    <div className="p-8 max-w-6xl mx-auto space-y-6 animate-in fade-in duration-200">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
            <Cpu className="w-6 h-6 text-amber-400" />
            <span>Proton & Wine Runners</span>
          </h2>
          <p className="text-sm text-zinc-400 mt-1">
            Locally detected Proton versions, GE-Proton releases, and Lutris runner environments.
          </p>
        </div>

        <button
          onClick={onRefresh}
          className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 border border-zinc-700 transition-colors"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          <span>Rescan System</span>
        </button>
      </div>

      {/* Runners List */}
      <div className="space-y-3">
        {runners.map((runner) => (
          <div
            key={runner.id}
            className="p-4 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-between hover:border-zinc-700 transition-colors"
          >
            <div className="flex items-center gap-3.5">
              <div className="w-10 h-10 rounded-xl bg-zinc-800 flex items-center justify-center text-purple-400">
                <Cpu className="w-5 h-5" />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h4 className="text-sm font-semibold text-white">{runner.name}</h4>
                  {runner.is_custom && (
                    <span className="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-300 border border-amber-500/30">
                      Custom GE
                    </span>
                  )}
                  {runner.is_system && (
                    <span className="text-[10px] uppercase font-bold px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-300 border border-blue-500/30">
                      System
                    </span>
                  )}
                </div>
                <p className="text-xs font-mono text-zinc-500 mt-0.5">{runner.path}</p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <span className="text-xs text-emerald-400 flex items-center gap-1.5">
                <CheckCircle2 className="w-4 h-4" />
                <span>Ready</span>
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
