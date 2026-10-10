import React, { useId, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import type { PluginCapability, StorePlugin } from '../types';
import { MonoLabel, cx } from './ui';

/** Why a capability isn't doing anything, in words. */
function stateOf(c: PluginCapability, initialized: boolean): { text: string; tone: string } {
  if (!c.known) return { text: 'unknown to this Platypus · ignored', tone: 'text-white/55' };
  if (!c.on) return { text: 'off right now', tone: 'text-orange-200' };
  if (!c.orchestrated) return { text: `${initialized ? 'on' : 'declared'} · not used by Platypus yet`, tone: 'text-white/55' };
  return { text: initialized ? 'on' : 'declared', tone: 'text-emerald-300' };
}

/** What a plugin offers: every declared capability, whether it is on right
 *  now, and its depth flags. Before the plugin has run, this is only the
 *  manifest's promise. The list folds away: the header keeps the count. */
export function PluginCapabilities({ plugin: p }: { plugin: StorePlugin }) {
  const [open, setOpen] = useState(false);
  const listId = useId();
  if (p.offers.length === 0 && p.warnings.length === 0) return null;
  return (
    <div className="space-y-2">
      {p.offers.length > 0 && (
        <button
          type="button"
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setOpen((o) => !o)}
          className="flex w-full items-center gap-1.5 text-left font-mono text-hint text-white/55 transition hover:text-white/75"
        >
          <ChevronDown className={cx('h-3 w-3 flex-none transition-transform', open ? '' : '-rotate-90')} aria-hidden />
          <span className="min-w-0 flex-1 truncate">
            capabilities · {p.offers.length}
            {!p.initialized ? ' · declared · not started yet' : ''}
            {p.protocol ? ` · protocol ${p.protocol}` : ''}
          </span>
        </button>
      )}
      {p.offers.length === 0 && (
        <MonoLabel className="normal-case tracking-normal">
          capabilities{p.protocol ? ` · protocol ${p.protocol}` : ''}
        </MonoLabel>
      )}
      {p.offers.length > 0 && open && (
        <ul id={listId} aria-label="Capabilities" className="space-y-1 font-mono text-hint">
          {p.offers.map((c) => {
            const s = stateOf(c, p.initialized);
            const flags = Object.entries(c.features).sort(([a], [b]) => a.localeCompare(b));
            return (
              <li key={c.name} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
                <span className="w-28 flex-none truncate text-white/85" title={c.name}>{c.name}</span>
                <span className={cx('flex-none', s.tone)}>{s.text}</span>
                {c.on && flags.length > 0 && (
                  <span className="flex flex-wrap gap-x-2 text-white/55">
                    {flags.map(([name, on]) => (
                      <span key={name} data-on={on} className={on ? 'text-white/70' : 'line-through decoration-white/30'}>{name}</span>
                    ))}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {p.warnings.map((w) => (
        <p key={w} className="font-mono text-hint text-orange-200">{w}</p>
      ))}
    </div>
  );
}
