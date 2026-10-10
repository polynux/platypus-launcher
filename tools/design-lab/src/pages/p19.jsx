import React, { useState } from 'react';
import { GAMES } from '../data.js';
import { WX, Sidebar, ContinueGrid, ActivityPanel, GamePageShell } from './shared16.jsx';

/* /19 — LAYOUT STUDY: horizontal timeline dashboard. Day-grouped
   "session story" strip on top; library below as wide poster row.
   Tests: does a chronological reading of the library beat grids? */
export function It19Dashboard() {
  const days = [
    { d: 'today', games: GAMES.slice(0, 3) },
    { d: 'yesterday', games: GAMES.slice(3, 6) },
    { d: 'this week', games: GAMES.slice(6, 12) },
  ];
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar active="Dashboard" />
        <main className="flex-1 p-7">
          <header className="mb-7 flex items-baseline justify-between">
            <h1 className="text-[23px] font-extrabold tracking-tight">Your week in games</h1>
            <span className="font-mono text-[10px] uppercase tracking-widest text-white/30">14 sessions · 12h 41m</span>
          </header>

          {/* week strip: 14 day columns with bars + game chips */}
          <section className="mb-8 rounded-3xl bg-white/[0.04] p-6 ring-1 ring-white/[0.07]">
            <div className="mb-4 flex items-baseline justify-between">
              <h2 className="text-[15px] font-bold">Playtime — last 14 days</h2>
              <span className="font-mono text-[10px] text-white/35">peak 10h · avg 4.2h</span>
            </div>
            <div className="flex h-28 items-end gap-2">
              {[2,5,3,8,6,9,4,7,10,6,3,5,8,4].map((h, i) => (
                <div key={i} className="group relative flex-1">
                  <div className={`w-full rounded-md ${i === 12 ? 'bg-amber-300' : 'bg-white/[0.1]'} group-hover:bg-amber-300/50`} style={{ height: h * 11 }} />
                </div>
              ))}
            </div>
            <div className="mt-2 flex justify-between font-mono text-[9px] uppercase tracking-widest text-white/25"><span>21 sep</span><span>today</span></div>
          </section>

          {/* day-grouped rows */}
          {days.map((day) => (
            <section key={day.d} className="mb-7">
              <div className="mb-3 flex items-center gap-4">
                <h2 className="text-[13px] font-bold uppercase tracking-[0.14em] text-white/60">{day.d}</h2>
                <span className="h-px flex-1 bg-white/[0.08]" />
                <span className="font-mono text-[9px] text-white/25">{day.games.length} sessions</span>
              </div>
              <div className="flex gap-3">
                {day.games.map((g) => (
                  <a key={g.id} href="/19-game" className="group flex w-[280px] flex-none items-center gap-3.5 rounded-2xl bg-white/[0.045] p-3 ring-1 ring-white/[0.07] transition hover:ring-amber-300/40">
                    <img src={g.cover} alt="" className="h-16 w-12 flex-none rounded-lg object-cover" />
                    <div className="min-w-0 leading-tight">
                      <div className="truncate text-[13px] font-bold">{g.title.split(':')[0]}</div>
                      <div className="mt-0.5 font-mono text-[8.5px] uppercase tracking-wider text-white/30">{g.last} · {g.hours}h</div>
                      <div className="mt-1.5 font-mono text-[8.5px] text-amber-300/70">{g.tier}</div>
                    </div>
                  </a>
                ))}
              </div>
            </section>
          ))}

          <div className="grid grid-cols-2 gap-5">
            <div>
              <h2 className="mb-3 text-[15px] font-bold">Continue playing</h2>
              <ContinueGrid games={GAMES.slice(12, 16)} url="/19-game" cols="grid-cols-4" />
            </div>
            <ActivityPanel limit={5} />
          </div>
        </main>
      </div>
    </div>
  );
}
export function It19Game() {
  return <GamePageShell game={GAMES[5]}>{ { back: '/19' } }</GamePageShell>;
}
