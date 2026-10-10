import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach } from 'vitest';

// jsdom lacks scrollTo; App scrolls main on view change.
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

// Every test runs as if inside the desktop shell: the UI only talks to the Go
// core through window.platypus (see effect/bridge.ts). Tests that assert on
// bridge calls override openExternal (or the whole stub) themselves.
beforeEach(() => {
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
