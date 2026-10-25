import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import './d21.css';
import { Play, Square, Search, AlertTriangle, Loader, Cloud, CloudOff, HardDrive, RefreshCw, LibraryBig, ArrowDownToLine, CircleAlert, SlidersHorizontal, Gauge, Activity, MonitorPlay, Sparkles, X, ChevronDown, Plus, Download, ChevronRight, Droplet, PaintBucket } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, vivid, toneAt, VIEWS } from '../../core/launcher.js';
import { StoreGlyph, SYNC_LABEL, TIER_LABEL, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, SessionsTable, SavesTable, NotesList, elapsed, AttnMark, viewCount, SETTINGS_CATS, SettingsCategory, catSummary, Select, runnerOptions } from '../../core/kit.jsx';
import { StoreAbout, Gallery, AchSummary, AchList, GameFacts, achievementsOf, pageOf } from '../../core/page.jsx';
import { STORES } from '../../core/data.js';

const VIEW_ICON = { all: LibraryBig, installed: HardDrive, downloads: ArrowDownToLine, attention: CircleAlert };
const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

// Quick settings: a desktop flyout. A preset for people who don't want to think,
// the four switches behind it for people who do, and a way into everything else.
const PRESETS = [
  { id: 'quality', label: 'Quality', set: { gamemode: true, gamescope: false, fsr: false } },
  { id: 'balanced', label: 'Balanced', set: { gamemode: true, gamescope: true, fsr: false } },
  { id: 'speed', label: 'Performance', set: { gamemode: true, gamescope: true, fsr: true } },
];
export function QuickSettings({ L, onAll, onClose }) {
  const g = L.game;
  const ref = useRef();
  useEffect(() => {
    ref.current?.querySelector('button')?.focus();
    const close = (e) => { if (!ref.current?.contains(e.target) && !e.target.closest('.qs-anchor') && !e.target.closest('.sel-list')) onClose(); };
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) onClose(); };
    window.addEventListener('pointerdown', close);
    window.addEventListener('keydown', key);
    return () => { window.removeEventListener('pointerdown', close); window.removeEventListener('keydown', key); };
  }, []);
  const rows = [
    { k: 'gamemode', label: 'Performance mode', sub: 'Keeps CPU and GPU at full speed', icon: Gauge },
    { k: 'mangohud', label: 'FPS overlay', sub: 'Frame rate and temperatures in a corner', icon: Activity },
    { k: 'gamescope', label: 'Lock resolution', sub: '2560×1440 at 165 Hz', icon: MonitorPlay },
    { k: 'fsr', label: 'Sharper upscaling', sub: 'Renders lower, sharpens the image', icon: Sparkles, needs: 'gamescope' },
  ];
  const preset = PRESETS.find((p) => Object.entries(p.set).every(([k, v]) => !!g[k] === v))?.id || null;
  const apply = (p) => Object.entries(p.set).forEach(([k, v]) => L.actions.setOpt(g.id, k, v));
  return (
    <div className="qs" ref={ref} role="dialog" aria-label={`Quick settings for ${g.title}`}>
      <div className="qs-sec">
        <div className="qs-h"><span>Preset</span><small>{preset ? 'Applies three of the switches below' : 'Custom'}</small></div>
        <div className="qs-seg" role="radiogroup" aria-label="Preset">
          {PRESETS.map((p) => <button key={p.id} type="button" role="radio" aria-checked={preset === p.id} className={preset === p.id ? 'is-on' : ''} onClick={() => apply(p)}>{p.label}</button>)}
        </div>
      </div>
      <ul className="qs-rows">
        {rows.map((t) => {
          const Icon = t.icon;
          const dis = t.needs && !g[t.needs];
          const on = !!g[t.k] && !dis;
          return (
            <li key={t.k}>
              <button type="button" role="switch" aria-checked={on} disabled={dis} className={`qs-row ${on ? 'is-on' : ''}`} onClick={() => L.actions.toggleOpt(g.id, t.k)}>
                <Icon size={16} className="qs-ico" />
                <span className="qs-text"><span>{t.label}</span><small>{dis ? 'Turn on Lock resolution first' : t.sub}</small></span>
                <span className="tgl-track" aria-hidden><span className="tgl-thumb" /></span>
              </button>
            </li>
          );
        })}
      </ul>
      <div className="qs-runner">
        <span className="qs-text"><span>Compatibility layer</span><small>{TIER_LABEL[g.tier]} on ProtonDB</small></span>
        <Select label="Compatibility layer" value={g.runner} options={runnerOptions(L.runners, g.tier === 'native' || g.runner === 'native')} onChange={(v) => L.actions.setRunner(g.id, v)} />
      </div>
      <button type="button" className="qs-all" onClick={onAll}><span>All settings</span><small>Saves, fixes, environment, launch command</small><ChevronRight size={15} /></button>
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
    <div className="scrim d21-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
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

const useStored = (key, init) => {
  const [v, setV] = useState(() => { try { return JSON.parse(localStorage.getItem(key)) ?? init; } catch { return init; } });
  useEffect(() => { localStorage.setItem(key, JSON.stringify(v)); }, [v]);
  return [v, setV];
};

export default function D21() {
  const L = useLauncher({ initial: 'fallout3' });
  useTick(1000);
  const [tab, setTab] = useState('store');
  const stage = useRef();
  const [quick, setQuick] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const g = L.game;
  const a = artOf(g);
  const copy = activeCopy(g);
  const run = L.running?.id === g.id ? L.running : null;
  const tone = useArtTone(a.hero);
  const SyncIcon = SYNC_ICON[g.sync];
  const [tint, setTint] = useStored('d21-tint', 'accent');
  const vars = tint === 'whole' ? {
    // the whole app takes the game's hue
    '--acc': toneAt(tone, 0.9, 0.66, '#9db0ff'),
    '--acc-dim': toneAt(tone, 0.42, 0.2, '#1f232a'),
    '--s0': toneAt(tone, 0.5, 0.06, '#13161b'),
    '--s1': toneAt(tone, 0.48, 0.08, '#15181d'),
    '--s-pop': toneAt(tone, 0.42, 0.13, '#1b1f26'),
    '--k-bg': toneAt(tone, 0.46, 0.1, '#181b21'),
    '--k-bg-2': toneAt(tone, 0.42, 0.14, '#1f232a'),
    '--k-bg-3': toneAt(tone, 0.38, 0.2, '#293039'),
    '--k-line': toneAt(tone, 0.34, 0.17, '#262b33'),
    '--k-line-strong': toneAt(tone, 0.3, 0.26, '#36404b'),
    '--k-text': toneAt(tone, 0.3, 0.95, '#e9edf2'),
    '--k-muted': toneAt(tone, 0.25, 0.72, '#9ca7b5'),
    '--k-faint': toneAt(tone, 0.22, 0.52, '#687384'),
    '--k-on-accent': toneAt(tone, 0.5, 0.07, '#0f1218'),
  } : { '--acc': vivid(tone, 0.7, 0.6) || '#9db0ff', '--acc-dim': toneAt(tone, 0.45, 0.17, '#1f232a') };
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  const dl = L.jobs.find((j) => j.kind === 'download');
  const ach = achievementsOf(g);
  useEffect(() => { stage.current?.scrollTo({ top: 0 }); }, [g.id]);
  const tabs = [
    { id: 'store', label: 'Store page' },
    ...(ach ? [{ id: 'achievements', label: 'Achievements', count: `${ach.unlocked}/${ach.total}` }] : []),
    { id: 'sessions', label: 'Activity', count: L.sessions.length },
    { id: 'saves', label: 'Saves', count: g.saves.length },
    { id: 'compat', label: 'Compatibility', count: g.notes.length },
  ];
  const curTab = tabs.some((t) => t.id === tab) ? tab : 'store';

  return (
    <div className="k d21" style={vars}>
      <header className="tb">
        <div className="tb-brand"><span className="tb-mark" aria-hidden>P</span>Platypus</div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>Search or run a command</span><kbd>Ctrl K</kbd></button>
        <div className="tb-right">
          <div className="tint" role="radiogroup" aria-label="Game colour">
            <button type="button" role="radio" aria-checked={tint === 'accent'} className={tint === 'accent' ? 'is-on' : ''} onClick={() => setTint('accent')} title="The game's colour on buttons and highlights"><Droplet size={13} />Accent</button>
            <button type="button" role="radio" aria-checked={tint === 'whole'} className={tint === 'whole' ? 'is-on' : ''} onClick={() => setTint('whole')} title="The game's colour across the whole app"><PaintBucket size={13} />Whole app</button>
          </div>
        </div>
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

        <main className="stage" ref={stage}>
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
              <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict, resolve</button>
            ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
          </div>
          <div className="readout">
            <Tabs label="Game page" tabs={tabs} value={curTab} onChange={setTab} />
            <div className="readout-body" role="tabpanel" key={`${g.id}-${curTab}`}>
              {curTab === 'store' && (
                <div className="storepg">
                  <div className="storepg-main">
                    <StoreAbout g={g} clamp={760} />
                  </div>
                  <aside className="storepg-side">
                    {pageOf(g)?.shots.length > 0 && <section className="side-sec"><h2>Pictures<small>{pageOf(g).shots.length} screenshots</small></h2><Gallery g={g} variant="sheet" max={7} /></section>}
                    <dl className="you">
                      <div><dt>Played</dt><dd>{Math.round(g.hours)} h</dd></div>
                      <div><dt>Last played</dt><dd>{ago(g.lastAgo)}</dd></div>
                      <div><dt>Runs with</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
                      <div><dt>ProtonDB</dt><dd>{TIER_LABEL[g.tier]}</dd></div>
                    </dl>
                    {ach && <section className="side-sec"><h2>Achievements</h2><AchSummary g={g} onOpen={() => setTab('achievements')} /></section>}
                    <section className="side-sec"><h2>Details</h2><GameFacts g={g} /></section>
                  </aside>
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

      {drawer && <SettingsDrawer L={L} onClose={() => setDrawer(false)} />}
      <Palette L={L} />
      <ConflictDialog L={L} />
      <Toast L={L} />
    </div>
  );
}
