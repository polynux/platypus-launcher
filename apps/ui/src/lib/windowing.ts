import { useCallback, useLayoutEffect, useRef, useState, type RefObject } from 'react';

/** Columns of a grid with `repeat(auto-fill, minmax(minCol, 1fr))`: as many as fit, at least one. */
export function gridColumns(width: number, minCol: number, gap: number): number {
  return Math.max(1, Math.floor((width + gap) / (minCol + gap)));
}

/** Width of one column of that grid, gaps taken out. */
export function columnWidth(width: number, cols: number, gap: number): number {
  return (width - gap * (cols - 1)) / cols;
}

/** The rows [start, end) to keep in the DOM. `top` is where the list starts in
 *  the scrolled content, `pitch` is a row plus the gap under it, and `overscan`
 *  is how far past the viewport (in px, each side) rows are still rendered. */
export function visibleRows(args: { scrollTop: number; viewport: number; top: number; pitch: number; rows: number; overscan: number }): [number, number] {
  const { scrollTop, viewport, top, pitch, rows, overscan } = args;
  if (rows <= 0 || pitch <= 0) return [0, 0];
  const from = scrollTop - top - overscan;
  const to = scrollTop - top + viewport + overscan;
  const start = Math.min(rows, Math.max(0, Math.floor(from / pitch)));
  const end = Math.min(rows, Math.max(start, Math.ceil(to / pitch)));
  return [start, end];
}

/** Marks the element a window of rows follows. Needed where a nearer ancestor only scrolls sideways:
 *  `overflow-x: auto` makes its `overflow-y` compute to auto too. */
export const SCROLL_ROOT_ATTR = 'data-scroll-root';

/** The element a list scrolls in: the marked one, else the nearest ancestor that scrolls vertically (the page when none does). */
function scrollParentOf(el: HTMLElement): HTMLElement {
  const marked = el.closest<HTMLElement>(`[${SCROLL_ROOT_ATTR}]`);
  if (marked) return marked;
  for (let p = el.parentElement; p; p = p.parentElement) {
    const overflowY = getComputedStyle(p).overflowY;
    if (overflowY === 'auto' || overflowY === 'scroll') return p;
  }
  return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
}

/** The width of an element, kept up to date. 0 before the first measure (and in jsdom). */
export function useElementWidth(ref: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    setWidth(el.clientWidth);
    if (typeof ResizeObserver === 'undefined') return;
    const ro = new ResizeObserver(() => setWidth(el.clientWidth));
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return width;
}

/** Which rows of a long list to render while its scroll parent scrolls. Rows
 *  are all `pitch` px apart, so the rest of the list is two padding blocks.
 *  `ready` is false until the list knows its width: it then renders its first
 *  `fallback` rows only. */
export function useRowWindow(ref: RefObject<HTMLElement | null>, opts: { rows: number; pitch: number; ready: boolean; fallback: number; overscanViewports?: number }) {
  const { rows, pitch, ready, fallback, overscanViewports = 0.5 } = opts;
  const [range, setRange] = useState<[number, number]>([0, Math.min(rows, fallback)]);
  const latest = useRef({ rows, pitch, ready, overscanViewports });
  latest.current = { rows, pitch, ready, overscanViewports };
  const frame = useRef(0);

  const measure = useCallback(() => {
    frame.current = 0;
    const el = ref.current;
    const { rows, pitch, ready, overscanViewports } = latest.current;
    if (!el || !ready) return;
    const parent = scrollParentOf(el);
    const viewport = parent.clientHeight;
    const top = el.getBoundingClientRect().top - parent.getBoundingClientRect().top + parent.scrollTop;
    const next = visibleRows({ scrollTop: parent.scrollTop, viewport, top, pitch, rows, overscan: Math.max(pitch, viewport * overscanViewports) });
    setRange((cur) => (cur[0] === next[0] && cur[1] === next[1] ? cur : next));
  }, [ref]);

  // The window follows every layout change (rows, width, filters opening above the list)...
  useLayoutEffect(() => { measure(); }, [measure, rows, pitch, ready]);

  // ...and the scrolling and resizing of the scroll parent, at most once per frame.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const parent = scrollParentOf(el);
    const target: HTMLElement | Window = parent === document.scrollingElement || parent === document.documentElement ? window : parent;
    const schedule = () => { if (!frame.current) frame.current = requestAnimationFrame(measure); };
    target.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', schedule);
    return () => {
      target.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', schedule);
      if (frame.current) cancelAnimationFrame(frame.current);
      frame.current = 0;
    };
  }, [ref, measure]);

  // A shorter list can leave the old range past its end until the next measure.
  const start = Math.min(range[0], rows);
  const end = ready ? Math.min(range[1], rows) : Math.min(rows, fallback);
  return { start, end, padTop: ready ? start * pitch : 0, padBottom: ready ? Math.max(0, rows - end) * pitch : 0 };
}
