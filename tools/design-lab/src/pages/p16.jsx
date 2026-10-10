import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, achievementsFor, SCREENS, HOUR_BARS,
  protonSettingsFor, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS,
} from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /16 — ITERATION 5: the synthesizer. Everything /13's
   density discipline and /14's hero taught, folded back into
   /12's warm tactile system. This is the "make it shippable"
   pass: every panel earns its place, every control is real.
   ============================================================ */

export function It16Dashboard() {
  const hero = GAMES[11];
  const tweak = GAMES.filter(g => g.status === 'tweak').slice(0, 3);
  return (
    <div
      className="min-h-screen text-[#ece6da]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1100px 620px at 14% -8%, rgba(150,96,58,.46), transparent 58%), radial-gradient(880px 520px at 90% 10%, rgba(86,62,128,.44), transparent 54%), radial-gradient(760px 460px at 52% 108%, rgba(30,78,96,.36), transparent 56%), #130f0a',
      }}
    >
      <div className="mx-auto flex max-w-[1600px]">
        <aside className="sticky top-0 flex h-screen w-56 flex-none flex-col border-r border-white/[0.06] bg-black/25 p-4">
          <div className="mb-5 flex items-center gap-2.5 px-1.5">
            <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[15px] font-black text-[#130f0a]">P</span>
            <div><div className="text-[15px] font-extrabold tracking-tight">Platypus</div><div className="font-mono text-[8.5px] uppercase tracking-[0.16em] text-white/30">game hub</div></div>
          </div>
          <div className="mb-4 rounded-xl bg-white/[0.06] px-3.5 py-2.5 text-[12px] text-white/35">🔍 Search…</div>
          {[['Dashboard', 0, true], ['Library', 24], ['Downloads', 1], ['Achievements', 0], ['Activity', 0]].map(([l, c, a]) => (
            <button key={l} className={`mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2 text-[12.5px] font-semibold ${a ? 'bg-amber-300/12 text-amber-200' : 'text-white/50 hover:bg-white/[0.05] hover:text-white'}`}>
              <span>{l}</span>{!!c && <span className="rounded-full bg-white/[0.07] px-2 font-mono text-[9.5px]">{c}</span>}
            </button>
          ))}
          <div className="px-2 pb-1.5 pt-4 font-mono text-[8.5px] uppercase tracking-[0.18em] text-white/25">sources</div>
          {['Steam', 'GOG', 'Epic'].map((s, i) => (
            <button key={s} className="mb-0.5 flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-[12.5px] text-white/50 hover:text-white">
              <span className="flex items-center gap-2"><span className="h-1.5 w-1.5 rounded-full bg-amber-300/50" />{s}</span>
              <span className="font-mono text-[9.5px] text-white/25">{[14, 5, 5][i]}</span>
            </button>
          ))}
          <div className="mt-auto rounded-2xl bg-white/[0.05] p-3.5">
            <div className="flex items-center justify-between text-[11px] font-bold"><span className="text-emerald-300">● 2 online</span><span className="font-mono text-white/30">scan 09:41</span></div>
          </div>
        </aside>

        <main className="flex-1 p-7">
          {/* hero band — compact cinematic, not full-page */}
          <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
            <img src={hero.wide} alt="" className="h-[300px] w-full object-cover" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)' }} />
            <div className="absolute bottom-6 left-7 max-w-md">
              <span className="rounded-lg bg-orange-400 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#130f0a]">action required</span>
              <h1 className="mt-2.5 text-[28px] font-extrabold leading-tight tracking-tight">{hero.title} needs its fix re-applied</h1>
              <div className="mt-4 flex gap-2.5">
                <a href="/16/game" className="rounded-xl bg-amber-300 px-5 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">Open settings</a>
                <button className="rounded-xl bg-white/10 px-4 py-2.5 text-[12.5px] font-bold text-white/85 backdrop-blur">Later</button>
              </div>
            </div>
            {/* mini stats riding the hero right */}
            <div className="absolute bottom-6 right-7 hidden gap-3 lg:flex">
              {[['hours', '12.7', 'text-amber-300'], ['pts', '1,240', 'text-amber-200'], ['ready', '19/24', 'text-emerald-300']].map(([k, v, c]) => (
                <div key={k} className="w-28 rounded-2xl bg-black/45 p-3.5 backdrop-blur">
                  <div className={`text-[19px] font-extrabold tabular-nums ${c}`}>{v}</div>
                  <div className="font-mono text-[8.5px] uppercase tracking-widest text-white/40">{k}</div>
                </div>
              ))}
            </div>
          </section>

          {/* continue + waiting-on, asymmetric */}
          <section className="mb-6 grid grid-cols-[1.75fr_1fr] gap-5">
            <div>
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Continue playing</h2>
                <button className="text-[11.5px] font-semibold text-white/35 hover:text-white/80">library →</button>
              </div>
              <div className="grid grid-cols-4 gap-3.5">
                {GAMES.slice(0, 8).map((g) => (
                  <a key={g.id} href="/16/game" className="group overflow-hidden rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07] transition duration-300 hover:-translate-y-0.5 hover:ring-amber-300/40">
                    <div className="relative">
                      <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover" />
                      {g.status === 'tweak' && <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,.8)]" />}
                    </div>
                    <div className="px-3 py-2.5">
                      <div className="truncate text-[12px] font-bold">{g.title.split(':')[0]}</div>
                      <div className="font-mono text-[8.5px] uppercase tracking-wider text-white/30">{g.hours}h · {g.runner.split(' ')[0]}</div>
                    </div>
                  </a>
                ))}
              </div>
            </div>
            <div>
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Waiting on you</h2>
              </div>
              <div className="space-y-3">
                {tweak.map((g) => (
                  <a key={g.id} href="/16/game" className="block rounded-2xl bg-white/[0.045] p-4 ring-1 ring-white/[0.06] transition hover:ring-orange-400/40">
                    <div className="flex items-center justify-between">
                      <span className="truncate text-[13px] font-bold">{g.title}</span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-orange-300">{g.tier}</span>
                    </div>
                    <p className="mt-1 text-[11px] leading-relaxed text-white/45 line-clamp-2">{g.verdict}</p>
                  </a>
                ))}
              </div>
            </div>
          </section>

          {/* activity full-width with day grouping */}
          <section className="pb-10">
            <div className="mb-3 flex items-baseline justify-between">
              <h2 className="text-[16px] font-bold tracking-tight">Activity</h2>
              <button className="font-mono text-[10px] uppercase tracking-widest text-white/30 hover:text-white/70">full log →</button>
            </div>
            <div className="rounded-3xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
              <ActivityFeed limit={8} />
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export function It16Game() {
  const game = GAMES[8];
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sync, setSync] = useState('fsync');
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [vkd3d, setVkd3d] = useState('VKD3D-Proton 2.14');
  const [ups, setUps] = useState('FSR 3');
  const [hud, setHud] = useState('full');
  const [tricks, setTricks] = useState(ps.winetricks);

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button key={o} onClick={() => onChange(o)} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${value === o ? 'bg-amber-300 font-bold text-[#130f0a]' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]'}`}>{o}</button>
      ))}
    </div>
  );

  return (
    <div
      className="min-h-screen text-[#ece6da]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1000px 560px at 88% -10%, rgba(86,62,128,.46), transparent 56%), #130f0a',
      }}
    >
      <div className="mx-auto max-w-[1300px] px-8 py-6">
        <a href="/16" className="text-[12px] font-semibold text-white/40 hover:text-white">← library</a>

        {/* head */}
        <section className="relative mt-4 mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <img src={game.wide} alt="" className="h-[260px] w-full object-cover" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.97) 8%, rgba(19,15,10,.3) 60%)' }} />
          <div className="absolute bottom-5 left-7 right-7 flex items-end justify-between">
            <div>
              <div className="mb-2 flex gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
                <span className="rounded-lg bg-amber-300 px-2.5 py-1 text-[#130f0a]">{game.tier} verified</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{game.store}</span>
              </div>
              <h1 className="text-[34px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <div className="flex items-center gap-3">
              <div className="text-right font-mono text-[10px] text-white/40"><div>{game.hours}h</div><div>last {game.last}</div></div>
              <button className="rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#130f0a] shadow-[0_12px_30px_rgba(252,211,77,.25)]">▶ Play</button>
            </div>
          </div>
        </section>

        <div className="grid grid-cols-[1.6fr_1fr] gap-5">
          <div className="space-y-5">
            {/* proton flags — grid of real toggles */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-4 flex items-baseline justify-between">
                <h2 className="text-[15px] font-bold">Proton environment flags</h2>
                <span className="font-mono text-[10px] text-white/30">{flags.filter(f => f.on).length} active of {flags.length}</span>
              </div>
              <div className="grid grid-cols-2 gap-2.5">
                {flags.map((f) => (
                  <button key={f.key} onClick={() => setFlags(flags.map(x => x.key === f.key ? { ...x, on: !x.on } : x))} className={`flex items-start justify-between gap-3 rounded-2xl border p-3.5 text-left transition ${f.on ? 'border-amber-300/40 bg-amber-300/[0.08]' : 'border-white/[0.07] hover:border-white/20'}`}>
                    <div><div className="font-mono text-[11.5px] font-bold text-white/90">{f.key}</div><div className="mt-0.5 text-[11px] leading-snug text-white/45">{f.desc}</div></div>
                    <span className={`mt-0.5 h-4 w-7 flex-none rounded-full transition ${f.on ? 'bg-amber-300' : 'bg-white/15'}`}>
                      <span className="block h-3 w-3 translate-x-0.5 translate-y-0.5 rounded-full bg-[#130f0a]" style={{ transform: f.on ? 'translateX(12px)' : 'translateX(2px)' }} />
                    </span>
                  </button>
                ))}
              </div>
            </section>

            {/* renderer */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-4 text-[15px] font-bold">Renderer & compatibility</h2>
              <div className="grid grid-cols-2 gap-x-6 gap-y-4">
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">dxvk</div><Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">vkd3d-proton</div><Seg options={['VKD3D-Proton 2.14', 'VKD3D-Proton 2.13', 'off']} value={vkd3d} onChange={setVkd3d} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">sync primitives</div><Seg options={SYNC_MODES} value={sync} onChange={setSync} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">upscaler</div><Seg options={UPSCALERS} value={ups} onChange={setUps} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">mangohud</div><Seg options={HUD_PRESETS} value={hud} onChange={setHud} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">shader cache</div><Seg options={['on', 'off']} value="on" onChange={() => {}} /></div>
              </div>
            </section>

            {/* winetricks */}
            <section className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[15px] font-bold">Winetricks verbs</h2>
                <span className="font-mono text-[10px] text-white/30">staged for next launch</span>
              </div>
              <div className="flex flex-wrap gap-1.5">
                {WINETRICKS.map((t) => (
                  <button key={t} onClick={() => setTricks(tricks.includes(t) ? tricks.filter(x => x !== t) : [...tricks, t])} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${tricks.includes(t) ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-white/[0.06] text-white/45 hover:bg-white/[0.1]'}`}>{tricks.includes(t) ? '✓ ' : '+ '}{t}</button>
                ))}
              </div>
            </section>
          </div>

          {/* right rail */}
          <aside className="space-y-5">
            <div className="rounded-3xl bg-black/30 p-4 font-mono text-[11px] leading-relaxed ring-1 ring-white/[0.07]">
              <div className="mb-1 text-white/30">$ effective command</div>
              <div className="text-emerald-200/90">gamemoderun mangohud --preset 4 \</div>
              <div className="pl-4 text-emerald-200/90">{ps.prefix.split('/').pop() && game.runner} run \</div>
              {flags.filter(f => f.on).map(f => <div key={f.key} className="pl-4 text-amber-200/80">{f.key}=1 \</div>)}
            </div>

            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-3 text-[15px] font-bold">Prefix & storage</h2>
              {[['Prefix path', ps.prefix], ['Runner', game.runner], ['Size on disk', game.size], ['Shader cache', '412 MB'], ['Last verified', '2h ago']].map(([k, v]) => (
                <div key={k} className="flex justify-between gap-3 border-b border-white/[0.05] py-2 font-mono text-[10.5px] last:border-0">
                  <span className="flex-none text-white/35">{k}</span><span className="truncate text-right text-white/80">{v}</span>
                </div>
              ))}
            </div>

            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[15px] font-bold">Achievements</h2>
                <span className="font-mono text-[10px] text-amber-300">{ach.score}%</span>
              </div>
              <div className="h-1.5 rounded-full bg-white/[0.08]"><div className="h-full rounded-full bg-amber-300" style={{ width: `${ach.score}%` }} /></div>
              <div className="mt-4 grid grid-cols-5 gap-2">
                {ach.list.slice(0, 5).map((a) => (
                  <div key={a.id} className={`grid aspect-square place-items-center rounded-xl text-[15px] ${a.unlocked ? 'bg-amber-300/12 ring-1 ring-amber-300/40' : 'bg-white/[0.05] opacity-25'}`}>★</div>
                ))}
              </div>
              <div className="mt-3 font-mono text-[10px] text-white/35">{ach.unlockedCount} of {ach.total}</div>
            </div>

            <div className="rounded-3xl bg-white/[0.045] p-5 ring-1 ring-white/[0.06]">
              <h2 className="mb-2 text-[13px] font-bold">Recent activity</h2>
              <ActivityFeed limit={3} />
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}