import React, { useState } from 'react';
import '@fontsource-variable/big-shoulders-display';
import '@fontsource-variable/schibsted-grotesk';
import '@fontsource-variable/geist-mono';
import './d7.css';
import { Play, Square, Search, AlertTriangle, Loader, Settings2 } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, toneAt } from '../../core/launcher.js';
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

const SORTS = [
  { id: 'title', label: 'A to Z' },
  { id: 'last', label: 'Recently played' },
  { id: 'hours', label: 'Most played' },
];

export default function D7() {
  const L = useLauncher({ initial: 'hades2', sort: 'title' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [level, setLevel] = useState('basic');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);

  // The masthead takes a pale wash of the banner; ink stays dark on it.
  const vars = {
    '--band': toneAt(tone, 0.8, 0.83, '#ecebe5'),
    '--band-deep': toneAt(tone, 0.8, 0.7, '#d8d6cc'),
    '--ink': toneAt(tone, 0.5, 0.1, '#141414'),
    '--sig': toneAt(tone, 0.8, 0.62, '#ecebe5'),
  };

  const groups = [];
  for (const x of L.visible) {
    const k = L.sort === 'title' ? (/[a-z]/i.test(x.title[0]) ? x.title[0].toUpperCase() : '#') : null;
    if (!groups.length || groups[groups.length - 1].k !== k) groups.push({ k, items: [] });
    groups[groups.length - 1].items.push(x);
  }
  const titleSize = g.title.length > 26 ? 'is-long' : g.title.length > 15 ? 'is-mid' : '';

  return (
    <div className="k d7" style={vars}>
      <div className="body">
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-k" onClick={() => L.actions.openPalette()}><Search size={13} />Find<kbd>Ctrl K</kbd></button>
          </div>
          <div className="spine-tools">
            <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search the library" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <FilterMenu L={L} label="Filter" icon={false} sorts={SORTS} />
          </div>
          <FilterChips L={L} />
          <div className="index">
            {groups.map((gr, i) => (
              <div key={gr.k ?? i} className={`index-group ${gr.k ? '' : 'is-flat'}`}>
                {gr.k && <span className="index-letter" aria-hidden>{gr.k}</span>}
                <ul>
                  {gr.items.map((x) => (
                    <li key={x.id}>
                      <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`irow ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                        <Img src={artOf(x).cover} className="irow-cover" />
                        <span className="irow-title">{x.title}</span>
                        <span className="irow-stores">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}</span>
                        <SyncDot state={x.sync} />
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
            {L.visible.length === 0 && <p className="index-empty">Nothing matches. <button type="button" onClick={L.actions.resetFilters}>Clear filters</button></p>}
          </div>
        </aside>

        <main className={`main ${L.inspector ? 'has-insp' : ''}`}>
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
          </div>
          <div className="mast">
            <Img key={a.cover} src={a.cover} className="mast-cover" />
            <div className="mast-body">
              <h1 className={`mast-title ${titleSize}`}>{g.title}</h1>
              <p className="mast-by">{g.dev}, {g.year}. {g.genre}.</p>
            </div>
            <div className="mast-side">
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={18} className="spin" /> : <Square size={15} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={18} fill="currentColor" /><span>Play</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-t">{copy.size}</span></button>
              )}
              <div className="mast-row" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={`mast-src ${g.source === s.store ? 'is-on' : ''}`} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={12} />{STORES[s.store].name}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <div className="colophon">
            <dl>
              <div><dt>Played</dt><dd>{Math.round(g.hours)} hours</dd></div>
              <div><dt>Last</dt><dd>{ago(g.lastAgo)}</dd></div>
              <div><dt>Runs with</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
              <div><dt>ProtonDB</dt><dd>{TIER_LABEL[g.tier]}</dd></div>
              <div><dt>Saves</dt><dd>{g.sync === 'conflict' ? <button type="button" className="col-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={13} />Conflict, resolve</button> : <span className="col-sync"><SyncDot state={g.sync} />{SYNC_LABEL[g.sync]}</span>}</dd></div>
            </dl>
            <button type="button" className={`insp-toggle ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} onClick={L.actions.toggleInspector} title="Ctrl I"><Settings2 size={15} />{L.inspector ? 'Hide settings' : 'Settings'}</button>
          </div>
          <div className="lower">
            <div className="readout">
              <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
              <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
            </div>
            {L.inspector && (
              <aside className="insp" aria-label="Launch settings">
                <div className="insp-tabs" role="tablist" aria-label="Settings level">
                  {[['basic', 'Basics'], ['advanced', 'Advanced']].map(([id, label]) => (
                    <button key={id} type="button" role="tab" aria-selected={level === id} className={level === id ? 'is-on' : ''} onClick={() => setLevel(id)}>{label}</button>
                  ))}
                </div>
                <InspectorFields L={L} Section={Section} level={level} />
              </aside>
            )}
          </div>
        </main>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live">Playing {L.games.find((x) => x.id === L.running.id)?.title} <b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label} <b>{Math.round(j.progress * 100)}%</b></span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">Nothing running</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
