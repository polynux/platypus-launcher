import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, achievementsFor, SCREENS, HOUR_BARS,
  protonSettingsFor, PROTON_FLAGS, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS,
} from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /13 — ITERATION 2: density. Galaxy's information with the
   gutters tightened: two-row summary grid, table-first
   library, compact activity with inline covers. For people
   who want everything visible at once.
   ============================================================ */

export function It13Dashboard() {
  return (
    <div
      className="min-h-screen text-[#eae4da]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(900px 500px at 10% -8%, rgba(150,96,58,.45), transparent 55%), radial-gradient(800px 480px at 90% 8%, rgba(86,62,128,.44), transparent 52%), #141009',
      }}
    >
      <div className="mx-auto flex max-w-[1620px]">
        {/* slim stat sidebar + library table = the whole page */}
        <aside className="sticky top-0 flex h-screen w-52 flex-none flex-col border-r border-white/[0.06] bg-black/25 p-4">
          <div className="mb-5 flex items-center gap-2.5">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-amber-300 to-rose-500 text-[13px] font-black text-[#141009]">P</span>
            <span className="text-[14px] font-extrabold">Platypus</span>
          </div>
          {[['Dashboard', true], ['All games', false], ['Downloads', false], ['Achievements', false], ['Activity', false], ['Runners', false]].map(([l, a]) => (
            <button key={l} className={`mb-0.5 rounded-lg px-3 py-2 text-left text-[12.5px] font-semibold ${a ? 'bg-amber-300/12 text-amber-200' : 'text-white/50 hover:text-white'}`}>{l}</button>
          ))}
          <div className="mt-auto space-y-1.5 font-mono text-[10px] text-white/30">
            <div className="flex justify-between"><span>DISK</span><span>1.2TB</span></div>
            <div className="flex justify-between"><span>STEAM</span><span>online</span></div>
            <div className="flex justify-between"><span>SCAN</span><span>09:41</span></div>
          </div>
        </aside>

        <main className="flex-1 p-6">
          <header className="mb-5 flex items-center justify-between">
            <h1 className="text-[19px] font-extrabold tracking-tight">Dashboard <span className="ml-2 font-mono text-[10px] font-normal uppercase tracking-widest text-white/30">week 41</span></h1>
            <div className="flex gap-2">
              <button className="rounded-lg bg-white/[0.07] px-3.5 py-2 text-[12px] font-bold text-white/70 hover:bg-white/[0.12]">Share stats</button>
              <button className="rounded-lg bg-amber-300 px-3.5 py-2 text-[12px] font-extrabold text-[#141009]">＋ Add</button>
            </div>
          </header>

          {/* two-row summary grid — six compact tiles */}
          <section className="mb-5 grid grid-cols-6 gap-px overflow-hidden rounded-2xl ring-1 ring-white/[0.07]">
            {[
              ['Hours this week', '12.7', '+3.1', 'text-amber-300'],
              ['Sessions', '14', '+3', 'text-emerald-300'],
              ['Achievement pts', '45', 'this wk', 'text-amber-300'],
              ['Updates', '5', 'today', 'text-sky-300'],
              ['Fixes waiting', '3', '2 stale', 'text-orange-300'],
              ['Disk added', '148G', 'this mo', 'text-white/80'],
            ].map(([k, v, s, c]) => (
              <div key={k} className="bg-[#1d1810]/90 p-4">
                <div className="font-mono text-[9px] uppercase tracking-[0.14em] text-white/35">{k}</div>
                <div className={`mt-1.5 text-[22px] font-extrabold tabular-nums leading-none ${c}`}>{v}</div>
                <div className="mt-1 font-mono text-[9px] text-white/25">{s}</div>
              </div>
            ))}
          </section>

          {/* the everything-table: library IS the dashboard */}
          <section className="mb-5 overflow-hidden rounded-2xl bg-black/25 ring-1 ring-white/[0.06]">
            <div className="flex items-center gap-4 border-b border-white/[0.07] px-4 py-2.5">
              <h2 className="text-[13px] font-bold">Library</h2>
              <span className="rounded bg-white/[0.06] px-2 py-0.5 font-mono text-[9px] uppercase tracking-wider text-white/45">24 titles · sorted: recent play</span>
              <input placeholder="filter…" className="ml-auto w-44 rounded-lg bg-white/[0.06] px-3 py-1.5 font-mono text-[11px] text-white placeholder-white/30 focus:outline-none focus:ring-1 focus:ring-amber-300/50" />
            </div>
            <div className="grid grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem] gap-3 border-b border-white/[0.05] px-4 py-2 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">
              <span /><span>title</span><span>tier</span><span>runner</span><span className="text-right">hrs</span><span className="text-right">size</span><span className="text-right">last played</span>
            </div>
            {GAMES.map((g, i) => (
              <a key={g.id} href="/13/game" className={`group grid grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem] items-center gap-3 px-4 py-2 ${i % 2 ? 'bg-white/[0.015]' : ''} hover:bg-amber-300/[0.05]`}>
                <span className={`h-1.5 w-1.5 rounded-full ${g.tier === 'platinum' ? 'bg-amber-300' : g.tier === 'gold' ? 'bg-orange-400' : g.tier === 'bronze' ? 'bg-orange-200/50' : 'bg-white/15'}`} />
                <span className="flex items-center gap-2.5">
                  <img src={g.cover} alt="" className="h-7 w-7 flex-none rounded object-cover ring-1 ring-white/10" />
                  <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
                </span>
                <span className="font-mono text-[10px] text-white/40">{g.tier}</span>
                <span className="truncate font-mono text-[10px] text-white/30">{g.runner}</span>
                <span className="text-right font-mono text-[10.5px] tabular-nums text-white/55">{g.hours}</span>
                <span className="text-right font-mono text-[10px] text-white/30">{g.size.split(' ')[0]}</span>
                <span className="text-right font-mono text-[10px] text-white/30">{g.last}</span>
              </a>
            ))}
          </section>

          {/* bottom strip: activity w/ covers + achievements ticker */}
          <section className="grid grid-cols-[1.5fr_1fr] gap-5 pb-8">
            <div className="rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
              <h2 className="mb-1.5 text-[13px] font-bold">Activity</h2>
              <ul className="divide-y divide-white/[0.05]">
                {ACTIVITY.slice(0, 5).map((a, i) => (
                  <li key={i} className="flex items-center gap-3 py-2">
                    <span className="font-mono text-[9.5px] tabular-nums text-white/25">{a.ts}</span>
                    <img src={GAMES[(i * 3) % 20].cover} alt="" className="h-7 w-7 rounded object-cover ring-1 ring-white/10" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[12px]"><span className="font-mono text-[9px] uppercase tracking-wider text-amber-300/80">{a.type}</span> <span className="text-white/85">{a.detail}</span></div>
                    </div>
                    <span className="hidden font-mono text-[9.5px] text-white/30 md:block">{a.meta}</span>
                  </li>
                ))}
              </ul>
            </div>
            <div className="rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
              <h2 className="mb-3 text-[13px] font-bold">Recent unlocks</h2>
              <div className="space-y-2">
                {achievementsFor(GAMES[1]).list.filter(a => a.unlocked).slice(0, 4).map((a, i) => (
                  <div key={a.id} className="flex items-center gap-3">
                    <span className="grid h-8 w-8 flex-none place-items-center rounded-lg bg-amber-300/12 text-[13px] ring-1 ring-amber-300/30">★</span>
                    <div className="min-w-0">
                      <div className="truncate text-[12px] font-bold">{a.name}</div>
                      <div className="font-mono text-[9px] text-white/30">{a.date} · {a.rarity}</div>
                    </div>
                    <span className="ml-auto font-mono text-[9px] text-amber-300/70">+{15 - i * 5} pts</span>
                  </div>
                ))}
              </div>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export function It13Game() {
  const game = GAMES[11];
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sync, setSync] = useState('ntsync');
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [hud, setHud] = useState('fps only');
  const [ups, setUps] = useState('FSR 3');

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1">
      {options.map((o) => (
        <button key={o} onClick={() => onChange(o)} className={`rounded px-2.5 py-1 font-mono text-[10.5px] transition ${value === o ? 'bg-amber-300 font-bold text-[#141009]' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]'}`}>{o}</button>
      ))}
    </div>
  );

  return (
    <div
      className="min-h-screen text-[#eae4da]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(900px 500px at 90% -8%, rgba(86,62,128,.44), transparent 55%), #141009',
      }}
    >
      <div className="mx-auto flex max-w-[1380px]">
        <div className="min-w-0 flex-1 p-6">
          <a href="/13" className="text-[12px] font-semibold text-white/40 hover:text-white">← back to dashboard</a>

          {/* compact head row — no big banner, table-like */}
          <section className="mt-3 mb-5 flex items-center gap-4 rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
            <img src={game.cover} alt="" className="h-20 w-14 rounded-lg object-cover ring-1 ring-white/10" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 font-mono text-[9px] uppercase tracking-[0.14em] text-white/35">
                <span className="rounded bg-amber-300/15 px-1.5 py-0.5 text-amber-300">{game.tier}</span>
                <span>{game.store}</span><span>·</span><span>{game.size}</span>
              </div>
              <h1 className="mt-1 text-[22px] font-extrabold tracking-tight">{game.title}</h1>
            </div>
            <div className="text-right font-mono text-[10px] text-white/35">
              <div>{game.hours}h logged</div><div>last {game.last}</div>
            </div>
            <button className="rounded-xl bg-amber-300 px-6 py-3 text-[13.5px] font-extrabold text-[#141009]">▶ Play</button>
          </section>

          {/* pro settings grid — 3 columns */}
          <div className="grid grid-cols-3 gap-5 pb-8">
            <section className="space-y-4">
              <div className="rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
                <h2 className="mb-3 text-[13px] font-bold">Renderer</h2>
                <div className="space-y-3.5">
                  <div><div className="mb-1 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">dxvk</div><Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} /></div>
                  <div><div className="mb-1 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">vkd3d</div><Seg options={['VKD3D-Proton 2.14', 'VKD3D-Proton 2.13', 'off']} value={'VKD3D-Proton 2.14'} onChange={() => {}} /></div>
                  <div><div className="mb-1 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">sync</div><Seg options={SYNC_MODES} value={sync} onChange={setSync} /></div>
                  <div><div className="mb-1 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">upscaler</div><Seg options={UPSCALERS} value={ups} onChange={setUps} /></div>
                </div>
              </div>
            </section>

            <section className="rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[13px] font-bold">Proton flags</h2>
                <span className="font-mono text-[9px] text-white/30">{flags.filter(f => f.on).length}/{flags.length}</span>
              </div>
              <div className="space-y-1">
                {flags.map((f) => (
                  <button key={f.key} onClick={() => setFlags(flags.map(x => x.key === f.key ? { ...x, on: !x.on } : x))} className="flex w-full items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-left transition hover:bg-white/[0.04]">
                    <div className="min-w-0">
                      <div className="truncate font-mono text-[10.5px] font-bold text-white/85">{f.key.replace('PROTON_', '').replace(/_/g, ' ').toLowerCase()}</div>
                      <div className="truncate font-mono text-[8.5px] text-white/30">{f.key}</div>
                    </div>
                    <span className={`h-3.5 w-6 flex-none rounded-full transition ${f.on ? 'bg-amber-300' : 'bg-white/15'}`}>
                      <span className="block h-2.5 w-2.5 translate-x-0.5 translate-y-0.5 rounded-full bg-[#141009]" style={{ transform: f.on ? 'translateX(10px)' : '' }} />
                    </span>
                  </button>
                ))}
              </div>
            </section>

            <aside className="space-y-4">
              <div className="rounded-2xl bg-black/25 p-4 font-mono text-[10.5px] leading-relaxed ring-1 ring-white/[0.06]">
                <div className="mb-1 font-mono text-[9px] uppercase tracking-widest text-white/30">command</div>
                <div className="text-emerald-200/90">gamemoderun mangohud \</div>
                <div className="pl-3 text-emerald-200/90">proton run hd2.exe \</div>
                {flags.filter(f => f.on).map(f => <div key={f.key} className="pl-3 text-amber-200/80">{f.key}=1 \</div>)}
              </div>
              <div className="rounded-2xl bg-black/25 p-4 ring-1 ring-white/[0.06]">
                <h3 className="mb-2.5 text-[13px] font-bold">Achievements</h3>
                <div className="flex items-baseline gap-1.5">
                  <span className="text-[22px] font-extrabold text-amber-300">{ach.unlockedCount}</span>
                  <span className="font-mono text-[10px] text-white/30">/{ach.total}</span>
                </div>
                <div className="mt-2.5 h-1 rounded bg-white/[0.08]"><div className="h-full rounded bg-amber-300" style={{ width: `${ach.score}%` }} /></div>
                <div className="mt-3 grid grid-cols-5 gap-1.5">
                  {ach.list.slice(0, 10).map((a) => (
                    <div key={a.id} className={`grid aspect-square place-items-center rounded text-[11px] ${a.unlocked ? 'bg-amber-300/12 ring-1 ring-amber-300/35' : 'bg-white/[0.05] opacity-20'}`}>★</div>
                  ))}
                </div>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </div>
  );
}