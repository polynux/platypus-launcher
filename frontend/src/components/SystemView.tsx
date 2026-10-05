import React from 'react';
import { Cpu, CheckCircle2, RefreshCw } from 'lucide-react';
import type { ProtonVersion } from '../types';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
}

export const SystemView: React.FC<SystemViewProps> = ({ runners, onRefresh }) => {
  return (
    <div className="px-8 py-6 max-w-6xl mx-auto space-y-5 animate-in fade-in duration-150">
      <div className="flex items-center justify-between border-b border-[#2a2d31] pb-3 mb-1">
        <div>
          <h2 className="text-[15px] font-bold tracking-[0.14em] text-[#f2f1ee] uppercase flex items-center gap-2.5" style={{ fontFamily: 'Hack, monospace' }}>
            <Cpu className="w-4 h-4 text-[#ffb000]" strokeWidth={1.75} />
            <span>Runners &amp; Proton</span>
          </h2>
          <p className="text-[13px] text-[#9aa0a6] mt-1">
            Locally detected Proton versions, GE-Proton releases, and Lutris runner environments.
          </p>
        </div>

        <button
          onClick={onRefresh}
          className="flex items-center gap-2 px-3 py-1.5 bg-[#1d2024] hover:border-[#ffb000]/70 hover:text-[#ffb000] font-mono text-[11px] text-[#e8e6e3] border border-[#3a3f45] transition-colors rounded-[3px]"
        >
          <RefreshCw className="w-3 h-3" />
          <span>Rescan System</span>
        </button>
      </div>

      {/* Runners List */}
      <div className="space-y-3">
        {runners.map((runner) => (
          <div
            key={runner.id}
            className="p-3.5 bg-[#16181b] border border-[#2e3238] flex items-center justify-between hover:border-[#3a3f45] transition-colors"
          >
            <div className="flex items-center gap-3.5">
              <div className="w-9 h-9 bg-[#1d2024] border border-[#3a3f45] flex items-center justify-center text-[#ffb000]">
                <Cpu className="w-4 h-4" strokeWidth={1.75} />
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h4 className="text-[13px] font-semibold text-[#f2f1ee]" style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}>{runner.name}</h4>
                  {runner.is_custom && (
                    <span className="font-mono text-[9px] uppercase tracking-[0.1em] px-1.5 py-0.5 bg-[#ffb000]/10 text-[#ffb000] border border-[#ffb000]/40 rounded-[3px]">
                      Custom GE
                    </span>
                  )}
                  {runner.is_system && (
                    <span className="font-mono text-[9px] uppercase tracking-[0.1em] px-1.5 py-0.5 bg-[#232327] text-[#9aa0a6] border border-[#3a3f45] rounded-[3px]">
                      System
                    </span>
                  )}
                </div>
                <p className="font-mono text-[10px] text-[#767c82] mt-0.5">{runner.path}</p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <span className="font-mono text-[10px] text-[#7dd48a] flex items-center gap-1.5 uppercase tracking-[0.1em]">
                <CheckCircle2 className="w-3 h-3" />
                <span>Ready</span>
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
