import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, achievementsFor, SCREENS, HOUR_BARS,
  protonSettingsFor, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS,
} from '../data.js';

/* ============================================================
   /15 — ITERATION 4: light mode. Same warm Galaxy DNA on a
   bone-paper ground, dark sidebar inversion. Tests whether
   the identity survives daylight. (Accent color stays part
   of the theme system, so this is structure, not a reskin.)
   ============================================================ */

const INK = '#241b12';
const game = GAMES[8];

export function It15Dashboard() {
  return (
    <div className="min-h-screen" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: 'radial-gradient(1000px 560px at 10% -8%, rgba(196,138,84,.18), transparent 55%), radial-gradient(800px 480px at 92% 10%, rgba(140,110,190,.14), transparent 50%), #f7f3ea', color: INK }}>
      <div className="mx-auto flex max-w-[1560px]">
        {/* INVERTED dark sidebar */}
        <aside className="sticky top-0 flex h-screen w-60 flex-none flex-col bg-[#241b12] p-5 text-[#f0e9dc]">
          <div className="mb-6 flex items-center gap-3 px-1">
            <span className="grid h-10 w-10 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[16px] font-black text-[#241b12]">P</span>
            <div>
              <div className="text-[16px] font-extrabold tracking-tight">Platypus</div>
              <div className="font-mono text-[9px] uppercase tracking-[0.16em] text-white/35">game hub</div>
            </div>
          </div>
          <div className="mb-5 rounded-xl bg-white/[0.07] px-4 py-2.5 text-[12.5px] text-white/35">🔍 Search library…</div>
          {[['Recent', 24, true], ['Installed', 21], ['Ready', 19], ['Tweaks', 5]].map(([l, c, a]) => (
            <button key={l} className={`mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2.5 font-semibold ${a ? 'bg-amber-300/15 text-amber-300' : 'text-white/55 hover:bg-white/[0.06] hover:text-white'}`}>
              <span>{l}</span><span className="rounded-full bg-white/[0.08] px-2 py-0.5 font-mono text-[10px]">{c}</span>
            </button>
          ))}
          <div className="px-2 pb-2 pt-5 font-mono text-[9.5px] uppercase tracking-[0.18em] text-white/30">sources</div>
          {['Steam', 'GOG', 'Epic'].map((s, i) => (
            <button key={s} className="mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2 text-[13px] text-white/55 hover:text-white">
              <span className="flex items-center gap-2.5"><span className="h-1.5 w-1.5 rounded-full" style={{ background: ['#e2543e', '#9a6dd7', '#3d9d6d'][i] }} />{s}</span>
              <span className="font-mono text-[10px] text-white/30">{[14, 5, 5][i]}</span>
            </button>
          ))}
          <div className="mt-auto rounded-2xl bg-white/[0.06] p-4">
            <div className="flex items-center justify-between text-[11.5px] font-bold"><span className="text-emerald-300">● online</span><span className="font-mono text-white/35">2 friends</span></div>
            <div className="mt-2.5 flex -space-x-2">
              {[1, 2, 3].map((i) => <img key={i} src={`https://picsum.photos/seed/hyd-f${i}/48/48`} alt="" className="h-7 w-7 rounded-full ring-2 ring-[#241b12]" />)}
            </div>
          </div>
        </aside>

        <main className="flex-1 p-8">
          <header className="mb-6 flex items-center justify-between">
            <h1 className="text-[23px] font-extrabold tracking-tight">Good evening, Polynux</h1>
            <button className="rounded-xl bg-[#241b12] px-5 py-2.5 text-[13px] font-extrabold text-[#f7f3ea]">＋ Add game</button>
          </header>

          {/* summary plates on paper */}
          <section className="mb-7 grid grid-cols-3 gap-5">
            {[
              ['Hours this week', '12.7', 'text-[#c2542e]', '34%'],
              ['Achievements', '1,240 pts', 'text-[#8a5fc4]', '45%'],
              ['Collection health', '19 / 24', 'text-[#3d9d6d]', '79%'],
            ].map(([k, v, c, w]) => (
              <div key={k} className="rounded-2xl border border-[#241b12]/10 bg-white p-5 shadow-[0_2px_10px_rgba(36,27,18,.05)]">
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[#241b12]/50">{k}</div>
                <div className={`mt-1 text-[28px] font-extrabold tabular-nums ${c}`}>{v}</div>
                <div className="mt-3.5 h-1.5 rounded-full bg-[#241b12]/8"><div className="h-full rounded-full" style={{ width: w, background: c }} /></div>
              </div>
            ))}
          </section>

          {/* continue cards */}
          <section className="mb-8">
            <div className="mb-3.5 flex items-baseline justify-between">
              <h2 className="text-[17px] font-bold tracking-tight">Continue playing</h2>
              <button className="text-[12px] font-semibold text-[#241b12]/45 hover:text-[#241b12]">all 24 →</button>
            </div>
            <div className="grid grid-cols-6 gap-4">
              {GAMES.slice(0, 6).map((g) => (
                <a key={g.id} href="/15/game" className="group overflow-hidden rounded-2xl border border-[#241b12]/10 bg-white shadow-[0_2px_10px_rgba(36,27,18,.05)] transition duration-300 hover:-translate-y-1 hover:shadow-[0_10px_28px_rgba(36,27,18,.13)]">
                  <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover" />
                  <div className="p-3">
                    <div className="truncate text-[12.5px] font-bold">{g.title.split(':')[0]}</div>
                    <div className="mt-0.5 font-mono text-[9px] uppercase tracking-wider text-[#241b12]/45">{g.hours}h · {g.tier}</div>
                  </div>
                </a>
              ))}
            </div>
          </section>

          {/* table + activity */}
          <section className="grid grid-cols-[1.6fr_1fr] gap-6 pb-10">
            <div className="overflow-hidden rounded-2xl border border-[#241b12]/10 bg-white">
              <div className="border-b border-[#241b12]/10 px-4 py-2.5 text-[12px] font-bold">All games — recently played first</div>
              {GAMES.slice(0, 8).map((g, i) => (
                <a key={g.id} href="/15/game" className="group grid grid-cols-[1fr_5rem_6rem_5rem] items-center gap-3 px-4 py-2 hover:bg-[#f7f3ea]">
                  <span className="flex items-center gap-2.5"><img src={g.cover} alt="" className="h-7 w-7 rounded object-cover" /><span className="truncate text-[12.5px] font-semibold group-hover:text-[#c2542e]">{g.title}</span></span>
                  <span className="font-mono text-[10px] text-[#241b12]/40">{g.tier}</span>
                  <span className="truncate font-mono text-[10px] text-[#241b12]/35">{g.runner}</span>
                  <span className="text-right font-mono text-[10.5px] tabular-nums text-[#241b12]/50">{g.hours}h</span>
                </a>
              ))}
            </div>
            <div className="rounded-2xl border border-[#241b12]/10 bg-white p-4">
              <h2 className="mb-1.5 text-[13px] font-bold">Activity</h2>
              <ul className="divide-y divide-[#241b12]/8">
                {ACTIVITY.slice(0, 6).map((a, i) => (
                  <li key={i} className="flex gap-2.5 py-2">
                    <span className="mt-1.5 h-1.5 w-1.5 flex-none rounded-full" style={{ background: { session: '#3d9d6d', unlock: '#d9a514', fix: '#c2542e', update: '#8a8175', download: '#8a5fc4', tweak: '#c2542e' }[a.type] }} />
                    <div className="min-w-0">
                      <div className="truncate text-[11.5px] font-semibold leading-snug">{a.detail}</div>
                      <div className="font-mono text-[9px] text-[#241b12]/40">{a.ts} · {a.meta}</div>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
}

export function It15Game() {
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sync, setSync] = useState('fsync');
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [ups, setUps] = useState('NIS');
  const [hud, setHud] = useState('minimal');
  const [tricks, setTricks] = useState(ps.winetricks);

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button key={o} onClick={() => onChange(o)} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${value === o ? 'bg-[#241b12] text-[#f7f3ea] font-bold' : 'bg-[#241b12]/[0.06] text-[#241b12]/60 hover:bg-[#241b12]/[0.1]'}`}>{o}</button>
      ))}
    </div>
  );

  return (
    <div className="min-h-screen" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: 'radial-gradient(1000px 520px at 90% -8%, rgba(140,110,190,.14), transparent 52%), #f7f3ea', color: INK }}>
      <div className="mx-auto max-w-[1240px] px-8 py-6">
        <a href="/15" className="mb-4 inline-block rounded-full border border-[#241b12]/15 bg-white px-4 py-2 font-mono text-[10px] tracking-widest text-[#241b12]/60">← LIBRARY</a>

        {/* head */}
        <section className="mb-6 flex items-center gap-6">
          <img src={game.cover} alt="" className="h-32 w-24 rounded-2xl object-cover shadow-[0_10px_28px_rgba(36,27,18,.15)]" />
          <div className="flex-1">
            <div className="mb-1.5 flex gap-2 font-mono text-[9px] font-black uppercase tracking-[0.14em]">
              <span className="rounded-md bg-[#3d9d6d] px-2 py-0.5 text-white">{game.tier} VERIFIED</span>
              <span className="rounded-md bg-[#241b12]/8 px-2 py-0.5">{game.store}</span>
            </div>
            <h1 className="text-[30px] font-extrabold tracking-tight">{game.title}</h1>
            <p className="mt-1.5 max-w-[56ch] text-[13px] leading-relaxed text-[#241b12]/60">{game.verdict}</p>
          </div>
          <button className="rounded-2xl bg-[#241b12] px-8 py-3.5 text-[15px] font-extrabold text-[#f7f3ea]">▶ Play</button>
        </section>

        {/* screenshots row */}
        <section className="mb-7 grid grid-cols-3 gap-4">
          {SCREENS(game).map((s, i) => <img key={i} src={s} alt="" className="aspect-video w-full rounded-2xl object-cover shadow-[0_2px_10px_rgba(36,27,18,.06)]" />)}
        </section>

        {/* settings */}
        <div className="grid grid-cols-[1.55fr_1fr] gap-5 pb-12">
          <div className="space-y-5">
            <section className="rounded-2xl border border-[#241b12]/10 bg-white p-5">
              <h2 className="mb-4 text-[15px] font-bold">Proton environment flags</h2>
              <div className="grid grid-cols-2 gap-2.5">
                {flags.map((f) => (
                  <button key={f.key} onClick={() => setFlags(flags.map(x => x.key === f.key ? { ...x, on: !x.on } : x))} className={`flex items-start justify-between gap-3 rounded-xl border p-3.5 text-left transition ${f.on ? 'border-[#c2542e]/40 bg-[#c2542e]/[0.05]' : 'border-[#241b12]/10 hover:border-[#241b12]/25'}`}>
                    <div><div className="font-mono text-[11px] font-bold">{f.key.replace('PROTON_', '')}</div><div className="mt-0.5 text-[10.5px] leading-snug text-[#241b12]/50">{f.desc}</div></div>
                    <span className={`mt-0.5 h-4 w-7 flex-none rounded-full transition ${f.on ? 'bg-[#c2542e]' : 'bg-[#241b12]/15'}`}>
                      <span className="block h-3 w-3 translate-x-0.5 translate-y-0.5 rounded-full bg-white transition" style={{ transform: f.on ? 'translateX(12px)' : 'translateX(2px)' }} />
                    </span>
                  </button>
                ))}
              </div>
            </section>

            <section className="rounded-2xl border border-[#241b12]/10 bg-white p-5">
              <h2 className="mb-4 text-[15px] font-bold">Renderer & sync</h2>
              <div className="space-y-4">
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-[#241b12]/45">dxvk</div><Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-[#241b12]/45">vkd3d</div><Seg options={['VKD3D-Proton 2.14', 'VKD3D-Proton 2.13', 'off']} value="VKD3D-Proton 2.14" onChange={() => {}} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-[#241b12]/45">sync</div><Seg options={SYNC_MODES} value={sync} onChange={setSync} /></div>
                <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-[#241b12]/45">upscaler</div><Seg options={UPSCALERS} value={ups} onChange={setUps} /></div>
              </div>
            </section>

            <section className="rounded-2xl border border-[#241b12]/10 bg-white p-5">
              <h2 className="mb-3 text-[15px] font-bold">Winetricks</h2>
              <div className="flex flex-wrap gap-1.5">
                {WINETRICKS.map((t) => (
                  <button key={t} onClick={() => setTricks(tricks.includes(t) ? tricks.filter(x => x !== t) : [...tricks, t])} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${tricks.includes(t) ? 'bg-[#3d9d6d]/12 text-[#2c7a52] ring-1 ring-[#3d9d6d]/40' : 'bg-[#241b12]/[0.06] text-[#241b12]/50'}`}>{tricks.includes(t) ? '✓ ' : '+ '}{t}</button>
                ))}
              </div>
            </section>
          </div>

          <aside className="space-y-5">
            <div className="rounded-2xl border border-[#241b12]/10 bg-white p-5">
              <h2 className="mb-3 text-[15px] font-bold">Session</h2>
              <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-[#241b12]/45">mangohud</div>
              <Seg options={HUD_PRESETS} value={hud} onChange={setHud} />
              <div className="mt-4 space-y-2 font-mono text-[11px]">
                {[['Runner', game.runner], ['Prefix', `~/.platypus/pfx/${game.id}`]].map(([k, v]) => (
                  <div key={k} className="flex justify-between border-t border-[#241b12]/8 pt-2"><span className="text-[#241b12]/45">{k}</span><span className="truncate pl-2">{v}</span></div>
                ))}
              </div>
            </div>
            <div className="rounded-2xl bg-[#241b12] p-5 font-mono text-[11px] leading-relaxed text-[#f0e9dc]">
              <div className="mb-1.5 text-[9.5px] uppercase tracking-widest text-white/40">effective command</div>
              <div className="text-emerald-300/90">gamemoderun mangohud \</div>
              <div className="pl-4 text-emerald-300/90">proton run game.exe \</div>
              {flags.filter(f => f.on).map(f => <div key={f.key} className="pl-4 text-amber-300/90">{f.key}=1 \</div>)}
            </div>
            <div className="rounded-2xl border border-[#241b12]/10 bg-white p-5">
              <h2 className="mb-3 text-[15px] font-bold">Achievements</h2>
              <div className="flex items-baseline gap-2">
                <span className="text-[26px] font-extrabold text-[#c2542e]">{ach.unlockedCount}</span>
                <span className="font-mono text-[11px] text-[#241b12]/40">/{ach.total} · {ach.score}%</span>
              </div>
              <div className="mt-3 grid grid-cols-5 gap-2">
                {ach.list.slice(0, 5).map((a) => (
                  <div key={a.id} className={`grid aspect-square place-items-center rounded-xl text-[15px] ${a.unlocked ? 'bg-[#c2542e]/[0.08] ring-1 ring-[#c2542e]/40' : 'bg-[#241b12]/[0.05] opacity-25'}`}>★</div>
                ))}
              </div>
            </div>
          </aside>
        </div>
      </div>
    </div>
  );
}