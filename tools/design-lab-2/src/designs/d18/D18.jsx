import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/big-shoulders-display';
import '@fontsource-variable/schibsted-grotesk';
import '@fontsource-variable/geist-mono';
import './d18.css';
import { Play, Square, Search, AlertTriangle, Loader, Download, Settings2, X, ChevronDown, Check } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, toneAt } from '../../core/launcher.js';
import { StoreGlyph, SyncDot, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, Tabs, SessionsTable, SavesTable, NotesList, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary } from '../../core/kit.jsx';
import { StoreAbout, Gallery, AchSummary, AchList, GameFacts, achievementsOf, pageOf } from '../../core/page.jsx';
import { STORES } from '../../core/data.js';

// The store picker: one line when closed, a checklist when open.
function StorePicker({ L }) {
  const [open, setOpen] = useState(false);
  const root = useRef();
  useEffect(() => {
    if (!open) return;
    root.current?.querySelector('.sp-item')?.focus();
    const close = (e) => { if (!root.current?.contains(e.target)) setOpen(false); };
    const key = (e) => { if (e.key === 'Escape') { setOpen(false); root.current?.querySelector('.sp-btn')?.focus(); } };
    window.addEventListener('pointerdown', close);
    window.addEventListener('keydown', key);
    return () => { window.removeEventListener('pointerdown', close); window.removeEventListener('keydown', key); };
  }, [open]);
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const ids = Object.keys(STORES).filter((s) => counts[s]);
  const label = L.stores.length === 0 ? 'Every store' : L.stores.length === 1 ? STORES[L.stores[0]].name : `${L.stores.length} stores`;
  return (
    <div className="sp" ref={root}>
      <button type="button" className="sp-btn" aria-haspopup="true" aria-expanded={open} onClick={() => setOpen(!open)}>
        <span className="sp-glyphs">{(L.stores.length ? L.stores : ids).slice(0, 4).map((s) => <StoreGlyph key={s} store={s} size={13} />)}</span>
        <span className="sp-label">{label}</span>
        <ChevronDown size={15} />
      </button>
      {open && (
        <div className="sp-panel" role="group" aria-label="Stores">
          <button type="button" className={`sp-item ${L.stores.length === 0 ? 'is-on' : ''}`} aria-pressed={L.stores.length === 0} onClick={() => L.actions.setStores([])}>
            <span className="sp-check">{L.stores.length === 0 && <Check size={12} strokeWidth={3} />}</span>Every store<span className="sp-n">{L.games.length}</span>
          </button>
          <div className="sp-sep" />
          {ids.map((s) => {
            const on = L.stores.includes(s);
            return (
              <button key={s} type="button" className={`sp-item ${on ? 'is-on' : ''}`} aria-pressed={on} onClick={() => L.actions.toggleStore(s)}>
                <span className="sp-check">{on && <Check size={12} strokeWidth={3} />}</span>
                <StoreGlyph store={s} size={14} />{STORES[s].name}<span className="sp-n">{counts[s]}</span>
              </button>
            );
          })}
          <div className="sp-foot">Pick several to combine them.</div>
        </div>
      )}
    </div>
  );
}

// Settings on a blurred pane over the page. The band colour marks only what is active.
function SettingsPane({ L, onClose }) {
  const [cat, setCat] = useState('compat');
  const ref = useRef();
  const g = L.game;
  useEffect(() => {
    ref.current?.querySelector('.toc-item.is-on')?.focus();
    const onKey = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  const c = SETTINGS_CATS.find((x) => x.id === cat);
  return (
    <div className="pane-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className="pane" ref={ref} role="dialog" aria-modal="true" aria-labelledby="pane-title">
        <header className="pane-head">
          <Img src={artOf(g).cover} className="pane-cover" />
          <h2 id="pane-title">Settings<span>{g.title}</span></h2>
          <button type="button" className="pane-x" onClick={onClose}><X size={16} />Close</button>
        </header>
        <div className="pane-body">
          <nav className="toc" aria-label="Settings chapters">
            {['basic', 'advanced'].map((lvl) => (
              <div key={lvl} className="toc-group">
                <div className="toc-h">{lvl === 'basic' ? 'Everyday' : 'Advanced'}</div>
                {SETTINGS_CATS.filter((x) => x.level === lvl).map((x) => (
                  <button key={x.id} type="button" className={`toc-item ${cat === x.id ? 'is-on' : ''}`} aria-current={cat === x.id} onClick={() => setCat(x.id)}>
                    <span className="toc-label">{x.label}</span>
                    <span className="toc-sum">{catSummary(L, x.id)}</span>
                  </button>
                ))}
              </div>
            ))}
          </nav>
          <section className="chapter" key={cat}>
            <h3>{c.label}</h3>
            <p className="chapter-blurb">{c.blurb}.</p>
            <SettingsCategory L={L} id={cat} />
          </section>
        </div>
      </div>
    </div>
  );
}

export default function D18() {
  const L = useLauncher({ initial: 'balatro', sort: 'title' });
  useTick(1000);
  const [tab, setTab] = useState('achievements');
  const main = useRef();
  const [settings, setSettings] = useState(false);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const vars = {
    '--band': toneAt(tone, 0.8, 0.83, '#ecebe5'),
    '--ink': toneAt(tone, 0.5, 0.1, '#141414'),
  };
  const groups = [];
  for (const x of L.visible) {
    const k = L.sort === 'title' ? (/[a-z]/i.test(x.title[0]) ? x.title[0].toUpperCase() : '#') : null;
    if (!groups.length || groups[groups.length - 1].k !== k) groups.push({ k, items: [] });
    groups[groups.length - 1].items.push(x);
  }
  const ach = achievementsOf(g);
  const page = pageOf(g);
  useEffect(() => { main.current?.scrollTo({ top: 0 }); }, [g.id]);
  const tabs = [
    ...(ach ? [{ id: 'achievements', label: 'Achievements', count: `${ach.unlocked}/${ach.total}` }] : []),
    { id: 'sessions', label: 'Sessions', count: L.sessions.length },
    { id: 'saves', label: 'Saves', count: g.saves.length },
    { id: 'compat', label: 'Compatibility', count: g.notes.length },
  ];
  const curTab = tabs.some((t) => t.id === tab) ? tab : tabs[0].id;
  const toReadout = () => { setTab('achievements'); document.getElementById('d18-readout')?.scrollIntoView({ behavior: 'smooth', block: 'start' }); };
  const titleSize = g.title.length > 26 ? 'is-long' : g.title.length > 15 ? 'is-mid' : '';
  const views = [['all', 'All'], ['installed', 'Installed'], ['downloads', 'Downloads'], ['attention', 'Attention']];

  return (
    <div className="k d18" style={vars}>
      <div className="body">
        <aside className="spine" aria-label="Library">
          <div className="spine-top">
            <span className="brand">Platypus</span>
            <button type="button" className="spine-k" onClick={() => L.actions.openPalette()}><Search size={13} />Find<kbd>Ctrl K</kbd></button>
          </div>
          <StorePicker L={L} />
          <div className="views" role="radiogroup" aria-label="Show">
            {views.map(([id, label]) => {
              const n = viewCount(L, id);
              if (id !== 'all' && id !== 'installed' && !n) return null;
              return (
                <button key={id} type="button" role="radio" aria-checked={L.stateFilter === id} className={`${L.stateFilter === id ? 'is-on' : ''} ${id === 'attention' ? 'is-warn' : ''}`} onClick={() => L.actions.setStateFilter(id)}>
                  {label}<sup>{n}</sup>
                </button>
              );
            })}
          </div>
          <label className="spine-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Search the library" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
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
                        <AttnMark L={L} x={x} />
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
            {L.visible.length === 0 && <p className="index-empty">Nothing matches. <button type="button" onClick={L.actions.resetFilters}>Show everything</button></p>}
          </div>
        </aside>

        <main className="main" ref={main}>
          <div className="hero"><Img key={a.hero} src={a.hero} className="hero-img" /></div>
          <div className="mast">
            <div className="sleeve">
              <Img key={a.cover} src={a.cover} className="sleeve-cover" />
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={18} className="spin" /> : <Square size={15} fill="currentColor" />}{run.phase === 'starting' ? 'Starting' : 'Stop'}<span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={18} fill="currentColor" />Play</button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Download size={18} />Install<span className="play-t">{copy.size}</span></button>
              )}
            </div>
            <div className="mast-body">
              <h1 className={`mast-title ${titleSize}`}>{g.title}</h1>
              <p className="mast-desc">{g.desc}</p>
              <div className="mast-row">
                <span className="mast-k">From</span>
                {g.stores.map((s) => (
                  <button key={s.store} type="button" aria-pressed={g.source === s.store} className={`mast-src ${g.source === s.store ? 'is-on' : ''}`} onClick={() => g.stores.length > 1 && L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={12} />{STORES[s.store].name}
                  </button>
                ))}
                <button type="button" className="mast-settings" onClick={() => setSettings(true)}><Settings2 size={15} />Settings</button>
              </div>
            </div>
          </div>
          <dl className="colophon">
            <div><dt>Studio</dt><dd>{g.dev}, {g.year}</dd></div>
            <div><dt>Played</dt><dd>{Math.round(g.hours)} hours</dd></div>
            <div><dt>Last</dt><dd>{ago(g.lastAgo)}</dd></div>
            <div><dt>Runs with</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
            <div><dt>Saves</dt><dd>{g.sync === 'conflict' ? <button type="button" className="col-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={13} />Conflict, resolve</button> : SYNC_LABEL[g.sync]}</dd></div>
          </dl>
          <div className="article">
            <article>
              <h2 className="article-h">About the game</h2>
              <StoreAbout g={g} clamp={900} />
            </article>
            <aside className="margin">
              {page?.shots.length > 0 && <section className="margin-sec"><h2 className="margin-h">Pictures<small>{page.shots.length} screenshots</small></h2><Gallery g={g} variant="sheet" max={7} /></section>}
              {ach && <section className="margin-sec"><h2 className="margin-h">Achievements</h2><AchSummary g={g} onOpen={toReadout} /></section>}
              <section className="margin-sec"><h2 className="margin-h">Credits</h2><GameFacts g={g} /></section>
            </aside>
          </div>
          <div className="readout" id="d18-readout">
            <Tabs label="Game details" tabs={tabs} value={curTab} onChange={setTab} />
            <div className="readout-body" role="tabpanel">
              {curTab === 'achievements' && <div className="achs"><AchList g={g} /></div>}
              {curTab === 'sessions' && <SessionsTable L={L} />}
              {curTab === 'saves' && <SavesTable L={L} />}
              {curTab === 'compat' && <NotesList L={L} />}
            </div>
          </div>
        </main>
      </div>

      {settings && <SettingsPane L={L} onClose={() => setSettings(false)} />}
      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live">Playing {L.games.find((x) => x.id === L.running.id)?.title} <b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label} <b>{Math.round(j.progress * 100)}%</b></span>)}
      </footer>

      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
