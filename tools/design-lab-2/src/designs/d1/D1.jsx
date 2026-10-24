import React, { useState } from 'react';
import '@fontsource-variable/geist';
import '@fontsource-variable/geist-mono';
import './d1.css';
import { Play, Square, Search, PanelRight, Command, ChevronRight, Cloud, CloudOff, AlertTriangle, RefreshCw, HardDrive, Loader } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, agoShort, useTick } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed, storeCounts } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ id, title, aside, children }) {
  const [open, setOpen] = useState(true);
  return (
    <section className={`isec ${open ? '' : 'is-closed'}`}>
      <h3>
        <button type="button" className="isec-h" aria-expanded={open} onClick={() => setOpen(!open)}>
          <ChevronRight size={12} className="isec-chev" />
          <span>{title}</span>
          {aside && <span className="isec-aside">{aside}</span>}
        </button>
      </h3>
      {open && <div className="isec-body">{children}</div>}
    </section>
  );
}

function SyncButton({ L, g }) {
  const icon = { synced: <Cloud size={13} />, syncing: <RefreshCw size={13} className="spin" />, conflict: <AlertTriangle size={13} />, local: <HardDrive size={13} />, off: <CloudOff size={13} /> }[g.sync];
  if (g.sync === 'conflict')
    return <button type="button" className="strip-sync is-conflict" onClick={() => L.actions.openConflict(g.id)}>{icon}<span>Save conflict</span><span className="strip-sync-cta">Resolve</span></button>;
  return <span className={`strip-sync sync-${g.sync}`}>{icon}<span>{SYNC_LABEL[g.sync]}</span></span>;
}

export default function D1() {
  const L = useLauncher();
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const counts = storeCounts(L.games);
  const installedList = L.visible.filter(isInstalled);
  const otherList = L.visible.filter((x) => !isInstalled(x));

  const row = (x) => (
    <li key={x.id}>
      <button type="button" data-game-row={x.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} aria-current={x.id === g.id} onClick={() => L.actions.select(x.id)}>
        <Img src={artOf(x).header} className="row-ico" />
        <span className="row-title">{x.title}</span>
        {L.running?.id === x.id && <span className="row-live" aria-label="Running" />}
        <span className="row-stores">{x.stores.map((s) => <span key={s.store} className={s.store === x.source ? 'is-src' : ''}><StoreGlyph store={s.store} size={10} /></span>)}</span>
        <SyncDot state={x.sync} />
      </button>
    </li>
  );

  return (
    <div className="k d1">
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden />Platypus</div>
        <nav className="tb-crumb" aria-label="Location"><span>Library</span><ChevronRight size={12} /><span className="tb-crumb-cur">{g.title}</span></nav>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}>
          <Search size={13} /><span>Run a command or jump to a game</span><kbd>Ctrl K</kbd>
        </button>
        <div className="tb-right">
          <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle inspector (Ctrl I)" title="Inspector · Ctrl I" onClick={L.actions.toggleInspector}><PanelRight size={15} /></button>
        </div>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-search">
            <Search size={13} />
            <input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter library" aria-label="Filter library" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} />
            <kbd>/</kbd>
          </div>
          <div className="spine-stores" role="radiogroup" aria-label="Store">
            {['all', 'steam', 'gog', 'epic', 'local'].map((s) => (
              <button key={s} type="button" role="radio" aria-checked={L.storeFilter === s} className={L.storeFilter === s ? 'is-on' : ''} onClick={() => L.actions.setStoreFilter(s)} title={s === 'all' ? 'All stores' : STORES[s].name}>
                {s === 'all' ? 'All' : <StoreGlyph store={s} size={11} />}<span className="spine-count">{counts[s] || 0}</span>
              </button>
            ))}
          </div>
          <div className="spine-list">
            {installedList.length > 0 && <h2 className="spine-h">Installed <span>{installedList.length}</span></h2>}
            <ul>{installedList.map(row)}</ul>
            {otherList.length > 0 && <h2 className="spine-h">In your library <span>{otherList.length}</span></h2>}
            <ul>{otherList.map(row)}</ul>
            {L.visible.length === 0 && <p className="spine-empty">No game matches “{L.query}”. <button type="button" onClick={() => L.actions.setQuery('')}>Clear filter</button></p>}
          </div>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <GameLogo g={g} className="hero-logo" />
          </div>
          <div className="strip">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>
                {run.phase === 'starting' ? <Loader size={14} className="spin" /> : <Square size={12} fill="currentColor" />}
                <span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span>
                <span className="play-time">{elapsed(Date.now() - run.startedAt)}</span>
              </button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={13} fill="currentColor" /><span>Play</span></button>
            ) : (
              <button type="button" className="play is-install" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-time">{copy.size}</span></button>
            )}
            {g.stores.length > 1 && (
              <div className="src" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={`src-opt ${g.source === s.store ? 'is-on' : ''}`} onClick={() => L.actions.setSource(g.id, s.store)} title={`${STORES[s.store].name} · ${s.installed ? 'installed' : 'not installed'} · ${s.version}`}>
                    <StoreGlyph store={s.store} size={12} />
                    <span>{STORES[s.store].name}</span>
                    {s.installed && <span className="src-inst" aria-label="installed" />}
                  </button>
                ))}
              </div>
            )}
            {g.stores.length === 1 && <span className="src-single"><StoreGlyph store={g.stores[0].store} size={12} />{STORES[g.stores[0].store].name}</span>}
            <SyncButton L={L} g={g} />
            <dl className="strip-meta">
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Last</dt><dd>{ago(g.lastAgo)}</dd></div>
              <div><dt>Runner</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
              <div><dt>ProtonDB</dt><dd className={`tier tier-${g.tier}`}>{TIER_LABEL[g.tier]}</dd></div>
              <div><dt>On disk</dt><dd>{copy.installed ? copy.size : '—'}</dd></div>
            </dl>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp" aria-label="Inspector">
            <div className="insp-head">
              <span className="insp-title">Launch settings</span>
              <span className="insp-sub">{g.title}</span>
            </div>
            <div className="insp-scroll"><InspectorFields L={L} Section={Section} /></div>
          </aside>
        )}
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />{L.games.find((x) => x.id === L.running.id)?.title} · {L.running.phase} · {elapsed(Date.now() - L.running.startedAt)}</span>}
        {L.jobs.map((j) => (
          <span key={j.id} className={`tele-item tele-${j.kind}`}>
            <span className="tele-bar" style={{ '--p': j.progress }} aria-hidden />
            <span className="tele-k">{j.kind}</span>
            <span className="tele-l">{j.label}</span>
            <span className="tele-p">{Math.round(j.progress * 100)}%</span>
          </span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item tele-idle">idle</span>}
        <span className="tele-right">{L.games.length} games · core 0.9.0</span>
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
