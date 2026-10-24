import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/mona-sans/wdth.css';
import '@fontsource-variable/geist-mono';
import './d17.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, SlidersHorizontal, X, Download, ChevronRight } from 'lucide-react';
import { useLauncher, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, toneAt, vivid } from '../../core/launcher.js';
import { StoreGlyph, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, Tabs, SessionsTable, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary } from '../../core/kit.jsx';
import { StoreAbout, Gallery, AchSummary, AchList, AchIcon, GameFacts, achievementsOf, rarityLabel } from '../../core/page.jsx';
import { STORES } from '../../core/data.js';

const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

// Settings are a section of the game page, like a desktop app's preferences pane.
function Prefs({ L, cat, setCat }) {
  const c = SETTINGS_CATS.find((x) => x.id === cat);
  const group = (lvl, title) => (
    <>
      <div className="prefs-h">{title}</div>
      {SETTINGS_CATS.filter((x) => x.level === lvl).map((x) => {
        const Icon = x.icon;
        return (
          <button key={x.id} type="button" className={`prefs-item ${cat === x.id ? 'is-on' : ''}`} aria-current={cat === x.id} onClick={() => setCat(x.id)}>
            <Icon size={17} strokeWidth={1.8} />
            <span><b>{x.label}</b><small>{catSummary(L, x.id)}</small></span>
          </button>
        );
      })}
    </>
  );
  return (
    <div className="prefs">
      <nav className="prefs-nav" aria-label="Settings sections">{group('basic', 'Everyday')}{group('advanced', 'Advanced')}</nav>
      <section className="prefs-pane" key={cat} aria-labelledby="prefs-title">
        <h3 id="prefs-title">{c.label}</h3>
        <p>{c.blurb}.</p>
        <SettingsCategory L={L} id={cat} />
      </section>
    </div>
  );
}

function AchPage({ g }) {
  const A = achievementsOf(g);
  const pct = A.unlocked / A.total;
  return (
    <div className="achs">
      <div className="ach-hero">
        <div className="ach-ring" style={{ '--p': pct }}><span>{Math.round(pct * 100)}%</span></div>
        <div><h3>{A.unlocked} of {A.total} achievements</h3><p>{A.unlocked === A.total ? 'All of them. Nicely done.' : `${A.total - A.unlocked} left to unlock.`}</p></div>
        {A.rarest && (
          <div className="ach-rarest"><AchIcon a={A.rarest} size={44} /><span><small>Your rarest, {rarityLabel(A.rarest.pct).toLowerCase()}</small><b>{A.rarest.name}</b></span></div>
        )}
      </div>
      <AchList g={g} />
    </div>
  );
}

export default function D17() {
  const L = useLauncher({ initial: 'hollow' });
  useTick(1000);
  const [page, setPage] = useState('overview');
  const [cat, setCat] = useState('compat');
  const [stuck, setStuck] = useState(false);
  const stage = useRef();
  const pagesRef = useRef();
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const ach = achievementsOf(g);
  const vars = {
    '--acc': vivid(tone, 0.7, 0.62) || '#f2f2f2',
    '--ground': toneAt(tone, 0.3, 0.085, '#151518'),
    '--ground-2': toneAt(tone, 0.26, 0.12, '#1c1c20'),
    '--ground-3': toneAt(tone, 0.22, 0.16, '#26262b'),
  };
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const attention = viewCount(L, 'attention');
  const views = [['all', 'All'], ['installed', 'Installed'], ['downloads', 'Downloads']];
  const nConflict = L.games.filter((x) => x.sync === 'conflict').length;
  const nUpdate = L.games.filter((x) => x.stores.some((s) => s.update)).length;
  const nRough = attention - nConflict - nUpdate;
  const attnWhy = [nConflict && `${nConflict} save conflict${nConflict > 1 ? 's' : ''}`, nUpdate && `${nUpdate} update${nUpdate > 1 ? 's' : ''}`, nRough > 0 && `${nRough} with known issues`].filter(Boolean).join(', ');

  useEffect(() => { setPage((p) => (p === 'achievements' && !achievementsOf(g) ? 'overview' : p)); }, [g.id]);
  useEffect(() => {
    const el = stage.current;
    const on = () => setStuck(pagesRef.current && pagesRef.current.getBoundingClientRect().top <= el.getBoundingClientRect().top + 1);
    el.addEventListener('scroll', on, { passive: true });
    return () => el.removeEventListener('scroll', on);
  }, []);
  const go = (p) => {
    setPage(p);
    const el = stage.current;
    const top = pagesRef.current.offsetTop;
    if (el.scrollTop < top) el.scrollTo({ top, behavior: 'smooth' });
  };

  const playBtn = (small) => run ? (
    <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={small ? 15 : 18} className="spin" /> : <Square size={small ? 12 : 14} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span>{!small && <span className="play-t">{elapsed(Date.now() - run.startedAt)}</span>}</button>
  ) : copy.installed ? (
    <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={small ? 15 : 18} fill="currentColor" /><span>Play</span></button>
  ) : (
    <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Download size={small ? 15 : 18} /><span>Install</span>{!small && <span className="play-t">{copy.size}</span>}</button>
  );

  const tabs = [
    { id: 'overview', label: 'Overview' },
    ...(ach ? [{ id: 'achievements', label: 'Achievements', count: `${ach.unlocked}/${ach.total}` }] : []),
    { id: 'activity', label: 'Activity' },
    { id: 'settings', label: 'Settings' },
  ];

  return (
    <div className="k d17" style={vars}>
      <div className="body">
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-k" onClick={() => L.actions.openPalette()} aria-label="Search and commands"><Search size={15} /><kbd>Ctrl K</kbd></button>
          </div>
          <label className="spine-search"><Search size={15} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search your games" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
          <div className="views" role="radiogroup" aria-label="Show">
            {views.map(([id, label]) => {
              const n = viewCount(L, id);
              return (
                <button key={id} type="button" role="radio" aria-checked={L.stateFilter === id} className={L.stateFilter === id ? 'is-on' : ''} onClick={() => L.actions.setStateFilter(id)}>
                  {label}{id === 'downloads' && n > 0 && <span className="views-n">{n}</span>}
                </button>
              );
            })}
          </div>
          <div className="stores" role="group" aria-label="Stores">
            {Object.keys(STORES).filter((s) => counts[s]).map((s) => {
              const on = L.stores.includes(s);
              return (
                <button key={s} type="button" aria-pressed={on} className={`store-chip ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleStore(s)}>
                  <StoreGlyph store={s} size={13} />{STORES[s].name}<span>{counts[s]}</span>
                </button>
              );
            })}
          </div>
          {attention > 0 && L.stateFilter !== 'attention' && (
            <button type="button" className="callout" onClick={() => L.actions.setStateFilter('attention')}>
              <AlertTriangle size={15} /><span><b>{attention} games need you</b><small>{attnWhy}</small></span><ChevronRight size={15} />
            </button>
          )}
          {L.stateFilter === 'attention' && <button type="button" className="callout is-on" onClick={() => L.actions.setStateFilter('all')}><span><b>Showing games that need you</b><small>Back to all games</small></span><X size={15} /></button>}
          <ul className="spine-list">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).cover} className="row-cover" />
                  <span className="row-text">
                    <span className="row-title">{x.title}</span>
                    <span className="row-sub">{isInstalled(x) ? `Played ${ago(x.lastAgo)}` : 'Not installed'}</span>
                  </span>
                  <AttnMark L={L} x={x} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="spine-empty"><p>No games here.</p><button type="button" className="btn" onClick={L.actions.resetFilters}>Show all games</button></li>}
          </ul>
        </aside>

        <main className="stage" ref={stage}>
          <div className="hero"><Img key={a.hero} src={a.hero} className="hero-img" /></div>
          <div className="head">
            <Img key={a.cover} src={a.cover} className="head-cover" />
            <div className="head-main">
              <h1>{g.title}</h1>
              <p className="head-desc">{g.desc}</p>
              <div className="head-actions">
                {playBtn(false)}
                <button type="button" className="ghost" onClick={() => go('settings')}><SlidersHorizontal size={17} />Settings</button>
              </div>
            </div>
          </div>
          <div className="strip">
            {g.stores.length > 1 ? (
              <div className="pill-group" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={`pill ${g.source === s.store ? 'is-on' : ''}`} onClick={() => L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={13} />{STORES[s.store].name}
                  </button>
                ))}
              </div>
            ) : <span className="pill is-static"><StoreGlyph store={copy.store} size={13} />{STORES[copy.store].name}</span>}
            {g.sync === 'conflict' ? (
              <button type="button" className="pill is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Your saves differ. Choose one</button>
            ) : <span className="pill is-static"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
            <span className="facts">
              <span><b>{Math.round(g.hours)} h</b> played</span>
              <span>Last played <b>{ago(g.lastAgo)}</b></span>
              <span><b>{TIER_LABEL[g.tier]}</b> on ProtonDB</span>
            </span>
          </div>

          <div className={`pages ${stuck ? 'is-stuck' : ''}`} ref={pagesRef}>
            <Tabs label="Game page" tabs={tabs} value={page} onChange={go} />
            <div className="pages-mini" aria-hidden={!stuck}>
              <Img src={a.cover} /><b>{g.title}</b>{playBtn(true)}
            </div>
          </div>
          <div className="sec" role="tabpanel" key={`${g.id}-${page}`}>
            {page === 'overview' && (
              <div className="ov">
                <div className="ov-main">
                  <Gallery g={g} />
                  <div className="bubble"><StoreAbout g={g} clamp={460} /></div>
                </div>
                <div className="ov-side">
                  {ach && <section className="bubble"><h2 className="bubble-h">Achievements</h2><AchSummary g={g} onOpen={() => go('achievements')} /></section>}
                  <section className="bubble"><h2 className="bubble-h">Details</h2><GameFacts g={g} /></section>
                </div>
              </div>
            )}
            {page === 'achievements' && ach && <AchPage g={g} />}
            {page === 'activity' && <div className="page-card"><SessionsTable L={L} /></div>}
            {page === 'settings' && <Prefs L={L} cat={cat} setCat={setCat} />}
          </div>
        </main>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}, {elapsed(Date.now() - L.running.startedAt)}</span>}
        {L.jobs.map((j) => (
          <span key={j.id} className="tele-item"><span className="tele-bar" aria-hidden><span style={{ width: `${j.progress * 100}%` }} /></span>{j.label}</span>
        ))}
      </footer>

      <Palette L={L} placeholder="Search games, settings and actions" />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
