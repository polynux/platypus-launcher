// App-level mock data and state for the full-app designs (/22 onward): pages,
// collections, filters, social, plugins, runners, storage, global settings.
// Everything here is illustrative; store data is an example, not a real account.
import { useCallback, useEffect, useMemo, useState } from 'react';
import { GAMES, RUNNERS } from './data.js';
import { isInstalled, activeCopy, TIER_ORDER } from './launcher.js';

const M = 60e3;
const H = 60 * M;
const D = 24 * H;
const NOW = Date.now();

// What each store's plugin can do. Features show only where a store supports them.
export const CAPS = {
  steam: ['cloud', 'achievements', 'friends', 'chat', 'families', 'redeem', 'storefront'],
  gog: ['cloud', 'achievements', 'friends', 'chat', 'redeem', 'storefront'],
  epic: ['cloud', 'achievements', 'friends', 'redeem', 'storefront'],
  amazon: ['storefront'],
  itch: ['storefront'],
  local: [],
};
export const can = (store, cap) => (CAPS[store] || []).includes(cap);
export const storesWith = (cap) => Object.keys(CAPS).filter((s) => can(s, cap));

export const PAGES = [
  { id: 'home', label: 'Home' },
  { id: 'library', label: 'Library' },
  { id: 'store', label: 'Store' },
  { id: 'friends', label: 'Friends' },
  { id: 'downloads', label: 'Downloads' },
  { id: 'attention', label: 'Needs attention' },
  { id: 'activity', label: 'Activity' },
  { id: 'runners', label: 'Runners' },
  { id: 'plugins', label: 'Plugins' },
  { id: 'storage', label: 'Storage' },
  { id: 'settings', label: 'Settings' },
];

// ---------- plugins ----------
export const PLUGINS = [
  { id: 'steam', name: 'Steam', version: '1.8.2', status: 'ready', account: 'polynux', games: 21, author: 'Platypus', caps: CAPS.steam },
  { id: 'gog', name: 'GOG', version: '1.4.0', status: 'ready', account: 'polynux', games: 9, author: 'Platypus', caps: CAPS.gog },
  { id: 'epic', name: 'Epic Games', version: '1.3.1', status: 'needs-login', account: null, games: 6, author: 'Platypus', caps: CAPS.epic, why: 'Your Epic session expired 2 days ago. Games stay playable offline.' },
  { id: 'amazon', name: 'Amazon Games', version: '0.9.4', status: 'ready', account: 'g.dorce', games: 1, author: 'Community', caps: CAPS.amazon },
  { id: 'itch', name: 'itch.io', version: '0.6.0', status: 'changed', account: 'polynux', games: 1, author: 'Community', caps: CAPS.itch, why: 'Updated to 0.6.0 and now asks to read your download keys.', perms: ['Read your library', 'Download games', 'Read download keys (new)'] },
  { id: 'humble', name: 'Humble Bundle', version: '0.2.1', status: 'unapproved', account: null, games: 0, author: 'Community', caps: [], why: 'Installed from a file. Approve it before it can run.', perms: ['Read your library', 'Download games', 'Open web pages'] },
  { id: 'ea', name: 'EA app', version: '0.1.3', status: 'error', account: 'polynux', games: 0, author: 'Community', caps: [], why: 'The plugin stopped: “origin.com returned 503”. Last good list 4 days ago.' },
];
export const PLUGIN_STATUS = {
  ready: { label: 'Ready', tone: 'ok' },
  'needs-login': { label: 'Log in needed', tone: 'warn' },
  unapproved: { label: 'Waiting for approval', tone: 'warn' },
  changed: { label: 'Permissions changed', tone: 'warn' },
  error: { label: 'Not working', tone: 'bad' },
};

// ---------- runners ----------
export const RUNNER_VERSIONS = [
  ...RUNNERS.map((r, i) => ({ ...r, installed: true, size: r.family === 'System' ? '—' : `${(1.1 + i * 0.07).toFixed(1)} GB`, added: NOW - (i + 1) * 11 * D, system: r.family === 'System' })),
  { id: 'ge-10-5', name: 'GE-Proton10-5', family: 'GE-Proton', wine: '10.0', note: 'Released 2 days ago', installed: false, size: '1.2 GB', isNew: true },
  { id: 'p8', name: 'Proton 8.0-5', family: 'Valve', wine: '8.0', note: 'Older Valve stable', installed: false, size: '1.0 GB' },
  { id: 'ge-9-27', name: 'GE-Proton9-27', family: 'GE-Proton', wine: '9.0', note: 'Last 9.x build', installed: false, size: '1.1 GB' },
];

// ---------- storage ----------
export const DRIVES = [
  { id: 'home', label: 'System drive', path: '/home/polynux/Games', device: 'NVMe, ext4', total: 1000, free: 214, isDefault: true },
  { id: 'games', label: 'Games SSD', path: '/mnt/games', device: 'SATA SSD, btrfs', total: 2000, free: 7.4 },
];
export const DRIVE_OF = { bg3: 'games', cyberpunk: 'games', witcher3: 'games', nightreign: 'games', kcd2: 'games', helldivers: 'games', gtav: 'games', metro: 'games', civ7: 'games' };
export const driveOf = (g) => (g.id === 'teardown' ? 'outside' : DRIVE_OF[g.id] || 'home');
export const LEFTOVERS = [
  { id: 'l1', label: 'Starfield, canceled install', size: 41.2, at: NOW - 6 * D },
  { id: 'l2', label: 'Subnautica, failed install', size: 3.9, at: NOW - 1 * D },
  { id: 'l3', label: 'Shader cache from removed games', size: 2.6, at: NOW - 30 * D },
];

// ---------- needs attention ----------
export const ISSUES = [
  { id: 'i-sub', gameId: 'subnautica', kind: 'install', title: 'Install failed', reason: 'The Games SSD ran out of space at 62%. Subnautica needs 8.1 GB more.', fix: 'Retry install', alt: 'Install on the system drive' },
  { id: 'i-gta', gameId: 'gtav', kind: 'runner', title: 'Runner missing', reason: 'GTA V is set to GE-Proton9-27, which was removed.', fix: 'Install GE-Proton9-27', alt: 'Use GE-Proton10-4' },
  { id: 'i-tear', gameId: 'teardown', kind: 'outside', title: 'Outside your game folders', reason: 'Teardown lives in ~/Downloads/teardown. Backups and storage totals skip it.', fix: 'Move to Games', alt: 'Add the folder' },
  { id: 'i-metro', gameId: 'metro', kind: 'prefix', title: 'Wine prefix damaged', reason: 'wineboot exited with code 3 on the last launch.', fix: 'Repair prefix', alt: 'Reset prefix' },
  { id: 'i-civ', gameId: 'civ7', kind: 'update', title: 'Update needed', reason: 'Online play needs update 1.2.4 (4.3 GB).', fix: 'Update now' },
  { id: 'i-bg3', gameId: 'bg3', kind: 'conflict', title: 'Save conflict', reason: 'Honour Mode changed on this PC and on steamdeck.', fix: 'Compare saves' },
];

// ---------- downloads history ----------
export const DL_HISTORY = [
  { id: 'h1', gameId: 'deltarune', kind: 'install', result: 'installed', at: NOW - 3 * H, size: '2.1 GB', took: '1 min' },
  { id: 'h2', gameId: 'balatro', kind: 'update', result: 'updated', at: NOW - 9 * H, size: '84 MB', took: '4 s' },
  { id: 'h3', gameId: 'subnautica', kind: 'install', result: 'failed', at: NOW - 1 * D, size: '8.1 GB', reason: 'Not enough space on Games SSD' },
  { id: 'h4', gameId: 'kcd2', kind: 'verify', result: 'repaired', at: NOW - 2 * D, size: '2 files', took: '6 min' },
  { id: 'h5', gameId: 'helldivers', kind: 'update', result: 'updated', at: NOW - 3 * D, size: '6.4 GB', took: '3 min' },
  { id: 'h6', gameId: 'hollow', kind: 'move', result: 'moved', at: NOW - 9 * D, size: '9.0 GB', took: '40 s' },
];

// ---------- activity ----------
export const ACTIVITY = [
  { id: 'a1', kind: 'session', gameId: 'fallout3', at: NOW - 40 * M, text: 'Played for 1 h 12' },
  { id: 'a2', kind: 'sync', gameId: 'hades2', at: NOW - 2 * H, text: 'Save uploaded to Epic cloud' },
  { id: 'a3', kind: 'install', gameId: 'deltarune', at: NOW - 3 * H, text: 'Installed from Steam, 2.1 GB' },
  { id: 'a4', kind: 'session', gameId: 'bg3', at: NOW - 3 * H, text: 'Played for 2 h 40' },
  { id: 'a5', kind: 'sync', gameId: 'bg3', at: NOW - 3 * H, text: 'Save conflict with steamdeck' },
  { id: 'a6', kind: 'update', gameId: 'balatro', at: NOW - 9 * H, text: 'Updated to 1.0.1o' },
  { id: 'a7', kind: 'achievement', gameId: 'hades2', at: NOW - 26 * H, text: 'Unlocked “Night and Darkness”' },
  { id: 'a8', kind: 'session', gameId: 'stardew', at: NOW - 30 * H, text: 'Played for 3 h 05' },
  { id: 'a9', kind: 'install', gameId: 'subnautica', at: NOW - 1 * D, text: 'Install failed: not enough space' },
  { id: 'a10', kind: 'update', gameId: 'helldivers', at: NOW - 3 * D, text: 'Updated to 1.003.200' },
  { id: 'a11', kind: 'session', gameId: 'witcher3', at: NOW - 4 * D, text: 'Played for 52 min' },
  { id: 'a12', kind: 'sync', gameId: 'cyberpunk', at: NOW - 5 * D, text: 'Saves downloaded from GOG cloud' },
];
export const ACTIVITY_KINDS = [
  { id: 'session', label: 'Sessions' },
  { id: 'install', label: 'Installs' },
  { id: 'update', label: 'Updates' },
  { id: 'sync', label: 'Cloud saves' },
  { id: 'achievement', label: 'Achievements' },
];

// ---------- collections ----------
// Smart collections store a rule; manual ones a list of ids. Ids that are not in
// the library (sold, refunded, plugin removed) still show, greyed.
export const BUILTIN_COLLECTIONS = [
  { id: 'fav', name: 'Favourites', kind: 'builtin', icon: 'star' },
  { id: 'installed', name: 'Installed', kind: 'builtin', icon: 'drive' },
  { id: 'recent', name: 'Recently played', kind: 'builtin', icon: 'clock' },
];
export const SEED_COLLECTIONS = [
  { id: 'coop', name: 'Co-op nights', kind: 'manual', ids: ['helldivers', 'drg', 'valheim', 'nightreign', 'portal2'], pinned: true },
  { id: 'short', name: 'Under an hour', kind: 'smart', rule: { genre: ['Roguelike', 'Roguelite', 'Deckbuilder', 'Platformer'] }, ruleText: 'Genre is a roguelike, deckbuilder or platformer', pinned: true },
  { id: 'deck', name: 'Steam Deck verified', kind: 'smart', rule: { tier: ['native', 'platinum'] }, ruleText: 'Runs natively or rated Platinum', pinned: false },
  { id: 'rpg', name: 'Long RPGs', kind: 'manual', ids: ['bg3', 'witcher3', 'kcd2', 'cyberpunk', 'starfield', 'fallout3'], pinned: false },
];
export const GHOSTS = { portal2: { title: 'Portal 2', why: 'Removed from your library on Steam' } };
export const DEFAULT_FAVS = ['bg3', 'factorio', 'hades2', 'stardew'];

export const OWN_STATUS = [
  { id: 'none', label: 'No status' },
  { id: 'backlog', label: 'Backlog' },
  { id: 'playing', label: 'Playing' },
  { id: 'finished', label: 'Finished' },
  { id: 'abandoned', label: 'Abandoned' },
];
const SEED_STATUS = { bg3: 'playing', fallout3: 'playing', hades2: 'playing', witcher3: 'finished', hollow: 'finished', stardew: 'finished', starfield: 'abandoned', outerwilds: 'backlog', subnautica: 'backlog', kcd2: 'backlog', sh2: 'backlog' };
const SEED_NOTES = { fallout3: 'Do Broken Steel before the ending. Mod list in ~/mods/fo3.txt.', bg3: 'Honour Mode run: Karlach, Shadowheart, Astarion, Gale.' };

// ---------- families / lending ----------
export const FAMILY = {
  store: 'steam',
  name: 'Dorce family',
  members: [
    { id: 'me', name: 'polynux', role: 'Adult', you: true },
    { id: 'cam', name: 'Camille', role: 'Adult' },
    { id: 'leo', name: 'Léo', role: 'Child' },
  ],
};
// Games lent to you: owner, and whether the owner is playing right now.
export const LENT = {
  outerwilds: { owner: 'Camille', store: 'steam', inUse: false },
  starfield: { owner: 'Camille', store: 'steam', inUse: true },
  subnautica: { owner: 'Léo', store: 'steam', inUse: false },
};

// ---------- friends and chat ----------
export const FRIENDS = [
  { id: 'f1', name: 'Mira', store: 'steam', status: 'ingame', gameId: 'helldivers', since: 34 * M, avatar: 1 },
  { id: 'f2', name: 'Kenji', store: 'steam', status: 'online', avatar: 2 },
  { id: 'f3', name: 'Camille', store: 'steam', status: 'ingame', gameId: 'starfield', since: 12 * M, avatar: 3, family: true },
  { id: 'f4', name: 'Sacha', store: 'gog', status: 'ingame', gameTitle: 'Disco Elysium', since: 2 * H, avatar: 4 },
  { id: 'f5', name: 'Ines', store: 'steam', status: 'away', avatar: 5 },
  { id: 'f6', name: 'Oskar', store: 'epic', status: 'online', avatar: 6 },
  { id: 'f7', name: 'Nadia', store: 'gog', status: 'offline', lastSeen: 3 * H, avatar: 7 },
  { id: 'f8', name: 'Theo', store: 'steam', status: 'offline', lastSeen: 2 * D, avatar: 8 },
  { id: 'f9', name: 'Ama', store: 'epic', status: 'offline', lastSeen: 6 * D, avatar: 9 },
  { id: 'f10', name: 'Léo', store: 'steam', status: 'offline', lastSeen: 20 * H, avatar: 10, family: true },
];
export const FRIEND_STATUS = { ingame: 'Playing', online: 'Online', away: 'Away', offline: 'Offline' };
export const SEED_CHATS = {
  f1: { unread: 2, msgs: [
    { id: 1, from: 'them', text: 'are you on tonight?', at: NOW - 50 * M },
    { id: 2, from: 'me', text: 'after dinner yes', at: NOW - 48 * M },
    { id: 3, from: 'them', text: 'cool, we’re doing level 10 bugs', at: NOW - 6 * M },
    { id: 4, from: 'them', text: 'bring the orbital laser pls', at: NOW - 5 * M },
  ], older: true },
  f4: { unread: 1, msgs: [
    { id: 1, from: 'them', text: 'finally started Disco Elysium. you were right', at: NOW - 2 * H },
  ] },
  f2: { unread: 0, msgs: [
    { id: 1, from: 'me', text: 'sending you the Factorio blueprint', at: NOW - 1 * D },
    { id: 2, from: 'me', text: '0eNrtvW2PHM…', at: NOW - 1 * D, failed: true },
  ] },
};
// Avatars: a hue per friend, initial on top (the lab has no real pictures).
export const avatarHue = (n) => (n * 47) % 360;

// ---------- global settings ----------
export const APP_SETTINGS_CATS = [
  { id: 'general', label: 'General' },
  { id: 'appearance', label: 'Appearance' },
  { id: 'folders', label: 'Game folders' },
  { id: 'saves', label: 'Save backups' },
  { id: 'data', label: 'Ratings and game info' },
  { id: 'social', label: 'Friends and chat' },
  { id: 'stores', label: 'Store clients' },
];

// ---------- lab-only state previews ----------
export const DEMO_STATES = [
  { id: 'normal', label: 'Normal' },
  { id: 'loading', label: 'Loading' },
  { id: 'offline', label: 'Offline' },
  { id: 'down', label: 'Core unreachable' },
  { id: 'empty', label: 'Empty library' },
  { id: 'migrated', label: 'After an update' },
];

// Short UI strings in English and French (French runs about 30% longer).
export const STRINGS = {
  en: {
    home: 'Home', library: 'Library', store: 'Store', friends: 'Friends', downloads: 'Downloads', attention: 'Needs attention', activity: 'Activity', runners: 'Runners', plugins: 'Plugins', storage: 'Storage', settings: 'Settings',
    play: 'Play', stop: 'Stop', install: 'Install', update: 'Update', retry: 'Retry install', tune: 'Tune', search: 'Search or run a command', allGames: 'All games', installed: 'Installed', collections: 'Collections', filters: 'Filters', sort: 'Sort', redeem: 'Redeem a code', addGame: 'Add a game', newCollection: 'New collection', sideloaded: 'Sideloaded',
  },
  fr: {
    home: 'Accueil', library: 'Bibliothèque', store: 'Boutique', friends: 'Amis', downloads: 'Téléchargements', attention: 'À corriger', activity: 'Activité', runners: 'Exécuteurs', plugins: 'Extensions', storage: 'Stockage', settings: 'Paramètres',
    play: 'Jouer', stop: 'Arrêter', install: 'Installer', update: 'Mettre à jour', retry: 'Relancer l’installation', tune: 'Réglages rapides', search: 'Rechercher ou lancer une commande', allGames: 'Tous les jeux', installed: 'Installés', collections: 'Collections', filters: 'Filtres', sort: 'Trier', redeem: 'Activer un code', addGame: 'Ajouter un jeu', newCollection: 'Nouvelle collection', sideloaded: 'Ajoutés à la main',
  },
};

const useStored = (key, init) => {
  const [v, setV] = useState(() => { try { return JSON.parse(localStorage.getItem(key)) ?? init; } catch { return init; } });
  useEffect(() => { localStorage.setItem(key, JSON.stringify(v)); }, [key, v]);
  return [v, setV];
};
export { useStored };

const genreOf = (g) => g.genre || '';
export function collectionIds(c, games, favs) {
  if (c.id === 'fav') return favs;
  if (c.id === 'installed') return games.filter(isInstalled).map((g) => g.id);
  if (c.id === 'recent') return [...games].filter((g) => g.lastAgo < 14 * D && g.hours > 0).sort((a, b) => a.lastAgo - b.lastAgo).slice(0, 8).map((g) => g.id);
  if (c.kind === 'manual') return c.ids;
  const r = c.rule || {};
  return games.filter((g) => (!r.genre || r.genre.some((x) => genreOf(g).includes(x))) && (!r.tier || r.tier.includes(g.tier))).map((g) => g.id);
}

// The app layer on top of useLauncher: one per design, keyed so designs keep their own prefs.
export function useApp(L, key, { page: page0 = 'library', view: view0 = 'list' } = {}) {
  const [page, setPage] = useState(page0);
  const [back, setBack] = useState(null);
  const [tint, setTint] = useStored(`${key}-tint`, 'accent'); // off | accent | whole
  const [theme, setTheme] = useStored(`${key}-theme`, 'dark'); // dark | light | black
  const [lang, setLang] = useStored(`${key}-lang`, 'en');
  const [view, setView] = useStored(`${key}-view`, view0); // list | grid | table
  const [own, setOwn] = useState('all'); // all | installed
  const [sideloaded, setSideloaded] = useState(false);
  const [collection, setCollection] = useState(null);
  const [tiers, setTiers] = useState([]);
  const [statuses, setStatuses] = useState([]);
  const [lentOnly, setLentOnly] = useState(false);
  const [collections, setCollections] = useState(SEED_COLLECTIONS);
  const [favs, setFavs] = useState(DEFAULT_FAVS);
  const [status, setStatusMap] = useState(SEED_STATUS);
  const [notes, setNotesMap] = useState(SEED_NOTES);
  const [plugins, setPlugins] = useState(PLUGINS);
  const [runners, setRunners] = useState(RUNNER_VERSIONS);
  const [issues, setIssues] = useState(ISSUES);
  const [leftovers, setLeftovers] = useState(LEFTOVERS);
  const [paused, setPaused] = useState([]);
  const [canceled, setCanceled] = useState([]);
  const [dismissed, setDismissed] = useState([]);
  const [demo, setDemo] = useState('normal');
  const [dialog, setDialog] = useState(null); // { kind, ... }
  const [social, setSocial] = useState({ me: 'online', showPlaying: true, alerts: true, stores: { steam: true, gog: true, epic: false }, chat: null, open: false });
  const [chats, setChats] = useState(SEED_CHATS);
  const [settings, setSettings] = useState({ cleanup: 'week', backups: true, keep: 5, ratings: true, anticheat: true, artLookup: true, infoLookup: true, tray: true, steamNotices: true, chatNotify: 'mentions', prefix: '~/.local/share/platypus/prefixes' });
  const t = STRINGS[lang] || STRINGS.en;

  const go = useCallback((p, opts = {}) => {
    setBack(opts.back ?? null);
    setPage(p);
    if (opts.game) L.actions.select(opts.game);
  }, [L.actions]);

  // Feed the library filters into the launcher so keyboard movement follows them.
  const colIds = useMemo(() => {
    if (!collection) return null;
    const c = [...BUILTIN_COLLECTIONS, ...collections].find((x) => x.id === collection);
    return c ? collectionIds(c, L.games, favs) : null;
  }, [collection, collections, favs, L.games]);
  const refineKey = [own, sideloaded, collection, colIds?.join(), tiers.join(), statuses.join(), lentOnly, demo].join('|');
  useEffect(() => {
    L.actions.setRefine({
      key: refineKey,
      fn: (g) =>
        demo !== 'empty' &&
        (own === 'all' || isInstalled(g)) &&
        (!sideloaded || g.stores.some((s) => s.store === 'local')) &&
        (!colIds || colIds.includes(g.id)) &&
        (!tiers.length || tiers.includes(g.tier)) &&
        (!statuses.length || statuses.includes(status[g.id] || 'none')) &&
        (!lentOnly || !!LENT[g.id]),
    });
  }, [refineKey, status]);

  const unread = Object.values(chats).reduce((n, c) => n + (c.unread || 0), 0);
  const pluginAttention = plugins.filter((p) => p.status !== 'ready').length;
  const activeFilters = tiers.length + statuses.length + (lentOnly ? 1 : 0);
  const say = L.actions.say;

  const actions = {
    go,
    goBack: () => { setPage(back || 'library'); setBack(null); },
    openGame: (id, from) => { L.actions.select(id); setBack(from || page); setPage('game'); },
    setTint, setTheme, setLang, setView, setOwn, setSideloaded, setDemo,
    setCollection: (id) => { setCollection(id); if (page !== 'library' && page !== 'game') setPage('library'); },
    toggleTier: (x) => setTiers((a) => (a.includes(x) ? a.filter((y) => y !== x) : [...a, x])),
    toggleStatus: (x) => setStatuses((a) => (a.includes(x) ? a.filter((y) => y !== x) : [...a, x])),
    setLentOnly,
    clearFilters: () => { setTiers([]); setStatuses([]); setLentOnly(false); },
    resetAll: () => { setTiers([]); setStatuses([]); setLentOnly(false); setCollection(null); setOwn('all'); setSideloaded(false); L.actions.resetFilters(); },
    toggleFav: (id) => setFavs((f) => (f.includes(id) ? f.filter((x) => x !== id) : [...f, id])),
    setStatus: (id, s) => setStatusMap((m) => ({ ...m, [id]: s })),
    setNote: (id, s) => setNotesMap((m) => ({ ...m, [id]: s })),
    // collections
    createCollection: (c) => { const id = `c${Date.now()}`; setCollections((cs) => [...cs, { id, pinned: false, ids: [], ...c }]); say(`Created “${c.name}”`); return id; },
    renameCollection: (id, name) => setCollections((cs) => cs.map((c) => (c.id === id ? { ...c, name } : c))),
    deleteCollection: (id) => { const c = collections.find((x) => x.id === id); setCollections((cs) => cs.filter((x) => x.id !== id)); if (collection === id) setCollection(null); say(`Deleted “${c?.name}”`); },
    pinCollection: (id) => setCollections((cs) => cs.map((c) => (c.id === id ? { ...c, pinned: !c.pinned } : c))),
    moveCollection: (id, d) => setCollections((cs) => { const i = cs.findIndex((c) => c.id === id); const j = i + d; if (j < 0 || j >= cs.length) return cs; const n = [...cs]; [n[i], n[j]] = [n[j], n[i]]; return n; }),
    toggleInCollection: (cid, gid) => setCollections((cs) => cs.map((c) => (c.id === cid && c.kind === 'manual' ? { ...c, ids: c.ids.includes(gid) ? c.ids.filter((x) => x !== gid) : [...c.ids, gid] } : c))),
    // plugins
    login: (id) => { setPlugins((ps) => ps.map((p) => (p.id === id ? { ...p, status: 'ready', account: 'polynux' } : p))); say('Logged in'); },
    logout: (id) => { setPlugins((ps) => ps.map((p) => (p.id === id ? { ...p, status: 'needs-login', account: null, why: 'Logged out. Installed games stay playable.' } : p))); say('Logged out'); },
    approve: (id) => { setPlugins((ps) => ps.map((p) => (p.id === id ? { ...p, status: 'ready' } : p))); say('Plugin approved'); },
    retryPlugin: (id) => say('Restarting the plugin', 'info'),
    // runners
    installRunner: (id) => { setRunners((rs) => rs.map((r) => (r.id === id ? { ...r, installed: true, added: Date.now(), isNew: false } : r))); say(`Installing ${runners.find((r) => r.id === id)?.name}`); },
    removeRunner: (id) => { setRunners((rs) => rs.map((r) => (r.id === id ? { ...r, installed: false } : r))); say(`Removed ${runners.find((r) => r.id === id)?.name}`); },
    // attention
    fixIssue: (id) => { const i = issues.find((x) => x.id === id); if (i?.kind === 'conflict') { L.actions.openConflict(i.gameId); return; } setIssues((is) => is.filter((x) => x.id !== id)); say(`${i.fix}: started`); },
    // storage
    cleanLeftovers: (ids) => { const n = leftovers.filter((l) => ids.includes(l.id)).reduce((a, l) => a + l.size, 0); setLeftovers((ls) => ls.filter((l) => !ids.includes(l.id))); say(`Freed ${n.toFixed(1)} GB`); },
    // downloads
    togglePause: (id) => setPaused((p) => (p.includes(id) ? p.filter((x) => x !== id) : [...p, id])),
    cancelJob: (id) => { setCanceled((c) => [...c, id]); say('Canceled. The partial files are listed in Storage.'); },
    dismiss: (id) => setDismissed((d) => [...d, id]),
    // dialogs
    open: (kind, extra = {}) => setDialog({ kind, ...extra }),
    close: () => setDialog(null),
    // social
    setMe: (me) => setSocial((s) => ({ ...s, me })),
    setSocialOpt: (k, v) => setSocial((s) => ({ ...s, [k]: v })),
    toggleSocialStore: (st) => setSocial((s) => ({ ...s, stores: { ...s.stores, [st]: !s.stores[st] } })),
    openSocial: (open = true) => setSocial((s) => ({ ...s, open })),
    openChat: (fid) => { setSocial((s) => ({ ...s, chat: fid, open: true })); if (fid) setChats((c) => (c[fid] ? { ...c, [fid]: { ...c[fid], unread: 0 } } : { ...c, [fid]: { unread: 0, msgs: [] } })); },
    send: (fid, text) => setChats((c) => ({ ...c, [fid]: { ...(c[fid] || { msgs: [] }), msgs: [...(c[fid]?.msgs || []), { id: Date.now(), from: 'me', text, at: Date.now() }] } })),
    resend: (fid, mid) => { setChats((c) => ({ ...c, [fid]: { ...c[fid], msgs: c[fid].msgs.map((m) => (m.id === mid ? { ...m, failed: false, at: Date.now() } : m)) } })); say('Sent'); },
    loadOlder: (fid) => setChats((c) => ({ ...c, [fid]: { ...c[fid], older: false, msgs: [{ id: -2, from: 'them', text: 'gg, that extraction was close', at: NOW - 2 * D }, { id: -1, from: 'me', text: 'next time I’m carrying the samples', at: NOW - 2 * D + 2 * M }, ...c[fid].msgs] } })),
    setSetting: (k, v) => setSettings((s) => ({ ...s, [k]: v })),
  };

  const visibleJobs = L.jobs.filter((j) => !canceled.includes(j.id));
  const attentionCount = issues.length;
  return {
    page, back, tint, theme, lang, t, view, own, sideloaded, collection, tiers, statuses, lentOnly, collections, favs, status, notes,
    plugins, runners, issues, leftovers, paused, dismissed, demo, dialog, social, chats, settings, unread, pluginAttention, activeFilters,
    jobs: visibleJobs, attentionCount, actions,
  };
}

// Colour variables for the three tint modes and three themes.
export function tintVars(tone, tint, theme, { vivid, toneAt }) {
  const light = theme === 'light';
  const black = theme === 'black';
  const base = light
    ? { '--s0': '#e9ecf0', '--s1': '#f1f3f6', '--s-pop': '#ffffff', '--k-bg': '#f7f8fa', '--k-bg-2': '#eceff3', '--k-bg-3': '#dfe3e9', '--k-line': '#dde1e7', '--k-line-strong': '#c4cad3', '--k-text': '#161a20', '--k-muted': '#525c6a', '--k-faint': '#7a8494', '--k-on-accent': '#ffffff', '--k-glass': 'rgba(250, 251, 252, 0.86)' }
    : black
      ? { '--s0': '#000000', '--s1': '#05060a', '--s-pop': '#101216', '--k-bg': '#08090c', '--k-bg-2': '#121418', '--k-bg-3': '#1c1f25', '--k-line': '#181b20', '--k-line-strong': '#2a2f37' }
      : {};
  if (tint === 'off') return { ...base, '--acc': light ? '#3b5bdb' : '#9db0ff', '--acc-dim': light ? '#dfe5f5' : black ? '#0e1016' : '#1f232a' };
  if (tint === 'accent') return { ...base, '--acc': light ? toneAt(tone, 0.75, 0.38, '#3b5bdb') : vivid(tone, 0.7, 0.6) || '#9db0ff', '--acc-dim': light ? toneAt(tone, 0.45, 0.88, '#dfe5f5') : toneAt(tone, 0.45, black ? 0.09 : 0.17, '#1f232a') };
  if (light) {
    return {
      ...base,
      '--acc': toneAt(tone, 0.75, 0.38, '#3b5bdb'), '--acc-dim': toneAt(tone, 0.5, 0.84, '#dfe5f5'),
      '--s0': toneAt(tone, 0.35, 0.88, '#e9ecf0'), '--s1': toneAt(tone, 0.32, 0.92, '#f1f3f6'), '--s-pop': toneAt(tone, 0.25, 0.98, '#fff'),
      '--k-bg': toneAt(tone, 0.3, 0.95, '#f7f8fa'), '--k-bg-2': toneAt(tone, 0.32, 0.9, '#eceff3'), '--k-bg-3': toneAt(tone, 0.32, 0.85, '#dfe3e9'),
      '--k-line': toneAt(tone, 0.25, 0.85, '#dde1e7'), '--k-line-strong': toneAt(tone, 0.25, 0.76, '#c4cad3'),
      '--k-text': toneAt(tone, 0.4, 0.1, '#161a20'), '--k-muted': toneAt(tone, 0.2, 0.36, '#525c6a'), '--k-faint': toneAt(tone, 0.15, 0.5, '#7a8494'),
    };
  }
  const d = black ? 0.5 : 1;
  return {
    ...base,
    '--acc': toneAt(tone, 0.9, 0.66, '#9db0ff'),
    '--acc-dim': toneAt(tone, 0.42, 0.2 * d, '#1f232a'),
    '--s0': toneAt(tone, 0.5, 0.06 * d, '#13161b'),
    '--s1': toneAt(tone, 0.48, 0.08 * d, '#15181d'),
    '--s-pop': toneAt(tone, 0.42, 0.13 * d, '#1b1f26'),
    '--k-bg': toneAt(tone, 0.46, 0.1 * d, '#181b21'),
    '--k-bg-2': toneAt(tone, 0.42, 0.14 * d, '#1f232a'),
    '--k-bg-3': toneAt(tone, 0.38, 0.2 * d, '#293039'),
    '--k-line': toneAt(tone, 0.34, 0.17 * d, '#262b33'),
    '--k-line-strong': toneAt(tone, 0.3, 0.26 * d, '#36404b'),
    '--k-text': toneAt(tone, 0.3, 0.95, '#e9edf2'),
    '--k-muted': toneAt(tone, 0.25, 0.72, '#9ca7b5'),
    '--k-faint': toneAt(tone, 0.22, 0.52, '#687384'),
    '--k-on-accent': toneAt(tone, 0.5, 0.07, '#0f1218'),
  };
}

export const gb = (n) => (n >= 1000 ? `${(n / 1000).toFixed(2)} TB` : `${n < 10 ? n.toFixed(1) : Math.round(n)} GB`);
export const tierRank = (t) => TIER_ORDER.indexOf(t);
export const copySize = (g) => parseFloat(activeCopy(g).size) || 0;
