import React, { useState } from 'react';
import '@fontsource-variable/figtree';
import '@fontsource-variable/geist-mono';
import './d6.css';
import { Play, Square, Search, PanelRight, ChevronRight, Cloud, CloudOff, AlertTriangle, RefreshCw, HardDrive, Loader, Command } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, agoShort, useTick, useArtTone, vivid, toneAt } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed, FilterMenu, FilterChips } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ title, aside, children }) {
  const [open, setOpen] = useState(true);
  return (
    <section className={`isec ${open ? '' : 'is-closed'}`}>
      <h3>
        <button type="button" className="isec-h" aria-expanded={open} onClick={() => setOpen(!open)}>
          <ChevronRight size={13} className="isec-chev" /><span>{title}</span>{aside && <span className="isec-aside">{aside}</span>}
        </button>
      </h3>
      {open && <div className="isec-body">{children}</div>}
    </section>
  );
}

const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

export default function D6() {
  const L = useLauncher();
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [level, setLevel] = useState('basic');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const vars = {
    '--acc': vivid(tone, 0.66, 0.6) || '#e8e8ea',
    '--acc-dim': toneAt(tone, 0.5, 0.2, '#25262a'),
  };

  return (
    <div className="k d6" style={vars}>
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden />Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}>
          <Search size={14} /><span>Search games and commands</span><kbd>Ctrl K</kbd>
        </button>
        <div className="tb-right">
          <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle settings panel (Ctrl I)" title="Settings panel · Ctrl I" onClick={L.actions.toggleInspector}><PanelRight size={16} /></button>
        </div>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <label className="spine-search">
              <Search size={14} />
              <input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter library" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} />
            </label>
            <FilterMenu L={L} label="" />
          </div>
          <FilterChips L={L} />
          <div className="spine-count">{L.visible.length} games</div>
          <ul className="spine-list">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="row-art" />
                  <span className="row-text">
                    <span className="row-title">{x.title}</span>
                    <span className="row-sub">
                      {x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}
                      <span>{L.running?.id === x.id ? 'Playing now' : isInstalled(x) ? agoShort(x.lastAgo) + ' ago' : 'Not installed'}</span>
                    </span>
                  </span>
                  <SyncDot state={x.sync} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="spine-empty">No game matches these filters. <button type="button" onClick={L.actions.resetFilters}>Clear filters</button></li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <GameLogo g={g} className="hero-logo" />
          </div>
          <div className="strip">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>
                {run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={12} fill="currentColor" />}
                <span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-time">{elapsed(Date.now() - run.startedAt)}</span>
              </button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>Play</span></button>
            ) : (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-time">{copy.size}</span></button>
            )}
            <div className="src" role="radiogroup" aria-label="Launch from">
              {g.stores.map((s) => (
                <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={`src-opt ${g.source === s.store ? 'is-on' : ''}`} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                  <StoreGlyph store={s.store} size={13} /><span>{STORES[s.store].name}</span>{s.installed && <span className="src-inst" aria-label="installed" />}
                </button>
              ))}
            </div>
            {g.sync === 'conflict' ? (
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} /><span>Save conflict</span><span className="sync-cta">Resolve</span></button>
            ) : (
              <span className={`sync s-${g.sync}`}><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>
            )}
            <dl className="meta">
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Last played</dt><dd>{ago(g.lastAgo)}</dd></div>
              <div><dt>ProtonDB</dt><dd className={`tier-${g.tier}`}>{TIER_LABEL[g.tier]}</dd></div>
              <div><dt>Size</dt><dd>{copy.size}</dd></div>
            </dl>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>

        {L.inspector && (
          <aside className="insp" aria-label="Game settings">
            <div className="insp-head">
              <div className="insp-title">Game settings</div>
              <div className="seg" role="tablist" aria-label="Settings level">
                {[['basic', 'Basics'], ['advanced', 'Advanced']].map(([id, label]) => (
                  <button key={id} type="button" role="tab" aria-selected={level === id} className={level === id ? 'is-on' : ''} onClick={() => setLevel(id)}>{label}</button>
                ))}
              </div>
            </div>
            <div className="insp-scroll" role="tabpanel"><InspectorFields L={L} Section={Section} level={level} /></div>
            <div className="insp-foot">Runs with {runnerOf(g.runner)?.name}</div>
          </aside>
        )}
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}<b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>
            <span className="tele-ring" aria-hidden />{j.label}<b>{Math.round(j.progress * 100)}%</b>
          </span>
        ))}
        {L.jobs.length === 0 && !L.running && <span className="tele-item">All caught up</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
