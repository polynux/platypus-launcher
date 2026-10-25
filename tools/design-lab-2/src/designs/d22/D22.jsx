import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import '../../core/app.css';
import './d22.css';
import { Search, AlertTriangle, Cloud, CloudOff, HardDrive, RefreshCw, SlidersHorizontal, X, ChevronDown, ChevronLeft, ArrowDownToLine, CircleAlert, Users, Menu, Activity, Cpu, Puzzle, HardDriveDownload, Settings, Ticket, Plus } from 'lucide-react';
import { useLauncher, runnerOf, activeCopy, isInstalled, artOf, ago, useTick, useArtTone, vivid, toneAt } from '../../core/launcher.js';
import { SYNC_LABEL, Palette, ConflictDialog, Toast, Img, GameLogo, Tabs, Select } from '../../core/kit.jsx';
import { StoreAbout, Gallery, AchSummary, AchList, GameFacts, achievementsOf, pageOf } from '../../core/page.jsx';
import { useApp, tintVars } from '../../core/app.js';
import { OwnSwitch, StoreTabs, CollectionsMenu, FilterButton, SortMenu, ViewSwitch, ActiveChips, SourceWarnings, LibraryEmpty, LibraryGrid, LibraryTable, LibraryBlocks, MainAction, ActionNote, CopySwitch, LaunchNotes, YouPanel, SessionsPanel, SavesPanel, CompatPanel, gameTabs, Skeleton, GameBadges, Popover } from '../../core/appLibrary.jsx';
import { HomePage, DownloadsPage, AttentionPage, ActivityPage, RunnersPage, PluginsPage, StoragePage, AppSettingsPage, StorefrontPage, SocialPanel, GameSettingsPanel, AppDialogs, StatusFooter, Banners, BackendDown, DemoStates, paletteExtra, Avatar } from '../../core/appPages.jsx';
import { QuickSettings } from '../d21/D21.jsx';

const SYNC_ICON = { synced: Cloud, syncing: RefreshCw, conflict: AlertTriangle, local: HardDrive, off: CloudOff };

export function SettingsDrawer({ L, A, cat, onClose }) {
  const ref = useRef();
  useEffect(() => {
    ref.current?.querySelector('.acc-h')?.focus();
    const key = (e) => { if (e.key === 'Escape' && !e.target.closest('.sel') && !e.target.closest('.ap-pop')) onClose(); };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  }, []);
  return (
    <div className="scrim d22-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <aside className="drawer" ref={ref} role="dialog" aria-modal="true" aria-labelledby="drawer-title">
        <header className="drawer-head">
          <div><h2 id="drawer-title">All settings</h2><span>{L.game.title}</span></div>
          <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}><X size={16} /></button>
        </header>
        <div className="drawer-body"><GameSettingsPanel L={L} A={A} initial={cat} /></div>
      </aside>
    </div>
  );
}

// The game page: hero, action strip, tabs. Same as /21 plus the new pieces.
export function GameStage({ L, A, onAll, back }) {
  const g = L.game;
  const [tab, setTab] = useState('store');
  const [quick, setQuick] = useState(false);
  const a = artOf(g);
  const SyncIcon = SYNC_ICON[g.sync];
  const ach = achievementsOf(g);
  const tabs = gameTabs(L, A, g);
  const curTab = tabs.some((t) => t.id === tab) ? tab : 'store';
  if (A.demo === 'loading') return <div className="stage-load"><Skeleton rows={1} kind="cards" /><Skeleton rows={5} /></div>;
  return (
    <>
      <div className="hero">
        <Img key={a.hero} src={a.hero} className="hero-img" />
        <GameLogo g={g} className="hero-logo" />
        {back && <button type="button" className="hero-back" onClick={back}><ChevronLeft size={15} />{A.t.library}</button>}
      </div>
      <div className="strip">
        <div className="strip-play">
          <MainAction L={L} A={A} g={g} />
          <div className="qs-anchor">
            <button type="button" className={`tune ${quick ? 'is-on' : ''}`} aria-expanded={quick} aria-haspopup="dialog" onClick={() => setQuick(!quick)}><SlidersHorizontal size={16} />{A.t.tune}<ChevronDown size={14} /></button>
            {quick && <QuickSettings L={L} onClose={() => setQuick(false)} onAll={() => { setQuick(false); onAll(); }} />}
          </div>
        </div>
        <CopySwitch L={L} A={A} g={g} />
        {g.sync === 'conflict' ? (
          <button type="button" className="sync is-conflict" onClick={() => L.actions.openConflict(g.id)}><AlertTriangle size={14} />Save conflict, resolve</button>
        ) : <span className="sync"><SyncIcon size={14} className={g.sync === 'syncing' ? 'spin' : ''} />{SYNC_LABEL[g.sync]}</span>}
        <ActionNote L={L} A={A} g={g} />
      </div>
      <LaunchNotes L={L} A={A} g={g} />
      <div className="readout">
        <Tabs label="Game page" tabs={tabs} value={curTab} onChange={setTab} />
        <div className="readout-body" role="tabpanel" key={`${g.id}-${curTab}`}>
          {curTab === 'store' && (
            <div className="storepg">
              <div className="storepg-main"><StoreAbout g={g} clamp={760} /></div>
              <aside className="storepg-side">
                {pageOf(g)?.shots.length > 0 && <section className="side-sec"><h2>Pictures<small>{pageOf(g).shots.length} screenshots</small></h2><Gallery g={g} variant="sheet" max={7} /></section>}
                <section className="side-sec"><h2>You</h2><YouPanel L={L} A={A} g={g} /></section>
                {ach && <section className="side-sec"><h2>Achievements</h2><AchSummary g={g} onOpen={() => setTab('achievements')} /></section>}
                <section className="side-sec"><h2>Details</h2><GameFacts g={g} /></section>
              </aside>
            </div>
          )}
          {curTab === 'achievements' && <div className="achs"><AchList g={g} /></div>}
          {curTab === 'sessions' && <div className="tabpad"><SessionsPanel L={L} g={g} /></div>}
          {curTab === 'saves' && <div className="tabpad"><SavesPanel L={L} A={A} g={g} /></div>}
          {curTab === 'compat' && <div className="tabpad"><CompatPanel L={L} A={A} g={g} /></div>}
        </div>
      </div>
    </>
  );
}

// Places that aren't the library live behind the right-hand icons and one menu.
const MANAGE = [
  { id: 'activity', icon: Activity },
  { id: 'runners', icon: Cpu },
  { id: 'plugins', icon: Puzzle },
  { id: 'storage', icon: HardDriveDownload },
  { id: 'settings', icon: Settings },
];

export default function D22() {
  const L = useLauncher({ initial: 'fallout3' });
  const A = useApp(L, 'd22');
  useTick(1000);
  const stage = useRef();
  const [drawer, setDrawer] = useState(null);
  const [menu, setMenu] = useState(false);
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
  // Picking a game in the palette from another page opens its page.
  const palWas = useRef(null);
  useEffect(() => { if (!libPage && palWas.current) C.openGame(g.id, A.page); }, [g.id]);
  useEffect(() => { palWas.current = L.palette; });
  // "Choose a version" and friends open the drawer here rather than a dialog.
  useEffect(() => { if (A.dialog?.kind === 'game-settings') { setDrawer(A.dialog.cat || 'compat'); C.close(); } }, [A.dialog]);
  useEffect(() => {
    const key = (e) => { if (e.key === 'Escape' && A.social.open && !e.target.closest('.ap-pop, .sel, [role="dialog"]')) C.openSocial(false); };
    window.addEventListener('keydown', key);
    return () => window.removeEventListener('keydown', key);
  });
  const openGame = (id) => { if (A.page === 'library' && split) L.actions.select(id); else C.openGame(id, A.page === 'game' ? A.back : A.page); };
  const nav = [
    { id: 'home', label: t.home },
    { id: 'library', label: t.library, on: libPage },
    { id: 'store', label: t.store },
  ];
  const managing = MANAGE.some((m) => m.id === A.page);

  return (
    <div className={`k ap d22 is-${A.theme}`} style={vars} lang={A.lang}>
      <header className="tb">
        <div className="tb-left">
          <div className="tb-brand"><span className="tb-mark" aria-hidden>P</span>Platypus</div>
          <nav className="tb-nav" aria-label="Main">
            {nav.map((n) => <button key={n.id} type="button" className={n.on || A.page === n.id ? 'is-on' : ''} aria-current={n.on || A.page === n.id ? 'page' : undefined} onClick={() => C.go(n.id)}>{n.label}</button>)}
          </nav>
        </div>
        <button type="button" className="tb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>{t.search}</span><kbd>Ctrl K</kbd></button>
        <div className="tb-right">
          <button type="button" className={`tb-ico ${A.page === 'downloads' ? 'is-on' : ''}`} aria-label={`${t.downloads}${dl.length ? `, ${dl.length} active` : ''}`} title={t.downloads} onClick={() => C.go('downloads')}>
            <ArrowDownToLine size={16} />{dl.length > 0 && <svg className="tb-ring" viewBox="0 0 36 36" aria-hidden><circle cx="18" cy="18" r="16" pathLength="100" style={{ strokeDasharray: `${dlP * 100} 100` }} /></svg>}
          </button>
          <button type="button" className={`tb-ico ${A.page === 'attention' ? 'is-on' : ''}`} aria-label={`${t.attention}, ${A.attentionCount}`} title={t.attention} onClick={() => C.go('attention')}>
            <CircleAlert size={16} />{A.attentionCount > 0 && <span className="ap-badge tb-badge">{A.attentionCount}</span>}
          </button>
          <button type="button" className={`tb-friends ${A.social.open ? 'is-on' : ''}`} aria-expanded={A.social.open} aria-label={`${t.friends}${A.unread ? `, ${A.unread} unread` : ''}`} onClick={() => C.openSocial(!A.social.open)}>
            <Avatar me={A.social.me} size={24} />{A.unread > 0 && <span className="ap-badge tb-badge">{A.unread}</span>}
          </button>
          <span className="ap-anchor">
            <button type="button" className={`tb-ico ${managing ? 'is-on' : ''}`} aria-label="More places" aria-expanded={menu} onClick={() => setMenu(!menu)}><Menu size={17} />{A.pluginAttention > 0 && <span className="tb-dot" aria-hidden />}</button>
            <Popover open={menu} onClose={() => setMenu(false)} label="More places" className="ap-menu" align="right">
              {MANAGE.map((m) => { const I = m.icon; return <button key={m.id} type="button" className={A.page === m.id ? 'is-on' : ''} onClick={() => { C.go(m.id); setMenu(false); }}><I size={15} />{t[m.id]}{m.id === 'plugins' && A.pluginAttention > 0 && <span className="ap-badge tb-menu-badge">{A.pluginAttention}</span>}</button>; })}
              <hr />
              <button type="button" onClick={() => { C.open('redeem'); setMenu(false); }}><Ticket size={15} />{t.redeem}</button>
              <button type="button" onClick={() => { C.open('add-game'); setMenu(false); }}><Plus size={15} />{t.addGame}</button>
            </Popover>
          </span>
        </div>
      </header>

      <Banners L={L} A={A} />

      {libPage && (
        <nav className="sources" aria-label="Library">
          <OwnSwitch L={L} A={A} />
          <span className="sources-sep" aria-hidden />
          <StoreTabs L={L} A={A} className="sources-stores" />
          <div className="sources-tools">
            <CollectionsMenu L={L} A={A} />
            <FilterButton L={L} A={A} align="right" />
            <ViewSwitch A={A} />
          </div>
        </nav>
      )}

      <div className={`body ${libPage && split ? 'is-split' : 'is-full'}`}>
        {libPage && split && (
          <aside className="list" aria-label="Games">
            <div className="list-tools">
              <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter" onKeyDown={(e) => { if (e.key === 'Escape') { L.actions.setQuery(''); e.currentTarget.blur(); } if (e.key === 'ArrowDown') { e.currentTarget.blur(); L.actions.move(1); } }} /></label>
              <SortMenu L={L} className="list-sort2" />
            </div>
            <SourceWarnings L={L} A={A} compact />
            <ActiveChips L={L} A={A} />
            {A.demo === 'loading' ? <Skeleton rows={9} /> : (
              <ul className="list-body">
                {L.visible.map((x) => (
                  <li key={x.id}>
                    <button type="button" data-game-row={x.id} aria-current={x.id === g.id} className={`row ${x.id === g.id ? 'is-sel' : ''} ${isInstalled(x) ? '' : 'is-absent'}`} onClick={() => L.actions.select(x.id)}>
                      <Img src={artOf(x).header} className="row-art" />
                      <span className="row-text"><span className="row-title">{x.title}</span><span className="row-sub">{isInstalled(x) ? ago(x.lastAgo) : 'Not installed'}</span></span>
                      <GameBadges L={L} A={A} g={x} />
                    </button>
                  </li>
                ))}
                {L.visible.length === 0 && <li><LibraryEmpty L={L} A={A} /></li>}
              </ul>
            )}
          </aside>
        )}

        <main className="stage" ref={stage}>
          {libPage && split && (L.visible.length || A.demo === 'loading' ? <GameStage L={L} A={A} onAll={() => setDrawer('compat')} /> : <LibraryEmpty L={L} A={A} />)}
          {A.page === 'game' && !split && <GameStage L={L} A={A} onAll={() => setDrawer('compat')} back={C.goBack} />}
          {A.page === 'library' && !split && (
            <div className="ap-page lib-full">
              <div className="lib-bar">
                <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter the library" /></label>
                <SortMenu L={L} />
              </div>
              <SourceWarnings L={L} A={A} />
              <ActiveChips L={L} A={A} />
              {A.demo === 'loading' ? <Skeleton rows={12} kind="grid" /> : L.visible.length === 0 ? <LibraryEmpty L={L} A={A} /> : (
                <>
                  {!A.collection && !A.activeFilters && !L.query && A.view === 'grid' && <LibraryBlocks L={L} A={A} onOpen={openGame} />}
                  {A.view === 'grid' ? <LibraryGrid L={L} A={A} onOpen={openGame} onSettings={() => setDrawer('compat')} /> : <LibraryTable L={L} A={A} onOpen={openGame} onSettings={() => setDrawer('compat')} />}
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

        {A.social.open && (
          <aside className="social" aria-label={t.friends} data-keys-local>
            <header className="social-head"><h2>{t.friends}</h2><button type="button" className="icon-btn" aria-label="Close" onClick={() => C.openSocial(false)}><X size={15} /></button></header>
            <SocialPanel L={L} A={A} layout="stack" />
          </aside>
        )}
      </div>

      <StatusFooter L={L} A={A} />

      {drawer && <SettingsDrawer L={L} A={A} cat={drawer} onClose={() => setDrawer(null)} />}
      <Palette L={L} extra={paletteExtra(L, A, { onSettings: () => setDrawer('compat') })} />
      <ConflictDialog L={L} />
      <AppDialogs L={L} A={A} gameSettings={false} />
      <BackendDown A={A} />
      <DemoStates A={A} />
      <Toast L={L} />
    </div>
  );
}
