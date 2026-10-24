import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/public-sans';
import '@fontsource-variable/red-hat-mono';
import './d4.css';
import { Play, Square, Search, PanelRight, AlertTriangle, Loader, ChevronUp, ChevronDown } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, clock, matches } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ title, aside, children }) {
  return (
    <fieldset className="grp">
      <legend>{title}{aside && <span> ({aside})</span>}</legend>
      {children}
    </fieldset>
  );
}

function status(L, x) {
  if (L.running?.id === x.id) return { text: 'Playing', cls: 'is-live' };
  const c = activeCopy(x);
  if (L.jobs.some((j) => j.id === `dl-${x.id}`)) return { text: 'Installing', cls: 'is-busy' };
  if (!c.installed) return { text: 'Not installed', cls: 'is-dim' };
  if (c.update) return { text: `Update ${c.update}`, cls: 'is-busy' };
  if (x.sync === 'conflict') return { text: 'Save conflict', cls: 'is-bad' };
  return { text: 'Ready', cls: '' };
}

// The status strip that opens into a console. ` toggles it, like the games did.
function Console({ L }) {
  const [open, setOpen] = useState(false);
  const [cmd, setCmd] = useState('');
  const [hist, setHist] = useState([]);
  const [clearedAt, setClearedAt] = useState(0);
  const out = useRef();
  const input = useRef();
  useEffect(() => {
    const onKey = (e) => {
      if (e.key === '`' || e.key === '~') {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  useEffect(() => { if (open) { input.current?.focus(); } }, [open]);
  useEffect(() => { if (out.current) out.current.scrollTop = out.current.scrollHeight; }, [L.log.length, hist.length, open]);
  const echo = (lines) => setHist((h) => [...h, ...lines.map((text) => ({ t: Date.now(), src: 'console', text }))]);
  const find = (q) => L.games.find((g) => matches(q, g.title) || g.id === q);
  const run = (line) => {
    const [verb, ...rest] = line.trim().split(/\s+/);
    const arg = rest.join(' ');
    echo([`] ${line}`]);
    if (!verb) return;
    if (verb === 'help') echo(['play [game]   start the selected game, or one by name', 'stop          stop the running game', 'select <game> jump to a game', 'runner <name> switch the selected game’s runner', 'runners       list runners', 'verify        verify the selected game’s files', 'clear         clear this console']);
    else if (verb === 'clear') { setHist([]); setClearedAt(Date.now()); }
    else if (verb === 'runners') echo(L.runners.map((r) => `${r.id.padEnd(9)} ${r.name}`));
    else if (verb === 'play') { const g = arg ? find(arg) : L.game; if (!g) echo([`no game matches "${arg}"`]); else { L.actions.select(g.id); L.actions.play(g.id); } }
    else if (verb === 'stop') L.running ? L.actions.stop() : echo(['nothing is running']);
    else if (verb === 'select') { const g = find(arg); g ? L.actions.select(g.id) : echo([`no game matches "${arg}"`]); }
    else if (verb === 'verify') L.actions.verify(L.game.id);
    else if (verb === 'runner') {
      const r = L.runners.find((x) => x.id === arg || matches(arg, x.name));
      if (!r) echo([`unknown runner "${arg}", try: runners`]);
      else if (L.game.runner === 'native') echo([`${L.game.title} runs natively`]);
      else { L.actions.setRunner(L.game.id, r.id); echo([`${L.game.title} → ${r.name}`]); }
    } else echo([`unknown command "${verb}", try: help`]);
  };
  const lines = [...L.log.filter((l) => l.t > clearedAt), ...hist].sort((a, b) => a.t - b.t);
  const last = L.log[L.log.length - 1];
  return (
    <div className={`con ${open ? 'is-open' : ''}`}>
      {open && (
        <div className="con-out" ref={out} role="log" aria-label="Console output">
          {lines.map((l, i) => (
            <div key={i} className={`con-line src-${l.src}`}><span className="con-t">{clock(l.t)}</span><span className="con-src">{l.src}</span><span className="con-text">{l.text}</span></div>
          ))}
        </div>
      )}
      {open && (
        <form className="con-in" onSubmit={(e) => { e.preventDefault(); run(cmd); setCmd(''); }}>
          <span aria-hidden>]</span>
          <input ref={input} value={cmd} onChange={(e) => setCmd(e.target.value.replace(/[`~]/g, ''))} aria-label="Console command" placeholder="help" spellCheck={false} autoComplete="off" />
        </form>
      )}
      <div className="con-bar">
        <button type="button" className="con-toggle" aria-expanded={open} onClick={() => setOpen(!open)} title="Console ( ` )">
          {open ? <ChevronDown size={13} /> : <ChevronUp size={13} />}Console <kbd>`</kbd>
        </button>
        {L.running && <span className="con-job is-live">{L.games.find((x) => x.id === L.running.id)?.title} <b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="con-job">
            <span className="con-meter" aria-hidden><span style={{ width: `${j.progress * 100}%` }} /></span>
            {j.label} <b>{Math.round(j.progress * 100)}%</b>
          </span>
        ))}
        {!open && last && <span className="con-last">{last.src}: {last.text}</span>}
      </div>
    </div>
  );
}

export default function D4() {
  const L = useLauncher({ initial: 'kcd2' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;

  return (
    <div className="k d4">
      <header className="mb">
        <span className="mb-brand">Platypus</span>
        <nav className="mb-tabs" aria-label="Sections">
          {['Library', 'Downloads', 'Runners', 'Plugins'].map((t) => (
            <button key={t} type="button" aria-current={t === 'Library' ? 'page' : undefined} onClick={() => t !== 'Library' && L.actions.say(`${t} isn’t part of this mockup`, 'info')}>{t}</button>
          ))}
        </nav>
        <button type="button" className="mb-cmd" onClick={() => L.actions.openPalette()}><Search size={13} />Find or run<kbd>Ctrl K</kbd></button>
        <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle properties (Ctrl I)" onClick={L.actions.toggleInspector}><PanelRight size={15} /></button>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine etched" aria-label="Library">
          <div className="spine-tools">
            <label className="field"><Search size={13} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <div className="seg" role="radiogroup" aria-label="Store">
              {['all', 'steam', 'gog', 'epic', 'local'].map((s) => (
                <button key={s} type="button" role="radio" aria-checked={L.storeFilter === s} className={L.storeFilter === s ? 'is-on' : ''} onClick={() => L.actions.setStoreFilter(s)} title={s === 'all' ? 'All stores' : STORES[s].name}>{s === 'all' ? 'All' : <StoreGlyph store={s} size={11} />}</button>
              ))}
            </div>
          </div>
          <div className="gl-head"><span>Game</span><span>Status</span></div>
          <ul className="gl">
            {L.visible.map((x) => {
              const st = status(L, x);
              return (
                <li key={x.id}>
                  <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`gl-row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                    <Img src={artOf(x).header} className="gl-ico" />
                    <span className="gl-title">{x.title}</span>
                    <span className="gl-stores">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={9} />)}</span>
                    <span className={`gl-st ${st.cls}`}>{st.text}</span>
                  </button>
                </li>
              );
            })}
            {L.visible.length === 0 && <li className="gl-empty">No game matches “{L.query}”.</li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero etched">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <GameLogo g={g} className="hero-logo" />
          </div>
          <div className="strip etched">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={14} className="spin" /> : <Square size={11} fill="currentColor" />}{run.phase === 'starting' ? 'Starting' : 'Stop'}<span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={13} fill="currentColor" />Play</button>
            ) : (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}>Install<span className="play-t">{copy.size}</span></button>
            )}
            <div className="strip-src">
              <span className="strip-lbl">Copy</span>
              {g.stores.length > 1 ? (
                <div className="seg" role="radiogroup" aria-label="Launch from">
                  {g.stores.map((s) => (
                    <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => L.actions.setSource(g.id, s.store)}><StoreGlyph store={s.store} size={11} />{STORES[s.store].name}{!s.installed && <i>not installed</i>}</button>
                  ))}
                </div>
              ) : <span className="strip-one"><StoreGlyph store={g.stores[0].store} size={11} />{STORES[g.stores[0].store].name}</span>}
            </div>
            <div className="strip-src">
              <span className="strip-lbl">Cloud</span>
              {g.sync === 'conflict' ? (
                <button type="button" className="strip-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={13} />Conflict, resolve</button>
              ) : <span className="strip-one"><SyncDot state={g.sync} />{SYNC_LABEL[g.sync]}</span>}
            </div>
            <dl className="strip-kv">
              <dt>Played</dt><dd>{Math.round(g.hours)} h</dd>
              <dt>Last</dt><dd>{ago(g.lastAgo)}</dd>
              <dt>ProtonDB</dt><dd className={`tier-${g.tier}`}>{TIER_LABEL[g.tier]}</dd>
            </dl>
          </div>
          <div className="readout etched">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp etched" aria-label="Properties">
            <div className="insp-head">Properties: {g.title}</div>
            <div className="insp-scroll"><InspectorFields L={L} Section={Section} /></div>
          </aside>
        )}
      </div>

      <Console L={L} />
      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
