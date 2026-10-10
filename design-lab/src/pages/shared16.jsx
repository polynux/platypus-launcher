// Shared building blocks for /17–/21 — the /16-synthesis world.
// All five iterations import from here so they only vary in what they test.
import React, { useState } from 'react';
import {
  GAMES, RUNNERS, FIXES, ACTIVITY, achievementsFor, SCREENS, HOUR_BARS, img,
  protonSettingsFor, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS, PROTON_FLAGS,
} from '../data.js';
import { ActivityFeed } from './p6.jsx';

export const WX = {
  ground: '#130f0a',
  wash: 'radial-gradient(1100px 620px at 14% -8%, rgba(150,96,58,.46), transparent 58%),' +
        'radial-gradient(880px 520px at 90% 10%, rgba(86,62,128,.44), transparent 54%),' +
        'radial-gradient(760px 460px at 52% 108%, rgba(30,78,96,.36), transparent 56%),' + ' #130f0a',
  gameWash: 'radial-gradient(1000px 560px at 88% -10%, rgba(86,62,128,.46), transparent 56%), #130f0a',
};

const ACC = {
  amber: 'bg-[#fbbf24]', amberText: 'text-[#fbbf24]', amberBg: 'bg-amber-300/[0.08]', amberRing: 'ring-amber-300/40',
  ok: 'text-emerald-300', err: 'text-orange-300',
};

/* ---------- Sidebar (16 + 12's friends panel) ---------- */
export function Sidebar({ active = 'Dashboard', libraryUrl = '/library' }) {
  return (
    <aside className="sticky top-0 flex h-screen w-56 flex-none flex-col border-r border-white/[0.06] bg-black/25 p-4">
      <div className="mb-5 flex items-center gap-2.5 px-1.5">
        <span className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-amber-300 to-rose-500 text-[15px] font-black text-[#130f0a]">P</span>
        <div><div className="text-[15px] font-extrabold tracking-tight">Platypus</div><div className="font-mono text-[8.5px] uppercase tracking-[0.16em] text-white/30">game hub</div></div>
      </div>
      <div className="mb-4 rounded-xl bg-white/[0.06] px-3.5 py-2.5 text-[12px] text-white/35">🔍 Search…</div>
      {[['Dashboard', 0], ['Library', 24], ['Downloads', 1], ['Achievements', 0], ['Activity', 0]].map(([l, c]) => (
        <a key={l} href={l === 'Library' ? libraryUrl : '#'} className={`mb-0.5 flex w-full items-center justify-between rounded-xl px-3 py-2 text-[12.5px] font-semibold ${active === l ? 'bg-amber-300/12 text-amber-200' : 'text-white/50 hover:bg-white/[0.05] hover:text-white'}`}>
          <span>{l}</span>{!!c && <span className="rounded-full bg-white/[0.07] px-2 font-mono text-[9.5px]">{c}</span>}
        </a>
      ))}
      <div className="px-2 pb-1.5 pt-4 font-mono text-[8.5px] uppercase tracking-[0.18em] text-white/25">sources</div>
      {['Steam', 'GOG', 'Epic'].map((s, i) => (
        <button key={s} className="mb-0.5 flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-[12.5px] text-white/50 hover:text-white">
          <span className="flex items-center gap-2"><span className="h-1.5 w-1.5 rounded-full bg-amber-300/50" />{s}</span>
          <span className="font-mono text-[9.5px] text-white/25">{[14, 5, 5][i]}</span>
        </button>
      ))}

      {/* friends online — pinned panel, fixed 3 rows, no overlap */}
      <div className="mt-auto flex flex-none flex-col gap-2 pb-1 pt-4">
        <div className="flex items-center justify-between px-1.5">
          <span className="font-mono text-[8.5px] uppercase tracking-[0.18em] text-white/30">Friends</span>
          <span className="font-mono text-[8.5px] text-emerald-300">● 2 online</span>
        </div>
        <div className="space-y-1 rounded-2xl bg-white/[0.05] p-2.5">
          {[
            { n: 'Kai', g: 'Deep Rock Galactic', st: 'in game', seed: 'f1' },
            { n: 'Mara', g: 'Stardew Valley', st: 'in game', seed: 'f2' },
            { n: 'Juno', g: 'away · 3h', st: 'away', seed: 'f3' },
          ].map((f) => (
            <div key={f.n} className="flex items-center gap-2.5">
              <div className="relative flex-none">
                <img src={`https://picsum.photos/seed/hyd-${f.seed}/48/48`} alt="" className="h-8 w-8 rounded-lg object-cover" />
                <span className={`absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full ${f.st === 'in game' ? 'bg-emerald-400' : 'bg-white/35'}`} style={{ boxShadow: '0 0 0 2px #171310' }} />
              </div>
              <div className="min-w-0 leading-tight">
                <div className="text-[11.5px] font-bold">{f.n}</div>
                <div className={`truncate font-mono text-[8.5px] uppercase tracking-wider ${f.st === 'in game' ? 'text-emerald-300/80' : 'text-white/30'}`}>{f.g}</div>
              </div>
            </div>
          ))}
        </div>
        <div className="px-1.5 font-mono text-[8.5px] text-white/20">platypus v0.2 · scan 09:41</div>
      </div>
    </aside>
  );
}

/* ---------- Dashboard hero variant A: fix-needed banner (16) ---------- */
export function HeroFix({ url }) {
  const hero = GAMES[11];
  return (
    <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
      <img src={hero.wide} alt="" className="h-[290px] w-full object-cover" />
      <div className="absolute inset-0" style={{ background: 'linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)' }} />
      <div className="absolute bottom-6 left-7 max-w-md">
        <span className="rounded-lg bg-orange-400 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#130f0a]">action required</span>
        <h1 className="mt-2.5 text-[27px] font-extrabold leading-tight tracking-tight">{hero.title} needs its fix re-applied</h1>
        <div className="mt-3.5 flex gap-2.5">
          <a href={url} className="rounded-xl bg-amber-300 px-5 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">Open game</a>
          <button className="rounded-xl bg-white/10 px-4 py-2.5 text-[12.5px] font-bold text-white/85">Later</button>
        </div>
      </div>
    </section>
  );
}

/* ---------- Stat chips riding hero right ---------- */
export function StatChipsOnHero({ stats }) {
  return (
    <div className="absolute bottom-6 right-7 hidden gap-3 lg:flex">
      {stats.map(([k, v, c]) => (
        <div key={k} className="w-28 rounded-2xl bg-black/45 p-3.5 backdrop-blur">
          <div className={`text-[19px] font-extrabold tabular-nums ${c}`}>{v}</div>
          <div className="font-mono text-[8.5px] uppercase tracking-widest text-white/40">{k}</div>
        </div>
      ))}
    </div>
  );
}

/* ---------- Continue grid (16) ---------- */
export function ContinueGrid({ games, url, cols = 'grid-cols-4', showDot = true }) {
  return (
    <div className={`grid ${cols} gap-3.5`}>
      {games.map((g) => (
        <a key={g.id} href={url} className="group overflow-hidden rounded-2xl bg-white/[0.045] ring-1 ring-white/[0.07] transition duration-300 hover:-translate-y-0.5 hover:ring-amber-300/40">
          <div className="relative">
            <img src={g.cover} alt="" className="aspect-[3/4] w-full object-cover" />
            {showDot && g.status === 'tweak' && <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,.8)]" />}
          </div>
          <div className="px-3 py-2.5">
            <div className="truncate text-[12px] font-bold">{g.title.split(':')[0]}</div>
            <div className="font-mono text-[8.5px] uppercase tracking-wider text-white/30">{g.hours}h · {g.runner.split(' ')[0]}</div>
          </div>
        </a>
      ))}
    </div>
  );
}

/* ---------- Recent list (13's compactness, card-era dressing) ---------- */
export function RecentList({ games, url }) {
  return (
    <ul className="divide-y divide-white/[0.05]">
      {games.map((g) => (
        <li key={g.id}>
          <a href={url} className="group -mx-1 flex items-center gap-3 rounded-lg px-1 py-2 hover:bg-white/[0.04]">
            <img src={g.cover} alt="" className="h-8 w-8 flex-none rounded object-cover ring-1 ring-white/10" />
            <div className="min-w-0 flex-1 leading-tight">
              <div className="truncate text-[12px] font-semibold">{g.title}</div>
              <div className="font-mono text-[8.5px] uppercase tracking-wider text-white/30">{g.last} · {g.hours}h</div>
            </div>
            <span className={`font-mono text-[9px] uppercase ${g.tier === 'platinum' ? 'text-amber-300/80' : 'text-white/30'}`}>{g.tier}</span>
          </a>
        </li>
      ))}
    </ul>
  );
}

/* ---------- Waiting-on list (16) ---------- */
export function WaitingOn({ games, url }) {
  return (
    <div className="space-y-3">
      {games.map((g) => (
        <a key={g.id} href={url} className="block rounded-2xl bg-white/[0.045] p-4 ring-1 ring-white/[0.06] transition hover:ring-orange-400/40">
          <div className="flex items-center justify-between">
            <span className="truncate text-[13px] font-bold">{g.title}</span>
            <span className="font-mono text-[9px] uppercase tracking-wider text-orange-300">{g.tier}</span>
          </div>
          <p className="mt-1 text-[11px] leading-relaxed text-white/45 line-clamp-2">{g.verdict}</p>
        </a>
      ))}
    </div>
  );
}

/* ---------- Activity panel ---------- */
export function ActivityPanel({ limit = 8 }) {
  return (
    <div className="rounded-3xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
      <div className="mb-2 flex items-center justify-between">
        <h2 className="text-[15px] font-bold">Activity</h2>
        <button className="font-mono text-[10px] uppercase tracking-widest text-white/30 hover:text-white/70">full log →</button>
      </div>
      <ActivityFeed limit={limit} />
    </div>
  );
}

/* ---------- Game page shell: media + summary + achievements default;
   settings behind a centered modal ---------- */
export function GamePageShell({ game, children }) {
  const ps = protonSettingsFor(game);
  const ach = achievementsFor(game);
  const [flags, setFlags] = useState(ps.flags);
  const [sync, setSync] = useState('fsync');
  const [dxvk, setDxvk] = useState(ps.dxvk);
  const [vkd3d, setVkd3d] = useState('VKD3D-Proton 2.14');
  const [ups, setUps] = useState('FSR 3');
  const [hud, setHud] = useState('full');
  const [tricks, setTricks] = useState(ps.winetricks);
  const [modal, setModal] = useState(false);
  const [activeShot, setActiveShot] = useState(0);

  const Seg = ({ options, value, onChange }) => (
    <div className="flex flex-wrap gap-1.5">
      {options.map((o) => (
        <button key={String(o)} onClick={() => onChange(o)} className={`rounded-lg px-3 py-1.5 font-mono text-[11px] transition ${value === o ? 'bg-amber-300 font-bold text-[#130f0a]' : 'bg-white/[0.06] text-white/55 hover:bg-white/[0.1]'}`}>{o}</button>
      ))}
    </div>
  );

  return (
    <div
      className="min-h-screen text-[#ece6da]"
      style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.gameWash }}
    >
      <div className="px-10 py-6">
        <a href={children.back || '/library'} className="text-[12px] font-semibold text-white/40 hover:text-white">← library</a>

        {/* head */}
        <section className="relative mt-4 mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <img src={game.wide} alt="" className="h-[280px] w-full object-cover" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.98) 10%, rgba(19,15,10,.35) 62%, rgba(19,15,10,.25) 100%)' }} />
          <div className="absolute bottom-6 left-8 right-8 flex items-end justify-between gap-6">
            <div className="min-w-0">
              <div className="mb-2.5 flex gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
                <span className="rounded-lg bg-amber-300 px-2.5 py-1 text-[#130f0a]">{game.tier} verified</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{game.store}</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{game.size}</span>
              </div>
              <h1 className="truncate text-[36px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <div className="flex flex-none items-center gap-4">
              <div className="text-right font-mono text-[10px] leading-relaxed text-white/40"><div>{game.hours}h logged</div><div>last {game.last}</div></div>
              <button className="rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#130f0a] shadow-[0_12px_30px_rgba(252,211,77,.25)]">▶ Play</button>
            </div>
          </div>
        </section>

        {/* DEFAULT VIEW: screenshot viewer + summary + achievements */}
        <div className="mb-6 grid grid-cols-[1.6fr_1fr] gap-5">
          {/* screenshot viewer */}
          <div className="space-y-3">
            <div className="overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
              <img key={activeShot} src={SCREENS(game)[activeShot]} alt="" className="aspect-video w-full object-cover" />
            </div>
            <div className="flex gap-2.5 overflow-x-auto pb-1" style={{ scrollbarWidth: 'none' }}>
              {SCREENS(game).map((s, i) => (
                <button key={i} onClick={() => setActiveShot(i)} className={`aspect-video w-[150px] flex-none overflow-hidden rounded-xl transition ${i === activeShot ? 'ring-2 ring-amber-300/80' : 'opacity-55 ring-1 ring-white/[0.08] hover:opacity-90'}`}>
                  <img src={s} alt="" className="w-full object-cover" />
                </button>
              ))}
            </div>
          </div>

          {/* summary rail */}
          <aside className="space-y-5">
            <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
              <h2 className="mb-2 text-[15px] font-bold">Summary</h2>
              <p className="text-[13px] leading-relaxed text-white/60">{game.verdict}</p>
              <div className="mt-4 space-y-2 font-mono text-[10.5px]">
                {[['Runner', game.runner], ['DX layer', game.dx], ['Uptime', `${game.hours}h`]].map(([k, v]) => (
                  <div key={k} className="flex justify-between border-t border-white/[0.05] pt-2"><span className="text-white/35">{k}</span><span className="text-white/85">{v}</span></div>
                ))}
              </div>
            </div>
            <button onClick={() => setModal(true)} className="w-full rounded-2xl border border-white/[0.1] bg-white/[0.06] px-5 py-3 text-[13px] font-bold text-white/80 transition hover:bg-white/[0.1]">⚙ Launcher settings</button>
          </aside>
        </div>

        {/* achievements — always on default view */}
        <section className="pb-10">
          <div className="mb-3.5 flex items-baseline justify-between">
            <h2 className="text-[16px] font-bold tracking-tight">Achievements</h2>
            <span className="font-mono text-[11px] text-white/40">{ach.unlockedCount}/{ach.total} · {ach.score}%</span>
          </div>
          <div className="mb-4 h-1.5 rounded-full bg-white/[0.08]"><div className="h-full rounded-full bg-amber-300" style={{ width: `${ach.score}%` }} /></div>
          <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(190px, 220px))' }}>
            {ach.list.slice(0, 12).map((a) => (
              <div key={a.id} className={`rounded-2xl p-3.5 ring-1 ${a.unlocked ? 'bg-amber-300/[0.07] ring-amber-300/30' : 'opacity-30 ring-white/[0.06]'}`}>
                <div className={`grid h-10 w-10 place-items-center rounded-xl text-[15px] ${a.unlocked ? 'bg-amber-300/15 ring-1 ring-amber-300/40' : 'bg-white/[0.05]'}`}>★</div>
                <div className="mt-2 truncate text-[12px] font-bold">{a.name}</div>
                <div className="font-mono text-[9px] text-white/30">{a.unlocked ? a.date : a.rarity}</div>
              </div>
            ))}
          </div>
        </section>

        {/* settings MODAL (14's layout, centered) */}
        {modal && (
          <div className="fixed inset-0 z-50 grid place-items-center bg-black/65 p-6" onClick={() => setModal(false)}>
            <div
              className="max-h-[86vh] w-[880px] overflow-y-auto rounded-3xl bg-[#191410] p-7 ring-1 ring-white/10"
              style={{ boxShadow: '0 40px 100px rgba(0,0,0,.6)' }}
              onClick={(e) => e.stopPropagation()}
            >
              <div className="mb-6 flex items-center justify-between">
                <div>
                  <h2 className="text-[20px] font-extrabold tracking-tight">Launcher settings</h2>
                  <div className="font-mono text-[10px] text-white/35">{game.title}</div>
                </div>
                <button onClick={() => setModal(false)} className="rounded-lg bg-white/[0.07] px-3 py-1.5 text-[12px] font-bold">close</button>
              </div>

              <div className="grid grid-cols-[1.5fr_1fr] gap-6">
                <div className="space-y-6">
                  {/* runner */}
                  <div>
                    <div className="mb-2 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">runner</div>
                    <div className="space-y-2">
                      {RUNNERS.slice(0, 4).map((r, i) => (
                        <button key={r.name} className={`flex w-full items-center justify-between rounded-xl border px-4 py-3 text-left transition ${i === 0 ? 'border-amber-300/50 bg-amber-300/[0.07]' : 'border-white/[0.08] hover:border-white/20'}`}>
                          <div><div className="text-[13px] font-bold">{r.name}</div><div className="font-mono text-[9.5px] text-white/35">{r.kind} · updated {r.updated}</div></div>
                          <span className={`h-4 w-4 rounded-full border-2 ${i === 0 ? 'border-amber-300 bg-amber-300' : 'border-white/20'}`} />
                        </button>
                      ))}
                    </div>
                  </div>
                  {/* renderer segments */}
                  <div className="grid grid-cols-2 gap-x-5 gap-y-4">
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">dxvk</div><Seg options={DXVK_VERSIONS} value={dxvk} onChange={setDxvk} /></div>
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">vkd3d-proton</div><Seg options={VKD3D_VERSIONS} value={vkd3d} onChange={setVkd3d} /></div>
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">sync</div><Seg options={SYNC_MODES} value={sync} onChange={setSync} /></div>
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">upscaler</div><Seg options={UPSCALERS} value={ups} onChange={setUps} /></div>
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">mangohud</div><Seg options={HUD_PRESETS} value={hud} onChange={setHud} /></div>
                    <div><div className="mb-1.5 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">shader cache</div><Seg options={['on', 'off']} value="on" onChange={() => {}} /></div>
                  </div>
                </div>

                {/* right col of modal */}
                <div className="space-y-5">
                  <div>
                    <div className="mb-2 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">proton flags</div>
                    <div className="space-y-1.5">
                      {flags.map((f) => (
                        <button key={f.key} onClick={() => setFlags(flags.map(x => x.key === f.key ? { ...x, on: !x.on } : x))} className={`flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 transition ${f.on ? 'bg-amber-300/[0.08] ring-1 ring-amber-300/35' : 'bg-white/[0.04]'}`}>
                          <span className="text-left font-mono text-[10.5px] font-bold text-white/85">{f.key.replace('PROTON_', '').replace(/_/g, ' ').toLowerCase()}</span>
                          <span className={`h-4 w-8 flex-none rounded-full ${f.on ? 'bg-amber-300' : 'bg-white/15'}`}>
                            <span className="block h-3 w-3 translate-x-0.5 translate-y-0.5 rounded-full bg-[#191410]" style={{ transform: f.on ? 'translateX(14px)' : '' }} />
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                  <div>
                    <div className="mb-2 font-mono text-[9.5px] uppercase tracking-[0.16em] text-white/35">winetricks</div>
                    <div className="flex flex-wrap gap-1.5">
                      {WINETRICKS.map((t) => (
                        <button key={t} onClick={() => setTricks(tricks.includes(t) ? tricks.filter(x => x !== t) : [...tricks, t])} className={`rounded-lg px-2.5 py-1.5 font-mono text-[10.5px] transition ${tricks.includes(t) ? 'bg-emerald-300/15 text-emerald-200 ring-1 ring-emerald-300/40' : 'bg-white/[0.06] text-white/45'}`}>{tricks.includes(t) ? '✓ ' : '+ '}{t}</button>
                      ))}
                    </div>
                  </div>
                  <div className="rounded-xl bg-black/35 p-3.5 font-mono text-[10.5px] leading-relaxed ring-1 ring-white/[0.07]">
                    <div className="mb-1 text-white/30">$ preview</div>
                    <div className="text-emerald-200/90">gamemoderun mangohud \</div>
                    <div className="pl-3 text-emerald-200/90">proton run \</div>
                    {flags.filter(f => f.on).map(f => <div key={f.key} className="pl-3 text-amber-200/80">{f.key}=1 \</div>)}
                  </div>
                  <div className="flex gap-2.5">
                    <button onClick={() => setModal(false)} className="flex-1 rounded-xl bg-white/[0.07] py-2.5 text-[12.5px] font-bold text-white/70">Cancel</button>
                    <button onClick={() => setModal(false)} className="flex-1 rounded-xl bg-amber-300 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">Save config</button>
                  </div>
                  <div className="font-mono text-[9px] text-white/25">prefix: {ps.prefix}</div>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}