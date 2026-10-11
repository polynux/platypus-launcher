import { app, BrowserWindow, dialog, ipcMain, Menu, net, protocol, screen, session, shell, type IpcMainInvokeEvent } from 'electron';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { Core } from './core.ts';
import { resolvePaths } from './paths.ts';
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

if (!app.requestSingleInstanceLock()) app.quit();

async function startCore(): Promise<Core> {
  const c = new Core(coreBinary);
  c.onEvent = (name, data) => {
    for (const w of BrowserWindow.getAllWindows()) w.webContents.send('platypus:event', name, data);
  };
  c.onCrash = (code) => {
    if (quitting) return;
    dialog.showErrorBox('Platypus', `The Platypus core stopped unexpectedly (exit ${code}). See the terminal for its log.`);
    app.exit(1);
  };
  c.handlers.set('dialog.openFile', async (p) => {
    const d = p as { title?: string; filters?: { name: string; extensions: string[] }[] };
    const r = await dialog.showOpenDialog(parentWindow(), { title: d.title, properties: ['openFile'], filters: d.filters });
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
  return c;
}

function parentWindow(): BrowserWindow {
  return BrowserWindow.getFocusedWindow() ?? BrowserWindow.getAllWindows()[0]!;
}

/** Only the app's own page may talk to the core. */
function fromApp(e: IpcMainInvokeEvent | Electron.IpcMainEvent): boolean {
  return isAppURL(e.senderFrame?.url ?? '');
}

function registerIPC(): void {
  ipcMain.handle('platypus:invoke', async (e, method: unknown, args: unknown) => {
    if (!fromApp(e)) return { ok: false, error: 'refused: not the app page' };
    if (typeof method !== 'string' || !core?.methods.has(method) || !Array.isArray(args)) {
      return { ok: false, error: `refused: ${String(method)} is not a core method` };
    }
    try {
      return { ok: true, result: await core.call(method, args) };
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : String(err) };
    }
  });
  ipcMain.on('platypus:openExternal', (e, raw: unknown) => {
    if (!fromApp(e)) return;
    const url = webURL(raw);
    if (url) void shell.openExternal(url);
  });
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
  ipcMain.on('platypus:store-close', current(() => stores.close()));
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
  const w = BrowserWindow.getAllWindows()[0];
  if (w) {
    if (w.isMinimized()) w.restore();
    w.focus();
  }
});

app.on('before-quit', (e) => {
  if (quitting || !core) return;
  quitting = true;
  e.preventDefault();
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
    dialog.showErrorBox('Platypus', `Could not start the Platypus core:\n${err instanceof Error ? err.message : String(err)}`);
    app.exit(1);
    return;
  }
  registerIPC();
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
