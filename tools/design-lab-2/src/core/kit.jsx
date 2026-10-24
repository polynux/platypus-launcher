// Shared, unstyled-by-default building blocks. Each design scopes their look
// under its own root class; the markup and behavior stay identical.
import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Check, ChevronDown, CornerDownLeft, HardDrive, Plus, X, Cloud, Monitor, ArrowRight, Package, SlidersHorizontal, Cpu, Gauge, MonitorPlay, Layers, Wrench, Braces, TerminalSquare, AlertTriangle, ArrowDownToLine, Search, Sparkles, FolderOpen, RotateCcw, Play as PlayIcon } from 'lucide-react';
import { STORE_PATHS } from './storePaths.js';
import { STORES, GRAPHICS, SYNC_PRIMITIVES, GAMES, RECOMMEND } from './data.js';
import { runnerOf, ago, clock, dayLabel, dur, score, activeCopy, isInstalled, artOf, STATE_FILTERS, stateMatch, VIEWS, isDownloading } from './launcher.js';

export function StoreGlyph({ store, size = 12, title }) {
  const label = title || STORES[store]?.name;
  if (store === 'local') return <HardDrive size={size} strokeWidth={1.75} aria-label={label} className="store-glyph" />;
  if (store === 'amazon') return <Package size={size} strokeWidth={1.75} aria-label={label} className="store-glyph" />;
  return (
    <svg className="store-glyph" width={size} height={size} viewBox="0 0 24 24" role="img" aria-label={label}>
      <title>{label}</title>
      <path d={STORE_PATHS[store]} fill="currentColor" />
    </svg>
  );
}

export const SYNC_LABEL = {
  synced: 'Saves in sync',
  syncing: 'Uploading save',
  conflict: 'Save conflict',
  local: 'Local saves only',
  off: 'Cloud saves off',
};

export const TIER_LABEL = { platinum: 'Platinum', gold: 'Gold', silver: 'Silver', bronze: 'Bronze', borked: 'Borked', native: 'Native' };

export function SyncDot({ state }) {
  return <span className={`sync-dot sync-${state}`} role="img" aria-label={SYNC_LABEL[state]} title={SYNC_LABEL[state]} />;
}

// A listbox select, keyboard complete (arrows, Home/End, type-ahead, Esc).
export function Select({ value, options, onChange, label, disabled, className = '' }) {
  const [open, setOpen] = useState(false);
  const [hi, setHi] = useState(0);
  const btn = useRef();
  const list = useRef();
  const opts = options.map((o) => (typeof o === 'string' ? { value: o, label: o } : o));
  const cur = opts.find((o) => o.value === value);
  useEffect(() => {
    if (!open) return;
    setHi(Math.max(0, opts.findIndex((o) => o.value === value)));
    list.current?.focus();
    const close = (e) => { if (!btn.current?.parentElement.contains(e.target)) setOpen(false); };
    window.addEventListener('pointerdown', close);
    return () => window.removeEventListener('pointerdown', close);
  }, [open]);
  const pick = (o) => { onChange(o.value); setOpen(false); btn.current?.focus(); };
  const onKey = (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setHi((h) => Math.min(opts.length - 1, h + 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setHi((h) => Math.max(0, h - 1)); }
    else if (e.key === 'Home') setHi(0);
    else if (e.key === 'End') setHi(opts.length - 1);
    else if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); pick(opts[hi]); }
    else if (e.key === 'Escape' || e.key === 'Tab') { e.stopPropagation(); setOpen(false); btn.current?.focus(); }
    else if (e.key.length === 1) {
      const i = opts.findIndex((o) => o.label.toLowerCase().startsWith(e.key.toLowerCase()));
      if (i >= 0) setHi(i);
    }
  };
  return (
    <div className={`sel ${open ? 'is-open' : ''} ${className}`}>
      <button ref={btn} type="button" className="sel-btn" aria-haspopup="listbox" aria-expanded={open} aria-label={label} disabled={disabled} onClick={() => setOpen((o) => !o)}>
        <span className="sel-val">{cur?.label ?? '—'}</span>
        {cur?.hint && <span className="sel-hint">{cur.hint}</span>}
        <ChevronDown size={13} strokeWidth={1.75} className="sel-chev" />
      </button>
      {open && (
        <ul ref={list} className="sel-list" role="listbox" tabIndex={-1} aria-label={label} onKeyDown={onKey} aria-activedescendant={`opt-${hi}`}>
          {opts.map((o, i) => (
            <li key={o.value} id={`opt-${i}`} role="option" aria-selected={o.value === value} className={`sel-opt ${i === hi ? 'is-hi' : ''}`} onPointerEnter={() => setHi(i)} onClick={() => pick(o)}>
              <Check size={12} className="sel-check" style={{ opacity: o.value === value ? 1 : 0 }} />
              <span className="sel-opt-label">{o.label}</span>
              {o.hint && <span className="sel-opt-hint">{o.hint}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function Toggle({ checked, onChange, label, hint, disabled }) {
  return (
    <button type="button" role="switch" aria-checked={checked} disabled={disabled} className={`tgl ${checked ? 'is-on' : ''}`} onClick={onChange}>
      <span className="tgl-track" aria-hidden><span className="tgl-thumb" /></span>
      <span className="tgl-label">{label}</span>
      {hint && <span className="tgl-hint">{hint}</span>}
    </button>
  );
}

export function RecipeToggles({ L }) {
  const g = L.game;
  const native = g.runner === 'native';
  return (
    <div className="recipes" role="group" aria-label="Fix recipes">
      {L.recipes.map((r) => {
        const on = g.recipes.includes(r.id);
        return (
          <button key={r.id} type="button" className={`recipe ${on ? 'is-on' : ''}`} aria-pressed={on} disabled={native} title={r.desc} onClick={() => L.actions.toggleRecipe(g.id, r.id)}>
            <span className="recipe-led" aria-hidden />
            <span className="recipe-name">{r.label}</span>
            <span className="recipe-kind">{r.kind}</span>
          </button>
        );
      })}
    </div>
  );
}

export function EnvTable({ L }) {
  const g = L.game;
  return (
    <div className="env">
      <table className="env-table">
        <thead className="sr-only"><tr><th>Variable</th><th>Value</th><th>Remove</th></tr></thead>
        <tbody>
          {g.env.map(([k, v], i) => (
            <tr key={i}>
              <td><input className="env-key" value={k} spellCheck={false} aria-label="Variable name" onChange={(e) => L.actions.setEnv(g.id, i, e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, '_'), v)} /></td>
              <td><input className="env-val" value={v} spellCheck={false} aria-label={`Value of ${k}`} placeholder="empty" onChange={(e) => L.actions.setEnv(g.id, i, k, e.target.value)} /></td>
              <td className="env-x"><button type="button" aria-label={`Remove ${k}`} onClick={() => L.actions.removeEnv(g.id, i)}><X size={12} /></button></td>
            </tr>
          ))}
          {g.env.length === 0 && (
            <tr><td colSpan={3} className="env-empty">No variables. The runner’s defaults apply.</td></tr>
          )}
        </tbody>
      </table>
      <button type="button" className="env-add" onClick={() => L.actions.addEnv(g.id)}><Plus size={12} /> Add variable</button>
    </div>
  );
}

// The launch command, with the tokens touched by the last edit lit up.
export function LaunchLine({ L, className = '' }) {
  const [flash, setFlash] = useState(null);
  const first = useRef(true);
  useEffect(() => {
    if (first.current) { first.current = false; return; }
    setFlash(L.lastChange.key);
    const t = setTimeout(() => setFlash(null), 1600);
    return () => clearTimeout(t);
  }, [L.lastChange.n]);
  const hit = (k) => flash && (k === flash || (flash.startsWith('env') && k.startsWith('env:') && (flash === 'env' || k === flash)) || (flash === 'fsr' && k === 'gamescope') || (flash === 'graphics' && k === 'graphics'));
  return (
    <code className={`launch ${className}`} aria-label="Launch command">
      {L.tokens.map((t, i) => (
        <span key={t.key + i} className={`tok tok-${t.kind || 'wrap'} ${hit(t.key) ? 'is-changed' : ''}`}>{t.text}</span>
      ))}
    </code>
  );
}

export const runnerOptions = (runners, native) => [
  ...(native ? [{ value: 'native', label: 'Native Linux build', hint: 'no runner' }] : []),
  ...runners.map((r) => ({ value: r.id, label: r.name, hint: r.family })),
];

// The common property sheet. Designs decide headings via `Section`.
export function InspectorFields({ L, Section = DefaultSection, level = 'all' }) {
  const g = L.game;
  const hasNative = g.tier === 'native' || g.runner === 'native';
  const native = g.runner === 'native';
  const basic = level !== 'advanced';
  const adv = level !== 'basic';
  return (
    <>
      {basic && (
      <Section id="runner" title="Runner">
        <Row label="Compatibility">
          <Select label="Runner" value={g.runner} options={runnerOptions(L.runners, hasNative)} onChange={(v) => L.actions.setRunner(g.id, v)} />
        </Row>
        {level === 'basic' && native && <p className="field-note">This game runs its Linux build directly.</p>}
      </Section>
      )}
      {adv && (
      <Section id="layers" title="Translation layers">
        <Row label="Graphics layer">
          <Select label="Graphics layer" value={native ? '—' : g.graphics} disabled={native} options={native ? ['—'] : GRAPHICS} onChange={(v) => L.actions.setOpt(g.id, 'graphics', v)} />
        </Row>
        <Row label="Sync primitive">
          <Select label="Sync primitive" value={native ? '—' : g.syncPrim} disabled={native} options={native ? ['—'] : SYNC_PRIMITIVES} onChange={(v) => L.actions.setOpt(g.id, 'syncPrim', v)} />
        </Row>
        {native && <p className="field-note">This game runs its Linux build directly. Runner settings don’t apply.</p>}
      </Section>
      )}
      {basic && (
      <Section id="wrappers" title="Wrappers">
        <div className="toggles">
          <Toggle label="GameMode" hint="gamemoderun" checked={g.gamemode} onChange={() => L.actions.toggleOpt(g.id, 'gamemode')} />
          <Toggle label="MangoHud" hint="overlay" checked={g.mangohud} onChange={() => L.actions.toggleOpt(g.id, 'mangohud')} />
          <Toggle label="Gamescope" hint="2560×1440 · 165 Hz" checked={g.gamescope} onChange={() => L.actions.toggleOpt(g.id, 'gamescope')} />
          <Toggle label="FSR upscaling" hint="needs Gamescope" disabled={!g.gamescope} checked={g.fsr && g.gamescope} onChange={() => L.actions.toggleOpt(g.id, 'fsr')} />
        </div>
      </Section>
      )}
      {adv && <>
      <Section id="recipes" title="Fix recipes" aside={native ? 'n/a' : `${g.recipes.length} on`}>
        <RecipeToggles L={L} />
      </Section>
      <Section id="env" title="Environment" aside={`${g.env.length}`}>
        <EnvTable L={L} />
      </Section>
      <Section id="args" title="Launch arguments">
        <input className="args-input" value={g.args} spellCheck={false} placeholder="none" aria-label="Launch arguments" onChange={(e) => L.actions.setOpt(g.id, 'args', e.target.value)} />
      </Section>
      <Section id="cmd" title="Command preview">
        <LaunchLine L={L} />
      </Section>
      </>}
    </>
  );
}

function DefaultSection({ title, aside, children }) {
  return (
    <section className="isec">
      <h3 className="isec-h"><span>{title}</span>{aside && <span className="isec-aside">{aside}</span>}</h3>
      <div className="isec-body">{children}</div>
    </section>
  );
}

export function Row({ label, children }) {
  return (
    <div className="prow">
      <span className="prow-k">{label}</span>
      <div className="prow-v">{children}</div>
    </div>
  );
}

export function SessionsTable({ L, compact }) {
  if (!L.sessions.length) return <p className="empty">No sessions yet. Install the game, then play it once to start a history.</p>;
  return (
    <table className="tbl">
      <thead>
        <tr><th>Started</th><th className="num">Length</th>{!compact && <th>Runner</th>}<th>Result</th></tr>
      </thead>
      <tbody>
        {L.sessions.map((s, i) => (
          <tr key={i} className={s.crash ? 'is-bad' : ''}>
            <td><span className="t-day">{dayLabel(s.start)}</span> <span className="t-time">{clock(s.start)}</span></td>
            <td className="num">{dur(s.dur)}</td>
            {!compact && <td className="t-dim">{runnerOf(s.runner)?.name}</td>}
            <td className={s.crash ? 't-bad' : 't-dim'}>{s.exit}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function SavesTable({ L }) {
  const g = L.game;
  if (!g.saves.length) return <p className="empty">No saves found. Platypus looks in the prefix and the store’s cloud once the game has been played.</p>;
  const now = L.now;
  return (
    <table className="tbl">
      <thead><tr><th>Save</th><th>This PC</th><th>Cloud</th><th className="num">Size</th></tr></thead>
      <tbody>
        {g.saves.map((s, i) => {
          const newer = s.local && s.cloud && s.cloud.at < s.local.at ? 'cloud' : s.local && s.cloud && s.local.at < s.cloud.at ? 'local' : null;
          return (
            <tr key={i} className={s.conflict ? 'is-conflict' : ''}>
              <td className="t-strong">{s.slot}</td>
              <td className={newer === 'cloud' ? 't-new' : ''}>{s.local ? ago(s.local.at) : <span className="t-dim">not on disk</span>}</td>
              <td className={newer === 'local' ? 't-new' : ''}>{s.cloud ? <>{ago(s.cloud.at)}{s.cloud.device !== 'this PC' && <span className="t-dim"> · {s.cloud.device}</span>}</> : <span className="t-dim">not uploaded</span>}</td>
              <td className="num">{(s.local || s.cloud).size}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

export function NotesList({ L }) {
  const notes = L.game.notes;
  if (!notes.length) return <p className="empty">No compatibility notes for this game yet.</p>;
  return (
    <table className="tbl tbl-notes">
      <thead><tr><th>Source</th><th>Note</th><th className="num">Added</th></tr></thead>
      <tbody>
        {notes.map((n, i) => (
          <tr key={i}>
            <td className="t-dim">{n.src}</td>
            <td className="t-wrap">{n.text}</td>
            <td className="num t-dim">{ago(n.when)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Local vs. cloud, side by side. Used inline in the palette and in the dialog.
export function ConflictCompare({ L, gameId, autoFocus }) {
  const g = L.games.find((x) => x.id === gameId);
  const s = g?.saves.find((x) => x.conflict);
  const first = useRef();
  useEffect(() => { if (autoFocus) first.current?.focus(); }, [autoFocus]);
  if (!s) return <p className="empty">{g?.title}: saves are in sync.</p>;
  const now = L.now;
  const localNewer = s.local.at < s.cloud.at;
  return (
    <div className="cfx">
      <p className="cfx-lead">“{s.slot}” changed in two places since the last sync.</p>
      <div className="cfx-grid">
        <div className={`cfx-side ${localNewer ? 'is-newer' : ''}`}>
          <div className="cfx-h"><Monitor size={13} /> This PC</div>
          <dl>
            <dt>Saved</dt><dd>{ago(s.local.at)} <span className="t-dim">· {clock(now - s.local.at)}</span></dd>
            <dt>Size</dt><dd>{s.local.size}</dd>
          </dl>
          {localNewer && <span className="cfx-flag">newer</span>}
        </div>
        <div className={`cfx-side ${!localNewer ? 'is-newer' : ''}`}>
          <div className="cfx-h"><Cloud size={13} /> Cloud · {s.cloud.device}</div>
          <dl>
            <dt>Saved</dt><dd>{ago(s.cloud.at)} <span className="t-dim">· {clock(now - s.cloud.at)}</span></dd>
            <dt>Size</dt><dd>{s.cloud.size}</dd>
          </dl>
          {!localNewer && <span className="cfx-flag">newer</span>}
        </div>
      </div>
      <div className="cfx-actions">
        {/* The newer save is the suggested choice. */}
        <button ref={localNewer ? first : undefined} type="button" className={`btn ${localNewer ? 'btn-primary' : ''}`} style={{ order: localNewer ? 0 : 1 }} onClick={() => L.actions.resolveConflict(g.id, 'local')}>Keep this PC’s save</button>
        <button ref={localNewer ? undefined : first} type="button" className={`btn ${localNewer ? '' : 'btn-primary'}`} style={{ order: localNewer ? 1 : 0 }} onClick={() => L.actions.resolveConflict(g.id, 'cloud')}>Use cloud save</button>
        <button type="button" className="btn btn-quiet" style={{ order: 2 }} onClick={() => L.actions.resolveConflict(g.id, 'both')}>Keep both</button>
      </div>
    </div>
  );
}

export function ConflictDialog({ L }) {
  const id = L.conflictFor;
  const ref = useRef();
  useEffect(() => {
    if (!id) return;
    const prev = document.activeElement;
    const onKey = (e) => { if (e.key === 'Escape') L.actions.closeConflict(); };
    window.addEventListener('keydown', onKey);
    return () => { window.removeEventListener('keydown', onKey); prev?.focus?.(); };
  }, [id]);
  if (!id) return null;
  const g = L.games.find((x) => x.id === id);
  return (
    <div className="scrim" onPointerDown={(e) => e.target === e.currentTarget && L.actions.closeConflict()}>
      <div ref={ref} className="dialog" role="dialog" aria-modal="true" aria-labelledby="cfx-title">
        <div className="dialog-head">
          <h2 id="cfx-title">Resolve save conflict · {g.title}</h2>
          <button type="button" className="icon-btn" aria-label="Close" onClick={L.actions.closeConflict}><X size={14} /></button>
        </div>
        <ConflictCompare L={L} gameId={id} autoFocus />
      </div>
    </div>
  );
}

// ---------- Command palette ----------
function buildItems(L, mode) {
  const g = L.game;
  const A = L.actions;
  if (mode === 'runner') {
    const opts = runnerOptions(L.runners, g.tier === 'native' || g.runner === 'native');
    return opts.map((o) => ({ id: `r-${o.value}`, group: `Runner for ${g.title}`, label: o.label, hint: o.hint, active: g.runner === o.value, run: () => { A.setRunner(g.id, o.value); A.say(`${g.title} now runs on ${o.label}`); A.closePalette(); } }));
  }
  if (mode === 'recipe') {
    return L.recipes.map((r) => ({ id: `x-${r.id}`, group: `Fix recipes for ${g.title}`, label: r.label, hint: r.desc, active: g.recipes.includes(r.id), keep: true, run: () => A.toggleRecipe(g.id, r.id) }));
  }
  if (mode === 'source') {
    return g.stores.map((s) => ({ id: `s-${s.store}`, group: `Launch ${g.title} from`, label: STORES[s.store].name, hint: s.installed ? `installed · ${s.version}` : `not installed · ${s.size}`, store: s.store, active: g.source === s.store, run: () => { A.setSource(g.id, s.store); A.closePalette(); } }));
  }
  const conflicts = L.games.filter((x) => x.sync === 'conflict');
  const items = [];
  for (const c of conflicts) items.push({ id: `cf-${c.id}`, group: 'Needs attention', label: `Resolve save conflict: ${c.title}`, hint: 'compare local and cloud', tone: 'bad', next: { mode: 'conflict', gameId: c.id } });
  const installed = activeCopy(g).installed;
  items.push(
    { id: 'play', group: g.title, label: installed ? `Play ${g.title}` : `Install ${g.title}`, kbd: '↵', run: () => { A.play(g.id); A.closePalette(); } },
    { id: 'runner', group: g.title, label: 'Switch runner…', hint: runnerOf(g.runner)?.name, next: { mode: 'runner' }, chip: 'Runner' },
    { id: 'recipe', group: g.title, label: 'Apply fix recipe…', hint: `${g.recipes.length} on`, next: { mode: 'recipe' }, chip: 'Recipe' },
  );
  if (g.stores.length > 1) items.push({ id: 'source', group: g.title, label: 'Launch from another store…', hint: STORES[g.source].name, next: { mode: 'source' }, chip: 'Store' });
  items.push(
    { id: 'mango', group: g.title, label: g.mangohud ? 'Turn MangoHud off' : 'Turn MangoHud on', run: () => { A.toggleOpt(g.id, 'mangohud'); A.closePalette(); } },
    { id: 'verify', group: g.title, label: 'Verify game files', run: () => { A.verify(g.id); A.closePalette(); } },
    { id: 'insp', group: 'View', label: L.inspector ? 'Hide inspector' : 'Show inspector', kbd: 'Ctrl I', run: () => { A.toggleInspector(); A.closePalette(); } },
  );
  for (const x of L.games) items.push({ id: `g-${x.id}`, group: 'Go to game', label: x.title, store: x.source, hint: isInstalled(x) ? `${Math.round(x.hours)} h played` : 'not installed', run: () => { A.select(x.id); A.closePalette(); } });
  return items;
}

export function Palette({ L, placeholder = 'Type a command or game…' }) {
  const p = L.palette;
  const [q, setQ] = useState('');
  const [hi, setHi] = useState(0);
  const input = useRef();
  const listRef = useRef();
  const mode = p?.mode || 'root';
  useEffect(() => { setQ(''); setHi(0); }, [mode, !!p]);
  useEffect(() => { if (p) setTimeout(() => input.current?.focus(), 0); }, [p, mode]);
  const all = useMemo(() => (p && mode !== 'conflict' ? buildItems(L, mode) : []), [p, mode, L.game, L.games, L.inspector]);
  const items = useMemo(() => {
    if (!q) {
      if (mode !== 'root') return all;
      const games = all.filter((i) => i.group === 'Go to game');
      return [...all.filter((i) => i.group !== 'Go to game'), ...games.slice(0, 5)];
    }
    return all.map((i) => ({ i, s: score(q, i.label) + (i.group === 'Go to game' ? 0.5 : 0) })).filter((x) => x.s > 0).sort((a, b) => b.s - a.s).map((x) => x.i).slice(0, 40);
  }, [all, q]);
  useEffect(() => { listRef.current?.querySelector('.is-hi')?.scrollIntoView({ block: 'nearest' }); }, [hi]);
  if (!p) return null;
  const chipFor = { runner: 'Runner', recipe: 'Recipe', source: 'Store', conflict: 'Save conflict' }[mode];
  const exec = (it) => {
    if (!it) return;
    if (it.next) { L.actions.setPalette({ ...it.next }); return; }
    it.run?.();
  };
  const onKey = (e) => {
    if (e.key === 'Escape') { e.preventDefault(); mode === 'root' ? L.actions.closePalette() : L.actions.setPalette({ mode: 'root' }); }
    else if (e.key === 'ArrowDown') { e.preventDefault(); setHi((h) => Math.min(items.length - 1, h + 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setHi((h) => Math.max(0, h - 1)); }
    else if (e.key === 'Enter') { e.preventDefault(); exec(items[hi]); }
    else if (e.key === 'Backspace' && !q && mode !== 'root') { e.preventDefault(); L.actions.setPalette({ mode: 'root' }); }
  };
  let lastGroup = null;
  return (
    <div className="pal-scrim" onPointerDown={(e) => e.target === e.currentTarget && L.actions.closePalette()}>
      <div className="pal" role="dialog" aria-modal="true" aria-label="Command palette" onKeyDown={onKey}>
        <div className="pal-bar">
          {chipFor && <button type="button" className="pal-chip" onClick={() => L.actions.setPalette({ mode: 'root' })} title="Back (Backspace)">{chipFor}<X size={11} /></button>}
          <input ref={input} className="pal-input" value={q} onChange={(e) => { setQ(e.target.value); setHi(0); }} placeholder={mode === 'conflict' ? 'Choose which save to keep' : chipFor ? `Search ${chipFor.toLowerCase()}s…` : placeholder} aria-label="Command" role="combobox" aria-expanded="true" aria-controls="pal-list" aria-activedescendant={`pal-${hi}`} readOnly={mode === 'conflict'} />
          <kbd className="pal-esc">esc</kbd>
        </div>
        {mode === 'conflict' ? (
          <div className="pal-body"><ConflictCompare L={L} gameId={p.gameId} autoFocus /></div>
        ) : (
          <ul ref={listRef} id="pal-list" className="pal-list" role="listbox">
            {items.length === 0 && <li className="pal-empty">Nothing matches “{q}”.</li>}
            {items.map((it, i) => {
              const head = it.group !== lastGroup ? (lastGroup = it.group) : null;
              return (
                <React.Fragment key={it.id}>
                  {head && <li className="pal-group" role="presentation">{head}</li>}
                  <li id={`pal-${i}`} role="option" aria-selected={i === hi} className={`pal-item ${i === hi ? 'is-hi' : ''} ${it.tone === 'bad' ? 'is-bad' : ''} ${it.active ? 'is-active' : ''}`} onPointerMove={() => setHi(i)} onClick={() => exec(it)}>
                    <span className="pal-ico">{it.store ? <StoreGlyph store={it.store} size={12} /> : it.active ? <Check size={13} /> : it.next ? <ArrowRight size={13} /> : <span className="pal-dot" />}</span>
                    <span className="pal-label">{it.label}</span>
                    {it.hint && <span className="pal-hint">{it.hint}</span>}
                    {it.kbd && <kbd className="pal-kbd">{it.kbd}</kbd>}
                    {i === hi && <CornerDownLeft size={12} className="pal-enter" />}
                  </li>
                </React.Fragment>
              );
            })}
          </ul>
        )}
        <div className="pal-foot">
          <span><kbd>↑</kbd><kbd>↓</kbd> move</span><span><kbd>↵</kbd> run</span>{mode !== 'root' && <span><kbd>⌫</kbd> back</span>}
        </div>
      </div>
    </div>
  );
}

export function Toast({ L }) {
  if (!L.toast) return null;
  return <div key={L.toast.n} className={`toast toast-${L.toast.tone}`} role="status">{L.toast.text}</div>;
}

export function Img({ src, alt = '', className, ...rest }) {
  const [ok, setOk] = useState(true);
  if (!src || !ok) return <span className={`${className || ''} img-missing`} aria-hidden />;
  return <img src={src} alt={alt} className={className} loading="lazy" decoding="async" onError={() => setOk(false)} {...rest} />;
}

export function GameLogo({ g, className }) {
  const [ok, setOk] = useState(true);
  const a = artOf(g);
  if (!ok || !a.logo) return <span className={`${className} logo-text`}>{g.title}</span>;
  return <img src={a.logo} alt={g.title} className={className} onError={() => setOk(false)} />;
}

// Tabs with roving arrow keys.
export function Tabs({ tabs, value, onChange, className = '', label }) {
  const onKey = (e) => {
    const i = tabs.findIndex((t) => t.id === value);
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault();
      const n = tabs[(i + (e.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length];
      onChange(n.id);
      e.currentTarget.querySelector(`[data-tab="${n.id}"]`)?.focus();
    }
  };
  return (
    <div className={`tabs ${className}`} role="tablist" aria-label={label} onKeyDown={onKey}>
      {tabs.map((t) => (
        <button key={t.id} data-tab={t.id} type="button" role="tab" aria-selected={t.id === value} tabIndex={t.id === value ? 0 : -1} className={`tab ${t.id === value ? 'is-on' : ''}`} onClick={() => onChange(t.id)}>
          {t.label}{t.count != null && <span className="tab-count">{t.count}</span>}
        </button>
      ))}
    </div>
  );
}

export const readoutTabs = (L) => [
  { id: 'sessions', label: 'Sessions', count: L.sessions.length },
  { id: 'saves', label: 'Saves', count: L.game.saves.length },
  { id: 'compat', label: 'Compatibility', count: L.game.notes.length },
];

export function ReadoutBody({ L, tab, compact }) {
  if (tab === 'saves') return <SavesTable L={L} />;
  if (tab === 'compat') return <NotesList L={L} />;
  return <SessionsTable L={L} compact={compact} />;
}

export function elapsed(ms) {
  const s = Math.max(0, Math.floor(ms / 1000));
  return [Math.floor(s / 3600), Math.floor((s % 3600) / 60), s % 60].map((n) => String(n).padStart(2, '0')).join(':');
}

export function storeCounts(games) {
  const c = { all: games.length };
  for (const g of games) for (const s of g.stores) c[s.store] = (c[s.store] || 0) + 1;
  return c;
}

// ---------- Library filters that scale to any number of stores ----------
export const SORTS = [
  { id: 'recent', label: 'Recently played' },
  { id: 'title', label: 'Title' },
  { id: 'hours', label: 'Most played' },
];

export function filterCount(L) {
  return L.stores.length + (L.stateFilter !== 'all' ? 1 : 0);
}

export function FilterMenu({ L, label = 'Filter', icon = true, align = 'left', sorts = SORTS }) {
  const [open, setOpen] = useState(false);
  const root = useRef();
  const panel = useRef();
  useEffect(() => {
    if (!open) return;
    panel.current?.querySelector('button, input')?.focus();
    const close = (e) => { if (!root.current?.contains(e.target)) setOpen(false); };
    window.addEventListener('pointerdown', close);
    return () => window.removeEventListener('pointerdown', close);
  }, [open]);
  const counts = {};
  for (const g of L.games) for (const s of g.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const n = filterCount(L);
  const onKey = (e) => {
    if (e.key === 'Escape') { e.stopPropagation(); setOpen(false); root.current?.querySelector('.fm-btn')?.focus(); }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      const items = [...panel.current.querySelectorAll('button')];
      const i = items.indexOf(document.activeElement);
      items[(i + (e.key === 'ArrowDown' ? 1 : items.length - 1)) % items.length]?.focus();
      e.preventDefault();
    }
  };
  return (
    <div className={`fm fm-${align}`} ref={root}>
      <button type="button" className={`fm-btn ${n ? 'is-active' : ''}`} aria-haspopup="dialog" aria-expanded={open} onClick={() => setOpen(!open)}>
        {icon && <SlidersHorizontal size={14} strokeWidth={1.8} />}
        {label && <span>{label}</span>}
        {n > 0 && <span className="fm-n">{n}</span>}
      </button>
      {open && (
        <div className="fm-panel" ref={panel} role="dialog" aria-label="Filter and sort the library" onKeyDown={onKey}>
          <div className="fm-sec">
            <div className="fm-h">Stores</div>
            {Object.keys(STORES).filter((s) => counts[s]).map((s) => {
              const on = L.stores.includes(s);
              return (
                <button key={s} type="button" aria-pressed={on} className={`fm-item ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleStore(s)}>
                  <span className="fm-check" aria-hidden>{on && <Check size={11} strokeWidth={3} />}</span>
                  <StoreGlyph store={s} size={13} />
                  <span className="fm-label">{STORES[s].name}</span>
                  <span className="fm-count">{counts[s]}</span>
                </button>
              );
            })}
          </div>
          <div className="fm-sec">
            <div className="fm-h">Show</div>
            {STATE_FILTERS.map((f) => (
              <button key={f.id} type="button" aria-pressed={L.stateFilter === f.id} className={`fm-item ${L.stateFilter === f.id ? 'is-on' : ''}`} onClick={() => L.actions.setStateFilter(f.id)}>
                <span className="fm-radio" aria-hidden />
                <span className="fm-label">{f.label}</span>
                <span className="fm-count">{L.games.filter((g) => stateMatch(g, f.id)).length}</span>
              </button>
            ))}
          </div>
          {sorts && (
            <div className="fm-sec">
              <div className="fm-h">Sort by</div>
              {sorts.map((o) => (
                <button key={o.id} type="button" aria-pressed={L.sort === o.id} className={`fm-item ${L.sort === o.id ? 'is-on' : ''}`} onClick={() => L.actions.setSort(o.id)}>
                  <span className="fm-radio" aria-hidden />
                  <span className="fm-label">{o.label}</span>
                </button>
              ))}
            </div>
          )}
          <div className="fm-foot">
            <span>{L.visible.length} of {L.games.length} games</span>
            <button type="button" className="fm-reset" disabled={!n} onClick={() => { L.actions.setStores([]); L.actions.setStateFilter('all'); }}>Reset</button>
          </div>
        </div>
      )}
    </div>
  );
}

export function FilterChips({ L }) {
  if (!filterCount(L)) return null;
  return (
    <div className="chips" aria-label="Active filters">
      {L.stores.map((s) => (
        <button key={s} type="button" className="chip" onClick={() => L.actions.toggleStore(s)} aria-label={`Remove ${STORES[s].name} filter`}>
          <StoreGlyph store={s} size={11} />{STORES[s].name}<X size={11} />
        </button>
      ))}
      {L.stateFilter !== 'all' && (
        <button type="button" className="chip" onClick={() => L.actions.setStateFilter('all')} aria-label="Remove state filter">
          {STATE_FILTERS.find((f) => f.id === L.stateFilter).label}<X size={11} />
        </button>
      )}
    </div>
  );
}

// Plain-language settings for people who don't want to know what a prefix is.
export function SimpleSettings({ L, onAdvanced }) {
  const g = L.game;
  const native = g.runner === 'native';
  const hasNative = g.tier === 'native' || native;
  const opts = runnerOptions(L.runners, hasNative).map((o, i) => (i === 0 ? { ...o, hint: 'recommended' } : o));
  const items = [
    { key: 'gamemode', label: 'Performance mode', desc: 'Keeps the CPU and GPU at full speed while you play.' },
    { key: 'mangohud', label: 'Show FPS and temperatures', desc: 'A small overlay in the corner of the game.' },
    { key: 'gamescope', label: 'Lock resolution and refresh rate', desc: 'Runs the game in its own display session. Helps with games that pick the wrong screen mode.' },
    { key: 'fsr', label: 'Sharper upscaling', desc: 'Renders at a lower resolution and sharpens it. Needs the option above.', needs: 'gamescope' },
  ];
  return (
    <div className="simple">
      <div className="simple-row">
        <div className="simple-text">
          <span className="simple-label">Compatibility</span>
          <span className="simple-desc">{native ? 'Runs natively on Linux. Nothing to translate.' : 'Translates the Windows game for Linux.'}</span>
        </div>
        <Select label="Compatibility" value={g.runner} options={opts} onChange={(v) => L.actions.setRunner(g.id, v)} />
      </div>
      {items.map((it) => {
        const disabled = it.needs && !g[it.needs];
        const on = !!g[it.key] && !disabled;
        return (
          <button key={it.key} type="button" role="switch" aria-checked={on} disabled={disabled} className={`simple-row simple-switch ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleOpt(g.id, it.key)}>
            <span className="simple-text">
              <span className="simple-label">{it.label}</span>
              <span className="simple-desc">{it.desc}</span>
            </span>
            <span className="tgl-track" aria-hidden><span className="tgl-thumb" /></span>
          </button>
        );
      })}
      <div className="simple-row">
        <div className="simple-text">
          <span className="simple-label">Fixes</span>
          <span className="simple-desc">{native ? 'Not needed for native games.' : g.recipes.length ? `${g.recipes.length} community fix${g.recipes.length > 1 ? 'es' : ''} applied: ${g.recipes.map((r) => L.recipes.find((x) => x.id === r)?.label).join(', ')}.` : 'No fixes needed so far.'}</span>
        </div>
        {onAdvanced && <button type="button" className="btn btn-quiet simple-more" onClick={onAdvanced}>Advanced</button>}
      </div>
    </div>
  );
}

// ---------- Attention: a mark only when something needs the user ----------
export function attentionOf(L, x) {
  if (L.running?.id === x.id) return { kind: 'playing', label: 'Playing now' };
  const job = L.jobs.find((j) => j.gameId === x.id && j.kind === 'download');
  if (job) return { kind: 'download', label: `Downloading, ${Math.round(job.progress * 100)}%`, progress: job.progress };
  if (x.sync === 'conflict') return { kind: 'conflict', label: 'Save conflict' };
  const upd = x.stores.find((s) => s.update);
  if (upd) return { kind: 'update', label: `Update ready, ${upd.update}` };
  return null;
}

export function AttnMark({ L, x }) {
  const a = attentionOf(L, x);
  if (!a) return null;
  if (a.kind === 'download') {
    return <span className="attn attn-download" title={a.label} aria-label={a.label} style={{ '--p': a.progress }} />;
  }
  const icon = a.kind === 'conflict' ? <AlertTriangle size={12} strokeWidth={2.2} /> : a.kind === 'update' ? <ArrowDownToLine size={12} strokeWidth={2.2} /> : null;
  return <span className={`attn attn-${a.kind}`} title={a.label} aria-label={a.label}>{icon}</span>;
}

export function viewCount(L, id) {
  if (id === 'downloads') return L.games.filter((g) => isDownloading(g, L.jobs)).length;
  return L.games.filter((g) => stateMatch(g, id)).length;
}

// ---------- Settings, organised by what the user wants to achieve ----------
export function SRow({ label, desc, children, wide }) {
  return (
    <div className={`srow ${wide ? 'is-wide' : ''}`}>
      <div className="srow-text">
        <span className="srow-label">{label}</span>
        {desc && <span className="srow-desc">{desc}</span>}
      </div>
      {children && <div className="srow-ctl">{children}</div>}
    </div>
  );
}

export function SSwitch({ L, k, label, desc, needs }) {
  const g = L.game;
  const disabled = needs && !g[needs];
  const on = !!g[k] && !disabled;
  return (
    <button type="button" role="switch" aria-checked={on} disabled={disabled} className={`srow sswitch ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleOpt(g.id, k)}>
      <span className="srow-text"><span className="srow-label">{label}</span>{desc && <span className="srow-desc">{desc}</span>}</span>
      <span className="tgl-track" aria-hidden><span className="tgl-thumb" /></span>
    </button>
  );
}

export const SETTINGS_CATS = [
  { id: 'compat', label: 'Compatibility', icon: Cpu, level: 'basic', blurb: 'How the game runs on Linux' },
  { id: 'performance', label: 'Performance', icon: Gauge, level: 'basic', blurb: 'Speed and the FPS overlay' },
  { id: 'display', label: 'Display', icon: MonitorPlay, level: 'basic', blurb: 'Resolution, refresh rate, upscaling' },
  { id: 'saves', label: 'Saves', icon: Cloud, level: 'basic', blurb: 'Cloud sync and save files' },
  { id: 'layers', label: 'Translation layers', icon: Layers, level: 'advanced', blurb: 'DXVK, VKD3D and sync primitives' },
  { id: 'fixes', label: 'Fixes', icon: Wrench, level: 'advanced', blurb: 'Extra components and game-specific fixes' },
  { id: 'env', label: 'Environment', icon: Braces, level: 'advanced', blurb: 'Variables passed to the game' },
  { id: 'launch', label: 'Launch options', icon: TerminalSquare, level: 'advanced', blurb: 'Arguments and the full command' },
];

export function catSummary(L, id) {
  const g = L.game;
  const native = g.runner === 'native';
  switch (id) {
    case 'compat': return native ? 'Native Linux' : runnerOf(g.runner)?.name;
    case 'performance': return [g.gamemode && 'GameMode', g.mangohud && 'Overlay'].filter(Boolean).join(', ') || 'Default';
    case 'display': return g.gamescope ? (g.fsr ? 'Gamescope, FSR' : 'Gamescope') : 'Default';
    case 'saves': return SYNC_LABEL[g.sync];
    case 'layers': return native ? 'Not used' : g.graphics === 'Bundled with runner' ? 'Bundled' : g.graphics;
    case 'fixes': {
      if (native) return 'Not needed';
      const sug = (RECOMMEND[g.id] || []).filter((r) => !g.recipes.includes(r.id)).length;
      return [g.recipes.length ? `${g.recipes.length} applied` : 'None applied', sug && `${sug} suggested`].filter(Boolean).join(', ');
    }
    case 'env': return g.env.length ? `${g.env.length} set` : 'None';
    case 'launch': return g.args || 'No arguments';
    default: return '';
  }
}

export function SettingsCategory({ L, id }) {
  const g = L.game;
  const native = g.runner === 'native';
  const hasNative = g.tier === 'native' || native;
  switch (id) {
    case 'compat':
      return (
        <>
          <SRow label="Compatibility layer" desc={native ? 'This game has a Linux version. It runs directly, nothing is translated.' : 'Translates the Windows version of the game so it runs on Linux. The first choice is the one Platypus recommends.'}>
            <Select label="Compatibility layer" value={g.runner} options={runnerOptions(L.runners, hasNative).map((o, i) => (i === 0 ? { ...o, hint: 'recommended' } : o))} onChange={(v) => L.actions.setRunner(g.id, v)} />
          </SRow>
          <SRow label="Community rating" desc={`Players rate this game ${TIER_LABEL[g.tier]} on ProtonDB.`} />
          {g.notes[0] && <SRow label="Latest tip" desc={g.notes[0].text} />}
        </>
      );
    case 'performance':
      return (
        <>
          <SSwitch L={L} k="gamemode" label="Performance mode" desc="Keeps the CPU and GPU at full speed while you play (GameMode)." />
          <SSwitch L={L} k="mangohud" label="Show FPS and temperatures" desc="A small overlay in the corner of the game (MangoHud)." />
        </>
      );
    case 'display':
      return (
        <>
          <SSwitch L={L} k="gamescope" label="Lock resolution and refresh rate" desc="Runs the game in its own display session at 2560×1440, 165 Hz. Helps games that pick the wrong screen mode (Gamescope)." />
          <SSwitch L={L} k="fsr" needs="gamescope" label="Sharper upscaling" desc="Renders at a lower resolution and sharpens the image (FSR). Needs the option above." />
        </>
      );
    case 'saves':
      return (
        <>
          <SRow label="Cloud saves" desc={g.sync === 'conflict' ? 'Your save changed on this PC and in the cloud. Choose which one to keep.' : `${SYNC_LABEL[g.sync]}. ${g.sync === 'off' ? 'This store does not sync saves for this game.' : ''}`}>
            {g.sync === 'conflict' && <button type="button" className="btn btn-primary" onClick={() => L.actions.openConflict(g.id)}>Resolve</button>}
          </SRow>
          <div className="srow-block"><SavesTable L={L} /></div>
        </>
      );
    case 'layers':
      return (
        <>
          <SRow label="Graphics layer" desc="Translates Direct3D to Vulkan. Bundled uses whatever the runner ships with.">
            <Select label="Graphics layer" value={native ? '—' : g.graphics} disabled={native} options={native ? ['—'] : GRAPHICS} onChange={(v) => L.actions.setOpt(g.id, 'graphics', v)} />
          </SRow>
          <SRow label="Sync primitive" desc="How Wine waits on threads. NTSync is fastest on recent kernels.">
            <Select label="Sync primitive" value={native ? '—' : g.syncPrim} disabled={native} options={native ? ['—'] : SYNC_PRIMITIVES} onChange={(v) => L.actions.setOpt(g.id, 'syncPrim', v)} />
          </SRow>
        </>
      );
    case 'fixes':
      return <FixesPanel L={L} />;
    case 'env':
      return (
        <>
          <SRow label="Environment variables" desc="Passed to the game and its runner on every launch." />
          <div className="srow-block"><EnvTable L={L} /></div>
        </>
      );
    case 'launch':
      return (
        <>
          <SRow label="Launch arguments" desc="Added after the game’s executable." wide>
            <input className="args-input" value={g.args} spellCheck={false} placeholder="none" aria-label="Launch arguments" onChange={(e) => L.actions.setOpt(g.id, 'args', e.target.value)} />
          </SRow>
          <SRow label="Full command" desc="Exactly what Platypus runs. It updates as you change settings." wide />
          <div className="srow-block"><LaunchLine L={L} /></div>
        </>
      );
    default:
      return null;
  }
}

// Fixes: what is applied, what players suggest for this game, and the rest of the catalog.
const KIND_LABEL = { winetricks: 'Winetricks', umu: 'UMU fix' };
export function FixesPanel({ L }) {
  const g = L.game;
  const native = g.runner === 'native';
  const [q, setQ] = useState('');
  const [cat, setCat] = useState('All');
  const [confirmReset, setConfirmReset] = useState(false);
  useEffect(() => { setQ(''); setCat('All'); setConfirmReset(false); }, [g.id]);
  if (native) {
    return <SRow label="Nothing to fix" desc="This game runs natively on Linux. Fixes only apply to games that run through a compatibility layer." />;
  }
  const byId = (id) => L.recipes.find((r) => r.id === id);
  const original = GAMES.find((x) => x.id === g.id)?.recipes || [];
  const suggested = (RECOMMEND[g.id] || []).filter((r) => !g.recipes.includes(r.id));
  const applied = g.recipes.map(byId).filter(Boolean);
  const cats = ['All', ...new Set(L.recipes.map((r) => r.cat))];
  const ql = q.trim().toLowerCase();
  const rest = L.recipes.filter((r) => !g.recipes.includes(r.id) && (cat === 'All' || r.cat === cat) && (!ql || `${r.name} ${r.label} ${r.desc}`.toLowerCase().includes(ql)));
  const toggle = (r, on) => { L.actions.toggleRecipe(g.id, r.id); L.actions.say(on ? `${r.name} will be installed on next launch` : `${r.name} removed from ${g.title}`); };
  return (
    <div className="fixes">
      {suggested.length > 0 && (
        <section className="fixes-sec" aria-labelledby="fx-sug">
          <h4 id="fx-sug" className="fixes-h"><Sparkles size={14} aria-hidden />Suggested for {g.title}</h4>
          <ul className="fixes-list">
            {suggested.map((s) => {
              const r = byId(s.id);
              return (
                <li key={s.id} className="fix is-suggested">
                  <span className="fix-text">
                    <span className="fix-name">{r.name}</span>
                    <span className="fix-why">{s.why}</span>
                    <span className="fix-src">{s.votes ? `${s.votes} ProtonDB reports` : `From ${s.src}`}</span>
                  </span>
                  <button type="button" className="btn btn-primary fix-btn" onClick={() => toggle(r, true)}>Apply</button>
                </li>
              );
            })}
          </ul>
        </section>
      )}
      <section className="fixes-sec" aria-labelledby="fx-on">
        <h4 id="fx-on" className="fixes-h">Applied to this game<span className="fixes-n">{applied.length}</span></h4>
        {applied.length === 0 ? <p className="fixes-empty">None. {g.title} runs without extra components.</p> : (
          <ul className="fixes-list">
            {applied.map((r) => {
              const pending = !original.includes(r.id);
              return (
                <li key={r.id} className="fix">
                  <span className="fix-text">
                    <span className="fix-name">{r.name}<code className="fix-id">{r.label}</code></span>
                    <span className="fix-why">{r.desc}</span>
                    <span className={`fix-state ${pending ? 'is-pending' : ''}`}>{pending ? 'Installs on next launch' : `Installed with ${KIND_LABEL[r.kind]}`}</span>
                  </span>
                  <button type="button" className="btn btn-quiet fix-btn" onClick={() => toggle(r, false)}>Remove</button>
                </li>
              );
            })}
          </ul>
        )}
      </section>
      <section className="fixes-sec" aria-labelledby="fx-add">
        <h4 id="fx-add" className="fixes-h">Add a fix</h4>
        <div className="fixes-tools">
          <label className="fixes-search"><Search size={14} aria-hidden /><input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search fixes, e.g. fonts, cutscenes" aria-label="Search fixes" /></label>
          <div className="fixes-cats" role="radiogroup" aria-label="Category">
            {cats.map((c) => <button key={c} type="button" role="radio" aria-checked={cat === c} className={cat === c ? 'is-on' : ''} onClick={() => setCat(c)}>{c}</button>)}
          </div>
        </div>
        <ul className="fixes-list is-catalog">
          {rest.map((r) => (
            <li key={r.id} className="fix">
              <span className="fix-text">
                <span className="fix-name">{r.name}<code className="fix-id">{r.label}</code></span>
                <span className="fix-why">{r.desc}</span>
              </span>
              <button type="button" className="btn fix-btn" onClick={() => toggle(r, true)}>Add</button>
            </li>
          ))}
          {rest.length === 0 && <li className="fixes-empty">No fix matches “{q}”.</li>}
        </ul>
      </section>
      <section className="fixes-sec" aria-labelledby="fx-pfx">
        <h4 id="fx-pfx" className="fixes-h">Wine prefix</h4>
        <p className="fixes-path"><code>~/.local/share/platypus/prefixes/{g.id}</code></p>
        <div className="fixes-actions">
          <button type="button" className="btn" onClick={() => L.actions.say('Opened the prefix folder')}><FolderOpen size={14} />Open folder</button>
          <button type="button" className="btn" onClick={() => L.actions.say('Pick a .exe to run inside this prefix')}><PlayIcon size={14} />Run a program in it</button>
          {confirmReset ? (
            <span className="fixes-confirm" role="alert">Delete the prefix and rebuild it on next launch? Saves stored in it are kept.
              <button type="button" className="btn btn-danger" onClick={() => { setConfirmReset(false); L.actions.say(`Prefix for ${g.title} will be rebuilt on next launch`); }}>Reset</button>
              <button type="button" className="btn btn-quiet" onClick={() => setConfirmReset(false)}>Cancel</button>
            </span>
          ) : <button type="button" className="btn btn-quiet" onClick={() => setConfirmReset(true)}><RotateCcw size={14} />Reset prefix…</button>}
        </div>
      </section>
    </div>
  );
}
