import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, achievementsFor, SCREENS, HOUR_BARS,
  protonSettingsFor, PROTON_FLAGS, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS,
} from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /14 — ITERATION 3: hero-forward. Keeps Galaxy's warm wash
   and sidebar, but the dashboard opens with /6's full-bleed
   cinematic hero ("tonight's game") and the game page leads
   with a media gallery; settings become a slide-over sheet.
   ============================================================ */

const game = GAMES[13]; // Witcher 3

export function It14Dashboard() {
  const hero = GAMES[11];
  return (
    <div
      className="min-h-screen text-[#eee8dd]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1200px 700px at 50% -20%, rgba(150,96,58,.42), transparent 60%), radial-gradient(900px 500px at 90% 90%, rgba(86,62,128,.4), transparent 55%), #120e08',
      }}
    >
      {/* full-bleed hero directly on the ground */}
      <section className="relative h-[420px]">
        <img src={hero.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg,#120e08 3%, rgba(18,14,8,.3) 45%, rgba(18,14,8,.65))' }} />
        <div className="absolute inset-x-0 top-0 flex items-center justify-between px-10 py-5">
          <div className="flex items-center gap-2.5">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-amber-300 to-rose-500 text-[13px] font-black text-[#120e08]">H</span>
            <span className="text-[15px] font-extrabold tracking-tight">Hydra</span>
          </div>
          <div className="flex items-center gap-2.5">
            <span className="rounded-full bg-black/40 px-4 py-2 text-[12px] font-bold text-white/70 backdrop-blur">search…</span>
            <img src="https://picsum.photos/seed/hyd-avatar/80/80" alt="" className="h-8 w-8 rounded-full ring-1 ring-white/25" />
          </div>
        </div>
        <div className="absolute bottom-9 left-10 max-w-xl">
          <div className="mb-3 flex gap-2 font-mono text-[9px] font-black uppercase tracking-[0.15em]">
            <span className="rounded-lg bg-orange-400 px-2 py-1 text-[#120e08]">fix needed</span>
            <span className="rounded-lg bg-white/10 px-2 py-1 text-white/70 backdrop-blur">helldivers 2 · patch 10.1</span>
          </div>
          <h1 className="text-[44px] font-extrabold leading-none tracking-tight">One fix before<br />you drop in.</h1>
          <p className="mt-3 max-w-[46ch] text-[13.5px] leading-relaxed text-white/60">{hero.verdict}</p>
          <div className="mt-5 flex gap-2.5">
            <a href="/14/game" className="rounded-xl bg-amber-300 px-6 py-3 text-[13px] font-extrabold text-[#120e08] transition hover:bg-amber-200">Review fix</a>
            <button className="rounded-xl bg-white/10 px-5 py-3 text-[13px] font-bold text-white/85 backdrop-blur">Dismiss</button>
          </div>
        </div>
      </section>

      <main className="mx-auto max-w-[1360px] px-10 pb-14">
        {/* plates row — compact trio like galaxy */}
        <section className="mb-8 grid grid-cols-3 gap-5">
          {[
            ['Hours this week', '12.7', '+3.1 vs last', 'text-amber-300'],
            ['Achievement points', '1,240', '+45 this week', 'text-amber-200'],
            ['Ready now', '19 / 24', '5 need tweaks', 'text-emerald-300'],
          ].map(([k, v, s, c]) => (
            <div key={k} className="rounded-2xl bg-white/[0.045] p-5 ring-1 ring-white/[0.07]">
              <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-white/35">{k}</div>
              <div className="mt-1.5 flex items-baseline justify-between">
                <span className={`text-[27px] font-extrabold tabular-nums ${c}`}>{v}</span>
                <span className="font-mono text-[9.5px] text-white/30">{s}</span>
              </div>
              <div className="mt-3 flex h-8 items-end gap-1">
                {HOUR_BARS.slice(0, 14).map((h, i) => <span key={i} className="w-2 rounded-sm bg-white/[0.12]" style={{ height: `${(h / 10) * 100}%`, opacity: i === 13 ? 1 : 0.5 }} />)}
              </div>
            </div>
          ))}
        </section>

        <Rail title="Continue playing" games={GAMES.slice(0, 6)} />

        {/* bottom: activity + achievements */}
        <section className="grid grid-cols-[1.55fr_1fr] gap-5 pb-6 pt-6">
          <div className="rounded-2xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
            <div className="mb-1 flex items-center justify-between">
              <h2 className="text-[15px] font-bold">Recent activity</h2>
              <button className="font-mono text-[10px] uppercase tracking-widest text-white/30 hover:text-white/70">full log →</button>
            </div>
            <ActivityFeed limit={6} />
          </div>
          <div className="rounded-2xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
            <h2 className="mb-3.5 text-[15px] font-bold">Latest unlocks</h2>
            <div className="grid grid-cols-2 gap-3">
              {achievementsFor(GAMES[0]).list.filter(a => a.unlocked).slice(0, 4).map((a) => (
                <div key={a.id} className="rounded-xl bg-black/25 p-3">
                  <div className="grid h-9 w-9 place-items-center rounded-lg bg-amber-300/12 text-[14px] ring-1 ring-amber-300/35">★</div>
                  <div className="mt-2 truncate text-[12px] font-bold">{a.name}</div>
                  <div className="font-mono text-[9px] text-white/30">{a.rarity}</div>
                </div>
              ))}
            </div>
          </div>
        </section>
      </main>
    </div>
  );
}

function Rail({ title, games }) {
  return (
    <section className="pt-8">
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-[17px] font-bold tracking-tight">{title}</h2>
        <button className="text-[12px] font-semibold text-white/35 hover:text-white/80">see all →</button>
      </div>
      <div className="flex gap-3.5 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
        {games.map((g) => (
          <a key={g.id} href="/14/game" className="group w-[172px] flex-none">
            <div className="overflow-hidden rounded-2xl ring-1 ring-white/[0.08] transition duration-300 group-hover:-translate-y-1 group-hover:ring-amber-300/40">
              <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover" />
            </div>
            <div className="mt-2 flex items-baseline justify-between">
              <span className="truncate text-[12.5px] font-bold">{g.title.split(':')[0]}</span>
              <span className="font-mono text-[9px] uppercase text-amber-300/80">{g.tier}</span>
            </div>
          </a>
        ))}
      </div>
    </section>
  );
}

export function It14Game() {
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sheet, setSheet] = useState(false);
  const [sync, setSync] = useState('fsync');
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [ups, setUps] = useState('FSR 3');
  const [hud, setHud] = useState('full');

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button key={o} onClick={() => onChange(o)} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${value === o ? 'bg-amber-300 font-bold text-[#120e08]' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]'}`}>{o}</button>
      ))}
    </div>
  );

  return (
    <div
      className="min-h-screen text-[#eee8dd]"
      style={{
        fontFamily: '"Hanken Grotesk", sans-serif',
        background: 'radial-gradient(1000px 560px at 88% -10%, rgba(86,62,128,.45), transparent 56%), #120e08',
      }}
    >
      {/* media-led header with gallery strip */}
      <section className="relative h-[340px]">
        <img src={game.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg,#120e08 4%, rgba(18,14,8,.15) 50%, rgba(18,14,8,.6))' }} />
        <div className="absolute inset-x-0 top-0 flex items-center justify-between px-9 py-5">
          <a href="/14" className="rounded-full bg-black/45 px-4 py-2 font-mono text-[10px] uppercase tracking-[0.16em] text-white/85 backdrop-blur">← library</a>
          <button onClick={() => setSheet(true)} className="rounded-full bg-amber-300 px-5 py-2 text-[12.5px] font-extrabold text-[#120e08]">⚙ Launcher settings</button>
        </div>
        <div className="absolute bottom-7 left-9 right-9 flex items-end justify-between">
          <div>
            <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.2em] text-white/55">{game.store} · {game.tier} · {game.size}</div>
            <h1 className="text-[42px] font-extrabold leading-none tracking-tight">{game.title}</h1>
          </div>
          <div className="text-right font-mono text-[10.5px] text-white/45">
            <div>{game.hours}h logged</div><div className="mt-1">{game.last}</div>
          </div>
        </div>
      </section>

      {/* floating gallery strip overlapping the fold */}
      <section className="relative z-10 mx-auto -mt-16 flex max-w-[1240px] gap-4 px-8">
        <button className="relative aspect-[2/3] w-40 flex-none overflow-hidden rounded-2xl ring-2 ring-amber-300/70">
          <img src={game.cover} alt="" className="h-full w-full object-cover" />
          <span className="absolute bottom-2 left-2 rounded bg-black/55 px-1.5 py-0.5 font-mono text-[8px] uppercase tracking-wider text-white/85">cover</span>
        </button>
        {SCREENS(game).map((s, i) => (
          <button key={i} className="aspect-[2/3] w-40 flex-none overflow-hidden rounded-2xl opacity-70 transition hover:opacity-100">
            <img src={s} alt="" className="h-full w-full object-cover" />
          </button>
        ))}
      </section>

      {/* content */}
      <main className="mx-auto grid max-w-[1240px] grid-cols-[1.5fr_1fr] gap-6 px-8 pb-14 pt-8">
        <div className="space-y-6">
          <p className="rounded-2xl bg-white/[0.045] p-5 text-[14px] leading-relaxed text-white/70 ring-1 ring-white/[0.07]">{game.verdict}</p>

          <section>
            <h2 className="mb-3.5 text-[16px] font-bold tracking-tight">Today's session recap</h2>
            <div className="rounded-2xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
              <ActivityFeed limit={4} />
            </div>
          </section>

          <section>
            <div className="mb-3.5 flex items-baseline justify-between">
              <h2 className="text-[16px] font-bold tracking-tight">Achievements</h2>
              <span className="font-mono text-[11px] text-white/40">{ach.unlockedCount}/{ach.total} · {ach.score}%</span>
            </div>
            <div className="grid grid-cols-4 gap-3">
              {ach.list.slice(0, 8).map((a) => (
                <div key={a.id} className={`rounded-2xl p-3.5 ring-1 ${a.unlocked ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'opacity-30 ring-white/[0.06]'}`}>
                  <div className={`grid h-10 w-10 place-items-center rounded-xl text-[15px] ${a.unlocked ? 'bg-amber-300/15 ring-1 ring-amber-300/40' : 'bg-white/[0.05]'}`}>★</div>
                  <div className="mt-2 truncate text-[12px] font-bold">{a.name}</div>
                  <div className="font-mono text-[9px] text-white/30">{a.unlocked ? a.date : a.rarity}</div>
                </div>
              ))}
            </div>
          </section>
        </div>

        <aside className="space-y-4">
          <div className="rounded-2xl bg-white/[0.045] p-5 ring-1 ring-white/[0.07]">
            <h3 className="mb-3 text-[13px] font-bold">Configuration summary</h3>
            {[['Runner', game.runner], ['DXVK', dxvk], ['Sync', sync], ['Upscaler', ups], ['HUD', hud], ['Prefix', `~/.hydra/pfx/${game.id}`]].map(([k, v]) => (
              <div key={k} className="flex justify-between border-b border-white/[0.05] py-2 font-mono text-[11px] last:border-0">
                <span className="text-white/35">{k}</span><span className="truncate pl-2 text-white/85">{v}</span>
              </div>
            ))}
            <button onClick={() => setSheet(true)} className="mt-3.5 w-full rounded-xl bg-white/[0.08] py-2.5 text-[12.5px] font-bold text-white/85 transition hover:bg-white/[0.14]">Open full settings</button>
          </div>
          <div className="rounded-2xl bg-amber-300/[0.07] p-5 ring-1 ring-amber-300/25">
            <h3 className="text-[13px] font-extrabold text-amber-300">Community verdict</h3>
            <p className="mt-2 text-[12.5px] leading-relaxed text-white/60">{FIXES[6]?.line}</p>
          </div>
        </aside>
      </main>

      {/* settings slide-over sheet */}
      {sheet && (
        <div className="fixed inset-0 z-50 bg-black/55" onClick={() => setSheet(false)}>
          <div className="absolute right-0 top-0 h-full w-[520px] overflow-y-auto bg-[#1a140c] p-7 shadow-2xl" style={{ boxShadow: '-30px 0 80px rgba(0,0,0,.5)' }} onClick={(e) => e.stopPropagation()}>
            <div className="mb-6 flex items-center justify-between">
              <h2 className="text-[19px] font-extrabold tracking-tight">{game.title} — launcher settings</h2>
              <button onClick={() => setSheet(false)} className="rounded-lg bg-white/[0.07] px-3 py-1.5 text-[12px] font-bold">close</button>
            </div>
            <div className="space-y-6">
              <div>
                <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">runner</div>
                <div className="space-y-2">
                  {RUNNERS.slice(0, 4).map((r, i) => (
                    <button key={r.name} className={`flex w-full items-center justify-between rounded-xl border px-4 py-3 text-left transition ${i === 0 ? 'border-amber-300/50 bg-amber-300/[0.07]' : 'border-white/[0.08] hover:border-white/20'}`}>
                      <div><div className="text-[13px] font-bold">{r.name}</div><div className="font-mono text-[9.5px] text-white/35">{r.kind} · updated {r.updated}</div></div>
                      <span className={`h-4 w-4 rounded-full border-2 ${i === 0 ? 'border-amber-300 bg-amber-300' : 'border-white/20'}`} />
                    </button>
                  ))}
                </div>
              </div>
              <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">dxvk</div><Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} /></div>
              <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">sync</div><Seg options={SYNC_MODES} value={sync} onChange={setSync} /></div>
              <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">upscaler</div><Seg options={UPSCALERS} value={ups} onChange={setUps} /></div>
              <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">mangohud</div><Seg options={HUD_PRESETS} value={hud} onChange={setHud} /></div>
              <div>
                <div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">proton flags</div>
                <div className="space-y-1.5">
                  {flags.map((f) => (
                    <button key={f.key} onClick={() => setFlags(flags.map(x => x.key === f.key ? { ...x, on: !x.on } : x))} className={`flex w-full items-center justify-between rounded-xl px-4 py-2.5 transition ${f.on ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04]'}`}>
                      <span className="font-mono text-[11px] font-bold text-white/85">{f.key.replace('PROTON_', '')}</span>
                      <span className={`h-4 w-8 rounded-full ${f.on ? 'bg-amber-300' : 'bg-white/15'}`}>
                        <span className="block h-3 w-3 translate-x-0.5 translate-y-0.5 rounded-full bg-[#120e08]" style={{ transform: f.on ? 'translateX(14px)' : '' }} />
                      </span>
                    </button>
                  ))}
                </div>
              </div>
              <button className="w-full rounded-xl bg-amber-300 py-3 text-[14px] font-extrabold text-[#120e08]">Save & close</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}