// i18n: converted
import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, ArrowRight, ExternalLink, Globe, Home, RotateCw, TriangleAlert, X } from 'lucide-react';
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
  /** Atlas shop bar (issue #138): store switch and actions rendered into the
   *  single browser row, so the page has one bar, not two tall rows. */
  toolbarLead?: React.ReactNode;
  toolbarTrail?: React.ReactNode;
}

/**
 * The embedded store view (issue #17): a slim toolbar over a native view
 * the shell places over the content area. The toolbar mirrors the view's
 * navigation state (back/forward, home, reload, address strip with host +
 * lock, loading); the strip warns when the host somehow left the store's
 * declared domains. Navigation outside the domains never loads: the toolbar
 * shows a non-modal notice offering to open the address in the browser.
 */
export function StoreView({ plugin, initialUrl, onClose, onNotify, toolbarLead, toolbarTrail }: StoreViewProps) {
  const { t } = useTranslation(['store', 'common']);
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
      setFailed(t('store:needsShell'));
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
  }, [plugin.id, initialUrl, t]);

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
        {toolbarLead}
        <ToolbarButton label={t('store:toolbar.back')} disabled={!state.canGoBack} onClick={() => store?.back()} data-testid="store-back">
          <ArrowLeft className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label={t('store:toolbar.forward')} disabled={!state.canGoForward} onClick={() => store?.forward()} data-testid="store-forward">
          <ArrowRight className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label={t('store:toolbar.reload')} onClick={() => store?.reload()} data-testid="store-reload">
          <RotateCw className={cx('h-4 w-4', state.loading && 'animate-spin')} />
        </ToolbarButton>
        <ToolbarButton label={t('store:toolbar.home')} onClick={goHome} data-testid="store-home">
          <Home className="h-4 w-4" />
        </ToolbarButton>
        <div
          data-testid="store-address"
          title={`${plugin.name} · ${state.url}`}
          className={cx(
            'mx-1.5 flex h-[30px] min-w-0 flex-1 items-center gap-2 rounded-lg bg-surface px-2.5 text-[12px] text-text-faint ring-1',
            outside ? 'bg-attention/[0.07] text-attention ring-attention/30' : 'ring-hairline',
          )}
        >
          {state.loading ? (
            <span data-testid="store-loading" aria-label={t('store:address.loading')} className="h-3 w-3 flex-none animate-spin rounded-full border border-hairline-strong border-t-transparent" />
          ) : (
            <Globe className="h-3 w-3 flex-none" aria-label={outside ? t('store:address.lockOutside') : t('store:address.lockInside')} />
          )}
          <span className="truncate">{host || '…'}</span>
          {outside && (
            <span className="flex flex-none items-center gap-1 text-attention/90">
              <TriangleAlert className="h-3 w-3" />
              <span className="hidden sm:inline">{t('store:address.outsideSuffix', { name: plugin.name })}</span>
            </span>
          )}
          {state.title && <span className="hidden min-w-0 flex-1 truncate text-right lg:inline">{state.title}</span>}
        </div>
        {toolbarTrail}
        <ToolbarButton label={t('store:toolbar.openExternal')} onClick={openInBrowser} data-testid="store-external">
          <ExternalLink className="h-4 w-4" />
        </ToolbarButton>
        <ToolbarButton label={t('store:toolbar.close', { name: plugin.name })} onClick={onClose} data-testid="store-close">
          <X className="h-4 w-4" />
        </ToolbarButton>
      </div>
      {blocked && !failed && (
        <div
          data-testid="store-blocked"
          role="note"
          className="mx-5 mb-3 flex flex-none items-center gap-2 rounded-xl bg-attention/[0.07] px-3 py-2 text-[12px] text-attention ring-1 ring-attention/30"
        >
          <TriangleAlert className="h-3.5 w-3.5 flex-none" />
          <span className="min-w-0 flex-1 truncate">
            {t('store:blocked', { host: blockedHost, name: plugin.name })}
          </span>
          <button
            type="button"
            onClick={openBlocked}
            data-testid="store-blocked-open"
            className="flex-none rounded-lg bg-attention/20 px-2.5 py-1 font-medium hover:bg-attention/30"
          >
            {t('common:open')}
          </button>
          <button
            type="button"
            onClick={() => setBlocked(null)}
            aria-label={t('common:dismiss')}
            className="grid h-6 w-6 flex-none place-items-center rounded-lg hover:bg-attention/20"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}
      {failed ? (
        <div className="mx-5 mb-5 flex flex-1 items-center justify-center rounded-2xl bg-surface px-6 text-center text-[13px] text-text-muted ring-1 ring-hairline">
          <p>{t('store:failed', { name: plugin.name, error: failed })}</p>
        </div>
      ) : (
        <div ref={area} data-testid="store-area" className="mx-5 mb-5 min-h-0 flex-1 rounded-2xl bg-surface ring-1 ring-hairline" />
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
      className="grid h-9 w-9 flex-none place-items-center rounded-xl bg-surface text-text-muted ring-1 ring-hairline transition hover:bg-surface-strong hover:text-text disabled:opacity-35"
      {...rest}
    >
      {children}
    </Button>
  );
}
