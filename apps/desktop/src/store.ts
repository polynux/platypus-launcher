// The embedded store view's policy (issue #17): what the launcher embeds,
// where it may go, and what the address strip shows. Everything here is pure
// (no Electron imports) so node:test covers it; the manager (storeview.ts)
// applies it to real WebContentsViews.
//
// A store plugin declares its official website in its manifest
// (`storefront: { url, domains }`, validated by the core in
// internal/plugins/manifest.go): the view opens there. Top-level navigation
// may then go anywhere over http(s) — checkout wanders to payment providers
// outside the declared domains — but the address strip warns about hosts
// outside them. Anything that isn't http(s) never loads: popups of it are
// refused, top-level attempts are ignored, downloads are cancelled.

/** A store plugin as the open call needs it (a row of ListStorePlugins). */
export interface StorePluginInfo {
  id: string;
  status: string;
  storefront_url?: string;
  storefront_domains?: string[];
}

/** The content-area rect the React page reports for the native view. */
export interface StoreRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

/** The rect to place the native view at, or null when it is unusable. */
export function validStoreBounds(raw: unknown): StoreRect | null {
  if (typeof raw !== 'object' || raw === null) return null;
  const r = raw as Record<string, unknown>;
  const nums = [r.x, r.y, r.width, r.height];
  if (!nums.every((n) => typeof n === 'number' && Number.isFinite(n))) return null;
  const [x, y, width, height] = nums as number[];
  if (width < 1 || height < 1 || width > 16384 || height > 16384) return null;
  if (x < 0 || y < 0 || x > 16384 || y > 16384) return null;
  return { x: Math.round(x), y: Math.round(y), width: Math.round(width), height: Math.round(height) };
}

/** The persistent session partition of a plugin's store view: logins stay, isolated per plugin. */
export function storePartition(pluginId: string): string {
  return `persist:store-${pluginId}`;
}

/** Whether a plugin id is shaped like a plugin id (the open call checks it is also known and enabled). */
export function validPluginId(raw: unknown): boolean {
  return typeof raw === 'string' && /^[a-z0-9][a-z0-9._-]{0,63}$/.test(raw);
}

function hostOf(raw: string): string {
  try {
    return new URL(raw).hostname.toLowerCase();
  } catch {
    return '';
  }
}

/** Whether host belongs to domains: an exact match or a subdomain of one. */
export function insideDomains(host: string, domains: string[]): boolean {
  const h = host.toLowerCase().replace(/\.$/, '');
  return domains.some((d) => {
    const base = d.toLowerCase().replace(/\.$/, '');
    return h === base || (base !== '' && h.endsWith(`.${base}`));
  });
}

/** Whether host is loopback (the test plugin's local page). */
export function isLoopbackHost(host: string): boolean {
  const h = host.toLowerCase();
  if (h === 'localhost') return true;
  if (/^\d+\.\d+\.\d+\.\d+$/.test(h)) {
    return h.startsWith('127.') || h === '::1';
  }
  return h === '::1';
}

/**
 * Whether an http loopback store page is acceptable for the plugin: the test
 * plugin (echo, echo-b), or an explicit operator opt-in. Mirrors the core's
 * allowHTTPStorefront (internal/plugins/manifest.go); production stays
 * https-only.
 */
export function allowHTTPStorefront(pluginId: string, env: NodeJS.ProcessEnv = process.env): boolean {
  if (env.PLATYPUS_ALLOW_HTTP_STOREFRONT) return true;
  return pluginId === 'echo' || pluginId === 'echo-b';
}

/**
 * The address an explicit open may load for the plugin (the storefront, or a
 * deep link from a game page), or '' when refused. An explicit address must
 * be https and inside the plugin's domains — http loopback only for the test
 * plugin. In-page navigation is looser (see navigationAllowed): checkout
 * uses payment providers outside the domains.
 */
export function explicitStoreURL(plugin: StorePluginInfo, rawURL: unknown): string {
  const fallback = typeof plugin.storefront_url === 'string' ? plugin.storefront_url : '';
  const raw = rawURL === undefined || rawURL === null || rawURL === '' ? fallback : rawURL;
  if (typeof raw !== 'string') return '';
  const s = raw.trim();
  let u: URL;
  try {
    u = new URL(s);
  } catch {
    return '';
  }
  const host = u.hostname.toLowerCase();
  const domains = Array.isArray(plugin.storefront_domains) ? plugin.storefront_domains : [];
  if (u.protocol === 'https:') {
    return insideDomains(host, domains) ? s : '';
  }
  if (u.protocol === 'http:' && isLoopbackHost(host) && allowHTTPStorefront(plugin.id)) {
    return insideDomains(host, domains) ? s : '';
  }
  return '';
}

/**
 * Whether the open call may embed the plugin: it is a known plugin whose
 * status is ready (approved and enabled) and it declares a storefront.
 */
export function canOpenStore(plugin: StorePluginInfo | undefined): boolean {
  return !!plugin && plugin.status === 'ready' && typeof plugin.storefront_url === 'string' && plugin.storefront_url !== '';
}

/**
 * Whether a top-level navigation may load: anything over http(s) (checkout
 * wanders to payment providers); anything else (steam://, epic://, file:,
 * javascript:) is blocked. The address strip (stripState) tells the user
 * when the host left the declared domains.
 */
export function navigationAllowed(rawURL: string): boolean {
  try {
    const u = new URL(rawURL);
    return u.protocol === 'https:' || u.protocol === 'http:';
  } catch {
    return false;
  }
}

/** A popup's address when it may open in the child window, or '' when blocked (non-http(s) never opens). */
export function popupURL(rawURL: string): string {
  return navigationAllowed(rawURL) ? rawURL : '';
}

/** What the address strip shows for an address: its host, and whether it left the store's domains. */
export function stripState(
  rawURL: string,
  domains: string[],
): { host: string; outside: boolean } {
  let host = '';
  try {
    host = new URL(rawURL).hostname.toLowerCase();
  } catch {
    host = '';
  }
  return { host, outside: host !== '' && !insideDomains(host, domains) };
}
