import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, ExternalLink, Home, Lock, RotateCw, TriangleAlert, X } from 'lucide-react';
import { cx } from './ui';
import { Button } from './ds/button';
import { browserOpen, onStoreEvent, storeBridge, type StoreEvent } from '../effect/bridge';
import type { StorePlugin } from '../types';

/** The host of an address, lowercased, or '' when it has none. Exported for tests. */
export function storeHostOf(url: string): string {
  try {
    return new URL(url).hostname.toLowerCase();
  } catch {
    return '';
  }
}

/**
 * Whether an address left the store's declared domains (exact or subdomain
 * match, like the shell's insideDomains). Exported for tests.
 */
export function storeHostOutside(url: string, domains: string[]): boolean {
  const host = storeHostOf(url);
  if (host === '') return false;
  return !domains.some((d) => {
    const base = d.toLowerCase().replace(/\.$/, '');
    const h = host.replace(/\.$/, '');
    return h === base || (base !== '' && h.endsWith(`.${base}`));
  });
}

interface StoreViewProps {
  plugin: StorePlugin;
  /** A deep link from a game page (inside the plugin's domains); absent means the storefront. */
  initialUrl?: string;
  onClose: () => void;
  onNotify: (msg: string, kind?: 'ok' | 'err' | 'busy') => void;
}

/**
 * The embedded store view (issue #17): a slim toolbar over a native view
 * the shell places over the content area. The toolbar mirrors the view's
 * navigation state (back/forward, home, reload, address strip with host +
 * lock, loading); the strip warns when the host somehow left the store's
 * declared domains. Navigation outside the domains never loads: the toolbar
 * shows a non-modal notice offering to open the address in the browser.
 */
export function StoreView({ plugin, initialUrl, onClose, onNotify }: StoreViewProps) {
  const store = storeBridge();
  const [state, setState] = useState<Pick<StoreEvent, 'url' | 'canGoBack' | 'canGoForward' | 'title' | 'loading'>>({
    url: initialUrl ?? plugin.storefront_url ?? '',
    canGoBack: false,
    canGoForward: false,
    title: '',
    loading: true,
  });
  const [failed, setFailed] = useState<string | null>(null);
  // An outside address the view refused to load (a 'blocked' store event):
  // the notice offers to open it in the browser, never auto-opening.
  const [blocked, setBlocked] = useState<string | null>(null);
  const area = useRef<HTMLDivElement>(null);
  const stateRef = useRef(state);
  stateRef.current = state;

  const reportBounds = useCallback(() => {
    const el = area.current;
    const bridge = storeBridge();
    if (!el || !bridge) return;
    const r = el.getBoundingClientRect();
    bridge.setBounds({ x: r.left, y: r.top, width: r.width, height: r.height });
  }, []);

  // Open the store on mount and when the plugin or deep link changes; the
  // shell validates the plugin and the address. Switching stores must NOT
  // close first: the shell's open hides the previous view itself, and a
  // close racing over a separate IPC channel would hide the new one (the
  // echo → other → echo return came back blank). Closing happens only when
  // the view unmounts (leaving the store), keeping the hidden view alive
  // for a fast return.
  useEffect(() => {
    const bridge = storeBridge();
    if (!bridge) {
      setFailed('The store view needs the app shell.');
      return;
    }
    setFailed(null);
    setBlocked(null);
    bridge.open(plugin.id, initialUrl).catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err);
      setFailed(msg);
      onNotify(msg, 'err');
    });
    // Open once per plugin+link: navigating inside the store must not reopen it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [plugin.id, initialUrl]);

  // Leaving the store view hides the native view (kept alive for a fast
  // return). Runs on unmount only, never on a plugin switch.
  const idRef = useRef(plugin.id);
  idRef.current = plugin.id;
  useEffect(
    () => () => {
      storeBridge()?.close(idRef.current);
    },
    [],
    // eslint-disable-next-line react-hooks/exhaustive-deps
  );

  // Follow the view's navigation (another plugin's events are ignored). A
  // successful navigation clears the outside-address notice.
  useEffect(() => onStoreEvent((ev) => {
    if (ev.pluginId !== plugin.id) return;
    if (ev.kind === 'blocked') {
      setBlocked(ev.url);
      return;
    }
    if (ev.kind === 'navigated') setBlocked(null);
    setState({ url: ev.url, canGoBack: ev.canGoBack, canGoForward: ev.canGoForward, title: ev.title, loading: ev.loading });
  }), [plugin.id]);

  // Keep the native view over the content area: layout, resize, scroll, remount.
  useLayoutEffect(() => {
    reportBounds();
  });
  useEffect(() => {
    reportBounds();
    const onScroll = () => reportBounds();
    window.addEventListener('resize', onScroll);
    window.addEventListener('scroll', onScroll, true);
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(reportBounds) : null;
    if (ro && area.current) ro.observe(area.current);
    return () => {
      window.removeEventListener('resize', onScroll);
      window.removeEventListener('scroll', onScroll, true);
      ro?.disconnect();
    };
  }, [reportBounds]);

  const domains = plugin.storefront_domains ?? [];
  const host = storeHostOf(state.url);
  const outside = storeHostOutside(state.url, domains);
  const blockedHost = blocked ? storeHostOf(blocked) : '';
  const openInBrowser = () => {
    try {
      browserOpen(state.url);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : String(err), 'err');
    }
  };
  const openBlocked = () => {
    if (!blocked) return;
    try {
      browserOpen(blocked);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : String(err), 'err');
    }
  };
  const goHome = () => {
    setBlocked(null);
    store?.open(plugin.id).catch((err: unknown) => {
      const msg = err instanceof Error ? err.message : String(err);
      setFailed(msg);
      onNotify(msg, 'err');
    });
  };

  return (
    <div className="flex h-full min-h-[480px] flex-col">
      <div className="flex flex-none items-center gap-1.5 px-5 pb-3 pt-5">
        <ToolbarButton label="Back" disabled={!state.canGoBack} onClick={() => store?.back()} data-testid="store-back">
          <ArrowLeft className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label="Forward" disabled={!state.canGoForward} onClick={() => store?.forward()} data-testid="store-forward">
          <ArrowRight className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label="Reload" onClick={() => store?.reload()} data-testid="store-reload">
          <RotateCw className={cx('h-4 w-4', state.loading && 'animate-spin')} />
        </ToolbarButton>
        <ToolbarButton label="Home" onClick={goHome} data-testid="store-home">
          <Home className="h-4 w-4" />
        </ToolbarButton>
        <div
          data-testid="store-address"
          title={state.url}
          className={cx(
            'mx-1.5 flex min-w-0 flex-1 items-center gap-2 rounded-xl px-3 py-2 font-mono text-[12px] ring-1',
            outside
              ? 'bg-amber-300/[0.07] text-amber-200 ring-amber-300/30'
              : 'bg-black/40 text-white/70 ring-white/[0.08]',
          )}
        >
          {state.loading ? (
            <span data-testid="store-loading" aria-label="loading" className="h-3 w-3 flex-none animate-spin rounded-full border border-white/30 border-t-transparent" />
          ) : (
            <Lock className="h-3 w-3 flex-none opacity-70" aria-label={outside ? 'outside the store' : 'store page'} />
          )}
          <span className="truncate">{host || '…'}</span>
          {outside && (
            <span className="flex flex-none items-center gap-1 text-amber-200/90">
              <TriangleAlert className="h-3 w-3" />
              <span className="hidden sm:inline">outside {plugin.name}</span>
            </span>
          )}
          {state.title && <span className="hidden min-w-0 flex-1 truncate text-right text-white/40 lg:inline">{state.title}</span>}
        </div>
        <ToolbarButton label="Open in browser" onClick={openInBrowser} data-testid="store-external">
          <ExternalLink className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label={`Close ${plugin.name}`} onClick={onClose} data-testid="store-close">
          <X className="h-4 w-4" />
        </ToolbarButton>
      </div>
      {blocked && !failed && (
        <div
          data-testid="store-blocked"
          role="note"
          className="mx-5 mb-3 flex flex-none items-center gap-2 rounded-xl bg-amber-300/[0.07] px-3 py-2 text-[12px] text-amber-200 ring-1 ring-amber-300/30"
        >
          <TriangleAlert className="h-3.5 w-3.5 flex-none" />
          <span className="min-w-0 flex-1 truncate">
            {blockedHost} isn&apos;t part of {plugin.name}. Open in your browser?
          </span>
          <button
            type="button"
            onClick={openBlocked}
            data-testid="store-blocked-open"
            className="flex-none rounded-lg bg-amber-300/20 px-2.5 py-1 font-medium hover:bg-amber-300/30"
          >
            Open
          </button>
          <button
            type="button"
            onClick={() => setBlocked(null)}
            aria-label="Dismiss"
            className="grid h-6 w-6 flex-none place-items-center rounded-lg hover:bg-white/10"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}
      {failed ? (
        <div className="mx-5 mb-5 flex flex-1 items-center justify-center rounded-2xl bg-white/[0.03] px-6 text-center text-[13px] text-white/60 ring-1 ring-white/[0.07]">
          <p>Couldn't open {plugin.name}: {failed}</p>
        </div>
      ) : (
        <div ref={area} data-testid="store-area" className="mx-5 mb-5 min-h-0 flex-1 rounded-2xl bg-black/30 ring-1 ring-white/[0.07]" />
      )}
    </div>
  );
}

function ToolbarButton({ label, children, ...rest }: {
  label: string;
  children: React.ReactNode;
  disabled?: boolean;
  onClick?: () => void;
  'data-testid'?: string;
}) {
  return (
    <Button
      variant="neutral"
      size="none"
      title={label}
      aria-label={label}
      className="grid h-9 w-9 flex-none place-items-center rounded-xl bg-black/50 text-white/80 hover:bg-black/70 disabled:opacity-35 disabled:hover:bg-black/50"
      {...rest}
    >
      {children}
    </Button>
  );
}
