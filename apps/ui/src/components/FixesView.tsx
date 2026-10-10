import React, { useState } from 'react';
import type { Game } from '../types';
import { Cover, EmptyState, SegRow, TierBadge, TweakDot } from './ui';
import { Badge } from './ds/badge';
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
          <p className="mt-0.5 max-w-[75ch] text-[12.5px] text-white/70">
            Curated recipes (Lutris, community) and ProtonDB ratings for your library. {withData.length} of {games.length} titles have data.
          </p>
          <p className="mt-1.5 max-w-[75ch] text-[12px] leading-relaxed text-white/70">
            A fix is a recipe of winetricks packages and DLL overrides for one game. ProtonDB tiers run from Platinum (works out of the box)
            through Gold, Silver and Bronze to Borked (doesn't run).
          </p>
        </div>
        <SegRow<Filter>
          label="Filter"
          value={filter}
          onChange={setFilter}
          options={[{ value: 'attention', label: 'needs attention' }, { value: 'all', label: 'all rated' }]}
        />
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
                    <Badge variant="neutral">{storeLabel(g.store)} · {f.source}</Badge>
                  </div>
                  <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">{needsAttention(g) ? attentionReason(g) : f.description}</p>
                  {(f.winetricks?.length || f.dll_overrides) && (
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {f.winetricks?.map((t) => <Badge key={t} variant="info" className="normal-case tracking-normal">{t}</Badge>)}
                      {f.dll_overrides && Object.entries(f.dll_overrides).map(([k, v]) => <Badge key={k} variant="neutral" className="normal-case tracking-normal">{k}={v}</Badge>)}
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
