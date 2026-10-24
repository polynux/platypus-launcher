import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/figtree';
import '@fontsource-variable/geist-mono';
import './d11.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, LibraryBig, Download, ArrowDownToLine, CircleAlert, Settings2, X, ChevronDown, Check } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, vivid, toneAt, VIEWS } from '../../core/launcher.js';
import { StoreGlyph, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, Tabs, readoutTabs, ReadoutBody, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary, Select } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

const VIEW_ICON = { all: LibraryBig, installed: HardDrive, downloads: ArrowDownToLine, attention: CircleAlert };
const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

function SettingsModal({ L, onClose }) {
  const [cat, setCat] = useState('compat');
  const g = L.game;
  const ref = useRef();
  useEffect(() => {
    ref.current?.querySelector('.snav-item.is-on')?.focus();
    const onKey = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  const c = SETTINGS_CATS.find((x) => x.id === cat);
  const group = (level, title) => (
    <>
      <div className="snav-h">{title}</div>
      {SETTINGS_CATS.filter((x) => x.level === level).map((x) => {
        const Icon = x.icon;
        return (
          <button key={x.id} type="button" className={`snav-item ${cat === x.id ? 'is-on' : ''}`} aria-current={cat === x.id} onClick={() => setCat(x.id)}>
            <Icon size={16} strokeWidth={1.8} />
            <span className="snav-text"><span>{x.label}</span><span className="snav-sum">{catSummary(L, x.id)}</span></span>
          </button>
        );
      })}
    </>
  );
  return (
    <div className="scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className="smodal" role="dialog" aria-modal="true" aria-labelledby="smodal-title">
        <nav className="snav" aria-label="Settings sections">
          <div className="snav-game">
            <Img src={artOf(g).cover} className="snav-cover" />
            <div><div className="snav-title" id="smodal-title">{g.title}</div><div className="snav-sub">Game settings</div></div>
          </div>
          {group('basic', 'Everyday')}
          {group('advanced', 'Advanced')}
        </nav>
        <section className="spane">
          <header className="spane-head">
            <div><h2>{c.label}</h2><p>{c.blurb}</p></div>
            <button type="button" className="icon-btn" aria-label="Close settings" onClick={onClose}><X size={16} /></button>
          </header>
          <div className="spane-body"><SettingsCategory L={L} id={cat} /></div>
        </section>
      </div>
    </div>
  );
}

export default function D11() {
  const L = useLauncher({ initial: 'cyberpunk' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [settings, setSettings] = useState(false);
  const [storesOpen, setStoresOpen] = useState(true);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const vars = {
    '--acc': vivid(tone, 0.66, 0.6) || '#ececee',
    '--g0': toneAt(tone, 0.3, 0.075, '#121315'),
    '--g1': toneAt(tone, 0.28, 0.1, '#17181b'),
    '--g2': toneAt(tone, 0.24, 0.135, '#1e1f23'),
    '--g3': toneAt(tone, 0.2, 0.18, '#28292e'),
  };
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const dl = L.jobs.find((j) => j.kind === 'download');

  return (
    <div className="k d11" style={vars}>
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden />Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>Search games and commands</span><kbd>Ctrl K</kbd></button>
        <span />
      </header>

      <div className="body">
        <nav className="nav" aria-label="Library">
          <ul className="nav-list">
            {VIEWS.map((v) => {
              const Icon = VIEW_ICON[v.id];
              const n = viewCount(L, v.id);
              const on = L.stateFilter === v.id;
              return (
                <li key={v.id}>
                  <button type="button" className={`nav-item ${on ? 'is-on' : ''} ${v.id === 'attention' && n ? 'is-warn' : ''}`} aria-current={on} onClick={() => L.actions.setStateFilter(v.id)}>
                    <Icon size={16} strokeWidth={1.8} />
                    <span className="nav-label">{v.label}</span>
                    {v.id === 'downloads' && dl ? <span className="nav-prog" style={{ '--p': dl.progress }}>{Math.round(dl.progress * 100)}%</span> : <span className="nav-n">{n}</span>}
                  </button>
                </li>
              );
            })}
          </ul>
          <button type="button" className="nav-h" aria-expanded={storesOpen} onClick={() => setStoresOpen(!storesOpen)}>
            <span>Stores</span>{L.stores.length > 0 && <span className="nav-h-n">{L.stores.length} selected</span>}<ChevronDown size={14} className={storesOpen ? '' : 'is-closed'} />
          </button>
          {storesOpen && (
            <ul className="nav-list">
              {Object.keys(STORES).filter((s) => counts[s]).map((s) => {
                const on = L.stores.includes(s);
                return (
                  <li key={s}>
                    <button type="button" className={`nav-item nav-store ${on ? 'is-on' : ''}`} aria-pressed={on} onClick={() => L.actions.toggleStore(s)}>
                      <span className="nav-check" aria-hidden>{on ? <Check size={11} strokeWidth={3} /> : <StoreGlyph store={s} size={14} />}</span>
                      <span className="nav-label">{STORES[s].name}</span>
                      <span className="nav-n">{counts[s]}</span>
                    </button>
                  </li>
                );
              })}
              {L.stores.length > 0 && <li><button type="button" className="nav-clear" onClick={() => L.actions.setStores([])}>Show every store</button></li>}
            </ul>
          )}
          <button type="button" className="nav-add" onClick={() => L.actions.say('Stores are added as plugins from the Plugins view', 'info')}>Add a store…</button>
        </nav>

        <aside className="list" aria-label="Games">
          <div className="list-tools">
            <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder={`Search ${VIEWS.find((v) => v.id === L.stateFilter).label.toLowerCase()}`} onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <Select label="Sort" className="list-sort" value={L.sort} onChange={L.actions.setSort} options={[{ value: 'recent', label: 'Recent' }, { value: 'title', label: 'A–Z' }, { value: 'hours', label: 'Most played' }]} />
          </div>
          <ul className="list-body">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="row-art" />
                  <span className="row-text">
                    <span className="row-title">{x.title}</span>
                    <span className="row-sub">{x.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={10} />)}<span>{isInstalled(x) ? ago(x.lastAgo) : 'Not installed'}</span></span>
                  </span>
                  <AttnMark L={L} x={x} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="list-empty">Nothing here. <button type="button" onClick={L.actions.resetFilters}>Show all games</button></li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero"><Img key={a.hero} src={a.hero} className="hero-img" /></div>
          <div className="head">
            <Img key={a.cover} src={a.cover} className="head-cover" />
            <div className="head-main">
              <h1>{g.title}</h1>
              <p className="head-by">{g.dev}, {g.year}</p>
              <p className="head-desc">{g.desc}</p>
            </div>
          </div>
          <div className="actions">
            {run ? (
              <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={16} className="spin" /> : <Square size={13} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
            ) : copy.installed ? (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={16} fill="currentColor" /><span>Play</span></button>
            ) : (
              <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Download size={16} /><span>Install</span><span className="play-t">{copy.size}</span></button>
            )}
            <button type="button" className="ghost" onClick={() => setSettings(true)}><Settings2 size={16} />Settings</button>
            {g.stores.length > 1 && (
              <div className="src" role="radiogroup" aria-label="Launch from">
                {g.stores.map((s) => (
                  <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed · ${s.version}` : `Not installed · ${s.size}`}>
                    <StoreGlyph store={s.store} size={13} />{STORES[s.store].name}
                  </button>
                ))}
              </div>
            )}
            {g.sync === 'conflict' ? (
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict, resolve</button>
            ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
            <dl className="meta">
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Runs with</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
              <div><dt>ProtonDB</dt><dd>{TIER_LABEL[g.tier]}</dd></div>
            </dl>
          </div>
          <div className="readout">
            <Tabs label="Game details" tabs={readoutTabs(L)} value={tab} onChange={setTab} />
            <div className="readout-body" role="tabpanel"><ReadoutBody L={L} tab={tab} /></div>
          </div>
        </main>
      </div>

      <footer className="tele" aria-label="Background activity">
        {L.running && <span className="tele-item is-live"><span className="tele-dot" />Playing {L.games.find((x) => x.id === L.running.id)?.title}<b>{elapsed(Date.now() - L.running.startedAt)}</b></span>}
        {L.jobs.map((j) => <span key={j.id} className="tele-item" style={{ '--p': j.progress }}>{j.label}<b>{Math.round(j.progress * 100)}%</b></span>)}
      </footer>

      {settings && <SettingsModal L={L} onClose={() => setSettings(false)} />}
      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
