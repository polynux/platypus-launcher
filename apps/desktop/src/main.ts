import { app, BrowserWindow, dialog, ipcMain, Menu, net, Notification, protocol, screen, session, shell, Tray, type IpcMainInvokeEvent } from 'electron';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { createChatNotifier, validNotifyMessage } from './chatNotify.ts';
import { Core, errorData } from './core.ts';
import { isMethodAllowed, notAllowedMessage } from './allowlist.ts';
import { approvalDetail, approvalMessage, runApproveGate, testApproveAnswer, type ApprovalPlugin } from './approve.ts';
import { resolveShellLanguage, shellText } from './locale.ts';
import { resolvePaths } from './paths.ts';
import { restoreWindow, TrayState, type TrayGame } from './tray.ts';
import { coreEnv, resolveUserData } from './userdata.ts';
import { APP_ORIGIN, csp, isAppURL, isArtPath, safePath, staticFile, webURL } from './security.ts';
import { isReloadOrInspect } from './shortcuts.ts';
import { validPluginId, validStoreBounds, type StorePluginInfo } from './store.ts';
import { StoreViews } from './storeview.ts';
import { clampState, DEFAULT_STATE, loadState, saveState } from './window-state.ts';

// Where things are: next to the app in production, the repo build outputs in
// development. run.mjs and the tests override with the environment.
const repo = path.resolve(import.meta.dirname, '..', '..', '..');
const runtime = resolvePaths({
  packaged: app.isPackaged,
  resourcesPath: process.resourcesPath,
  exeDir: path.dirname(app.getPath('exe')),
  env: process.env,
  repoRoot: repo,
});
const coreBinary = runtime.coreBinary;
const distDir = runtime.frontendDist;
// Window state + Chromium profile (issue #41): redirected config/data moves
// userData with it, before app.ready; otherwise Electron's default (which
// already follows XDG_CONFIG_HOME). PLATYPUS_* reaches the core through
// coreEnv at spawn.
app.setPath('userData', resolveUserData(process.env, app.getPath('userData')));
// Set when the page comes from a Vite dev server (hot reload, looser CSP).
const devURL = !app.isPackaged && process.env.PLATYPUS_DEV_URL ? new URL(process.env.PLATYPUS_DEV_URL) : undefined;
const devPort = devURL ? Number(devURL.port) : undefined;

// Wayland where there is one, X11 otherwise.
app.commandLine.appendSwitch('ozone-platform-hint', 'auto');
// For driving the window from a test script (Playwright over CDP).
// Development only, loopback only: a packaged app never opens a debugger,
// so there is nothing a child process could inherit or a neighbor could join.
if (!app.isPackaged && process.env.PLATYPUS_REMOTE_DEBUGGING_PORT) {
  app.commandLine.appendSwitch('remote-debugging-port', process.env.PLATYPUS_REMOTE_DEBUGGING_PORT);
  app.commandLine.appendSwitch('remote-debugging-address', '127.0.0.1');
}

// The page is app://platypus/... : a secure, standard origin of our own, so
// nothing depends on file:// rules and the art URLs (/steam-art/...) the Go
// core hands out keep working unchanged.
protocol.registerSchemesAsPrivileged([{ scheme: 'app', privileges: { standard: true, secure: true, supportFetchAPI: true, stream: true } }]);

let core: Core | undefined;
let quitting = false;
let storesRef: StoreViews | undefined;
// Minimize to tray while a game runs (issue #49): the running games, the
// switch (read from the core, refreshed on every session event), and the
// tray icon itself — which exists only while the window is hidden for a
// running game, never permanently.
const trayState = new TrayState();
let tray: Tray | null = null;
const gameTitles = new Map<string, string>();
let libraryTitlesAt = 0;

async function refreshTraySetting(): Promise<void> {
  try {
    const s = (await core?.call('GetLauncherSettings', [])) as { minimize_to_tray?: unknown } | undefined;
    trayState.settingOn = s?.minimize_to_tray === true;
  } catch {
    // An unreadable setting keeps the last one (off until the first read).
  }
}

/** Best-effort game titles for the tray menu (game ids when unknown). */
async function resolveGameTitle(gameId: string): Promise<string> {
  const known = gameTitles.get(gameId);
  if (known) return known;
  // The library read is cached in the core (issue #27), so this costs a
  // store round trip at most once every few minutes.
  if (Date.now() - libraryTitlesAt > 2 * 60 * 1000) {
    try {
      const lib = (await core?.call('GetLibrary', [])) as { games?: { id?: string; title?: string }[] } | undefined;
      libraryTitlesAt = Date.now();
      for (const g of lib?.games ?? []) {
        if (g?.id) gameTitles.set(g.id, g.title || g.id);
      }
    } catch {
      // Ids stay as labels.
    }
  }
  return gameTitles.get(gameId) ?? gameId;
}

function trayText(): { show: string; stop: (title: string) => string; quit: string } {
  return {
    show: shellText(shellLang, 'trayShow'),
    stop: (title: string) => shellText(shellLang, 'trayStop', { title }),
    quit: shellText(shellLang, 'trayQuit'),
  };
}

function trayGames(): TrayGame[] {
  return trayState.games().map((g) => ({ id: g.id, title: gameTitles.get(g.id) ?? g.title }));
}

/** (Re)builds the tray menu: Show Platypus, the running game(s) with Stop, Quit. */
function updateTrayMenu(): void {
  if (!tray) return;
  const games = trayGames();
  const text = trayText();
  const template: Electron.MenuItemConstructorOptions[] = [
    {
      label: text.show,
      click: () => {
        const w = BrowserWindow.getAllWindows()[0];
        if (w) restoreWindow(w);
      },
    },
  ];
  if (games.length > 0) template.push({ type: 'separator' });
  for (const g of games) {
    const gameId = g.id;
    template.push({
      label: text.stop(g.title || gameId),
      click: () => {
        void core?.call('StopGame', [gameId]).catch(() => {});
      },
    });
  }
  template.push({ type: 'separator' });
  template.push({ label: text.quit, click: () => app.quit() });
  tray.setContextMenu(Menu.buildFromTemplate(template));
  const titles = games.map((g) => g.title || g.id).join(', ');
  tray.setToolTip(titles ? shellText(shellLang, 'trayTooltipGame', { title: titles }) : shellText(shellLang, 'trayTooltip'));
}

/**
 * Creates the tray icon for the hidden window. Tray support can't be
 * detected reliably (GNOME without AppIndicator shows nothing and throws
 * nothing deterministic): a throwing constructor — or PLATYPUS_NO_TRAY=1 —
 * marks the host as tray-less, and the caller minimizes instead of hiding.
 */
function ensureTray(): boolean {
  if (tray) return true;
  if (process.env.PLATYPUS_NO_TRAY === '1') {
    trayState.trayAvailable = false;
    return false;
  }
  try {
    tray = new Tray(runtime.iconFile);
  } catch {
    tray = null;
    trayState.trayAvailable = false;
    return false;
  }
  tray.on('click', () => {
    const w = BrowserWindow.getAllWindows()[0];
    if (w) restoreWindow(w);
  });
  updateTrayMenu();
  return true;
}

function destroyTray(): void {
  tray?.destroy();
  tray = null;
}

function hideWindowForGame(win: BrowserWindow): void {
  if (ensureTray()) {
    updateTrayMenu();
    win.hide();
  } else {
    win.minimize();
  }
}

/**
 * Shows the window again when the last game exits — without stealing focus:
 * the user may be in the game that just exited, on another app, or on
 * another desktop. showInactive() maps a hidden window without activating
 * it; a minimized (fallback) window restores the same way. Explicit user
 * actions (tray click, Show, second launch) focus instead (restoreWindow).
 */
function showWindowQuiet(win: BrowserWindow): void {
  if (!win.isVisible()) win.showInactive();
  else if (win.isMinimized()) win.restore();
}

async function handleSessionEvent(data: unknown): Promise<void> {
  const e = data as { game_id?: string; state?: string } | null;
  const gameId = typeof e?.game_id === 'string' ? e.game_id : '';
  const state = typeof e?.state === 'string' ? e.state : '';
  if (!gameId || !state) return;
  await refreshTraySetting();
  if (state === 'stopped' || state === 'failed') {
    gameTitles.delete(gameId);
    if (trayState.onSessionEnd(gameId)) {
      destroyTray();
      const w = BrowserWindow.getAllWindows()[0];
      if (w) showWindowQuiet(w);
    } else {
      updateTrayMenu();
    }
    return;
  }
  const title = await resolveGameTitle(gameId);
  const action = trayState.onSessionEvent(gameId, title, state);
  const w = BrowserWindow.getAllWindows()[0];
  if (!w) return;
  if (action === 'hide' || action === 'minimize') hideWindowForGame(w);
  else updateTrayMenu();
}
// The shell's own language (issue #39): resolved from the launcher setting
// once the core answers, system languages until then.
let shellLang = resolveShellLanguage('', app.getPreferredSystemLanguages());

async function refreshShellLang(): Promise<void> {
  try {
    const s = (await core?.call('GetLauncherSettings', [])) as { language?: unknown } | undefined;
    shellLang = resolveShellLanguage(
      typeof s?.language === 'string' ? s.language : '',
      app.getPreferredSystemLanguages(),
    );
  } catch {
    // An unreadable setting keeps the system-resolved language.
  }
}

if (!app.requestSingleInstanceLock()) app.quit();

async function startCore(): Promise<Core> {
  const c = new Core(coreBinary, [], coreEnv(process.env));
  c.onEvent = (name, data) => {
    for (const w of BrowserWindow.getAllWindows()) w.webContents.send('platypus:event', name, data);
    if (name === 'session') void handleSessionEvent(data);
  };
  c.onCrash = (code) => {
    if (quitting) return;
    dialog.showErrorBox(shellText(shellLang, 'appName'), shellText(shellLang, 'coreCrashed', { code: code ?? '?' }));
    app.exit(1);
  };
  c.handlers.set('dialog.openFile', async (p) => {
    const d = p as { title?: string; default?: string; filters?: { name: string; extensions: string[] }[] };
    const r = await dialog.showOpenDialog(parentWindow(), { title: d.title, defaultPath: d.default || undefined, properties: ['openFile'], filters: d.filters });
    return r.canceled ? '' : (r.filePaths[0] ?? '');
  });
  c.handlers.set('dialog.openDirectory', async (p) => {
    const d = p as { title?: string; default?: string; canCreate?: boolean };
    const r = await dialog.showOpenDialog(parentWindow(), {
      title: d.title,
      defaultPath: d.default || undefined,
      properties: d.canCreate ? ['openDirectory', 'createDirectory'] : ['openDirectory'],
    });
    return r.canceled ? '' : (r.filePaths[0] ?? '');
  });
  await c.initialize();
  // The core resolves the effective locale from the setting plus these
  // desktop languages (issue #39, phase 4). Best-effort: an old core
  // without the method still starts.
  try {
    await c.call('SetSystemLanguages', [app.getPreferredSystemLanguages()]);
  } catch {
    // Old core or a core that failed to start: startCore's caller reports it.
  }
  return c;
}

function parentWindow(): BrowserWindow {
  return BrowserWindow.getFocusedWindow() ?? BrowserWindow.getAllWindows()[0]!;
}

/**
 * Desktop notifications for chat (issue #12): the page asks, main decides and
 * shows. The page itself has no notification power (permissions are refused
 * below); main validates the payload, checks focus and the launcher switch,
 * and coalesces bursts per conversation. Clicking focuses the window and opens
 * the conversation through the page's own navigation (a `platypus:chat-open`
 * event the UI handles like any other open).
 */
const chatNotifier = createChatNotifier({
  show: (title, body, onClick) => {
    const n = new Notification({ title, body });
    n.on('click', onClick);
    n.show();
    return { close: () => n.close() };
  },
});

function openChatConversation(plugin: string, friendId: string): void {
  const w = BrowserWindow.getFocusedWindow() ?? BrowserWindow.getAllWindows()[0];
  if (!w) return;
  if (w.isMinimized()) w.restore();
  w.focus();
  w.webContents.send('platypus:chat-open', { plugin, friendId });
}

function registerChatIPC(): void {
  ipcMain.handle('platypus:notify-chat', async (e, raw: unknown) => {
    if (!fromApp(e)) return { ok: false, error: 'refused: not the app page' };
    if (!validNotifyMessage(raw)) return { ok: false, error: 'refused: not a chat notification' };
    const focused = BrowserWindow.getAllWindows().some((w) => w.isFocused());
    // The switch is on by default; an unreadable setting keeps it.
    let settingOn = true;
    try {
      const s = (await core?.call('GetLauncherSettings', [])) as { chat_desktop_notify?: unknown } | undefined;
      settingOn = s?.chat_desktop_notify !== false;
    } catch {
      settingOn = true;
    }
    const outcome = chatNotifier.notify(raw, { focused, settingOn, lang: shellLang }, () =>
      openChatConversation(raw.plugin, raw.friendId),
    );
    return { ok: true, shown: outcome !== 'dropped' };
  });
}

/** The shell session for the UI's language choice (issue #39): the OS
 *  language list and whether this is a dev session. Only the app page may ask. */
function registerSessionIPC(): void {
  ipcMain.handle('platypus:session-info', (e) => {
    if (!fromApp(e)) return { ok: false, error: 'refused: not the app page' };
    return {
      ok: true,
      result: {
        systemLanguages: app.getPreferredSystemLanguages(),
        devtools: !app.isPackaged && process.env.PLATYPUS_DEVTOOLS === '1',
      },
    };
  });
}

/** Only the app's own page may talk to the core. */
function fromApp(e: IpcMainInvokeEvent | Electron.IpcMainEvent): boolean {
  return isAppURL(e.senderFrame?.url ?? '');
}

function registerIPC(): void {
  ipcMain.handle('platypus:invoke', async (e, method: unknown, args: unknown) => {
    if (!fromApp(e)) return { ok: false, error: 'refused: not the app page' };
    if (typeof method !== 'string' || !isMethodAllowed('app', method) || !Array.isArray(args)) {
      return { ok: false, error: notAllowedMessage(method) };
    }
    // Approving a plugin lets it run code (issue #89): the page may ask,
    // but only a native confirmation parented to the window grants it.
    if (method === 'ApprovePlugin') return approvePlugin(e, args);
    if (!core) return { ok: false, error: notAllowedMessage(method) };
    try {
      return { ok: true, result: await core.call(method, args) };
    } catch (err) {
      // A coded core error (issue #39, phase 3) keeps its translatable
      // payload across the IPC: the page looks up errors:<code>.
      const data = errorData(err);
      const error = err instanceof Error ? err.message : String(err);
      return data === undefined ? { ok: false, error } : { ok: false, error, data };
    }
  });
  ipcMain.on('platypus:openExternal', (e, raw: unknown) => {
    if (!fromApp(e)) return;
    const url = webURL(raw);
    if (url) void shell.openExternal(url);
  });
}

/**
 * ApprovePlugin through the native confirmation (issue #89). The dialog
 * names the plugin (name + id), its folder and its declared permissions,
 * looked up on the core; it forwards to the core only on an explicit
 * Approve. The default button is Cancel, and a decline never reaches the
 * core: the page gets a coded error it can show.
 */
async function approvePlugin(e: IpcMainInvokeEvent, args: unknown[]): Promise<unknown> {
  const win = BrowserWindow.fromWebContents(e.sender) ?? parentWindow();
  try {
    return await runApproveGate(args, {
      listPlugins: async () => ((await core?.call('ListStorePlugins', [])) ?? []) as ApprovalPlugin[],
      showDialog: async (plugin) => {
        const r = await dialog.showMessageBox(win, {
          type: 'question',
          buttons: [shellText(shellLang, 'approvePluginApprove'), shellText(shellLang, 'approvePluginCancel')],
          defaultId: 1,
          cancelId: 1,
          title: shellText(shellLang, 'approvePluginTitle'),
          message: approvalMessage(plugin, shellLang),
          detail: approvalDetail(plugin, shellLang),
        });
        return r.response === 0;
      },
      forward: (a) => core!.call('ApprovePlugin', a),
      // Test runs only: a packaged build always shows the dialog, like the
      // other PLATYPUS_* switches that only work unpackaged.
      testAnswer: app.isPackaged ? undefined : testApproveAnswer(),
    });
  } catch (err) {
    const data = errorData(err);
    const error = err instanceof Error ? err.message : String(err);
    return data === undefined ? { ok: false, error } : { ok: false, error, data };
  }
}

/**
 * The embedded store view's bridge (issue #17): open / bounds / history /
 * close, validated here like the core calls. Plugin and address checks live
 * in store.ts (pure, tested); only the app page may call.
 */
function registerStoreIPC(stores: StoreViews): void {
  ipcMain.handle('platypus:store-open', async (e, pluginId: unknown, url: unknown) => {
    if (!fromApp(e)) return { ok: false, error: 'refused: not the app page' };
    if (!validPluginId(pluginId)) return { ok: false, error: `refused: not a plugin id` };
    try {
      await stores.open(pluginId as string, url);
      return { ok: true };
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) };
    }
  });
  const current = (fn: () => void) => (e: Electron.IpcMainEvent, ...args: unknown[]) => {
    void args;
    if (!fromApp(e)) return;
    fn();
  };
  ipcMain.on('platypus:store-bounds', (e, raw: unknown) => {
    if (!fromApp(e)) return;
    const bounds = validStoreBounds(raw);
    if (bounds) stores.setBounds(bounds);
  });
  ipcMain.on('platypus:store-back', current(() => stores.back()));
  ipcMain.on('platypus:store-forward', current(() => stores.forward()));
  ipcMain.on('platypus:store-reload', current(() => stores.reload()));
  // The close names its plugin: a stale close racing the next store's open
  // over a separate IPC channel must not hide the newly shown view.
  ipcMain.on('platypus:store-close', (e, pluginId: unknown) => {
    if (!fromApp(e)) return;
    stores.close(pluginId);
  });
}

/** Serves app://platypus: the built frontend (or the dev server) and, through the core, the art. */
function registerAppProtocol(): void {
  const policy = csp(devPort);
  const withPolicy = (r: Response): Response => {
    const headers = new Headers(r.headers);
    headers.set('Content-Security-Policy', policy);
    headers.set('X-Content-Type-Options', 'nosniff');
    return new Response(r.body, { status: r.status, statusText: r.statusText, headers });
  };
  protocol.handle('app', async (req) => {
    const url = new URL(req.url);
    if (url.host !== 'platypus' || req.method !== 'GET') return new Response('not found', { status: 404 });
    if (isArtPath(url.pathname)) {
      if (!core?.info.artURL || !safePath(decodeURIComponentSafe(url.pathname))) return new Response('not found', { status: 404 });
      const r = await net.fetch(core.info.artURL + url.pathname + url.search, { headers: { 'X-Platypus-Token': core.info.artToken } });
      return withPolicy(r);
    }
    if (devURL) return withPolicy(await net.fetch(new URL(url.pathname + url.search, devURL).href));
    const file = staticFile(distDir, url.pathname);
    if (!file) return new Response('not found', { status: 404 });
    return withPolicy(await net.fetch(pathToFileURL(file).href));
  });
}

function decodeURIComponentSafe(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return '/..'; // unsafe on purpose
  }
}

function createWindow(): StoreViews {
  const statePath = path.join(app.getPath('userData'), 'window-state.json');
  const saved = loadState(statePath);
  const state = clampState(saved ?? DEFAULT_STATE, screen.getAllDisplays().map((d) => d.workArea));
  const { maximized, ...bounds } = state;
  const win = new BrowserWindow({
    ...bounds,
    title: 'Platypus',
    icon: runtime.iconFile,
    backgroundColor: '#1b2636',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(import.meta.dirname, 'preload.cjs'),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      // Kept on (issue #49): a hidden or unfocused renderer must not burn
      // cycles on timers and animation frames while a game runs.
      backgroundThrottling: true,
      webSecurity: true,
      spellcheck: false,
    },
  });
  // Links open in the user's browser, never in the app.
  win.webContents.setWindowOpenHandler(({ url }) => {
    const external = webURL(url);
    if (external) void shell.openExternal(external);
    return { action: 'deny' };
  });
  win.webContents.on('will-navigate', (e, url) => {
    if (isAppURL(url)) return;
    e.preventDefault();
    const external = webURL(url);
    if (external) void shell.openExternal(external);
  });
  win.webContents.on('will-attach-webview', (e) => e.preventDefault());
  // Reload and DevTools keys: development keeps them (hot reload, F12 with
  // PLATYPUS_DEVTOOLS=1); production answers neither, so a stray Ctrl+R can
  // never drop the session and no shortcut opens DevTools.
  win.webContents.on('before-input-event', (e, input) => {
    if (input.type !== 'keyDown') return;
    if (!app.isPackaged && process.env.PLATYPUS_DEVTOOLS) {
      if (input.key === 'F12' || (input.control && input.shift && input.key.toLowerCase() === 'i')) win.webContents.toggleDevTools();
      return;
    }
    if (app.isPackaged && isReloadOrInspect(input)) e.preventDefault();
  });
  // Remember the window for the next run (clamped to the displays then).
  // Saved on close for a window the user closes, and again in before-quit:
  // app.exit() never delivers close, so a SIGTERM quit would lose it.
  win.on('close', () => saveWindowState(statePath, win));
  // The embedded store view (issue #17): one native view per store plugin,
  // over the UI's content area. The open call checks the plugin against the
  // core's list: known, enabled (ready), with a storefront.
  const stores = new StoreViews(win, async (): Promise<StorePluginInfo[]> => {
    if (!core) return [];
    try {
      return (await core.call('ListStorePlugins', [])) as StorePluginInfo[];
    } catch {
      return [];
    }
  });
  // The embedded store view keeps its place over the content area.
  win.on('resize', () => stores.relayout());
  win.on('closed', () => stores.destroy());
  if (maximized) win.maximize();
  void win.loadURL(`${APP_ORIGIN}/`);
  return stores;
}

function saveWindowState(statePath: string, win: BrowserWindow): void {
  saveState(statePath, { ...win.getBounds(), maximized: win.isMaximized() });
}

app.on('second-instance', () => {
  // An explicit user action: a hidden (tray) window shows, a minimized one
  // restores, then it focuses — so the window is never unreachable.
  const w = BrowserWindow.getAllWindows()[0];
  if (w) restoreWindow(w);
});

app.on('before-quit', (e) => {
  if (quitting || !core) return;
  quitting = true;
  e.preventDefault();
  destroyTray();
  for (const w of BrowserWindow.getAllWindows()) {
    saveWindowState(path.join(app.getPath('userData'), 'window-state.json'), w);
  }
  storesRef?.destroy();
  // The core stops games it started, the plugins and a last cloud sync; give it time.
  void core.close().finally(() => app.exit(0));
});

// A bare SIGTERM/SIGINT would kill Electron at once and strand the core's
// children (games, plugin helpers). Quitting instead runs before-quit, which
// shuts the core down cleanly first.
for (const sig of ['SIGTERM', 'SIGINT'] as const) {
  process.on(sig, () => {
    if (!quitting) app.quit();
  });
}

app.on('window-all-closed', () => app.quit());

void app.whenReady().then(async () => {
  Menu.setApplicationMenu(null);
  // Nothing in the page asks for a permission (camera, notifications...); refuse them all.
  session.defaultSession.setPermissionRequestHandler((_wc, _perm, cb) => cb(false));
  try {
    core = await startCore();
  } catch (err) {
    dialog.showErrorBox(
      shellText(shellLang, 'appName'),
      shellText(shellLang, 'coreStartFailed', { error: err instanceof Error ? err.message : String(err) }),
    );
    app.exit(1);
    return;
  }
  registerIPC();
  registerSessionIPC();
  registerChatIPC();
  await refreshShellLang();
  await refreshTraySetting();
  registerAppProtocol();
  const stores = createWindow();
  registerStoreIPC(stores);
  // Store views never get the platypus bridge; their events reach the app
  // page as store events on its own channel.
  stores.onEvent((ev) => {
    for (const w of BrowserWindow.getAllWindows()) w.webContents.send('platypus:store-event', ev);
  });
  storesRef = stores;
});
