import { describe, expect, it } from 'vitest';
import { columnWidth, gridColumns, visibleRows } from './windowing';

describe('gridColumns', () => {
  it('fits as many minimum-width columns as the width holds, like auto-fill', () => {
    expect(gridColumns(1000, 140, 12)).toBe(6); // 6*140 + 5*12 = 900 <= 1000 < 7*140 + 6*12
    expect(gridColumns(912, 140, 12)).toBe(6);
    expect(gridColumns(900, 140, 12)).toBe(6);
    expect(gridColumns(899, 140, 12)).toBe(5);
  });
  it('is never below one column', () => {
    expect(gridColumns(50, 140, 12)).toBe(1);
    expect(gridColumns(0, 140, 12)).toBe(1);
  });
});

describe('columnWidth', () => {
  it('shares the width left after the gaps', () => {
    expect(columnWidth(912, 6, 12)).toBe(142);
    expect(columnWidth(300, 1, 12)).toBe(300);
  });
});

describe('visibleRows', () => {
  const base = { viewport: 600, top: 100, pitch: 50, rows: 1000, overscan: 100 };
  it('starts at the first row while the list is still below the viewport', () => {
    expect(visibleRows({ ...base, scrollTop: 0 })).toEqual([0, 12]); // (0 - 100 + 600 + 100) / 50
  });
  it('follows the scroll position, with the overscan on both sides', () => {
    // list-relative window: 5000 - 100 - 100 = 4800 .. 4800 + 800 = 5600
    expect(visibleRows({ ...base, scrollTop: 5000 })).toEqual([96, 112]);
  });
  it('stops at the last row', () => {
    expect(visibleRows({ ...base, scrollTop: 1e9 })).toEqual([1000, 1000]);
    expect(visibleRows({ ...base, scrollTop: 49500 })).toEqual([986, 1000]);
  });
  it('renders nothing for an empty list', () => {
    expect(visibleRows({ ...base, rows: 0, scrollTop: 0 })).toEqual([0, 0]);
  });
});
