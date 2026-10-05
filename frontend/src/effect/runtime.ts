import { Effect } from 'effect';
import type { Game, ProtonVersion, PluginInfo } from '../types';

// Declare Wails bindings dynamically to work in both browser dev mode and compiled app
const getWailsApp = () => {
  return (window as any).go?.main?.App;
};

export const LauncherApi = {
  getGames: (): Effect.Effect<Game[], Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.GetGames) {
          return await app.GetGames();
        }
        // Dev fallback sample data
        return [
          {
            id: 'steam_730',
            store: 'steam',
            store_app_id: '730',
            title: 'Counter-Strike 2',
            cover_url: 'https://steamcdn-a.akamaihd.net/steam/apps/730/library_600x900_2x.jpg',
            banner_url: 'https://steamcdn-a.akamaihd.net/steam/apps/730/header.jpg',
            install_dir: '/home/polynux/.local/share/Steam/steamapps/common/Counter-Strike Global Offensive',
            executable: 'cs2.sh',
            is_installed: true,
            settings: {
              runner_id: 'auto',
              wine_prefix: '',
              launch_arguments: '-nojoy -novid',
              env_vars: {},
              use_gamemode: true,
              use_mangohud: true,
              use_gamescope: false,
              gamescope_w: 1920,
              gamescope_h: 1080,
              gamescope_r: 144,
              gamescope_args: '',
              eac_runtime: false,
              battleye_runtime: false,
              working_dir: '',
            },
            fix_info: {
              source: 'protondb',
              tier: 'platinum',
              description: 'Runs native Linux Vulkan build smoothly.',
              launch_args: '-nojoy -novid',
            },
          },
          {
            id: 'gog_witcher1',
            store: 'gog',
            store_app_id: '1207658924',
            title: 'The Witcher: Enhanced Edition',
            cover_url: 'https://images.igdb.com/igdb/image/upload/t_cover_big/co1r7h.jpg',
            banner_url: 'https://images.igdb.com/igdb/image/upload/t_screenshot_big/sc8b8c.jpg',
            install_dir: '/home/polynux/Games/GOG/The Witcher',
            executable: 'system/witcher.exe',
            is_installed: true,
            settings: {
              runner_id: 'compat:GE-Proton11-3',
              wine_prefix: '',
              launch_arguments: '',
              env_vars: {},
              use_gamemode: true,
              use_mangohud: false,
              use_gamescope: false,
              gamescope_w: 1920,
              gamescope_h: 1080,
              gamescope_r: 144,
              gamescope_args: '',
              eac_runtime: false,
              battleye_runtime: false,
              working_dir: '',
            },
            fix_info: {
              source: 'clarity/lutris',
              tier: 'gold',
              description: 'Requires GOG registry fix and d3dx9 winetricks verb to prevent silent crash on start.',
              winetricks: ['d3dx9', 'corefonts'],
            },
          },
          {
            id: 'epic_cyberpunk',
            store: 'epic',
            store_app_id: 'Ginger',
            title: 'Cyberpunk 2077',
            cover_url: 'https://images.igdb.com/igdb/image/upload/t_cover_big/co2m35.jpg',
            banner_url: 'https://images.igdb.com/igdb/image/upload/t_screenshot_big/sc69n2.jpg',
            install_dir: '/home/polynux/Games/Epic/Cyberpunk2077',
            executable: 'bin/x64/Cyberpunk2077.exe',
            is_installed: true,
            settings: {
              runner_id: 'steam:Proton - Experimental',
              wine_prefix: '',
              launch_arguments: '--launcher-skip',
              env_vars: { PROTON_ENABLE_NVAPI: '1' },
              use_gamemode: true,
              use_mangohud: true,
              use_gamescope: true,
              gamescope_w: 2560,
              gamescope_h: 1440,
              gamescope_r: 144,
              gamescope_args: '-f --adaptive-sync',
              eac_runtime: false,
              battleye_runtime: false,
              working_dir: '',
            },
            fix_info: {
              source: 'protondb',
              tier: 'gold',
              description: 'ProtonDB Tier: Gold. Direct3D12 and Ray Tracing supported via VKD3D-Proton.',
            },
          },
          {
            id: 'custom_fallout_london',
            store: 'custom',
            store_app_id: 'fallout_london',
            title: 'Fallout: London',
            cover_url: 'https://images.igdb.com/igdb/image/upload/t_cover_big/co8fgv.jpg',
            banner_url: 'https://images.igdb.com/igdb/image/upload/t_screenshot_big/scfp1a.jpg',
            install_dir: '/home/polynux/Games/Fallout London',
            executable: 'f4se_loader.exe',
            is_installed: true,
            settings: {
              runner_id: 'compat:GE-Proton11-3',
              wine_prefix: '',
              launch_arguments: '',
              env_vars: {},
              use_gamemode: true,
              use_mangohud: false,
              use_gamescope: false,
              gamescope_w: 1920,
              gamescope_h: 1080,
              gamescope_r: 144,
              gamescope_args: '',
              eac_runtime: false,
              battleye_runtime: false,
              working_dir: '',
            },
            fix_info: {
              source: 'lutris',
              tier: 'gold',
              description: 'Apply missing GOG post-install finishing steps and xaudio overrides for black screen fix.',
              winetricks: ['faudio', 'xact'],
              dll_overrides: { xaudio2_7: 'n,b' },
            },
          },
        ];
      },
      catch: (err) => new Error(String(err)),
    }),

  getProtonVersions: (): Effect.Effect<ProtonVersion[], Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.GetProtonVersions) {
          return await app.GetProtonVersions();
        }
        return [
          { id: 'steam:Proton - Experimental', name: 'Proton - Experimental', path: '/home/polynux/.local/share/Steam/steamapps/common/Proton - Experimental/proton', is_system: false, is_custom: false, is_default: true },
          { id: 'compat:GE-Proton11-3', name: 'GE-Proton11-3 (Custom/GE)', path: '/home/polynux/.local/share/lutris/runners/wine/GE-Proton11-3/bin/wine', is_system: false, is_custom: true, is_default: false },
          { id: 'compat:GE-Proton11-1', name: 'GE-Proton11-1 (Custom/GE)', path: '/home/polynux/.local/share/lutris/runners/wine/GE-Proton11-1/bin/wine', is_system: false, is_custom: true, is_default: false },
          { id: 'system:wine', name: 'System Wine (/usr/bin/wine)', path: '/usr/bin/wine', is_system: true, is_custom: false, is_default: false },
        ];
      },
      catch: (err) => new Error(String(err)),
    }),

  getPlugins: (): Effect.Effect<PluginInfo[], Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.ListPlugins) {
          return await app.ListPlugins();
        }
        return [
          { id: 'store-steam', name: 'Steam Library Integrator', author: 'Hydra Team', version: '1.0.0', description: 'Automatically parses local Steam libraries (libraryfolders.vdf & appmanifest_*.acf)', enabled: true, store: 'steam' },
          { id: 'store-epic', name: 'Epic Games Connector (Legendary)', author: 'Hydra Team', version: '0.9.2', description: 'Integrates with Legendary CLI to sync Epic Games Store library and launch via Proton', enabled: true, store: 'epic' },
          { id: 'store-gog', name: 'GOG Galaxy Connector (Gogdl)', author: 'Hydra Team', version: '0.8.5', description: 'Syncs DRM-free GOG library, cloud saves and automatic Linux Wine wrappers', enabled: true, store: 'gog' },
          { id: 'store-ubisoft', name: 'Ubisoft Connect Bridge', author: 'Hydra Team', version: '0.5.1', description: 'Manages Ubisoft Connect launcher prefix and authenticated token launches', enabled: false, store: 'ubisoft' },
          { id: 'fix-protondb', name: 'ProtonDB Compatibility Feed', author: 'Community', version: '1.2.0', description: 'Fetches live user reports, ratings, and launch tweaks from ProtonDB API', enabled: true, store: 'fixes' },
          { id: 'fix-lutris', name: 'Lutris Community Recipes', author: 'Lutris Team', version: '1.0.0', description: 'Imports community winetricks scripts and registry tweaks directly from lutris.net', enabled: true, store: 'fixes' },
        ];
      },
      catch: (err) => new Error(String(err)),
    }),

  togglePlugin: (id: string, enabled: boolean): Effect.Effect<boolean, Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.TogglePlugin) {
          return await app.TogglePlugin(id, enabled);
        }
        return true;
      },
      catch: (err) => new Error(String(err)),
    }),

  launchGame: (game: Game): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.LaunchGame) {
          return await app.LaunchGame(game);
        }
        return `Launched ${game.title} (Simulated PID 41285)`;
      },
      catch: (err) => new Error(String(err)),
    }),

  previewCommand: (game: Game): Effect.Effect<string, Error> =>
    Effect.tryPromise({
      try: async () => {
        const app = getWailsApp();
        if (app && app.PreviewLaunchCommand) {
          const res = await app.PreviewLaunchCommand(game);
          return typeof res === 'string' ? res : JSON.stringify(res);
        }
        // Fallback preview
        const wrappers: string[] = [];
        if (game.settings.use_gamemode) wrappers.push('gamemoderun');
        if (game.settings.use_gamescope) wrappers.push(`gamescope -w ${game.settings.gamescope_w || 1920} -h ${game.settings.gamescope_h || 1080} -r ${game.settings.gamescope_r || 144} --`);
        if (game.settings.use_mangohud) wrappers.push('mangohud');
        
        const runnerPath = game.settings.runner_id || 'proton';
        const target = game.executable || `steam -applaunch ${game.store_app_id}`;
        return `WINEPREFIX=~/.local/share/hydra/prefixes/${game.id} ${wrappers.join(' ')} ${runnerPath} run ${target} ${game.settings.launch_arguments || ''}`.trim();
      },
      catch: (err) => new Error(String(err)),
    }),
};
