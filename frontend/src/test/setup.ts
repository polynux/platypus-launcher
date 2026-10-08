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
