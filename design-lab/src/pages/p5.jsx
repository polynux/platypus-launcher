import React from 'react';
import { GAMES } from '../data.js';

const tiers = {
  platinum: 'text-emerald-600',
  gold: 'text-amber-600',
  silver: 'text-stone-400',
  bronze: 'text-orange-400',
};

export function CalmGallery() {
  // arrange: long rows of large covers, generous whitespace
  const shelf = GAMES.slice(0, 14);
  return (
    <div className="min-h-screen bg-[#fbfbf9] text-stone-800" style={{ fontFamily: 'Sora, sans-serif' }}>
      {/* air header */}
      <header className="mx-auto max-w-[1440px] px-14 pb-10 pt-14">
        <div className="flex items-end justify-between">
          <div>
            <div className="mb-3 flex items-center gap-2 text-[#10b981]">
              <span className="h-2.5 w-2.5 rounded-[3px] bg-[#10b981]" />
              <span className="text-[12px] font-bold uppercase tracking-[0.18em]">All systems ready</span>
            </div>
            <h1 className="text-[44px] font-light leading-none tracking-[-0.02em] text-stone-900">
              Good evening, <span className="font-extrabold">Polynux</span>
            </h1>
            <p className="mt-3 text-[14.5px] text-stone-500">
              44 titles shelved across four stores — 39 verified playable, 5 with notes, 0 broken.
            </p>
          </div>
          <div className="flex gap-8 text-right">
            {[
              ['44', 'titles'],
              ['39', 'ready'],
              ['5', 'tweaks'],
              ['3.1k', 'hours'],
            ].map(([n, l]) => (
              <div key={l}>
                <div className="text-[30px] font-light leading-none tracking-tight text-stone-900">{n}</div>
                <div className="mt-1.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-stone-400">{l}</div>
              </div>
            ))}
          </div>
        </div>
      </header>

      {/* shelf rail — one scroll row, big covers */}
      <section className="border-y border-stone-200/70 bg-white/60 py-5 backdrop-blur">
        <div className="mx-auto max-w-[1440px] px-14">
          <div className="mb-3 flex items-baseline justify-between text-[11px] font-bold uppercase tracking-[0.16em] text-stone-400">
            <span>Continue playing</span>
            <button className="transition hover:text-stone-700">everything →</button>
          </div>
          <div className="flex gap-3 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
            {GAMES.slice(0, 10).map((g) => (
              <button key={g.id} className="group w-[132px] flex-none text-left">
                <div className="overflow-hidden rounded-xl shadow-[0_1px_3px_rgba(0,0,0,.08)] ring-1 ring-black/[0.04] transition duration-300 group-hover:-translate-y-1 group-hover:shadow-[0_10px_28px_rgba(0,0,0,.13)]">
                  <img src={g.cover} alt="" className="aspect-[2/3] w-full object-cover" />
                </div>
                <div className="mt-2 truncate text-[12px] font-semibold leading-tight">{g.title}</div>
                <div className={`mt-0.5 text-[10px] font-bold uppercase tracking-wide ${tiers[g.tier]}`}>{g.tier}</div>
              </button>
            ))}
          </div>
        </div>
      </section>

      {/* airy grid */}
      <main className="mx-auto max-w-[1440px] px-14 py-10 pb-20">
        <h2 className="mb-6 text-[11px] font-bold uppercase tracking-[0.16em] text-stone-400">Recently added & updated</h2>
        <div className="grid grid-cols-4 gap-x-8 gap-y-10">
          {shelf.map((g) => (
            <div key={g.id} className="group">
              <div className="overflow-hidden rounded-2xl shadow-[0_1px_3px_rgba(0,0,0,.07)] ring-1 ring-black/[0.04]">
                <img src={g.wide} alt="" className="aspect-[2.4/1] w-full object-cover transition duration-700 group-hover:scale-[1.03]" />
              </div>
              <div className="mt-3.5 flex items-baseline justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate text-[14.5px] font-bold tracking-tight">{g.title}</div>
                  <div className="mt-0.5 text-[11.5px] text-stone-400">
                    {g.store} · {g.runner} · {g.hours}h
                  </div>
                </div>
                <button className="flex-none rounded-full bg-stone-900 px-4 py-1.5 text-[11px] font-bold text-white opacity-0 transition group-hover:opacity-100 hover:bg-stone-700">
                  Play
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* closing note */}
        <div className="mt-16 flex items-center justify-between rounded-2xl bg-white p-6 px-8 shadow-[0_1px_3px_rgba(0,0,0,.06)] ring-1 ring-black/[0.04]">
          <p className="text-[13.5px] text-stone-500">
            <span className="font-bold text-stone-800">Helldivers 2</span> is running an older Proton than recommended — community reports suggest GE-Proton10-4.
          </p>
          <button className="rounded-full border border-stone-300 px-4 py-2 text-[12px] font-bold text-stone-700 transition hover:border-stone-900 hover:text-stone-900">
            Review suggestion
          </button>
        </div>
      </main>
    </div>
  );
}