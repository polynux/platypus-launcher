import React, { useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import './d9.css';
import { Play, Square, Search, PanelRight, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, LibraryBig, Download, CircleAlert, Plus, ChevronDown } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, stateMatch } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed, Select } from '../../core/kit.jsx';
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

export default function D9() {
  const L = useLauncher({ initial: 'witcher3' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [showAdv, setShowAdv] = useState(false);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const SyncIcon = SYNC_ICON[g.sync];

  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const attention = L.games.filter((x) => stateMatch(x, 'attention')).length;
  const scopes = [
    { id: 'all', label: 'All games', icon: LibraryBig, on: L.stateFilter === 'all' && !L.stores.length, go: () => { L.actions.setStores([]); L.actions.setStateFilter('all'); } },
    { id: 'installed', label: 'Installed', icon: Download, on: L.stateFilter === 'installed' && !L.stores.length, go: () => { L.actions.setStores([]); L.actions.setStateFilter('installed'); } },
    { id: 'attention', label: 'Needs attention', icon: CircleAlert, badge: attention, on: L.stateFilter === 'attention', go: () => { L.actions.setStores([]); L.actions.setStateFilter('attention'); } },
  ];
  const pickStore = (s, e) => {
    L.actions.setStateFilter('all');
    if (e.shiftKey || e.ctrlKey) L.actions.toggleStore(s);
    else L.actions.setStores(L.stores.length === 1 && L.stores[0] === s ? [] : [s]);
  };
  const scopeTitle = L.stores.length ? L.stores.map((s) => STORES[s].name).join(' + ') : scopes.find((x) => x.on)?.label || 'Library';

  return (
    <div className="k d9">
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden>P</span>Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>Search or run a command</span><kbd>Ctrl K</kbd></button>
        <div className="tb-right">
          <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle settings (Ctrl I)" onClick={L.actions.toggleInspector}><PanelRight size={16} /></button>
        </div>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <nav className="rail" aria-label="Library scope">
          {scopes.map((s) => {
            const Icon = s.icon;
            return (
              <button key={s.id} type="button" className={`rail-btn ${s.on ? 'is-on' : ''}`} aria-pressed={s.on} title={s.label} aria-label={s.label} onClick={s.go}>
                <Icon size={18} strokeWidth={1.7} />
                {s.badge > 0 && <span className="rail-badge">{s.badge}</span>}
              </button>
            );
          })}
          <span className="rail-sep" aria-hidden />
          {Object.keys(STORES).filter((s) => counts[s]).map((s) => {
            const on = L.stores.includes(s);
            return (
              <button key={s} type="button" className={`rail-btn rail-store ${on ? 'is-on' : ''}`} aria-pressed={on} title={`${STORES[s].name} · ${counts[s]} games (Shift-click to combine)`} aria-label={`${STORES[s].name}, ${counts[s]} games`} onClick={(e) => pickStore(s, e)}>
                <StoreGlyph store={s} size={18} />
                <span className="rail-count">{counts[s]}</span>
              </button>
            );
          })}
          <button type="button" className="rail-btn rail-add" title="Add a store" aria-label="Add a store" onClick={() => L.actions.say('Stores are added as plugins from the Plugins view', 'info')}><Plus size={17} /></button>
        </nav>

        <aside className="list" aria-label="Games">
          <div className="list-head">
            <h2>{scopeTitle}</h2>
            <span className="list-n">{L.visible.length}</span>
          </div>
          <div className="list-tools">
            <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <Select label="Sort" className="list-sort" value={L.sort} onChange={L.actions.setSort} options={[{ value: 'recent', label: 'Recent' }, { value: 'title', label: 'Title' }, { value: 'hours', label: 'Most played' }]} />
          </div>
          <ul className="list-body">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="row-art" />
                  <span className="row-text">
                    <span className="row-title">{x.title}</span>
                    <span className="row-sub">{L.running?.id === x.id ? <em>Playing</em> : isInstalled(x) ? ago(x.lastAgo) : 'Not installed'}</span>
                  </span>
                  <span className="row-end">
                    <span className="row-stores">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}</span>
                    <SyncDot state={x.sync} />
                  </span>
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="list-empty">Nothing here yet. <button type="button" onClick={L.actions.resetFilters}>Show all games</button></li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <GameLogo g={g} className="hero-logo" />
          </div>
          <div className="strip">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={12} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>Play</span></button>
            ) : (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-t">{copy.size}</span></button>
            )}
            <div className="from">
              <span className="from-k">Launch from</span>
              {g.stores.length > 1 ? (
                <Select label="Launch from" className="from-sel" value={g.source} onChange={(v) => L.actions.setSource(g.id, v)} options={g.stores.map((s) => ({ value: s.store, label: STORES[s.store].name, hint: s.installed ? 'installed' : 'not installed' }))} />
              ) : <span className="from-one"><StoreGlyph store={copy.store} size={13} />{STORES[copy.store].name}</span>}
            </div>
            {g.sync === 'conflict' ? (
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict<span>Resolve</span></button>
            ) : <span className={`sync s-${g.sync}`}><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
            <dl className="meta">
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Last played</dt><dd>{ago(g.lastAgo)}</dd></div>
              <div><dt>ProtonDB</dt><dd className={`tier-${g.tier}`}>{TIER_LABEL[g.tier]}</dd></div>
            </dl>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp" aria-label="Game settings">
            <div className="insp-scroll">
              <div className="insp-group">Essentials</div>
              <InspectorFields L={L} Section={Section} level="basic" />
              <button type="button" className="insp-more" aria-expanded={showAdv} onClick={() => setShowAdv(!showAdv)}>
                <ChevronDown size={14} className={showAdv ? 'is-up' : ''} />{showAdv ? 'Hide advanced settings' : 'Show advanced settings'}
                <span>{g.recipes.length + g.env.length} set</span>
              </button>
              {showAdv && <InspectorFields L={L} Section={Section} level="advanced" />}
            </div>
          </aside>
        )}
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}<b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label}<b>{Math.round(j.progress * 100)}%</b></span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">No background activity</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
