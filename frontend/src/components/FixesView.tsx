import React, { useState } from 'react';
import type { Game } from '../types';
import { Cover, EmptyState, TierBadge, TweakDot, cx } from './ui';
import { attentionReason, needsAttention, storeLabel } from '../lib/games';

type Filter = 'attention' | 'all';

export function FixesView({ games, onOpenGame }: { games: Game[]; onOpenGame: (id: string) => void }) {
  const [filter, setFilter] = useState<Filter>('attention');
  const withData = games.filter((g) => g.fix_info);
  const shown = filter === 'attention' ? withData.filter(needsAttention) : withData;

  return (
    <div className="p-7">
      <header className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[23px] font-extrabold tracking-tight">Fixes</h1>
          <p className="mt-0.5 max-w-xl text-[12.5px] text-white/50">
            Curated recipes (Lutris, community) and ProtonDB ratings for your library. {withData.length} of {games.length} titles have data.
          </p>
        </div>
        <div role="radiogroup" aria-label="Filter" className="flex rounded-xl bg-white/[0.05] p-1">
          {([['attention', 'needs attention'], ['all', 'all rated']] as const).map(([v, l]) => (
            <button key={v} role="radio" aria-checked={filter === v} onClick={() => setFilter(v)} className={cx('rounded-lg px-3.5 py-1.5 text-[12px] font-bold transition', filter === v ? 'bg-white/12 text-white' : 'text-white/45 hover:text-white/80')}>
              {l}
            </button>
          ))}
        </div>
      </header>

      {shown.length === 0 ? (
        <EmptyState label={withData.length ? 'all clear' : 'no fix data yet'}>
          {withData.length
            ? 'No titles need attention. Switch to "all rated" to browse ProtonDB tiers.'
            : 'Compatibility data loads in the background after a scan. ProtonDB needs a network connection.'}
        </EmptyState>
      ) : (
        <div className="space-y-3">
          {shown.map((g) => {
            const f = g.fix_info!;
            return (
              <button key={g.id} onClick={() => onOpenGame(g.id)} className="flex w-full gap-4 rounded-2xl bg-white/[0.045] p-4 text-left ring-1 ring-white/[0.06] transition hover:ring-orange-400/40">
                <Cover src={g.cover_url} title={g.title} className="h-24 w-16 flex-none rounded-xl ring-1 ring-white/10" />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2.5">
                    {needsAttention(g) && <TweakDot />}
                    <span className="truncate text-[14px] font-bold">{g.title}</span>
                    <TierBadge tier={f.tier} />
                    <span className="font-mono text-[11px] uppercase tracking-wider text-white/40">{storeLabel(g.store)} · {f.source}</span>
                  </div>
                  <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">{needsAttention(g) ? attentionReason(g) : f.description}</p>
                  {(f.winetricks?.length || f.dll_overrides) && (
                    <div className="mt-2 flex flex-wrap gap-1.5 font-mono text-[11.5px]">
                      {f.winetricks?.map((t) => <span key={t} className="rounded-lg bg-sky-300/10 px-2 py-0.5 text-sky-200">{t}</span>)}
                      {f.dll_overrides && Object.entries(f.dll_overrides).map(([k, v]) => <span key={k} className="rounded-lg bg-white/[0.06] px-2 py-0.5 text-white/60">{k}={v}</span>)}
                    </div>
                  )}
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
