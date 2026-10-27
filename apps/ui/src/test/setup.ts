import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach } from 'vitest';
import i18n, { setSessionInfo } from '../lib/i18n';

// jsdom lacks scrollTo; App scrolls main on view change, and the router
// scrolls the window on navigation.
if (!Element.prototype.scrollTo) {
  (Element.prototype as unknown as Record<string, unknown>).scrollTo = () => {};
} else {
  const orig = Element.prototype.scrollTo;
  try {
    (Element.prototype as unknown as Record<string, unknown>).scrollTo = () => {};
  } catch {
    void orig;
  }
}
if (typeof window.scrollTo !== 'function' || window.scrollTo.toString().includes('not implemented')) {
  (window as unknown as Record<string, unknown>).scrollTo = () => {};
} else {
  try {
    (window as unknown as Record<string, unknown>).scrollTo = () => {};
  } catch {
    // Keep jsdom's stub.
  }
}

// Radix (Select, Dialog, ScrollArea, Tooltip) needs a few DOM APIs jsdom lacks.
const proto = Element.prototype as unknown as Record<string, unknown>;
if (!proto.hasPointerCapture) proto.hasPointerCapture = () => false;
if (!proto.setPointerCapture) proto.setPointerCapture = () => {};
if (!proto.releasePointerCapture) proto.releasePointerCapture = () => {};
if (!proto.scrollIntoView) proto.scrollIntoView = () => {};
if (!('ResizeObserver' in globalThis)) {
  (globalThis as unknown as Record<string, unknown>).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

// Network guard (issue #40): the UI under test must never dial the real
// network — covers and artwork come through the Go core (window.platypus),
// so fetch has no business leaving localhost. Refuse non-local fetches
// with a clear message instead of hanging on a real connection. Tests that
// genuinely need the network opt in with PLATYPUS_TEST_NETWORK=1 (see
// testenv.ts); there are currently none.
const origFetch = globalThis.fetch.bind(globalThis);
function isLocalURL(url: string): boolean {
  try {
    const u = new URL(url, 'http://localhost/');
    const h = u.hostname;
    return h === 'localhost' || h === '127.0.0.1' || h === '::1';
  } catch {
    return true;
  }
}
globalThis.fetch = ((input: unknown, init?: unknown) => {
  if (process.env.PLATYPUS_TEST_NETWORK !== '1') {
    const url = typeof input === 'string' ? input : (input as Request).url;
    if (!isLocalURL(url)) {
      return Promise.reject(
        new Error(
          `test fetch refused non-localhost URL ${url} (gate the test behind PLATYPUS_TEST_NETWORK=1 or point it at a localhost fake)`,
        ),
      );
    }
  }
  return origFetch(input as RequestInfo, init as RequestInit);
}) as typeof fetch;
// Every test runs as if inside the desktop shell: the UI only talks to the Go
// core through window.platypus (see effect/bridge.ts). Tests that assert on
// bridge calls override openExternal (or the whole stub) themselves.
// i18n starts in English (issue #39): tests asserting English text run under
// the default `en`, and each test resets any language it changed.
beforeEach(async () => {
  // Each test starts at the dashboard route (issue #140): the App builds a
  // hash-history router per mount, so clear the hash between tests.
  window.location.hash = '';
  setSessionInfo(null);
  if (i18n.language !== 'en') await i18n.changeLanguage('en');
  window.platypus = {
    invoke: async () => {
      throw new Error('unexpected core call in test');
    },
    on: () => () => {},
    openExternal: () => {},
  };
});

afterEach(() => {
  delete window.platypus;
});
