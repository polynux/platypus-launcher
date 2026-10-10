import React from 'react';
import { GAMES } from '../data.js';
import { WX, Sidebar, ContinueGrid, WaitingOn, ActivityPanel, GamePageShell } from './shared16.jsx';

/* /21 — THE FINALIST ASSEMBLY: 16 dashboard + friends panel + a
   slimmed recent strip; game page default view; library at /library. */
export function It21Dashboard() {
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="flex">
        <Sidebar active="Dashboard" />
        <main className="flex-1 p-7">
          {/* hero with stats riding */}
          <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
            <img src={GAMES[11].wide} alt="" className="h-[290px] w-full object-cover" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)' }} />
            <div className="absolute bottom-6 left-7 max-w-md">
              <span className="rounded-lg bg-orange-400 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#130f0a]">action required</span>
              <h1 className="mt-2.5 text-[27px] font-extrabold leading-tight tracking-tight">Helldivers 2 needs its fix re-applied</h1>
              <div className="mt-3.5 flex gap-2.5">
                <a href="/21-game" className="rounded-xl bg-amber-300 px-5 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">Open game</a>
                <button className="rounded-xl bg-white/10 px-4 py-2.5 text-[12.5px] font-bold text-white/85">Later</button>
              </div>
            </div>
            <div className="absolute bottom-6 right-7 hidden gap-3 lg:flex">
              {[['hours', '12.7', 'text-amber-300'], ['pts', '1,240', 'text-amber-200'], ['ready', '19/24', 'text-emerald-300']].map(([k, v, c]) => (
                <div key={k} className="w-28 rounded-2xl bg-black/45 p-3.5 backdrop-blur">
                  <div className={`text-[19px] font-extrabold tabular-nums ${c}`}>{v}</div>
                  <div className="font-mono text-[8.5px] uppercase tracking-widest text-white/40">{k}</div>
                </div>
              ))}
            </div>
          </section>

          <section className="grid grid-cols-[1.75fr_1fr] gap-5">
            <div>
              {/* big last-played spotlight */}
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Continue playing</h2>
                <a href="/library" className="text-[11.5px] font-semibold text-white/35 hover:text-white/80">full library →</a>
              </div>
              <ContinueGrid games={GAMES.slice(0, 8)} url="/21-game" />
            </div>
            <div className="space-y-5">
              <div>
                <h2 className="mb-2.5 text-[15px] font-bold">Waiting on you</h2>
                <WaitingOn games={GAMES.filter(g => g.status === 'tweak').slice(0, 2)} url="/21-game" />
              </div>
              <div className="rounded-3xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
                <h2 className="mb-2.5 text-[14px] font-bold">Recently added</h2>
                <RecentMini />
              </div>
            </div>
          </section>

          <div className="mt-6">
            <ActivityPanel limit={7} />
          </div>
        </main>
      </div>
    </div>
  );
}

function RecentMini() {
  const G = GAMES.slice(6, 11);
  return (
    <ul className="divide-y divide-white/[0.05]">
      {G.map((g) => (
        <li key={g.id} className="flex items-center gap-3 py-2">
          <img src={g.cover} alt="" className="h-8 w-8 rounded object-cover ring-1 ring-white/10" />
          <div className="min-w-0 flex-1 leading-tight">
            <div className="truncate text-[12px] font-semibold">{g.title.split(':')[0]}</div>
            <div className="font-mono text-[8.5px] uppercase tracking-wider text-white/30">{g.store} · {g.last}</div>
          </div>
        </li>
      ))}
    </ul>
  );
}

export function It21Game() {
  return <GamePageShell game={GAMES[2]}>{ { back: '/21' } }</GamePageShell>;
}
