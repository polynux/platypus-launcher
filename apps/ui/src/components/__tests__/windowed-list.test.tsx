import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { WindowedGrid, WindowedRows } from '../WindowedList';

// jsdom has no layout: give every element the box the list measures.
const VIEWPORT = 600;
const WIDTH = 1000;

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockImplementation(function (this: HTMLElement) {
    return WIDTH;
  });
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) {
    return this.hasAttribute('data-scroll-root') ? VIEWPORT : 0;
  });
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    // The scroll root sits at 0, the list 100px below it (a header), scrolled with the root.
    const root = this.closest<HTMLElement>('[data-scroll-root]');
    const top = this.hasAttribute('data-scroll-root') ? 0 : 100 - (root?.scrollTop ?? 0);
    return { top, left: 0, right: WIDTH, bottom: top, width: WIDTH, height: 0, x: 0, y: top, toJSON() {} } as DOMRect;
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const items = Array.from({ length: 1000 }, (_, i) => ({ id: `g${i}` }));

function scrollTo(root: HTMLElement, top: number) {
  act(() => {
    Object.defineProperty(root, 'scrollTop', { value: top, configurable: true }); // jsdom does not scroll
    root.dispatchEvent(new Event('scroll'));
  });
}

describe('WindowedGrid', () => {
  const renderGrid = () =>
    render(
      <div data-scroll-root data-testid="root">
        <WindowedGrid
          items={items}
          itemKey={(g) => g.id}
          renderItem={(g) => <div data-testid="item">{g.id}</div>}
          minColumn={140}
          gap={12}
          rowHeight={() => 88}
          className="grid"
        />
      </div>,
    );

  it('keeps only the rows near the viewport in the DOM, and pads the rest', () => {
    const { getAllByTestId, container } = renderGrid();
    const shown = getAllByTestId('item').map((n) => n.textContent);
    // 1000px wide: 7 columns of 130px fit 7*140+6*12 = 1052 > 1000, so 6 columns (and 100px per row pitch).
    expect(shown[0]).toBe('g0');
    expect(shown.length).toBeLessThan(120);
    expect(shown.length % 6).toBe(0);
    const outer = container.querySelector('[data-testid="root"] > div') as HTMLElement;
    expect(outer.style.paddingTop).toBe('0px');
    // 167 rows of 100px pitch in all; the padding under the window stands for the rows not rendered.
    const rows = shown.length / 6;
    expect(parseFloat(outer.style.paddingBottom)).toBe((167 - rows) * 100);
  });

  it('moves the window when the scroll root scrolls', async () => {
    const { getAllByTestId, getByTestId } = renderGrid();
    scrollTo(getByTestId('root'), 10000);
    await act(async () => { await new Promise((r) => requestAnimationFrame(r)); }); // the measure runs once per frame
    const shown = getAllByTestId('item').map((n) => Number(n.textContent!.slice(1)));
    // 10000px of scroll, a 100px header and 100px rows: the viewport shows rows 99 to 105. The overscan
    // (half a viewport, 300px) adds three rows before it and after it: rows 96 to 107.
    expect(shown[0]).toBe(96 * 6);
    expect(shown.at(-1)).toBe(108 * 6 - 1);
  });
});

describe('WindowedRows', () => {
  it('renders the rows near the viewport with their index', () => {
    const { getAllByTestId } = render(
      <div data-scroll-root>
        <WindowedRows items={items} itemKey={(g) => g.id} rowHeight={44} renderRow={(g, i) => <div data-testid="row">{`${i}:${g.id}`}</div>} />
      </div>,
    );
    const rows = getAllByTestId('row').map((n) => n.textContent);
    expect(rows[0]).toBe('0:g0');
    expect(rows.length).toBeLessThan(40);
    expect(rows.length).toBeGreaterThan(10);
  });
});
