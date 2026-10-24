import React, { useMemo, useState } from 'react';
import '@fontsource-variable/overpass';
import '@fontsource/overpass-mono/400.css';
import '@fontsource/overpass-mono/600.css';
import './d2.css';
import { Play, Square, Search, PanelRight, ArrowUp, ArrowDown, AlertTriangle, Loader } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, agoShort, useTick, clock } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, InspectorFields, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

function Section({ title, aside, children }) {
  return (
    <section className="isec">
      <h3 className="isec-h"><span>{title}</span>{aside && <span className="isec-aside">{aside}</span>}</h3>
      <div className="isec-body">{children}</div>
    </section>
  );
}

const COLS = [
  { id: 'title', label: 'Game', cls: 'c-title' },
  { id: 'last', label: 'Last played', cls: 'c-last num' },
  { id: 'hours', label: 'Hours', cls: 'c-hours num' },
];

// Fourteen days on a 24-hour rail; each session a bar. The rhythm of your play.
function Schedule({ L }) {
  const days = 14;
  const now = new Date(L.now);
  const midnight = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const rows = useMemo(() => {
    const r = Array.from({ length: days }, (_, i) => ({ day: midnight - i * 864e5, bars: [] }));
    for (const s of L.sessions) {
      let start = s.start;
      let end = s.start + s.dur * 60e3;
      while (start < end) {
        const dayStart = new Date(start); dayStart.setHours(0, 0, 0, 0);
        const idx = Math.round((midnight - dayStart.getTime()) / 864e5);
        const segEnd = Math.min(end, dayStart.getTime() + 864e5);
        if (idx >= 0 && idx < days) r[idx].bars.push({ a: (start - dayStart) / 864e5, b: (segEnd - dayStart) / 864e5, crash: s.crash && segEnd === end });
        start = segEnd;
      }
    }
    return r;
  }, [L.sessions, midnight]);
  const total = L.sessions.filter((s) => L.now - s.start < days * 864e5).reduce((a, s) => a + s.dur, 0);
  const nowX = (L.now - midnight) / 864e5;
  return (
    <figure className="sched" aria-label={`Play sessions over the last ${days} days`}>
      <figcaption className="sched-cap">
        <span className="sched-t">Last 14 days</span>
        <span className="sched-sum">{L.sessions.length ? `${Math.floor(total / 60)} h ${total % 60} min over ${L.sessions.length} sessions` : 'No sessions'}</span>
      </figcaption>
      <div className="sched-grid">
        <div className="sched-hours" aria-hidden>
          {[0, 3, 6, 9, 12, 15, 18, 21].map((h) => <span key={h} style={{ left: `${(h / 24) * 100}%` }}>{String(h).padStart(2, '0')}</span>)}
        </div>
        {rows.map((r, i) => (
          <div key={r.day} className={`sched-row ${new Date(r.day).getDay() % 6 === 0 ? 'is-weekend' : ''}`}>
            <span className="sched-day">{i === 0 ? 'Today' : new Date(r.day).toLocaleDateString('en-GB', { weekday: 'short', day: '2-digit' })}</span>
            <span className="sched-rail">
              {r.bars.map((b, j) => <span key={j} className={`sched-bar ${b.crash ? 'is-crash' : ''}`} style={{ left: `${b.a * 100}%`, width: `${Math.max(0.4, (b.b - b.a) * 100)}%` }} />)}
              {i === 0 && <span className="sched-now" style={{ left: `${nowX * 100}%` }} />}
            </span>
          </div>
        ))}
      </div>
    </figure>
  );
}

export default function D2() {
  const L = useLauncher();
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const sortCol = L.sort.replace(/-desc$/, '');
  const sortDesc = L.sort.endsWith('-desc');
  const sortBy = (id) => L.actions.setSort(sortCol === id ? (sortDesc ? id : `${id}-desc`) : id);

  return (
    <div className="k d2">
      <header className="hd">
        <div className="hd-brand">Platypus</div>
        <div className="hd-clock" aria-label="Current time">{clock(Date.now())}<span>{new Date().toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })}</span></div>
        <button type="button" className="hd-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>Command or game</span><kbd>Ctrl K</kbd></button>
        <button type="button" className={`icon-btn ${L.inspector ? 'is-on' : ''}`} aria-pressed={L.inspector} aria-label="Toggle inspector (Ctrl I)" onClick={L.actions.toggleInspector}><PanelRight size={16} /></button>
      </header>

      <div className={`body ${L.inspector ? 'has-insp' : ''}`}>
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Find a game" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <div className="spine-stores" role="radiogroup" aria-label="Store">
              {['all', 'steam', 'gog', 'epic', 'local'].map((s) => (
                <button key={s} type="button" role="radio" aria-checked={L.storeFilter === s} className={L.storeFilter === s ? 'is-on' : ''} onClick={() => L.actions.setStoreFilter(s)} title={s === 'all' ? 'All stores' : STORES[s].name}>
                  {s === 'all' ? 'All' : <StoreGlyph store={s} size={12} />}
                </button>
              ))}
            </div>
          </div>
          <div className="tt" role="table" aria-label="Library">
            <div className="tt-head" role="row">
              <span className="c-dot" role="columnheader" aria-label="Save sync" />
              {COLS.map((c) => (
                <span key={c.id} role="columnheader" className={c.cls} aria-sort={sortCol === c.id ? (sortDesc ? 'descending' : 'ascending') : 'none'}>
                  <button type="button" onClick={() => sortBy(c.id)} className={sortCol === c.id ? 'is-on' : ''}>
                    {c.label}{sortCol === c.id && (sortDesc ? <ArrowUp size={11} /> : <ArrowDown size={11} />)}
                  </button>
                </span>
              ))}
            </div>
            <div className="tt-body" role="rowgroup">
              {L.visible.map((x) => (
                <button key={x.id} type="button" role="row" data-game-row={x.id} aria-current={x.id === g.id} className={`tt-row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <span className="c-dot" role="cell"><SyncDot state={x.sync} /></span>
                  <span className="c-title" role="cell">
                    <span className="tt-name">{x.title}</span>
                    <span className="tt-stores">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}{!isInstalled(x) && <em>not installed</em>}{L.running?.id === x.id && <em className="tt-live">playing</em>}</span>
                  </span>
                  <span className="c-last num" role="cell">{agoShort(x.lastAgo)}</span>
                  <span className="c-hours num" role="cell">{x.hours < 10 ? x.hours.toFixed(1) : Math.round(x.hours)}</span>
                </button>
              ))}
              {L.visible.length === 0 && <p className="spine-empty">Nothing matches “{L.query}”.</p>}
            </div>
          </div>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
          </div>
          <div className="strip">
            <div className="strip-title">
              <h1>{g.title}</h1>
              <span className="strip-sub"><span>{g.dev}</span><span>{g.year}</span><span>{g.genre}</span></span>
            </div>
            <div className="strip-actions">
              {g.stores.length > 1 ? (
                <div className="src" role="radiogroup" aria-label="Launch from">
                  {g.stores.map((s) => (
                    <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => L.actions.setSource(g.id, s.store)}>
                      <StoreGlyph store={s.store} size={12} /><span className="src-name">{STORES[s.store].name}</span><span className="src-state">{s.installed ? s.version : 'not installed'}</span>
                    </button>
                  ))}
                </div>
              ) : (
                <span className="src-one"><StoreGlyph store={g.stores[0].store} size={12} />{STORES[g.stores[0].store].name}<span className="src-state">{copy.version}</span></span>
              )}
              {g.sync === 'conflict' ? (
                <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} /> Save conflict, resolve</button>
              ) : (
                <span className="sync"><SyncDot state={g.sync} /> {SYNC_LABEL[g.sync]}</span>
              )}
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={13} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-sub">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>Play</span><span className="play-sub">{runnerOf(g.runner)?.name}</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><span>Install</span><span className="play-sub">{copy.size} from {STORES[copy.store].name}</span></button>
              )}
            </div>
          </div>
          <div className="board">
            <Schedule L={L} />
            <dl className="facts">
              <div><dt>Played</dt><dd>{Math.round(g.hours)}<small> h</small></dd></div>
              <div><dt>Last session</dt><dd>{ago(g.lastAgo)}</dd></div>
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
          <aside className="insp" aria-label="Launch settings">
            <div className="insp-head">Launch settings</div>
            <div className="insp-scroll"><InspectorFields L={L} Section={Section} /></div>
          </aside>
        )}
      </div>

      <footer className="dep" aria-label="Background activity">
        <span className="dep-h">Now running</span>
        {L.running && <span className="dep-item is-live"><span className="dep-what">{L.games.find((x) => x.id === L.running.id)?.title}</span><span className="dep-eta">{elapsed(Date.now() - L.running.startedAt)}</span></span>}
        {L.jobs.map((j) => {
          const eta = Math.round(((1 - j.progress) / j.rate) * 0.7);
          return (
            <span key={j.id} className={`dep-item dep-${j.kind}`}>
              <span className="dep-kind">{j.kind}</span>
              <span className="dep-what">{j.label}</span>
              <span className="dep-eta">{eta < 60 ? `${eta}s` : `${Math.round(eta / 60)} min`}</span>
            </span>
          );
        })}
        {L.jobs.length === 0 && !L.running && <span className="dep-item">Nothing in progress</span>}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
