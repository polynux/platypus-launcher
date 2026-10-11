import React from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Cover } from '../ui';
import { coverMisses } from '../../lib/coverMisses';

const BAD = 'https://cdn.example/steam/apps/400/library_600x900.jpg';
const HEADER = 'https://cdn.example/steam/apps/400/header.jpg';

const imgOf = (c: HTMLElement) => c.querySelector('img');

beforeEach(() => {
  coverMisses.clear();
});

describe('Cover fallback (issue #26)', () => {
  it('skips a known-404 portrait without requesting it and shows the header', () => {
    coverMisses.markBad(BAD);
    const { container } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    expect(imgOf(container)).toHaveAttribute('src', HEADER);
  });

  it('falls back to the next source on error and remembers the miss', () => {
    const { container, rerender } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    const first = imgOf(container);
    expect(first).toHaveAttribute('src', BAD);
    fireEvent.error(first!);
    expect(imgOf(container)).toHaveAttribute('src', HEADER);
    expect(coverMisses.isBad(BAD)).toBe(true);
    // A later mount skips the known 404 outright.
    rerender(<Cover src={[BAD, HEADER]} title="Other" className="x" />);
    expect(imgOf(container)).toHaveAttribute('src', HEADER);
  });

  it('shows the title placeholder when every source is a known miss', () => {
    coverMisses.markBad(BAD);
    coverMisses.markBad(HEADER);
    const { container } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    expect(imgOf(container)).toBeNull();
    expect(screen.getByText('Portal')).toBeInTheDocument();
  });
});

describe('Cover live fallback (follow-up to #26)', () => {
  /** jsdom never loads images (complete is false, dimensions are 0), so the
   *  tests below fake what Chromium reports for a URL that answers 200 with
   *  an empty body or a tracking-pixel placeholder: a load event whose image
   *  has no pixels. Neither fires onError, so without the load check the
   *  card would sit on a blank image until a restart. */
  function pixels(img: HTMLImageElement, w: number, h: number) {
    Object.defineProperty(img, 'naturalWidth', { configurable: true, value: w });
    Object.defineProperty(img, 'naturalHeight', { configurable: true, value: h });
  }

  it('falls back live when the image loads with no pixels (a 200 with an empty body)', () => {
    const { container } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    const first = imgOf(container);
    expect(first).toHaveAttribute('src', BAD);
    pixels(first!, 0, 0);
    fireEvent.load(first!);
    expect(imgOf(container)).toHaveAttribute('src', HEADER);
    expect(coverMisses.isBad(BAD)).toBe(true);
  });

  it('treats a 1x1 placeholder as a miss and shows the next source', () => {
    const { container } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    const first = imgOf(container);
    pixels(first!, 1, 1);
    fireEvent.load(first!);
    expect(imgOf(container)).toHaveAttribute('src', HEADER);
    expect(coverMisses.isBad(BAD)).toBe(true);
  });

  it('keeps an image that decodes to real pixels', () => {
    const { container } = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    const first = imgOf(container);
    pixels(first!, 460, 215);
    fireEvent.load(first!);
    expect(imgOf(container)).toHaveAttribute('src', BAD);
    expect(coverMisses.isBad(BAD)).toBe(false);
  });

  it('a fresh mount after unmount (windowed list scrolled back) skips the remembered miss', () => {
    const first = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    const img = imgOf(first.container);
    pixels(img!, 0, 0);
    fireEvent.load(img!);
    expect(coverMisses.isBad(BAD)).toBe(true);
    first.unmount();
    const second = render(<Cover src={[BAD, HEADER]} title="Portal" className="x" />);
    expect(imgOf(second.container)).toHaveAttribute('src', HEADER);
  });
});
