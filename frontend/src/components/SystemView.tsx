import React from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import type { ProtonVersion } from '../types';
import { EmptyState, btnNeutral, cx } from './ui';

interface SystemViewProps {
  runners: ProtonVersion[];
  onRefresh: () => void;
  scanning: boolean;
}

export function SystemView({ runners, onRefresh, scanning }: SystemViewProps) {
  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Runners</h1>
          <p className="mt-0.5 text-[12.5px] text-white/50">Proton, GE-Proton and Wine builds detected on this system.</p>
        </div>
        <button onClick={onRefresh} disabled={scanning} className={cx(btnNeutral, 'flex items-center gap-2 px-4 py-2.5 text-[12.5px] disabled:opacity-60')}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? 'Scanning' : 'Rescan'}
        </button>
      </header>

      {runners.length === 0 ? (
        <EmptyState label="no runners">
          No Proton or Wine builds found. Install Proton from Steam, or drop GE-Proton into ~/.steam/root/compatibilitytools.d.
        </EmptyState>
      ) : (
        <div className="space-y-2">
          {runners.map((r) => (
            <div key={r.id} className="flex items-center justify-between gap-4 rounded-2xl bg-white/[0.045] px-4 py-3 ring-1 ring-white/[0.07]">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[13px] font-bold">{r.name}</span>
                  {r.is_custom && <span className="rounded-md bg-amber-300/15 px-2 py-0.5 font-mono text-[11px] font-bold uppercase tracking-[0.14em] text-amber-300 ring-1 ring-amber-300/40">custom</span>}
                  {r.is_system && <span className="rounded-md bg-white/[0.06] px-2 py-0.5 font-mono text-[11px] font-bold uppercase tracking-[0.14em] text-white/55">system</span>}
                </div>
                <div className="truncate font-mono text-[11px] text-white/45 select-text">{r.path}</div>
              </div>
              <span className="flex flex-none items-center gap-1.5 font-mono text-[11px] uppercase tracking-wider text-emerald-300">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> found
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
