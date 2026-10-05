import React from 'react';
import { GAMES, RUNNERS, FIXES, ACTIVITY, PIPELINE, achievementsFor, SCREENS, HOUR_BARS } from '../data.js';
import { ActivityFeed } from './p6.jsx';
import { Rail } from './p6.jsx';

/* ============================================================
   /9 — "Curated catalog": light, editorial-meets-app, big type.
   Inspired by museum exhibition design: plenty of air, one
   saturated coral accent on bone. Game = magazine feature.
   ============================================================ */

const game = GAMES[2]; // BG3

const INK = '#201d17';
const PAPER = '#f3efe6';
const CORAL = '#e2543e';

export function EditorialDashboard() {
  return (
    <div className="min-h-screen" style={{ background: PAPER, color: INK, fontFamily: '"Bricolage Grotesque", sans-serif' }}>
      {/* masthead band */}
      <header className="border-b-2 px-10 pb-5 pt-7" style={{ borderColor: INK }}>
        <div className="mx-auto flex max-w-[1360px] items-baseline justify-between">
          <div className="flex items-baseline gap-3">
            <span className="font-mono text-[26px] font-black leading-none">🜋</span>
            <span className="text-[30px] font-extrabold leading-none tracking-[-0.02em]">hydra.</span>
          </div>
          <nav className="flex gap-8 text-[14px] font-semibold">
            <span className="border-b-2 pb-0.5" style={{ borderColor: CORAL, color: CORAL }}>Shelf</span>
            <span className="text-[#6e675a]">Workshop</span>
            <span className="text-[#6e675a]">Journal</span>
            <span className="text-[#6e675a]">Collection stats</span>
          </nav>
          <span className="font-mono text-[10px] uppercase tracking-[0.2em] text-[#6e675a]">tue · 05 oct</span>
        </div>
      </header>

      <main className="mx-auto max-w-[1360px] px-10">
        {/* opening statement + hero row */}
        <section className="grid grid-cols-[1.45fr_1fr] gap-12 border-b border-[#201d17]/15 py-9">
          <div>
            {/* oversized headline */}
            <h1 className="text-[64px] font-extrabold leading-[0.98] tracking-[-0.03em]">
              Three shelves,<br />one collection.
            </h1>
            <p className="mt-5 max-w-[52ch] text-[16px] leading-[1.6] text-[#423c31]">
              Steam, GOG and Epic now agree with each other. The collection numbers{' '}
              <span className="font-bold" style={{ color: CORAL }}>{GAMES.length} titles</span>, plays{' '}
              <span className="font-bold">3,146 hours</span>, and needs your attention in exactly{' '}
              <span className="font-bold">{GAMES.filter(g => g.status === 'tweak').length} places</span>.
              Everything else should simply launch when you double-click it.
            </p>

            {/* the three-shelf diagram — pure type + color blocks */}
            <div className="mt-9 space-y-2.5">
              {[
                ['Steam', 14, '#2a2620'],
                ['GOG', 5, '#5a5348'],
                ['Epic', 5, CORAL],
              ].map(([name, n, bg]) => (
                <div key={name} className="group flex cursor-default items-stretch overflow-hidden rounded-md">
                  <span className="grid w-24 place-items-center text-[13px] font-black text-[#f3efe6] transition-all group-hover:w-32" style={{ background: bg }}>{name}</span>
                  <span className="flex flex-1 items-center gap-4 bg-[#eae4d6] px-5">
                    <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-[#6e675a]">{n} titles</span>
                    <span className="flex flex-1 gap-1">
                      {GAMES.slice(0, 8).map((g, i) => (
                        <img key={i} src={g.cover} alt="" className="h-10 w-7 rounded-[3px] object-cover opacity-90 transition group-hover:opacity-100" />
                      ))}
                    </span>
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* right: poster + verdict */}
          <div>
            <div className="relative overflow-hidden rounded-md shadow-[0_16px_40px_rgba(32,29,23,.18)]">
              <img src={GAMES[0].cover} alt="" className="aspect-[3/4] w-full object-cover" />
              <span className="absolute left-4 top-4 rounded-full px-3 py-1 text-[11px] font-black uppercase tracking-wider text-white" style={{ background: CORAL }}>shelf pick</span>
            </div>
            <h3 className="mt-4 text-[22px] font-extrabold tracking-tight">{GAMES[0].title}</h3>
            <p className="mt-2 text-[13.5px] leading-relaxed text-[#423c31]">{GAMES[0].verdict}</p>
            <a href="/9/game" className="mt-3 inline-block border-b-2 pb-0.5 text-[13px] font-bold transition" style={{ borderColor: CORAL, color: CORAL }}>Read the workshop notes →</a>
          </div>
        </section>

        {/* ledger + activity in two columns like a paper */}
        <section className="grid grid-cols-[1.6fr_1fr] gap-12 py-9">
          <div>
            <h2 className="mb-5 text-[13px] font-black uppercase tracking-[0.2em]">The week in fixes</h2>
            {FIXES.slice(0, 5).map((f, i) => (
              <div key={f.game} className="group grid grid-cols-[3rem_1fr_auto] items-baseline gap-4 border-t border-[#201d17]/12 py-4 hover:bg-[#ece6d7]">
                <span className="font-mono text-[12px] text-[#a39c8c]">{String(i + 1).padStart(2, '0')}</span>
                <div>
                  <span className="text-[17px] font-bold tracking-tight group-hover:text-[#e2543e]">{f.game}</span>
                  <p className="mt-0.5 max-w-[58ch] text-[13px] leading-relaxed text-[#423c31]">{f.line}</p>
                </div>
                <span className="font-mono text-[10px] uppercase tracking-wider" style={{ color: f.tier === 'bronze' ? CORAL : '#6e675a' }}>{f.tier}</span>
              </div>
            ))}
          </div>

          {/* activity — compact, mono stamps */}
          <aside className="rounded-md p-6" style={{ background: '#2a2620', color: '#f3efe6' }}>
            <h2 className="mb-4 text-[13px] font-black uppercase tracking-[0.2em]">Session log</h2>
            <ul className="space-y-3.5">
              {ACTIVITY.slice(0, 6).map((a, i) => (
                <li key={i} className="flex gap-3 border-b border-white/10 pb-3 last:border-0">
                  <span className="font-mono text-[10px] text-white/40">{a.ts}</span>
                  <div className="min-w-0">
                    <div className="text-[13px] font-semibold leading-snug">{a.detail}</div>
                    <div className="font-mono text-[9.5px] uppercase tracking-wider text-white/35">{a.meta}</div>
                  </div>
                </li>
              ))}
            </ul>
          </aside>
        </section>
      </main>
    </div>
  );
}

export function EditorialGame() {
  const { list, unlockedCount, total, score } = achievementsFor(game);
  return (
    <div className="min-h-screen" style={{ background: PAPER, color: INK, fontFamily: '"Bricolage Grotesque", sans-serif' }}>
      {/* feature banner */}
      <header className="relative h-[380px] overflow-hidden bg-[#2a2620]">
        <img src={game.wide} alt="" className="absolute inset-0 h-full w-full object-cover opacity-80" />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(0deg, #f3efe6 2%, rgba(243,239,230,0) 40%)' }} />
        <div className="absolute inset-x-0 top-0 flex items-center justify-between px-10 py-5">
          <a href="/9" className="rounded-full bg-[#201d17] px-4 py-2 font-mono text-[11px] tracking-widest text-[#f3efe6]">← ALL SHELVES</a>
          <span className="rounded-full px-4 py-2 font-mono text-[11px] uppercase tracking-widest text-white" style={{ background: CORAL }}>{game.tier} on linux</span>
        </div>
        <div className="absolute bottom-8 left-10 right-10">
          <div className="font-mono text-[11px] uppercase tracking-[0.24em] text-[#201d17]/60">workshop report · {game.store} · {game.runner}</div>
          <h1 className="mt-2 max-w-[20ch] text-[56px] font-extrabold leading-[0.98] tracking-[-0.03em]">{game.title}</h1>
        </div>
      </header>

      <main className="mx-auto grid max-w-[1360px] grid-cols-[1.6fr_1fr] gap-12 px-10 py-10">
        {/* feature body */}
        <article className="space-y-10">
          <p className="text-[20px] font-semibold leading-[1.55] tracking-[-0.01em] first-letter:float-left first-letter:mr-2 first-letter:text-[54px] first-letter:font-black first-letter:leading-[0.85]" style={{ color: CORAL }}>
            {game.verdict}
          </p>

          {/* settings as a printed worksheet */}
          <section className="border-2" style={{ borderColor: INK }}>
            <div className="flex items-center justify-between border-b-2 px-5 py-3" style={{ borderColor: INK }}>
              <h2 className="text-[13px] font-black uppercase tracking-[0.2em]">Configuration worksheet</h2>
              <span className="font-mono text-[10px] text-[#6e675a]">form 12-b · rev 4</span>
            </div>
            <div className="divide-y divide-[#201d17]/12">
              {[
                ['Compatibility runner', game.runner, 'select'],
                ['DX translation', game.dx, 'select'],
                ['Wine prefix', '~/.hydra/prefixes/bg3', 'text'],
                ['Launch arguments', '--continue-launcher -skiplauncher', 'text'],
                ['Gamescope', 'off', 'toggle'],
                ['MangoHud', 'on', 'toggle'],
                ['GameMode', 'on', 'toggle'],
              ].map(([label, value, kind]) => (
                <div key={label} className="grid grid-cols-[200px_1fr_auto] items-center gap-4 px-5 py-3.5">
                  <span className="text-[13px] font-bold">{label}</span>
                  {kind === 'toggle' ? (
                    <span className="justify-self-end flex items-center gap-2 font-mono text-[11px] uppercase tracking-wider">
                      <span className={`rounded-full px-3 py-1 text-[10px] font-black ${value === 'on' ? 'text-[#f3efe6]' : 'text-[#6e675a]'}`} style={{ background: value === 'on' ? CORAL : '#d8d1c0' }}>{value}</span>
                    </span>
                  ) : (
                    <>
                      <span className="border-b border-dotted border-[#201d17]/40 pb-1 font-mono text-[13px]">{value}</span>
                      <span className="font-mono text-[9px] uppercase tracking-wider text-[#a39c8c]">edit</span>
                    </>
                  )}
                </div>
              ))}
            </div>
          </section>

          {/* gallery */}
          <section>
            <h2 className="mb-4 text-[13px] font-black uppercase tracking-[0.2em]">Captured in session</h2>
            <div className="grid grid-cols-3 gap-3">
              {SCREENS(game).map((s, i) => (
                <img key={i} src={s} alt="" className="aspect-[4/3] w-full rounded-md object-cover shadow-[0_8px_20px_rgba(32,29,23,.12)]" />
              ))}
            </div>
          </section>
        </article>

        {/* sidebar column */}
        <aside className="space-y-9">
          <div>
            <h2 className="mb-3 border-b-2 pb-2 text-[13px] font-black uppercase tracking-[0.2em]" style={{ borderColor: CORAL }}>Achievements</h2>
            <div className="flex items-baseline gap-1">
              <span className="text-[44px] font-extrabold leading-none tracking-tight">{score}%</span>
              <span className="font-mono text-[11px] text-[#6e675a]">{unlockedCount}/{total}</span>
            </div>
            <div className="mt-3 space-y-1.5">
              {list.slice(0, 6).map((a) => (
                <div key={a.id} className={`flex items-center justify-between gap-2 border-b border-dotted border-[#201d17]/25 py-1.5 text-[12.5px] ${a.unlocked ? '' : 'opacity-35'}`}>
                  <span className="truncate font-semibold">{a.name}</span>
                  <span className="font-mono text-[10px] text-[#6e675a]">{a.unlocked ? a.date : a.rarity}</span>
                </div>
              ))}
            </div>
          </div>

          <div>
            <h2 className="mb-3 border-b-2 pb-2 text-[13px] font-black uppercase tracking-[0.2em]" style={{ borderColor: INK }}>Recent activity</h2>
            <ul className="space-y-3">
              {ACTIVITY.slice(0, 5).map((a, i) => (
                <li key={i} className="border-b border-[#201d17]/10 pb-2.5">
                  <div className="text-[12.5px] font-semibold leading-snug">{a.detail}</div>
                  <div className="font-mono text-[9.5px] uppercase tracking-wider text-[#a39c8c]">{a.ts} · {a.meta}</div>
                </li>
              ))}
            </ul>
          </div>
        </aside>
      </main>
    </div>
  );
}