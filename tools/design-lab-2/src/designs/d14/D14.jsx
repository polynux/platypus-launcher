import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import './d14.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, LibraryBig, ArrowDownToLine, CircleAlert, SlidersHorizontal, Gauge, Activity, MonitorPlay, Sparkles, X, ChevronDown, Plus, Download } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, vivid, toneAt, VIEWS } from '../../core/launcher.js';
import { StoreGlyph, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, readoutTabs, ReadoutBody, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary, Select, runnerOptions } from '../../core/kit.jsx';
import { STORES } from '../../core/data.js';

const VIEW_ICON = { all: LibraryBig, installed: HardDrive, downloads: ArrowDownToLine, attention: CircleAlert };
const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

// Quick settings: the four things people actually change, as big tiles.
function QuickSettings({ L, onAll, onClose }) {
  const g = L.game;
  const ref = useRef();
  useEffect(() => {
    ref.current?.querySelector('button')?.focus();
    const close = (e) => { if (!ref.current?.contains(e.target) && !e.target.closest('.qs-anchor')) onClose(); };
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) onClose(); };
    window.addEventListener('pointerdown', close);
    window.addEventListener('keydown', key);
    return () => { window.removeEventListener('pointerdown', close); window.removeEventListener('keydown', key); };
  }, []);
  const tiles = [
    { k: 'gamemode', label: 'Performance mode', icon: Gauge },
    { k: 'mangohud', label: 'FPS overlay', icon: Activity },
    { k: 'gamescope', label: 'Lock resolution', icon: MonitorPlay },
    { k: 'fsr', label: 'Sharper upscaling', icon: Sparkles, needs: 'gamescope' },
  ];
  return (
    <div className="qs" ref={ref} role="dialog" aria-label="Quick settings">
      <div className="qs-tiles">
        {tiles.map((t) => {
          const Icon = t.icon;
          const dis = t.needs && !g[t.needs];
          const on = g[t.k] && !dis;
          return (
            <button key={t.k} type="button" role="switch" aria-checked={!!on} disabled={dis} className={`qs-tile ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleOpt(g.id, t.k)}>
              <Icon size={18} /><span>{t.label}</span><small>{dis ? 'Needs Lock resolution' : on ? 'On' : 'Off'}</small>
            </button>
          );
        })}
      </div>
      <div className="qs-runner">
        <span>Compatibility</span>
        <Select label="Compatibility" value={g.runner} options={runnerOptions(L.runners, g.tier === 'native' || g.runner === 'native')} onChange={(v) => L.actions.setRunner(g.id, v)} />
      </div>
      <button type="button" className="qs-all" onClick={onAll}>All settings<span>Saves, fixes, environment, launch command</span></button>
    </div>
  );
}

function SettingsDrawer({ L, onClose }) {
  const [open, setOpen] = useState('compat');
  const ref = useRef();
  useEffect(() => {
    ref.current?.querySelector('.acc-h')?.focus();
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) onClose(); };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, []);
  return (
    <div className="scrim d14-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <aside className="drawer" ref={ref} role="dialog" aria-modal="true" aria-labelledby="drawer-title">
        <header className="drawer-head">
          <div><h2 id="drawer-title">All settings</h2><span>{L.game.title}</span></div>
          <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}><X size={16} /></button>
        </header>
        <div className="drawer-body">
          {['basic', 'advanced'].map((lvl) => (
            <div key={lvl}>
              <div className="acc-group">{lvl === 'basic' ? 'Everyday' : 'Advanced'}</div>
              {SETTINGS_CATS.filter((c) => c.level === lvl).map((c) => {
                const Icon = c.icon;
                const isOpen = open === c.id;
                return (
                  <section key={c.id} className={`acc ${isOpen ? 'is-open' : ''}`}>
                    <h3>
                      <button type="button" className="acc-h" aria-expanded={isOpen} onClick={() => setOpen(isOpen ? null : c.id)}>
                        <Icon size={16} /><span className="acc-label">{c.label}</span><span className="acc-sum">{catSummary(L, c.id)}</span><ChevronDown size={15} className="acc-chev" />
                      </button>
                    </h3>
                    {isOpen && <div className="acc-body"><SettingsCategory L={L} id={c.id} /></div>}
                  </section>
                );
              })}
            </div>
          ))}
        </div>
      </aside>
    </div>
  );
}

export default function D14() {
  const L = useLauncher({ initial: 'fallout3' });
  useTick(1000);
  const [tab, setTab] = useState('sessions');
  const [quick, setQuick] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const vars = { '--acc': vivid(tone, 0.7, 0.6) || '#9db0ff', '--acc-dim': toneAt(tone, 0.45, 0.17, '#1f232a') };
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const dl = L.jobs.find((j) => j.kind === 'download');

  return (
    <div className="k d14" style={vars}>
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden>P</span>Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>Search or run a command</span><kbd>Ctrl K</kbd></button>
        <span />
      </header>

      <nav className="sources" aria-label="Library and stores">
        {VIEWS.map((v) => {
          const Icon = VIEW_ICON[v.id];
          const n = viewCount(L, v.id);
          const on = L.stateFilter === v.id && !L.stores.length;
          return (
            <button key={v.id} type="button" className={`src-tab ${on ? 'is-on' : ''} ${v.id === 'attention' && n ? 'is-warn' : ''}`} aria-pressed={on} onClick={() => { L.actions.setStores([]); L.actions.setStateFilter(v.id); }}>
              <Icon size={15} />{v.label}
              {v.id === 'downloads' && dl ? <span className="src-prog" style={{ '--p': dl.progress }} aria-label={`${Math.round(dl.progress * 100)}%`} /> : <span className="src-n">{n}</span>}
            </button>
          );
        })}
        <span className="sources-sep" aria-hidden />
        <div className="sources-stores">
          {Object.keys(STORES).filter((s) => counts[s]).map((s) => {
            const on = L.stores.includes(s);
            return (
              <button key={s} type="button" className={`src-tab ${on ? 'is-on' : ''}`} aria-pressed={on} title="Shift-click to combine stores" onClick={(e) => { L.actions.setStateFilter('all'); e.shiftKey ? L.actions.toggleStore(s) : L.actions.setStores(on && L.stores.length === 1 ? [] : [s]); }}>
                <StoreGlyph store={s} size={14} />{STORES[s].name}<span className="src-n">{counts[s]}</span>
              </button>
            );
          })}
          <button type="button" className="src-tab src-add" onClick={() => L.actions.say('Stores are added as plugins from the Plugins view', 'info')}><Plus size={14} />Add store</button>
        </div>
      </nav>

      <div className="body">
        <aside className="list" aria-label="Games">
          <div className="list-tools">
            <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
            <Select label="Sort" className="list-sort" value={L.sort} onChange={L.actions.setSort} options={[{ value: 'recent', label: 'Recent' }, { value: 'title', label: 'Title' }, { value: 'hours', label: 'Most played' }]} />
          </div>
          <ul className="list-body">
            {L.visible.map((x) => (
              <li key={x.id}>
                <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                  <Img src={artOf(x).header} className="row-art" />
                  <span className="row-text"><span className="row-title">{x.title}</span><span className="row-sub">{isInstalled(x) ? ago(x.lastAgo) : 'Not installed'}</span></span>
                  <AttnMark L={L} x={x} />
                </button>
              </li>
            ))}
            {L.visible.length === 0 && <li className="list-empty">Nothing here. <button type="button" onClick={L.actions.resetFilters}>Show all games</button></li>}
          </ul>
        </aside>

        <main className="stage">
          <div className="hero">
            <Img key={a.hero} src={a.hero} className="hero-img" />
            <GameLogo g={g} className="hero-logo" />
          </div>
          <div className="strip">
            <div className="strip-play">
              {run ? (
                <button type="button" className="play is-running" onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={12} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : 'Stop'}</span><span className="play-t">{elapsed(Date.now() - run.startedAt)}</span></button>
              ) : copy.installed ? (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>Play</span></button>
              ) : (
                <button type="button" className="play" onClick={() => L.actions.play(g.id)}><Download size={15} /><span>Install</span><span className="play-t">{copy.size}</span></button>
              )}
              <div className="qs-anchor">
                <button type="button" className={`tune ${quick ? 'is-on' : ''}`} aria-expanded={quick} aria-haspopup="dialog" onClick={() => setQuick(!quick)}><SlidersHorizontal size={16} />Tune<ChevronDown size={14} /></button>
                {quick && <QuickSettings L={L} onClose={() => setQuick(false)} onAll={() => { setQuick(false); setDrawer(true); }} />}
              </div>
            </div>
            <div className="from">
              <span className="from-k">Launch from</span>
              {g.stores.length > 1 ? (
                <Select label="Launch from" className="from-sel" value={g.source} onChange={(v) => L.actions.setSource(g.id, v)} options={g.stores.map((s) => ({ value: s.store, label: STORES[s.store].name, hint: s.installed ? 'installed' : 'not installed' }))} />
              ) : <span className="from-one"><StoreGlyph store={copy.store} size={13} />{STORES[copy.store].name}</span>}
            </div>
            {g.sync === 'conflict' ? (
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict<span>Resolve</span></button>
            ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
          </div>
          <div className="about">
            <p>{g.desc}</p>
            <dl>
              <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
              <div><dt>Last played</dt><dd>{ago(g.lastAgo)}</dd></div>
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

      {drawer && <SettingsDrawer L={L} onClose={() => setDrawer(false)} />}
      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
