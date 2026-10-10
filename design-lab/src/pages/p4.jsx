import React, { useState } from 'react';
import { GAMES, RUNNERS, PIPELINE } from '../data.js';

const game = GAMES[0]; // Cyberpunk

export function WorkshopBench() {
  const [runner, setRunner] = useState(game.runner);
  const [dxvk, setDxvk] = useState('DXVK 2.5.1');
  const [opts, setOpts] = useState({ gamemode: true, mangohud: true, gamescope: false, eac: true, esync: true, fuzzymatch: false });

  const Toggle = ({ k, label, hint }) => (
    <button
      onClick={() => setOpts((o) => ({ ...o, [k]: !o[k] }))}
      className="flex w-full items-center justify-between gap-4 rounded-lg border border-stone-200 bg-white px-4 py-3 text-left transition hover:border-stone-300"
    >
      <span>
        <span className="block text-[13.5px] font-semibold text-stone-800">{label}</span>
        <span className="block text-[11.5px] text-stone-500">{hint}</span>
      </span>
      <span className={`relative h-5 w-9 flex-none rounded-full transition ${opts[k] ? 'bg-emerald-500' : 'bg-stone-300'}`}>
        <span className="absolute top-0.5 h-4 w-4 rounded-full bg-white shadow transition-all" style={{ left: opts[k] ? 18 : 2 }} />
      </span>
    </button>
  );

  return (
    <div className="min-h-screen bg-[#f6f6f4] text-stone-800" style={{ fontFamily: 'Sora, sans-serif' }}>
      {/* top bar */}
      <header className="border-b border-stone-200 bg-white">
        <div className="mx-auto flex max-w-[1200px] items-center gap-3 px-8 py-3.5">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-emerald-500 text-[14px] font-black text-white">P</span>
          <span className="text-[15px] font-bold">Platypus</span>
          <span className="mx-3 h-4 w-px bg-stone-200" />
          <span className="font-mono text-[11px] text-stone-400">workshop / cyberpunk-2077</span>
          <div className="ml-auto flex items-center gap-2">
            <button className="rounded-lg border border-stone-200 px-3.5 py-2 text-[12.5px] font-semibold text-stone-600 hover:bg-stone-50">Library</button>
            <button className="rounded-lg bg-stone-900 px-4 py-2 text-[12.5px] font-semibold text-white hover:bg-stone-700">Save changes</button>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-[1200px] grid-cols-[1fr_20rem] gap-8 px-8 py-8">
        {/* left column */}
        <div className="space-y-8">
          {/* game head */}
          <section className="flex items-center gap-5">
            <img src={game.cover} alt="" className="h-28 w-20 rounded-xl object-cover shadow-sm" />
            <div>
              <div className="mb-1 flex items-center gap-2">
                <span className="rounded-md bg-emerald-100 px-2 py-0.5 font-mono text-[10px] font-bold text-emerald-700">VERIFIED PLAYABLE</span>
                <span className="font-mono text-[10px] text-stone-400">{game.store} · {game.size} · last session {game.last}</span>
              </div>
              <h1 className="text-[28px] font-extrabold tracking-tight">{game.title}</h1>
              <p className="mt-1 max-w-[58ch] text-[13px] leading-relaxed text-stone-500">{game.verdict}</p>
            </div>
          </section>

          {/* runner pick — segmented cards */}
          <section>
            <h2 className="mb-3 text-[11px] font-bold uppercase tracking-[0.14em] text-stone-400">Compatibility runner</h2>
            <div className="grid grid-cols-3 gap-2.5">
              {RUNNERS.slice(0, 3).map((r) => (
                <button
                  key={r.name}
                  onClick={() => setRunner(r.name)}
                  className={`rounded-xl border p-3.5 text-left transition ${runner === r.name ? 'border-emerald-500 bg-emerald-50/60 ring-1 ring-emerald-500' : 'border-stone-200 bg-white hover:border-stone-300'}`}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-[13px] font-bold">{r.name}</span>
                    {runner === r.name && <span className="h-2 w-2 rounded-full bg-emerald-500" />}
                  </div>
                  <div className="mt-1 font-mono text-[10px] text-stone-400">{r.kind} · {r.size}</div>
                  <p className="mt-2 text-[11px] leading-snug text-stone-500 line-clamp-2">{r.note}</p>
                </button>
              ))}
            </div>
          </section>

          {/* two-up: renderer + toggles */}
          <section className="grid grid-cols-2 gap-8">
            <div>
              <h2 className="mb-3 text-[11px] font-bold uppercase tracking-[0.14em] text-stone-400">Translation layer</h2>
              <div className="space-y-2">
                {['DXVK 2.5.1', 'DXVK 2.4', 'VKD3D 2.14', 'GDI (raw)'].map((d) => (
                  <button
                    key={d}
                    onClick={() => setDxvk(d)}
                    className={`flex w-full items-center justify-between rounded-lg border px-4 py-2.5 text-[13px] font-semibold transition ${dxvk === d ? 'border-stone-800 bg-stone-900 text-white' : 'border-stone-200 bg-white hover:border-stone-300'}`}
                  >
                    {d}
                    {dxvk === d && <span className="font-mono text-[10px] font-normal opacity-60">active</span>}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <h2 className="mb-3 text-[11px] font-bold uppercase tracking-[0.14em] text-stone-400">Session options</h2>
              <div className="space-y-2">
                <Toggle k="gamemode" label="Feral GameMode" hint="Request CPU/GPU performance governor" />
                <Toggle k="mangohud" label="MangoHud overlay" hint="FPS, frametime, VRAM on-screen" />
                <Toggle k="gamescope" label="Gamescope" hint="Isolated compositor, 2560×1440 @144" />
                <Toggle k="eac" label="EasyAntiCheat bridge" hint="Required for online sessions" />
              </div>
            </div>
          </section>

          {/* pipeline receipt */}
          <section>
            <h2 className="mb-3 text-[11px] font-bold uppercase tracking-[0.14em] text-stone-400">Launch pipeline — as constructed</h2>
            <div className="overflow-hidden rounded-xl border border-stone-200 bg-white">
              {PIPELINE.map((step, i) => (
                <div key={i} className="grid grid-cols-[2.5rem_1fr_16rem] items-center gap-4 border-b border-stone-100 px-4 py-2.5 last:border-0">
                  <span className="font-mono text-[10px] text-stone-400">{String(i + 1).padStart(2, '0')}</span>
                  <span className="truncate font-mono text-[12px] text-stone-700">$ {step.chain}</span>
                  <span className="truncate text-right font-mono text-[10.5px] text-stone-400">{step.env}</span>
                </div>
              ))}
            </div>
          </section>
        </div>

        {/* right rail — summary card */}
        <aside className="h-fit rounded-2xl border border-stone-200 bg-white p-5 shadow-sm">
          <h3 className="text-[13px] font-bold">Session summary</h3>
          <dl className="mt-4 space-y-2.5 text-[12.5px]">
            {[
              ['Runner', runner],
              ['Renderer', dxvk],
              ['Prefix', '~/.platypus/prefixes/cyber-2077'],
              ['Display', opts.gamescope ? 'gamescope 2560×1440' : 'native Wayland'],
              ['Overlay', opts.mangohud ? 'MangoHud preset 4' : 'off'],
              ['Anti-cheat', opts.eac ? 'EAC bridge bound' : 'off'],
            ].map(([k, v]) => (
              <div key={k} className="flex justify-between gap-3 border-b border-stone-100 pb-2 last:border-0">
                <dt className="text-stone-400">{k}</dt>
                <dd className="truncate text-right font-medium">{v}</dd>
              </div>
            ))}
          </dl>
          <button className="mt-5 w-full rounded-xl bg-emerald-500 py-3 text-[14px] font-bold text-white transition hover:bg-emerald-600">
            Launch configured
          </button>
          <p className="mt-3 text-center text-[10.5px] text-stone-400">Command copied to clipboard on launch</p>
        </aside>
      </main>
    </div>
  );
}