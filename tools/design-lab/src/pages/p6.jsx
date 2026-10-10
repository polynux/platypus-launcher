import React, { useEffect, useState } from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';

/* ============================================================
   /6 — "Cirno-style" cinematic store front: full-bleed hero,
   glass dock nav, rails; game page = media-first with tabs;
   achievements = rarity chips + progress; activity = feed.
   ============================================================ */

const hero = GAMES[7]; // Kingdom Come II

function DockIcon({ label, active, children }) {
  return (
    <button
      title={label}
      className={`grid h-10 w-10 place-items-center rounded-xl transition ${
        active ? 'bg-white/12 text-white' : 'text-white/40 hover:text-white/80'
      }`}
    >
      {children}
    </button>
  );
}

const Icon = ({ d }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" className="h-5 w-5">
    <path d={d} />
  </svg>
);
const D = {
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  play: 'M6 4l14 8-14 8z',
  trophy: 'M8 21h8M12 17v4M7 4h10v4a5 5 0 01-10 0zM4 5h3v3a3 3 0 01-3-3zM20 5h-3v3a3 3 0 003-3z',
  pulse: 'M3 12h4l2-7 4 14 2-7h6',
  download: 'M12 3v12M6 11l6 6 6-6M5 21h14',
};

export function CinematicDock() {
  return (
    <div className="min-h-screen bg-[#0a0b0d] text-white" style={{ fontFamily: 'Sora, sans-serif' }}>
      <Dashboard title="Good evening" />
    </div>
  );
}

export function CinematicDashboard() {
  return (
    <Shell view="home">
      <Dashboard />
    </Shell>
  );
}

function Shell({ view, game, children }) {
  return (
    <div className="relative mx-auto flex min-h-screen max-w-[1600px] bg-[#0a0b0d] text-white" style={{ fontFamily: 'Sora, sans-serif' }}>
      {/* glass dock */}
      <nav className="sticky top-0 flex h-screen w-16 flex-none flex-col items-center gap-2 py-5">
        <div className="mb-4 grid h-10 w-10 place-items-center rounded-2xl bg-white/10 text-[15px] font-black text-[#7dd3fc]">P</div>
        {[['home', D.grid], ['library', D.play], ['achievements', D.trophy], ['activity', D.pulse]].map(([k, d], i) => (
          <DockIcon key={k} label={k} active={(view === 'home' && i === 0) || (view === 'game' && i === 1)}>
            <Icon d={d} />
          </DockIcon>
        ))}
        <div className="mt-auto flex flex-col gap-2">
          <DockIcon label="downloads"><Icon d={D.download} /></DockIcon>
          <img src="https://picsum.photos/seed/hyd-avatar/80/80" className="h-8 w-8 rounded-full ring-1 ring-white/20" alt="" />
        </div>
      </nav>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function Dashboard() {
  return (
    <div>
      {/* full-bleed hero */}
      <section className="relative -ml-16 h-[440px] overflow-hidden">
        <img src={hero.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
        <div className="absolute -left-16 w-[120%] inset-0" style={{ background: 'linear-gradient(75deg, rgba(10,11,13,.96) 20%, rgba(10,11,13,.25) 55%, rgba(10,11,13,.9))' }} />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, #0a0b0d 3%, transparent 50%)' }} />
        <div className="relative flex h-full max-w-[46rem] flex-col justify-end px-10 pb-10">
          <div className="mb-3 flex gap-2 text-[10px] font-bold uppercase tracking-[0.16em] text-white/50">
            <span className="rounded bg-white/10 px-2 py-1 backdrop-blur">{hero.store}</span>
            <span className="rounded bg-white/10 px-2 py-1 backdrop-blur">{hero.tier} on linux</span>
          </div>
          <h1 className="text-[46px] font-extrabold leading-none tracking-tight">{hero.title}</h1>
          <p className="mt-3 max-w-[52ch] text-[14px] leading-relaxed text-white/60">{hero.verdict}</p>
          <div className="mt-6 flex items-center gap-3">
            <a href="/6/game" className="flex items-center gap-2 rounded-xl bg-white px-6 py-3 text-[14px] font-bold text-[#0a0b0d] transition hover:bg-white/85">
              <span>Open game page</span>
            </a>
            <button className="rounded-xl border border-white/15 px-5 py-3 text-[14px] font-semibold text-white/80 backdrop-blur transition hover:bg-white/10">View 12 fixes</button>
          </div>
        </div>
        <div className="absolute right-10 top-24 hidden grid-cols-3 gap-3 lg:grid">
          {GAMES.slice(3, 6).map((g) => (
            <img key={g.id} src={g.cover} alt="" className="h-28 w-20 rounded-lg object-cover ring-1 ring-white/15" />
          ))}
        </div>
      </section>

      <div className="px-10">
        {/* rails */}
        <Rail title="Continue playing" games={GAMES.slice(0, 6)} badge={(g) => `${g.hours}h`} />
        <Rail title="Fixes updated this week" games={GAMES.filter((g) => g.status === 'tweak')} badge={(g) => g.tier} small />
        <Rail title="Installed but untouched" games={GAMES.slice(10, 16)} badge={() => 'new'} small />

        {/* summary plates like GOG */}
        <section className="grid grid-cols-3 gap-4 pt-10">
          {[
            ['Sessions this week', '14', '+3 vs last week', 'text-[#7dd3fc]'],
            ['Achievement points', '1,240', '+45 this week', 'text-[#fbbf24]'],
            ['Hours on record', '3,146', '12h 41m today', 'text-[#34d399]'],
          ].map(([label, value, sub, cls]) => (
            <div key={label} className="rounded-2xl border border-white/[0.07] bg-white/[0.04] p-5">
              <div className="text-[12px] font-semibold text-white/40">{label}</div>
              <div className={`mt-1 text-[30px] font-extrabold tabular-nums ${cls}`}>{value}</div>
              <div className="mt-1 flex h-8 items-end gap-1">
                {HOUR_BARS.slice(0, 14).map((h, i) => (
                  <span key={i} className="w-2 rounded-sm bg-white/12" style={{ height: `${(h / 10) * 100}%`, opacity: i === 13 ? 1 : 0.5 }} />
                ))}
              </div>
            </div>
          ))}
        </section>

        {/* activity */}
        <section className="pb-16 pt-10">
          <h2 className="mb-4 text-[17px] font-bold tracking-tight">Recent activity</h2>
          <div className="grid grid-cols-[1.7fr_1fr] gap-8">
            <div className="rounded-2xl border border-white/[0.07] bg-white/[0.03] p-5">
              <ActivityFeed limit={6} />
            </div>
            <div className="space-y-3">
              {FIXES.slice(0, 3).map((f) => (
                <div key={f.game} className="rounded-xl border border-[#7dd3fc]/15 bg-[#7dd3fc]/[0.05] p-4 text-[13px]">
                  <div className="font-bold">{f.game}</div>
                  <div className="mt-1 text-[12px] text-white/50">{f.line}</div>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}

export function Rail({ title, games, badge, small }) {
  return (
    <section className="pt-8">
      <div className="mb-3 flex items-baseline justify-between">
        <h2 className="text-[17px] font-bold tracking-tight">{title}</h2>
        <span className="text-[12px] text-white/35">see all →</span>
      </div>
      <div className="flex gap-3 overflow-x-auto pb-2" style={{ scrollbarWidth: 'none' }}>
        {games.map((g) => (
          <a key={g.id} href="/6/game" className={`${small ? 'w-[136px]' : 'w-[180px]'} group flex-none`}>
            <div className="relative rounded-xl ring-1 ring-white/[0.08] transition duration-300 group-hover:ring-white/35">
              <img src={small ? g.wide : g.cover} alt="" className={`${small ? 'aspect-[2.2/1]' : 'aspect-[2/3]'} w-full rounded-xl object-cover`} />
              <div className="absolute bottom-2 left-2 rounded bg-black/60 px-1.5 py-0.5 font-mono text-[9px] uppercase text-white/80 backdrop-blur">{badge(g)}</div>
            </div>
            {!small && <div className="mt-2 truncate text-[12.5px] font-semibold">{g.title}</div>}
          </a>
        ))}
      </div>
    </section>
  );
}

export function ActivityFeed({ limit = 5 }) {
  const color = { session: 'text-[#34d399]', unlock: 'text-[#fbbf24]', fix: 'text-[#7dd3fc]', update: 'text-white/60', download: 'text-[#a78bfa]', add: 'text-white/60', tweak: 'text-[#f97316]' };
  const label = { session: 'PLAYED', unlock: 'UNLOCK', fix: 'FIX', update: 'UPDATE', download: 'DOWNLOAD', add: 'ADD', tweak: 'TWEAK' };
  return (
    <ul className="divide-y divide-white/[0.06]">
      {ACTIVITY.slice(0, limit).map((a, i) => (
        <li key={i} className="flex items-start gap-4 py-3">
          <span className="pt-0.5 font-mono text-[10px] text-white/30 tabular-nums">{a.ts}</span>
          <span className={`mt-1 h-1.5 w-1.5 flex-none rounded-full bg-current ${color[a.type]}`} />
          <div className="min-w-0 flex-1">
            <div className="text-[13px]"><span className={`font-bold uppercase tracking-wide text-[10px] ${color[a.type]}`}>{label[a.type]}</span> <span className="text-white/80">{a.detail}</span></div>
            <div className="font-mono text-[10px] text-white/35">{a.meta}</div>
          </div>
        </li>
      ))}
    </ul>
  );
}

export function CinematicGame() {
  const game = hero;
  const { list, unlockedCount, total, score } = achievementsFor(game);
  const tabs = ['Overview', 'Fixes & pipeline', 'Achievements', 'Activity'];
  return (
    <Shell view="game">
      {/* banner */}
      <section className="relative -ml-16 h-[340px] overflow-hidden">
        <img src={game.wide} alt="" className="absolute inset-0 h-full w-full object-cover" />
        <div className="absolute -left-16 w-[120%] inset-0" style={{ background: 'linear-gradient(0deg, #0a0b0d 8%, rgba(10,11,13,.4) 60%, rgba(10,11,13,.75))' }} />
        <div className="relative flex h-full flex-col justify-end px-10 pb-6">
          <div className="flex items-end justify-between">
            <div>
              <div className="font-mono text-[10px] uppercase tracking-[0.2em] text-white/50">{game.store} · {game.runner} · {game.size}</div>
              <h1 className="mt-1 text-[40px] font-extrabold leading-none tracking-tight">{game.title}</h1>
            </div>
            <div className="flex gap-3">
              <button className="flex items-center gap-2 rounded-xl bg-white px-7 py-3 text-[14px] font-bold text-[#0a0b0d]">Play</button>
              <button className="rounded-xl border border-white/15 px-5 py-3 text-[14px] font-semibold text-white/80">⚙ Configure</button>
            </div>
          </div>
          {/* tab bar floating on the fold */}
          <div className="mt-5 flex gap-1 rounded-xl border border-white/[0.07] bg-white/[0.05] p-1 backdrop-blur">
            {tabs.map((t, i) => (
              <button key={t} className={`flex-1 rounded-lg py-2 text-[13px] font-semibold ${i === 0 ? 'bg-white/12 text-white' : 'text-white/45 hover:text-white/80'}`}>{t}</button>
            ))}
          </div>
        </div>
      </section>

      <div className="grid grid-cols-[1.4fr_1fr] gap-8 px-10 py-8">
        {/* left: overview + pipeline */}
        <div className="space-y-8">
          {/* pipeline config — embedded */}
          <section>
            <h2 className="mb-3 text-[15px] font-bold tracking-tight">Launch pipeline</h2>
            <div className="rounded-2xl border border-white/[0.07] bg-white/[0.03]">
              {PIPELINE.map((s, i) => (
                <div key={i} className="flex items-center gap-4 border-b border-white/[0.05] px-5 py-3 last:border-0">
                  <span className="font-mono text-[10px] text-white/30">{String(i + 1).padStart(2, '0')}</span>
                  <span className="flex-1 truncate font-mono text-[12px] text-[#7dd3fc]">{s.chain}</span>
                  <span className="truncate font-mono text-[10px] text-white/35">{s.env}</span>
                  <button className="rounded-lg bg-white/5 px-2.5 py-1 text-[10px] font-bold text-white/60 hover:bg-white/10">toggle</button>
                </div>
              ))}
            </div>
          </section>

          {/* achievements preview */}
          <section>
            <div className="mb-3 flex items-baseline justify-between">
              <h2 className="text-[15px] font-bold tracking-tight">Achievements</h2>
              <span className="font-mono text-[11px] text-white/40">{unlockedCount}/{total} · {score}% score</span>
            </div>
            <div className="rounded-2xl border border-white/[0.07] bg-white/[0.03] p-5">
              <div className="h-1.5 rounded-full bg-white/8">
                <div className="h-full rounded-full bg-[#fbbf24]" style={{ width: '58%' }} />
              </div>
              <div className="mt-4 grid grid-cols-6 gap-3">
                {list.slice(0, 6).map((a) => (
                  <div key={a.id} className={`text-center ${a.unlocked ? '' : 'opacity-25 grayscale'}`}>
                    <div className={`mx-auto grid h-14 w-14 place-items-center rounded-full border-2 ${a.unlocked ? 'border-[#fbbf24] bg-[#fbbf24]/10' : 'border-white/20'}`}>★</div>
                    <div className="mt-1.5 truncate text-[10.5px] font-semibold">{a.name}</div>
                  </div>
                ))}
              </div>
            </div>
          </section>
        </div>

        {/* right: facts */}
        <aside className="space-y-4">
          <div className="rounded-2xl border border-white/[0.07] bg-white/[0.03] p-5 font-mono text-[12px]">
            <h3 className="mb-3 font-sans text-[13px] font-bold">Details</h3>
            {[
              ['Runner', game.runner], ['DX layer', game.dx], ['Prefix', '~/.platypus/pfx/kcd2'],
              ['Last played', game.last], ['Installed', '28 Feb'], ['Size', game.size],
            ].map(([k, v]) => (
              <div key={k} className="flex justify-between border-b border-white/[0.05] py-2 last:border-0">
                <span className="text-white/35">{k}</span><span className="text-white/85">{v}</span>
              </div>
            ))}
          </div>
          <div className="rounded-2xl border border-[#fbbf24]/20 bg-[#fbbf24]/[0.06] p-5">
            <h3 className="text-[13px] font-bold text-[#fbbf24]">Community verdict</h3>
            <p className="mt-2 text-[12.5px] leading-relaxed text-white/70">{game.verdict}</p>
            <div className="mt-3 font-mono text-[10px] text-white/40">ProtonDB · {game.tier} · 921 reports</div>
          </div>
        </aside>
      </div>
    </Shell>
  );
}