import React, { useState } from 'react';
import { GAMES } from '../data.js';
import { WX, Sidebar, ContinueGrid, WaitingOn, RecentList, ActivityPanel, GamePageShell } from './shared16.jsx';

/* /18 — DASHBOARD VARIANT: "shelf first". No hero banner; instead
   a big last-played spotlight trio + denser continue grid + recent list. */
export function It18Dashboard() {
  const last = GAMES.slice(0, 3);
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar active="Dashboard" />
        <main className="flex-1 p-7">
          <header className="mb-6 flex items-center justify-between">
            <div>
              <h1 className="text-[23px] font-extrabold tracking-tight">Good evening, Polynux</h1>
              <p className="text-[13px] text-white/40">3 fixes waiting · 5 games updated today</p>
            </div>
            <div className="flex gap-3 font-mono text-[10px] text-white/35">
              <span className="rounded-lg bg-white/[0.05] px-3 py-2">12.7h this wk</span>
              <span className="rounded-lg bg-white/[0.05] px-3 py-2 text-amber-300">1,240 pts</span>
              <span className="rounded-lg bg-white/[0.05] px-3 py-2 text-emerald-300">19/24 ready</span>
            </div>
          </header>

          {/* spotlight trio — last played, cinema cards */}
          <section className="mb-7 grid grid-cols-3 gap-4">
            {last.map((g, i) => (
              <a key={g.id} href="/18-game" className={`group relative overflow-hidden rounded-3xl ring-1 ring-white/[0.08] ${i === 0 ? 'h-[300px]' : 'h-[300px]'}`}>
                <img src={g.wide} alt="" className="absolute inset-0 h-full w-full object-cover transition duration-500 group-hover:scale-105" />
                <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.95) 6%, transparent 55%)' }} />
                <div className="absolute bottom-5 left-5 right-5">
                  <div className="font-mono text-[8.5px] uppercase tracking-[0.16em] text-amber-300/80">last played · {g.last}</div>
                  <div className="mt-1 truncate text-[19px] font-extrabold tracking-tight">{g.title.split(':')[0]}</div>
                  <div className="mt-0.5 font-mono text-[9px] text-white/40">{g.hours}h · {g.tier}</div>
                </div>
                {i === 0 && <span className="absolute left-5 top-5 grid h-10 w-10 place-items-center rounded-full bg-amber-300 text-[15px] text-[#130f0a] shadow-lg">▶</span>}
              </a>
            ))}
          </section>

          <section className="mb-6 grid grid-cols-[1.75fr_1fr] gap-5">
            <div>
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Jump back in</h2>
                <a href="/library" className="text-[11.5px] font-semibold text-white/35 hover:text-white/80">library →</a>
              </div>
              <ContinueGrid games={GAMES.slice(3, 11)} url="/18-game" />
            </div>
            <div className="space-y-5">
              <div>
                <h2 className="mb-2.5 text-[15px] font-bold">Recent activity</h2>
                <RecentList games={GAMES.slice(11, 19)} url="/18-game" />
              </div>
              <div>
                <h2 className="mb-2.5 text-[15px] font-bold">Waiting on you</h2>
                <WaitingOn games={GAMES.filter(g => g.status === 'tweak').slice(0, 2)} url="/18-game" />
              </div>
            </div>
          </section>
          <ActivityPanel limit={6} />
        </main>
      </div>
    </div>
  );
}
export function It18Game() {
  return <GamePageShell game={GAMES[2]}>{ { back: '/18' } }</GamePageShell>;
}
