import '@testing-library/jest-dom/vitest';

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
