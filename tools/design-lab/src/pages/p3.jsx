import React from 'react';
import { GAMES, FIXES } from '../data.js';

const tierWord = { platinum: 'immaculate', gold: 'excellent', silver: 'workable', bronze: 'fussy', borked: 'difficult' };

export function PosterEditorial() {
  const featured = GAMES[0];
  return (
    <div className="min-h-screen bg-[#f4f1ea] text-[#1a1814]" style={{ fontFamily: '"Bricolage Grotesque", serif' }}>
      {/* masthead */}
      <header className="border-b border-[#1a1814]">
        <div className="mx-auto flex max-w-[1180px] items-center justify-between px-10 py-5">
          <div className="flex items-baseline gap-2">
            <span className="grid h-8 w-8 place-items-center rounded-full bg-[#1a1814] text-[15px] font-black text-[#f4f1ea]">P</span>
            <span className="text-[19px] font-extrabold tracking-tight">The Platypus Register</span>
          </div>
          <nav className="flex gap-7 text-[13px] font-semibold">
            <button className="border-b-2 border-[#c14a2e] pb-0.5 text-[#c14a2e]">Collection</button>
            <button className="pb-0.5 text-[#5c5648] hover:text-[#1a1814]">Workshop</button>
            <button className="pb-0.5 text-[#5c5648] hover:text-[#1a1814]">Journal</button>
            <button className="pb-0.5 text-[#5c5648] hover:text-[#1a1814]">About</button>
          </nav>
        </div>
      </header>

      <main className="mx-auto max-w-[1180px] px-10">
        {/* opening spread */}
        <section className="grid grid-cols-[1.5fr_1fr] gap-14 border-b border-[#1a1814]/15 py-12">
          <div>
            <div className="mb-4 flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.22em] text-[#c14a2e]">
              <span className="h-px w-8 bg-[#c14a2e]" />
              Tuesday · 5 October · forty titles shelved
            </div>
            <h1 className="text-[54px] font-extrabold leading-[1.0] tracking-[-0.025em]">
              The whole collection,<br />kept in working order.
            </h1>
            <p className="mt-6 max-w-[54ch] text-[16.5px] leading-[1.65] text-[#3d382e]">
              Platypus keeps every store you buy from in one shelf, and — this is the part the
              others skip — records what each game needs to actually run on Linux. Below,
              the state of the shelf this morning: what verified clean, what wants a tweak,
              and what the community has learned since Friday.
            </p>
            <div className="mt-8 flex items-center gap-5">
              <button className="rounded-full bg-[#1a1814] px-7 py-3 text-[14px] font-bold text-[#f4f1ea] transition hover:bg-[#c14a2e]">Open the library</button>
              <span className="text-[13px] font-medium text-[#5c5648]">{GAMES.filter(g => g.status === 'tweak').length} titles need attention</span>
            </div>
          </div>

          {/* featured plate */}
          <div className="flex flex-col justify-end">
            <div className="relative overflow-hidden rounded-[4px]">
              <img src={featured.cover} alt="" className="aspect-[4/5] w-full object-cover" style={{ filter: 'saturate(.92)' }} />
              <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/70 to-transparent p-5">
                <div className="font-mono text-[9px] uppercase tracking-[0.2em] text-white/70">shelf highlight · {featured.store}</div>
                <div className="mt-1 text-[22px] font-bold text-white">{featured.title}</div>
              </div>
            </div>
            <p className="mt-3 border-l-2 border-[#c14a2e] pl-3 text-[12px] leading-relaxed text-[#5c5648]">{featured.verdict}</p>
          </div>
        </section>

        {/* the ledger — text-first list, not cards */}
        <section className="grid grid-cols-[1fr_20rem] gap-14 py-12">
          <div>
            <h2 className="mb-1 text-[28px] font-bold tracking-[-0.02em]">This week&rsquo;s ledger</h2>
            <p className="mb-8 text-[13.5px] italic text-[#5c5648]">Every change since Friday&rsquo;s press, newest first.</p>
            <ul>
              {GAMES.slice(2, 10).map((g, i) => (
                <li key={g.id} className="group grid grid-cols-[2.4rem_1fr_auto] items-baseline gap-4 border-t border-[#1a1814]/12 py-4 transition-colors hover:bg-[#ece7db]/60">
                  <span className="font-mono text-[11px] text-[#8a8272]">{String(i + 1).padStart(2, '0')}</span>
                  <div>
                    <span className="text-[16px] font-bold tracking-tight group-hover:text-[#c14a2e]">{g.title}</span>
                    <span className="ml-3 font-mono text-[10px] uppercase tracking-widest text-[#8a8272]">{g.store} · {g.runner}</span>
                    <p className="mt-1 max-w-[62ch] text-[13px] leading-relaxed text-[#3d382e]">{g.verdict}</p>
                  </div>
                  <span className={`justify-self-end rounded-full px-3 py-1 text-[11px] font-bold ${g.status === 'tweak' ? 'bg-[#c14a2e] text-[#f4f1ea]' : 'bg-transparent text-[#5c5648] ring-1 ring-[#1a1814]/25'}`}>
                    {tierWord[g.tier]}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          {/* community column */}
          <aside className="border-l border-[#1a1814]/15 pl-10">
            <h3 className="text-[15px] font-extrabold uppercase tracking-[0.16em]">From the community</h3>
            <p className="mt-2 text-[12.5px] italic text-[#5c5648]">Aggregated ProtonDB reports, edited for sense.</p>
            <ul className="mt-6 space-y-5">
              {FIXES.slice(0, 5).map((f) => (
                <li key={f.game}>
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="text-[13.5px] font-bold">{f.game}</span>
                    <span className="font-mono text-[10px] text-[#8a8272]">{(f.reports / 1000).toFixed(1)}k rpt</span>
                  </div>
                  <p className="mt-1 text-[12px] leading-relaxed text-[#3d382e]">{f.line}</p>
                </li>
              ))}
            </ul>
            <button className="mt-8 w-full rounded-full border border-[#1a1814] py-2.5 text-[12.5px] font-bold transition hover:bg-[#1a1814] hover:text-[#f4f1ea]">
              All community notes
            </button>
          </aside>
        </section>

        {/* footer strip */}
        <footer className="flex items-center justify-between border-t border-[#1a1814] py-5 font-mono text-[10px] uppercase tracking-[0.2em] text-[#8a8272]">
          <span>Platypus Register · set in Bricolage Grotesque</span>
          <span>printed locally · no cloud required</span>
        </footer>
      </main>
    </div>
  );
}