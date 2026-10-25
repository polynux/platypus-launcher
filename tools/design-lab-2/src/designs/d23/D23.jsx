import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import '../../core/app.css';
import '../d22/d22.css';
import './d23.css';
import { Search, Home, LibraryBig, Store, Users, ArrowDownToLine, CircleAlert, Activity, Cpu, Puzzle, HardDriveDownload, Settings, X, Minus, ChevronDown, ChevronRight, Ticket, Plus } from 'lucide-react';
import { useLauncher, isInstalled, artOf, ago, useTick, useArtTone, vivid, toneAt } from '../../core/launcher.js';
import { Palette, ConflictDialog, Toast, Img } from '../../core/kit.jsx';
import { useApp, tintVars, FRIENDS } from '../../core/app.js';
import { OwnSwitch, StoreTabs, CollectionsList, FilterButton, SortMenu, ViewSwitch, ActiveChips, SourceWarnings, LibraryEmpty, LibraryGrid, LibraryTable, LibraryBlocks, Skeleton, GameBadges } from '../../core/appLibrary.jsx';
import { HomePage, DownloadsPage, AttentionPage, ActivityPage, RunnersPage, PluginsPage, StoragePage, AppSettingsPage, StorefrontPage, FriendsList, MeCard, FamilyPanel, ChatView, AppDialogs, StatusFooter, Banners, BackendDown, DemoStates, paletteExtra, Avatar } from '../../core/appPages.jsx';
import { GameStage } from '../d22/D22.jsx';

// Every place on one rail, like an editor's activity bar. Library tools live in
// the column next to it; friends dock on the right and chats float.
const TOP = [
  { id: 'home', icon: Home },
  { id: 'library', icon: LibraryBig },
  { id: 'store', icon: Store },
  { id: 'downloads', icon: ArrowDownToLine },
  { id: 'attention', icon: CircleAlert },
  { id: 'activity', icon: Activity },
];
const BOTTOM = [
  { id: 'runners', icon: Cpu },
  { id: 'plugins', icon: Puzzle },
  { id: 'storage', icon: HardDriveDownload },
  { id: 'settings', icon: Settings },
];

function Section({ title, count, children, initial = true, action }) {
  const [open, setOpen] = useState(initial);
  return (
    <section className={`col-sec ${open ? 'is-open' : ''}`}>
      <h2><button type="button" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}{title}{count != null && <span className="ap-n">{count}</span>}</button>{action}</h2>
      {open && children}
    </section>
  );
}

export default function D23() {
  const L = useLauncher({ initial: 'hades2' });
  const A = useApp(L, 'd23');
  useTick(1000);
  const stage = useRef();
  const [friends, setFriends] = useState(true);
  const [chats, setChats] = useState(['f1']);
  const [mini, setMini] = useState(['f1']);
  const g = L.game;
  const tone = useArtTone(artOf(g).hero);
  const vars = tintVars(tone, A.tint, A.theme, { vivid, toneAt });
  const C = A.actions;
  const t = A.t;
  const libPage = A.page === 'library' || A.page === 'game';
  const split = A.view === 'list';
  const dl = A.jobs.filter((j) => j.kind === 'download');
  const dlP = dl.length ? dl.reduce((s, j) => s + j.progress, 0) / dl.length : 0;
  useEffect(() => { stage.current?.scrollTo({ top: 0 }); }, [g.id, A.page]);
  const palWas = useRef(null);
  useEffect(() => { if (!libPage && palWas.current) C.openGame(g.id, A.page); }, [g.id]);
  useEffect(() => { palWas.current = L.palette; });
  const openChat = (fid) => { C.openChat(fid); setChats((c) => [fid, ...c.filter((x) => x !== fid)].slice(0, 3)); setMini((m) => m.filter((x) => x !== fid)); };
  const openGame = (id) => { if (A.page === 'library' && split) L.actions.select(id); else C.openGame(id, A.page === 'game' ? A.back : A.page); };
  const badge = { attention: A.attentionCount, plugins: A.pluginAttention };
  const RailBtn = ({ id, icon: I }) => {
    const on = A.page === id || (id === 'library' && A.page === 'game');
    return (
      <button type="button" className={`rail-btn ${on ? 'is-on' : ''}`} aria-label={t[id]} aria-current={on ? 'page' : undefined} onClick={() => C.go(id)} data-tip={t[id]}>
        <I size={19} />
        {id === 'downloads' && dl.length > 0 && <svg className="tb-ring" viewBox="0 0 36 36" aria-hidden><circle cx="18" cy="18" r="16" pathLength="100" style={{ strokeDasharray: `${dlP * 100} 100` }} /></svg>}
        {badge[id] > 0 && <span className={`ap-badge rail-badge ${id === 'plugins' ? 'is-warn' : ''}`}>{badge[id]}</span>}
      </button>
    );
  };

  return (
    <div className={`k ap d22 d23 is-${A.theme}`} style={vars} lang={A.lang}>
      <div className="frame">
        <nav className="rail" aria-label="Places">
          <span className="tb-mark rail-mark" aria-hidden>P</span>
          {TOP.map((r) => <RailBtn key={r.id} {...r} />)}
          <button type="button" className={`rail-btn ${friends ? 'is-on' : ''}`} aria-label={`${t.friends}${A.unread ? `, ${A.unread} unread` : ''}`} aria-pressed={friends} data-tip={t.friends} onClick={() => setFriends(!friends)}><Users size={19} />{A.unread > 0 && <span className="ap-badge rail-badge">{A.unread}</span>}</button>
          <span className="rail-sp" />
          {BOTTOM.map((r) => <RailBtn key={r.id} {...r} />)}
        </nav>

        {libPage && (
          <aside className="col" aria-label="Library">
            <div className="col-head">
              <button type="button" className="col-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>{t.search}</span><kbd>Ctrl K</kbd></button>
              <ViewSwitch A={A} />
            </div>
            <div className="col-scroll">
              <div className="col-own"><OwnSwitch L={L} A={A} /></div>
              <Section title="Stores" initial={false} count={L.stores.length ? `${L.stores.length} picked` : 'All'}><StoreTabs L={L} A={A} className="col-stores" /></Section>
              <Section title={t.collections} action={<button type="button" className="icon-btn col-add" aria-label={t.newCollection} onClick={() => C.open('collection')}><Plus size={14} /></button>}><CollectionsList L={L} A={A} /></Section>
              {split && (
                <section className="col-sec is-open is-games">
                  <div className="col-tools">
                    <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
                    <FilterButton L={L} A={A} />
                  </div>
                  <div className="col-sort"><SortMenu L={L} /><span className="ap-n">{L.visible.length} games</span></div>
                  <SourceWarnings L={L} A={A} compact />
                  {A.demo === 'loading' ? <Skeleton rows={8} /> : (
                    <ul className="list-body">
                      {L.visible.map((x) => (
                        <li key={x.id}>
                          <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => { L.actions.select(x.id); if (A.page !== 'library') C.go('library'); }}>
                            <Img src={artOf(x).header} className="row-art" />
                            <span className="row-text"><span className="row-title">{x.title}</span><span className="row-sub">{isInstalled(x) ? ago(x.lastAgo) : 'Not installed'}</span></span>
                            <GameBadges L={L} A={A} g={x} />
                          </button>
                        </li>
                      ))}
                      {L.visible.length === 0 && <li><LibraryEmpty L={L} A={A} /></li>}
                    </ul>
                  )}
                </section>
              )}
              {!split && <Section title={t.filters}><div className="col-filters"><FilterButton L={L} A={A} /><SortMenu L={L} /></div></Section>}
            </div>
          </aside>
        )}

        <div className="main">
          <Banners L={L} A={A} />
          <main className="stage" ref={stage}>
            {libPage && split && (L.visible.length || A.demo === 'loading' ? <GameStage L={L} A={A} onAll={() => C.open('game-settings')} /> : <LibraryEmpty L={L} A={A} />)}
            {A.page === 'game' && !split && <GameStage L={L} A={A} onAll={() => C.open('game-settings')} back={C.goBack} />}
            {A.page === 'library' && !split && (
              <div className="ap-page lib-full">
                <div className="lib-bar"><label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter the library" /></label></div>
                <SourceWarnings L={L} A={A} />
                <ActiveChips L={L} A={A} />
                {A.demo === 'loading' ? <Skeleton rows={12} kind="grid" /> : L.visible.length === 0 ? <LibraryEmpty L={L} A={A} /> : (
                  <>
                    {!A.collection && !A.activeFilters && !L.query && A.view === 'grid' && <LibraryBlocks L={L} A={A} onOpen={openGame} />}
                    {A.view === 'grid' ? <LibraryGrid L={L} A={A} onOpen={openGame} onSettings={() => C.open('game-settings')} /> : <LibraryTable L={L} A={A} onOpen={openGame} onSettings={() => C.open('game-settings')} />}
                  </>
                )}
              </div>
            )}
            {A.page === 'home' && <HomePage L={L} A={A} onOpen={(id) => C.openGame(id, 'home')} />}
            {A.page === 'store' && <StorefrontPage L={L} A={A} onOpen={(id) => C.openGame(id, 'store')} />}
            {A.page === 'downloads' && <DownloadsPage L={L} A={A} onOpen={(id) => C.openGame(id, 'downloads')} />}
            {A.page === 'attention' && <AttentionPage L={L} A={A} onOpen={(id) => C.openGame(id, 'attention')} />}
            {A.page === 'activity' && <ActivityPage L={L} A={A} onOpen={(id) => C.openGame(id, 'activity')} />}
            {A.page === 'runners' && <RunnersPage L={L} A={A} onOpen={(id) => C.openGame(id, 'runners')} />}
            {A.page === 'plugins' && <PluginsPage L={L} A={A} />}
            {A.page === 'storage' && <StoragePage L={L} A={A} onOpen={(id) => C.openGame(id, 'storage')} />}
            {A.page === 'settings' && <AppSettingsPage L={L} A={A} />}
          </main>
        </div>

        {friends && (
          <aside className="dock" aria-label={t.friends}>
            <MeCard L={L} A={A} />
            <div className="dock-scroll">
              <FriendsList L={L} A={A} onChat={openChat} />
              <h3 className="ap-fgroup">Family<span /></h3>
              <FamilyPanel L={L} A={A} />
            </div>
            <div className="dock-foot">
              <button type="button" className="btn btn-quiet" onClick={() => C.open('redeem')}><Ticket size={14} />{t.redeem}</button>
              <button type="button" className="btn btn-quiet" onClick={() => C.open('add-game')}><Plus size={14} />{t.addGame}</button>
            </div>
          </aside>
        )}
      </div>

      {/* chats float over the bottom edge, one window per conversation */}
      <div className="chats" data-keys-local>
        {chats.map((fid) => {
          const f = FRIENDS.find((x) => x.id === fid);
          const c = A.chats[fid];
          const isMini = mini.includes(fid);
          return (
            <section key={fid} className={`chatwin ${isMini ? 'is-mini' : ''}`} aria-label={`Chat with ${f.name}`}>
              <header>
                <button type="button" className="chatwin-title" onClick={() => setMini((m) => (m.includes(fid) ? m.filter((x) => x !== fid) : [...m, fid]))} aria-expanded={!isMini}><Avatar f={f} size={22} /><span>{f.name}</span>{c?.unread > 0 && <span className="ap-badge">{c.unread}</span>}</button>
                <button type="button" className="icon-btn" aria-label={isMini ? 'Open' : 'Minimize'} onClick={() => setMini((m) => (m.includes(fid) ? m.filter((x) => x !== fid) : [...m, fid]))}><Minus size={14} /></button>
                <button type="button" className="icon-btn" aria-label="Close chat" onClick={() => setChats((cs) => cs.filter((x) => x !== fid))}><X size={14} /></button>
              </header>
              {!isMini && <ChatView L={L} A={A} fid={fid} compact />}
            </section>
          );
        })}
      </div>

      <StatusFooter L={L} A={A} />
      <Palette L={L} extra={paletteExtra(L, A)} />
      <ConflictDialog L={L} />
      <AppDialogs L={L} A={A} />
      <BackendDown A={A} />
      <DemoStates A={A} />
      <Toast L={L} />
    </div>
  );
}
