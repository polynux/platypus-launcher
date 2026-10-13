import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation('nav');
  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">{t('nav:title.runners')}</h1>
          <p className="mt-0.5 text-[12.5px] text-white/70">Proton, GE-Proton and Wine builds detected on this system.</p>
          <p className="mt-1.5 max-w-xl text-[12px] leading-relaxed text-white/70">
            A runner is the build that runs a Windows game on Linux. Each game picks one in its settings; native Linux games need none.
          </p>
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
                <div className="truncate font-mono text-hint text-white/55 select-text">{r.path}</div>
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
