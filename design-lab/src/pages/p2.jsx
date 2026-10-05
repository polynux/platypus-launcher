import React, { useState } from 'react';
import { GAMES } from '../data.js';

const hero = GAMES[1]; // Elden Ring Nightreign

const statusDot = {
  ready: 'bg-emerald-400',
  tweak: 'bg-amber-400',
  idle: 'bg-white/25',
};

export function OpsTonight() {
  const [selected, setSelected] = useState(hero.id);
  const game = GAMES.find((g) => g.id === selected) || hero;

  return (
    <div className="min-h-screen bg-[#0b0d12] text-white" style={{ fontFamily: 'Hanken Grotesk, sans-serif' }}>
      <div className="mx-auto flex min-h-screen max-w-[1400px]">
        {/* slim nav */}
        <nav className="flex w-16 flex-col items-center gap-8 border-r border-white/[0.06] py-6">
          <div className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-indigo-400 to-fuchsia-500 text-[15px] font-black">H</div>
          {['Library', 'Tonight', 'Fixes', 'Runners'].map((t, i) => (
            <button key={t} className={`h-9 w-9 rounded-xl text-[13px] font-black transition ${i === 1 ? 'bg-white/10 text-white' : 'text-white/25 hover:text-white/70'}`}>
              {['L', 'T', 'F', 'R'][i]}
            </button>
          ))}
          <div className="mt-auto h-2 w-2 rounded-full bg-emerald-400" title="scan idle" />
        </nav>

        {/* main */}
        <div className="flex-1">
          {/* hero: one game, tonight */}
          <section className="relative h-[380px] overflow-hidden">
            <img key={game.id} src={game.wide} alt="" className="absolute inset-0 h-full w-full object-cover opacity-60" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(90deg, #0b0d12 4%, rgba(11,13,18,.35) 55%, rgba(11,13,18,.85))' }} />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, #0b0d12 2%, transparent 45%)' }} />

            <div className="relative flex h-full flex-col justify-end px-10 pb-9">
              <div className="mb-3 flex items-center gap-3 text-[11px] font-semibold uppercase tracking-[0.2em] text-white/50">
                <span className="flex items-center gap-2"><span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> ready</span>
                <span>·</span><span>{game.store}</span><span>·</span><span>{game.runner}</span><span>·</span><span>{game.dx}</span>
              </div>
              <h1 className="max-w-xl text-[52px] font-extrabold leading-[1.02] tracking-[-0.02em]">{game.title}</h1>
              <p className="mt-3 max-w-lg text-[14px] leading-relaxed text-white/60">{game.verdict}</p>
              <div className="mt-6 flex items-center gap-3">
                <button className="rounded-xl bg-white px-7 py-3 text-[14px] font-bold text-[#0b0d12] transition hover:bg-white/85">Play now</button>
                <button className="rounded-xl border border-white/15 bg-white/5 px-5 py-3 text-[14px] font-semibold text-white/80 backdrop-blur transition hover:bg-white/10">Configure runner</button>
                <span className="ml-2 font-mono text-[11px] text-white/35 tabular-nums">{game.hours}h logged · last {game.last}</span>
              </div>
            </div>
          </section>

          {/* continue rail */}
          <section className="border-t border-white/[0.06] px-10 py-8">
            <div className="mb-5 flex items-baseline justify-between">
              <h2 className="text-[17px] font-bold tracking-tight">Pick up tonight</h2>
              <button className="text-[12px] font-semibold text-white/40 transition hover:text-white/80">Full library →</button>
            </div>
            <div className="grid grid-cols-5 gap-4">
              {GAMES.slice(0, 5).map((g) => (
                <button
                  key={g.id}
                  onClick={() => setSelected(g.id)}
                  className={`group relative overflow-hidden rounded-2xl text-left ring-1 transition ${selected === g.id ? 'ring-white/60' : 'ring-white/[0.06] hover:ring-white/25'}`}
                >
                  <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover transition duration-500 group-hover:scale-105" />
                  <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(11,13,18,.95) 8%, transparent 55%)' }} />
                  <div className="absolute inset-x-0 bottom-0 p-3">
                    <div className="truncate text-[12.5px] font-bold leading-tight">{g.title}</div>
                    <div className="mt-1 flex items-center gap-1.5 font-mono text-[9px] uppercase tracking-wider text-white/45">
                      <span className={`h-1 w-1 rounded-full ${statusDot[g.status]}`} />
                      {g.tier} · {g.last}
                    </div>
                  </div>
                </button>
              ))}
            </div>
          </section>

          {/* two quiet columns */}
          <section className="grid grid-cols-[1.6fr_1fr] gap-10 border-t border-white/[0.06] px-10 py-8 pb-14">
            <div>
              <h2 className="mb-4 text-[17px] font-bold tracking-tight">Recently played</h2>
              <ul className="divide-y divide-white/[0.06]">
                {GAMES.slice(5, 11).map((g) => (
                  <li key={g.id} className="group flex items-center gap-4 py-2.5">
                    <img src={g.cover} alt="" className="h-10 w-10 rounded-lg object-cover ring-1 ring-white/10" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[13px] font-semibold">{g.title}</div>
                      <div className="font-mono text-[10px] text-white/35">{g.runner} · {g.dx}</div>
                    </div>
                    <span className="font-mono text-[11px] tabular-nums text-white/40">{g.hours}h</span>
                    <button className="rounded-lg bg-white/5 px-3 py-1.5 text-[11px] font-bold text-white/70 opacity-0 transition group-hover:opacity-100 hover:bg-white/15">Play</button>
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <h2 className="mb-4 text-[17px] font-bold tracking-tight">Attention needed</h2>
              <div className="space-y-3">
                {GAMES.filter((g) => g.status === 'tweak').slice(0, 3).map((g) => (
                  <div key={g.id} className="rounded-2xl border border-amber-400/15 bg-amber-400/[0.05] p-4">
                    <div className="flex items-center justify-between">
                      <span className="text-[13px] font-bold">{g.title}</span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-amber-300/80">{g.tier}</span>
                    </div>
                    <p className="mt-1.5 text-[11.5px] leading-relaxed text-white/50 line-clamp-2">{g.verdict}</p>
                  </div>
                ))}
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}