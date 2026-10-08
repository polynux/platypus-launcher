import React from 'react';
import { Loader2, RefreshCw } from 'lucide-react';
import type { ProtonVersion } from '../types';
import { EmptyState } from './ui';
import { Button } from './ds/button';
import { Badge } from './ds/badge';

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
        <Button variant="neutral" onClick={onRefresh} disabled={scanning}>
          {scanning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
          {scanning ? 'Scanning' : 'Rescan'}
        </Button>
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
                  {r.is_custom && <Badge variant="gold">custom</Badge>}
                  {r.is_system && <Badge variant="neutral">system</Badge>}
                </div>
                <div className="truncate font-mono text-[11px] text-white/45 select-text">{r.path}</div>
              </div>
              <Badge variant="ok" className="flex-none gap-1.5">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> found
              </Badge>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
