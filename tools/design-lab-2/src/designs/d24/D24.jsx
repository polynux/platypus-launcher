import React, { useEffect, useRef, useState } from 'react';
import '@fontsource-variable/albert-sans';
import '@fontsource-variable/geist-mono';
import '../../core/app.css';
import '../d22/d22.css';
import './d24.css';
import { Search, Home, Store, Users, ArrowDownToLine, CircleAlert, Activity, Cpu, Puzzle, HardDriveDownload, Settings, ChevronDown, ChevronRight, ChevronLeft, LibraryBig, HardDrive, Ticket, Plus } from 'lucide-react';
import { useLauncher, isInstalled, artOf, useTick, useArtTone, vivid, toneAt } from '../../core/launcher.js';
import { Palette, ConflictDialog, Toast, StoreGlyph } from '../../core/kit.jsx';
import { useApp, tintVars } from '../../core/app.js';
import { StoreTabs, CollectionsList, FilterButton, SortMenu, ViewSwitch, ActiveChips, SourceWarnings, LibraryEmpty, LibraryGrid, LibraryTable, LibraryBlocks, Skeleton, collectionName, storeName } from '../../core/appLibrary.jsx';
import { HomePage, DownloadsPage, AttentionPage, ActivityPage, RunnersPage, PluginsPage, StoragePage, AppSettingsPage, StorefrontPage, SocialPanel, GameSettingsPanel, MeCard, AppDialogs, StatusFooter, Banners, BackendDown, DemoStates, paletteExtra, PageHead } from '../../core/appPages.jsx';
import { GameStage } from '../d22/D22.jsx';

// One labelled sidebar holds every place, the library's stores and collections.
// Games open as full pages; friends get a page of their own; game settings too.
export default function D24() {
  const L = useLauncher({ initial: 'witcher3' });
  const A = useApp(L, 'd24', { page: 'home', view: 'grid' });
  useTick(1000);
  const stage = useRef();
  const [storesOpen, setStoresOpen] = useState(false);
  const g = L.game;
  const tone = useArtTone(artOf(g).hero);
  const vars = tintVars(tone, A.tint, A.theme, { vivid, toneAt });
  const C = A.actions;
  const t = A.t;
  const view = A.view === 'list' ? 'grid' : A.view;
  const dl = A.jobs.filter((j) => j.kind === 'download');
  const dlP = dl.length ? dl.reduce((s, j) => s + j.progress, 0) / dl.length : 0;
  useEffect(() => { stage.current?.scrollTo({ top: 0 }); }, [g.id, A.page]);
  const palWas = useRef(null);
  useEffect(() => { if (A.page !== 'game' && A.page !== 'gsettings' && palWas.current) C.openGame(g.id, A.page); }, [g.id]);
  useEffect(() => { palWas.current = L.palette; });
  useEffect(() => { if (A.dialog?.kind === 'game-settings') { C.close(); setGsCat(A.dialog.cat || 'compat'); C.go('gsettings', { back: A.page }); } }, [A.dialog]);
  const [gsCat, setGsCat] = useState('compat');
  // Friends are a page here: anything that asks for the friends panel lands on it.
  useEffect(() => { if (A.social.open && A.page !== 'friends') { C.openSocial(false); C.go('friends'); } }, [A.social.open]);
  const openSettings = (cat = 'compat') => { setGsCat(cat); C.go('gsettings', { back: A.page }); };
  const openGame = (id) => C.openGame(id, A.page === 'game' ? A.back : A.page);
  const lib = A.page === 'library';
  const inst = L.games.filter(isInstalled).length;

  const Item = ({ id, icon: I, label, on, onClick, extra }) => (
    <button type="button" className={`sb-item ${on ? 'is-on' : ''}`} aria-current={on ? 'page' : undefined} onClick={onClick || (() => C.go(id))}><I size={16} /><span className="sb-label">{label}</span>{extra}</button>
  );
  const libTitle = A.collection ? collectionName(A, A.collection) : A.sideloaded ? t.sideloaded : L.stores.length ? L.stores.map(storeName).join(' + ') : A.own === 'installed' ? t.installed : t.allGames;

  return (
    <div className={`k ap d22 d24 is-${A.theme}`} style={vars} lang={A.lang}>
      <div className="frame">
        <nav className="sb" aria-label="Places">
          <div className="sb-top">
            <div className="tb-brand"><span className="tb-mark" aria-hidden>P</span>Platypus</div>
            <button type="button" className="sb-cmd" onClick={() => L.actions.openPalette()}><Search size={14} /><span>{t.search.split(" ")[0]}</span><kbd>Ctrl K</kbd></button>
          </div>
          <div className="sb-scroll">
            <Item id="home" icon={Home} label={t.home} on={A.page === 'home'} />
            <Item id="store" icon={Store} label={t.store} on={A.page === 'store'} />
            <Item id="friends" icon={Users} label={t.friends} on={A.page === 'friends'} extra={A.unread > 0 && <span className="ap-badge">{A.unread}</span>} />

            <h2 className="sb-h">{t.library}</h2>
            <Item icon={LibraryBig} label={t.allGames} on={lib && A.own === 'all' && !A.collection && !L.stores.length && !A.sideloaded} onClick={() => { C.resetAll(); C.go('library'); }} extra={<span className="ap-n">{L.games.length}</span>} />
            <Item icon={HardDrive} label={t.installed} on={lib && A.own === 'installed' && !A.collection} onClick={() => { C.setCollection(null); C.setOwn('installed'); C.go('library'); }} extra={<span className="ap-n">{inst}</span>} />
            <button type="button" className="sb-item sb-fold" aria-expanded={storesOpen} onClick={() => setStoresOpen(!storesOpen)}>
              {storesOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}<span className="sb-label">Stores</span>
              {!storesOpen && <span className="sb-glyphs">{(L.stores.length ? L.stores : ['steam', 'gog', 'epic']).map((s) => <StoreGlyph key={s} store={s} size={11} />)}{!L.stores.length && <span className="ap-n">+3</span>}</span>}
            </button>
            {storesOpen && <div className="sb-stores" onClick={() => C.go('library')}><StoreTabs L={L} A={A} /></div>}
            <h2 className="sb-h">{t.collections}<button type="button" className="icon-btn" aria-label={t.newCollection} onClick={() => C.open('collection')}><Plus size={13} /></button></h2>
            <div className="sb-cols" onClick={(e) => { if (e.target.closest('.ap-col-pick')) C.go('library'); }}><CollectionsList L={L} A={A} /></div>

            <h2 className="sb-h">Keep up</h2>
            <Item id="downloads" icon={ArrowDownToLine} label={t.downloads} on={A.page === 'downloads'} extra={dl.length > 0 && <span className="sb-prog" style={{ '--p': dlP }} aria-label={`${Math.round(dlP * 100)}%`} />} />
            <Item id="attention" icon={CircleAlert} label={t.attention} on={A.page === 'attention'} extra={A.attentionCount > 0 && <span className="ap-badge">{A.attentionCount}</span>} />
            <Item id="activity" icon={Activity} label={t.activity} on={A.page === 'activity'} />
          </div>
          <div className="sb-bottom">
            <div className="sb-tools" role="group" aria-label="System">
              {[['runners', Cpu], ['plugins', Puzzle], ['storage', HardDriveDownload], ['settings', Settings]].map(([id, I]) => (
                <button key={id} type="button" className={`sb-tool ${A.page === id ? 'is-on' : ''}`} aria-label={t[id]} title={t[id]} onClick={() => C.go(id)}><I size={16} />{id === 'plugins' && A.pluginAttention > 0 && <span className="tb-dot" aria-hidden />}</button>
              ))}
            </div>
            <MeCard L={L} A={A} />
          </div>
        </nav>

        <div className="main">
          <Banners L={L} A={A} />
          <main className="stage" ref={stage}>
            {A.page === 'library' && (
              <div className="ap-page lib-full">
                <PageHead title={libTitle} sub={`${L.visible.length} games`}>
                  <button type="button" className="btn btn-quiet" onClick={() => C.open('redeem')}><Ticket size={14} />{t.redeem}</button>
                  <button type="button" className="btn btn-quiet" onClick={() => C.open('add-game')}><Plus size={14} />{t.addGame}</button>
                </PageHead>
                <div className="lib-bar">
                  <label className="list-search"><Search size={14} /><input data-search value={L.query} onChange={(e) => L.actions.setQuery(e.target.value)} placeholder="Filter the library" /></label>
                  <FilterButton L={L} A={A} />
                  <span className="lib-sp" />
                  <SortMenu L={L} />
                  <ViewSwitch A={{ ...A, view }} withList={false} />
                </div>
                <SourceWarnings L={L} A={A} />
                <ActiveChips L={L} A={A} />
                {A.demo === 'loading' ? <Skeleton rows={12} kind="grid" /> : L.visible.length === 0 ? <LibraryEmpty L={L} A={A} /> : (
                  <>
                    {!A.collection && !A.activeFilters && !L.query && view === 'grid' && A.own === 'all' && !L.stores.length && <LibraryBlocks L={L} A={A} onOpen={openGame} />}
                    {view === 'grid' ? <LibraryGrid L={L} A={A} onOpen={openGame} onSettings={() => openSettings()} /> : <LibraryTable L={L} A={A} onOpen={openGame} onSettings={() => openSettings()} />}
                  </>
                )}
              </div>
            )}
            {A.page === 'game' && <GameStage L={L} A={A} onAll={() => openSettings()} back={C.goBack} />}
            {A.page === 'gsettings' && (
              <div className="gs-page">
                <header className="gs-head">
                  <button type="button" className="btn btn-quiet" onClick={() => C.go('game', { back: 'library' })}><ChevronLeft size={15} />{g.title}</button>
                  <h1>Settings</h1>
                </header>
                <GameSettingsPanel L={L} A={A} initial={gsCat} layout="split" />
              </div>
            )}
            {A.page === 'friends' && <div className="friends-page"><SocialPanel L={L} A={A} layout="split" /></div>}
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
      </div>

      <StatusFooter L={L} A={A} />
      <Palette L={L} extra={paletteExtra(L, A, { onSettings: () => openSettings() })} />
      <ConflictDialog L={L} />
      <AppDialogs L={L} A={A} gameSettings={false} />
      <BackendDown A={A} />
      <DemoStates A={A} />
      <Toast L={L} />
    </div>
  );
}
