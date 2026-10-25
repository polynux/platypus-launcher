// Pages, social, dialogs and chrome shared by the full-app designs.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Play, Pause, X, Download, RotateCcw, ArrowUp, Check, AlertTriangle, CircleAlert, Wrench, HardDrive, Trash2, RefreshCw, Plus, LogIn, LogOut, ShieldCheck, Puzzle, FolderOpen, Search, ChevronLeft, ChevronRight, Ticket, Send, MessageSquare, Bell, BellOff, Users, Eye, WifiOff, ServerCrash, PackageOpen, Gamepad2, Sparkles, Cloud, Clock, Trophy, ArrowDownToLine, Globe, ChevronDown, Square, Star } from 'lucide-react';
import { STORES, GAMES } from './data.js';
import { runnerOf, activeCopy, isInstalled, artOf, ago, dayLabel, clock } from './launcher.js';
import { StoreGlyph, Img, Select, SRow, TIER_LABEL, ConflictCompare } from './kit.jsx';
import { achievementsOf } from './page.jsx';
import { PLUGIN_STATUS, DRIVES, driveOf, ACTIVITY, ACTIVITY_KINDS, DL_HISTORY, FRIEND_STATUS, FRIENDS, FAMILY, LENT, avatarHue, APP_SETTINGS_CATS, DEMO_STATES, GHOSTS, storesWith, can, gb, collectionIds, BUILTIN_COLLECTIONS, OWN_STATUS } from './app.js';
import { Seg, Switch, Check2, Empty, Skeleton, Popover, storeName, MainAction, GAME_CATS, gameCatSummary, GameSettingsCategory, TIERS, issueOf } from './appLibrary.jsx';

const gameById = (L, id) => L.games.find((g) => g.id === id);
const greeting = () => { const h = new Date().getHours(); return h < 5 ? 'Good night' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'; };

export function PageHead({ title, sub, children }) {
  return (
    <header className="ap-head">
      <div><h1>{title}</h1>{sub && <p>{sub}</p>}</div>
      {children && <div className="ap-head-act">{children}</div>}
    </header>
  );
}

// ---------- Home ----------
export function HomePage({ L, A, onOpen }) {
  const cont = [...L.games].filter((g) => isInstalled(g) && g.hours).sort((a, b) => a.lastAgo - b.lastAgo).slice(0, 3);
  const added = ['deltarune', 'fallout3', 'civ7', 'teardown', 'celeste'].map((id) => gameById(L, id)).filter(Boolean);
  const toInstall = L.games.filter((g) => !isInstalled(g) && A.status[g.id] === 'backlog');
  const toFinish = L.games.filter((g) => A.status[g.id] === 'playing');
  const hours = Math.round(L.games.reduce((a, g) => a + g.hours, 0));
  const inst = L.games.filter(isInstalled);
  const size = inst.reduce((a, g) => a + (parseFloat(activeCopy(g).size) || 0), 0);
  const online = FRIENDS.filter((f) => f.status !== 'offline' && A.social.stores[f.store] !== false);
  const sug = gameById(L, 'outerwilds');
  if (A.demo === 'loading') return <div className="ap-page"><Skeleton rows={3} kind="cards" /><Skeleton rows={6} /></div>;
  return (
    <div className="ap-page ap-home">
      <PageHead title={`${greeting()}, polynux`} sub={L.running ? `${gameById(L, L.running.id)?.title} is running.` : `${inst.length} games ready to play.`} />
      {!A.dismissed.includes('sug') && sug && (
        <section className="ap-sug" style={{ '--bg': `url(${artOf(sug).hero})` }}>
          <div>
            <small>From your backlog</small>
            <h2>{sug.title}</h2>
            <p>On your backlog for a year. Camille lends it to you through the family, and it’s rated Platinum.</p>
            <div className="ap-row"><MainAction L={L} A={A} g={sug} className="btn btn-primary ap-sug-btn" /><button type="button" className="btn btn-quiet" onClick={() => onOpen(sug.id)}>Open game</button></div>
          </div>
          <button type="button" className="icon-btn ap-sug-x" aria-label="Don’t suggest this" onClick={() => A.actions.dismiss('sug')}><X size={15} /></button>
        </section>
      )}
      <section>
        <h2 className="ap-h2">Continue playing</h2>
        <ul className="ap-cont">
          {cont.map((g) => {
            const ach = achievementsOf(g);
            return (
              <li key={g.id}>
                <button type="button" className="ap-cont-hit" onClick={() => onOpen(g.id)} aria-label={`Open ${g.title}`}><Img src={artOf(g).hero} className="ap-cont-art" /></button>
                <div className="ap-cont-body">
                  <b>{g.title}</b>
                  <span>{ago(g.lastAgo)} · {Math.round(g.hours)} h{ach ? ` · ${ach.unlocked}/${ach.total} achievements` : ''}</span>
                </div>
                <MainAction L={L} A={A} g={g} className="ap-cont-play" />
              </li>
            );
          })}
        </ul>
      </section>
      <div className="ap-home-cols">
        <section>
          <h2 className="ap-h2">Recently added</h2>
          <ul className="ap-shelf">
            {added.map((g) => <li key={g.id}><button type="button" onClick={() => onOpen(g.id)}><Img src={artOf(g).cover} className="ap-shelf-art" /><span>{g.title}</span></button></li>)}
          </ul>
        </section>
        <section className="ap-wait">
          <h2 className="ap-h2">Waiting for you</h2>
          <div className="ap-wait-g">
            <div><h3>To finish</h3><ul>{toFinish.map((g) => <li key={g.id}><button type="button" onClick={() => onOpen(g.id)}>{g.title}<small>{Math.round(g.hours)} h</small></button></li>)}</ul></div>
            <div><h3>To install</h3><ul>{toInstall.map((g) => <li key={g.id}><button type="button" onClick={() => onOpen(g.id)}>{g.title}<small>{activeCopy(g).size}</small></button></li>)}</ul></div>
          </div>
        </section>
      </div>
      <div className="ap-home-cols">
        <section className="ap-panel">
          <header><h3>Your library</h3><button type="button" className="btn btn-quiet" onClick={() => A.actions.go('library')}>Open library</button></header>
          <dl className="ap-stats">
            <div><dt>Games</dt><dd>{L.games.length}</dd></div>
            <div><dt>Installed</dt><dd>{inst.length}</dd></div>
            <div><dt>Played</dt><dd>{hours.toLocaleString('en')} h</dd></div>
            <div><dt>On disk</dt><dd>{gb(size)}</dd></div>
          </dl>
          <div className="ap-storebar">{Object.keys(STORES).map((s) => { const n = L.games.filter((g) => g.stores.some((c) => c.store === s)).length; return n ? <span key={s} style={{ flex: n }} title={`${storeName(s)}: ${n}`}><StoreGlyph store={s} size={11} />{n}</span> : null; })}</div>
        </section>
        <section className="ap-panel">
          <header><h3>Needs attention</h3>{A.issues.length > 0 && <button type="button" className="btn btn-quiet" onClick={() => A.actions.go('attention')}>See all {A.issues.length}</button>}</header>
          {A.issues.length ? <ul className="ap-list">{A.issues.slice(0, 3).map((i) => <li key={i.id}><span><b>{gameById(L, i.gameId)?.title}</b><br /><span className="ap-muted">{i.title}</span></span><button type="button" className="btn" onClick={() => A.actions.fixIssue(i.id)}>{i.fix}</button></li>)}</ul> : <p className="ap-muted">Everything works.</p>}
        </section>
        <section className="ap-panel">
          <header><h3>Friends online</h3><button type="button" className="btn btn-quiet" onClick={() => A.actions.openSocial(true)}>{online.length} online</button></header>
          <ul className="ap-flist is-mini">{online.slice(0, 4).map((f) => <FriendRow key={f.id} L={L} A={A} f={f} />)}</ul>
        </section>
      </div>
    </div>
  );
}

// ---------- Downloads ----------
const XFER = ['download', 'verify', 'move', 'uninstall'];
export function DownloadsPage({ L, A, onOpen }) {
  const queue = A.jobs.filter((j) => XFER.includes(j.kind));
  const bg = A.jobs.filter((j) => !XFER.includes(j.kind));
  const [hist, setHist] = useState(DL_HISTORY);
  return (
    <div className="ap-page">
      <PageHead title={A.t.downloads} sub={queue.length ? `${queue.length} in the queue. 41.8 MB/s.` : 'Nothing downloading.'}>
        <button type="button" className="btn" onClick={() => queue.forEach((j) => A.actions.togglePause(j.id))}><Pause size={14} />Pause all</button>
      </PageHead>
      <section>
        <h2 className="ap-h2">Queue</h2>
        {queue.length ? (
          <ul className="ap-queue">
            {queue.map((j, i) => {
              const g = gameById(L, j.gameId);
              const paused = A.paused.includes(j.id);
              return (
                <li key={j.id} className={paused ? 'is-paused' : ''}>
                  {g && <Img src={artOf(g).header} className="ap-q-art" />}
                  <div className="ap-q-body">
                    <div className="ap-q-top"><b>{g?.title || j.label}</b><span className="ap-muted">{j.kind === 'verify' ? 'Verifying' : j.label.split('·')[1]?.trim()}</span></div>
                    <div className="ap-bar"><span style={{ width: `${j.progress * 100}%` }} /></div>
                    <div className="ap-q-meta"><span>{Math.round(j.progress * 100)}%</span><span>{paused ? 'Paused' : j.unit}</span>{i === 0 && !paused && <span>about 6 min left</span>}</div>
                  </div>
                  <div className="ap-row">
                    {i > 0 && <button type="button" className="icon-btn" aria-label="Move to the top" title="Move to the top"><ArrowUp size={15} /></button>}
                    <button type="button" className="icon-btn" aria-label={paused ? 'Resume' : 'Pause'} onClick={() => A.actions.togglePause(j.id)}>{paused ? <Play size={15} /> : <Pause size={15} />}</button>
                    <button type="button" className="icon-btn" aria-label="Cancel" onClick={() => A.actions.cancelJob(j.id)}><X size={15} /></button>
                  </div>
                </li>
              );
            })}
          </ul>
        ) : <Empty icon={Download} title="Nothing downloading">Installs, updates, moves and file checks show up here.</Empty>}
      </section>
      {bg.length > 0 && (
        <section>
          <h2 className="ap-h2">Background work</h2>
          <ul className="ap-list">{bg.map((j) => <li key={j.id}><span>{j.label}</span><span className="ap-muted">{j.unit}</span><span className="ap-mini-bar"><span style={{ width: `${j.progress * 100}%` }} /></span></li>)}</ul>
        </section>
      )}
      <section>
        <h2 className="ap-h2">Recent</h2>
        <table className="tbl ap-hist">
          <tbody>
            {hist.map((h) => {
              const g = gameById(L, h.gameId);
              return (
                <tr key={h.id} className={h.result === 'failed' ? 'is-bad' : ''}>
                  <td><button type="button" className="ap-tt" onClick={() => onOpen(h.gameId)}><Img src={artOf(g).header} className="ap-tt-art" /><span>{g.title}</span></button></td>
                  <td className={h.result === 'failed' ? 't-bad' : ''}>{h.result === 'failed' ? `Failed: ${h.reason}` : h.result[0].toUpperCase() + h.result.slice(1)}</td>
                  <td className="t-dim">{h.size}</td>
                  <td className="t-dim">{ago(Date.now() - h.at)}</td>
                  <td className="num">{h.result === 'failed' ? <button type="button" className="btn" onClick={() => { setHist((x) => x.filter((y) => y.id !== h.id)); L.actions.play(h.gameId); }}><RotateCcw size={13} />Retry</button> : <span className="t-dim">{h.took}</span>}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </section>
    </div>
  );
}

// ---------- Needs attention ----------
const ISSUE_ICON = { install: Download, runner: Gamepad2, outside: FolderOpen, prefix: Wrench, update: ArrowDownToLine, conflict: AlertTriangle };
export function AttentionPage({ L, A, onOpen }) {
  return (
    <div className="ap-page">
      <PageHead title={A.t.attention} sub={A.issues.length ? `${A.issues.length} games can’t start or need a decision.` : 'Nothing needs you.'} />
      {A.issues.length === 0 ? <Empty icon={Check} title="Everything works">Games that fail to install, lose their runner or need an update show up here.</Empty> : (
        <ul className="ap-issues">
          {A.issues.map((i) => {
            const g = gameById(L, i.gameId);
            const I = ISSUE_ICON[i.kind] || CircleAlert;
            return (
              <li key={i.id} className={`is-${i.kind}`}>
                <Img src={artOf(g).header} className="ap-is-art" />
                <div className="ap-is-body">
                  <span className="ap-is-kind"><I size={13} />{i.title}</span>
                  <button type="button" className="ap-is-title" onClick={() => onOpen(g.id)}>{g.title}</button>
                  <p>{i.reason}</p>
                </div>
                <div className="ap-is-act">
                  <button type="button" className="btn btn-primary" onClick={() => A.actions.fixIssue(i.id)}>{i.fix}</button>
                  {i.alt && <button type="button" className="btn btn-quiet" onClick={() => A.actions.fixIssue(i.id)}>{i.alt}</button>}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

// ---------- Activity ----------
const ACT_ICON = { session: Clock, install: Download, update: ArrowDownToLine, sync: Cloud, achievement: Trophy };
export function ActivityPage({ L, A, onOpen }) {
  const [kinds, setKinds] = useState([]);
  const [game, setGame] = useState('all');
  const items = ACTIVITY.filter((a) => (!kinds.length || kinds.includes(a.kind)) && (game === 'all' || a.gameId === game));
  const days = [];
  for (const a of items) { const d = dayLabel(a.at); const last = days[days.length - 1]; if (last?.d === d) last.items.push(a); else days.push({ d, items: [a] }); }
  const games = [...new Set(ACTIVITY.map((a) => a.gameId))].map((id) => gameById(L, id));
  return (
    <div className="ap-page">
      <PageHead title={A.t.activity} sub="What happened in your library, newest first." />
      <div className="ap-toolbar">
        <div className="ap-chips">{ACTIVITY_KINDS.map((k) => <button key={k.id} type="button" className={`ap-chip ${kinds.includes(k.id) ? 'is-on' : ''}`} aria-pressed={kinds.includes(k.id)} onClick={() => setKinds((x) => (x.includes(k.id) ? x.filter((y) => y !== k.id) : [...x, k.id]))}>{k.label}</button>)}</div>
        <Select label="Game" className="ap-w200" value={game} options={[{ value: 'all', label: 'Every game' }, ...games.map((g) => ({ value: g.id, label: g.title }))]} onChange={setGame} />
      </div>
      {days.length === 0 ? <Empty icon={Clock} title="Nothing matches" action={<button type="button" className="btn" onClick={() => { setKinds([]); setGame('all'); }}>Clear filters</button>} /> : days.map((d) => (
        <section key={d.d} className="ap-day">
          <h2 className="ap-h2">{d.d}</h2>
          <ul className="ap-feed">
            {d.items.map((a) => {
              const g = gameById(L, a.gameId);
              const I = ACT_ICON[a.kind];
              return <li key={a.id} className={`is-${a.kind}`}><span className="ap-feed-ico"><I size={14} /></span><span className="ap-feed-t">{clock(a.at)}</span><button type="button" className="ap-feed-g" onClick={() => onOpen(g.id)}>{g.title}</button><span className="ap-feed-x">{a.text}</span></li>;
            })}
          </ul>
        </section>
      ))}
    </div>
  );
}

// ---------- Runners ----------
export function RunnersPage({ L, A, onOpen }) {
  const inst = A.runners.filter((r) => r.installed);
  const avail = A.runners.filter((r) => !r.installed);
  const [def, setDef] = useState('ge-10-4');
  const users = (id) => L.games.filter((g) => g.runner === id);
  return (
    <div className="ap-page">
      <PageHead title={A.t.runners} sub="Proton and Wine builds that run your Windows games.">
        <button type="button" className="btn" onClick={() => L.actions.say('Found 6 runners', 'info')}><RefreshCw size={14} />Rescan</button>
      </PageHead>
      <SRow label="Default for new games" desc="Each game can still pick its own."><Select label="Default runner" value={def} options={inst.map((r) => ({ value: r.id, label: r.name }))} onChange={setDef} /></SRow>
      <section>
        <h2 className="ap-h2">Installed</h2>
        <ul className="ap-runners">
          {inst.map((r) => {
            const u = users(r.id);
            return (
              <li key={r.id}>
                <div className="ap-r-main"><b>{r.name}</b>{r.id === def && <span className="ap-pill">Default</span>}<span className="ap-muted">{r.note}</span></div>
                <div className="ap-r-used">{u.length ? <>{u.slice(0, 5).map((g) => <button key={g.id} type="button" className="ap-r-g" title={g.title} onClick={() => onOpen(g.id)}><Img src={artOf(g).header} /></button>)}<span className="ap-muted">{u.length} {u.length === 1 ? 'game' : 'games'}</span></> : <span className="ap-muted">No game uses it</span>}</div>
                <span className="ap-muted ap-r-size">{r.size}</span>
                {r.system ? <span className="ap-muted ap-r-act">From your system</span> : <button type="button" className="btn btn-quiet ap-r-act" disabled={u.length > 0} title={u.length ? 'Switch these games to another runner first' : undefined} onClick={() => A.actions.open('confirm', { title: `Remove ${r.name}?`, body: `Frees ${r.size}.`, yes: 'Remove', run: () => A.actions.removeRunner(r.id) })}><Trash2 size={14} />Remove</button>}
              </li>
            );
          })}
        </ul>
      </section>
      <section>
        <h2 className="ap-h2">Available</h2>
        <ul className="ap-runners">
          {avail.map((r) => (
            <li key={r.id}>
              <div className="ap-r-main"><b>{r.name}</b>{r.isNew && <span className="ap-pill is-acc">New</span>}<span className="ap-muted">{r.note}</span></div>
              <div className="ap-r-used"><span className="ap-muted">Wine {r.wine}</span></div>
              <span className="ap-muted ap-r-size">{r.size}</span>
              <button type="button" className="btn ap-r-act" onClick={() => A.actions.installRunner(r.id)}><Download size={14} />Install</button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

// ---------- Plugins ----------
const CAP_LABEL = { cloud: 'Cloud saves', achievements: 'Achievements', friends: 'Friends', chat: 'Chat', families: 'Family sharing', redeem: 'Redeem codes', storefront: 'Store' };
export function PluginsPage({ L, A }) {
  const [sel, setSel] = useState(A.plugins.find((p) => p.status !== 'ready')?.id || 'steam');
  const p = A.plugins.find((x) => x.id === sel);
  const st = PLUGIN_STATUS[p.status];
  const [opts, setOpts] = useState({ hidden: false, beta: false, family: true });
  return (
    <div className="ap-page ap-plugins">
      <PageHead title={A.t.plugins} sub="Each store comes in through a plugin. Plugins only do what you approve.">
        <button type="button" className="btn" onClick={() => L.actions.say('Rescanning every store', 'info')}><RefreshCw size={14} />Rescan all</button>
        <button type="button" className="btn btn-primary" onClick={() => A.actions.open('add-plugin')}><Plus size={14} />Add a store</button>
      </PageHead>
      <div className="ap-split">
        <ul className="ap-plist" role="listbox" aria-label="Plugins">
          {A.plugins.map((x) => {
            const s = PLUGIN_STATUS[x.status];
            return (
              <li key={x.id}>
                <button type="button" role="option" aria-selected={x.id === sel} className={x.id === sel ? 'is-on' : ''} onClick={() => setSel(x.id)}>
                  <span className="ap-p-ico">{STORES[x.id] ? <StoreGlyph store={x.id} size={16} /> : <Puzzle size={16} />}</span>
                  <span className="ap-p-name">{x.name}<small>{x.account ? x.account : 'Not logged in'}</small></span>
                  <span className={`ap-status is-${s.tone}`}>{s.label}</span>
                </button>
              </li>
            );
          })}
        </ul>
        <section className="ap-pdetail">
          <header>
            <span className="ap-p-ico is-big">{STORES[p.id] ? <StoreGlyph store={p.id} size={22} /> : <Puzzle size={22} />}</span>
            <div><h2>{p.name}</h2><span className="ap-muted">Version {p.version}, by {p.author}{p.games ? `, ${p.games} games` : ''}</span></div>
            <span className={`ap-status is-${st.tone}`}>{st.label}</span>
          </header>
          {p.why && <div className={`ap-alert is-${st.tone}`}><CircleAlert size={14} /><span>{p.why}</span></div>}
          {(p.status === 'unapproved' || p.status === 'changed') && (
            <div className="ap-panel">
              <header><h3>It asks to</h3></header>
              <ul className="ap-perms">{p.perms.map((x) => <li key={x} className={x.includes('(new)') ? 'is-new' : ''}><ShieldCheck size={14} />{x.replace(' (new)', '')}{x.includes('(new)') && <span className="ap-pill is-acc">New</span>}</li>)}</ul>
              <div className="ap-row"><button type="button" className="btn btn-primary" onClick={() => A.actions.approve(p.id)}>Approve</button><button type="button" className="btn btn-quiet">Keep it off</button></div>
            </div>
          )}
          <SRow label="Account" desc={p.account ? `Logged in as ${p.account}` : 'Not logged in'}>
            {p.account ? <button type="button" className="btn btn-quiet" onClick={() => A.actions.logout(p.id)}><LogOut size={14} />Log out</button> : <button type="button" className="btn btn-primary" onClick={() => A.actions.login(p.id)}><LogIn size={14} />Log in</button>}
          </SRow>
          {p.status === 'error' && <SRow label="The plugin stopped" desc="Platypus keeps the last list it got."><button type="button" className="btn" onClick={() => A.actions.retryPlugin(p.id)}><RefreshCw size={14} />Restart</button></SRow>}
          {p.caps.length > 0 && <SRow label="What it brings" desc={p.caps.map((c) => CAP_LABEL[c]).join(', ')} />}
          <h3 className="ap-h3">Settings</h3>
          <Switch checked={opts.hidden} onChange={(v) => setOpts((o) => ({ ...o, hidden: v }))} label="Show games hidden in the store" />
          <Switch checked={opts.beta} onChange={(v) => setOpts((o) => ({ ...o, beta: v }))} label="Offer beta branches" desc="Lists the store’s beta builds in each game’s Versions." />
          {p.caps.includes('families') && <Switch checked={opts.family} onChange={(v) => setOpts((o) => ({ ...o, family: v }))} label="Show games lent by my family" />}
          <SRow label="Library list" desc="Last refreshed 9 minutes ago."><button type="button" className="btn btn-quiet" onClick={() => L.actions.say(`Rescanning ${p.name}`, 'info')}><RefreshCw size={14} />Rescan</button></SRow>
        </section>
      </div>
    </div>
  );
}

// ---------- Storage ----------
export function StoragePage({ L, A, onOpen }) {
  const [pick, setPick] = useState(A.leftovers.map((l) => l.id));
  const inst = L.games.filter(isInstalled);
  const sizeOf = (g) => { const s = activeCopy(g).size; const n = parseFloat(s) || 0; return s.includes('MB') ? n / 1000 : n; };
  const outside = inst.filter((g) => driveOf(g) === 'outside');
  return (
    <div className="ap-page">
      <PageHead title={A.t.storage} sub="Where your games live and how much room is left.">
        <button type="button" className="btn" onClick={() => L.actions.say('Storage rescanned', 'info')}><RefreshCw size={14} />Rescan</button>
      </PageHead>
      {DRIVES.map((d) => {
        const games = inst.filter((g) => driveOf(g) === d.id).sort((a, b) => sizeOf(b) - sizeOf(a));
        const used = d.total - d.free;
        const gamesSize = games.reduce((a, g) => a + sizeOf(g), 0);
        return (
          <section key={d.id} className="ap-drive">
            <header>
              <HardDrive size={18} />
              <div><h2>{d.label}{d.isDefault && <span className="ap-pill">Default</span>}</h2><span className="ap-muted">{d.path}, {d.device}</span></div>
              <span className={`ap-free ${d.free < 20 ? 'is-low' : ''}`}><b>{gb(d.free)}</b> free of {gb(d.total)}</span>
            </header>
            <div className="ap-dbar" role="img" aria-label={`${gb(gamesSize)} of games, ${gb(used - gamesSize)} other files, ${gb(d.free)} free`}>
              {games.map((g) => <span key={g.id} className="is-game" style={{ flex: sizeOf(g) }} title={`${g.title}: ${activeCopy(g).size}`} />)}
              <span className="is-other" style={{ flex: Math.max(0, used - gamesSize) }} title="Other files" />
              <span className="is-free" style={{ flex: d.free }} />
            </div>
            {d.free < 20 && <p className="ap-anote is-warn"><AlertTriangle size={13} />Almost full. Installs on this drive will fail.</p>}
            <table className="tbl ap-dtable">
              <tbody>
                {games.slice(0, 6).map((g) => (
                  <tr key={g.id}>
                    <td><button type="button" className="ap-tt" onClick={() => onOpen(g.id)}><Img src={artOf(g).header} className="ap-tt-art" /><span>{g.title}</span></button></td>
                    <td className="t-dim">{g.hours ? `Played ${ago(g.lastAgo)}` : 'Never played'}</td>
                    <td className="num">{activeCopy(g).size}</td>
                    <td className="num"><button type="button" className="btn btn-quiet" onClick={() => A.actions.open('confirm', { title: `Uninstall ${g.title}?`, body: `Frees ${activeCopy(g).size} on ${d.label}. Saves stay in backups.`, yes: 'Uninstall', run: () => L.actions.say(`Uninstalling ${g.title}`) })}>Uninstall</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
            {games.length > 6 && <p className="ap-muted ap-small">and {games.length - 6} more</p>}
          </section>
        );
      })}
      {outside.length > 0 && (
        <section className="ap-panel">
          <header><h3>Outside your game folders</h3></header>
          <ul className="ap-list">{outside.map((g) => <li key={g.id}><span><b>{g.title}</b><br /><span className="ap-muted">~/Downloads/teardown, {activeCopy(g).size}</span></span><button type="button" className="btn" onClick={() => L.actions.say('Moving to Games')}>Move to Games</button></li>)}</ul>
        </section>
      )}
      <section className="ap-panel">
        <header><h3>Leftovers</h3><button type="button" className="btn btn-primary" disabled={!pick.length} onClick={() => { A.actions.cleanLeftovers(pick); setPick([]); }}>Clean {gb(A.leftovers.filter((l) => pick.includes(l.id)).reduce((a, l) => a + l.size, 0))}</button></header>
        {A.leftovers.length ? <div className="ap-checks">{A.leftovers.map((l) => <Check2 key={l.id} checked={pick.includes(l.id)} onChange={() => setPick((p) => (p.includes(l.id) ? p.filter((x) => x !== l.id) : [...p, l.id]))} hint={gb(l.size)}>{l.label}</Check2>)}</div> : <p className="ap-muted">Nothing left behind.</p>}
      </section>
    </div>
  );
}

// ---------- Global settings ----------
export function AppSettingsBody({ L, A, cat }) {
  const S = A.settings;
  const set = A.actions.setSetting;
  switch (cat) {
    case 'general':
      return (
        <>
          <SRow label="Language" desc="French strings run about 30% longer; try it to see the layout hold."><Select label="Language" value={A.lang} options={[{ value: 'en', label: 'English' }, { value: 'fr', label: 'Français' }, { value: 'system', label: 'Same as the system' }]} onChange={(v) => A.actions.setLang(v === 'system' ? 'en' : v)} /></SRow>
          <Switch checked={S.tray} onChange={(v) => set('tray', v)} label="Minimize to the tray" desc="Closing the window keeps downloads and chat running." />
        </>
      );
    case 'appearance':
      return (
        <>
          <SRow label="Theme"><Seg label="Theme" value={A.theme} onChange={A.actions.setTheme} options={[{ value: 'dark', label: 'Dark' }, { value: 'black', label: 'Black' }, { value: 'light', label: 'Light' }]} /></SRow>
          <SRow label="Game colour" desc="Take colour from the selected game’s art."><Seg label="Game colour" value={A.tint} onChange={A.actions.setTint} options={[{ value: 'off', label: 'Off' }, { value: 'accent', label: 'Accent' }, { value: 'whole', label: 'Whole app' }]} /></SRow>
        </>
      );
    case 'folders':
      return (
        <>
          {DRIVES.map((d) => <SRow key={d.id} label={d.path} desc={`${d.label}, ${gb(d.free)} free`}>{d.isDefault ? <span className="ap-pill">Default</span> : <button type="button" className="btn btn-quiet">Make default</button>}</SRow>)}
          <SRow label="Add a folder"><button type="button" className="btn"><Plus size={14} />Add</button></SRow>
          <SRow label="Wine prefixes" desc={S.prefix}><button type="button" className="btn btn-quiet">Change</button></SRow>
          <SRow label="Canceled downloads" desc="Partial files from canceled installs."><Select label="Canceled downloads" value={S.cleanup} options={[{ value: 'now', label: 'Delete right away' }, { value: 'week', label: 'Keep for a week' }, { value: 'keep', label: 'Keep until I clean' }]} onChange={(v) => set('cleanup', v)} /></SRow>
        </>
      );
    case 'saves':
      return (
        <>
          <Switch checked={S.backups} onChange={(v) => set('backups', v)} label="Back up saves automatically" desc="Before every cloud sync and after each session." />
          <SRow label="Keep" desc="Older backups are removed."><Select label="Backups to keep" value={String(S.keep)} options={['3', '5', '10', '20'].map((x) => ({ value: x, label: `${x} per game` }))} onChange={(v) => set('keep', +v)} /></SRow>
        </>
      );
    case 'data':
      return (
        <>
          <Switch checked={S.ratings} onChange={(v) => set('ratings', v)} label="Compatibility ratings" desc="ProtonDB tiers and Steam Deck ratings." />
          <Switch checked={S.anticheat} onChange={(v) => set('anticheat', v)} label="Anti-cheat and protection data" desc="Warns before you buy or install a game that blocks Linux." />
          <Switch checked={S.artLookup} onChange={(v) => set('artLookup', v)} label="Look up artwork for games you add" desc="Covers and banners from SteamGridDB." />
          <Switch checked={S.infoLookup} onChange={(v) => set('infoLookup', v)} label="Look up game info for games you add" desc="Description, developer and year." />
        </>
      );
    case 'social':
      return (
        <>
          <SRow label="Chat notifications"><Select label="Chat notifications" value={S.chatNotify} options={[{ value: 'all', label: 'Every message' }, { value: 'mentions', label: 'Only while away from chat' }, { value: 'off', label: 'Off' }]} onChange={(v) => set('chatNotify', v)} /></SRow>
          {storesWith('friends').map((s) => <Switch key={s} checked={A.social.stores[s] !== false} onChange={() => A.actions.toggleSocialStore(s)} label={`${storeName(s)} friends`} />)}
        </>
      );
    case 'stores':
      return <Switch checked={S.steamNotices} onChange={(v) => set('steamNotices', v)} label="Steam client notices" desc="Tell me when a game needs the Steam client running in the background." />;
    default:
      return null;
  }
}
export function AppSettingsPage({ L, A }) {
  const [cat, setCat] = useState('general');
  return (
    <div className="ap-page ap-settings">
      <PageHead title={A.t.settings} />
      <div className="ap-split is-narrow">
        <nav className="ap-snav" aria-label="Settings sections">{APP_SETTINGS_CATS.map((c) => <button key={c.id} type="button" className={cat === c.id ? 'is-on' : ''} aria-current={cat === c.id} onClick={() => setCat(c.id)}>{c.label}</button>)}</nav>
        <section className="ap-sbody"><h2>{APP_SETTINGS_CATS.find((c) => c.id === cat).label}</h2><AppSettingsBody L={L} A={A} cat={cat} /></section>
      </div>
    </div>
  );
}

// ---------- Embedded storefront ----------
const SHOP = [
  { appid: 1030300, title: 'Hollow Knight: Silksong', price: '19,50 €', tag: 'New' },
  { appid: 1903340, title: 'Clair Obscur: Expedition 33', price: '44,99 €', was: '49,99 €' },
  { appid: 2694490, title: 'Path of Exile 2', price: '29,99 €', note: 'Anti-cheat works on Linux' },
  { appid: 1145360, title: 'Hades', price: '6,24 €', was: '24,99 €' },
  { appid: 413150, title: 'Stardew Valley', owned: 'stardew' },
  { appid: 1086940, title: 'Baldur’s Gate 3', owned: 'bg3' },
];
export function StorefrontPage({ L, A, onOpen }) {
  const stores = storesWith('storefront').filter((s) => L.games.some((g) => g.stores.some((c) => c.store === s)));
  const [store, setStore] = useState('steam');
  const [path, setPath] = useState(['Featured']);
  const host = { steam: 'store.steampowered.com', gog: 'www.gog.com', epic: 'store.epicgames.com', amazon: 'gaming.amazon.com', itch: 'itch.io' }[store];
  const plug = A.plugins.find((p) => p.id === store);
  return (
    <div className="ap-shop">
      <div className="ap-shop-bar">
        <div className="ap-seg" role="tablist" aria-label="Store">{stores.map((s) => <button key={s} type="button" role="tab" aria-selected={s === store} className={s === store ? 'is-on' : ''} onClick={() => { setStore(s); setPath(['Featured']); }}><StoreGlyph store={s} size={13} />{storeName(s)}</button>)}</div>
        <div className="ap-browser">
          <button type="button" className="icon-btn" aria-label="Back" disabled={path.length < 2} onClick={() => setPath((p) => p.slice(0, -1))}><ChevronLeft size={16} /></button>
          <button type="button" className="icon-btn" aria-label="Forward" disabled><ChevronRight size={16} /></button>
          <button type="button" className="icon-btn" aria-label="Reload" onClick={() => L.actions.say('Reloaded', 'info')}><RefreshCw size={14} /></button>
          <span className="ap-url"><Globe size={12} />{host}/{path[path.length - 1].toLowerCase().replace(/[^a-z0-9]+/g, '-')}</span>
        </div>
        {can(store, 'redeem') && <button type="button" className="btn" onClick={() => A.actions.open('redeem', { store })}><Ticket size={14} />{A.t.redeem}</button>}
      </div>
      {plug?.status === 'needs-login' ? <Empty icon={LogIn} title={`Log in to ${plug.name} to shop`} action={<button type="button" className="btn btn-primary" onClick={() => A.actions.login(store)}>Log in</button>}>Your session expired. Games you own stay playable.</Empty> : (
        <div className="ap-shop-page">
          <p className="ap-shop-crumb">{path.join(' / ')}</p>
          <section className="ap-shop-hero" style={{ '--bg': `url(https://cdn.cloudflare.steamstatic.com/steam/apps/${SHOP[0].appid}/library_hero.jpg)` }}>
            <div><small>{storeName(store)} store page, shown inside Platypus</small><h2>{SHOP[0].title}</h2><p>Runs natively on Linux. Wishlisted by 3 friends.</p><button type="button" className="btn btn-primary" onClick={() => setPath((p) => [...p, SHOP[0].title])}>{SHOP[0].price}, view</button></div>
          </section>
          <ul className="ap-shop-grid">
            {SHOP.slice(1).map((x) => (
              <li key={x.appid}>
                <button type="button" onClick={() => (x.owned ? onOpen(x.owned) : setPath((p) => [...p, x.title]))}>
                  <Img src={`https://cdn.cloudflare.steamstatic.com/steam/apps/${x.appid}/header.jpg`} className="ap-shop-art" />
                  <span className="ap-shop-t">{x.title}</span>
                  {x.owned ? <span className="ap-pill is-ok">In your library</span> : <span className="ap-shop-p">{x.was && <s>{x.was}</s>}{x.price}</span>}
                  {x.note && <small className="ap-muted">{x.note}</small>}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

// ---------- Social ----------
export function Avatar({ f, size = 28, me }) {
  const name = me ? 'polynux' : f.name;
  const hue = me ? 210 : avatarHue(f.avatar);
  const status = me ? me : f.status;
  return (
    <span className="ap-av" style={{ '--h': hue, '--s': `${size}px` }} aria-hidden>
      {name[0].toUpperCase()}
      <span className={`ap-av-dot is-${status}`} />
    </span>
  );
}
export function friendLine(L, f) {
  if (f.status === 'ingame') return `Playing ${f.gameId ? gameById(L, f.gameId)?.title : f.gameTitle}`;
  if (f.status === 'offline') return `Last online ${ago(f.lastSeen)}`;
  return FRIEND_STATUS[f.status];
}
export function FriendRow({ L, A, f, onChat }) {
  const c = A.chats[f.id];
  return (
    <li>
      <button type="button" className={`ap-friend is-${f.status} ${A.social.chat === f.id ? 'is-on' : ''}`} onClick={() => (onChat || A.actions.openChat)(f.id)}>
        <Avatar f={f} />
        <span className="ap-friend-t"><span>{f.name}<StoreGlyph store={f.store} size={10} /></span><small>{friendLine(L, f)}</small></span>
        {c?.unread > 0 && <span className="ap-badge">{c.unread}</span>}
      </button>
    </li>
  );
}

const ME_STATUS = [{ value: 'online', label: 'Online' }, { value: 'away', label: 'Away' }, { value: 'invisible', label: 'Invisible' }];
export function MeCard({ L, A, compact }) {
  const [open, setOpen] = useState(false);
  const S = A.social;
  return (
    <div className="ap-me">
      <span className="ap-anchor">
        <button type="button" className="ap-me-btn" aria-expanded={open} onClick={() => setOpen(!open)}>
          <Avatar me={S.me} />
          {!compact && <span className="ap-friend-t"><span>polynux</span><small>{L.running && S.showPlaying && S.me !== 'invisible' ? `Playing ${gameById(L, L.running.id)?.title}` : ME_STATUS.find((x) => x.value === S.me).label}</small></span>}
          {!compact && <ChevronDown size={13} />}
        </button>
        <Popover open={open} onClose={() => setOpen(false)} label="Your status" className="ap-menu ap-me-pop">
          {ME_STATUS.map((s) => <button key={s.value} type="button" className={S.me === s.value ? 'is-on' : ''} onClick={() => { A.actions.setMe(s.value); }}><span className={`ap-av-dot is-${s.value} is-inline`} />{s.label}{S.me === s.value && <Check size={13} className="ap-menu-check" />}</button>)}
          <hr />
          <Check2 checked={S.showPlaying} onChange={(v) => A.actions.setSocialOpt('showPlaying', v)}>Show what I’m playing</Check2>
          <Check2 checked={S.alerts} onChange={(v) => A.actions.setSocialOpt('alerts', v)}>Message alerts</Check2>
        </Popover>
      </span>
    </div>
  );
}

export function FriendsList({ L, A, onChat, q = '' }) {
  const on = (f) => A.social.stores[f.store] !== false;
  const list = FRIENDS.filter((f) => on(f) && f.name.toLowerCase().includes(q.toLowerCase()));
  const groups = [
    ['Playing', list.filter((f) => f.status === 'ingame')],
    ['Online', list.filter((f) => f.status === 'online' || f.status === 'away')],
    ['Offline', list.filter((f) => f.status === 'offline')],
  ].filter(([, l]) => l.length);
  const off = storesWith('friends').filter((s) => A.social.stores[s] === false);
  return (
    <div className="ap-friends">
      {groups.map(([h, l]) => (
        <section key={h}><h3 className="ap-fgroup">{h}<span>{l.length}</span></h3><ul className="ap-flist">{l.map((f) => <FriendRow key={f.id} L={L} A={A} f={f} onChat={onChat} />)}</ul></section>
      ))}
      {off.map((s) => (
        <div key={s} className="ap-foff"><StoreGlyph store={s} size={12} /><span>{storeName(s)} friends are off</span><button type="button" onClick={() => A.actions.toggleSocialStore(s)}>Turn on</button></div>
      ))}
      <div className="ap-foff is-warn"><WifiOff size={12} /><span>GOG chat disconnected 2 min ago</span><button type="button" onClick={() => L.actions.say('Reconnected to GOG', 'ok')}>Reconnect</button></div>
    </div>
  );
}

export function ChatView({ L, A, fid, onBack, compact }) {
  const f = FRIENDS.find((x) => x.id === fid);
  const c = A.chats[fid] || { msgs: [] };
  const [text, setText] = useState('');
  const body = useRef();
  useEffect(() => { body.current?.scrollTo({ top: 1e6 }); }, [fid, c.msgs.length]);
  if (!f) return null;
  const chatCap = can(f.store, 'chat');
  return (
    <div className={`ap-chat ${compact ? 'is-compact' : ''}`} data-keys-local>
      <header>
        {onBack && <button type="button" className="icon-btn" aria-label="Back to friends" onClick={onBack}><ChevronLeft size={16} /></button>}
        <Avatar f={f} size={30} />
        <span className="ap-friend-t"><span>{f.name}<StoreGlyph store={f.store} size={10} /></span><small>{friendLine(L, f)}</small></span>
        {f.status === 'ingame' && f.gameId && <button type="button" className="btn btn-quiet" onClick={() => A.actions.openGame(f.gameId)}>View game</button>}
        <button type="button" className="icon-btn" aria-label={A.social.alerts ? 'Mute alerts' : 'Unmute alerts'} onClick={() => A.actions.setSocialOpt('alerts', !A.social.alerts)}>{A.social.alerts ? <Bell size={14} /> : <BellOff size={14} />}</button>
      </header>
      <div className="ap-chat-body" ref={body}>
        {c.older && <button type="button" className="ap-older" onClick={() => A.actions.loadOlder(fid)}>Load older messages</button>}
        {c.msgs.length === 0 && <p className="ap-muted ap-chat-empty">Say hi to {f.name}.</p>}
        {c.msgs.map((m) => (
          <div key={m.id} className={`ap-msg is-${m.from} ${m.failed ? 'is-failed' : ''}`}>
            <p>{m.text}</p>
            <small>{m.failed ? <>Not sent. <button type="button" onClick={() => A.actions.resend(fid, m.id)}>Resend</button></> : clock(m.at)}</small>
          </div>
        ))}
      </div>
      {chatCap ? (
        <form className="ap-chat-in" onSubmit={(e) => { e.preventDefault(); if (text.trim()) { A.actions.send(fid, text.trim()); setText(''); } }}>
          <input value={text} onChange={(e) => setText(e.target.value)} placeholder={`Message ${f.name}`} aria-label={`Message ${f.name}`} />
          <button type="submit" className="icon-btn" aria-label="Send" disabled={!text.trim()}><Send size={15} /></button>
        </form>
      ) : <p className="ap-chat-none">{storeName(f.store)} doesn’t offer chat to plugins.</p>}
    </div>
  );
}

export function FamilyPanel({ L, A }) {
  const lent = Object.entries(LENT).map(([id, l]) => ({ g: gameById(L, id), ...l })).filter((x) => x.g);
  return (
    <div className="ap-family">
      <header><StoreGlyph store={FAMILY.store} size={13} /><b>{FAMILY.name}</b><span className="ap-muted">{FAMILY.members.length} members</span></header>
      <ul className="ap-fam-m">{FAMILY.members.map((m) => <li key={m.id}><span className="ap-av" style={{ '--h': (m.name.length * 71) % 360, '--s': '22px' }} aria-hidden>{m.name[0]}</span>{m.name}{m.you && <small> (you)</small>}<small className="ap-muted">{m.role}</small></li>)}</ul>
      <h3 className="ap-fgroup">Lent to you<span>{lent.length}</span></h3>
      <ul className="ap-lentlist">
        {lent.map((x) => (
          <li key={x.g.id} className={x.inUse ? 'is-busy' : ''}>
            <button type="button" onClick={() => A.actions.openGame(x.g.id)}><Img src={artOf(x.g).header} className="ap-tt-art" /><span><b>{x.g.title}</b><small>{x.inUse ? `${x.owner} is playing it now` : `From ${x.owner}`}</small></span></button>
          </li>
        ))}
      </ul>
    </div>
  );
}

// The full social surface: friends, chat, family, as tabs.
export function SocialPanel({ L, A, layout = 'stack' }) {
  const [tab, setTab] = useState('friends');
  const [q, setQ] = useState('');
  const fid = A.social.chat;
  const chats = Object.entries(A.chats).filter(([, c]) => c.msgs.length);
  if (layout === 'stack' && fid) return <ChatView L={L} A={A} fid={fid} onBack={() => A.actions.openChat(null)} />;
  return (
    <div className={`ap-social is-${layout}`}>
      <div className="ap-social-side">
        <MeCard L={L} A={A} />
        <div className="ap-seg ap-social-tabs" role="tablist" aria-label="Social">
          {[['friends', 'Friends'], ['chats', `Chats${A.unread ? ` ${A.unread}` : ''}`], ['family', 'Family']].map(([id, label]) => <button key={id} type="button" role="tab" aria-selected={tab === id} className={tab === id ? 'is-on' : ''} onClick={() => setTab(id)}>{label}</button>)}
        </div>
        {tab === 'friends' && <><label className="ap-fsearch"><Search size={13} /><input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Find a friend" /></label><FriendsList L={L} A={A} q={q} /></>}
        {tab === 'chats' && <ul className="ap-flist">{chats.map(([id, c]) => { const f = FRIENDS.find((x) => x.id === id); return <li key={id}><button type="button" className={`ap-friend ${fid === id ? 'is-on' : ''}`} onClick={() => A.actions.openChat(id)}><Avatar f={f} /><span className="ap-friend-t"><span>{f.name}</span><small>{c.msgs[c.msgs.length - 1].text}</small></span>{c.unread > 0 && <span className="ap-badge">{c.unread}</span>}</button></li>; })}</ul>}
        {tab === 'family' && <FamilyPanel L={L} A={A} />}
      </div>
      {layout === 'split' && <div className="ap-social-main">{fid ? <ChatView L={L} A={A} fid={fid} /> : <Empty icon={MessageSquare} title="Pick a friend to chat">Messages from Steam and GOG friends land here.</Empty>}</div>}
    </div>
  );
}

// ---------- per-game settings panel ----------
export function GameSettingsPanel({ L, A, initial = 'compat', layout = 'accordion' }) {
  const [open, setOpen] = useState(initial);
  useEffect(() => setOpen(initial), [initial]);
  const groups = [...new Set(GAME_CATS.map((c) => c.group))];
  if (layout === 'split') {
    const cur = GAME_CATS.find((c) => c.id === open) || GAME_CATS[0];
    return (
      <div className="ap-gs is-split">
        <nav className="ap-snav" aria-label="Game settings">
          {groups.map((gr) => <React.Fragment key={gr}><span className="ap-snav-h">{gr}</span>{GAME_CATS.filter((c) => c.group === gr).map((c) => <button key={c.id} type="button" className={cur.id === c.id ? 'is-on' : ''} aria-current={cur.id === c.id} onClick={() => setOpen(c.id)}>{c.label}<small>{gameCatSummary(L, A, c.id)}</small></button>)}</React.Fragment>)}
        </nav>
        <section className="ap-sbody" key={cur.id}><h2>{cur.label}</h2><p className="ap-muted">{cur.blurb}</p><GameSettingsCategory L={L} A={A} id={cur.id} /></section>
      </div>
    );
  }
  return (
    <div className="ap-gs">
      {groups.map((gr) => (
        <div key={gr}>
          <div className="acc-group">{gr}</div>
          {GAME_CATS.filter((c) => c.group === gr).map((c) => {
            const isOpen = open === c.id;
            const Icon = c.icon || Sparkles;
            return (
              <section key={c.id} className={`acc ${isOpen ? 'is-open' : ''}`}>
                <h3><button type="button" className="acc-h" aria-expanded={isOpen} onClick={() => setOpen(isOpen ? null : c.id)}><Icon size={16} /><span className="acc-label">{c.label}</span><span className="acc-sum">{gameCatSummary(L, A, c.id)}</span><ChevronDown size={15} className="acc-chev" /></button></h3>
                {isOpen && <div className="acc-body"><GameSettingsCategory L={L} A={A} id={c.id} /></div>}
              </section>
            );
          })}
        </div>
      ))}
    </div>
  );
}

// ---------- dialogs ----------
function Dialog({ title, sub, onClose, children, foot, className = '' }) {
  const ref = useRef();
  useEffect(() => {
    const prev = document.activeElement;
    ref.current?.querySelector('input, button:not(.dialog-x)')?.focus();
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel')) { e.stopPropagation(); onClose(); } };
    window.addEventListener('keydown', key, true);
    return () => { window.removeEventListener('keydown', key, true); prev?.focus?.(); };
  }, []);
  return (
    <div className="scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className={`dialog ap-dialog ${className}`} role="dialog" aria-modal="true" aria-label={title}>
        <div className="dialog-head"><div><h2>{title}</h2>{sub && <span className="ap-muted">{sub}</span>}</div><button type="button" className="icon-btn dialog-x" aria-label="Close" onClick={onClose}><X size={15} /></button></div>
        <div className="ap-dialog-body">{children}</div>
        {foot && <div className="ap-dialog-foot">{foot}</div>}
      </div>
    </div>
  );
}

function RedeemDialog({ L, A, d }) {
  const stores = storesWith('redeem').filter((s) => A.plugins.some((p) => p.id === s));
  const [store, setStore] = useState(d.store || 'steam');
  const [code, setCode] = useState('');
  const [state, setState] = useState(null);
  const ok = code.replace(/[^A-Z0-9]/gi, '').length >= 15;
  return (
    <Dialog title={A.t.redeem} sub="Product keys from bundles, boxes and gifts." onClose={A.actions.close} foot={<><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Cancel</button><button type="button" className="btn btn-primary" disabled={!ok || state === 'busy'} onClick={() => { setState('busy'); setTimeout(() => setState(code.startsWith('X') ? 'used' : 'done'), 900); }}>{state === 'busy' ? 'Checking' : 'Redeem'}</button></>}>
      <SRow label="Store"><Select label="Store" value={store} options={stores.map((s) => ({ value: s, label: storeName(s) }))} onChange={setStore} /></SRow>
      <label className="ap-field"><span>Code</span><input className="ap-input is-code" value={code} onChange={(e) => { setCode(e.target.value.toUpperCase()); setState(null); }} placeholder={store === 'steam' ? 'AAAAA-BBBBB-CCCCC' : 'XXXX-XXXX-XXXX-XXXX'} spellCheck={false} /></label>
      {state === 'done' && <div className="ap-alert is-ok"><Check size={14} /><span><b>Hades</b> was added to your {storeName(store)} library.</span><button type="button" className="btn" onClick={() => { A.actions.close(); A.actions.go('library'); }}>Show</button></div>}
      {state === 'used' && <div className="ap-alert is-bad"><AlertTriangle size={14} /><span>This code was already used on another account.</span></div>}
      <p className="ap-muted ap-small">Codes starting with X show the error state.</p>
    </Dialog>
  );
}

function AddGameDialog({ L, A }) {
  const [path, setPath] = useState('');
  const [title, setTitle] = useState('');
  return (
    <Dialog title={A.t.addGame} sub="Any program on your disk: a GOG installer, an itch download, a Windows .exe." onClose={A.actions.close} foot={<><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Cancel</button><button type="button" className="btn btn-primary" disabled={!path} onClick={() => { L.actions.say(`Added ${title || 'the game'}`); A.actions.close(); }}>Add to library</button></>}>
      <label className="ap-field"><span>Program</span><span className="ap-row"><input className="ap-input" value={path} onChange={(e) => setPath(e.target.value)} placeholder="~/Games/…/game.exe" /><button type="button" className="btn" onClick={() => { setPath('~/Games/Tunic/Tunic.exe'); setTitle('Tunic'); }}><FolderOpen size={14} />Browse</button></span></label>
      <label className="ap-field"><span>Title</span><input className="ap-input" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Found from the file name" /></label>
      {path.endsWith('.exe') && <SRow label="Runs with" desc="It’s a Windows program, so it needs a runner."><Select label="Runner" value="ge-10-4" options={A.runners.filter((r) => r.installed).map((r) => ({ value: r.id, label: r.name }))} onChange={() => {}} /></SRow>}
      <Switch checked={A.settings.artLookup} onChange={(v) => A.actions.setSetting('artLookup', v)} label="Look up artwork and info" desc={title ? `Found “${title}” on SteamGridDB` : 'Searches by title once you pick a file.'} />
    </Dialog>
  );
}

function InstallDialog({ L, A, d }) {
  const g = gameById(L, d.gameId);
  const copy = activeCopy(g);
  const [drive, setDrive] = useState(DRIVES.find((x) => x.free > 50)?.id || 'home');
  const [ver, setVer] = useState('latest');
  const [opts, setOpts] = useState({ shortcut: true, dlc: true });
  const issue = issueOf(A, g);
  const need = parseFloat(copy.size) || 1;
  const dv = DRIVES.find((x) => x.id === drive);
  const short = dv.free < need;
  return (
    <Dialog title={`${d.move ? 'Move' : 'Install'} ${g.title}`} sub={`${storeName(copy.store)}, ${copy.size}`} onClose={A.actions.close} className="is-wide" foot={<><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Cancel</button><button type="button" className="btn btn-primary" disabled={short} onClick={() => { if (d.move) L.actions.say(`Moving ${g.title}`); else L.actions.play(g.id); A.actions.close(); }}>{d.move ? 'Move' : 'Install'}{short ? '' : ` to ${dv.label}`}</button></>}>
      {issue?.kind === 'install' && <div className="ap-alert is-bad"><AlertTriangle size={14} /><span>Last attempt: {issue.reason}</span></div>}
      <fieldset className="ap-drives">
        <legend>Folder</legend>
        {DRIVES.map((x) => (
          <button key={x.id} type="button" role="radio" aria-checked={drive === x.id} className={`ap-drv ${drive === x.id ? 'is-on' : ''}`} onClick={() => setDrive(x.id)}>
            <HardDrive size={16} /><span><b>{x.label}</b><small>{x.path}</small></span><span className={x.free < need ? 't-bad' : 'ap-muted'}>{gb(x.free)} free</span>
          </button>
        ))}
        <button type="button" className="ap-drv is-other"><FolderOpen size={16} /><span><b>Another folder…</b></span></button>
      </fieldset>
      {short && <p className="ap-anote is-bad"><AlertTriangle size={13} />Needs {gb(need - dv.free)} more on {dv.label}.</p>}
      {!d.move && <SRow label="Version"><Select label="Version" value={ver} options={[{ value: 'latest', label: `${copy.version} (latest)` }, { value: 'beta', label: 'Public beta' }]} onChange={setVer} /></SRow>}
      {!d.move && <div className="ap-checks"><Check2 checked={opts.dlc} onChange={(v) => setOpts((o) => ({ ...o, dlc: v }))} hint="included">Add-ons you own</Check2><Check2 checked={opts.shortcut} onChange={(v) => setOpts((o) => ({ ...o, shortcut: v }))}>Add to the applications menu</Check2></div>}
    </Dialog>
  );
}

function CompareDialog({ L, A, d }) {
  const g = gameById(L, d.gameId);
  const rows = [
    ['Installed', (s) => (s.installed ? 'Yes' : 'No')],
    ['Version', (s) => s.version],
    ['Size', (s) => s.size],
    ['Add-ons', (s) => (s.store === 'steam' ? '1 of 1' : s.store === 'gog' ? '1 of 1, DRM-free' : '—')],
    ['Cloud saves', (s) => (can(s.store, 'cloud') ? 'Yes' : 'No')],
    ['Achievements', (s) => (can(s.store, 'achievements') ? 'Yes' : 'No')],
    ['Lent', (s) => (LENT[g.id]?.store === s.store ? `By ${LENT[g.id].owner}` : 'You own it')],
  ];
  return (
    <Dialog title={`Your copies of ${g.title}`} onClose={A.actions.close} className="is-wide">
      <table className="tbl ap-cmp">
        <thead><tr><th />{g.stores.map((s) => <th key={s.store}><StoreGlyph store={s.store} size={13} /> {storeName(s.store)}{g.source === s.store && <span className="ap-pill">Launches</span>}</th>)}</tr></thead>
        <tbody>{rows.map(([k, fn]) => <tr key={k}><td className="t-dim">{k}</td>{g.stores.map((s) => <td key={s.store}>{fn(s)}</td>)}</tr>)}</tbody>
        <tfoot><tr><td />{g.stores.map((s) => <td key={s.store}>{g.source === s.store ? <span className="ap-muted">In use</span> : <button type="button" className="btn" onClick={() => { L.actions.setSource(g.id, s.store); A.actions.close(); }}>Launch this one</button>}</td>)}</tr></tfoot>
      </table>
    </Dialog>
  );
}

function ConfirmDialog({ A, d }) {
  return (
    <Dialog title={d.title} onClose={A.actions.close} foot={<><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Cancel</button><button type="button" className={`btn ${d.tone === 'primary' ? 'btn-primary' : 'btn-bad'}`} onClick={() => { d.run?.(); A.actions.close(); }}>{d.yes}</button></>}>
      <p className="ap-muted">{d.body}</p>
    </Dialog>
  );
}

function TermsDialog({ L, A, d }) {
  return (
    <Dialog title={d.game ? `${d.game} end user licence` : `${storeName(d.store)} subscriber agreement`} sub={d.pending ? 'Accept to continue' : 'Accepted on 12 Mar 2025'} onClose={A.actions.close} className="is-wide" foot={d.pending ? <><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Decline</button><button type="button" className="btn btn-primary" onClick={() => { L.actions.say('Accepted'); A.actions.close(); }}>Accept</button></> : <button type="button" className="btn" onClick={A.actions.close}>Close</button>}>
      <div className="ap-terms">
        <h3>1. Registration as a subscriber</h3><p>You become a subscriber by completing the registration process and agreeing to these terms. You are responsible for the confidentiality of your login and password.</p>
        <h3>2. Licences</h3><p>The content and services are licensed, not sold. Your licence confers no title or ownership in the content and services.</p>
        <h3>3. Billing, payment and other subscriptions</h3><p>All charges incurred on your account will be billed to the payment method you designate during the registration process.</p>
        <p className="ap-muted">Shortened for the design lab. The real text comes from the store plugin.</p>
      </div>
    </Dialog>
  );
}

function CollectionDialog({ L, A, d }) {
  const edit = d.edit && A.collections.find((c) => c.id === d.edit);
  const [name, setName] = useState(edit?.name || '');
  const [kind, setKind] = useState(edit?.kind || 'manual');
  const [tiers, setTiers] = useState(edit?.rule?.tier || []);
  const [status, setStatus] = useState(edit?.rule?.status || []);
  const preview = kind === 'smart' ? L.games.filter((g) => (!tiers.length || tiers.includes(g.tier)) && (!status.length || status.includes(A.status[g.id] || 'none'))).length : d.add ? 1 : edit?.ids?.length || 0;
  const save = () => {
    const rule = { tier: tiers.length ? tiers : undefined, status: status.length ? status : undefined };
    if (edit) A.actions.renameCollection(edit.id, name);
    else A.actions.createCollection({ name, kind, ids: d.add ? [d.add] : [], rule, ruleText: 'Custom rule' });
    A.actions.close();
  };
  return (
    <Dialog title={edit ? `Edit “${edit.name}”` : A.t.newCollection} onClose={A.actions.close} foot={<><button type="button" className="btn btn-quiet" onClick={A.actions.close}>Cancel</button><button type="button" className="btn btn-primary" disabled={!name.trim()} onClick={save}>{edit ? 'Save' : 'Create'}</button></>}>
      <label className="ap-field"><span>Name</span><input className="ap-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Weekend games" /></label>
      {!edit && <SRow label="Kind" desc={kind === 'smart' ? 'Fills itself from a rule and stays up to date.' : 'You add games by hand.'}><Seg label="Kind" value={kind} onChange={setKind} options={[{ value: 'manual', label: 'Manual' }, { value: 'smart', label: 'Smart' }]} /></SRow>}
      {kind === 'smart' && (
        <div className="ap-rule">
          <span className="ap-rule-h">Runs on Linux</span>
          <div className="ap-chips">{TIERS.map((t) => <button key={t} type="button" className={`ap-chip ${tiers.includes(t) ? 'is-on' : ''}`} aria-pressed={tiers.includes(t)} onClick={() => setTiers((x) => (x.includes(t) ? x.filter((y) => y !== t) : [...x, t]))}>{TIER_LABEL[t]}</button>)}</div>
          <span className="ap-rule-h">Your status</span>
          <div className="ap-chips">{OWN_STATUS.map((s) => <button key={s.id} type="button" className={`ap-chip ${status.includes(s.id) ? 'is-on' : ''}`} aria-pressed={status.includes(s.id)} onClick={() => setStatus((x) => (x.includes(s.id) ? x.filter((y) => y !== s.id) : [...x, s.id]))}>{s.label}</button>)}</div>
        </div>
      )}
      <p className="ap-muted ap-small">{preview} {preview === 1 ? 'game' : 'games'} {kind === 'smart' ? 'match today' : 'in it'}.</p>
    </Dialog>
  );
}

const CATALOG = [
  { id: 'humble', name: 'Humble Bundle', by: 'Community', note: 'Library and downloads' },
  { id: 'ubisoft', name: 'Ubisoft Connect', by: 'Community', note: 'Library only, games launch through the client' },
  { id: 'battlenet', name: 'Battle.net', by: 'Community', note: 'Client launches' },
  { id: 'emu', name: 'ROM folders', by: 'Platypus', note: 'Emulated games from a folder' },
];
function AddPluginDialog({ L, A }) {
  return (
    <Dialog title="Add a store" sub="Plugins bring a store’s library, downloads and friends into Platypus." onClose={A.actions.close} className="is-wide">
      <ul className="ap-list">{CATALOG.map((c) => <li key={c.id}><span className="ap-p-ico"><Puzzle size={16} /></span><span><b>{c.name}</b><br /><span className="ap-muted">{c.note}, by {c.by}</span></span><button type="button" className="btn" onClick={() => { L.actions.say(`${c.name} installed. Approve it in Plugins.`); A.actions.close(); }}>Install</button></li>)}</ul>
      <SRow label="From a file" desc="A plugin someone sent you. You approve what it may do before it runs."><button type="button" className="btn btn-quiet"><FolderOpen size={14} />Choose file</button></SRow>
    </Dialog>
  );
}

function GameSettingsDialog({ L, A, d }) {
  return (
    <Dialog title={`${L.game.title} settings`} onClose={A.actions.close} className="is-settings">
      <GameSettingsPanel L={L} A={A} initial={d.cat || 'compat'} layout="split" />
    </Dialog>
  );
}

export function AppDialogs({ L, A, gameSettings = true }) {
  const d = A.dialog;
  if (!d) return null;
  if (d.kind === 'redeem') return <RedeemDialog L={L} A={A} d={d} />;
  if (d.kind === 'add-game') return <AddGameDialog L={L} A={A} />;
  if (d.kind === 'install') return <InstallDialog L={L} A={A} d={d} />;
  if (d.kind === 'compare') return <CompareDialog L={L} A={A} d={d} />;
  if (d.kind === 'confirm') return <ConfirmDialog A={A} d={d} />;
  if (d.kind === 'terms') return <TermsDialog L={L} A={A} d={d} />;
  if (d.kind === 'collection') return <CollectionDialog L={L} A={A} d={d} />;
  if (d.kind === 'add-plugin') return <AddPluginDialog L={L} A={A} />;
  if (d.kind === 'game-settings' && gameSettings) return <GameSettingsDialog L={L} A={A} d={d} />;
  return null;
}

// ---------- chrome ----------
export function StatusFooter({ L, A }) {
  const run = L.running && gameById(L, L.running.id);
  const sec = run ? Math.floor((Date.now() - L.running.startedAt) / 1000) : 0;
  return (
    <footer className="ap-foot" aria-label="Status">
      {run && <span className="ap-foot-i is-live"><span className="tele-dot" />{run.title}<b>{Math.floor(sec / 60)}:{String(sec % 60).padStart(2, '0')}</b><button type="button" className="ap-foot-stop" onClick={L.actions.stop} aria-label={`Stop ${run.title}`}><Square size={9} fill="currentColor" />{A.t.stop}</button></span>}
      {A.jobs.map((j) => <button key={j.id} type="button" className={`ap-foot-i ${A.paused.includes(j.id) ? 'is-paused' : ''}`} style={{ '--p': j.progress }} onClick={() => A.actions.go('downloads')}>{j.label.split('·')[0].trim()}<b>{Math.round(j.progress * 100)}%</b></button>)}
      <span className="ap-foot-sp" />
      {A.demo === 'offline' && <span className="ap-foot-i is-warn"><WifiOff size={12} />Offline</span>}
      <span className="ap-foot-i is-dim">Library scanned 9 min ago</span>
      <span className="ap-foot-i is-dim">Platypus 0.9.0</span>
    </footer>
  );
}

export function Banners({ L, A }) {
  return (
    <>
      {A.demo === 'migrated' && !A.dismissed.includes('mig') && (
        <div className="ap-banner" role="status"><PackageOpen size={15} /><span><b>Platypus moved your data.</b> Plugins now live in ~/.local/share/platypus/plugins. 7 plugins and 26 games came across.</span><button type="button" className="btn btn-quiet" onClick={() => A.actions.go('plugins')}>Check plugins</button><button type="button" className="icon-btn" aria-label="Dismiss" onClick={() => A.actions.dismiss('mig')}><X size={14} /></button></div>
      )}
      {A.demo === 'offline' && (
        <div className="ap-banner is-warn" role="status"><WifiOff size={15} /><span><b>You’re offline.</b> Installed games still play. Downloads, friends and cloud saves wait until you’re back.</span></div>
      )}
    </>
  );
}

export function BackendDown({ A }) {
  if (A.demo !== 'down') return null;
  return (
    <div className="ap-down" role="alertdialog" aria-label="Core unreachable">
      <ServerCrash size={30} />
      <h2>Platypus can’t reach its core</h2>
      <p>The background process stopped. Your games and settings are safe on disk.</p>
      <div className="ap-row"><button type="button" className="btn btn-primary" onClick={() => A.actions.setDemo('normal')}><RefreshCw size={14} />Try again</button><button type="button" className="btn btn-quiet">Open the log</button></div>
    </div>
  );
}

// Lab-only: preview cross-cutting states.
export function DemoStates({ A }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="ap-demo">
      <span className="ap-anchor">
        <button type="button" className="ap-demo-btn" aria-expanded={open} onClick={() => setOpen(!open)}><Eye size={13} />States{A.demo !== 'normal' && <b>{DEMO_STATES.find((x) => x.id === A.demo).label}</b>}</button>
        <Popover open={open} onClose={() => setOpen(false)} label="Preview a state" className="ap-menu is-up" align="right">
          <span className="ap-menu-h">Design lab: preview a state</span>
          {DEMO_STATES.map((s) => <button key={s.id} type="button" className={A.demo === s.id ? 'is-on' : ''} onClick={() => { A.actions.setDemo(s.id); setOpen(false); }}>{s.label}{A.demo === s.id && <Check size={13} className="ap-menu-check" />}</button>)}
          <hr />
          <button type="button" onClick={() => { A.actions.setLang(A.lang === 'fr' ? 'en' : 'fr'); setOpen(false); }}>{A.lang === 'fr' ? 'Back to English' : 'French (longer strings)'}</button>
        </Popover>
      </span>
    </div>
  );
}

// Extra command palette entries: pages, redeem, collections, add a game.
export function paletteExtra(L, A, { onSettings } = {}) {
  const pages = ['home', 'library', 'store', 'friends', 'downloads', 'attention', 'activity', 'runners', 'plugins', 'storage', 'settings'];
  return {
    chips: { collection: 'Collection', page: 'Page' },
    root: () => [
      { id: 'x-redeem', group: 'Library', label: `${A.t.redeem}…`, run: () => { L.actions.closePalette(); A.actions.open('redeem'); } },
      { id: 'x-add', group: 'Library', label: `${A.t.addGame}…`, run: () => { L.actions.closePalette(); A.actions.open('add-game'); } },
      { id: 'x-col', group: 'Library', label: 'Show a collection…', next: { mode: 'collection' } },
      { id: 'x-newcol', group: 'Library', label: `${A.t.newCollection}…`, run: () => { L.actions.closePalette(); A.actions.open('collection'); } },
      { id: 'x-gs', group: L.game.title, label: 'Game settings…', run: () => { L.actions.closePalette(); onSettings ? onSettings() : A.actions.open('game-settings'); } },
      { id: 'x-stop', group: 'Running', label: L.running ? `Stop ${gameById(L, L.running.id)?.title}` : 'Stop the running game', hint: L.running ? undefined : 'nothing running', run: () => { L.actions.stop(); L.actions.closePalette(); } },
      ...storesWith('storefront').filter((s) => A.plugins.some((p) => p.id === s)).map((s) => ({ id: `x-shop-${s}`, group: 'Open', label: `Open the ${storeName(s)} store`, store: s, run: () => { L.actions.closePalette(); A.actions.go('store'); } })),
      ...pages.map((p) => ({ id: `x-p-${p}`, group: 'Open', label: A.t[p] || p, run: () => { L.actions.closePalette(); p === 'friends' ? A.actions.openSocial(true) : A.actions.go(p); } })),
    ],
    collection: () => [...BUILTIN_COLLECTIONS, ...A.collections].map((c) => ({ id: `c-${c.id}`, group: 'Collections', label: c.name, active: A.collection === c.id, hint: `${collectionIds(c, L.games, A.favs).length} games`, run: () => { A.actions.setCollection(c.id); A.actions.go('library'); L.actions.closePalette(); } })),
  };
}
