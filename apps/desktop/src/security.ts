import path from 'node:path';

/** The address, as written, when it is a web link the user's browser may open ('' otherwise). */
export function webURL(raw: unknown): string {
  if (typeof raw !== 'string') return '';
  const s = raw.trim();
  try {
    const u = new URL(s);
    return u.protocol === 'https:' || u.protocol === 'http:' ? s : '';
  } catch {
    return '';
  }
}

/** The page's origin. Everything the window loads comes from it. */
export const APP_ORIGIN = 'app://platypus';

/** Whether an address belongs to the app page. (URL.origin of a custom scheme is "null", so compare the parts.) */
export function isAppURL(raw: string): boolean {
  try {
    const u = new URL(raw);
    return u.protocol === 'app:' && u.host === 'platypus';
  } catch {
    return false;
  }
}

/** Paths the core serves (Steam's library cache, the user's custom art): internal/steam.ArtURLPrefix, internal/customize.URLPrefix. */
const ART_PREFIXES = ['/steam-art/', '/platypus-art/'];

export function isArtPath(pathname: string): boolean {
  return ART_PREFIXES.some((p) => pathname.startsWith(p));
}

/** Whether a decoded URL path stays inside its root: no "..", no NUL, no backslash. */
export function safePath(pathname: string): boolean {
  if (pathname.includes('\0') || pathname.includes('\\')) return false;
  return !pathname.split('/').includes('..');
}

/**
 * The file of root that a request path names ('/' is index.html), or null when
 * the path is malformed or would leave root.
 */
export function staticFile(root: string, requestPath: string): string | null {
  let decoded: string;
  try {
    decoded = decodeURIComponent(requestPath);
  } catch {
    return null;
  }
  if (!safePath(decoded)) return null;
  const rel = decoded === '/' ? '/index.html' : decoded;
  const file = path.join(root, rel);
  return file.startsWith(root + path.sep) ? file : null;
}

/**
 * The Content-Security-Policy of the app page. Scripts only from the app
 * itself; images may come from the web (store covers) and from the loopback
 * (a plugin's local image server); videos (trailers) come from https store
 * CDNs and the loopback art server, never with sound autoplay; nothing
 * connects out. The dev policy adds what Vite's refresh runtime needs.
 */
export function csp(devPort?: number): string {
  const dev = devPort !== undefined;
  return [
    "default-src 'none'",
    dev ? "script-src 'self' 'unsafe-inline'" : "script-src 'self'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https: http://127.0.0.1:* http://localhost:*",
    "media-src 'self' https: http://127.0.0.1:* http://localhost:*",
    "font-src 'self' data:",
    dev ? `connect-src 'self' ws://127.0.0.1:${devPort}` : "connect-src 'self'",
    "base-uri 'none'",
    "form-action 'none'",
    "frame-ancestors 'none'",
    "object-src 'none'",
  ].join('; ');
}
