import React, { useState } from 'react';
import { GAMES, achievementsFor } from '../data.js';
import { WX, Sidebar, ActivityPanel } from './shared16.jsx';

/* /library — the dedicated LIBRARY VIEW (referenced by /17–/21 navs):
   "Last played" as large cards with progress, "Recently added" small
   list, and /13's dense table as a toggleable secondary view with
   sort + filter. Shared across iterations. */
export function LibraryView1626() {
  const [view, setView] = useState('cozy'); // cozy | table
  const [q, setQ] = useState('');
  const [source, setSource] = useState('all');

  const filtered = GAMES.filter(g =>
    (source === 'all' || g.store.toLowerCase() === source) &&
    g.title.toLowerCase().includes(q.toLowerCase())
  );

  return (
    <div className="min-h-screen text-[#ece6da]" style={{ fontFamily: '"Hanken Grotesk", sans-serif', background: WX.wash }}>
      <div className="mx-auto flex max-w-[1600px]">
        <Sidebar active="Library" libraryUrl="/library" />
        <main className="flex-1 p-7">
          <header className="mb-6 flex items-end justify-between">
            <div>
              <h1 className="text-[23px] font-extrabold tracking-tight">Library</h1>
              <p className="mt-0.5 text-[12.5px] text-white/40">{filtered.length} of {GAMES.length} titles · all sources verified 09:41</p>
            </div>
            <div className="flex items-center gap-2.5">
              <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="filter…" className="w-48 rounded-xl bg-white/[0.06] px-3.5 py-2 font-mono text-[11.5px] text-white placeholder-white/30 focus:outline-none focus:ring-1 focus:ring-amber-300/50" />
              <div className="flex rounded-xl bg-white/[0.05] p-1">
                {['all', 'steam', 'gog', 'epic'].map((s) => (
                  <button key={s} onClick={() => setSource(s)} className={`rounded-lg px-3.5 py-1.5 font-mono text-[10px] uppercase tracking-wider transition ${source === s ? 'bg-white/12 text-white' : 'text-white/40 hover:text-white/80'}`}>{s}</button>
                ))}
              </div>
              <div className="flex rounded-xl bg-white/[0.05] p-1">
                {[['cozy', 'cards'], ['table', 'table']].map(([v, l]) => (
                  <button key={v} onClick={() => setView(v)} className={`rounded-lg px-3.5 py-1.5 text-[12px] font-bold transition ${view === v ? 'bg-white/12 text-white' : 'text-white/40 hover:text-white/80'}`}>{l}</button>
                ))}
              </div>
              <button className="rounded-xl bg-amber-300 px-4 py-2.5 text-[12.5px] font-extrabold text-[#130f0a]">＋ Add game</button>
            </div>
          </header>

          {view === 'cozy' ? (
            <>
              {/* last played — big cards with playtime bar */}
              <section className="mb-8">
                <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-white/60">Last played</h2>
                <div className="grid grid-cols-3 gap-4">
                  {filtered.slice(0, 3).map((g) => (
                    <a key={g.id} href="/17-game" className="group relative overflow-hidden rounded-3xl ring-1 ring-white/[0.08] transition hover:ring-amber-300/40">
                      <img src={g.wide} alt="" className="h-[210px] w-full object-cover transition duration-500 group-hover:scale-[1.04]" />
                      <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, rgba(19,15,10,.95) 8%, transparent 60%)' }} />
                      <div className="absolute bottom-4 left-5 right-5">
                        <div className="flex items-baseline justify-between">
                          <span className="truncate text-[16px] font-extrabold tracking-tight">{g.title.split(':')[0]}</span>
                          <span className={`font-mono text-[9px] uppercase tracking-wider ${g.tier === 'platinum' ? 'text-amber-300' : 'text-white/45'}`}>{g.tier}</span>
                        </div>
                        <div className="mt-1 flex items-center gap-2 font-mono text-[9px] text-white/45">
                          <span>{g.hours}h</span><span>·</span><span>{g.last}</span><span>·</span><span>{g.runner.split(' ')[0]}</span>
                        </div>
                        {/* playtime progress vs most-played */}
                        <div className="mt-2.5 h-1 rounded-full bg-white/[0.14]">
                          <div className="h-full rounded-full bg-amber-300/80" style={{ width: `${Math.min(100, (g.hours / 1340) * 100)}%` }} />
                        </div>
                      </div>
                      <span className="absolute right-4 top-4 grid h-9 w-9 place-items-center rounded-full bg-black/45 text-[12px] text-white opacity-0 backdrop-blur transition group-hover:opacity-100">▶</span>
                    </a>
                  ))}
                </div>
              </section>

              {/* everything else — medium covers grid */}
              <section className="mb-8">
                <h2 className="mb-3.5 text-[13px] font-bold uppercase tracking-[0.14em] text-white/60">All titles</h2>
                <div className="grid grid-cols-5 gap-3">
                  {filtered.slice(3).map((g) => (
                    <a key={g.id} href="/17-game" className="group overflow-hidden rounded-2xl bg-white/[0.04] ring-1 ring-white/[0.06] transition hover:-translate-y-0.5 hover:ring-amber-300/40">
                      <div className="relative">
                        <img src={g.cover} alt="" className="aspect-[2/3] w-full object-cover" />
                        {g.status === 'tweak' && <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,.8)]" />}
                      </div>
                      <div className="px-2.5 py-2">
                        <div className="truncate text-[11.5px] font-bold">{g.title.split(':')[0]}</div>
                        <div className="font-mono text-[8px] uppercase tracking-wider text-white/30">{g.hours}h · {g.last}</div>
                      </div>
                    </a>
                  ))}
                </div>
              </section>
            </>
          ) : (
            /* table view (13's) */
            <section className="mb-8 overflow-hidden rounded-2xl bg-black/25 ring-1 ring-white/[0.06]">
              <div className="grid grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem] gap-3 border-b border-white/[0.05] px-4 py-2 font-mono text-[9px] uppercase tracking-[0.14em] text-white/30">
                <span /><span>title</span><span>tier</span><span>runner</span><span className="text-right">hrs</span><span className="text-right">size</span><span className="text-right">last played</span>
              </div>
              {filtered.map((g, i) => (
                <a key={g.id} href="/17-game" className={`group grid grid-cols-[16px_minmax(180px,2fr)_5rem_7.5rem_4.5rem_4.5rem_7rem] items-center gap-3 px-4 py-2 ${i % 2 ? 'bg-white/[0.015]' : ''} hover:bg-amber-300/[0.05]`}>
                  <span className={`h-1.5 w-1.5 rounded-full ${g.tier === 'platinum' ? 'bg-amber-300' : g.tier === 'gold' ? 'bg-orange-400' : 'bg-white/15'}`} />
                  <span className="flex items-center gap-2.5">
                    <img src={g.cover} alt="" className="h-7 w-7 flex-none rounded object-cover ring-1 ring-white/10" />
                    <span className="truncate text-[12.5px] font-semibold group-hover:text-amber-200">{g.title}</span>
                  </span>
                  <span className="font-mono text-[10px] text-white/40">{g.tier}</span>
                  <span className="truncate font-mono text-[10px] text-white/30">{g.runner}</span>
                  <span className="text-right font-mono text-[10.5px] tabular-nums text-white/55">{g.hours}</span>
                  <span className="text-right font-mono text-[10px] text-white/30">{g.size.split(' ')[0]}</span>
                  <span className="text-right font-mono text-[10px] text-white/30">{g.last}</span>
                </a>
              ))}
            </section>
          )}

          {/* right-column extras when cozy */}
          {view === 'cozy' && (
            <section className="mb-10 grid grid-cols-[1.5fr_1fr] gap-5">
              <ActivityPanel limit={6} />
              <div className="rounded-3xl bg-white/[0.04] p-5 ring-1 ring-white/[0.07]">
                <h2 className="mb-3 text-[14px] font-bold">Recently added</h2>
                <ul className="divide-y divide-white/[0.05]">
                  {GAMES.slice(6, 12).map((g) => (
                    <li key={g.id} className="flex items-center gap-3 py-2">
                      <img src={g.cover} alt="" className="h-8 w-8 rounded object-cover ring-1 ring-white/10" />
                      <div className="min-w-0 flex-1 leading-tight">
                        <div className="truncate text-[12px] font-semibold">{g.title}</div>
                        <div className="font-mono text-[8.5px] uppercase tracking-wider text-white/30">added {g.last} · {g.store}</div>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            </section>
          )}
        </main>
      </div>
    </div>
  );
}