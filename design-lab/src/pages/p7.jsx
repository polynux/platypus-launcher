import React, { useState } from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /7 — GOG Galaxy-inspired warm gradient dashboard for Hydra:
   colored radial wash, slim icon+list sidebar, summary plates,
   recently-played rails with avatar chips, dense table.
   Game page: two-pane with media top, settings form inline.
   ============================================================ */

const game = GAMES[8]; // Hades II for game page

export function GalaxyDashboard() {
  return (
    <div
      className="min-h-screen text-[#eae6df]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1200px 700px at 12% -10%, rgba(147,90,60,.55), transparent 60%), radial-gradient(1000px 600px at 95% 15%, rgba(64,48,110,.5), transparent 55%), radial-gradient(900px 500px at 55% 110%, rgba(28,74,94,.45), transparent 60%), #17120e',
      }}
    >
      <div className="flex">
        {/* sidebar */}
        <aside className="flex h-screen w-56 flex-none flex-col border-r border-white/[0.07] bg-black/25 backdrop-blur-sm">
          <div className="flex items-center gap-2.5 px-5 py-5">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-amber-300 to-rose-500 text-[14px] font-black text-black">H</span>
            <span className="text-[15px] font-bold tracking-tight">Hydra</span>
          </div>
          <div className="px-5 pb-3 text-[10px] font-bold uppercase tracking-[0.18em] text-white/35">Games</div>
          <nav className="text-[13.5px]">
            {[['Recent', '1429', true], ['Owned games', '1429', false], ['Installed', '212', false], ['GOG.com', '486', false], ['Steam', '740', false], ['Epic', '203', false]].map(([label, count, active]) => (
              <button key={label} className={`flex w-full items-center justify-between px-5 py-2 ${active ? 'bg-white/[0.08] font-bold text-white' : 'text-white/55 hover:text-white'}`}>
                <span>{label}</span>
                <span className="rounded-full bg-white/10 px-2 py-0.5 font-mono text-[10px]">{count}</span>
              </button>
            ))}
          </nav>
          <div className="px-5 pb-2 pt-5 text-[10px] font-bold uppercase tracking-[0.18em] text-white/35">Friends</div>
          {['Activity', 'Invites', 'Friends online'].map((label, i) => (
            <button key={label} className="flex w-full items-center justify-between px-5 py-2 text-[13.5px] text-white/55 hover:text-white">
              <span>{label}</span>
              {i === 1 && <span className="rounded-full bg-amber-400 px-1.5 font-mono text-[9px] font-bold text-black">9</span>}
            </button>
          ))}
          <div className="mt-auto p-4 text-[10px] text-white/25">Hydra v0.2 · wayland</div>
        </aside>

        {/* main */}
        <div className="flex-1">
          {/* topbar */}
          <header className="flex items-center gap-4 px-8 py-4">
            <div className="flex-1" />
            <button className="rounded-lg bg-white/10 px-4 py-2 text-[13px] font-bold backdrop-blur hover:bg-white/15">＋ Add games & friends</button>
            <button className="rounded-lg bg-white/10 px-4 py-2 text-[13px] font-bold backdrop-blur hover:bg-white/15">Share stats</button>
          </header>

          <div className="space-y-9 px-8 pb-14">
            {/* recently played rails */}
            <section className="grid grid-cols-2 gap-10">
              <div>
                <div className="mb-3 flex items-baseline gap-3 text-[11px] font-bold uppercase tracking-[0.18em] text-white/40">
                  Recently played by me <span className="text-white/20">‹ ›</span>
                </div>
                <div className="flex gap-3.5">
                  {GAMES.slice(0, 4).map((g) => (
                    <a key={g.id} href="/7/game" className="group">
                      <img src={g.cover} alt="" className="aspect-[3/4] w-[104px] rounded-lg object-cover shadow-lg ring-1 ring-white/10 transition group-hover:-translate-y-1" />
                      <div className="mt-1.5 w-[104px] truncate text-[11px] font-semibold">{g.title.split(':')[0]}</div>
                    </a>
                  ))}
                </div>
              </div>
              <div>
                <div className="mb-3 text-[11px] font-bold uppercase tracking-[0.18em] text-white/40">Friends played this week</div>
                <div className="flex gap-3.5">
                  {GAMES.slice(4, 8).map((g, i) => (
                    <a key={g.id} className="group relative">
                      <img src={g.cover} alt="" className="aspect-[3/4] w-[104px] rounded-lg object-cover shadow-lg ring-1 ring-white/10 transition group-hover:-translate-y-1" />
                      <img src={`https://picsum.photos/seed/hyd-fr-${i}/48/48`} alt="" className="absolute -bottom-2 left-1/2 h-7 w-7 -translate-x-1/2 rounded-full ring-2 ring-[#17120e]" />
                    </a>
                  ))}
                </div>
              </div>
            </section>

            {/* summary plates */}
            <section>
              <div className="mb-3 flex items-baseline justify-between text-[11px] font-bold uppercase tracking-[0.18em] text-white/40">
                This month's summary
                <span className="rounded bg-white/10 px-2 py-1 text-[10px] text-white/60">Summary: Monthly ▾</span>
              </div>
              <div className="grid grid-cols-3 gap-5">
                {[
                  ['Games added', '+12', '1429 total', 'text-amber-300', 60],
                  ['Achievements', '+37', '5513 total', 'text-emerald-300', 74],
                  ['Hours played', '97h', '5 728h total', 'text-sky-300', 45],
                ].map(([label, delta, total, cls, w]) => (
                  <div key={label} className="rounded-xl border border-white/[0.06] bg-white/[0.045] p-5 backdrop-blur-sm">
                    <div className={`text-[14px] font-bold ${cls}`}>{delta} {label.toLowerCase()} <span className="font-normal text-white/40">({total})</span></div>
                    <div className="mt-4 flex h-10 items-end gap-1.5">
                      {HOUR_BARS.map((h, i) => (
                        <span key={i} className="w-2.5 rounded-sm bg-white/12" style={{ height: `${(h / 10) * w}%`, minHeight: 3, opacity: i === 13 ? 1 : 0.55 }} />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </section>

            {/* recently added table */}
            <section className="rounded-xl border border-white/[0.06] bg-black/20 backdrop-blur-sm">
              <div className="flex gap-8 border-b border-white/[0.07] px-5 py-3 font-mono text-[10px] uppercase tracking-widest text-white/35">
                <span className="flex-1">Recently added</span><span>rating</span><span>tags</span><span>platform</span>
              </div>
              {GAMES.slice(2, 12).map((g) => (
                <div key={g.id} className="group grid grid-cols-[24px_1fr_6rem_8rem_5rem] items-center gap-3 px-5 py-2.5 hover:bg-white/[0.05]">
                  <span className={`h-2 w-2 rounded-full ${g.tier === 'platinum' ? 'bg-amber-300' : g.tier === 'gold' ? 'bg-orange-400' : 'bg-white/20'}`} />
                  <a href="/7/game" className="truncate text-[13.5px] font-semibold group-hover:underline">{g.title}</a>
                  <span className="font-mono text-[11px] text-white/40">{g.tier}</span>
                  <span className="truncate text-[11px] text-white/35">{g.runner}</span>
                  <span className="text-right font-mono text-[10px] text-white/30">{g.store}</span>
                </div>
              ))}
            </section>

            {/* activity strip */}
            <section className="grid grid-cols-[1.5fr_1fr] gap-6">
              <div className="rounded-xl border border-white/[0.06] bg-black/20 p-5 backdrop-blur-sm">
                <h3 className="mb-2 text-[14px] font-bold">Activity</h3>
                <ActivityFeed limit={5} />
              </div>
              <div className="space-y-3">
                <h3 className="text-[14px] font-bold">Deals on your wishlist</h3>
                {FIXES.slice(3, 6).map((f) => (
                  <div key={f.game} className="rounded-lg border border-white/[0.06] bg-white/[0.04] px-4 py-3 text-[12.5px]">
                    <span className="font-bold">{f.game}</span>
                    <span className="ml-2 text-white/40">{f.tier}</span>
                  </div>
                ))}
              </div>
            </section>
          </div>
        </div>
      </div>
    </div>
  );
}

export function GalaxyGame() {
  const { list, unlockedCount, total } = achievementsFor(game);
  const [runner, setRunner] = useState('GE-Proton10-4');
  return (
    <div
      className="min-h-screen text-[#eae6df]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1100px 600px at 80% -10%, rgba(64,48,110,.55), transparent 60%), radial-gradient(900px 500px at 10% 100%, rgba(147,90,60,.5), transparent 55%), #17120e',
      }}
    >
      <div className="flex">
        <aside className="flex h-screen w-56 flex-none flex-col border-r border-white/[0.07] bg-black/25">
          <div className="px-5 py-5 text-[15px] font-bold">Hydra</div>
          {[['Dashboard', '/7'], ['Game page', null], ['Achievements', null], ['Activity', null]].map(([label, href]) => (
            <a key={label} href={href} className="px-5 py-2 text-[13.5px] text-white/55 hover:text-white">{label}</a>
          ))}
          <div className="mt-auto p-4 text-[10px] text-white/25">← back to dashboard</div>
        </aside>

        <div className="flex-1">
          {/* media head */}
          <section className="relative h-[300px] overflow-hidden">
            <img src={game.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, #17120e 4%, rgba(23,18,14,.35) 55%, rgba(23,18,14,.6))' }} />
            <div className="absolute bottom-5 left-8 right-8 flex items-end justify-between">
              <div>
                <div className="font-mono text-[10px] uppercase tracking-[0.2em] text-white/55">{game.store} · {game.tier} · {game.size}</div>
                <h1 className="mt-1 text-[36px] font-extrabold tracking-tight">{game.title}</h1>
              </div>
              <button className="rounded-xl bg-amber-300 px-7 py-3 text-[14px] font-black text-black transition hover:bg-amber-200">▶ Play</button>
            </div>
          </section>

          <div className="grid grid-cols-[1.5fr_1fr] gap-8 px-8 py-7">
            <div className="space-y-7">
              {/* pipeline settings */}
              <section>
                <h2 className="mb-3 text-[15px] font-bold">Runner & pipeline</h2>
                <div className="rounded-xl border border-white/[0.07] bg-black/25 p-5 backdrop-blur-sm">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-widest text-white/40">Runner</label>
                      <select value={runner} onChange={(e) => setRunner(e.target.value)} className="w-full rounded-lg border border-white/10 bg-black/40 px-3 py-2.5 text-[13px] text-white focus:outline-none focus:border-amber-300/60">
                        {RUNNERS.map((r) => <option key={r.name}>{r.name}</option>)}
                      </select>
                    </div>
                    <div>
                      <label className="mb-1.5 block text-[10px] font-bold uppercase tracking-widest text-white/40">Wine prefix</label>
                      <input defaultValue="~/.hydra/prefixes/hades-2" className="w-full rounded-lg border border-white/10 bg-black/40 px-3 py-2.5 font-mono text-[12px] text-white focus:outline-none focus:border-amber-300/60" />
                    </div>
                  </div>
                  <div className="mt-4 grid grid-cols-4 gap-2">
                    {['GameMode', 'MangoHud', 'Gamescope', 'EAC bridge'].map((t, i) => (
                      <button key={t} className={`rounded-lg px-3 py-2 text-[11.5px] font-bold ${i < 2 ? 'bg-amber-300/20 text-amber-200 ring-1 ring-amber-300/40' : 'bg-white/[0.06] text-white/50'}`}>{t}</button>
                    ))}
                  </div>
                  <div className="mt-4 rounded-lg bg-black/40 p-3 font-mono text-[11px] leading-relaxed text-emerald-200/90">
                    {PIPELINE.slice(0, 4).map((p) => p.chain).join('  →  ')}
                  </div>
                </div>
              </section>

              {/* screenshots */}
              <section>
                <h2 className="mb-3 text-[15px] font-bold">Screenshots</h2>
                <div className="grid grid-cols-2 gap-3">
                  {SCREENS(game).map((src, i) => (
                    <img key={i} src={src} alt="" className="aspect-video w-full rounded-lg object-cover ring-1 ring-white/10" />
                  ))}
                </div>
              </section>
            </div>

            {/* right pane */}
            <aside className="space-y-5">
              <div className="rounded-xl border border-white/[0.07] bg-black/25 p-5 backdrop-blur-sm">
                <h3 className="mb-3 text-[13px] font-bold">Achievements</h3>
                <div className="flex items-center gap-3">
                  <span className="text-[26px] font-extrabold text-amber-300">{unlockedCount}<span className="text-[14px] text-white/40">/{ACH_total(total)}</span></span>
                  <div className="flex-1">
                    <div className="h-1.5 rounded-full bg-white/10"><div className="h-full rounded-full bg-amber-300" style={{ width: `${(unlockedCount / ACH_total(total)) * 100}%` }} /></div>
                  </div>
                </div>
                <div className="mt-4 grid grid-cols-5 gap-2">
                  {list.slice(0, 5).map((a) => (
                    <div key={a.id} className={a.unlocked ? '' : 'opacity-20 grayscale'}>
                      <div className={`grid aspect-square place-items-center rounded-lg text-[18px] ${a.unlocked ? 'bg-amber-300/15 ring-1 ring-amber-300/40' : 'bg-white/[0.05]'}`}>★</div>
                    </div>
                  ))}
                </div>
              </div>
              <div className="rounded-xl border border-white/[0.07] bg-black/25 p-5 font-mono text-[12px] backdrop-blur-sm">
                {[['Runner', game.runner], ['DX layer', game.dx], ['Playtime', `${game.hours}h`], ['Last played', game.last], ['Storage', game.size]].map(([k, v]) => (
                  <div key={k} className="flex justify-between border-b border-white/[0.06] py-2 last:border-0">
                    <span className="text-white/35">{k}</span><span className="text-white/85">{v}</span>
                  </div>
                ))}
              </div>
            </aside>
          </div>
        </div>
      </div>
    </div>
  );
}
const ACH_total = (t) => t;