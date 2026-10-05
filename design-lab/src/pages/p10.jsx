import React, { useState } from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /10 — "Tactile dark utility": Cirno's close cousin but its
   own thing — deep slate, soft lime accent, chunky rounded
   cards with real depth, horizontal feel. Dashboard feels
   like a console OS home; game page = big media + panels.
   ============================================================ */

const game = GAMES[5]; // Factorio? no — use metro
const g = GAMES[13] || GAMES[5];

export function UtilityDashboard() {
  return (
    <div className="min-h-screen bg-[#101316] text-[#e7eaee]" style={{ fontFamily: '"Hanken Grotesk", sans-serif' }}>
      <div className="flex">
        {/* wide sidebar */}
        <aside className="flex h-screen w-60 flex-none flex-col border-r border-white/[0.06] bg-[#15181c] p-4">
          <div className="flex items-center gap-3 px-2 py-3">
            <span className="grid h-10 w-10 place-items-center rounded-xl bg-lime-300 text-[17px] font-black text-[#101316]">H</span>
            <div>
              <div className="text-[15px] font-extrabold tracking-tight">hydra</div>
              <div className="font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">game hub</div>
            </div>
          </div>

          <div className="my-4 rounded-xl bg-[#1c2025] px-4 py-2.5 text-[12.5px] text-white/35">🔍 Search…</div>

          <nav className="text-[13.5px]">
            <div className="px-2 pb-2 pt-3 font-mono text-[9px] uppercase tracking-[0.16em] text-white/25">play</div>
            {[['Dashboard', true], ['All games', false], ['Downloads', false]].map(([l, a]) => (
              <button key={l} className={`mb-0.5 w-full rounded-lg px-3 py-2.5 text-left font-semibold ${a ? 'bg-lime-300/10 text-lime-300' : 'text-white/50 hover:bg-white/[0.04] hover:text-white/85'}`}>{l}</button>
            ))}
            <div className="px-2 pb-2 pt-4 font-mono text-[9px] uppercase tracking-[0.16em] text-white/25">tools</div>
            {['Achievements', 'Activity', 'Runners', 'Fix DB'].map((l) => (
              <button key={l} className="mb-0.5 w-full rounded-lg px-3 py-2.5 text-left font-semibold text-white/50 hover:bg-white/[0.04] hover:text-white/85">{l}</button>
            ))}
          </nav>

          <div className="mt-auto rounded-xl bg-[#1c2025] p-3.5">
            <div className="flex items-center justify-between text-[11px] font-bold">
              <span className="text-lime-300">● online</span>
              <span className="font-mono text-white/30">2 friends</span>
            </div>
            <div className="mt-2.5 flex -space-x-2">
              {[1, 2, 3, 4].map((i) => (
                <img key={i} src={`https://picsum.photos/seed/hyd-f${i}/48/48`} alt="" className="h-7 w-7 rounded-full ring-2 ring-[#1c2025]" />
              ))}
              <span className="grid h-7 w-7 place-items-center rounded-full bg-white/10 text-[10px] font-bold ring-2 ring-[#1c2025]">+9</span>
            </div>
          </div>
        </aside>

        <main className="flex-1 p-7">
          {/* greeting bar */}
          <div className="mb-6 flex items-center justify-between">
            <div>
              <h1 className="text-[26px] font-extrabold tracking-tight">Hey Polynux 👋</h1>
              <p className="mt-0.5 text-[13.5px] text-white/45">Helldivers 2 got a new patch — one fix needs re-applying before you play.</p>
            </div>
            <button className="rounded-xl bg-lime-300 px-5 py-2.5 text-[13.5px] font-extrabold text-[#101316]">＋ Add game</button>
          </div>

          {/* big feature card */}
          <section className="relative mb-7 overflow-hidden rounded-3xl">
            <img src={GAMES[11].wide} alt="" className="aspect-[2.9/1] w-full object-cover" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(80deg, rgba(16,19,22,.94) 22%, rgba(16,19,22,.2) 60%, transparent)' }} />
            <div className="absolute bottom-6 left-7 max-w-sm">
              <span className="rounded-md bg-orange-400 px-2 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-black">fix needed</span>
              <h2 className="mt-2.5 text-[26px] font-extrabold tracking-tight">{GAMES[11].title}</h2>
              <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60 line-clamp-2">{GAMES[11].verdict}</p>
              <div className="mt-3.5 flex gap-2.5">
                <a href="/10/game" className="rounded-xl bg-white px-4 py-2 text-[12.5px] font-extrabold text-[#101316]">Open page</a>
                <button className="rounded-xl bg-white/10 px-4 py-2 text-[12.5px] font-bold text-white/85 backdrop-blur">Apply fix</button>
              </div>
            </div>
          </section>

          {/* three-panel row */}
          <section className="mb-7 grid grid-cols-[1.2fr_1fr_1fr] gap-4">
            {/* continue */}
            <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
              <h3 className="mb-3.5 text-[13.5px] font-bold">Continue playing</h3>
              <div className="space-y-2">
                {GAMES.slice(0, 4).map((gm) => (
                  <a key={gm.id} href="/10/game" className="group flex items-center gap-3 rounded-xl p-1.5 transition hover:bg-white/[0.05]">
                    <img src={gm.cover} alt="" className="h-11 w-11 rounded-lg object-cover" />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-[12.5px] font-bold">{gm.title}</div>
                      <div className="font-mono text-[9.5px] text-white/35">{gm.hours}h · {gm.last}</div>
                    </div>
                    <span className="grid h-8 w-8 place-items-center rounded-lg bg-white/[0.06] text-[10px] opacity-0 transition group-hover:opacity-100">▶</span>
                  </a>
                ))}
              </div>
            </div>

            {/* week chart */}
            <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
              <div className="flex items-baseline justify-between">
                <h3 className="text-[13.5px] font-bold">This week</h3>
                <span className="font-mono text-[10px] text-white/30">12h 41m</span>
              </div>
              <div className="mt-4 flex h-24 items-end gap-1.5">
                {HOUR_BARS.slice(0, 10).map((h, i) => (
                  <div key={i} className={`flex-1 rounded-md ${i === 9 ? 'bg-lime-300' : 'bg-white/[0.09]'}`} style={{ height: `${(h / 10) * 100}%`, minHeight: 4 }} />
                ))}
              </div>
              <div className="mt-3 border-t border-white/[0.06] pt-3 font-mono text-[10px] text-white/35">
                14 sessions · avg 54 min
              </div>
            </div>

            {/* achievements snapshot */}
            <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
              <div className="flex items-baseline justify-between">
                <h3 className="text-[13.5px] font-bold">Latest unlocks</h3>
                <span className="font-mono text-[10px] text-amber-300">+45 pts</span>
              </div>
              <div className="mt-3.5 space-y-3">
                {['Night Shift', 'Marathon', 'Full Sync'].map((n, i) => (
                  <div key={n} className="flex items-center gap-3">
                    <span className="grid h-9 w-9 place-items-center rounded-lg bg-amber-300/12 text-[15px] ring-1 ring-amber-300/30">★</span>
                    <div>
                      <div className="text-[12px] font-bold">{n}</div>
                      <div className="font-mono text-[9.5px] text-white/30">{['yesterday', 'today', '2 days ago'][i]}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>

          {/* activity strip */}
          <section className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-[13.5px] font-bold">Activity</h3>
              <button className="font-mono text-[10px] uppercase tracking-widest text-white/30 hover:text-white/70">full log →</button>
            </div>
            <ActivityFeed limit={4} />
          </section>
        </main>
      </div>
    </div>
  );
}

export function UtilityGame() {
  const { list, unlockedCount, total, score } = achievementsFor(g);
  const [tab, setTab] = useState('overview');
  return (
    <div className="min-h-screen bg-[#101316] text-[#e7eaee]" style={{ fontFamily: '"Hanken Grotesk", sans-serif' }}>
      {/* banner */}
      <section className="relative h-[330px]">
        <img src={g.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, #101316 5%, rgba(16,19,22,.25) 55%, rgba(16,19,22,.55))' }} />
        <div className="absolute bottom-6 left-8 right-8 flex items-end justify-between">
          <div>
            <div className="mb-2 flex gap-2">
              <span className="rounded-lg bg-lime-300 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#101316]">{g.tier} on linux</span>
              <span className="rounded-lg bg-white/10 px-2.5 py-1 font-mono text-[9px] font-bold uppercase tracking-wider text-white/75 backdrop-blur">{g.store}</span>
            </div>
            <h1 className="text-[40px] font-extrabold leading-none tracking-tight">{g.title}</h1>
          </div>
          <button className="flex items-center gap-2 rounded-2xl bg-lime-300 px-8 py-3.5 text-[15px] font-extrabold text-[#101316] shadow-[0_10px_30px_rgba(163,230,53,.25)] transition hover:bg-lime-200">▶ Play now</button>
        </div>
      </section>

      {/* tab pills */}
      <div className="sticky top-0 z-10 border-b border-white/[0.06] bg-[#101316]/90 px-8 py-3 backdrop-blur">
        <div className="flex gap-2">
          {['overview', 'launcher settings', 'achievements', 'activity'].map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`rounded-xl px-4 py-2 text-[12.5px] font-bold capitalize transition ${tab === t ? 'bg-white/10 text-white' : 'text-white/40 hover:text-white/80'}`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      <main className="grid max-w-[1200px] grid-cols-[1.5fr_1fr] gap-6 p-8">
        {tab === 'overview' && (
          <>
            <div className="space-y-4">
              <img src={SCREENS(g)[0]} alt="" className="aspect-video w-full rounded-2xl object-cover ring-1 ring-white/[0.06]" />
              <div className="grid grid-cols-3 gap-4">
                {SCREENS(g, 3).map((s, i) => <img key={i} src={s} alt="" className="aspect-video w-full rounded-xl object-cover ring-1 ring-white/[0.06]" />)}
              </div>
              <p className="rounded-2xl bg-[#171b1f] p-5 text-[13.5px] leading-relaxed text-white/65 ring-1 ring-white/[0.05]">{g.verdict}</p>
            </div>
            <aside className="space-y-4">
              <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
                <h3 className="mb-3 text-[13.5px] font-bold">Facts</h3>
                {[['Runner', g.runner], ['DX layer', g.dx], ['Playtime', `${g.hours}h`], ['Last session', g.last], ['Storage', g.size], ['Store', g.store]].map(([k, v]) => (
                  <div key={k} className="flex justify-between border-b border-white/[0.05] py-2 text-[12.5px] last:border-0">
                    <span className="text-white/35">{k}</span><span className="font-semibold">{v}</span>
                  </div>
                ))}
              </div>
              <div className="rounded-2xl bg-lime-300/[0.07] p-5 ring-1 ring-lime-300/20">
                <h3 className="text-[13px] font-extrabold text-lime-300">Ready to launch</h3>
                <p className="mt-1.5 text-[12px] leading-relaxed text-white/55">Pipeline verified 2h ago. No action needed — just press play.</p>
              </div>
            </aside>
          </>
        )}

        {tab === 'launcher settings' && (
          <>
            <div className="space-y-4">
              <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
                <h3 className="mb-4 text-[13.5px] font-bold">Runner</h3>
                <div className="grid grid-cols-2 gap-3">
                  {RUNNERS.slice(0, 4).map((r, i) => (
                    <button key={r.name} className={`rounded-xl border p-3.5 text-left transition ${i === 0 ? 'border-lime-300/60 bg-lime-300/[0.07]' : 'border-white/[0.07] hover:border-white/20'}`}>
                      <div className="text-[13px] font-bold">{r.name}</div>
                      <div className="mt-0.5 font-mono text-[9.5px] text-white/35">{r.kind} · {r.updated}</div>
                    </button>
                  ))}
                </div>
              </div>
              <div className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
                <h3 className="mb-3 text-[13.5px] font-bold">Pipeline — live preview</h3>
                <div className="space-y-1.5 font-mono text-[11.5px]">
                  {PIPELINE.map((p, i) => (
                    <div key={i} className="flex items-center gap-3 rounded-lg bg-black/30 px-3.5 py-2.5">
                      <span className="text-white/25">{i + 1}</span>
                      <span className="flex-1 truncate text-lime-200/90">{p.chain}</span>
                      <span className="truncate text-white/25">{p.env.split('=')[0]}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <aside className="rounded-2xl bg-[#171b1f] p-5 ring-1 ring-white/[0.05]">
              <h3 className="mb-3 text-[13.5px] font-bold">Session options</h3>
              <div className="space-y-2">
                {[['Feral GameMode', true], ['MangoHud', true], ['Gamescope', false], ['EAC bridge', true], ['Shader pre-caching', true]].map(([l, on]) => (
                  <div key={l} className="flex items-center justify-between rounded-xl bg-black/25 px-4 py-3">
                    <span className="text-[12.5px] font-semibold">{l}</span>
                    <span className={`relative h-5 w-9 rounded-full transition ${on ? 'bg-lime-300' : 'bg-white/15'}`}>
                      <span className="absolute top-0.5 h-4 w-4 rounded-full bg-[#101316]" style={{ left: on ? 18 : 2 }} />
                    </span>
                  </div>
                ))}
              </div>
            </aside>
          </>
        )}

        {tab === 'achievements' && (
          <>
            <div className="rounded-2xl bg-[#171b1f] p-6 ring-1 ring-white/[0.05]">
              <div className="mb-5 flex items-center gap-5">
                <div className="grid h-24 w-24 flex-none place-items-center rounded-full bg-lime-300/[0.09] text-[24px] font-black text-lime-300 ring-[3px] ring-lime-300/40">{score}%</div>
                <div>
                  <div className="text-[17px] font-extrabold">{unlockedCount} unlocked</div>
                  <div className="font-mono text-[11px] text-white/35">of ~{total} · latest {list.find((a) => a.unlocked)?.date}</div>
                </div>
              </div>
              <div className="grid grid-cols-4 gap-3">
                {list.map((a) => (
                  <div key={a.id} className={`rounded-xl bg-black/25 p-3.5 text-center ring-1 ${a.unlocked ? 'ring-amber-300/25' : 'opacity-30 ring-white/[0.05]'}`}>
                    <div className={`mx-auto grid h-11 w-11 place-items-center rounded-lg text-[17px] ${a.unlocked ? 'bg-amber-300/15' : 'bg-white/[0.05]'}`}>★</div>
                    <div className="mt-2 truncate text-[10.5px] font-bold">{a.name}</div>
                    <div className="font-mono text-[8.5px] text-white/30">{a.unlocked ? a.date : a.rarity}</div>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}

        {tab === 'activity' && (
          <div className="col-span-2 rounded-2xl bg-[#171b1f] p-6 ring-1 ring-white/[0.05]">
            <ActivityFeed limit={9} />
          </div>
        )}
      </main>
    </div>
  );
}