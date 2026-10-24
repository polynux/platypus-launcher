import React, { useState } from 'react';
import '@fontsource-variable/big-shoulders-display';
import '@fontsource-variable/schibsted-grotesk';
import './d5.css';
import { Play, Square, Search, PanelRight, AlertTriangle, Loader } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, Tabs, readoutTabs, ReadoutBody, elapsed } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ title, aside, children }) {
  return (
    <section className="isec">
      <h3 className="isec-h">{title}{aside && <span>{aside}</span>}</h3>
      {children}
    </section>
  );
}

const initial = (t) => {
  const c = t[0].toUpperCase();
  return /[A-Z]/.test(c) ? c : '#';
};

export default function D5() {
  const L = useLauncher({ initial: 'hades2', sort: 'title' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;

  // Group by first letter for the index spine.
  const groups = [];
  for (const x of L.visible) {
    const k = initial(x.title);
    if (!groups.length || groups[groups.length - 1].k !== k) groups.push({ k, items: [] });
    groups[groups.length - 1].items.push(x);
  }
  const titleSize = g.title.length > 26 ? 'is-long' : g.title.length > 16 ? 'is-mid' : '';

  return (
    <div className="k d5">
      <div className="body">
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-k" onClick={() => L.actions.openPalette()} aria-label="Command palette"><kbd>Ctrl K</kbd></button>
          </div>
          <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search the library" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
          <div className="spine-sort" role="radiogroup" aria-label="Order">
            {[['title', 'A–Z'], ['last', 'Recent'], ['hours', 'Most played']].map(([id, label]) => (
              <button key={id} type="button" role="radio" aria-checked={L.sort === id} className={L.sort === id ? 'is-on' : ''} onClick={() => L.actions.setSort(id)}>{label}</button>
            ))}
          </div>
          <div className="index">
            {L.sort === 'title'
              ? groups.map((gr) => (
                  <div key={gr.k} className="index-group">
                    <span className="index-letter" aria-hidden>{gr.k}</span>
                    <ul>{gr.items.map((x) => <Row key={x.id} L={L} x={x} sel={x.id === g.id} />)}</ul>
                  </div>
                ))
              : <ul className="index-flat">{L.visible.map((x) => <Row key={x.id} L={L} x={x} sel={x.id === g.id} />)}</ul>}
            {L.visible.length === 0 && <p className="index-empty">Nothing in the library matches “{L.query}”.</p>}
          </div>
        </aside>

        <main className={`main ${L.inspector ? 'has-insp' : ''}`}>
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
          </div>
          <div className="mast">
            <h1 className={`mast-title ${titleSize}`}>{g.title}</h1>
            <div className="mast-side">
              <div className="mast-row">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" aria-pressed={g.source === s.store} className={`mast-src ${g.source === s.store ? 'is-on' : ''}`} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={12} />{STORES[s.store].name}{!s.installed && <span className="mast-na">not installed</span>}
                  </button>
                ))}
                {g.sync === 'conflict' ? (
                  <button type="button" className="mast-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={13} />Save conflict, resolve</button>
                ) : <span className="mast-sync"><SyncDot state={g.sync} />{SYNC_LABEL[g.sync]}</span>}
              </div>
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={16} className="spin" /> : <Square size={13} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={16} fill="currentColor" /><span>Play</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-t">{copy.size}</span></button>
              )}
            </div>
          </div>
          <div className="colophon"><dl>
            <div><dt>Studio</dt><dd>{g.dev}</dd></div>
            <div><dt>Released</dt><dd>{g.year}</dd></div>
            <div><dt>Played</dt><dd>{Math.round(g.hours)} hours</dd></div>
            <div><dt>Last</dt><dd>{ago(g.lastAgo)}</dd></div>
            <div><dt>Runs on</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
            <div><dt>ProtonDB</dt><dd>{TIER_LABEL[g.tier]}</dd></div>
            </dl>
            <button type="button" className={`insp-toggle ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} onClick={L.actions.toggleInspector} title="Ctrl I"><PanelRight size={14} />{L.inspector ? 'Hide settings' : 'Settings'}</button>
          </div>
          <div className="lower">
            <div className="readout">
              <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
              <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
            </div>
            {L.inspector && (
              <aside className="insp" aria-label="Launch settings">
                <InspectorFields L={L} Section={Section} />
              </aside>
            )}
          </div>
        </main>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live">Playing {L.games.find((x) => x.id === L.running.id)?.title} <b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>
            {j.label} <b>{Math.round(j.progress * 100)}%</b>
          </span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">Nothing running</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}

function Row({ L, x, sel }) {
  return (
    <li>
      <button type="button" data-game-row={x.id} aria-current={sel} className={`irow ${sel ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
        <span className="irow-title">{x.title}</span>
        <span className="irow-stores">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}</span>
        <SyncDot state={x.sync} />
      </button>
    </li>
  );
}
