// Shared mock data — one truth, five readings.
const img = (id, w, h) => `https://picsum.photos/seed/${id}/${w}/${h}`;

export const runnerName = (i) =>
  ['GE-Proton10-4', 'Proton Experimental', 'GE-Proton11-2', 'Proton 9.0', 'System Wine 10.2', 'CachyOS Proton'][i % 6];

export const GAMES = [
  { id: 'cyber-2077', title: 'Cyberpunk 2077', store: 'GOG', runner: runnerName(0), tier: 'platinum', hours: 87, last: '2h ago', size: '64.8 GB', dx: 'DXVK 2.5.1', status: 'ready', cover: img('hyd-cyber', 600, 900), wide: img('hyd-cyber-w', 1200, 500), verdict: 'Runs clean. VRAM headroom comfortable at 1440p Ultra + RT off. Frame-pacing solid after 10.4 patch.' },
  { id: 'elden-night', title: 'Elden Ring: Nightreign', store: 'Steam', runner: runnerName(1), tier: 'gold', hours: 212, last: 'yesterday', size: '29.1 GB', dx: 'VKD3D 2.14', status: 'tweak', cover: img('hyd-elden', 600, 900), wide: img('hyd-elden-w', 1200, 500), verdict: 'Playable with launch flag -vulkan and EasyAntiCheat bridge bound. Occasional shader stutter first 5 min.' },
  { id: 'baldur-3', title: 'Baldur\u2019s Gate 3', store: 'Steam', runner: runnerName(2), tier: 'platinum', hours: 1340, last: '3 days ago', size: '122 GB', dx: 'native', status: 'ready', cover: img('hyd-bg3', 600, 900), wide: img('hyd-bg3-w', 1200, 500), verdict: 'Native Vulkan build. Act 3 city drops to 70fps in dense districts; expected, not Proton related.' },
  { id: 'rogue-mecha', title: 'Armored Core VI', store: 'Steam', runner: runnerName(3), tier: 'gold', hours: 45, last: 'last week', size: '38.4 GB', dx: 'DXVK 2.4', status: 'ready', cover: img('hyd-ac6', 600, 900), wide: img('hyd-ac6-w', 1200, 500), verdict: 'Solid 120fps cap holds. MangoHud overlay recommended for boss-vfx dips.' },
  { id: 'hades-2', title: 'Hades II', store: 'Epic', runner: runnerName(4), tier: 'platinum', hours: 66, last: '4h ago', size: '11.2 GB', dx: 'native', status: 'ready', cover: img('hyd-hades', 600, 900), wide: img('hyd-hades-w', 1200, 500), verdict: 'Early access build runs native, zero tweaks. Save-sync via Legendary verified twice daily.' },
  { id: 'factorio-dlc', title: 'Factorio: Space Age', store: 'Steam', runner: runnerName(5), tier: 'platinum', hours: 318, last: 'today', size: '3.8 GB', dx: 'native', status: 'ready', cover: img('hyd-factorio', 600, 900), wide: img('hyd-factorio-w', 1200, 500), verdict: 'Native. UPS-capped at 240 on megabase saves. Nothing to tune.' },
  { id: 'silent-2', title: 'Silent Hills 2 (2024)', store: 'GOG', runner: runnerName(0), tier: 'silver', hours: 12, last: '2 weeks ago', size: '48.6 GB', dx: 'DXVK 2.5.1', status: 'tweak', cover: img('hyd-sh2', 600, 900), wide: img('hyd-sh2-w', 1200, 500), verdict: 'Needs stutter-fix collection + 4GB patch. Fog volume shader compile hitch on first run.' },
  { id: 'kcd-2', title: 'Kingdom Come II', store: 'Steam', runner: runnerName(1), tier: 'gold', hours: 95, last: 'yesterday', size: '98 GB', dx: 'VKD3D 2.14', status: 'ready', cover: img('hyd-kcd', 600, 900), wide: img('hyd-kcd-w', 1200, 500), verdict: 'PipeWire audio fix required, one registry verb. CPU-bound in Kuttenberg; -1 core parked helps.' },
  { id: 'moonstone', title: 'Moonstone Island', store: 'Epic', runner: runnerName(2), tier: 'platinum', hours: 28, last: 'today', size: '1.9 GB', dx: 'native', status: 'ready', cover: img('hyd-moon', 600, 900), wide: img('hyd-moon-w', 1200, 500), verdict: 'Native. Deck-verified. Nothing to report.' },
  { id: 'wolf-old', title: 'Wolfenstein: Old Blood', store: 'Steam', runner: runnerName(3), tier: 'gold', hours: 0, last: 'never', size: '21.3 GB', dx: 'DXVK 2.4', status: 'idle', cover: img('hyd-wolf', 600, 900), wide: img('hyd-wolf-w', 1200, 500), verdict: 'idTech6 Vulkan. Historically flawless. Installed, not yet launched under Hydra.' },
  { id: 'deltarune', title: 'Deltarune', store: 'Epic', runner: runnerName(4), tier: 'platinum', hours: 9, last: 'last month', size: '0.9 GB', dx: 'native', status: 'ready', cover: img('hyd-delta', 600, 900), wide: img('hyd-delta-w', 1200, 500), verdict: 'Native, tiny, perfect.' },
  { id: 'helldivers', title: 'Helldivers 2', store: 'Steam', runner: runnerName(5), tier: 'silver', hours: 154, last: '1h ago', size: '52 GB', dx: 'VKD3D 2.14', status: 'tweak', cover: img('hyd-hell', 600, 900), wide: img('hyd-hell-w', 1200, 500), verdict: 'nProtect saga continues: requires EAC bridge + launch flag. Verified working 10.1.' },
  { id: 'deep-rock', title: 'Deep Rock Galactic', store: 'Steam', runner: runnerName(0), tier: 'platinum', hours: 240, last: '3h ago', size: '28 GB', dx: 'DXVK 2.5.1', status: 'ready', cover: img('hyd-drg', 600, 900), wide: img('hyd-drg-w', 1200, 500), verdict: 'Bosco approves. Rock and stone.' },
  { id: 'witcher-3', title: 'The Witcher 3', store: 'GOG', runner: runnerName(1), tier: 'gold', hours: 388, last: '2 months ago', size: '50.6 GB', dx: 'DXVK 2.4', status: 'ready', cover: img('hyd-witch', 600, 900), wide: img('hyd-witch-w', 1200, 500), verdict: 'HUD flicker fixed by mangohud no-font-cache flag. Ray Tracing via VKD3D stable.' },
  { id: 'stardew', title: 'Stardew Valley', store: 'Steam', runner: runnerName(2), tier: 'platinum', hours: 412, last: 'today', size: '1.2 GB', dx: 'native', status: 'ready', cover: img('hyd-stardew', 600, 900), wide: img('hyd-stardew-w', 1200, 500), verdict: 'Native with SMAPI mod loader detected. 8 mods active.' },
  { id: 'starfield', title: 'Starfield', store: 'Steam', runner: runnerName(3), tier: 'bronze', hours: 22, last: 'a month ago', size: '139 GB', dx: 'VKD3D 2.14', status: 'tweak', cover: img('hyd-star', 600, 900), wide: img('hyd-star-w', 1200, 500), verdict: 'Upscale-only framegen workaround. Med pacing issues in New Atlantis; Lutris recipe #4412 pending.' },
  { id: 'dave', title: 'Dave the Diver', store: 'Steam', runner: runnerName(4), tier: 'platinum', hours: 33, last: 'last night', size: '6.8 GB', dx: 'native', status: 'ready', cover: img('hyd-dave', 600, 900), wide: img('hyd-dave-w', 1200, 500), verdict: 'Native. Runs on an e-reader.' },
  { id: 'gta-v', title: 'Grand Theft Auto V', store: 'Epic', runner: runnerName(5), tier: 'silver', hours: 61, last: '2 weeks ago', size: '105 GB', dx: 'DXVK 2.5.1', status: 'tweak', cover: img('hyd-gta', 600, 900), wide: img('hyd-gta-w', 1200, 500), verdict: 'BattlEye bridge verified. Menu cursor-grab needs xwayland video mode.' },
  { id: 'palworld', title: 'Palworld', store: 'Steam', runner: runnerName(0), tier: 'gold', hours: 71, last: '3 days ago', size: '44 GB', dx: 'DXVK 2.5.1', status: 'ready', cover: img('hyd-pal', 600, 900), wide: img('hyd-pal-w', 1200, 500), verdict: 'Co-op crossplay needs steam-native flag. Boss-fight shader cache warm after first session.' },
  { id: 'subnautica', title: 'Subnautica', store: 'Epic', runner: runnerName(1), tier: 'gold', hours: 18, last: 'long ago', size: '17.4 GB', dx: 'DXVK 2.4', status: 'idle', cover: img('hyd-sub', 600, 900), wide: img('hyd-sub-w', 1200, 500), verdict: 'Unity title, stable. Old save format warns once on import.' },
  { id: 'valheim', title: 'Valheim', store: 'Steam', runner: runnerName(2), tier: 'platinum', hours: 205, last: '2 days ago', size: '12 GB', dx: 'native', status: 'ready', cover: img('hyd-val', 600, 900), wide: img('hyd-val-w', 1200, 500), verdict: 'Native Vulkan beta branch. Ashlands biome holds 90fps.' },
  { id: 'metro-ex', title: 'Metro Exodus Enhanced', store: 'GOG', runner: runnerName(3), tier: 'gold', hours: 140, last: 'last night', size: '86 GB', dx: 'VKD3D 2.14', status: 'ready', cover: img('hyd-metro', 600, 900), wide: img('hyd-metro-w', 1200, 500), verdict: 'Full ray tracing via VKD3D on 7900XTX. Two vendor extensions required, auto-applied.' },
  { id: 'civil-7', title: 'Civilization VII', store: 'Epic', runner: runnerName(4), tier: 'silver', hours: 8, last: 'last month', size: '33 GB', dx: 'DXVK 2.5.1', status: 'tweak', cover: img('hyd-civ', 600, 900), wide: img('hyd-civ-w', 1200, 500), verdict: 'Late-game turn times CPU-bound. MLPerf governor profile recommended.' },
  { id: 'outer-wilds', title: 'Outer Wilds', store: 'Steam', runner: runnerName(5), tier: 'platinum', hours: 26, last: 'a year ago', size: '9.8 GB', dx: 'native', status: 'ready', cover: img('hyd-outer', 600, 900), wide: img('hyd-outer-w', 1200, 500), verdict: 'Native. Perfect. Play it blind.' },
];

export const RUNNERS = [
  { name: 'GE-Proton10-4', kind: 'GE-Proton', path: '~/.steam/steam/compatibilitytools.d/GE-Proton10-4', games: 8, updated: '3 days ago', size: '412 MB', note: 'NVapi + FSR4 fixes backported; best all-round for DX12 titles.' },
  { name: 'Proton Experimental', kind: 'Valve', path: '~/.steam/steam/steamapps/common/Proton - Experimental', games: 6, updated: 'today', size: '398 MB', note: 'Valve auto-updating track. Newestwine server 10.7.' },
  { name: 'GE-Proton11-2', kind: 'GE-Proton', path: '~/.local/share/lutris/runners/proton/GE-Proton11-2', games: 4, updated: '1 week ago', size: '425 MB', note: 'Bleeding edge. VKD3D 2.15, ntsync default on.' },
  { name: 'Proton 9.0', kind: 'Valve', path: '~/.steam/steam/steamapps/common/Proton 9.0', games: 3, updated: '2 months ago', size: '381 MB', note: 'Locked stable branch for regression-averse setups.' },
  { name: 'System Wine 10.2', kind: 'Wine', path: '/usr/bin/wine', games: 2, updated: '3 weeks ago', size: '88 MB', note: 'Native win32 apps, not for modern games.' },
  { name: 'CachyOS Proton', kind: 'Distro', path: '/opt/proton-cachyos/proton', games: 1, updated: '8 days ago', size: '402 MB', note: 'XanMod kernel-tuned build, x86-64-v3 optimized.' },
];

export const FIXES = [
  { game: 'Helldivers 2', appId: '553850', tier: 'silver', reports: 1284, updated: '10.1', line: 'Bind EAC bridge + set PROTON_EAC_RUNTIME; nProtect blocks wine-staging.' },
  { game: 'Silent Hills 2', appId: '2124490', tier: 'silver', reports: 872, updated: '10.2', line: 'Stutter-fix collection + 4GB LAA patch; disable ray-fog on first boot.' },
  { game: 'Starfield', appId: '2300320', tier: 'bronze', reports: 2388, updated: '10.0', line: 'Framegen upscale-only; DLSS-FSR swizzle. Lutris recipe #4412 covers New Atlantis pacing.' },
  { game: 'GTA V Enhanced', appId: '1817090', tier: 'silver', reports: 1450, updated: '10.1', line: 'BattlEye bridge; xwayland cursor-grab workaround for menus.' },
  { game: 'Civilization VII', appId: '1295660', tier: 'silver', reports: 604, updated: '10.2', line: 'Governor profile MLPerf; turn timers recover 40%.' },
  { game: 'Cyberpunk 2077', appId: '1091500', tier: 'platinum', reports: 4210, updated: '10.2', line: 'Clean. Optional: RT via VKD3D with two vendor extension envs.' },
  { game: 'Kingdom Come II', appId: '1771300', tier: 'gold', reports: 921, updated: '10.1', line: 'PipeWire audio verb + registry fix; core-parking tweak for Kuttenberg.' },
  { game: 'Elden Ring: Nightreign', appId: '2338960', tier: 'gold', reports: 1105, updated: '10.2', line: 'EAC bridge + -vulkan flag; 5-min shader warm-up expected.' },
];

export const PIPELINE = [
  { chain: 'gamemoderun', env: 'GPU_BOOST=auto' },
  { chain: 'gamescope -w 2560 -h 1440 -r 144 -f', env: 'GAMESCOPE_CURSOR=auto' },
  { chain: 'mangohud --preset 4', env: 'MANGOHUD_CONFIG_FILE=~/.config/MangoHud/p4' },
  { chain: '~/.steam/compatibilitytools.d/GE-Proton10-4/proton run', env: 'STEAM_COMPAT_DATA_PATH=~/.local/share/hydra/prefixes/cyber-2077' },
  { chain: 'bin/x64/Cyberpunk2077.exe', env: 'PROTON_ENABLE_NVAPI=1' },
];

export const GAMES_NO_COVERS = GAMES.map((g) => ({ ...g, cover: g.cover, wide: g.wide }));
export { img };
// ---- batch 2: achievements, activity, screenshots ----
export const ACH_POOL = [
  { id: 'a1', name: 'First Contact', desc: 'Complete the prologue', rarity: '92.4%', kind: 'bronze' },
  { id: 'a2', name: 'Night Shift', desc: 'Play a session after midnight', rarity: '61.8%', kind: 'bronze' },
  { id: 'a3', name: 'Full Sync', desc: 'Reach maximum progression in a district', rarity: '34.1%', kind: 'silver' },
  { id: 'a4', name: 'Ghost Protocol', desc: 'Finish a mission undetected', rarity: '18.6%', kind: 'silver' },
  { id: 'a5', name: 'Collector', desc: 'Own every weapon variant', rarity: '9.7%', kind: 'gold' },
  { id: 'a6', name: 'Iron Will', desc: 'Beat the boss without taking damage', rarity: '2.3%', kind: 'gold' },
  { id: 'a7', name: 'Marathon', desc: 'Log 50 hours total', rarity: '22.9%', kind: 'silver' },
  { id: 'a8', name: 'Day One', desc: 'Play within 24h of release', rarity: '41.5%', kind: 'bronze' },
  { id: 'a9', name: 'Perfectionist', desc: 'All missions above S rank', rarity: '4.8%', kind: 'gold' },
  { id: 'a10', name: 'The Long Way', desc: 'Complete every side story', rarity: '12.2%', kind: 'silver' },
];

// deterministic pseudo-random unlocked set per game
export function achievementsFor(game) {
  const idx = Math.abs([...game.id].reduce((a, c) => a + c.charCodeAt(0), 0));
  const total = 28 + (idx % 25);
  const done = ACH_POOL.map((a, i) => ({
    ...a,
    unlocked: (idx + i * 7) % 10 < (game.tier === 'platinum' ? 7 : game.tier === 'gold' ? 5 : 3),
    date: ['12 Sep', '28 Sep', '30 Sep', '2 Oct', '4 Oct', '5 Oct'][(idx + i) % 6],
  }));
  const unlockedCount = done.filter((a) => a.unlocked).length;
  return { list: done, total, unlockedCount, score: Math.round((unlockedCount / ACH_POOL.length) * 100) };
}

export const ACTIVITY = [
  { ts: '21:42', type: 'session', game: 'Elden Ring: Nightreign', detail: '2h 18m played · Elden Ring: Nightreign', meta: 'GE-Proton10-4 · 4K · MangoHud' },
  { ts: '21:40', type: 'unlock', game: 'Elden Ring: Nightreign', detail: 'Achievement unlocked — “Night Shift”', meta: 'rarity 61.8% · +15 pts' },
  { ts: '20:07', type: 'fix', game: 'Helldivers 2', detail: 'Fix applied — EAC bridge re-bound after 10.1 patch', meta: 'source: ProtonDB report #4471' },
  { ts: '19:55', type: 'update', game: 'Proton Experimental', detail: 'Runner updated — Proton Experimental → wine 10.7', meta: ' Valle · auto' },
  { ts: '18:31', type: 'session', game: 'Hades II', detail: '46m played', meta: 'native · 120fps locked' },
  { ts: '17:58', type: 'download', game: 'Silent Hills 2 (2024)', detail: 'Update downloaded — 4.2 GB shader cache', meta: 'GOG · finished 17:58' },
  { ts: '16:20', type: 'add', game: 'Moonstone Island', detail: 'Added to library from Epic', meta: 'sync · legendary' },
  { ts: '15:44', type: 'session', game: 'Baldur’s Gate 3', detail: '1h 04m played', meta: 'native Vulkan · Act 3' },
  { ts: '14:02', type: 'tweak', game: 'Cyberpunk 2077', detail: 'Runner changed — GE-Proton11-2 → GE-Proton10-4', meta: 'reason: frame-pacing' },
  { ts: '11:15', type: 'unlock', game: 'Factorio: Space Age', detail: 'Achievement unlocked — “Marathon”', meta: 'rarity 22.9% · +30 pts' },
];

export const SCREENS = (game, n = 4) => Array.from({ length: n }, (_, i) => img(`${game.id}-scr-${i}`, 800, 450));

export const HOUR_BARS = [2, 5, 3, 8, 6, 9, 4, 7, 10, 6, 3, 5, 8, 4]; // fake 14-day play histogram

// ---- batch 3: proton-specific settings model ----
export const PROTON_FLAGS = [
  { key: 'PROTON_ENABLE_NVAPI', label: 'Enable NVAPI', desc: 'NVIDIA extension bridge — DLSS & RT', on: true },
  { key: 'PROTON_USE_WINED3D', label: 'Use WineD3D', desc: "Wine's OpenGL D3D instead of DXVK", on: false },
  { key: 'PROTON_FORCE_LARGE_ADDRESS_AWARE', label: 'Large address aware', desc: 'Force >2GB address space flag', on: true },
  { key: 'PROTON_USE_WOW64', label: 'WOW64 mode', desc: 'New 32-on-64, no 32-bit unix libs', on: false },
  { key: 'PROTON_NO_ESYNC', label: 'Disable Esync', desc: 'Turn off event-sync primitives', on: false },
  { key: 'PROTON_NO_FSYNC', label: 'Disable Fsync', desc: 'Turn off futex2 kernel sync', on: false },
  { key: 'PROTON_OLDGLString', label: 'Legacy GL strings', desc: 'Workaround for old OpenGL games', on: false },
  { key: 'PROTON_HEAP_DELAY_FREE', label: 'Heap delay free', desc: 'Reduce use-after-free in old titles', on: false },
];

export const SYNC_MODES = ['esync', 'fsync', 'ntsync', 'off'];
export const DXVK_VERSIONS = ['DXVK 2.5.1', 'DXVK 2.4', 'DXVK 2.3', 'WineD3D (no DXVK)'];
export const VKD3D_VERSIONS = ['VKD3D-Proton 2.14', 'VKD3D-Proton 2.13', 'off (DXVK only)'];
export const WINETRICKS = ['vcrun2022', 'corefonts', 'd3dcompiler_47', 'faudio', 'xact', 'dotnet48', 'd3dx9'];
export const HUD_PRESETS = ['off', 'minimal', 'full', 'fps only'];
export const UPSCALERS = ['native', 'FSR 3', 'NIS', 'Integer scaling'];

export function protonSettingsFor(game) {
  const idx = Math.abs([...game.id].reduce((a, c) => a + c.charCodeAt(0), 0));
  return {
    runner: game.runner,
    sync: SYNC_MODES[idx % 4],
    dxvk: DXVK_VERSIONS[idx % 3],
    vkd3d: VKD3D_VERSIONS[idx % 3],
    shaderCache: 'on',
    prefix: `~/.hydra/prefixes/${game.id}`,
    env: idx % 2 ? ['PROTON_LOG=1'] : [],
    winetricks: WINETRICKS.slice(0, 2 + (idx % 3)),
    flags: PROTON_FLAGS.map((f, i) => ({ ...f, on: idx % PROTON_FLAGS.length === i ? !f.on : f.on })),
  };
}
