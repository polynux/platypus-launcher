import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS,
  protonSettingsFor, PROTON_FLAGS, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS,
} from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /12 — ITERATION 1: the literal merge of /7 (Galaxy warm
   wash, count sidebar, summary plates) + /10 (tactile rounded
   cards, lime accent, chunky feel). Everything /7 had, but
   cards gain weight and touch; activity promoted to cards.
   ============================================================ */

function Topbar({ title, sub }) {
  return (
    <header className="mb-7 flex items-center justify-between">
      <div>
        <h1 className="text-[24px] font-extrabold tracking-tight">{title}</h1>
        {sub && <p className="mt-0.5 text-[13.5px] text-white/45">{sub}</p>}
      </div>
      <button className="rounded-xl bg-amber-300 px-5 py-2.5 text-[13.5px] font-extrabold text-[#17120e] shadow-[0_8px_24px_rgba(252,211,77,.2)] transition hover:bg-amber-200">＋ Add game</button>
    </header>
  );
}

export function It12Dashboard() {
  return (
    <div
      className="min-h-screen text-[#efe9df]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1100px 640px at 12% -8%, rgba(150,96,58,.5), transparent 58%), radial-gradient(900px 560px at 92% 12%, rgba(86,62,128,.48), transparent 55%), radial-gradient(800px 480px at 55% 108%, rgba(30,78,96,.42), transparent 58%), #161009',
      }}
    >
      <div className="mx-auto flex max-w-[1560px]">
        <aside className="sticky top-0 flex h-screen w-60 flex-none flex-col border-r border-white/[0.07] bg-black/30 p-5 backdrop-blur-sm">
          <div className="mb-6 flex items-center gap-3 px-1">
            <span className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[16px] font-black text-[#161009]">H</span>
            <div>
              <div className="text-[16px] font-extrabold tracking-tight">Hydra</div>
              <div className="font-mono text-[9px] uppercase tracking-[0.16em] text-white/35">game hub</div>
            </div>
          </div>
          <div className="mb-5 rounded-xl bg-white/[0.06] px-4 py-2.5 text-[12.5px] text-white/35">🔍 Search library…</div>
          <div className="px-2 pb-2 font-mono text-[9.5px] uppercase tracking-[0.18em] text-white/30">library</div>
          {[['Recent', 24, true], ['Installed', 21], ['Ready to play', 19], ['Needs tweaks', 5]].map(([l, c, a]) => (
            <button key={l} className={`mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2.5 font-semibold ${a ? 'bg-amber-300/12 text-amber-200' : 'text-white/55 hover:bg-white/[0.05] hover:text-white'}`}>
              <span>{l}</span>
              <span className={`rounded-full px-2 py-0.5 font-mono text-[10px] ${a ? 'bg-amber-300/20' : 'bg-white/[0.07]'}`}>{c}</span>
            </button>
          ))}
          <div className="px-2 pb-2 pt-5 font-mono text-[9.5px] uppercase tracking-[0.18em] text-white/30">sources</div>
          {[['Steam', 14], ['GOG', 5], ['Epic', 5]].map(([s, c]) => (
            <button key={s} className="mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2 text-[13px] text-white/50 hover:text-white">
              <span className="flex items-center gap-2.5"><span className="h-1.5 w-1.5 rounded-full bg-amber-300/60" />{s}</span>
              <span className="font-mono text-[10px] text-white/30">{c}</span>
            </button>
          ))}
          <div className="mt-auto rounded-2xl bg-white/[0.05] p-4">
            <div className="flex items-center justify-between text-[11.5px] font-bold">
              <span className="text-emerald-300">● online</span><span className="font-mono text-white/30">2 friends</span>
            </div>
            <div className="mt-2.5 flex -space-x-2">
              {[1, 2, 3, 4].map((i) => <img key={i} src={`https://picsum.photos/seed/hyd-f${i}/48/48`} alt="" className="h-7 w-7 rounded-full ring-2 ring-[#1e1710]" />)}
              <span className="grid h-7 w-7 place-items-center rounded-full bg-white/10 text-[10px] font-bold ring-2 ring-[#1e1710]">+9</span>
            </div>
          </div>
        </aside>

        <main className="flex-1 p-8">
          <Topbar title="Good evening, Polynux" sub="5 games got updates today — 3 of them need a fix re-check." />

          {/* feature + quick stats */}
          <section className="mb-7 grid grid-cols-[1.9fr_1fr] gap-5">
            <div className="relative overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
              <img src={GAMES[11].wide} alt="" className="aspect-[2.55/1] w-full object-cover" />
              <div className="absolute inset-0" style={{ background: 'linear-gradient(80deg, rgba(22,16,9,.95) 25%, rgba(22,16,9,.25) 62%, transparent)' }} />
              <div className="absolute bottom-6 left-7 max-w-md">
                <span className="rounded-lg bg-orange-400 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#161009]">fix needed</span>
                <h2 className="mt-2.5 text-[26px] font-extrabold tracking-tight">{GAMES[11].title}</h2>
                <p className="mt-1.5 text-[12.5px] leading-relaxed text-white/60 line-clamp-2">{GAMES[11].verdict}</p>
                <div className="mt-4 flex gap-2.5">
                  <a href="/12/game" className="rounded-xl bg-white px-4 py-2 text-[12.5px] font-extrabold text-[#161009]">Open page</a>
                  <button className="rounded-xl bg-white/10 px-4 py-2 text-[12.5px] font-bold text-white/85 backdrop-blur">Apply fix</button>
                </div>
              </div>
            </div>

            <div className="grid grid-rows-2 gap-5">
              <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
                <div className="flex items-baseline justify-between">
                  <h3 className="text-[13px] font-bold text-white/85">This week</h3>
                  <span className="font-mono text-[10px] text-white/35">12h 41m</span>
                </div>
                <div className="mt-3.5 flex h-16 items-end gap-1.5">
                  {HOUR_BARS.slice(0, 12).map((h, i) => (
                    <div key={i} className={`flex-1 rounded-md ${i === 11 ? 'bg-amber-300' : 'bg-white/[0.12]'}`} style={{ height: `${(h / 10) * 100}%`, minHeight: 3 }} />
                  ))}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-5">
                {[['Achievement pts', '1,240', 'text-amber-300'], ['Sessions', '14', 'text-emerald-300']].map(([k, v, c]) => (
                  <div key={k} className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
                    <div className={`text-[24px] font-extrabold tabular-nums ${c}`}>{v}</div>
                    <div className="mt-0.5 text-[10.5px] font-semibold uppercase tracking-wider text-white/35">{k}</div>
                  </div>
                ))}
              </div>
            </div>
          </section>

          {/* continue rail — tactile cards */}
          <section className="mb-7">
            <div className="mb-3.5 flex items-baseline justify-between">
              <h2 className="text-[17px] font-bold tracking-tight">Continue playing</h2>
              <button className="text-[12px] font-semibold text-white/35 hover:text-white/80">all →</button>
            </div>
            <div className="grid grid-cols-6 gap-4">
              {GAMES.slice(0, 6).map((g) => (
                <a key={g.id} href="/12/game" className="group overflow-hidden rounded-2xl bg-white/[0.05] ring-1 ring-white/[0.07] transition duration-300 hover:-translate-y-1 hover:ring-amber-300/40">
                  <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover" />
                  <div className="p-3">
                    <div className="truncate text-[12.5px] font-bold">{g.title.split(':')[0]}</div>
                    <div className="mt-0.5 flex items-center justify-between font-mono text-[9px] uppercase tracking-wider text-white/35">
                      <span>{g.hours}h</span>
                      <span className={g.tier === 'platinum' ? 'text-amber-300' : ''}>{g.tier}</span>
                    </div>
                  </div>
                </a>
              ))}
            </div>
          </section>

          {/* activity + fixes */}
          <section className="grid grid-cols-[1.55fr_1fr] gap-5 pb-10">
            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-2 flex items-center justify-between">
                <h2 className="text-[15px] font-bold">Activity</h2>
                <button className="font-mono text-[10px] uppercase tracking-widest text-white/30 hover:text-white/70">full log →</button>
              </div>
              <ActivityFeed limit={6} />
            </div>
            <div className="space-y-3">
              <h2 className="text-[15px] font-bold">Fixes waiting</h2>
              {FIXES.slice(0, 4).map((f, i) => (
                <div key={f.game} className={`rounded-2xl p-4 ring-1 ${i === 0 ? 'bg-orange-400/[0.08] ring-orange-400/30' : 'bg-white/[0.04] ring-white/[0.06]'}`}>
                  <div className="flex items-center justify-between">
                    <span className="text-[13px] font-bold">{f.game}</span>
                    <span className="font-mono text-[9.5px] uppercase tracking-wider text-white/40">{f.tier}</span>
                  </div>
                  <p className="mt-1 text-[11.5px] leading-relaxed text-white/50 line-clamp-2">{f.line}</p>
                </div>
              ))}
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export function It12Game() {
  const game = GAMES[8];
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sync, setSync] = useState(ps.sync);
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [vkd3d, setVkd3d] = useState(ps.vkd3d);
  const [ups, setUps] = useState('FSR 3');
  const [hud, setHud] = useState('full');
  const [tricks, setTricks] = useState(ps.winetricks);

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button
          key={o}
          onClick={() => onChange(o)}
          className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${value === o ? 'bg-amber-300 text-[#161009] font-bold' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]'}`}
        >{o}</button>
      ))}
    </div>
  );

  return (
    <div
      className="min-h-screen text-[#efe9df]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1000px 600px at 85% -10%, rgba(86,62,128,.5), transparent 58%), radial-gradient(900px 520px at 8% 105%, rgba(150,96,58,.45), transparent 55%), #161009',
      }}
    >
      <div className="mx-auto max-w-[1280px] px-8 py-6">
        <a href="/12" className="mb-5 inline-flex items-center gap-2 rounded-xl bg-white/[0.06] px-4 py-2 text-[12px] font-bold text-white/60 transition hover:text-white">← library</a>

        {/* head */}
        <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <img src={game.wide} alt="" className="h-[290px] w-full object-cover" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(22,16,9,.97) 6%, rgba(22,16,9,.25) 58%)' }} />
          <div className="absolute bottom-5 left-7 right-7 flex items-end justify-between">
            <div>
              <div className="mb-2 flex gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
                <span className="rounded-lg bg-amber-300 px-2.5 py-1 text-[#161009]">{game.tier} verified</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75 backdrop-blur">{game.store}</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75 backdrop-blur">{game.size}</span>
              </div>
              <h1 className="text-[36px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <button className="rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#161009] shadow-[0_12px_30px_rgba(252,211,77,.25)] transition hover:bg-amber-200">▶ Play</button>
          </div>
        </section>

        {/* launcher settings — full panel set */}
        <div className="grid grid-cols-[1.55fr_1fr] gap-5">
          <div className="space-y-5">
            {/* proton flags */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-4 flex items-baseline justify-between">
                <h2 className="text-[15px] font-bold">Proton environment flags</h2>
                <span className="font-mono text-[10px] text-white/30">{flags.filter(f => f.on).length} active</span>
              </div>
              <div className="grid grid-cols-2 gap-2.5">
                {flags.map((f) => (
                  <button
                    key={f.key}
                    onClick={() => setFlags(flags.map((x) => x.key === f.key ? { ...x, on: !x.on } : x))}
                    className={`flex items-start justify-between gap-3 rounded-2xl border p-3.5 text-left transition ${f.on ? 'border-amber-300/40 bg-amber-300/[0.08]' : 'border-white/[0.07] hover:border-white/20'}`}
                  >
                    <div>
                      <div className="font-mono text-[11.5px] font-bold text-white/90">{f.key}</div>
                      <div className="mt-0.5 text-[11px] leading-snug text-white/45">{f.desc}</div>
                    </div>
                    <span className={`mt-0.5 h-4 w-7 flex-none rounded-full transition ${f.on ? 'bg-amber-300' : 'bg-white/15'}`}>
                      <span className="block h-3 w-3 translate-x-0.5 translate-y-0.5 rounded-full bg-[#161009] transition" style={{ transform: f.on ? 'translateX(12px)' : 'translateX(2px)' }} />
                    </span>
                  </button>
                ))}
              </div>
            </section>

            {/* renderer stack */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-4 text-[15px] font-bold">Renderer & sync</h2>
              <div className="space-y-4">
                <div>
                  <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">DXVK version</div>
                  <Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} />
                </div>
                <div>
                  <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">VKD3D-Proton</div>
                  <Seg options={VKD3D_VERSIONS} value={vkd3d} onChange={setVkd3d} />
                </div>
                <div>
                  <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">Synchronization</div>
                  <Seg options={SYNC_MODES} value={sync} onChange={setSync} />
                </div>
                <div>
                  <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">Upscaler</div>
                  <Seg options={UPSCALERS} value={ups} onChange={setUps} />
                </div>
              </div>
            </section>

            {/* winetricks */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[15px] font-bold">Winetricks verbs</h2>
                <span className="font-mono text-[10px] text-white/30">applied to prefix on next launch</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {WINETRICKS.map((t) => (
                  <button
                    key={t}
                    onClick={() => setTricks(tricks.includes(t) ? tricks.filter((x) => x !== t) : [...tricks, t])}
                    className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${tricks.includes(t) ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-white/[0.06] text-white/45 hover:bg-white/[0.1]'}`}
                  >{tricks.includes(t) ? '✓ ' : '+ '}{t}</button>
                ))}
              </div>
            </section>
          </div>

          {/* right rail */}
          <aside className="space-y-5">
            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-3.5 text-[15px] font-bold">Session</h2>
              <div className="space-y-2.5">
                <div>
                  <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">MangoHud preset</div>
                  <Seg options={HUD_PRESETS} value={hud} onChange={setHud} />
                </div>
                {[['Shader cache', ps.shaderCache], ['Prefix', ps.prefix], ['Runner', game.runner]].map(([k, v]) => (
                  <div key={k} className="flex items-center justify-between border-t border-white/[0.06] pt-2.5 font-mono text-[11px]">
                    <span className="text-white/35">{k}</span><span className="truncate pl-3 text-white/80">{v}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="rounded-3xl bg-black/30 p-4 font-mono text-[11px] leading-relaxed ring-1 ring-white/[0.07]">
              <div className="mb-1 text-white/30">$ effective command</div>
              <div className="text-emerald-200/90">gamemoderun mangohud --preset 4 \</div>
              <div className="pl-4 text-emerald-200/90">{game.runner} run {game.executable || 'game.exe'} \</div>
              {flags.filter(f => f.on).map((f) => <div key={f.key} className="pl-4 text-amber-200/80">{f.key}=1 \</div>)}
              <div className="pl-4 text-white/50">{game.settings?.launch_arguments || ''}</div>
            </div>

            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-3 text-[15px] font-bold">Achievements</h2>
              <div className="flex items-baseline gap-2">
                <span className="text-[26px] font-extrabold text-amber-300">{ach.unlockedCount}</span>
                <span className="font-mono text-[11px] text-white/35">/ {ach.total} · {ach.score}%</span>
              </div>
              <div className="mt-3 grid grid-cols-5 gap-2">
                {ach.list.slice(0, 5).map((a) => (
                  <div key={a.id} className={`grid aspect-square place-items-center rounded-xl text-[15px] ${a.unlocked ? 'bg-amber-300/12 ring-1 ring-amber-300/40' : 'bg-white/[0.05] opacity-25'}`}>★</div>
                ))}
              </div>
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}