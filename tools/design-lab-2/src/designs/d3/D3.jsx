import React, { createContext, useContext, useEffect, useRef, useState } from 'react';
import '@fontsource-variable/archivo/wdth.css';
import '@fontsource-variable/azeret-mono';
import './d3.css';
import { Play, Square, Search, Cpu, FlaskConical, Braces, TerminalSquare, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, X } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, vivid } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

const DOCK = [
  { id: 'runner', label: 'Runner and wrappers', icon: Cpu, shows: ['runner', 'layers', 'wrappers'] },
  { id: 'recipes', label: 'Fix recipes', icon: FlaskConical, shows: ['recipes'] },
  { id: 'env', label: 'Environment', icon: Braces, shows: ['env', 'args'] },
  { id: 'cmd', label: 'Command preview', icon: TerminalSquare, shows: ['cmd'] },
];

const PaneCtx = createContext('runner');

// Show only the sections the docked pane owns.
function Section({ id, title, aside, children }) {
  const pane = useContext(PaneCtx);
  if (!DOCK.find((d) => d.id === pane)?.shows.includes(id)) return null;
  return (
    <section className="dsec">
      <h3 className="dsec-h">{title}{aside && <span>{aside}</span>}</h3>
      {children}
    </section>
  );
}

export default function D3() {
  const L = useLauncher({ initial: 'cyberpunk' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [pane, setPane] = useState('runner');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const sig = vivid(tone) || '#d8d8d4';

  // Reel change: replay the wipe whenever the game changes.
  const [reel, setReel] = useState(0);
  const prev = useRef(g.id);
  useEffect(() => { if (prev.current !== g.id) { prev.current = g.id; setReel((n) => n + 1); } }, [g.id]);

  const dockClick = (id) => {
    if (L.inspector && pane === id) L.actions.setInspector(false);
    else { setPane(id); L.actions.setInspector(true); }
  };
  const syncIcon = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff }[g.sync];
  const SyncIcon = syncIcon;

  return (
    <div className="k d3" style={{ '--sig': sig }}>
      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-brand">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-cmd" onClick={() => L.actions.openPalette()} aria-label="Command palette"><kbd>Ctrl K</kbd></button>
          </div>
          <label className="spine-search"><Search size={13} /><input data-search value={L.query} placeholder="Search" onChange={(e) => L.actions.setQuery(e.target.value)} onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
          <ul className="reel">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`frame ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="frame-img" />
                  <span className="frame-title">{x.title}</span>
                  <span className="frame-meta">
                    {x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={9} />)}
                    <SyncDot state={x.sync} />
                  </span>
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="spine-empty">No match for “{L.query}”.</li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="screen">
            <div className="scope">
              <Img key={`${g.id}-${reel}`} src={a.hero} className="scope-img" />
              <GameLogo g={g} className="scope-logo" />
            </div>
          </div>
          <div className="strip">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={12} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="tc">{elapsed(Date.now() - run.startedAt)}</span></button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>Play</span></button>
            ) : (
              <button type="button" className="play is-install" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="tc">{copy.size}</span></button>
            )}
            <div className="src" role="radiogroup" aria-label="Launch from">
              {g.stores.map((s) => (
                <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                  <StoreGlyph store={s.store} size={13} /><span>{STORES[s.store].name}</span>{!s.installed && <span className="src-na">not installed</span>}
                </button>
              ))}
            </div>
            {g.sync === 'conflict' ? (
              <button type="button" className="cloud is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict<span className="cloud-cta">Resolve</span></button>
            ) : (
              <span className={`cloud c-${g.sync}`}><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>
            )}
            <dl className="meta">
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Last</dt><dd>{ago(g.lastAgo)}</dd></div>
              <div><dt>ProtonDB</dt><dd>{TIER_LABEL[g.tier]}</dd></div>
            </dl>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="dock-panel" aria-label={DOCK.find((d) => d.id === pane)?.label}>
            <div className="dock-head">
              <span>{DOCK.find((d) => d.id === pane)?.label}</span>
              <span className="dock-runner">{runnerOf(g.runner)?.name}</span>
              <button type="button" className="icon-btn" aria-label="Close panel" onClick={() => L.actions.setInspector(false)}><X size={14} /></button>
            </div>
            <div className="dock-scroll"><PaneCtx.Provider value={pane}><InspectorFields L={L} Section={Section} /></PaneCtx.Provider></div>
          </aside>
        )}
        <nav className="rail" aria-label="Inspector panels">
          {DOCK.map((d) => {
            const Icon = d.icon;
            const on = L.inspector && pane === d.id;
            return (
              <button key={d.id} type="button" className={`rail-btn ${on ? 'is-on' : ''}`} aria-pressed={on} title={d.label} aria-label={d.label} onClick={() => dockClick(d.id)}>
                <Icon size={17} strokeWidth={1.6} />
                {d.id === 'recipes' && g.recipes.length > 0 && <span className="rail-badge">{g.recipes.length}</span>}
              </button>
            );
          })}
        </nav>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="rec" />{L.games.find((x) => x.id === L.running.id)?.title}<span className="tc">{elapsed(Date.now() - L.running.startedAt)}</span></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item">
            <span className="tele-k">{j.kind}</span>{j.label}
            <span className="tele-meter" aria-hidden><span style={{ width: `${j.progress * 100}%` }} /></span>
            <span className="tc">{String(Math.round(j.progress * 100)).padStart(2, '0')}%</span>
          </span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">Idle</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
