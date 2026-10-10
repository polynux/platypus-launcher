import { app, BrowserWindow, dialog, ipcMain, Menu, net, protocol, session, shell, type IpcMainInvokeEvent } from 'electron';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { Core } from './core.ts';
import { APP_ORIGIN, csp, isAppURL, isArtPath, safePath, staticFile, webURL } from './security.ts';

// Where things are. run.mjs sets these; the defaults are the repo layout.
const repo = path.resolve(import.meta.dirname, '..', '..');
const coreBinary = process.env.PLATYPUS_CORE ?? path.join(repo, 'electron', '.build', 'platypus-core');
const distDir = path.resolve(process.env.PLATYPUS_FRONTEND_DIST ?? path.join(repo, 'frontend', 'dist'));
// Set when the page comes from a Vite dev server (hot reload, looser CSP).
const devURL = process.env.PLATYPUS_DEV_URL ? new URL(process.env.PLATYPUS_DEV_URL) : undefined;
const devPort = devURL ? Number(devURL.port) : undefined;

// Wayland where there is one, X11 otherwise.
app.commandLine.appendSwitch('ozone-platform-hint', 'auto');
// For driving the window from a test script (Playwright connectOverCDP); loopback only.
if (process.env.PLATYPUS_REMOTE_DEBUGGING_PORT) app.commandLine.appendSwitch('remote-debugging-port', process.env.PLATYPUS_REMOTE_DEBUGGING_PORT);

// The page is app://platypus/... : a secure, standard origin of our own, so
// nothing depends on file:// rules and the art URLs (/steam-art/...) the Go
// core hands out keep working unchanged.
protocol.registerSchemesAsPrivileged([{ scheme: 'app', privileges: { standard: true, secure: true, supportFetchAPI: true, stream: true } }]);

let core: Core | undefined;
let quitting = false;

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

function createWindow(): void {
  const win = new BrowserWindow({
    width: 1024,
    height: 768,
    title: 'Platypus',
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
  if (process.env.PLATYPUS_DEVTOOLS) {
    win.webContents.on('before-input-event', (_e, input) => {
      if (input.type === 'keyDown' && (input.key === 'F12' || (input.control && input.shift && input.key.toLowerCase() === 'i'))) win.webContents.toggleDevTools();
    });
  }
  void win.loadURL(`${APP_ORIGIN}/`);
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
  // The core stops games it started, the plugins and a last cloud sync; give it time.
  void core.close().finally(() => app.exit(0));
});

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
  createWindow();
});
