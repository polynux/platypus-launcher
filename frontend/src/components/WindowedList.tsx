import React, { useRef } from 'react';
import { columnWidth, gridColumns, useElementWidth, useRowWindow } from '../lib/windowing';

/** Rendered before the list has measured itself (and always in jsdom, which has no layout): about a screenful. */
const FALLBACK_ITEMS = 30;

interface WindowedGridProps<T> {
  items: T[];
  itemKey: (item: T) => string;
  renderItem: (item: T) => React.ReactNode;
  /** Minimum column width, like `minmax(minColumn px, 1fr)`. */
  minColumn: number;
  gap: number;
  /** Height of every row, from the width of one column. */
  rowHeight: (columnWidth: number) => number;
  /** The plain CSS grid (`display: grid`, auto-fill columns, the gap) used until the width is known. */
  className: string;
}

/** A grid like `repeat(auto-fill, minmax(minColumn, 1fr))` that only keeps the
 *  rows near the viewport in the DOM (a library can hold thousands of cards).
 *  Every row has the same height, so the rows that are not rendered are padding. */
export function WindowedGrid<T>({ items, itemKey, renderItem, minColumn, gap, rowHeight, className }: WindowedGridProps<T>) {
  const ref = useRef<HTMLDivElement>(null);
  const width = useElementWidth(ref);
  const ready = width > 0;
  const cols = ready ? gridColumns(width, minColumn, gap) : 1;
  const rowH = ready ? rowHeight(columnWidth(width, cols, gap)) : 0;
  const rows = ready ? Math.ceil(items.length / cols) : items.length;
  const { start, end, padTop, padBottom } = useRowWindow(ref, { rows, pitch: rowH + gap, ready, fallback: FALLBACK_ITEMS });
  const shown = ready ? items.slice(start * cols, end * cols) : items.slice(0, FALLBACK_ITEMS);

  return (
    <div ref={ref} style={{ paddingTop: padTop, paddingBottom: padBottom }}>
      <div
        className={className}
        style={ready ? { gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`, gridAutoRows: `${rowH}px`, gap } : undefined}
      >
        {shown.map((item) => <React.Fragment key={itemKey(item)}>{renderItem(item)}</React.Fragment>)}
      </div>
    </div>
  );
}

interface WindowedRowsProps<T> {
  items: T[];
  itemKey: (item: T) => string;
  renderRow: (item: T, index: number) => React.ReactNode;
  rowHeight: number;
}

/** A column of equal-height rows with only the ones near the viewport in the DOM. */
export function WindowedRows<T>({ items, itemKey, renderRow, rowHeight }: WindowedRowsProps<T>) {
  const ref = useRef<HTMLDivElement>(null);
  const width = useElementWidth(ref);
  const ready = width > 0;
  const { start, end, padTop, padBottom } = useRowWindow(ref, { rows: items.length, pitch: rowHeight, ready, fallback: FALLBACK_ITEMS });
  const from = ready ? start : 0;
  return (
    <div ref={ref} style={{ paddingTop: padTop, paddingBottom: padBottom }}>
      {items.slice(from, end).map((item, i) => (
        <React.Fragment key={itemKey(item)}>{renderRow(item, from + i)}</React.Fragment>
      ))}
    </div>
  );
}
