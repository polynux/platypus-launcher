import React, { useState } from 'react';
import '@fontsource-variable/bricolage-grotesque/opsz.css';
import '@fontsource-variable/geist-mono';
import './d10.css';
import { Play, Square, Search, PanelRight, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, X } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, agoShort, useTick, useArtTone, toneAt } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, Tabs, readoutTabs, ReadoutBody, elapsed, FilterMenu, FilterChips } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ title, aside, children }) {
  return (
    <section className="isec">
      <h3 className="isec-h">{title}{aside && <span>{aside}</span>}</h3>
      {children}
    </section>
  );
}

const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

export default function D10() {
  const L = useLauncher({ initial: 'nightreign' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [level, setLevel] = useState('basic');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  // One hue, four depths: the whole shell is cut from the game's colour.
  const vars = {
    '--t0': toneAt(tone, 0.42, 0.07, '#121214'),
    '--t1': toneAt(tone, 0.4, 0.1, '#18181b'),
    '--t2': toneAt(tone, 0.36, 0.14, '#202024'),
    '--t3': toneAt(tone, 0.32, 0.2, '#2b2b30'),
    '--t-line': toneAt(tone, 0.35, 0.2, '#2b2b30'),
    '--t-text': toneAt(tone, 0.25, 0.94, '#f1f1f1'),
    '--t-muted': toneAt(tone, 0.22, 0.7, '#a6a6a6'),
    '--t-faint': toneAt(tone, 0.2, 0.5, '#707070'),
    '--t-acc': toneAt(tone, 0.85, 0.68, '#f1f1f1'),
  };

  return (
    <div className="k d10" style={vars}>
      <header className="tb">
        <div className="tb-brand">Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={15} /><span>Find a game or a setting</span><kbd>Ctrl K</kbd></button>
        <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle settings (Ctrl I)" onClick={L.actions.toggleInspector}><PanelRight size={16} /></button>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-tools">
            <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <FilterMenu L={L} label="Filter" />
          </div>
          <FilterChips L={L} />
          <ul className="reel">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`frame ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="frame-img" />
                  <span className="frame-text">
                    <span className="frame-title">{x.title}</span>
                    <span className="frame-meta">
                      {x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}
                      <span>{L.running?.id === x.id ? 'playing' : isInstalled(x) ? agoShort(x.lastAgo) : 'not installed'}</span>
                    </span>
                  </span>
                  <SyncDot state={x.sync} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="spine-empty">No game matches. <button type="button" onClick={L.actions.resetFilters}>Clear filters</button></li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
          </div>
          <div className="strip">
            <div className="strip-title">
              <h1>{g.title}</h1>
              <div className="strip-meta">
                <span>{Math.round(g.hours)} h played</span>
                <span>Last played {ago(g.lastAgo)}</span>
                <span>{TIER_LABEL[g.tier]} on ProtonDB</span>
              </div>
            </div>
            <div className="strip-side">
              <div className="src" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={13} /><span>{STORES[s.store].name}</span>
                  </button>
                ))}
              </div>
              {g.sync === 'conflict' ? (
                <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Resolve save conflict</button>
              ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={17} className="spin" /> : <Square size={14} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={17} fill="currentColor" /><span>Play</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-t">{copy.size}</span></button>
              )}
            </div>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp" aria-label="Game settings">
            <div className="insp-head">
              <div className="insp-tabs" role="tablist" aria-label="Settings level">
                {[['basic', 'Basics'], ['advanced', 'Advanced']].map(([id, label]) => (
                  <button key={id} type="button" role="tab" aria-selected={level === id} className={level === id ? 'is-on' : ''} onClick={() => setLevel(id)}>{label}</button>
                ))}
              </div>
              <button type="button" className="icon-btn" aria-label="Close settings" onClick={() => L.actions.setInspector(false)}><X size={15} /></button>
            </div>
            <div className="insp-scroll" role="tabpanel">
              <InspectorFields L={L} Section={Section} level={level} />
              <p className="insp-foot">Runs with {runnerOf(g.runner)?.name}</p>
            </div>
          </aside>
        )}
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}<b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label}<b>{Math.round(j.progress * 100)}%</b></span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">Nothing running</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
