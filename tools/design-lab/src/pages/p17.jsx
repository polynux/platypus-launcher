import React from 'react';
import { GAMES } from '../data.js';
import { WX, Sidebar, HeroFix, StatChipsOnHero, ContinueGrid, WaitingOn, ActivityPanel, GamePageShell } from './shared16.jsx';

/* /17 — BASELINE: dashboard exactly /16 + friends panel from /12;
   game page = new default (shots+summary+achievements, settings in modal). */
export function It17Dashboard() {
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar active="Dashboard" />
        <main className="flex-1 p-7">
          <HeroFix url="/17-game" />
          <section className="mb-6 grid grid-cols-[1.75fr_1fr] gap-5">
            <div>
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Continue playing</h2>
                <a href="/library" className="text-[11.5px] font-semibold text-white/35 hover:text-white/80">library →</a>
              </div>
              <ContinueGrid games={GAMES.slice(0, 8)} url="/17-game" />
            </div>
            <div>
              <div className="mb-3"><h2 className="text-[16px] font-bold tracking-tight">Waiting on you</h2></div>
              <WaitingOn games={GAMES.filter(g => g.status === 'tweak').slice(0, 3)} url="/17-game" />
            </div>
          </section>
          <ActivityPanel limit={8} />
        </main>
      </div>
    </div>
  );
}
export function It17Game() {
  return <GamePageShell game={GAMES[8]}>{ { back: '/17' } }</GamePageShell>;
}
