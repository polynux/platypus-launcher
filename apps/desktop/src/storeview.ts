// The embedded store view (issue #17): one WebContentsView per store plugin,
// placed over the UI's content area. It is NOT the app page: a separate
// persistent session per plugin (logins stay, isolated from the launcher and
// from each other), no preload, no bridge, permissions denied; downloads
// cancelled; popups open in a child window of the same session (login and
// payment popups); non-http(s) navigation never loads.
//
// A hidden view stays alive (fast return; its session keeps the login), and
// is destroyed only when the window closes with the app. Leaving the view
// hides it; the UI invalidates the plugin's library through the core
// (InvalidateLibrary) so purchases appear.
import { BrowserWindow, session, WebContentsView } from 'electron';
import {
  canOpenStore,
  explicitStoreURL,
  navigationAllowed,
  popupURL,
  storePartition,
  type StorePluginInfo,
  type StoreRect,
} from './store.ts';

export interface StoreEvent {
  /** 'navigated' (url, history, title) or 'loading' (spinner state). */
  kind: 'navigated' | 'loading';
  pluginId: string;
  url: string;
  canGoBack: boolean;
  canGoForward: boolean;
  title: string;
  loading: boolean;
}

interface Held {
  view: WebContentsView;
  plugin: StorePluginInfo;
  bounds: StoreRect | null;
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

  private watch(id: string, h: Held): void {
    const wc = h.view.webContents;
    wc.on('did-navigate', (_e, url) => {
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
    view.webContents.on('will-navigate', (e, url) => {
      if (navigationAllowed(url)) return;
      // steam://install and friends: ignored, never loaded, never crashed on.
      e.preventDefault();
    });
    view.webContents.setWindowOpenHandler(({ url }) => {
      const child = popupURL(url);
      if (!child) return { action: 'deny' };
      // Login and payment popups stay in the store's session, under the main
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
      popup.webContents.on('will-navigate', (ev, to) => {
        if (!navigationAllowed(to)) ev.preventDefault();
      });
      popup.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
      void popup.loadURL(child);
      return { action: 'deny' };
    });
    h = { view, plugin, bounds: null };
    this.watch(id, h);
    this.held.set(id, h);
    return h;
  }

  private show(h: Held): void {
    const content = this.win.contentView;
    if (!content.children.includes(h.view)) content.addChildView(h.view);
    if (h.bounds) h.view.setBounds(h.bounds);
  }

  private hide(h: Held): void {
    const content = this.win.contentView;
    if (content.children.includes(h.view)) content.removeChildView(h.view);
  }

  /**
   * Opens the plugin's store (the storefront, or a deep link inside its
   * domains). Throws when the plugin isn't a known, enabled plugin with a
   * storefront, or the address is refused.
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
    if (h.view.webContents.getURL() !== url) await h.view.webContents.loadURL(url);
    this.report(h, 'navigated', h.view.webContents.isLoading());
  }

  /** Places the shown view over the UI's content area. */
  setBounds(bounds: StoreRect): void {
    const h = this.currentHeld();
    if (!h) return;
    h.bounds = bounds;
    h.view.setBounds(bounds);
  }

  /** Re-applies the shown view's bounds (after the window resized). */
  relayout(): void {
    const h = this.currentHeld();
    if (h?.bounds) h.view.setBounds(h.bounds);
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

  /** Hides the shown view, keeping it alive for a fast return. */
  close(): void {
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
