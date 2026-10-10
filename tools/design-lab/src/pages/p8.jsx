import React, { useState } from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';
import { ActivityFeed } from './p6.jsx';

/* ============================================================
   /8 — "Compact pro": dense 14px-base productivity layout.
   Neutral charcoal + single teal accent, sharp 6px corners,
   master-detail like a modern IDE. Game page = settings form
   left, live preview right. Achievements = completion table.
   ============================================================ */

const game = GAMES[11]; // Helldivers 2

function Side({ active }) {
  return (
    <aside className="flex h-screen w-52 flex-none flex-col border-r border-[#26282c] bg-[#141518]">
      <div className="flex items-center gap-2 px-4 py-4">
        <span className="h-6 w-6 rounded bg-[#2dd4bf]" />
        <span className="text-[14px] font-bold tracking-tight text-[#e9eaec]">platypus</span>
        <span className="ml-auto rounded bg-[#26282c] px-1.5 py-0.5 font-mono text-[9px] text-[#8b8f94]">⌘K</span>
      </div>
      <div className="px-4 pb-4">
        <div className="rounded-md bg-[#1c1e21] px-3 py-2 text-[12px] text-[#6f747a]">Search library…</div>
      </div>
      <nav className="text-[13px] text-[#a3a7ac]">
        <div className="px-4 pb-1.5 pt-2 font-mono text-[9px] uppercase tracking-[0.16em] text-[#5c6167]">library</div>
        {[['All games', '24'], ['Ready', '19'], ['Needs tweaks', '5'], ['Idle', '2']].map(([l, c]) => (
          <div key={l} className="flex items-center justify-between px-4 py-1.5 hover:bg-[#1c1e21]">
            <span>{l}</span><span className="font-mono text-[10px] text-[#5c6167]">{c}</span>
          </div>
        ))}
        <div className="px-4 pb-1.5 pt-4 font-mono text-[9px] uppercase tracking-[0.16em] text-[#5c6167]">system</div>
        <div className="flex items-center justify-between px-4 py-1.5">
          <span>Runners</span><span className="font-mono text-[10px] text-[#5c6167]">6</span>
        </div>
        <div className="px-4 py-1.5">Activity log</div>
      </nav>
      <div className="mt-auto border-t border-[#26282c] p-3 font-mono text-[10px] text-[#5c6167]">
        <div className="flex items-center gap-2">
          <span className="h-1.5 w-1.5 rounded-full bg-[#2dd4bf]" /> scan idle · 09:41
        </div>
      </div>
    </aside>
  );
}

export function ProDashboard() {
  const focus = GAMES[0];
  return (
    <div className="flex min-h-screen bg-[#191a1d] text-[#d7d9db]" style={{ fontFamily: '"Hanken Grotesk", sans-serif' }}>
      <Side active="home" />
      <div className="flex-1">
        <header className="flex h-12 items-center gap-4 border-b border-[#26282c] px-6">
          <span className="text-[13px] font-bold text-[#e9eaec]">Dashboard</span>
          <span className="text-[12px] text-[#6f747a]">week 41</span>
          <button className="ml-auto rounded-md bg-[#2dd4bf] px-3 py-1.5 text-[12px] font-bold text-[#0d1112]">Rescan</button>
        </header>

        <div className="space-y-6 p-6">
          {/* stat strip — flat boxes, no cards */}
          <section className="grid grid-cols-6 gap-px overflow-hidden rounded-md border border-[#26282c] bg-[#26282c]">
            {[
              ['titles', '24'], ['ready', '19'], ['tweaks', '5'], ['runners', '6'], ['this week', '14 sessions'], ['hours', '12.7'],
            ].map(([k, v]) => (
              <div key={k} className="bg-[#1c1e21] px-4 py-3.5">
                <div className="font-mono text-[9px] uppercase tracking-[0.14em] text-[#6f747a]">{k}</div>
                <div className="mt-1 text-[19px] font-bold tabular-nums text-[#e9eaec]">{v}</div>
              </div>
            ))}
          </section>

          <section className="grid grid-cols-[2fr_1fr] gap-6">
            {/* play histogram */}
            <div className="rounded-md border border-[#26282c] bg-[#1c1e21] p-5">
              <div className="mb-4 flex items-baseline justify-between">
                <h2 className="text-[13.5px] font-bold text-[#e9eaec]">Playtime — last 14 days</h2>
                <span className="font-mono text-[10px] text-[#6f747a]">peak 10h · avg 4.2h</span>
              </div>
              <div className="flex h-36 items-end gap-1.5">
                {HOUR_BARS.map((h, i) => (
                  <div key={i} className="group relative flex-1">
                    <div className={`w-full rounded-sm ${i === 13 ? 'bg-[#2dd4bf]' : 'bg-[#2a2d31]'} transition-all group-hover:bg-[#2dd4bf]/60`} style={{ height: h * 14 }} />
                  </div>
                ))}
              </div>
              <div className="mt-2 flex justify-between font-mono text-[9px] text-[#5c6167]"><span>21 SEP</span><span>5 OCT</span></div>
            </div>

            {/* session feed */}
            <div className="rounded-md border border-[#26282c] bg-[#1c1e21] p-5">
              <h2 className="mb-3 text-[13.5px] font-bold text-[#e9eaec]">Today</h2>
              <ActivityFeed limit={5} />
            </div>
          </section>

          {/* master-detail strip */}
          <section className="rounded-md border border-[#26282c] bg-[#1c1e21]">
            <div className="border-b border-[#26282c] px-5 py-3 text-[12px] font-bold text-[#e9eaec]">Library focus</div>
            {GAMES.slice(0, 6).map((g, i) => (
              <a key={g.id} href="/8/game" className={`grid grid-cols-[1fr_6rem_7rem_5rem_9rem] items-center gap-4 px-5 py-2.5 text-[13px] ${i === 0 ? 'bg-[#232529]' : 'hover:bg-[#202225]'}`}>
                <span className="truncate font-semibold text-[#e9eaec]">{g.title}</span>
                <span className="font-mono text-[10px] text-[#6f747a]">{g.store}</span>
                <span className="font-mono text-[10px] text-[#8b8f94]">{g.runner}</span>
                <span className={`font-mono text-[10px] ${g.tier === 'platinum' ? 'text-[#2dd4bf]' : g.tier === 'gold' ? 'text-[#fbbf24]' : 'text-[#8b8f94]'}`}>{g.tier}</span>
                <span className="text-right font-mono text-[10px] text-[#5c6167]">{g.hours}h · {g.last}</span>
              </a>
            ))}
          </section>
        </div>
      </div>
    </div>
  );
}

export function ProGame() {
  const { list, unlockedCount, total, score } = achievementsFor(game);
  const [tab, setTab] = useState('settings');
  return (
    <div className="flex min-h-screen bg-[#191a1d] text-[#d7d9db]" style={{ fontFamily: '"Hanken Grotesk", sans-serif' }}>
      <Side active="game" />
      <div className="flex-1">
        <header className="flex h-12 items-center gap-3 border-b border-[#26282c] px-6">
          <a href="/8" className="text-[13px] text-[#6f747a] hover:text-[#e9eaec]">← dashboard</a>
          <span className="text-[13px] text-[#3a3d41]">/</span>
          <span className="text-[13px] font-bold text-[#e9eaec]">{game.title}</span>
          <div className="ml-6 flex gap-1">
            {['overview', 'settings', 'achievements', 'activity'].map((t) => (
              <button key={t} onClick={() => setTab(t)} className={`rounded-md px-3 py-1.5 text-[12px] font-semibold capitalize ${tab === t ? 'bg-[#2a2d31] text-[#e9eaec]' : 'text-[#8b8f94] hover:text-[#d7d9db]'}`}>{t}</button>
            ))}
          </div>
          <button className="ml-auto rounded-md bg-[#2dd4bf] px-4 py-1.5 text-[12px] font-bold text-[#0d1112]">▶ Launch</button>
        </header>

        {/* settings tab */}
        {tab === 'settings' && (
          <div className="grid max-w-5xl grid-cols-[1fr_1fr] gap-6 p-6">
            <div className="space-y-5">
              <Field label="Runner" hint="compatibility tool used for launch">
                <select className="input">{RUNNERS.map((r) => <option key={r.name}>{r.name}</option>)}</select>
              </Field>
              <Field label="Gamescope resolution" hint="ignored when gamescope is off">
                <div className="flex gap-2">
                  <input className="input flex-1" defaultValue="2560" />
                  <input className="input flex-1" defaultValue="1440" />
                  <input className="input w-20" defaultValue="144" />
                </div>
              </Field>
              <Field label="Launch arguments" hint="appended verbatim after the executable">
                <input className="input font-mono" defaultValue="--use-vulkan --skip-intro" />
              </Field>
              <Field label="Environment" hint="KEY=value pairs">
                <div className="space-y-2">
                  {['PROTON_ENABLE_NVAPI=1', 'DXVK_STATE_CACHE_PATH=/tmp'].map((v) => (
                    <div key={v} className="flex gap-2"><input className="input flex-1 font-mono" defaultValue={v} /></div>
                  ))}
                  <button className="rounded border border-dashed border-[#3a3d41] px-3 py-1.5 text-[11px] text-[#8b8f94] hover:border-[#2dd4bf]">+ add variable</button>
                </div>
              </Field>
            </div>
            <div className="space-y-5">
              <div>
                <h3 className="mb-2 font-mono text-[9px] uppercase tracking-[0.16em] text-[#6f747a]">Switch stack</h3>
                <div className="space-y-2">
                  {[['Feral GameMode', true], ['MangoHud overlay', true], ['Gamescope', false], ['EAC bridge', true], ['Esync', false], ['Piped audio', true]].map(([label, on]) => (
                    <div key={String(label)} className="flex items-center justify-between rounded-md border border-[#26282c] bg-[#1c1e21] px-3.5 py-2.5">
                      <span className="text-[13px]">{label}</span>
                      <span className={`relative h-4 w-8 rounded-full ${on ? 'bg-[#2dd4bf]' : 'bg-[#3a3d41]'}`}>
                        <span className="absolute top-0.5 h-3 w-3 rounded-full bg-[#141518]" style={{ left: on ? 18 : 2 }} />
                      </span>
                    </div>
                  ))}
                </div>
              </div>
              {/* live pipeline preview */}
              <div>
                <h3 className="mb-2 font-mono text-[9px] uppercase tracking-[0.16em] text-[#6f747a]">Preview — updates live</h3>
                <div className="rounded-md border border-[#26282c] bg-[#141518] p-4 font-mono text-[11.5px] leading-relaxed">
                  <div className="text-[#6f747a]">$ platypus exec</div>
                  <div className="mt-1 text-[#2dd4bf]">gamemoderun mangohud \</div>
                  <div className="pl-4 text-[#2dd4bf]">GE-Proton10-4 run helldivers2.exe \</div>
                  <div className="pl-4 text-[#a3a7ac]">--use-vulkan --skip-intro</div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* achievements tab */}
        {tab === 'achievements' && (
          <div className="max-w-5xl p-6">
            <div className="mb-5 flex items-center gap-6">
              <div className="grid h-20 w-20 flex-none place-items-center rounded-full border-[3px] border-[#2dd4bf] text-[20px] font-black text-[#2dd4bf]" style={{ borderStyle: 'double' }}>{score}%</div>
              <div>
                <div className="text-[15px] font-bold text-[#e9eaec]">{unlockedCount} of ~{total} unlocked</div>
                <div className="mt-1 font-mono text-[11px] text-[#6f747a]">rarity-weighted score · {game.title}</div>
              </div>
            </div>
            <div className="overflow-hidden rounded-md border border-[#26282c]">
              <div className="grid grid-cols-[2rem_1fr_10rem_6rem] gap-4 border-b border-[#26282c] bg-[#1c1e21] px-4 py-2 font-mono text-[9px] uppercase tracking-widest text-[#6f747a]">
                <span>✓</span><span>achievement</span><span>rarity</span><span>date</span>
              </div>
              {list.map((a) => (
                <div key={a.id} className={`grid grid-cols-[2rem_1fr_10rem_6rem] items-center gap-4 px-4 py-2.5 text-[13px] ${a.unlocked ? '' : 'opacity-35'}`}>
                  <span className={a.unlocked ? 'text-[#2dd4bf]' : 'text-[#3a3d41]'}>{a.unlocked ? '●' : '○'}</span>
                  <div><span className="font-semibold text-[#e9eaec]">{a.name}</span> <span className="text-[12px] text-[#6f747a]">— {a.desc}</span></div>
                  <span className="font-mono text-[10px] text-[#8b8f94]">{a.rarity}</span>
                  <span className="font-mono text-[10px] text-[#5c6167]">{a.unlocked ? a.date : '—'}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* activity tab */}
        {tab === 'activity' && (
          <div className="max-w-3xl p-6">
            <div className="rounded-md border border-[#26282c] bg-[#1c1e21] p-5">
              <ActivityFeed limit={8} />
            </div>
          </div>
        )}

        {/* overview tab */}
        {tab === 'overview' && (
          <div className="grid max-w-5xl grid-cols-[1.3fr_1fr] gap-6 p-6">
            <div className="space-y-4">
              <img src={game.wide} alt="" className="aspect-video w-full rounded-md object-cover" />
              <div className="grid grid-cols-2 gap-3">
                {SCREENS(game, 2).map((s, i) => <img key={i} src={s} alt="" className="aspect-video w-full rounded-md object-cover" />)}
              </div>
            </div>
            <div className="rounded-md border border-[#26282c] bg-[#1c1e21] p-5">
              <h3 className="text-[13.5px] font-bold text-[#e9eaec]">Community verdict</h3>
              <p className="mt-2 text-[13px] leading-relaxed text-[#a3a7ac]">{game.verdict}</p>
              <div className="mt-5 space-y-2 font-mono text-[11px]">
                {[['tier', game.tier], ['reports', '1,284'], ['runner', game.runner], ['dx', game.dx]].map(([k, v]) => (
                  <div key={k} className="flex justify-between border-b border-[#26282c] py-1.5"><span className="text-[#6f747a]">{k}</span><span className="text-[#d7d9db]">{v}</span></div>
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({ label, hint, children }) {
  return (
    <div>
      <div className="mb-1.5 font-mono text-[9px] uppercase tracking-[0.16em] text-[#6f747a]">{label}</div>
      {children}
      <div className="mt-1 text-[11px] text-[#5c6167]">{hint}</div>
    </div>
  );
}