import React, { useState } from 'react';
import { GAMES, RUNNERS, achievementsFor, SCREENS, HOUR_BARS, protonSettingsFor, SYNC_MODES, DXVK_VERSIONS, VKD3D_VERSIONS, WINETRICKS, UPSCALERS, HUD_PRESETS } from '../data.js';
import { WX, Sidebar, ContinueGrid, WaitingOn, ActivityPanel, GamePageShell } from './shared16.jsx';

/* /20 — GAME PAGE STUDY: tabbed game page (overview / achievements /
   runs) where settings stays a modal but achievements get their own
   full tab; dashboard is 16-base. Tests focused achievement surface. */
export function It20Dashboard() {
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar active="Dashboard" />
        <main className="flex-1 p-7">
          {/* compact hero */}
          <section className="relative mb-6 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
            <img src={GAMES[11].wide} alt="" className="h-[280px] w-full object-cover" />
            <div className="absolute inset-0" style={{ background: 'linear-gradient(84deg, rgba(19,15,10,.97) 30%, rgba(19,15,10,.35) 62%, transparent)' }} />
            <div className="absolute bottom-6 left-7 max-w-md">
              <span className="rounded-lg bg-orange-400 px-2.5 py-1 font-mono text-[9px] font-black uppercase tracking-wider text-[#130f0a]">action required</span>
              <h1 className="mt-2.5 text-[26px] font-extrabold leading-tight tracking-tight">{GAMES[11].title} needs its fix re-applied</h1>
              <div className="mt-3.5 flex gap-2.5">
                <a href="/20-game" className="rounded-xl bg-amber-300 px-5 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">Open game</a>
                <button className="rounded-xl bg-white/10 px-4 py-2.5 text-[12.5px] font-bold text-white/85">Later</button>
              </div>
            </div>
          </section>

          <section className="mb-6 grid grid-cols-[1.75fr_1fr] gap-5">
            <div>
              <div className="mb-3 flex items-baseline justify-between">
                <h2 className="text-[16px] font-bold tracking-tight">Continue playing</h2>
                <a href="/library" className="text-[11.5px] font-semibold text-white/35 hover:text-white/80">library →</a>
              </div>
              <ContinueGrid games={GAMES.slice(0, 8)} url="/20-game" />
            </div>
            <div>
              <div className="mb-3"><h2 className="text-[16px] font-bold tracking-tight">Waiting on you</h2></div>
              <WaitingOn games={GAMES.filter(g => g.status === 'tweak').slice(0, 3)} url="/20-game" />
            </div>
          </section>
          <ActivityPanel limit={8} />
        </main>
      </div>
    </div>
  );
}

export function It20Game({ game = GAMES[1] }) {
  const ach = achievementsFor(game);
  const [tab, setTab] = useState('overview');
  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.gameWash }}>
      <div className="mx-auto max-w-[1240px] px-8 py-6">
        <a href="/20" className="text-[12px] font-semibold text-white/40 hover:text-white">← library</a>

        {/* head */}
        <section className="relative mt-4 mb-5 overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <img src={game.wide} alt="" className="h-[240px] w-full object-cover" />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.97) 8%, rgba(19,15,10,.3) 60%)' }} />
          <div className="absolute bottom-5 left-7 right-7 flex items-end justify-between">
            <div>
              <div className="mb-2 flex gap-2 font-mono text-[9px] font-bold uppercase tracking-[0.14em]">
                <span className="rounded-lg bg-amber-300 px-2.5 py-1 text-[#130f0a]">{game.tier} verified</span>
                <span className="rounded-lg bg-black/50 px-2.5 py-1 text-white/75">{game.store}</span>
              </div>
              <h1 className="text-[30px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <button className="rounded-2xl bg-amber-300 px-8 py-3.5 text-[15px] font-extrabold text-[#130f0a] shadow-[0_12px_30px_rgba(252,211,77,.25)]">▶ Play</button>
          </div>
        </section>

        {/* tab pills */}
        <div className="mb-6 flex w-fit gap-1 rounded-2xl bg-white/[0.05] p-1.5">
          {['overview', 'achievements', 'runs', 'fixes'].map((t) => (
            <button key={t} onClick={() => setTab(t)} className={`rounded-xl px-4 py-2 text-[12.5px] font-bold capitalize transition ${tab === t ? 'bg-white/12 text-white' : 'text-white/45 hover:text-white/85'}`}>{t}</button>
          ))}
        </div>

        {tab === 'overview' && <Overview game={game} />}
        {tab === 'achievements' && <AchievementsTab game={game} />}
        {tab === 'runs' && <RunsTab game={game} />}
        {tab === 'fixes' && <FixesTab game={game} />}
      </div>
    </div>
  );
}

function Overview({ game }) {
  const ach = achievementsFor(game);
  return (
    <div className="grid grid-cols-[1.6fr_1fr] gap-5 pb-10">
      <div className="space-y-3">
        <div className="overflow-hidden rounded-3xl ring-1 ring-white/[0.08]">
          <img src={SCREENS(game)[0]} alt="" className="aspect-video w-full object-cover" />
        </div>
        <div className="flex gap-2.5">
          {SCREENS(game).slice(1, 4).map((s, i) => <img key={i} src={s} alt="" className="w-1/3 flex-none rounded-2xl object-cover aspect-video ring-1 ring-white/[0.08]" />)}
        </div>
      </div>
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
        <div className="rounded-3xl bg-white/[0.05] p-5 ring-1 ring-white/[0.07]">
          <div className="mb-2 flex items-baseline justify-between">
            <h3 className="text-[14px] font-bold">Achievements</h3>
            <span className="font-mono text-[10px] text-white/35">{ach.unlockedCount}/{ach.total}</span>
          </div>
          <div className="h-1.5 rounded-full bg-white/[0.08]"><div className="h-full rounded-full bg-amber-300" style={{ width: `${ach.score}%` }} /></div>
          <div className="mt-3.5 grid grid-cols-5 gap-2">
            {ach.list.slice(0, 5).map((a) => (
              <div key={a.id} className={`grid aspect-square place-items-center rounded-xl text-[14px] ${a.unlocked ? 'bg-amber-300/12 ring-1 ring-amber-300/40' : 'bg-white/[0.05] opacity-25'}`}>★</div>
            ))}
          </div>
        </div>
        <button className="w-full rounded-2xl border border-white/[0.1] bg-white/[0.06] px-5 py-3 text-[13px] font-bold text-white/80 transition hover:bg-white/[0.1]">⚙ Launcher settings</button>
      </aside>
    </div>
  );
}

function AchievementsTab({ game }) {
  const ach = achievementsFor(game);
  const [filter, setFilter] = useState('all');
  const list = ach.list.filter(a => filter === 'all' || (filter === 'unlocked' ? a.unlocked : !a.unlocked));
  return (
    <div className="pb-10">
      <div className="mb-5 flex items-center justify-between">
        <div className="flex items-center gap-5">
          <div className="grid h-20 w-20 place-items-center rounded-full border-[3px] border-amber-300/60 text-[19px] font-black text-amber-300" style={{ borderStyle: 'double' }}>{ach.score}%</div>
          <div>
            <div className="text-[16px] font-extrabold">{ach.unlockedCount} of {ach.total}</div>
            <div className="font-mono text-[10.5px] text-white/35">rarity-weighted · {game.title}</div>
          </div>
        </div>
        <div className="flex gap-1 rounded-xl bg-white/[0.05] p-1">
          {['all', 'unlocked', 'locked'].map((f) => (
            <button key={f} onClick={() => setFilter(f)} className={`rounded-lg px-3.5 py-1.5 text-[12px] font-bold capitalize ${filter === f ? 'bg-white/12 text-white' : 'text-white/45'}`}>{f}</button>
          ))}
        </div>
      </div>
      <div className="rounded-3xl bg-white/[0.04] p-2 ring-1 ring-white/[0.07]">
        {list.map((a) => (
          <div key={a.id} className={`flex items-center gap-4 border-b border-white/[0.04] px-4 py-3 last:border-0 ${a.unlocked ? '' : 'opacity-40'}`}>
            <div className={`grid h-11 w-11 flex-none place-items-center rounded-xl text-[16px] ${a.unlocked ? 'bg-amber-300/15 ring-1 ring-amber-300/40' : 'bg-white/[0.05]'}`}>★</div>
            <div className="min-w-0 flex-1">
              <div className="text-[13.5px] font-bold">{a.name}</div>
              <div className="text-[12px] text-white/45">{a.desc}</div>
            </div>
            <span className="font-mono text-[10.5px] text-white/40">{a.rarity}</span>
            <span className="w-20 text-right font-mono text-[10px] text-white/30">{a.unlocked ? a.date : '—'}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function RunsTab({ game }) {
  const rows = [
    ['today', '2h 18m', 'GE-Proton10-4', '4K · MangoHud', 'clean'],
    ['yesterday', '1h 46m', 'GE-Proton10-4', '4K · MangoHud', 'stutter 5 min'],
    ['3 days ago', '3h 02m', 'Proton Experimental', '1440p', 'crashed once'],
    ['last week', '52m', 'Proton Experimental', '1440p · FSR', 'clean'],
  ];
  return (
    <div className="pb-10">
      <div className="overflow-hidden rounded-3xl bg-white/[0.04] ring-1 ring-white/[0.07]">
        <div className="grid grid-cols-[8rem_7rem_1fr_1fr_8rem] gap-4 border-b border-white/[0.06] px-5 py-3 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">
          <span>when</span><span>length</span><span>runner</span><span>config</span><span className="text-right">result</span>
        </div>
        {rows.map((r, i) => (
          <div key={i} className="grid grid-cols-[8rem_7rem_1fr_1fr_8rem] items-center gap-4 border-b border-white/[0.04] px-5 py-3 text-[12.5px] last:border-0">
            <span className="font-mono text-[10.5px] text-white/35">{r[0]}</span>
            <span className="font-bold">{r[1]}</span>
            <span className="font-mono text-[11px] text-white/55">{r[2]}</span>
            <span className="font-mono text-[10.5px] text-white/35">{r[3]}</span>
            <span className={`text-right font-mono text-[10px] uppercase tracking-wider ${r[4] === 'clean' ? 'text-emerald-300' : 'text-orange-300'}`}>{r[4]}</span>
          </div>
        ))}
      </div>
      <p className="mt-4 font-mono text-[10.5px] text-white/30">run history is kept locally per game · 4 of 4 shown</p>
    </div>
  );
}

function FixesTab({ game }) {
  return (
    <div className="space-y-3 pb-10">
      <div className="rounded-3xl bg-amber-300/[0.07] p-6 ring-1 ring-amber-300/30">
        <div className="flex items-center justify-between">
          <h3 className="text-[15px] font-extrabold text-amber-300">EAC bridge re-bind required</h3>
          <button className="rounded-xl bg-amber-300 px-4 py-2 text-[12px] font-extrabold text-[#130f0a]">Apply fix</button>
        </div>
        <p className="mt-2 text-[13px] leading-relaxed text-white/60">Source: ProtonDB report #4471 · verified 10.1. Binding the bridge restores online matchmaking.</p>
      </div>
      <div className="rounded-3xl bg-white/[0.04] p-6 ring-1 ring-white/[0.07]">
        <h3 className="text-[14px] font-bold text-white/85">Shader warm-up (optional)</h3>
        <p className="mt-2 text-[12.5px] leading-relaxed text-white/50">First 5 minutes compile shaders; stutter after that indicates a cache miss. Disable MangoHud during warm-up.</p>
      </div>
    </div>
  );
}