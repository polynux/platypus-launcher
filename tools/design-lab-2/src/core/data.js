// Mock library for the design lab. Art comes from the Steam CDN; every number
// below is illustrative, not read from a real machine.

const cdn = (appid, file) => `https://cdn.cloudflare.steamstatic.com/steam/apps/${appid}/${file}`;

export const art = (g) => ({
  hero: cdn(g.appid, 'library_hero.jpg'),
  logo: cdn(g.appid, 'logo.png'),
  cover: cdn(g.appid, 'library_600x900.jpg'),
  header: cdn(g.appid, 'header.jpg'),
});

export const STORES = {
  steam: { name: 'Steam' },
  gog: { name: 'GOG' },
  epic: { name: 'Epic' },
  amazon: { name: 'Amazon' },
  itch: { name: 'itch.io' },
  local: { name: 'Local' },
};

export const RUNNERS = [
  { id: 'ge-10-4', name: 'GE-Proton10-4', family: 'GE-Proton', wine: '10.0', note: 'Community build, media codecs and game fixes' },
  { id: 'ge-10-3', name: 'GE-Proton10-3', family: 'GE-Proton', wine: '10.0', note: 'Previous GE release' },
  { id: 'exp', name: 'Proton Experimental', family: 'Valve', wine: '10.0', note: 'Valve, updates on its own' },
  { id: 'p9', name: 'Proton 9.0-4', family: 'Valve', wine: '9.0', note: 'Valve stable' },
  { id: 'cachy', name: 'Proton-CachyOS 10.0', family: 'CachyOS', wine: '10.0', note: 'x86-64-v3 build' },
  { id: 'wine', name: 'Wine 10.17', family: 'System', wine: '10.17', note: '/usr/bin/wine' },
];

export const NATIVE = { id: 'native', name: 'Native Linux', family: 'Native' };

export const GRAPHICS = ['Bundled with runner', 'DXVK 2.7.1', 'DXVK 2.6.2', 'VKD3D-Proton 2.14.1', 'WineD3D (OpenGL)'];
export const SYNC_PRIMITIVES = ['NTSync', 'Fsync', 'Esync', 'Off'];
export const RESOLUTIONS = ['Native (2560×1440)', '1920×1080', '3840×2160 (upscaled)', '1280×800'];

export const RECIPES = [
  { id: 'vcrun2022', label: 'vcrun2022', name: 'Visual C++ runtime', kind: 'winetricks', cat: 'Runtimes', desc: 'Visual C++ 2015–2022 libraries. Most modern games need them.' },
  { id: 'dotnet48', label: 'dotnet48', name: '.NET Framework 4.8', kind: 'winetricks', cat: 'Runtimes', desc: 'Needed by some launchers and mod managers.' },
  { id: 'd3dcompiler_47', label: 'd3dcompiler_47', name: 'Direct3D shader compiler', kind: 'winetricks', cat: 'Runtimes', desc: 'Microsoft’s compiler, for games that ship shaders as source.' },
  { id: 'physx', label: 'physx', name: 'PhysX', kind: 'winetricks', cat: 'Runtimes', desc: 'NVIDIA physics runtime for older games.' },
  { id: 'xact_x64', label: 'xact_x64', name: 'XAudio sound engine', kind: 'winetricks', cat: 'Audio and video', desc: 'Fixes missing or crackling sound in games built on XACT.' },
  { id: 'mf', label: 'mf', name: 'Media Foundation', kind: 'umu', cat: 'Audio and video', desc: 'Plays cutscene videos that otherwise stay black.' },
  { id: 'corefonts', label: 'corefonts', name: 'Microsoft core fonts', kind: 'winetricks', cat: 'Fonts and text', desc: 'Arial, Times and friends, for menus that show boxes instead of text.' },
  { id: 'cjkfonts', label: 'cjkfonts', name: 'Asian language fonts', kind: 'winetricks', cat: 'Fonts and text', desc: 'Chinese, Japanese and Korean glyphs.' },
  { id: 'eac', label: 'eac', name: 'Easy Anti-Cheat bridge', kind: 'umu', cat: 'Anti-cheat', desc: 'Lets EAC-protected games go online when the developer allows Linux.' },
  { id: 'battleye', label: 'battleye', name: 'BattlEye bridge', kind: 'umu', cat: 'Anti-cheat', desc: 'Same as above, for BattlEye.' },
  { id: 'nogfwl', label: 'nogfwl', name: 'Remove Games for Windows Live', kind: 'umu', cat: 'Game fixes', desc: 'Strips the dead GFWL login so the game starts.' },
  { id: 'nofsr', label: 'nofsr', name: 'Turn off Proton’s FSR hack', kind: 'umu', cat: 'Game fixes', desc: 'Stops Proton from upscaling the game on its own.' },
  { id: 'skiplauncher', label: 'skiplauncher', name: 'Skip the publisher launcher', kind: 'umu', cat: 'Game fixes', desc: 'Starts the game directly instead of its own launcher.' },
];

// Fixes players or curators suggest per game. "why" is shown as-is.
export const RECOMMEND = {
  bg3: [{ id: 'skiplauncher', why: 'The Larian launcher adds 20 s and sometimes hangs.', src: 'ProtonDB', votes: 412 }],
  cyberpunk: [{ id: 'vcrun2022', why: 'REDmod tools fail to start without it.', src: 'ProtonDB', votes: 188 }],
  kcd2: [{ id: 'xact_x64', why: 'Crackling dialogue under PipeWire.', src: 'Curated' }, { id: 'mf', why: 'Intro cutscenes stay black.', src: 'Curated' }],
  sh2: [{ id: 'mf', why: 'Some cutscenes freeze on the first frame.', src: 'ProtonDB', votes: 96 }],
  fallout3: [{ id: 'nogfwl', why: 'The game will not start while GFWL is present.', src: 'Curated' }, { id: 'd3dcompiler_47', why: 'Water renders black without it.', src: 'ProtonDB', votes: 57 }],
  witcher3: [{ id: 'd3dcompiler_47', why: 'Needed by the DX11 executable.', src: 'ProtonDB', votes: 230 }],
  nightreign: [{ id: 'eac', why: 'Online play needs the EAC bridge.', src: 'Curated' }],
  starfield: [{ id: 'vcrun2022', why: 'Script extender needs the runtime.', src: 'ProtonDB', votes: 41 }, { id: 'nofsr', why: 'Double upscaling blurs the image with FSR 3 on.', src: 'ProtonDB', votes: 33 }],
  gtav: [{ id: 'vcrun2022', why: 'The Rockstar launcher needs it.', src: 'Curated' }],
  teardown: [{ id: 'vcrun2022', why: 'Added by hand, so nothing was set up for you.', src: 'Platypus' }],
};

const H = 3600e3;
const D = 24 * H;

// stores: first entry is the copy Platypus launches by default.
export const GAMES = [
  {
    id: 'bg3', appid: 1086940, title: 'Baldur’s Gate 3', dev: 'Larian Studios', year: 2023, genre: 'RPG',
    stores: [{ store: 'steam', installed: true, size: '122.4 GB', version: 'Patch 8 hotfix 4' }, { store: 'gog', installed: false, size: '119.8 GB', version: 'Patch 8 hotfix 4' }],
    runner: 'exp', tier: 'platinum', hours: 312.6, lastAgo: 3 * H, sync: 'conflict',
    recipes: ['vcrun2022'], env: [['DXVK_ASYNC', '1'], ['PROTON_ENABLE_NVAPI', '1']], args: '--skip-launcher',
    graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [
      { src: 'ProtonDB', when: 4 * D, text: 'Skip the Larian launcher with --skip-launcher; Vulkan renderer is the smoother path.' },
      { src: 'Curated', when: 30 * D, text: 'vcrun2022 needed for the mod manager; the game itself runs without it.' },
      { src: 'You', when: 9 * D, text: 'Act 3 city drops to ~70 fps in Lower City; same on Windows.' },
    ],
    saves: [
      { slot: 'Honour Mode — Act 3', local: { at: 3 * H, size: '47.9 MB', device: 'this PC' }, cloud: { at: 1.2 * H, size: '48.6 MB', device: 'steamdeck' }, conflict: true },
      { slot: 'Tactician — Karlach', local: { at: 6 * D, size: '31.0 MB', device: 'this PC' }, cloud: { at: 6 * D, size: '31.0 MB', device: 'this PC' } },
      { slot: 'Profile / settings', local: { at: 2.1 * H, size: '84 KB', device: 'this PC' }, cloud: { at: 2.1 * H, size: '84 KB', device: 'this PC' } },
    ],
  },
  {
    id: 'cyberpunk', appid: 1091500, title: 'Cyberpunk 2077', dev: 'CD PROJEKT RED', year: 2020, genre: 'Action RPG',
    stores: [{ store: 'gog', installed: true, size: '71.2 GB', version: '2.31' }, { store: 'steam', installed: false, size: '71.0 GB', version: '2.31' }],
    runner: 'ge-10-4', tier: 'platinum', hours: 87.4, lastAgo: 26 * H, sync: 'synced',
    recipes: [], env: [['PROTON_ENABLE_NVAPI', '1'], ['VKD3D_CONFIG', 'dxr11']], args: '--launcher-skip -modded',
    graphics: 'VKD3D-Proton 2.14.1', syncPrim: 'NTSync', mangohud: true, gamemode: true, gamescope: true, fsr: false,
    notes: [
      { src: 'ProtonDB', when: 2 * D, text: 'Path tracing works through VKD3D-Proton with DXR 1.1 enabled.' },
      { src: 'Curated', when: 60 * D, text: 'REDmod needs --launcher-skip -modded to load mods without the launcher.' },
    ],
    saves: [
      { slot: 'Phantom Liberty — ManualSave-12', local: { at: 26 * H, size: '6.1 MB', device: 'this PC' }, cloud: { at: 26 * H, size: '6.1 MB', device: 'this PC' } },
      { slot: 'AutoSave-3', local: { at: 26 * H, size: '6.0 MB', device: 'this PC' }, cloud: { at: 26 * H, size: '6.0 MB', device: 'this PC' } },
    ],
  },
  {
    id: 'hades2', appid: 1145350, title: 'Hades II', dev: 'Supergiant Games', year: 2025, genre: 'Roguelike',
    stores: [{ store: 'epic', installed: true, size: '11.2 GB', version: '1.0.4' }, { store: 'steam', installed: false, size: '11.1 GB', version: '1.0.4' }],
    runner: 'exp', tier: 'platinum', hours: 66.0, lastAgo: 5 * H, sync: 'syncing',
    recipes: [], env: [], args: '',
    graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 6 * D, text: 'Works out of the box. Vulkan renderer selectable in settings.' }],
    saves: [
      { slot: 'Profile 1 — Night 41', local: { at: 5 * H, size: '1.2 MB', device: 'this PC' }, cloud: { at: 3 * D, size: '1.1 MB', device: 'this PC' }, uploading: true },
    ],
  },
  {
    id: 'nightreign', appid: 2622380, title: 'Elden Ring Nightreign', dev: 'FromSoftware', year: 2025, genre: 'Action',
    stores: [{ store: 'steam', installed: true, size: '29.1 GB', version: '1.02.3' }],
    runner: 'ge-10-4', tier: 'gold', hours: 41.2, lastAgo: 30 * H, sync: 'synced',
    recipes: ['eac'], env: [['PROTON_EAC_RUNTIME', '~/.local/share/platypus/runtimes/eac']], args: '',
    graphics: 'VKD3D-Proton 2.14.1', syncPrim: 'Fsync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [
      { src: 'ProtonDB', when: 1 * D, text: 'Expect 3–5 minutes of shader stutter on a fresh cache; it settles.' },
      { src: 'Curated', when: 40 * D, text: 'Easy Anti-Cheat works with the EAC runtime recipe enabled.' },
    ],
    saves: [{ slot: 'NR0000.sl2', local: { at: 30 * H, size: '27.4 MB', device: 'this PC' }, cloud: { at: 30 * H, size: '27.4 MB', device: 'this PC' } }],
  },
  {
    id: 'factorio', appid: 427520, title: 'Factorio', dev: 'Wube Software', year: 2020, genre: 'Automation',
    stores: [{ store: 'steam', installed: true, size: '3.8 GB', version: '2.0.72' }, { store: 'gog', installed: false, size: '3.7 GB', version: '2.0.72' }],
    runner: 'native', tier: 'native', hours: 318.0, lastAgo: 2 * H, sync: 'synced',
    recipes: [], env: [], args: '',
    graphics: '—', syncPrim: '—', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 90 * D, text: 'Native Linux build. Nothing to configure.' }],
    saves: [{ slot: 'megabase-space-age.zip', local: { at: 2 * H, size: '212 MB', device: 'this PC' }, cloud: { at: 2 * H, size: '212 MB', device: 'this PC' } }],
  },
  {
    id: 'witcher3', appid: 292030, title: 'The Witcher 3: Wild Hunt', dev: 'CD PROJEKT RED', year: 2015, genre: 'RPG',
    stores: [{ store: 'gog', installed: true, size: '50.6 GB', version: '4.04a' }, { store: 'steam', installed: false, size: '50.4 GB', version: '4.04a' }, { store: 'epic', installed: false, size: '50.4 GB', version: '4.04' }],
    runner: 'ge-10-4', tier: 'gold', hours: 388.1, lastAgo: 61 * D, sync: 'synced',
    recipes: ['d3dcompiler_47'], env: [['DXVK_HUD', 'compiler']], args: '',
    graphics: 'DXVK 2.7.1', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 20 * D, text: 'Use the DX11 executable; DX12 next-gen mode stutters on some drivers.' }],
    saves: [{ slot: 'QuickSave — Toussaint', local: { at: 61 * D, size: '4.4 MB', device: 'this PC' }, cloud: { at: 61 * D, size: '4.4 MB', device: 'this PC' } }],
  },
  {
    id: 'kcd2', appid: 1771300, title: 'Kingdom Come: Deliverance II', dev: 'Warhorse Studios', year: 2025, genre: 'RPG',
    stores: [{ store: 'steam', installed: true, size: '98.0 GB', version: '1.4.2' }, { store: 'gog', installed: false, size: '97.6 GB', version: '1.4.2' }],
    runner: 'exp', tier: 'gold', hours: 95.3, lastAgo: 20 * H, sync: 'local',
    recipes: ['xact_x64', 'mf'], env: [['PULSE_LATENCY_MSEC', '60']], args: '',
    graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 15 * D, text: 'Crackling audio under PipeWire: PULSE_LATENCY_MSEC=60 fixes it.' }],
    saves: [{ slot: 'Kuttenberg — 113 days', local: { at: 20 * H, size: '9.8 MB', device: 'this PC' }, cloud: null }],
  },
  {
    id: 'helldivers', appid: 553850, title: 'Helldivers 2', dev: 'Arrowhead', year: 2024, genre: 'Co-op shooter',
    stores: [{ store: 'steam', installed: true, size: '52.0 GB', version: '01.004.002' }],
    runner: 'exp', tier: 'silver', hours: 154.0, lastAgo: 1 * H, sync: 'synced',
    recipes: [], env: [], args: '',
    graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 3 * D, text: 'GameGuard sometimes fails on first launch after an update; relaunch once.' }],
    saves: [{ slot: 'Server-side progress', local: null, cloud: { at: 1 * H, size: '—', device: 'Arrowhead servers' } }],
  },
  {
    id: 'stardew', appid: 413150, title: 'Stardew Valley', dev: 'ConcernedApe', year: 2016, genre: 'Farming sim',
    stores: [{ store: 'steam', installed: true, size: '1.2 GB', version: '1.6.15' }, { store: 'gog', installed: false, size: '1.1 GB', version: '1.6.15' }],
    runner: 'native', tier: 'native', hours: 412.4, lastAgo: 6 * H, sync: 'synced',
    recipes: [], env: [['SMAPI_NO_TERMINAL', '1']], args: '',
    graphics: '—', syncPrim: '—', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'You', when: 12 * D, text: 'SMAPI detected, 8 mods active.' }],
    saves: [{ slot: 'Polyfarm_338712', local: { at: 6 * H, size: '2.3 MB', device: 'this PC' }, cloud: { at: 6 * H, size: '2.3 MB', device: 'this PC' } }],
  },
  {
    id: 'balatro', appid: 2379780, title: 'Balatro', dev: 'LocalThunk', year: 2024, genre: 'Deckbuilder',
    stores: [{ store: 'steam', installed: true, size: '62 MB', version: '1.0.1o' }],
    runner: 'p9', tier: 'platinum', hours: 140.2, lastAgo: 4 * H, sync: 'synced',
    recipes: [], env: [], args: '',
    graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 10 * D, text: 'Perfect. Runs on anything.' }],
    saves: [{ slot: 'profile 1', local: { at: 4 * H, size: '220 KB', device: 'this PC' }, cloud: { at: 4 * H, size: '220 KB', device: 'this PC' } }],
  },
  {
    id: 'drg', appid: 548430, title: 'Deep Rock Galactic', dev: 'Ghost Ship Games', year: 2020, genre: 'Co-op shooter',
    stores: [{ store: 'steam', installed: true, size: '28.0 GB', version: 'S5 1.39' }],
    runner: 'ge-10-4', tier: 'platinum', hours: 240.0, lastAgo: 3 * D, sync: 'synced',
    recipes: [], env: [], args: '', graphics: 'DXVK 2.7.1', syncPrim: 'NTSync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 8 * D, text: 'Rock and stone. DX11 mode recommended.' }],
    saves: [{ slot: 'Player.sav', local: { at: 3 * D, size: '1.9 MB', device: 'this PC' }, cloud: { at: 3 * D, size: '1.9 MB', device: 'this PC' } }],
  },
  {
    id: 'sh2', appid: 2124490, title: 'Silent Hill 2', dev: 'Bloober Team', year: 2024, genre: 'Horror',
    stores: [{ store: 'steam', installed: true, size: '48.6 GB', version: '1.0.6' }],
    runner: 'ge-10-4', tier: 'silver', hours: 12.1, lastAgo: 14 * D, sync: 'synced',
    recipes: ['d3dcompiler_47'], env: [['VKD3D_SHADER_CACHE_PATH', '~/.cache/platypus/sh2']], args: '-dx12',
    graphics: 'VKD3D-Proton 2.14.1', syncPrim: 'Fsync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 5 * D, text: 'Traversal stutter in town; a warm shader cache halves it.' }],
    saves: [{ slot: 'Slot 03 — Brookhaven', local: { at: 14 * D, size: '3.0 MB', device: 'this PC' }, cloud: { at: 14 * D, size: '3.0 MB', device: 'this PC' } }],
  },
  {
    id: 'valheim', appid: 892970, title: 'Valheim', dev: 'Iron Gate', year: 2021, genre: 'Survival',
    stores: [{ store: 'steam', installed: true, size: '12.0 GB', version: '0.220.5' }],
    runner: 'native', tier: 'native', hours: 205.0, lastAgo: 2 * D, sync: 'synced',
    recipes: [], env: [], args: '-window-mode exclusive', graphics: '—', syncPrim: '—', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 70 * D, text: 'Native Vulkan with -force-vulkan is faster than the GL default.' }],
    saves: [{ slot: 'world: Ashlands', local: { at: 2 * D, size: '88 MB', device: 'this PC' }, cloud: { at: 2 * D, size: '88 MB', device: 'this PC' } }],
  },
  {
    id: 'metro', appid: 412020, title: 'Metro Exodus', dev: '4A Games', year: 2019, genre: 'Shooter',
    stores: [{ store: 'epic', installed: true, size: '86.0 GB', version: 'Enhanced 1.0.7.17' }, { store: 'steam', installed: false, size: '85.2 GB', version: '1.0.7.17' }],
    runner: 'ge-10-3', tier: 'gold', hours: 40.0, lastAgo: 9 * D, sync: 'synced',
    recipes: [], env: [['VKD3D_CONFIG', 'dxr']], args: '', graphics: 'VKD3D-Proton 2.14.1', syncPrim: 'NTSync', mangohud: true, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 25 * D, text: 'Enhanced Edition needs DXR; enable VKD3D_CONFIG=dxr.' }],
    saves: [{ slot: 'Caspian — chapter 5', local: { at: 9 * D, size: '1.4 MB', device: 'this PC' }, cloud: { at: 9 * D, size: '1.4 MB', device: 'this PC' } }],
  },
  {
    id: 'civ7', appid: 1295660, title: 'Sid Meier’s Civilization VII', dev: 'Firaxis', year: 2025, genre: 'Strategy',
    stores: [{ store: 'steam', installed: true, size: '33.0 GB', version: '1.2.3', update: '2.1 GB' }, { store: 'epic', installed: false, size: '32.8 GB', version: '1.2.3' }],
    runner: 'native', tier: 'native', hours: 8.0, lastAgo: 30 * D, sync: 'off',
    recipes: [], env: [], args: '', graphics: '—', syncPrim: '—', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 30 * D, text: 'Native build. Late-game turn times are CPU-bound.' }],
    saves: [{ slot: 'AutoSave_0212', local: { at: 30 * D, size: '5.5 MB', device: 'this PC' }, cloud: null }],
  },
  {
    id: 'hollow', appid: 367520, title: 'Hollow Knight', dev: 'Team Cherry', year: 2017, genre: 'Metroidvania',
    stores: [{ store: 'gog', installed: true, size: '9.0 GB', version: '1.5.78' }, { store: 'steam', installed: false, size: '9.0 GB', version: '1.5.78' }],
    runner: 'native', tier: 'native', hours: 120.0, lastAgo: 50 * D, sync: 'synced',
    recipes: [], env: [], args: '', graphics: '—', syncPrim: '—', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 100 * D, text: 'Native. Silksong save import is not shared.' }],
    saves: [{ slot: 'user2.dat', local: { at: 50 * D, size: '640 KB', device: 'this PC' }, cloud: { at: 50 * D, size: '640 KB', device: 'this PC' } }],
  },
  {
    id: 'dave', appid: 1868140, title: 'Dave the Diver', dev: 'Mintrocket', year: 2023, genre: 'Adventure',
    stores: [{ store: 'steam', installed: true, size: '6.8 GB', version: '1.0.3.1' }],
    runner: 'p9', tier: 'platinum', hours: 33.0, lastAgo: 20 * D, sync: 'synced',
    recipes: [], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 40 * D, text: 'Works out of the box.' }],
    saves: [{ slot: 'GameSave_02', local: { at: 20 * D, size: '1.0 MB', device: 'this PC' }, cloud: { at: 20 * D, size: '1.0 MB', device: 'this PC' } }],
  },
  {
    id: 'vampire', appid: 1794680, title: 'Vampire Survivors', dev: 'poncle', year: 2022, genre: 'Roguelite',
    stores: [{ store: 'steam', installed: true, size: '1.1 GB', version: '1.13' }],
    runner: 'native', tier: 'native', hours: 54.0, lastAgo: 12 * D, sync: 'synced',
    recipes: [], env: [], args: '', graphics: '—', syncPrim: '—', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [], saves: [{ slot: 'SaveData', local: { at: 12 * D, size: '410 KB', device: 'this PC' }, cloud: { at: 12 * D, size: '410 KB', device: 'this PC' } }],
  },
  {
    id: 'gtav', appid: 271590, title: 'Grand Theft Auto V', dev: 'Rockstar North', year: 2015, genre: 'Open world',
    stores: [{ store: 'epic', installed: true, size: '105 GB', version: 'Enhanced 1.0.1013' }, { store: 'steam', installed: false, size: '104 GB', version: '1.0.1013' }],
    runner: 'ge-10-4', tier: 'bronze', hours: 61.0, lastAgo: 16 * D, sync: 'synced',
    recipes: ['vcrun2022'], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [
      { src: 'Curated', when: 200 * D, text: 'Story mode runs. GTA Online blocks Linux through BattlEye; Platypus cannot change that.' },
    ],
    saves: [{ slot: 'SGTA50000', local: { at: 16 * D, size: '512 KB', device: 'this PC' }, cloud: { at: 16 * D, size: '512 KB', device: 'Rockstar' } }],
  },
  {
    id: 'starfield', appid: 1716740, title: 'Starfield', dev: 'Bethesda', year: 2023, genre: 'RPG',
    stores: [{ store: 'steam', installed: false, size: '139 GB', version: '1.15.222' }],
    runner: 'exp', tier: 'bronze', hours: 22.0, lastAgo: 40 * D, sync: 'off',
    recipes: [], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 12 * D, text: 'Frame pacing is rough in New Atlantis. Needs VKD3D-Proton 2.14+.' }],
    saves: [{ slot: 'Save 41 — Akila', local: null, cloud: { at: 40 * D, size: '18 MB', device: 'old PC' } }],
  },
  {
    id: 'subnautica', appid: 264710, title: 'Subnautica', dev: 'Unknown Worlds', year: 2018, genre: 'Survival',
    stores: [{ store: 'epic', installed: false, size: '17.4 GB', version: '71288' }, { store: 'steam', installed: false, size: '17.4 GB', version: '71288' }],
    runner: 'p9', tier: 'gold', hours: 18.0, lastAgo: 400 * D, sync: 'off',
    recipes: [], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 300 * D, text: 'Stable. Older saves warn once when imported.' }], saves: [],
  },
  {
    id: 'outerwilds', appid: 753640, title: 'Outer Wilds', dev: 'Mobius Digital', year: 2019, genre: 'Exploration',
    stores: [{ store: 'steam', installed: false, size: '9.8 GB', version: '1.1.15' }, { store: 'epic', installed: false, size: '9.8 GB', version: '1.1.15' }],
    runner: 'p9', tier: 'platinum', hours: 26.0, lastAgo: 380 * D, sync: 'off',
    recipes: [], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'You', when: 380 * D, text: 'Finished. Echoes of the Eye not started.' }], saves: [],
  },
  {
    id: 'deltarune', appid: 1671210, title: 'Deltarune', dev: 'Toby Fox', year: 2025, genre: 'RPG',
    stores: [{ store: 'steam', installed: true, size: '940 MB', version: '1.04' }],
    runner: 'p9', tier: 'platinum', hours: 9.0, lastAgo: 32 * D, sync: 'synced',
    recipes: [], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [], saves: [{ slot: 'filech4_0', local: { at: 32 * D, size: '12 KB', device: 'this PC' }, cloud: { at: 32 * D, size: '12 KB', device: 'this PC' } }],
  },
  {
    id: 'teardown', appid: null, title: 'Teardown (local install)', dev: 'Tuxedo Labs', year: 2022, genre: 'Sandbox',
    stores: [{ store: 'local', installed: true, size: '4.1 GB', version: '1.6.0' }],
    runner: 'cachy', tier: 'gold', hours: 3.5, lastAgo: 7 * D, sync: 'off',
    recipes: ['vcrun2022'], env: [], args: '', graphics: 'Bundled with runner', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'You', when: 7 * D, text: 'Added by hand from ~/Games/Teardown/teardown.exe' }], saves: [],
    heroOverride: 'https://cdn.cloudflare.steamstatic.com/steam/apps/1167630/library_hero.jpg',
    logoOverride: 'https://cdn.cloudflare.steamstatic.com/steam/apps/1167630/logo.png',
    headerOverride: 'https://cdn.cloudflare.steamstatic.com/steam/apps/1167630/header.jpg',
    coverOverride: 'https://cdn.cloudflare.steamstatic.com/steam/apps/1167630/library_600x900.jpg',
  },
  {
    id: 'celeste', appid: 504230, title: 'Celeste', dev: 'Extremely OK Games', year: 2018, genre: 'Platformer',
    stores: [{ store: 'itch', installed: true, size: '1.2 GB', version: '1.4.0.0' }, { store: 'steam', installed: false, size: '1.2 GB', version: '1.4.0.0' }],
    runner: 'native', tier: 'native', hours: 61.0, lastAgo: 11 * D, sync: 'local',
    recipes: [], env: [], args: '', graphics: '—', syncPrim: '—', mangohud: false, gamemode: false, gamescope: false, fsr: false,
    notes: [{ src: 'Curated', when: 120 * D, text: 'Native build from itch.io. Steam copy shares no saves.' }],
    saves: [{ slot: 'Save 0 — Farewell', local: { at: 11 * D, size: '96 KB', device: 'this PC' }, cloud: null }],
  },
  {
    id: 'fallout3', appid: 22370, title: 'Fallout 3: Game of the Year', dev: 'Bethesda', year: 2009, genre: 'RPG',
    stores: [{ store: 'amazon', installed: true, size: '7.4 GB', version: '1.7.0.3' }, { store: 'gog', installed: false, size: '7.6 GB', version: '1.7.0.4' }, { store: 'steam', installed: false, size: '7.4 GB', version: '1.7.0.3' }],
    runner: 'ge-10-4', tier: 'gold', hours: 34.0, lastAgo: 45 * D, sync: 'off',
    recipes: ['d3dcompiler_47'], env: [], args: '', graphics: 'DXVK 2.7.1', syncPrim: 'NTSync', mangohud: false, gamemode: true, gamescope: false, fsr: false,
    notes: [{ src: 'ProtonDB', when: 60 * D, text: 'Remove Games for Windows Live with the curated recipe or it will not start.' }],
    saves: [{ slot: 'Save 22 — Megaton', local: { at: 45 * D, size: '4.0 MB', device: 'this PC' }, cloud: null }],
  },
];

// Deterministic session history: last 14 days.
function rng(seed) {
  let s = 0;
  for (const c of seed) s = (s * 31 + c.charCodeAt(0)) >>> 0;
  return () => ((s = (s * 1664525 + 1013904223) >>> 0) / 4294967296);
}

export function sessionsFor(g, now) {
  if (!g.stores.some((s) => s.installed)) return [];
  const r = rng(g.id);
  const out = [];
  let t = now - g.lastAgo;
  const count = Math.max(1, Math.min(9, Math.round(g.hours / 30) + 2));
  for (let i = 0; i < count; i++) {
    const dur = Math.round(20 + r() * (g.hours > 100 ? 220 : 110));
    const crash = r() < 0.08;
    out.push({
      start: t - dur * 60e3,
      dur,
      runner: g.runner,
      exit: crash ? 'crashed · SIGSEGV' : 'exit 0',
      crash,
    });
    t -= (8 + r() * 50) * H;
    if (now - t > 14 * D) break;
  }
  return out;
}

export const JOBS_SEED = [
  { id: 'shader', gameId: 'nightreign', kind: 'shaders', label: 'Elden Ring Nightreign · Fossilize shader replay', progress: 0.38, rate: 0.012, unit: '1,204 / 3,170 pipelines' },
  { id: 'prefix', kind: 'prefix', label: 'GE-Proton10-4 · updating 3 prefixes', progress: 0.71, rate: 0.02, unit: 'wineboot -u' },
  { id: 'sync', gameId: 'hades2', kind: 'sync', label: 'Hades II · uploading save to Epic cloud', progress: 0.2, rate: 0.05, unit: '1.2 MB' },
  { id: 'dl-civ7', gameId: 'civ7', kind: 'download', label: 'Civilization VII · update 1.2.4', progress: 0.12, rate: 0.006, unit: '41.8 MB/s' },
];

export const NOW_FIXED = Date.now();

// Short store-page style descriptions (paraphrased for the mockup).
const DESCRIPTIONS = {
  bg3: 'Gather your party and return to the Forgotten Realms. A party-based RPG built on D&D 5th edition, where every choice can be turned against you, alone or in co-op.',
  cyberpunk: 'An open-world action adventure set in Night City, a megalopolis obsessed with power and body modification. You play V, a mercenary chasing an implant that holds the key to immortality.',
  hades2: 'Battle beyond the Underworld as Melinoë, Princess of the Underworld, using dark sorcery to take on the Titan of Time in this sequel to the award-winning rogue-like dungeon crawler.',
  nightreign: 'A standalone co-op adventure set in the Elden Ring universe. Team up with two other players, survive three days and nights, and face the Nightlord before the rain closes in.',
  factorio: 'Build and maintain factories. Mine resources, research technologies, automate production and defend your base, from the first hand-fed furnace to rockets leaving the planet.',
  witcher3: 'You are Geralt of Rivia, a monster slayer for hire, searching a war-torn open world for the child of prophecy. Includes both expansions, Hearts of Stone and Blood and Wine.',
  kcd2: 'A story-driven open-world RPG set in 15th-century Bohemia. Follow Henry through a realistic, brutal medieval kingdom where how you act shapes how the world treats you.',
  helldivers: 'A co-op third-person shooter where up to four Helldivers spread managed democracy across the galaxy, calling down orbital strikes and supply drops, and trying not to shoot each other.',
  stardew: 'You inherit your grandfather’s old farm plot in Stardew Valley. Grow crops, raise animals, fish, mine the caves and become part of a small town’s life, alone or with friends.',
  balatro: 'A poker-inspired roguelike deck builder. Play hands, collect jokers that bend the rules, and build ridiculous synergies to beat each blind before you run out of chips.',
  drg: 'A one to four player co-op FPS about space dwarves, destructible caves and swarms of alien bugs. Mine, fight and get back to the drop pod before it leaves without you.',
  sh2: 'A remake of the psychological horror classic. James Sunderland receives a letter from his wife, who died three years ago, asking him to meet her in Silent Hill.',
  valheim: 'A brutal exploration and survival game for one to ten players, set in a procedurally generated purgatory inspired by Viking culture. Build, sail, craft and fight to prove yourself to Odin.',
  metro: 'Flee the shattered ruins of Moscow and set out across post-apocalyptic Russia aboard the Aurora in this story-driven shooter, now with a sandbox world and a full year of seasons.',
  civ7: 'Build an empire across three ages of history. Found cities, lead famous figures, and change civilization as your people move from antiquity into the modern age.',
  hollow: 'Descend into Hallownest, a vast ruined kingdom of insects and heroes. Explore twisting caverns, fight tainted creatures and meet strange bugs in this hand-drawn action adventure.',
  dave: 'By day, dive into the mysterious Blue Hole to fish and explore. By night, run a sushi restaurant with what you caught. A casual adventure with a surprising amount going on.',
  vampire: 'Mow down thousands of night creatures and survive until dawn. A gothic horror casual game with rogue-lite elements, where your choices snowball into a whirlwind of destruction.',
  gtav: 'When a young street hustler, a retired bank robber and a terrifying psychopath get tangled up with the criminal underworld, they pull off a series of dangerous heists to survive.',
  starfield: 'The first new universe in 25 years from Bethesda Game Studios. Create any character and explore with unparalleled freedom on an epic journey to answer humanity’s greatest mystery.',
  subnautica: 'Descend into the depths of an alien underwater world filled with wonder and peril. Craft equipment, pilot submarines and out-smart wildlife to explore lush reefs and deep trenches.',
  outerwilds: 'An open-world mystery about a solar system trapped in an endless time loop. Explore at your own pace, follow your curiosity, and piece together what happened to a lost civilization.',
  deltarune: 'A story-driven RPG where a human and a monster fall into a dark world. Fight or spare the enemies you meet in turn-based battles that play out as bullet-hell dodging.',
  teardown: 'Plan the perfect heist in a fully destructible, simulated voxel world. Every wall can be broken, every vehicle driven, and every shortcut you make is part of the plan.',
  celeste: 'Help Madeline survive her inner demons on her journey to the top of Celeste Mountain, in this tight, hand-crafted platformer about hundreds of tricky jumps and one very long climb.',
  fallout3: 'Post-nuclear Washington, D.C., 200 years after the bombs. Leave Vault 101 to find your father in the Capital Wasteland. Includes all five add-ons.',
};
for (const g of GAMES) g.desc = DESCRIPTIONS[g.id] || '';
