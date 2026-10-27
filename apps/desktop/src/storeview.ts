// The embedded store view (issue #17): one WebContentsView per store plugin,
// placed over the UI's content area. It is NOT the app page: a separate
// persistent session per plugin (logins stay, isolated from the launcher and
// from each other), no preload, no bridge, permissions denied; downloads
// cancelled; popups inside the store's domains open in a child window of the
// same session (login popups); anything outside the plugin's declared
// storefront.domains never loads — the toolbar offers to open it in the
// user's browser instead. Non-http(s) navigation never loads.
//
// A hidden view stays alive (fast return; its session keeps the login), and
// is destroyed only when the window closes with the app. Leaving the view
// hides it; the UI invalidates the plugin's library through the core
// (InvalidateLibrary) so purchases appear.
import { app, BrowserWindow, session, WebContentsView } from 'electron';
import {
  applyStoreViewBounds,
  blockedHost,
  canOpenStore,
  explicitStoreURL,
  navigationAllowed,
  popupURL,
  shouldCloseView,
  storePartition,
  type StorePluginInfo,
  type StoreRect,
} from './store.ts';

export interface StoreEvent {
  /**
   * 'navigated' (url, history, title), 'loading' (spinner state), or
   * 'blocked' (an http(s) navigation to an outside host was refused; url is
   * the attempted address, the toolbar offers to open it in the browser), or
   * 'failed' (the main-frame load failed; error names it).
   */
  kind: 'navigated' | 'loading' | 'blocked' | 'failed';
  pluginId: string;
  url: string;
  canGoBack: boolean;
  canGoForward: boolean;
  title: string;
  loading: boolean;
  /** 'failed' only: the main-frame load error. */
  error?: string;
}

interface Held {
  view: WebContentsView;
  plugin: StorePluginInfo;
  bounds: StoreRect | null;
  hidden: boolean;
  /** The address whose main-frame load failed, until a load succeeds: open
   * reloads it even though the contents already show that URL. */
  failedURL?: string;
}

/**
 * Holds the store views of a window. Needs the window (to attach to and to
 * place popups under) and a way to list the core's plugins (to check an open
 * is a known, enabled plugin with a storefront).
 */
export class StoreViews {
  private held = new Map<string, Held>();
  private current: string | null = null;
  private emit: (e: StoreEvent) => void = () => {};
  /** The last content-area rect reported: a newly shown view with no stored
   * bounds yet is placed there (its first show happens before the UI's next
   * bounds report). */
  private lastBounds: StoreRect | null = null;

  constructor(
    private win: BrowserWindow,
    private listPlugins: () => Promise<StorePluginInfo[]>,
  ) {}

  onEvent(cb: (e: StoreEvent) => void): void {
    this.emit = cb;
  }

  /** The plugin id whose view is shown, or null. */
  currentPlugin(): string | null {
    return this.current;
  }

  private currentHeld(): Held | undefined {
    return this.current ? this.held.get(this.current) : undefined;
  }

  private report(h: Held, kind: StoreEvent['kind'], loading: boolean): void {
    const wc = h.view.webContents;
    this.emit({
      kind,
      pluginId: this.current ?? '',
      url: wc.getURL(),
      canGoBack: wc.navigationHistory.canGoBack(),
      canGoForward: wc.navigationHistory.canGoForward(),
      title: wc.getTitle(),
      loading,
    });
  }

  private reportBlocked(id: string, h: Held, url: string): void {
    const wc = h.view.webContents;
    let canGoBack = false;
    let canGoForward = false;
    try {
      canGoBack = wc.navigationHistory.canGoBack();
      canGoForward = wc.navigationHistory.canGoForward();
    } catch {
      // The contents may be gone; the notice still names the attempt.
    }
    this.emit({
      kind: 'blocked',
      pluginId: id,
      url,
      canGoBack,
      canGoForward,
      title: '',
      loading: false,
    });
  }

  /** F12 / Ctrl+Shift+I while focus is inside the store view or a popup:
   * development with PLATYPUS_DEVTOOLS opens DevTools for THAT contents
   * (detached). Production unchanged. */
  private devtools(wc: Electron.WebContents): void {
    wc.on('before-input-event', (_e, input) => {
      if (input.type !== 'keyDown') return;
      if (!app.isPackaged && process.env.PLATYPUS_DEVTOOLS) {
        if (input.key === 'F12' || (input.control && input.shift && input.key.toLowerCase() === 'i')) {
          wc.openDevTools({ mode: 'detach' });
        }
      }
    });
  }

  /** Refuses a navigation outside the plugin's domains (emitting the
   * browser-open notice) or to a non-http(s) scheme (silently). */
  private guard(id: string, h: Held, wc: Electron.WebContents): void {
    const check = (e: Electron.Event, url: string) => {
      const domains = h.plugin.storefront_domains ?? [];
      if (navigationAllowed(url, domains)) return;
      e.preventDefault();
      // steam://install and friends: ignored, never loaded, never crashed
      // on. An http(s) page outside the domains gets the toolbar notice
      // (open in the browser, never auto-opened).
      if (blockedHost(url, domains) !== '') this.reportBlocked(id, h, url);
    };
    wc.on('will-navigate', (e, url) => check(e, url));
    // Server redirects (e.g. a storefront bouncing through an outside host).
    wc.on('will-redirect', (e, url) => check(e, url));
  }

  private watch(id: string, h: Held): void {
    const wc = h.view.webContents;
    wc.on('did-navigate', (_e, url, httpResponseCode) => {
      // Only a real response clears a failed load: Chromium's error page
      // also loads, which must not hide the failure from the next open.
      if (httpResponseCode > 0) h.failedURL = undefined;
      if (this.current === id) this.report(h, 'navigated', false);
    });
    wc.on('did-navigate-in-page', (_e, url) => {
      void url;
      if (this.current === id) this.report(h, 'navigated', false);
    });
    wc.on('page-title-updated', () => {
      if (this.current === id) this.report(h, 'navigated', false);
    });
    wc.on('did-start-loading', () => {
      if (this.current === id) this.report(h, 'loading', true);
    });
    wc.on('did-stop-loading', () => {
      if (this.current === id) this.report(h, 'loading', false);
    });
    // A failed main-frame load (not an abort from a newer navigation) shows
    // the UI's error state instead of a white page, also for later loads.
    wc.on('did-fail-load', (_e, errorCode, errorDescription, validatedURL, isMainFrame) => {
      if (!isMainFrame || errorCode === -3) return;
      h.failedURL = validatedURL;
      if (this.current === id) {
        this.emit({
          kind: 'failed',
          pluginId: id,
          url: validatedURL,
          canGoBack: false,
          canGoForward: false,
          title: '',
          loading: false,
          error: `${errorDescription} (${errorCode}) loading '${validatedURL}'`,
        });
      }
    });
  }

  private hold(id: string, plugin: StorePluginInfo): Held {
    let h = this.held.get(id);
    if (h) {
      h.plugin = plugin;
      return h;
    }
    const sess = session.fromPartition(storePartition(id));
    // Nothing in a store asks for a permission; refuse them all (a store that
    // needs one for checkout documents it and gets an explicit allow here).
    sess.setPermissionRequestHandler((_wc, _perm, cb) => cb(false));
    // The view never keeps a file: checkout downloads nothing the launcher wants.
    sess.on('will-download', (e) => e.preventDefault());
    const view = new WebContentsView({
      webPreferences: {
        session: sess,
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
        webSecurity: true,
        spellcheck: false,
      },
    });
    h = { view, plugin, bounds: null, hidden: false };
    view.setVisible(false);
    this.devtools(view.webContents);
    this.guard(id, h, view.webContents);
    view.webContents.setWindowOpenHandler(({ url }) => {
      const held = this.held.get(id);
      const domains = (held?.plugin ?? plugin).storefront_domains ?? [];
      const child = popupURL(url, domains);
      if (!child) {
        // Outside the domains (or a non-http(s) scheme): the same toolbar
        // notice as a blocked top-level navigation, never auto-opened.
        if (held && blockedHost(url, domains) !== '') this.reportBlocked(id, held, url);
        return { action: 'deny' };
      }
      // Login popups stay in the store's session, under the main
      // window, sandboxed like the view itself.
      const popup = new BrowserWindow({
        parent: this.win,
        show: true,
        autoHideMenuBar: true,
        webPreferences: {
          session: sess,
          contextIsolation: true,
          sandbox: true,
          nodeIntegration: false,
          webSecurity: true,
          spellcheck: false,
        },
      });
      this.devtools(popup.webContents);
      if (held) this.guard(id, held, popup.webContents);
      popup.webContents.setWindowOpenHandler(({ url: nested }) => {
        // A popup never opens its own child: inside would be a surprise,
        // outside gets the same notice on the main view.
        if (held && blockedHost(nested, domains) !== '') this.reportBlocked(id, held, nested);
        return { action: 'deny' };
      });
      void popup.loadURL(child);
      return { action: 'deny' };
    });
    this.watch(id, h);
    this.held.set(id, h);
    return h;
  }

  private show(h: Held): void {
    // Views stay attached once created and are toggled with setVisible:
    // detaching (removeChildView) and re-attaching leaves the contents
    // blank on return (echo → other → echo never painted again, not even
    // after reload), while setVisible restores painting.
    const content = this.win.contentView;
    if (!content.children.includes(h.view)) content.addChildView(h.view);
    // A view held before any bounds report has none yet: place it where the
    // content area last was (the UI's next report corrects it).
    const bounds = h.bounds ?? this.lastBounds;
    if (bounds) {
      if (bounds.width > 0 && bounds.height > 0) {
        h.bounds = bounds;
        h.view.setBounds(bounds);
      } else {
        h.hidden = true;
        applyStoreViewBounds(h.view, bounds);
        return;
      }
    }
    h.hidden = false;
    h.view.setVisible(true);
  }

  private hide(h: Held): void {
    // Kept attached (and alive) for a fast return; invisible means no
    // paint and no input.
    h.view.setVisible(false);
  }

  /**
   * Opens the plugin's store (the storefront, or a deep link inside its
   * domains). Throws when the plugin isn't a known, enabled plugin with a
   * storefront, or the address is refused. Switching stores never closes
   * through the UI's unmount: open hides the previous view itself, so a
   * stale close racing over a separate IPC channel can't hide the new one.
   */
  async open(pluginId: string, rawURL?: unknown): Promise<void> {
    const plugins = await this.listPlugins();
    const plugin = plugins.find((p) => p.id === pluginId);
    if (!canOpenStore(plugin)) throw new Error(`refused: ${pluginId} has no store to open`);
    const url = explicitStoreURL(plugin!, rawURL);
    if (!url) throw new Error(`refused: not an address ${pluginId} may open`);
    const prev = this.currentHeld();
    if (prev && this.current !== pluginId) this.hide(prev);
    const h = this.hold(pluginId, plugin!);
    this.current = pluginId;
    this.show(h);
    // After a failed load the contents already report the URL: load again.
    if (h.view.webContents.getURL() !== url || h.failedURL) {
      h.failedURL = undefined;
      try {
        await h.view.webContents.loadURL(url);
      } catch (err) {
        // Switching stores mid-load aborts the pending load
        // (ERR_ABORTED): the new view is already showing, nothing failed.
        if (!isAbortedLoad(err)) throw err;
      }
    }
    this.report(h, 'navigated', h.view.webContents.isLoading());
  }

  /** Places the shown view over the UI's content area. */
  setBounds(bounds: StoreRect): void {
    if (bounds.width === 0 || bounds.height === 0) {
      const h = this.currentHeld();
      if (h) h.hidden = true;
      if (h) applyStoreViewBounds(h.view, bounds);
      return;
    }
    this.lastBounds = bounds;
    const h = this.currentHeld();
    if (!h) return;
    h.bounds = bounds;
    h.hidden = false;
    applyStoreViewBounds(h.view, bounds);
  }

  /** Re-applies the shown view's bounds (after the window resized). */
  relayout(): void {
    const h = this.currentHeld();
    const bounds = h?.bounds ?? this.lastBounds;
    if (h && bounds && !h.hidden) {
      if (bounds.width > 0 && bounds.height > 0) {
        h.bounds = bounds;
        h.view.setBounds(bounds);
        h.view.setVisible(true);
      }
    }
  }

  back(): void {
    const wc = this.currentHeld()?.view.webContents;
    if (wc?.navigationHistory.canGoBack()) wc.navigationHistory.goBack();
  }

  forward(): void {
    const wc = this.currentHeld()?.view.webContents;
    if (wc?.navigationHistory.canGoForward()) wc.navigationHistory.goForward();
  }

  reload(): void {
    this.currentHeld()?.view.webContents.reload();
  }

  /**
   * Hides the shown view, keeping it alive for a fast return. Names its
   * plugin, so a stale close (the previous store's effect cleanup racing the
   * next store's open over separate IPC channels) never hides the newly
   * shown view. An absent id closes whatever is shown (leaving the store
   * view entirely).
   */
  close(pluginId?: unknown): void {
    if (!shouldCloseView(this.current, pluginId)) return;
    const h = this.currentHeld();
    if (h) this.hide(h);
    this.current = null;
  }

  /** Destroys every held view (app shutdown). */
  destroy(): void {
    for (const h of this.held.values()) {
      h.view.webContents.close();
      if (this.win.contentView.children.includes(h.view)) this.win.contentView.removeChildView(h.view);
    }
    this.held.clear();
    this.current = null;
  }
}

/** Whether a loadURL failure is an interrupted load (switching stores
 * mid-load), not a real failure worth reporting. */
function isAbortedLoad(err: unknown): boolean {
  const msg = err instanceof Error ? `${err.message} ${'code' in err ? String((err as { code: unknown }).code) : ''}` : String(err);
  return msg.includes('ERR_ABORTED') || msg.includes('-3');
}
