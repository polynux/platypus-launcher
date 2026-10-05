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
    <div className="px-8 py-6 max-w-6xl mx-auto space-y-5 animate-in fade-in duration-150">
      <div className="flex items-center justify-between border-b border-[#2a2d31] pb-3 mb-1">
        <div>
          <h2 className="text-[15px] font-bold tracking-[0.14em] text-[#f2f1ee] uppercase flex items-center gap-2.5" style={{ fontFamily: 'Hack, monospace' }}>
            <Puzzle className="w-4 h-4 text-[#ffb000]" strokeWidth={1.75} />
            <span>Source Plugins</span>
          </h2>
          <p className="text-[13px] text-[#9aa0a6] mt-1">
            Extend Hydra with store connectors, scrapers, and fix repositories (Arachnel-style modularity).
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={onRefresh}
            className="flex items-center gap-2 px-3 py-1.5 bg-[#1d2024] hover:border-[#ffb000]/70 hover:text-[#ffb000] font-mono text-[11px] text-[#e8e6e3] border border-[#3a3f45] transition-colors rounded-[3px]"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            <span>Reload Plugins</span>
          </button>
          <button
            onClick={() => alert('Plugin package install is not implemented yet — place a manifest in ~/.local/share/hydra/plugins')}
            className="flex items-center gap-2 px-3 py-1.5 bg-[#ffb000] hover:bg-[#ffc23d] font-mono text-[11px] text-[#101214] transition-colors rounded-[3px]"
          >
            <PlusCircle className="w-3.5 h-3.5" />
            <span>INSTALL PACKAGE</span>
          </button>
        </div>
      </div>

      {/* Plugins Grid */}
      <div className="grid grid-cols-2 gap-4">
        {plugins.map((plugin) => (
          <div
            key={plugin.id}
            className="p-4 bg-[#16181b] border border-[#2e3238] hover:border-[#3a3f45] transition-colors flex flex-col justify-between space-y-4"
          >
            <div>
              <div className="flex items-center justify-between border-b border-[#2a2d31] pb-3 mb-1">
                <div className="flex items-center gap-2.5">
                  <div className="w-8 h-8 bg-[#1d2024] border border-[#3a3f45] flex items-center justify-center text-[#ffb000]">
                    <Puzzle className="w-4 h-4" strokeWidth={1.75} />
                  </div>
                  <div>
                    <h3 className="text-[14px] font-semibold text-[#f2f1ee]" style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}>{plugin.name}</h3>
                    <p className="font-mono text-[10px] text-[#767c82]">v{plugin.version} · {plugin.author}</p>
                  </div>
                </div>

                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={plugin.enabled}
                    onChange={(e) => onToggle(plugin.id, e.target.checked)}
                    className="sr-only peer"
                  />
                  <div className="w-9 h-5 bg-[#2e3238] peer-focus:outline-none rounded-none peer peer-checked:after:translate-x-4 peer-checked:after:border-[#101214] after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-[#9aa0a6] peer-checked:after:bg-[#ffb000] after:border after:border-[#101214] after:h-4 after:w-4 after:transition-all peer-checked:bg-[#3d2f10]"></div>
                </label>
              </div>

              <p className="text-xs text-[#9aa0a6] mt-3 leading-relaxed">
                {plugin.description}
              </p>
            </div>

            <div className="flex items-center justify-between pt-3 border-t border-[#2a2d31] font-mono text-[10px] text-[#767c82]">
              <span className="uppercase tracking-[0.12em]">{plugin.store}</span>
              <span className={plugin.enabled ? 'text-[#7dd48a] flex items-center gap-1' : 'text-[#767c82] flex items-center gap-1'}>
                {plugin.enabled ? (
                  <>
                    <CheckCircle className="w-3 h-3" />
                    <span>Enabled</span>
                  </>
                ) : (
                  <>
                    <XCircle className="w-3 h-3" />
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
