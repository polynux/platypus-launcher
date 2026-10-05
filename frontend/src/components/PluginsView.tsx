import React from 'react';
import { Puzzle, CheckCircle, XCircle, RefreshCw, PlusCircle, ExternalLink } from 'lucide-react';
import type { PluginInfo } from '../types';

interface PluginsViewProps {
  plugins: PluginInfo[];
  onToggle: (id: string, enabled: boolean) => void;
  onRefresh: () => void;
}

export const PluginsView: React.FC<PluginsViewProps> = ({
  plugins,
  onToggle,
  onRefresh,
}) => {
  return (
    <div className="p-8 max-w-6xl mx-auto space-y-6 animate-in fade-in duration-200">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
            <Puzzle className="w-6 h-6 text-blue-400" />
            <span>Modular Source Plugins</span>
          </h2>
          <p className="text-sm text-zinc-400 mt-1">
            Extend Hydra with store connectors, scrapers, and fix repositories (Arachnel-style modularity).
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={onRefresh}
            className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 border border-zinc-700 transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            <span>Reload Plugins</span>
          </button>
          <button
            onClick={() => alert('Install plugin package: Select a .arach or zip manifest')}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-xs font-medium text-white shadow-md shadow-purple-600/30 transition-transform active:scale-95"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Install Plugin Package</span>
          </button>
        </div>
      </div>

      {/* Plugins Grid */}
      <div className="grid grid-cols-2 gap-4">
        {plugins.map((plugin) => (
          <div
            key={plugin.id}
            className="p-5 rounded-2xl bg-zinc-900 border border-zinc-800/80 hover:border-zinc-700 transition-all flex flex-col justify-between space-y-4"
          >
            <div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2.5">
                  <div className="w-8 h-8 rounded-lg bg-zinc-800 flex items-center justify-center text-zinc-300">
                    <Puzzle className="w-4 h-4 text-purple-400" />
                  </div>
                  <div>
                    <h3 className="text-base font-semibold text-white">{plugin.name}</h3>
                    <p className="text-xs text-zinc-500">v{plugin.version} by {plugin.author}</p>
                  </div>
                </div>

                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={plugin.enabled}
                    onChange={(e) => onToggle(plugin.id, e.target.checked)}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-zinc-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-zinc-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-purple-600"></div>
                </label>
              </div>

              <p className="text-xs text-zinc-400 mt-3 leading-relaxed">
                {plugin.description}
              </p>
            </div>

            <div className="flex items-center justify-between pt-3 border-t border-zinc-800 text-[11px] text-zinc-500">
              <span className="uppercase font-mono tracking-wider">Type: {plugin.store}</span>
              <span className={plugin.enabled ? 'text-emerald-400 flex items-center gap-1' : 'text-zinc-500 flex items-center gap-1'}>
                {plugin.enabled ? (
                  <>
                    <CheckCircle className="w-3.5 h-3.5" />
                    <span>Active & Syncing</span>
                  </>
                ) : (
                  <>
                    <XCircle className="w-3.5 h-3.5" />
                    <span>Disabled</span>
                  </>
                )}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
