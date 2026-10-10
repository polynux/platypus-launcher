import React from 'react';
import { GAMES, RUNNERS } from '../data.js';

const TIERS = {
  platinum: 'bg-neutral-900 text-white',
  gold: 'border border-neutral-900 text-neutral-900',
  silver: 'bg-neutral-200 text-neutral-600',
  bronze: 'bg-neutral-200 text-neutral-600',
  borked: 'bg-neutral-900 text-white',
};

function Row({ game, index }) {
  return (
    <button
      className="group grid w-full grid-cols-[2rem_minmax(11rem,2fr)_5rem_5rem_repeat(4,minmax(4.5rem,1fr))_2rem] items-center gap-x-3 border-b border-neutral-200 px-3 py-2 text-left transition-colors hover:bg-neutral-900 hover:text-white"
    >
      <span className="font-mono text-[10px] text-neutral-400 group-hover:text-neutral-500">{String(index + 1).padStart(2, '0')}</span>
      <span className="truncate text-[13px] font-semibold tracking-tight">{game.title}</span>
      <span className="font-mono text-[10px] uppercase text-neutral-500 group-hover:text-neutral-400">{game.store}</span>
      <span className={`justify-self-start px-1.5 py-px font-mono text-[9px] uppercase tracking-wider ${TIERS[game.tier] || TIERS.silver} group-hover:border-white/40 group-hover:bg-transparent group-hover:text-inherit`}>{game.tier}</span>
      <span className="truncate font-mono text-[10px] text-neutral-500 group-hover:text-neutral-300">{game.runner.replace('GE-', '').replace(' Proton', '')}</span>
      <span className="text-right font-mono text-[10px] tabular-nums">{game.hours > 999 ? `${(game.hours / 1000).toFixed(1)}k` : game.hours}h</span>
      <span className="text-right font-mono text-[10px] tabular-nums text-neutral-500 group-hover:text-neutral-400">{game.size.split(' ')[0]}</span>
      <span className="text-right font-mono text-[10px] text-neutral-400 group-hover:text-neutral-500">{game.last}</span>
      <span className="justify-self-end h-1.5 w-1.5 rounded-full bg-neutral-300 group-hover:bg-[#6ee77a]" title={game.status} />
    </button>
  );
}

export function SwissSpecSheet() {
  const played = GAMES.reduce((a, g) => a + g.hours, 0);
  return (
    <div className="min-h-screen bg-[#fafaf8] text-neutral-900" style={{ fontFamily: 'Archivo, sans-serif' }}>
      {/* masthead */}
      <header className="border-b border-neutral-900 px-8 pb-4 pt-6">
        <div className="flex items-start justify-between">
          <div className="flex items-baseline gap-3">
            <span className="text-[26px] font-black leading-none tracking-tighter">PLATYPUS</span>
            <span className="font-mono text-[10px] uppercase tracking-widest text-neutral-500">library index / rev 47</span>
          </div>
          <div className="text-right font-mono text-[10px] uppercase leading-relaxed tracking-wider text-neutral-500">
            <div>linux x86_64 · wayland</div>
            <div>scan completed 09:41</div>
          </div>
        </div>
      </header>

      {/* figure row */}
      <section className="grid grid-cols-4 divide-x divide-neutral-200 border-b border-neutral-200">
        {[
          ['titles indexed', String(GAMES.length).padStart(2, '0')],
          ['hours on record', `${(played / 1000).toFixed(1)}k`],
          ['proton runners', String(RUNNERS.length).padStart(2, '0')],
          ['avg. readiness', 'GOLD+'],
        ].map(([label, value], i) => (
          <div key={label} className={`px-8 py-5 ${i === 0 ? 'pl-8' : ''}`}>
            <div className="font-mono text-[9px] uppercase tracking-[0.2em] text-neutral-500">{label}</div>
            <div className="mt-1 text-[34px] font-black leading-none tracking-tighter tabular-nums">{value}</div>
          </div>
        ))}
      </section>

      {/* index table */}
      <main className="px-8 pb-16 pt-6">
        <div className="mb-2 flex items-baseline justify-between">
          <h2 className="text-[13px] font-bold uppercase tracking-[0.18em]">Master Index — all sources</h2>
          <span className="font-mono text-[9px] uppercase tracking-widest text-neutral-400">click any row</span>
        </div>

        <div className="grid grid-cols-[2rem_minmax(11rem,2fr)_5rem_5rem_repeat(4,minmax(4.5rem,1fr))_2rem] gap-x-3 border-b-2 border-neutral-900 px-3 pb-1.5 font-mono text-[9px] uppercase tracking-widest text-neutral-500">
          <span>№</span><span>Title</span><span>Src</span><span>Tier</span><span>Runner</span>
          <span className="text-right">Hrs</span><span className="text-right">Size</span><span className="text-right">Last</span><span className="text-right">●</span>
        </div>

        {GAMES.map((g, i) => <Row key={g.id} game={g} index={i} />)}

        {/* footnote */}
        <div className="mt-6 grid grid-cols-3 gap-6 font-mono text-[10px] leading-relaxed text-neutral-500">
          <p className="col-span-2">
            NOTE — tier figures reflect community reports aggregated nightly; runner column shows the effective binary after per-game override.
            Sizes are on-disk after deduplication across prefixes. A hollow dot marks titles never launched under Platypus.
          </p>
          <div className="border-t border-neutral-900 pt-2 text-neutral-900">
            <div className="flex justify-between"><span>STEAM</span><span>14</span></div>
            <div className="flex justify-between"><span>GOG</span><span>4</span></div>
            <div className="flex justify-between"><span>EPIC</span><span>5</span></div>
            <div className="flex justify-between"><span>SIDELOAD</span><span>1</span></div>
          </div>
        </div>
      </main>
    </div>
  );
}