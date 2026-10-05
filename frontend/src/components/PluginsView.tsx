import React from 'react';
import { RefreshCw } from 'lucide-react';
import type { PluginInfo } from '../types';
import { EmptyState, MonoLabel, Panel, TogglePill, btnNeutral, cx } from './ui';

interface PluginsViewProps {
  plugins: PluginInfo[];
  onToggle: (id: string, enabled: boolean) => void;
  onRefresh: () => void;
}

export function PluginsView({ plugins, onToggle, onRefresh }: PluginsViewProps) {
  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Plugins</h1>
          <p className="mt-0.5 max-w-xl text-[12.5px] text-white/50">
            Store connectors and fix sources. Toggles are remembered for this session but don't change scanning yet.
          </p>
        </div>
        <button onClick={onRefresh} className={cx(btnNeutral, 'flex items-center gap-2 px-4 py-2.5 text-[12.5px]')}>
          <RefreshCw className="h-3.5 w-3.5" /> Reload
        </button>
      </header>

      {plugins.length === 0 ? (
        <EmptyState label="no plugins">No plugins are registered.</EmptyState>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {plugins.map((p) => (
            <Panel key={p.id} className="flex flex-col justify-between gap-4">
              <div>
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <h3 className="text-[14px] font-bold">{p.name}</h3>
                    <p className="font-mono text-[9.5px] text-white/45">v{p.version} · {p.author} · {p.store}</p>
                  </div>
                  <span className={cx('rounded-md px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-[0.14em]', p.enabled ? 'bg-emerald-400/10 text-emerald-300' : 'bg-white/[0.06] text-white/45')}>
                    {p.enabled ? 'enabled' : 'disabled'}
                  </span>
                </div>
                <p className="mt-2.5 text-[12.5px] leading-relaxed text-white/60">{p.description}</p>
              </div>
              <button
                onClick={() => onToggle(p.id, !p.enabled)}
                aria-pressed={p.enabled}
                className={cx('flex items-center justify-between rounded-xl px-3.5 py-2.5 transition', p.enabled ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04] hover:bg-white/[0.06]')}
              >
                <MonoLabel className="text-white/70">{p.enabled ? 'disable plugin' : 'enable plugin'}</MonoLabel>
                <TogglePill on={p.enabled} />
              </button>
            </Panel>
          ))}
        </div>
      )}
    </div>
  );
}
