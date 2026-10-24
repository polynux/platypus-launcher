import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/bricolage-grotesque/opsz.css';
import '@fontsource-variable/geist-mono';
import './d20.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, LibraryBig, ArrowDownToLine, CircleAlert, Settings2, X, ChevronDown, ChevronRight, ChevronLeft, Check, Download } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, agoShort, useTick, useArtTone, toneAt, VIEWS } from '../../core/launcher.js';
import { StoreGlyph, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, Tabs, SessionsTable, SavesTable, NotesList, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary, Select } from '../../core/kit.jsx';
import { StoreAbout, Gallery, AchSummary, AchList, GameFacts, achievementsOf } from '../../core/page.jsx';
import { STORES } from '../../core/data.js';

const VIEW_ICON = { all: LibraryBig, installed: HardDrive, downloads: ArrowDownToLine, attention: CircleAlert };
const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

// Settings as a stack of pages: a list of topics, then one topic at a time.
function SettingsStack({ L, onClose }) {
  const [page, setPage] = useState(null);
  const pageRef = useRef(null);
  pageRef.current = page;
  const ref = useRef();
  const g = L.game;
  useEffect(() => {
    ref.current?.querySelector('.stack-page button')?.focus();
  }, [page]);
  useEffect(() => {
    const key = (e) => {
      if (e.key !== 'Escape' || e.target.closest('.sel')) return;
      if (pageRef.current) setPage(null);
      else onClose();
    };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, []);
  const c = SETTINGS_CATS.find((x) => x.id === page);
  return (
    <div className="scrim d20-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <aside className="stack" ref={ref} role="dialog" aria-modal="true" aria-label={`Settings for ${g.title}`}>
        {!c ? (
          <div className="stack-page" key="root">
            <header className="stack-head">
              <Img src={artOf(g).cover} className="stack-cover" />
              <div><h2>Settings</h2><span>{g.title}</span></div>
              <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}><X size={16} /></button>
            </header>
            {['basic', 'advanced'].map((lvl) => (
              <div key={lvl} className="stack-group">
                <div className="stack-group-h">{lvl === 'basic' ? 'Everyday' : 'Advanced'}</div>
                {SETTINGS_CATS.filter((x) => x.level === lvl).map((x) => {
                  const Icon = x.icon;
                  return (
                    <button key={x.id} type="button" className="stack-link" onClick={() => setPage(x.id)}>
                      <span className="stack-ico"><Icon size={16} /></span>
                      <span className="stack-text"><span>{x.label}</span><small>{x.blurb}</small></span>
                      <span className="stack-sum">{catSummary(L, x.id)}</span>
                      <ChevronRight size={16} />
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        ) : (
          <div className="stack-page is-sub" key={c.id}>
            <header className="stack-head">
              <button type="button" className="stack-back" onClick={() => setPage(null)}><ChevronLeft size={17} />Settings</button>
              <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}><X size={16} /></button>
            </header>
            <h2 className="stack-title">{c.label}</h2>
            <p className="stack-blurb">{c.blurb}.</p>
            <SettingsCategory L={L} id={c.id} />
          </div>
        )}
      </aside>
    </div>
  );
}

export default function D20() {
  const L = useLauncher({ initial: 'dave' });
  useTick(1000);
  const [tab, setTab] = useState('overview');
  const [settings, setSettings] = useState(false);
  const [storesOpen, setStoresOpen] = useState(false);
  const stage = useRef();
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const vars = {
    '--t0': toneAt(tone, 0.5, 0.07, '#121214'),
    '--t1': toneAt(tone, 0.46, 0.1, '#18181b'),
    '--t2': toneAt(tone, 0.42, 0.14, '#202024'),
    '--t3': toneAt(tone, 0.38, 0.2, '#2b2b30'),
    '--t-text': toneAt(tone, 0.3, 0.95, '#f1f1f1'),
    '--t-muted': toneAt(tone, 0.25, 0.72, '#a6a6a6'),
    '--t-faint': toneAt(tone, 0.22, 0.52, '#707070'),
    '--t-acc': toneAt(tone, 0.9, 0.66, '#f1f1f1'),
  };
  const ach = achievementsOf(g);
  useEffect(() => { stage.current?.scrollTo({ top: 0 }); }, [g.id]);
  const tabs = [
    { id: 'overview', label: 'Overview' },
    ...(ach ? [{ id: 'achievements', label: 'Achievements', count: `${ach.unlocked}/${ach.total}` }] : []),
    { id: 'sessions', label: 'Sessions', count: L.sessions.length },
    { id: 'saves', label: 'Saves', count: g.saves.length },
    { id: 'compat', label: 'Compatibility', count: g.notes.length },
  ];
  const curTab = tabs.some((t) => t.id === tab) ? tab : 'overview';
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;

  return (
    <div className="k d20" style={vars}>
      <header className="tb">
        <div className="tb-brand">Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={15} /><span>Find a game or a setting</span><kbd>Ctrl K</kbd></button>
        <span />
      </header>

      <div className="body">
        <aside className="spine" aria-label="Library">
          <div className="tiles" role="radiogroup" aria-label="Show">
            {VIEWS.map((v) => {
              const Icon = VIEW_ICON[v.id];
              const n = viewCount(L, v.id);
              const on = L.stateFilter === v.id;
              return (
                <button key={v.id} type="button" role="radio" aria-checked={on} className={`tile ${on ? 'is-on' : ''} ${v.id === 'attention' && n ? 'is-warn' : ''}`} onClick={() => L.actions.setStateFilter(v.id)}>
                  <Icon size={16} /><b>{n}</b><span>{v.label}</span>
                </button>
              );
            })}
          </div>
          <div className="stores">
            <button type="button" className="stores-h" aria-expanded={storesOpen} onClick={() => setStoresOpen(!storesOpen)}>
              Stores<span>{L.stores.length ? `${L.stores.length} of ${Object.keys(counts).length}` : 'All'}</span><ChevronDown size={14} className={storesOpen ? '' : 'is-closed'} />
            </button>
            {!storesOpen && L.stores.length > 0 && (
              <div className="stores-picked">{L.stores.map((s) => <button key={s} type="button" onClick={() => L.actions.toggleStore(s)} aria-label={`Remove ${STORES[s].name} filter`}><StoreGlyph store={s} size={12} />{STORES[s].name}<X size={11} /></button>)}</div>
            )}
            {storesOpen && Object.keys(STORES).filter((s) => counts[s]).map((s) => {
              const on = L.stores.includes(s);
              return (
                <button key={s} type="button" role="checkbox" aria-checked={on} className={`store ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleStore(s)}>
                  <span className="store-box">{on && <Check size={11} strokeWidth={3} />}</span>
                  <StoreGlyph store={s} size={14} />{STORES[s].name}<span className="store-n">{counts[s]}</span>
                </button>
              );
            })}
          </div>
          <div className="spine-tools">
            <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <Select label="Sort" className="spine-sort" value={L.sort} onChange={L.actions.setSort} options={[{ value: 'recent', label: 'Recent' }, { value: 'title', label: 'A–Z' }, { value: 'hours', label: 'Played' }]} />
          </div>
          <ul className="reel">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`frame ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="frame-img" />
                  <span className="frame-text">
                    <span className="frame-title">{x.title}</span>
                    <span className="frame-meta">{isInstalled(x) ? `${agoShort(x.lastAgo)} ago` : 'not installed'}</span>
                  </span>
                  <AttnMark L={L} x={x} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="spine-empty">No game matches. <button type="button" onClick={L.actions.resetFilters}>Clear filters</button></li>}
          </ul>
        </aside>

        <main className="stage" ref={stage}>
          <div className="hero"><Img key={a.hero} src={a.hero} className="hero-img" /></div>
          <div className="head">
            <Img key={a.cover} src={a.cover} className="head-cover" />
            <div className="head-main">
              <h1>{g.title}</h1>
              <p className="head-desc">{g.desc}</p>
            </div>
            <div className="head-side">
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={17} className="spin" /> : <Square size={14} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={17} fill="currentColor" /><span>Play</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Download size={17} /><span>Install</span><span className="play-t">{copy.size}</span></button>
              )}
              <button type="button" className="ghost" onClick={() => setSettings(true)}><Settings2 size={16} />Settings</button>
            </div>
          </div>
          <div className="strip">
            {g.stores.length > 1 ? (
              <div className="src" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={13} />{STORES[s.store].name}
                  </button>
                ))}
              </div>
            ) : <span className="src-one"><StoreGlyph store={copy.store} size={13} />{STORES[copy.store].name}</span>}
            {g.sync === 'conflict' ? (
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Resolve save conflict</button>
            ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
            <span className="facts"><span><b>{Math.round(g.hours)} h</b> played</span><span>last <b>{ago(g.lastAgo)}</b></span><span><b>{TIER_LABEL[g.tier]}</b> on ProtonDB</span></span>
          </div>
          <div className="readout">
            <Tabs label="Game page" tabs={tabs} value={curTab} onChange={setTab} />
            <div className="readout-body" role="tabpanel" key={`${g.id}-${curTab}`}>
              {curTab === 'overview' && (
                <div className="ov">
                  <Gallery g={g} className="ov-gallery" />
                  <div className="ov-cols">
                    <StoreAbout g={g} clamp={520} />
                    <aside className="ov-side">
                      {ach && <section className="ov-card"><h2>Achievements</h2><AchSummary g={g} onOpen={() => setTab('achievements')} /></section>}
                      <section className="ov-card"><h2>Details</h2><GameFacts g={g} /></section>
                    </aside>
                  </div>
                </div>
              )}
              {curTab === 'achievements' && <div className="achs"><AchList g={g} /></div>}
              {curTab === 'sessions' && <SessionsTable L={L} />}
              {curTab === 'saves' && <SavesTable L={L} />}
              {curTab === 'compat' && <NotesList L={L} />}
            </div>
          </div>
        </main>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}<b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label}<b>{Math.round(j.progress * 100)}%</b></span>)}
      </footer>

      {settings && <SettingsStack L={L} onClose={() => setSettings(false)} />}
      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
