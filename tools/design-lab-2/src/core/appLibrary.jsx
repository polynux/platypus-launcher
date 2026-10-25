// Library and game page pieces for the full-app designs. Unstyled by default
// (app.css gives structure); each design restyles under its root class.
import React, { useEffect, useRef, useState } from 'react';
import { Play, Square, Loader, Download, RotateCcw, ArrowDownToLine, Star, Settings2, FolderPlus, Check, Plus, Pin, PinOff, Pencil, Trash2, ArrowUp, ArrowDown, ListFilter, LayoutGrid, Rows3, Table2, ChevronDown, Clock, HardDrive, Sparkles, X, AlertTriangle, Users, ExternalLink, FolderOpen, GitCompare, Info, ArrowUpDown, LogIn, CircleAlert, Wand2, Wine, GitBranch, Puzzle, Plug, Palette as PaletteIcon, Hammer, Scale } from 'lucide-react';
import { STORES, RECOMMEND, RECIPES } from './data.js';
import { runnerOf, activeCopy, isInstalled, artOf, ago, dayLabel, clock, dur } from './launcher.js';
import { StoreGlyph, Img, AttnMark, TIER_LABEL, SYNC_LABEL, Select, SRow, SSwitch, SETTINGS_CATS, SettingsCategory, catSummary, SavesTable, Tabs } from './kit.jsx';
import { achievementsOf, pageOf } from './page.jsx';
import { BUILTIN_COLLECTIONS, GHOSTS, LENT, OWN_STATUS, ISSUES, CAPS, can, collectionIds, FRIENDS } from './app.js';

// ---------- small primitives ----------
export function Popover({ open, onClose, className = '', label, children, align = 'left' }) {
  const ref = useRef();
  useEffect(() => {
    if (!open) return;
    const anchor = ref.current?.parentElement;
    const down = (e) => { if (!anchor?.contains(e.target) && !e.target.closest('.sel-list')) onClose(); };
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) { e.stopPropagation(); onClose(); anchor?.querySelector('button')?.focus(); } };
    window.addEventListener('pointerdown', down);
    window.addEventListener('keydown', key, true);
    return () => { window.removeEventListener('pointerdown', down); window.removeEventListener('keydown', key, true); };
  }, [open]);
  if (!open) return null;
  return <div ref={ref} className={`ap-pop is-${align} ${className}`} role="dialog" aria-label={label} data-keys-local>{children}</div>;
}

export function Seg({ value, options, onChange, label, className = '' }) {
  return (
    <div className={`ap-seg ${className}`} role="radiogroup" aria-label={label}>
      {options.map((o) => {
        const Icon = o.icon;
        return (
          <button key={o.value} type="button" role="radio" aria-checked={value === o.value} className={value === o.value ? 'is-on' : ''} onClick={() => onChange(o.value)} title={o.title} aria-label={o.iconOnly ? o.label : undefined}>
            {Icon && <Icon size={14} />}{!o.iconOnly && o.label}{o.count != null && <span className="ap-seg-n">{o.count}</span>}
          </button>
        );
      })}
    </div>
  );
}

export function Check2({ checked, onChange, children, hint }) {
  return (
    <button type="button" role="checkbox" aria-checked={checked} className={`ap-check ${checked ? 'is-on' : ''}`} onClick={() => onChange(!checked)}>
      <span className="ap-check-box" aria-hidden>{checked && <Check size={11} strokeWidth={3} />}</span>
      <span className="ap-check-label">{children}</span>{hint != null && <span className="ap-check-hint">{hint}</span>}
    </button>
  );
}

export function Switch({ checked, onChange, label, desc, disabled }) {
  return (
    <button type="button" role="switch" aria-checked={checked} disabled={disabled} className={`srow sswitch ${checked ? 'is-on' : ''}`} onClick={() => onChange(!checked)}>
      <span className="srow-text"><span className="srow-label">{label}</span>{desc && <span className="srow-desc">{desc}</span>}</span>
      <span className="tgl-track" aria-hidden><span className="tgl-thumb" /></span>
    </button>
  );
}

export function Empty({ icon: Icon = Info, title, children, action }) {
  return (
    <div className="ap-empty">
      <Icon size={22} className="ap-empty-ico" />
      <h3>{title}</h3>
      {children && <p>{children}</p>}
      {action}
    </div>
  );
}

export function Skeleton({ rows = 6, kind = 'rows' }) {
  return (
    <div className={`ap-skel is-${kind}`} aria-busy="true" aria-label="Loading">
      {Array.from({ length: rows }, (_, i) => <span key={i} className="ap-skel-i" style={{ '--i': i }} />)}
    </div>
  );
}

export const storeName = (s) => STORES[s]?.name || s;

// ---------- library controls ----------
export function OwnSwitch({ L, A }) {
  const all = L.games.length;
  const inst = L.games.filter(isInstalled).length;
  return <Seg label="Ownership" value={A.own} onChange={A.actions.setOwn} options={[{ value: 'all', label: A.t.allGames, count: all }, { value: 'installed', label: A.t.installed, count: inst }]} />;
}

export function storeCounts(L) {
  const counts = {};
  for (const x of L.games) for (const s of x.stores) counts[s.store] = (counts[s.store] || 0) + 1;
  return counts;
}

// One tab per store with its count, Sideloaded last. Shift-click combines.
export function StoreTabs({ L, A, className = '' }) {
  const counts = storeCounts(L);
  const warn = Object.fromEntries(A.plugins.filter((p) => p.status !== 'ready').map((p) => [p.id, p]));
  const stores = Object.keys(STORES).filter((s) => counts[s] && s !== 'local');
  return (
    <div className={`ap-stores ${className}`} role="group" aria-label="Stores">
      <button type="button" className={`ap-store ${!L.stores.length && !A.sideloaded ? 'is-on' : ''}`} aria-pressed={!L.stores.length && !A.sideloaded} onClick={() => { L.actions.setStores([]); A.actions.setSideloaded(false); }}>Every store</button>
      {stores.map((s) => {
        const on = L.stores.includes(s);
        return (
          <button key={s} type="button" className={`ap-store ${on ? 'is-on' : ''}`} aria-pressed={on} title="Shift-click to combine stores" onClick={(e) => { A.actions.setSideloaded(false); e.shiftKey ? L.actions.toggleStore(s) : L.actions.setStores(on && L.stores.length === 1 ? [] : [s]); }}>
            <StoreGlyph store={s} size={13} />{storeName(s)}<span className="ap-n">{counts[s]}</span>{warn[s] && <span className={`ap-dot is-${warn[s].status === 'error' ? 'bad' : 'warn'}`} title={warn[s].why} />}
          </button>
        );
      })}
      <button type="button" className={`ap-store ${A.sideloaded ? 'is-on' : ''}`} aria-pressed={A.sideloaded} onClick={() => { L.actions.setStores([]); A.actions.setSideloaded(!A.sideloaded); }}>
        <HardDrive size={13} />{A.t.sideloaded}<span className="ap-n">{counts.local || 0}</span>
      </button>
    </div>
  );
}

export const allCollections = (A) => [...BUILTIN_COLLECTIONS, ...A.collections];
export const collectionName = (A, id) => allCollections(A).find((c) => c.id === id)?.name;

const COL_ICON = { fav: Star, installed: HardDrive, recent: Clock };
export function CollectionRow({ L, A, c, on, manage, onPick }) {
  const Icon = COL_ICON[c.id] || (c.kind === 'smart' ? Wand2 : null);
  const ids = collectionIds(c, L.games, A.favs);
  const missing = ids.filter((id) => !L.games.some((g) => g.id === id)).length;
  return (
    <li className={`ap-col ${on ? 'is-on' : ''}`}>
      <button type="button" className="ap-col-pick" aria-pressed={on} onClick={() => onPick(on ? null : c.id)}>
        {Icon ? <Icon size={14} className="ap-col-ico" /> : <span className="ap-col-sw" style={{ '--h': (c.name.length * 53) % 360 }} aria-hidden />}
        <span className="ap-col-name">{c.name}</span>
        {c.pinned && <Pin size={11} className="ap-col-pin" aria-label="Pinned" />}
        <span className="ap-n">{ids.length - missing}</span>
      </button>
      {manage && c.kind !== 'builtin' && <CollectionManage A={A} c={c} />}
    </li>
  );
}

function CollectionManage({ A, c }) {
  const [open, setOpen] = useState(false);
  const C = A.actions;
  return (
    <span className="ap-anchor">
      <button type="button" className="icon-btn ap-col-more" aria-label={`Manage ${c.name}`} aria-expanded={open} onClick={() => setOpen(!open)}><Settings2 size={13} /></button>
      <Popover open={open} onClose={() => setOpen(false)} label={`Manage ${c.name}`} className="ap-menu" align="right">
        <button type="button" onClick={() => { C.pinCollection(c.id); setOpen(false); }}>{c.pinned ? <PinOff size={14} /> : <Pin size={14} />}{c.pinned ? 'Unpin' : 'Pin to top'}</button>
        <button type="button" onClick={() => { C.open('collection', { edit: c.id }); setOpen(false); }}><Pencil size={14} />Rename{c.kind === 'smart' ? ' or edit rule' : ''}</button>
        <button type="button" onClick={() => C.moveCollection(c.id, -1)}><ArrowUp size={14} />Move up</button>
        <button type="button" onClick={() => C.moveCollection(c.id, 1)}><ArrowDown size={14} />Move down</button>
        <button type="button" className="is-bad" onClick={() => { C.open('confirm', { title: `Delete “${c.name}”?`, body: 'The games stay in your library. Only the collection goes.', yes: 'Delete collection', run: () => C.deleteCollection(c.id) }); setOpen(false); }}><Trash2 size={14} />Delete</button>
      </Popover>
    </span>
  );
}

// Pinned first, then the rest in your order.
export function CollectionsList({ L, A, manage = true, onPick }) {
  const pick = onPick || A.actions.setCollection;
  const pinned = A.collections.filter((c) => c.pinned);
  const rest = A.collections.filter((c) => !c.pinned);
  return (
    <ul className="ap-cols">
      {BUILTIN_COLLECTIONS.map((c) => <CollectionRow key={c.id} L={L} A={A} c={c} on={A.collection === c.id} manage={manage} onPick={pick} />)}
      {pinned.length > 0 && <li className="ap-cols-sep" role="presentation" />}
      {pinned.map((c) => <CollectionRow key={c.id} L={L} A={A} c={c} on={A.collection === c.id} manage={manage} onPick={pick} />)}
      {rest.map((c) => <CollectionRow key={c.id} L={L} A={A} c={c} on={A.collection === c.id} manage={manage} onPick={pick} />)}
      <li><button type="button" className="ap-col-new" onClick={() => A.actions.open('collection')}><Plus size={14} />{A.t.newCollection}</button></li>
    </ul>
  );
}

export function CollectionsMenu({ L, A }) {
  const [open, setOpen] = useState(false);
  const cur = collectionName(A, A.collection);
  return (
    <span className="ap-anchor">
      <button type="button" className={`ap-tool ${A.collection ? 'is-set' : ''}`} aria-expanded={open} onClick={() => setOpen(!open)}><FolderPlus size={14} />{cur || A.t.collections}<ChevronDown size={13} /></button>
      <Popover open={open} onClose={() => setOpen(false)} label="Collections" className="ap-colpop">
        <CollectionsList L={L} A={A} onPick={(id) => { A.actions.setCollection(id); setOpen(false); }} />
      </Popover>
    </span>
  );
}

export const TIERS = ['native', 'platinum', 'gold', 'silver', 'bronze', 'borked'];
export function FilterPanel({ L, A }) {
  const C = A.actions;
  const n = (fn) => L.games.filter(fn).length;
  return (
    <div className="ap-filters">
      <fieldset>
        <legend>Runs on Linux</legend>
        {TIERS.map((t) => <Check2 key={t} checked={A.tiers.includes(t)} onChange={() => C.toggleTier(t)} hint={n((g) => g.tier === t)}>{t === 'native' ? 'Native' : `${TIER_LABEL[t]} on ProtonDB`}</Check2>)}
      </fieldset>
      <fieldset>
        <legend>Your status</legend>
        {OWN_STATUS.map((s) => <Check2 key={s.id} checked={A.statuses.includes(s.id)} onChange={() => C.toggleStatus(s.id)} hint={n((g) => (A.status[g.id] || 'none') === s.id)}>{s.label}</Check2>)}
      </fieldset>
      <fieldset>
        <legend>Family</legend>
        <Check2 checked={A.lentOnly} onChange={C.setLentOnly} hint={Object.keys(LENT).length}>Lent to me</Check2>
      </fieldset>
      <div className="ap-filters-foot"><button type="button" className="btn btn-quiet" disabled={!A.activeFilters} onClick={C.clearFilters}>Clear filters</button></div>
    </div>
  );
}

export function FilterButton({ L, A, align = 'left' }) {
  const [open, setOpen] = useState(false);
  return (
    <span className="ap-anchor">
      <button type="button" className={`ap-tool ${A.activeFilters ? 'is-set' : ''}`} aria-expanded={open} onClick={() => setOpen(!open)}><ListFilter size={14} />{A.t.filters}{A.activeFilters > 0 && <span className="ap-badge">{A.activeFilters}</span>}</button>
      <Popover open={open} onClose={() => setOpen(false)} label="Filters" align={align}><FilterPanel L={L} A={A} /></Popover>
    </span>
  );
}

export const SORT_OPTIONS = [
  { value: 'recent', label: 'Recent' },
  { value: 'title', label: 'Title' },
  { value: 'hours', label: 'Hours played' },
  { value: 'year', label: 'Year' },
  { value: 'size', label: 'Size' },
  { value: 'tier', label: 'Runs best' },
];
export function SortMenu({ L, className = '' }) {
  const key = L.sort.replace(/-desc$/, '');
  const desc = L.sort.endsWith('-desc');
  return (
    <span className={`ap-sort ${className}`}>
      <Select label="Sort" value={key} options={SORT_OPTIONS} onChange={(v) => L.actions.setSort(desc ? `${v}-desc` : v)} />
      <button type="button" className="icon-btn" aria-label={desc ? 'Reverse order on' : 'Reverse order'} aria-pressed={desc} title="Reverse order" onClick={() => L.actions.setSort(desc ? key : `${key}-desc`)}><ArrowUpDown size={14} /></button>
    </span>
  );
}

export function ViewSwitch({ A, withList = true }) {
  const opts = [
    ...(withList ? [{ value: 'list', label: 'List', icon: Rows3, iconOnly: true, title: 'List with the game beside it' }] : []),
    { value: 'grid', label: 'Covers', icon: LayoutGrid, iconOnly: true, title: 'Grid of covers' },
    { value: 'table', label: 'Table', icon: Table2, iconOnly: true, title: 'Table' },
  ];
  return <Seg label="View" value={A.view} onChange={A.actions.setView} options={opts} className="ap-viewsw" />;
}

// Filters currently narrowing the list, each removable.
export function ActiveChips({ L, A }) {
  const C = A.actions;
  const chips = [
    A.collection && { k: 'c', label: collectionName(A, A.collection), off: () => C.setCollection(null) },
    ...L.stores.map((s) => ({ k: s, label: storeName(s), off: () => L.actions.toggleStore(s) })),
    ...A.tiers.map((t) => ({ k: t, label: TIER_LABEL[t], off: () => C.toggleTier(t) })),
    ...A.statuses.map((s) => ({ k: s, label: OWN_STATUS.find((x) => x.id === s)?.label, off: () => C.toggleStatus(s) })),
    A.lentOnly && { k: 'lent', label: 'Lent to me', off: () => C.setLentOnly(false) },
    L.query && { k: 'q', label: `“${L.query}”`, off: () => L.actions.setQuery('') },
  ].filter(Boolean);
  if (!chips.length) return null;
  return (
    <div className="ap-chips">
      {chips.map((c) => <button key={c.k} type="button" className="ap-chip" onClick={c.off} aria-label={`Remove filter ${c.label}`}>{c.label}<X size={11} /></button>)}
      <button type="button" className="ap-chip-clear" onClick={C.resetAll}>Reset</button>
    </div>
  );
}

// Stores whose list could not load or need a login.
export function SourceWarnings({ L, A, compact }) {
  const bad = A.plugins.filter((p) => p.status === 'needs-login' || p.status === 'error');
  if (!bad.length || A.dismissed.includes('srcwarn')) return null;
  return (
    <div className={`ap-srcwarn ${compact ? 'is-compact' : ''}`} role="status">
      <CircleAlert size={15} />
      <span className="ap-srcwarn-text">
        {bad.map((p, i) => (
          <span key={p.id}>{i > 0 && ' '}<b>{p.name}</b> {p.status === 'error' ? (compact ? 'list failed.' : 'could not load its list.') : compact ? 'needs a login.' : `needs you to log in. ${p.games} games can’t update.`}</span>
        ))}
      </span>
      <button type="button" className="btn btn-quiet" onClick={() => A.actions.go('plugins')}>{compact ? 'Fix' : 'Fix in Plugins'}</button>
      <button type="button" className="icon-btn" aria-label="Hide for now" onClick={() => A.actions.dismiss('srcwarn')}><X size={13} /></button>
    </div>
  );
}

export function LibraryEmpty({ L, A }) {
  if (A.demo === 'empty') return <Empty icon={Sparkles} title="Your library is empty" action={<div className="ap-row"><button type="button" className="btn btn-primary" onClick={() => A.actions.go('plugins')}><LogIn size={14} />Connect a store</button><button type="button" className="btn" onClick={() => A.actions.open('add-game')}><Plus size={14} />{A.t.addGame}</button></div>}>Connect Steam, GOG or another store to bring in the games you own, or add one from your disk.</Empty>;
  if (A.own === 'installed' && !L.games.some(isInstalled)) return <Empty icon={HardDrive} title="Nothing installed yet" action={<button type="button" className="btn" onClick={() => A.actions.setOwn('all')}>Show all owned games</button>}>Pick a game from your library and press Install.</Empty>;
  return <Empty icon={ListFilter} title="No game matches these filters" action={<button type="button" className="btn" onClick={A.actions.resetAll}>Reset filters</button>} />;
}

// ---------- per-game bits ----------
export function issueOf(A, g) { return A.issues.find((i) => i.gameId === g.id); }
export function jobOf(L, g) { return L.jobs.find((j) => j.gameId === g.id && (j.kind === 'download' || j.kind === 'verify')); }

export function QuickActions({ L, A, g, onSettings }) {
  const [open, setOpen] = useState(false);
  const fav = A.favs.includes(g.id);
  const manual = A.collections.filter((c) => c.kind === 'manual');
  const copy = activeCopy(g);
  return (
    <span className="ap-qa">
      <button type="button" className="ap-qa-play" aria-label={copy.installed ? `Play ${g.title}` : `Install ${g.title}`} onClick={(e) => { e.stopPropagation(); copy.installed ? L.actions.play(g.id) : A.actions.open('install', { gameId: g.id }); }}>{copy.installed ? <Play size={14} fill="currentColor" /> : <Download size={14} />}</button>
      <button type="button" className="icon-btn" aria-label={`Settings for ${g.title}`} onClick={(e) => { e.stopPropagation(); L.actions.select(g.id); onSettings?.(); }}><Settings2 size={14} /></button>
      <span className="ap-anchor">
        <button type="button" className="icon-btn" aria-label={`Collections for ${g.title}`} aria-expanded={open} onClick={(e) => { e.stopPropagation(); setOpen(!open); }}><FolderPlus size={14} /></button>
        <Popover open={open} onClose={() => setOpen(false)} label="Collections" className="ap-menu" align="right">
          <Check2 checked={fav} onChange={() => A.actions.toggleFav(g.id)}>Favourites</Check2>
          {manual.map((c) => <Check2 key={c.id} checked={c.ids.includes(g.id)} onChange={() => A.actions.toggleInCollection(c.id, g.id)}>{c.name}</Check2>)}
          <button type="button" onClick={() => { A.actions.open('collection', { add: g.id }); setOpen(false); }}><Plus size={14} />New collection</button>
        </Popover>
      </span>
    </span>
  );
}

export function GameBadges({ L, A, g }) {
  const lent = LENT[g.id];
  return (
    <>
      {A.favs.includes(g.id) && <Star size={12} className="ap-fav" fill="currentColor" aria-label="Favourite" />}
      {lent && <span className={`ap-lent ${lent.inUse ? 'is-busy' : ''}`} title={lent.inUse ? `${lent.owner} is playing it` : `Lent by ${lent.owner}`}><Users size={11} />{lent.owner}</span>}
      {issueOf(A, g) && <span className="ap-issue" title={issueOf(A, g).title}><AlertTriangle size={11} /></span>}
      <AttnMark L={L} x={g} />
    </>
  );
}

export function LibraryGrid({ L, A, onOpen, onSettings }) {
  return (
    <ul className="ap-grid" aria-label="Games">
      {L.visible.map((g) => (
        <li key={g.id} className={`ap-card ${isInstalled(g) ? '' : 'is-absent'} ${L.game.id === g.id ? 'is-sel' : ''}`}>
          <button type="button" className="ap-card-hit" data-game-row={g.id} onClick={() => onOpen(g.id)} aria-label={g.title}>
            <Img src={artOf(g).cover} className="ap-card-art" />
            {!artOf(g).cover && <span className="ap-card-fallback">{g.title}</span>}
          </button>
          <span className="ap-card-badges"><GameBadges L={L} A={A} g={g} /></span>
          <QuickActions L={L} A={A} g={g} onSettings={onSettings} />
          <span className="ap-card-title">{g.title}</span>
          <span className="ap-card-sub">{isInstalled(g) ? (g.hours ? `${Math.round(g.hours)} h · ${ago(g.lastAgo)}` : 'Not played') : 'Not installed'}</span>
        </li>
      ))}
    </ul>
  );
}

const STATUS_LABEL = Object.fromEntries(OWN_STATUS.map((s) => [s.id, s.label]));
export function LibraryTable({ L, A, onOpen, onSettings }) {
  const head = (k, label, cls = '') => {
    const cur = L.sort.replace(/-desc$/, '');
    const desc = L.sort.endsWith('-desc');
    return (
      <th className={cls} aria-sort={cur === k ? (desc ? 'descending' : 'ascending') : undefined}>
        {k ? <button type="button" className="ap-th" onClick={() => L.actions.setSort(cur === k && !desc ? `${k}-desc` : k)}>{label}{cur === k && (desc ? <ArrowUp size={11} /> : <ArrowDown size={11} />)}</button> : label}
      </th>
    );
  };
  return (
    <table className="tbl ap-table">
      <thead><tr>{head('title', 'Title')}<th>Stores</th>{head(null, 'Status')}{head('hours', 'Played', 'num')}{head('recent', 'Last played')}{head('size', 'Size', 'num')}{head('tier', 'Runs')}{head('year', 'Year', 'num')}<th aria-label="Actions" /></tr></thead>
      <tbody>
        {L.visible.map((g) => (
          <tr key={g.id} className={`${L.game.id === g.id ? 'is-sel' : ''} ${isInstalled(g) ? '' : 'is-absent'}`}>
            <td><button type="button" className="ap-tt" data-game-row={g.id} onClick={() => onOpen(g.id)}><Img src={artOf(g).header} className="ap-tt-art" /><span>{g.title}</span></button><GameBadges L={L} A={A} g={g} /></td>
            <td className="ap-tstores">{g.stores.map((s) => <StoreGlyph key={s.store} store={s.store} size={13} />)}</td>
            <td className="t-dim">{STATUS_LABEL[A.status[g.id] || 'none']}</td>
            <td className="num">{g.hours ? `${Math.round(g.hours)} h` : '—'}</td>
            <td className="t-dim">{isInstalled(g) ? ago(g.lastAgo) : '—'}</td>
            <td className="num t-dim">{activeCopy(g).size}</td>
            <td><span className={`tier-${g.tier}`}>{TIER_LABEL[g.tier]}</span></td>
            <td className="num t-dim">{g.year}</td>
            <td className="ap-tqa"><QuickActions L={L} A={A} g={g} onSettings={onSettings} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// "Last played", "Recently added" and "Activity" blocks above the grid.
export function LibraryBlocks({ L, A, onOpen }) {
  const last = [...L.games].filter(isInstalled).sort((a, b) => a.lastAgo - b.lastAgo).slice(0, 4);
  const added = ['deltarune', 'fallout3', 'civ7', 'teardown'].map((id) => L.games.find((g) => g.id === id)).filter(Boolean);
  return (
    <div className="ap-blocks">
      <section>
        <h2>Last played</h2>
        <ul className="ap-mini">
          {last.map((g) => <li key={g.id}><button type="button" onClick={() => onOpen(g.id)}><Img src={artOf(g).header} className="ap-mini-art" /><span>{g.title}</span><small>{ago(g.lastAgo)}</small></button></li>)}
        </ul>
      </section>
      <section>
        <h2>Recently added</h2>
        <ul className="ap-mini">
          {added.map((g, i) => <li key={g.id}><button type="button" onClick={() => onOpen(g.id)}><Img src={artOf(g).header} className="ap-mini-art" /><span>{g.title}</span><small>{['3 h ago', '2 days ago', '5 days ago', '1 week ago'][i]}</small></button></li>)}
        </ul>
      </section>
    </div>
  );
}

// ---------- game page ----------
// Play / Stop / Install / Update / Retry, with job progress.
export function MainAction({ L, A, g, className = 'play' }) {
  const run = L.running?.id === g.id ? L.running : null;
  const copy = activeCopy(g);
  const job = jobOf(L, g);
  const issue = issueOf(A, g);
  const t = A.t;
  if (run) return <button type="button" className={`${className} is-running`} onClick={L.actions.stop}>{run.phase === 'starting' ? <Loader size={15} className="spin" /> : <Square size={12} fill="currentColor" />}<span>{run.phase === 'starting' ? 'Starting' : t.stop}</span><span className="play-t">{elapsedShort(Date.now() - run.startedAt)}</span></button>;
  if (job) {
    const paused = A.paused.includes(job.id);
    return <button type="button" className={`${className} is-job`} style={{ '--p': job.progress }} onClick={() => A.actions.togglePause(job.id)} title={paused ? 'Resume' : 'Pause'}>{paused ? <Play size={14} /> : <Loader size={15} className="spin" />}<span>{paused ? 'Paused' : job.kind === 'verify' ? 'Verifying' : 'Installing'}</span><span className="play-t">{Math.round(job.progress * 100)}%</span></button>;
  }
  if (issue?.kind === 'install') return <button type="button" className={`${className} is-retry`} onClick={() => { A.actions.fixIssue(issue.id); L.actions.play(g.id); }}><RotateCcw size={15} /><span>{t.retry}</span></button>;
  if (copy.update) return <button type="button" className={className} onClick={() => L.actions.say(`Updating ${g.title}`)}><ArrowDownToLine size={15} /><span>{t.update}</span><span className="play-t">{copy.update}</span></button>;
  if (copy.installed) return <button type="button" className={className} onClick={() => L.actions.play(g.id)}><Play size={15} fill="currentColor" /><span>{t.play}</span></button>;
  return <button type="button" className={className} onClick={() => A.actions.open('install', { gameId: g.id })}><Download size={15} /><span>{t.install}</span><span className="play-t">{copy.size}</span></button>;
}
const elapsedShort = (ms) => { const s = Math.floor(ms / 1000); const h = Math.floor(s / 3600); const m = Math.floor((s % 3600) / 60); return h ? `${h}:${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}` : `${m}:${String(s % 60).padStart(2, '0')}`; };

// The reason the main action can't simply play.
export function ActionNote({ L, A, g }) {
  const issue = issueOf(A, g);
  const lent = LENT[g.id];
  if (issue?.kind === 'install') return <p className="ap-anote is-bad"><AlertTriangle size={13} />Last attempt failed: {issue.reason}</p>;
  if (lent?.inUse) return <p className="ap-anote is-warn"><Users size={13} />{lent.owner} is playing this game. You can play once they stop.</p>;
  return null;
}

// Which copy to run; compare copies.
export function CopySwitch({ L, A, g }) {
  const copy = activeCopy(g);
  const lent = LENT[g.id];
  return (
    <div className="ap-copy">
      {g.stores.length > 1 ? (
        <div className="src" role="radiogroup" aria-label="Launch from">
          {g.stores.map((s) => (
            <button key={s.store} type="button" role="radio" aria-checked={g.source === s.store} className={g.source === s.store ? 'is-on' : ''} onClick={() => L.actions.setSource(g.id, s.store)} title={s.installed ? `Installed, ${s.version}` : `Not installed, ${s.size}`}>
              <StoreGlyph store={s.store} size={13} />{storeName(s.store)}{lent?.store === s.store && <span className="ap-borrow">borrowed</span>}
            </button>
          ))}
        </div>
      ) : <span className="src-one"><StoreGlyph store={copy.store} size={13} />{storeName(copy.store)}{lent && <span className="ap-borrow">borrowed</span>}</span>}
      {g.stores.length > 1 && <button type="button" className="icon-btn ap-compare" aria-label="Compare copies" title="Compare copies" onClick={() => A.actions.open('compare', { gameId: g.id })}><GitCompare size={15} /></button>}
    </div>
  );
}

export const CLIENT_LAUNCH = { gtav: 'Rockstar Games Launcher', fallout3: 'Amazon Games' };
export function LaunchNotes({ L, A, g }) {
  const notes = [];
  if (CLIENT_LAUNCH[g.id]) notes.push({ k: 'client', icon: ExternalLink, text: `Launches through the ${CLIENT_LAUNCH[g.id]}. Platypus starts it for you and waits.` });
  if (g.id === 'teardown') notes.push({ k: 'outside', icon: AlertTriangle, tone: 'warn', text: 'Installed outside your game folders.', act: { label: 'Move to Games', run: () => A.actions.go('attention') } });
  if (g.id === 'civ7' || g.id === 'bg3') notes.push({ k: 'ver', icon: GitCompare, text: g.id === 'civ7' ? 'Version 1.2.3, update 1.2.4 waiting.' : 'Patch 8 hotfix 4, public branch.', act: { label: 'Choose a version', run: () => A.actions.open('game-settings', { cat: 'versions' }) } });
  if (!notes.length) return null;
  return (
    <ul className="ap-lnotes">
      {notes.map((n) => { const I = n.icon; return <li key={n.k} className={n.tone ? `is-${n.tone}` : ''}><I size={13} /><span>{n.text}</span>{n.act && <button type="button" onClick={n.act.run}>{n.act.label}</button>}</li>; })}
    </ul>
  );
}

// "You" block: your relationship with the game.
export function YouPanel({ L, A, g }) {
  const inCols = A.collections.filter((c) => collectionIds(c, L.games, A.favs).includes(g.id));
  const [note, setNote] = useState(A.notes[g.id] || '');
  useEffect(() => setNote(A.notes[g.id] || ''), [g.id]);
  return (
    <div className="ap-you">
      <dl className="you">
        <div><dt>Played</dt><dd>{g.hours ? `${Math.round(g.hours)} h` : '—'}</dd></div>
        <div><dt>Last played</dt><dd>{g.hours ? ago(g.lastAgo) : 'Never'}</dd></div>
        <div><dt>Runs with</dt><dd>{runnerOf(g.runner)?.name}</dd></div>
        <div><dt>ProtonDB</dt><dd className={`tier-${g.tier}`}>{TIER_LABEL[g.tier]}</dd></div>
      </dl>
      <div className="ap-you-row">
        <span>Status</span>
        <Select label="Your status" value={A.status[g.id] || 'none'} options={OWN_STATUS.map((s) => ({ value: s.id, label: s.label }))} onChange={(v) => A.actions.setStatus(g.id, v)} />
      </div>
      <label className="ap-you-note">
        <span>Private notes</span>
        <textarea value={note} rows={3} placeholder="Only you see this" onChange={(e) => setNote(e.target.value)} onBlur={() => A.actions.setNote(g.id, note)} />
      </label>
      <div className="ap-you-cols">
        <span>Collections</span>
        <div className="ap-chips">
          {A.favs.includes(g.id) && <span className="ap-chip is-static"><Star size={11} />Favourites</span>}
          {inCols.map((c) => <button key={c.id} type="button" className="ap-chip" onClick={() => A.actions.setCollection(c.id)}>{c.name}</button>)}
          <QuickAddCollection L={L} A={A} g={g} />
        </div>
      </div>
    </div>
  );
}
function QuickAddCollection({ L, A, g }) {
  const [open, setOpen] = useState(false);
  return (
    <span className="ap-anchor">
      <button type="button" className="ap-chip is-add" aria-expanded={open} onClick={() => setOpen(!open)}><Plus size={11} />Add</button>
      <Popover open={open} onClose={() => setOpen(false)} label="Add to collection" className="ap-menu">
        <Check2 checked={A.favs.includes(g.id)} onChange={() => A.actions.toggleFav(g.id)}>Favourites</Check2>
        {A.collections.filter((c) => c.kind === 'manual').map((c) => <Check2 key={c.id} checked={c.ids.includes(g.id)} onChange={() => A.actions.toggleInCollection(c.id, g.id)}>{c.name}</Check2>)}
        <button type="button" onClick={() => { A.actions.open('collection', { add: g.id }); setOpen(false); }}><Plus size={14} />New collection</button>
      </Popover>
    </span>
  );
}

// Activity tab: sessions with dates and lengths.
export function SessionsPanel({ L, g }) {
  if (!L.sessions.length) return <Empty icon={Clock} title="No sessions yet">Play {g.title} once to start a history.</Empty>;
  const total = L.sessions.reduce((a, s) => a + s.dur, 0);
  const max = Math.max(...L.sessions.map((s) => s.dur));
  return (
    <div className="ap-sessions">
      <p className="ap-lead">{L.sessions.length} sessions in the last two weeks, {dur(total)} in all.</p>
      <ul>
        {L.sessions.map((s, i) => (
          <li key={i} className={s.crash ? 'is-bad' : ''}>
            <span className="ap-s-day">{dayLabel(s.start)}</span>
            <span className="ap-s-time">{clock(s.start)}</span>
            <span className="ap-s-bar"><span style={{ width: `${(s.dur / max) * 100}%` }} /></span>
            <span className="ap-s-dur">{dur(s.dur)}</span>
            <span className="ap-s-exit">{s.crash ? 'Crashed' : 'Quit normally'}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// Saves tab: cloud panel, local backups, save location.
export function SavesPanel({ L, A, g }) {
  const copy = activeCopy(g);
  const cloud = can(copy.store, 'cloud');
  const [backups, setBackups] = useState(() => (g.saves.length ? [{ id: 1, at: Date.now() - 2 * 864e5, size: g.saves[0].local?.size || '1 MB', auto: true }, { id: 2, at: Date.now() - 9 * 864e5, size: g.saves[0].local?.size || '1 MB', auto: true }] : []));
  useEffect(() => setBackups(g.saves.length ? [{ id: 1, at: Date.now() - 2 * 864e5, size: g.saves[0].local?.size || '1 MB', auto: true }, { id: 2, at: Date.now() - 9 * 864e5, size: g.saves[0].local?.size || '1 MB', auto: true }] : []), [g.id]);
  const [mode, setMode] = useState(g.sync === 'off' ? 'off' : 'auto');
  return (
    <div className="ap-saves">
      <section className="ap-panel">
        <header><h3>Cloud saves</h3>{cloud ? <span className="ap-muted">{storeName(copy.store)} cloud</span> : <span className="ap-muted">{storeName(copy.store)} has no cloud saves</span>}</header>
        {cloud ? (
          <>
            {g.sync === 'conflict' && <div className="ap-alert is-bad"><AlertTriangle size={14} /><span>Your save changed here and on another device.</span><button type="button" className="btn btn-primary" onClick={() => L.actions.openConflict(g.id)}>Compare and choose</button></div>}
            <div className="ap-you-row"><span>Sync</span><Seg label="Sync mode" value={mode} onChange={setMode} options={[{ value: 'auto', label: 'Automatic' }, { value: 'ask', label: 'Ask first' }, { value: 'off', label: 'Off' }]} /></div>
            <SavesTable L={L} />
          </>
        ) : <p className="ap-muted">Local backups below still protect your saves.</p>}
      </section>
      <section className="ap-panel">
        <header><h3>Backups on this PC</h3><button type="button" className="btn" onClick={() => { setBackups((b) => [{ id: Date.now(), at: Date.now(), size: g.saves[0]?.local?.size || '0 KB' }, ...b]); L.actions.say('Backup created'); }}><Plus size={14} />Back up now</button></header>
        {backups.length ? (
          <ul className="ap-list">
            {backups.map((b) => <li key={b.id}><span>{dayLabel(b.at)} {clock(b.at)}</span><span className="ap-muted">{b.auto ? 'Automatic' : 'By you'}, {b.size}</span><button type="button" className="btn btn-quiet" onClick={() => A.actions.open('confirm', { title: 'Restore this backup?', body: 'Your current save is backed up first, so you can go back.', yes: 'Restore', tone: 'primary', run: () => L.actions.say('Backup restored') })}>Restore</button></li>)}
          </ul>
        ) : <p className="ap-muted">No backups yet. Platypus backs up before each sync once you’ve played.</p>}
      </section>
      <section className="ap-panel">
        <header><h3>Save location</h3><button type="button" className="btn btn-quiet" onClick={() => L.actions.say('Opened the save folder', 'info')}><FolderOpen size={14} />Open folder</button></header>
        <code className="ap-path">{g.runner === 'native' ? `~/.local/share/${g.id}/` : `~/.local/share/platypus/prefixes/${g.id}/drive_c/users/steamuser/Documents/My Games/`}</code>
      </section>
    </div>
  );
}

// Compatibility tab: facts, crash hints, notes and suggested tweaks.
export function CompatPanel({ L, A, g }) {
  const r = runnerOf(g.runner);
  const native = g.runner === 'native';
  const crashes = L.sessions.filter((s) => s.crash).length;
  const tweaks = (RECOMMEND[g.id] || []).filter((x) => !g.recipes.includes(x.id));
  const facts = [
    ['Runner', r?.name],
    ['DirectX layer', native ? 'Not used' : g.graphics === 'Bundled with runner' ? 'DXVK, VKD3D-Proton (bundled)' : g.graphics],
    ['Wrappers', [g.gamemode && 'GameMode', g.mangohud && 'MangoHud', g.gamescope && 'Gamescope'].filter(Boolean).join(', ') || 'None'],
    ['Launches via', CLIENT_LAUNCH[g.id] || (native ? 'Native binary' : 'umu-run')],
    ['ProtonDB', TIER_LABEL[g.tier]],
    ['Steam Deck', g.tier === 'native' || g.tier === 'platinum' ? 'Verified' : g.tier === 'gold' ? 'Playable' : 'Unsupported'],
  ];
  return (
    <div className="ap-compat">
      <dl className="ap-facts">{facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
      {crashes > 0 && <div className="ap-alert is-warn"><AlertTriangle size={14} /><span>{crashes} of the last {L.sessions.length} sessions crashed (SIGSEGV). Players fixed this with the Direct3D shader compiler.</span><button type="button" className="btn" onClick={() => A.actions.open('game-settings', { cat: 'fixes' })}>See fixes</button></div>}
      {tweaks.length > 0 && (
        <section className="ap-panel">
          <header><h3>Suggested tweaks</h3></header>
          <ul className="ap-list">
            {tweaks.map((x) => {
              const rec = RECIPES.find((y) => y.id === x.id);
              return <li key={x.id}><span><b>{rec?.name}</b><br /><span className="ap-muted">{x.why}</span></span><span className="ap-muted">{x.src}{x.votes ? `, ${x.votes} votes` : ''}</span><button type="button" className="btn" onClick={() => { L.actions.toggleRecipe(g.id, x.id); L.actions.say(`${rec?.name} added, installs on next launch`); }}>Apply</button></li>;
            })}
          </ul>
        </section>
      )}
      <section className="ap-panel">
        <header><h3>Community notes</h3><span className="ap-muted">{g.notes.length}</span></header>
        {g.notes.length ? <ul className="ap-list is-notes">{g.notes.map((n, i) => <li key={i}><span className="ap-src">{n.src}</span><span>{n.text}</span><span className="ap-muted">{ago(n.when)}</span></li>)}</ul> : <p className="ap-muted">No notes yet.</p>}
      </section>
    </div>
  );
}

// ---------- per-game settings: the kit's categories plus the rest ----------
export const GAME_CATS = [
  ...SETTINGS_CATS.map((c) => ({ ...c, group: c.level === 'basic' ? 'Everyday' : 'Advanced' })),
  { id: 'prefix', icon: Wine, label: 'Wine prefix', group: 'Advanced', blurb: 'Where Windows files live' },
  { id: 'versions', icon: GitBranch, label: 'Versions', group: 'This copy', blurb: 'Build, branch, roll back' },
  { id: 'addons', icon: Puzzle, label: 'Add-ons', group: 'This copy', blurb: 'DLC and optional parts' },
  { id: 'plugin', icon: Plug, label: 'Store options', group: 'This copy', blurb: 'Settings the store plugin adds' },
  { id: 'custom', icon: PaletteIcon, label: 'Customization', group: 'Library', blurb: 'Title, artwork, year' },
  { id: 'maint', icon: Hammer, label: 'Maintenance', group: 'Library', blurb: 'Verify, move, uninstall' },
  { id: 'legal', icon: Scale, label: 'Terms', group: 'Library', blurb: 'The store’s terms and EULA' },
];
export function gameCatSummary(L, A, id) {
  const g = L.game;
  const copy = activeCopy(g);
  switch (id) {
    case 'prefix': return g.runner === 'native' ? 'Not used' : 'Default preset';
    case 'versions': return copy.version || '—';
    case 'addons': return DLC[g.id] ? `${DLC[g.id].filter((d) => d.on).length} of ${DLC[g.id].length}` : 'None';
    case 'plugin': return storeName(copy.store);
    case 'custom': return g.heroOverride ? 'Custom art' : 'From the store';
    case 'maint': return copy.installed ? copy.size : 'Not installed';
    case 'legal': return copy.store === 'local' ? 'None' : 'Accepted';
    default: return catSummary(L, id);
  }
}
export const DLC = {
  bg3: [{ id: 'dd', name: 'Digital Deluxe Edition', size: '1.1 GB', on: true }],
  cyberpunk: [{ id: 'pl', name: 'Phantom Liberty', size: '28.0 GB', on: true }, { id: 'redmod', name: 'REDmod', size: '2.1 GB', on: false }],
  witcher3: [{ id: 'hos', name: 'Hearts of Stone', size: '3.2 GB', on: true }, { id: 'baw', name: 'Blood and Wine', size: '9.6 GB', on: true }, { id: 'hd', name: 'HD textures (optional)', size: '11.0 GB', on: false }],
  fallout3: [{ id: 'bs', name: 'Broken Steel', size: '0.9 GB', on: true }, { id: 'pl', name: 'Point Lookout', size: '0.7 GB', on: true }, { id: 'zeta', name: 'Mothership Zeta', size: '0.6 GB', on: true }],
  civ7: [{ id: 'cross', name: 'Crossroads of the World', size: '3.4 GB', on: false }],
};
const BRANCHES = { bg3: ['Public', 'Hotfix preview', 'Patch 7 (rollback)'], civ7: ['Public', 'Public test'], helldivers: ['Public'], witcher3: ['Next-gen 4.04', 'Classic 1.32'] };
const STORE_OPTS = {
  steam: [{ k: 'overlay', label: 'Steam overlay', desc: 'Shift Tab in game. Needs the Steam client running.' }, { k: 'input', label: 'Steam Input', desc: 'Let Steam remap your controller for this game.' }],
  gog: [{ k: 'galaxy', label: 'GOG Galaxy features', desc: 'Achievements and friends while you play.' }],
  epic: [{ k: 'eos', label: 'Epic online services', desc: 'Needed for cross-play.' }],
  amazon: [{ k: 'prime', label: 'Check Prime status at launch', desc: 'The game refuses to start without it.' }],
};

export function GameSettingsCategory({ L, A, id }) {
  const g = L.game;
  const copy = activeCopy(g);
  const [opts, setOpts] = useState({ overlay: true, input: false, galaxy: true, eos: true, prime: true, nvapi: !!g.env.find(([k]) => k === 'PROTON_ENABLE_NVAPI'), fps: 'Off', scale: 'Fit', preset: 'Default', shortcut: true });
  const set = (k, v) => setOpts((o) => ({ ...o, [k]: v }));
  switch (id) {
    case 'performance':
      return (<><SettingsCategory L={L} id={id} /><SRow label="Frame rate limit" desc="Caps the frame rate to save power and fan noise."><Select label="Frame rate limit" value={opts.fps} options={['Off', '30', '60', '120', '165 (display)']} onChange={(v) => set('fps', v)} /></SRow></>);
    case 'display':
      return (<><SettingsCategory L={L} id={id} /><SRow label="Scaling" desc="How a lower resolution fills the screen."><Select label="Scaling" value={opts.scale} options={['Fit', 'Stretch', 'Integer', 'Off']} onChange={(v) => set('scale', v)} /></SRow></>);
    case 'layers':
      return (<><SettingsCategory L={L} id={id} /><Switch checked={opts.nvapi} onChange={(v) => set('nvapi', v)} label="NVIDIA features (NVAPI)" desc="DLSS and Reflex on NVIDIA cards." /></>);
    case 'prefix':
      if (g.runner === 'native') return <SRow label="Not used" desc="Native games don’t need a Wine prefix." />;
      return (
        <>
          <SRow label="Location" desc={`~/.local/share/platypus/prefixes/${g.id}`}><button type="button" className="btn btn-quiet" onClick={() => L.actions.say('Opened the prefix', 'info')}><FolderOpen size={14} />Open</button></SRow>
          <SRow label="Preset" desc="Default is fresh per game. Shared reuses one prefix for several games."><Select label="Prefix preset" value={opts.preset} options={['Default', 'Clean (no extras)', 'Shared: Bethesda games']} onChange={(v) => set('preset', v)} /></SRow>
          <SRow label="Winetricks" desc="Opens Winetricks on this prefix for anything Fixes doesn’t list."><button type="button" className="btn" onClick={() => L.actions.say('Winetricks opened', 'info')}>Open Winetricks</button></SRow>
          <SRow label="Executable" desc={`${g.title.replace(/[^A-Za-z0-9]/g, '')}.exe, picked by the store`}><button type="button" className="btn btn-quiet">Change</button></SRow>
        </>
      );
    case 'versions': {
      const br = BRANCHES[g.id] || ['Public'];
      return (
        <>
          <SRow label="Installed version" desc={`${copy.version || 'Unknown'} from ${storeName(copy.store)}`} />
          <SRow label="Branch" desc="Beta and older branches the store offers."><Select label="Branch" value={br[0]} options={br} onChange={(v) => L.actions.say(`Switching to ${v}`)} /></SRow>
          <SRow label="Roll back" desc="Go back to the build you had before the last update."><button type="button" className="btn" disabled={!copy.installed} onClick={() => A.actions.open('confirm', { title: 'Roll back to the previous build?', body: 'Your saves stay. Platypus downloads only the changed files.', yes: 'Roll back', tone: 'primary', run: () => L.actions.say('Rolling back') })}>Roll back</button></SRow>
          <Switch checked={!copy.update} onChange={() => L.actions.say('Saved')} label="Update automatically" desc="Off keeps the current build until you choose to update." />
        </>
      );
    }
    case 'addons': {
      const list = DLC[g.id];
      if (!list) return <SRow label="No add-ons" desc={`${storeName(copy.store)} lists no DLC for ${g.title}.`} />;
      return <div className="ap-addons">{list.map((d) => <Check2 key={d.id} checked={d.on} onChange={() => L.actions.say(d.on ? `${d.name} will be removed` : `${d.name} will be installed`)} hint={d.size}>{d.name}</Check2>)}</div>;
    }
    case 'plugin': {
      const list = STORE_OPTS[copy.store];
      if (!list) return <SRow label="Nothing to set" desc={`The ${storeName(copy.store)} plugin adds no options for this game.`} />;
      return <>{list.map((o) => <Switch key={o.k} checked={opts[o.k]} onChange={(v) => set(o.k, v)} label={o.label} desc={o.desc} />)}<p className="ap-muted ap-small">Declared by the {storeName(copy.store)} plugin.</p></>;
    }
    case 'custom':
      return (
        <>
          <SRow label="Title" wide><input className="ap-input" defaultValue={g.title} aria-label="Custom title" /></SRow>
          <div className="ap-artpick">
            {[['Cover', 'cover'], ['Banner', 'hero'], ['Logo', 'logo'], ['Icon', 'header']].map(([label, k]) => (
              <button key={k} type="button" className={`ap-art is-${k}`} onClick={() => L.actions.say(`Pick a ${label.toLowerCase()} image`, 'info')}><Img src={artOf(g)[k]} /><span>{label}</span></button>
            ))}
          </div>
          <SRow label="Year"><input className="ap-input is-short" defaultValue={g.year} aria-label="Year" /></SRow>
          <SRow label="Steam App ID for artwork" desc="Platypus looks up art and store pages with it."><input className="ap-input is-short" defaultValue={g.appid || ''} placeholder="e.g. 1167630" aria-label="Steam App ID" /></SRow>
        </>
      );
    case 'maint':
      return (
        <>
          <SRow label="Verify files" desc="Checks every file and downloads the broken ones."><button type="button" className="btn" disabled={!copy.installed} onClick={() => L.actions.verify(g.id)}>Verify</button></SRow>
          <SRow label="Move the install" desc={`${copy.size} on ${g.id === 'teardown' ? '~/Downloads' : '/mnt/games'}`}><button type="button" className="btn" disabled={!copy.installed} onClick={() => A.actions.open('install', { gameId: g.id, move: true })}>Move</button></SRow>
          <SRow label="Open the game folder"><button type="button" className="btn btn-quiet" onClick={() => L.actions.say('Opened the game folder', 'info')}><FolderOpen size={14} />Open</button></SRow>
          {g.runner !== 'native' && <SRow label="Clean the prefix" desc="Removes components that no fix asks for. Saves stay."><button type="button" className="btn">Clean</button></SRow>}
          <SRow label="Repair" desc="Reinstalls the runtimes and redoes the first-launch setup."><button type="button" className="btn">Repair</button></SRow>
          <SRow label="Uninstall" desc="Removes the game files. Saves stay in the cloud and in backups."><button type="button" className="btn is-bad" disabled={!copy.installed} onClick={() => A.actions.open('confirm', { title: `Uninstall ${g.title}?`, body: `Frees ${copy.size}. Your saves stay in backups${can(copy.store, 'cloud') ? ' and in the cloud' : ''}.`, yes: 'Uninstall', run: () => L.actions.say(`Uninstalling ${g.title}`) })}>Uninstall</button></SRow>
        </>
      );
    case 'legal':
      if (copy.store === 'local') return <SRow label="No terms" desc="Games you add by hand come with whatever terms their files include." />;
      return (
        <>
          <SRow label={`${storeName(copy.store)} subscriber agreement`} desc="Accepted when you connected the store."><button type="button" className="btn btn-quiet" onClick={() => A.actions.open('terms', { store: copy.store })}>Read</button></SRow>
          <SRow label={`${g.title} EULA`} desc={`From ${g.dev}. Accepted on first install.`}><button type="button" className="btn btn-quiet" onClick={() => A.actions.open('terms', { store: copy.store, game: g.title })}>Read</button></SRow>
        </>
      );
    default:
      return <SettingsCategory L={L} id={id} />;
  }
}

// Store page tab counts and the tab list for a game.
export function gameTabs(L, A, g) {
  const ach = can(activeCopy(g).store, 'achievements') || g.stores.some((s) => can(s.store, 'achievements')) ? achievementsOf(g) : null;
  return [
    { id: 'store', label: 'Store page' },
    ...(ach ? [{ id: 'achievements', label: 'Achievements', count: `${ach.unlocked}/${ach.total}` }] : []),
    { id: 'sessions', label: 'Activity', count: L.sessions.length },
    { id: 'saves', label: 'Saves', count: g.saves.length },
    { id: 'compat', label: 'Compatibility', count: g.notes.length },
  ];
}

export { Tabs, CAPS, ISSUES, FRIENDS };
