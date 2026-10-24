import React, { useState } from 'react';
import '@fontsource-variable/mona-sans/wdth.css';
import '@fontsource-variable/geist-mono';
import './d8.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, SlidersHorizontal, ChevronLeft, X } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, toneAt, vivid } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, SimpleSettings, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed, FilterMenu, FilterChips } from '../../core/kit.jsx';
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

export default function D8() {
  const L = useLauncher({ initial: 'stardew' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [advanced, setAdvanced] = useState(false);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const vars = {
    '--acc': vivid(tone, 0.68, 0.65) || '#f2f2f2',
    '--ground': toneAt(tone, 0.22, 0.085, '#151518'),
    '--ground-2': toneAt(tone, 0.2, 0.115, '#1c1c20'),
    '--ground-3': toneAt(tone, 0.18, 0.15, '#26262b'),
  };

  // Lounge groups: what you were just playing, then the rest.
  const sorted = L.visible;
  const recent = L.sort === 'recent' && !L.query && L.stateFilter === 'all' ? sorted.filter(isInstalled).slice(0, 3) : [];
  const rest = sorted.filter((x) => !recent.includes(x));
  const row = (x, big) => (
    <li key={x.id}>
      <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${big ? 'is-big' : ''} ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
        <Img src={artOf(x).cover} className="row-cover" />
        <span className="row-text">
          <span className="row-title">{x.title}</span>
          <span className="row-sub">{L.running?.id === x.id ? <em className="row-live">Playing now</em> : isInstalled(x) ? `Played ${ago(x.lastAgo)}` : 'Not installed'}</span>
        </span>
        {x.sync === 'conflict' && <AlertTriangle size={14} className="row-warn" aria-label="Save conflict" />}
      </button>
    </li>
  );

  return (
    <div className="k d8" style={vars}>
      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-k" onClick={() => L.actions.openPalette()} aria-label="Search and commands"><Search size={15} /><kbd>Ctrl K</kbd></button>
          </div>
          <div className="spine-tools">
            <label className="spine-search"><Search size={15} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search your games" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <FilterMenu L={L} label="" align="right" />
          </div>
          <FilterChips L={L} />
          <div className="spine-list">
            {recent.length > 0 && <><h2 className="spine-h">Continue playing</h2><ul>{recent.map((x) => row(x, true))}</ul></>}
            {rest.length > 0 && <h2 className="spine-h">{recent.length ? 'All games' : `${rest.length} games`}</h2>}
            <ul>{rest.map((x) => row(x, false))}</ul>
            {L.visible.length === 0 && <div className="spine-empty"><p>No games match.</p><button type="button" className="btn" onClick={L.actions.resetFilters}>Show all games</button></div>}
          </div>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <div className="hero-foot">
              <GameLogo g={g} className="hero-logo" />
              <div className="hero-actions">
                {run ? (
                  <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={18} className="spin" /> : <Square size={14} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting…' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
                ) : copy.installed ? (
                  <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={18} fill="currentColor" /><span>Play</span></button>
                ) : (
                  <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-t">{copy.size}</span></button>
                )}
                <button type="button" className={`ghost ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} onClick={L.actions.toggleInspector} title="Ctrl I"><SlidersHorizontal size={16} />Settings</button>
              </div>
            </div>
          </div>
          <div className="strip">
            <div className="pill-group" role="radiogroup" aria-label="Launch from">
              {g.stores.map((s) => (
                <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={`pill ${g.source === s.store ? 'is-on' : ''}`} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                  <StoreGlyph store={s.store} size={13} />{STORES[s.store].name}{!s.installed && <span className="pill-na">not installed</span>}
                </button>
              ))}
            </div>
            {g.sync === 'conflict' ? (
              <button type="button" className="pill is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Your saves differ. Choose one</button>
            ) : (
              <span className={`pill is-static s-${g.sync}`}><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>
            )}
            <span className="facts">
              <span><b>{Math.round(g.hours)} h</b> played</span>
              <span><b>{TIER_LABEL[g.tier]}</b> on ProtonDB</span>
              <span><b>{copy.size}</b></span>
            </span>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} compact /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp" aria-label="Game settings">
            <div className="insp-head">
              {advanced ? (
                <button type="button" className="back" onClick={() => setAdvanced(false)}><ChevronLeft size={16} />Settings</button>
              ) : <span className="insp-title">Settings</span>}
              <button type="button" className="icon-btn" aria-label="Close settings" onClick={() => L.actions.setInspector(false)}><X size={15} /></button>
            </div>
            <div className="insp-scroll">
              {advanced ? (
                <>
                  <p className="insp-note">Advanced settings change exactly what Platypus runs. The command at the bottom updates as you go.</p>
                  <InspectorFields L={L} Section={Section} level="advanced" />
                </>
              ) : (
                <SimpleSettings L={L} onAdvanced={() => setAdvanced(true)} />
              )}
            </div>
          </aside>
        )}
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title} · {elapsed(Date.now() - L.running.startedAt)}</span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item">
            <span className="tele-bar" aria-hidden><span style={{ width: `${j.progress * 100}%` }} /></span>
            {j.label}
          </span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">Everything is up to date</span>}
      </footer>

      <Palette L={L} placeholder="Search games, settings and actions" />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
