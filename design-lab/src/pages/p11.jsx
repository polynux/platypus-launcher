import React, { useState } from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /11 — "Aurora glass": translucent panels over a slow color
   field, hairline strokes, floating pill nav — the premium
   'operating system for your library' look, kept disciplined.
   ============================================================ */

const FIELD = {
  backgroundColor: '#0b0d13',
  backgroundImage:
    'radial-gradient(900px 600px at 15% 20%, rgba(109,88,246,.28), transparent 60%),' +
    'radial-gradient(1000px 700px at 85% 10%, rgba(14,165,233,.22), transparent 55%),' +
    'radial-gradient(800px 500px at 70% 90%, rgba(236,72,153,.16), transparent 55%)',
  backgroundAttachment: 'fixed',
};

export function GlassDashboard() {
  return (
    <div className="min-h-screen text-[#eceef2]" style={{ ...FIELD, fontFamily: '"Hanken Grotesk", sans-serif' }}>
      {/* floating pill nav */}
      <nav className="sticky top-4 z-40 mx-auto mb-8 flex w-fit items-center gap-1 rounded-2xl border border-white/[0.08] bg-[#161a24]/90 p-1.5 shadow-[0_10px_40px_rgba(0,0,0,.35)]">
        {['Dashboard', 'Games', 'Achievements', 'Activity', 'Runners'].map((t, i) => (
          <button key={t} className={`rounded-xl px-4 py-2 text-[13px] font-semibold transition ${i === 0 ? 'bg-white/12 text-white' : 'text-white/50 hover:text-white'}`}>{t}</button>
        ))}
        <img src="https://picsum.photos/seed/hyd-avatar/80/80" alt="" className="ml-2 h-8 w-8 rounded-xl ring-1 ring-white/20" />
      </nav>

      <main className="mx-auto max-w-[1240px] px-6 pb-20">
        {/* greeting + headline stat block */}
        <section className="grid grid-cols-[1.2fr_1fr] gap-6">
          <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-8">
            <div className="mb-6 flex items-center gap-2.5 text-[12px] font-semibold text-white/50">
              <span className="h-2 w-2 rounded-full bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,.7)]" />
              all 24 titles verified · scan 09:41
            </div>
            <h1 className="text-[40px] font-extrabold leading-[1.04] tracking-[-0.02em]">
              Your library.<br />
              <span className="text-white/40">Running beautifully.</span>
            </h1>
            <div className="mt-7 flex gap-9">
              {[['Ready', '19'], ['Tweaks open', '5'], ['Hours', '3.1k']].map(([k, v]) => (
                <div key={k}>
                  <div className="text-[30px] font-extrabold tracking-tight">{v}</div>
                  <div className="mt-0.5 text-[12px] font-semibold text-white/40">{k}</div>
                </div>
              ))}
            </div>
          </div>

          {/* spotlight card */}
          <a href="/11/game" className="group relative overflow-hidden rounded-3xl border border-white/[0.08]">
            <img src={GAMES[1].wide} alt="" className="absolute inset-0 h-full w-full object-cover transition duration-700 group-hover:scale-105" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(11,13,19,.96) 10%, rgba(11,13,19,.15) 60%)' }} />
            <div className="relative flex h-full min-h-[300px] flex-col justify-end p-7">
              <span className="mb-2 w-fit rounded-full border border-white/25 bg-white/10 px-3 py-1 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">tonight's pick</span>
              <h2 className="text-[24px] font-extrabold tracking-tight">{GAMES[1].title}</h2>
              <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60 line-clamp-2">{GAMES[1].verdict}</p>
            </div>
          </a>
        </section>

        {/* rails */}
        <section className="pt-10">
          <div className="mb-4 flex items-baseline justify-between">
            <h2 className="text-[18px] font-bold tracking-tight">Continue</h2>
            <button className="text-[12.5px] font-semibold text-white/40 hover:text-white/80">library →</button>
          </div>
          <div className="flex gap-3.5 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
            {GAMES.slice(0, 8).map((g) => (
              <a key={g.id} href="/11/game" className="group w-[150px] flex-none">
                <div className="overflow-hidden rounded-2xl border border-white/[0.08] transition duration-300 group-hover:-translate-y-1 group-hover:border-white/25">
                  <img src={g.cover} alt="" className="aspect-[2/3] w-full object-cover" />
                </div>
                <div className="mt-2 truncate text-[12px] font-semibold">{g.title.split(':')[0]}</div>
                <div className="font-mono text-[9px] uppercase tracking-wider text-white/30">{g.tier} · {g.hours}h</div>
              </a>
            ))}
          </div>
        </section>

        {/* panels row */}
        <section className="grid grid-cols-[1.5fr_1fr] gap-6 pt-9">
          <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
            <div className="mb-3 flex items-baseline justify-between">
              <h2 className="text-[16px] font-bold tracking-tight">Week in play</h2>
              <span className="font-mono text-[10px] text-white/35">12h 41m</span>
            </div>
            <div className="flex h-28 items-end gap-2 rounded-2xl bg-black/20 p-4">
              {HOUR_BARS.map((h, i) => (
                <div key={i} className={`flex-1 rounded-md ${i === 13 ? 'bg-sky-400' : 'bg-white/10'}`} style={{ height: `${(h / 10) * 100}%`, minHeight: 4 }} />
              ))}
            </div>
            <div className="mt-5 border-t border-white/[0.07] pt-4">
              <h3 className="mb-2 text-[13px] font-bold">Latest activity</h3>
              <ActivityFeed limit={4} />
            </div>
          </div>

          <div className="space-y-4">
            <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
              <h2 className="mb-3 text-[16px] font-bold tracking-tight">Achievement score</h2>
              <div className="flex items-end gap-2">
                <span className="text-[38px] font-extrabold leading-none tracking-tight text-amber-300">1,240</span>
                <span className="mb-1 font-mono text-[10px] text-white/35">pts · +45 this week</span>
              </div>
              <div className="mt-4 grid grid-cols-5 gap-2">
                {achievementsFor(GAMES[2]).list.slice(0, 5).map((a, i) => (
                  <div key={a.id} className={`grid aspect-square place-items-center rounded-xl border text-[16px] ${a.unlocked ? 'border-amber-300/40 bg-amber-300/10' : 'border-white/[0.08] opacity-30'}`}>★</div>
                ))}
              </div>
            </div>
            <div className="rounded-3xl border border-sky-400/20 bg-sky-400/[0.07] p-6">
              <h3 className="text-[13px] font-bold text-sky-300">Maintenance hint</h3>
              <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60">Helldivers 2 runner predates the 10.1 patch. A one-click fix is available on the game page.</p>
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}

export function GlassGame() {
  const game = GAMES[1];
  const { list, unlockedCount, total, score } = achievementsFor(game);
  const [tab, setTab] = useState('overview');
  return (
    <div className="min-h-screen text-[#eceef2]" style={{ ...FIELD, fontFamily: '"Hanken Grotesk", sans-serif' }}>
      <main className="mx-auto max-w-[1240px] px-6 pb-20 pt-6">
        <a href="/11" className="mb-5 inline-block rounded-full border border-white/[0.1] bg-[#161a24]/90 px-4 py-2 font-mono text-[10px] uppercase tracking-[0.14em] text-white/60 transition hover:text-white">← dashboard</a>

        {/* hero band */}
        <section className="relative overflow-hidden rounded-3xl border border-white/[0.08]">
          <img src={game.wide} alt="" className="h-[300px] w-full object-cover" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(11,13,19,.97) 6%, rgba(11,13,19,.2) 55%)' }} />
          <div className="absolute bottom-6 left-7 right-7 flex items-end justify-between">
            <div>
              <div className="mb-2 flex gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
                <span className="rounded-full border border-emerald-400/40 bg-emerald-400/15 px-2.5 py-1 text-emerald-300">gold on linux</span>
                <span className="rounded-full border border-white/20 bg-white/10 px-2.5 py-1 text-white/70">{game.store}</span>
                <span className="rounded-full border border-white/20 bg-white/10 px-2.5 py-1 text-white/70">{game.size}</span>
              </div>
              <h1 className="text-[38px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <button className="rounded-2xl bg-white px-7 py-3 text-[14px] font-extrabold text-[#0b0d13] shadow-[0_12px_30px_rgba(255,255,255,.15)] transition hover:bg-white/90">▶ Play</button>
          </div>
        </section>

        {/* glass tabs */}
        <div className="my-6 flex w-fit gap-1 rounded-2xl border border-white/[0.08] bg-[#161a24]/90 p-1.5">
          {['overview', 'launcher settings', 'achievements', 'activity'].map((t) => (
            <button key={t} onClick={() => setTab(t)} className={`rounded-xl px-4 py-2 text-[12.5px] font-bold capitalize transition ${tab === t ? 'bg-white/12 text-white' : 'text-white/45 hover:text-white/80'}`}>{t}</button>
          ))}
        </div>

        {tab === 'overview' && (
          <section className="grid grid-cols-[1.5fr_1fr] gap-6">
            <div className="space-y-5">
              <img src={SCREENS(game)[0]} alt="" className="aspect-video w-full rounded-3xl object-cover border border-white/[0.08]" />
              <p className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6 text-[14px] leading-relaxed text-white/65">{game.verdict}</p>
            </div>
            <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
              <h3 className="mb-3 text-[15px] font-bold">Details</h3>
              {[['Runner', game.runner], ['DX layer', game.dx], ['Playtime', `${game.hours}h`], ['Last played', game.last], ['Prefix', '~/.platypus/pfx/nightreign']].map(([k, v]) => (
                <div key={k} className="flex justify-between border-b border-white/[0.06] py-2.5 font-mono text-[11.5px] last:border-0">
                  <span className="text-white/35">{k}</span><span className="text-white/85">{v}</span>
                </div>
              ))}
            </div>
          </section>
        )}

        {tab === 'launcher settings' && (
          <section className="grid grid-cols-[1.5fr_1fr] gap-6">
            <div className="space-y-5">
              <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
                <h3 className="mb-4 text-[15px] font-bold">Runner selection</h3>
                <div className="space-y-2.5">
                  {RUNNERS.slice(0, 3).map((r, i) => (
                    <button key={r.name} className={`flex w-full items-center justify-between rounded-2xl border px-5 py-3.5 text-left transition ${i === 0 ? 'border-sky-400/50 bg-sky-400/[0.08]' : 'border-white/[0.08] hover:border-white/20'}`}>
                      <div>
                        <div className="text-[13.5px] font-bold">{r.name}</div>
                        <div className="font-mono text-[10px] text-white/35">{r.note}</div>
                      </div>
                      <span className={`h-4 w-4 rounded-full border-2 ${i === 0 ? 'border-sky-400 bg-sky-400' : 'border-white/25'}`} />
                    </button>
                  ))}
                </div>
              </div>
              <div className="rounded-3xl border border-white/[0.08] bg-black/25 p-6 font-mono text-[11.5px] leading-relaxed">
                <h3 className="mb-3 font-sans text-[15px] font-bold">Pipeline preview</h3>
                {PIPELINE.map((p, i) => (
                  <div key={i} className="text-sky-300/90">{p.chain}</div>
                ))}
              </div>
            </div>
            <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
              <h3 className="mb-4 text-[15px] font-bold">Session options</h3>
              <div className="space-y-3">
                {[['Feral GameMode', true], ['MangoHud', true], ['Gamescope', false], ['EAC bridge', true], ['Fuzzy mod sync', false]].map(([l, on]) => (
                  <div key={l} className="flex items-center justify-between rounded-2xl bg-black/20 px-4 py-3">
                    <span className="text-[12.5px] font-semibold">{l}</span>
                    <span className={`relative h-5 w-9 rounded-full transition ${on ? 'bg-sky-400' : 'bg-white/15'}`}>
                      <span className="absolute top-0.5 h-4 w-4 rounded-full bg-[#0b0d13]" style={{ left: on ? 18 : 2 }} />
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </section>
        )}

        {tab === 'achievements' && (
          <section className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-7">
            <div className="mb-6 flex items-center gap-6">
              <div className="grid h-24 w-24 place-items-center rounded-full border-[3px] border-amber-300/50 text-[22px] font-black text-amber-300">{score}%</div>
              <div>
                <div className="text-[18px] font-extrabold">{unlockedCount} unlocked</div>
                <div className="font-mono text-[11px] text-white/35">of ~{total} · rarity-weighted</div>
              </div>
            </div>
            <div className="grid grid-cols-4 gap-3">
              {list.map((a) => (
                <div key={a.id} className={`rounded-2xl border p-4 ${a.unlocked ? 'border-amber-300/25 bg-amber-300/[0.06]' : 'border-white/[0.06] opacity-30'}`}>
                  <div className={`grid h-11 w-11 place-items-center rounded-xl text-[17px] ${a.unlocked ? 'bg-amber-300/15' : 'bg-[#161a24]/90'}`}>★</div>
                  <div className="mt-2.5 truncate text-[12px] font-bold">{a.name}</div>
                  <div className="truncate font-mono text-[9px] text-white/30">{a.unlocked ? a.date : a.rarity}</div>
                </div>
              ))}
            </div>
          </section>
        )}

        {tab === 'activity' && (
          <div className="rounded-3xl border border-white/[0.08] bg-[#161a24]/90 p-6">
            <ActivityFeed limit={9} />
          </div>
        )}
      </main>
    </div>
  );
}