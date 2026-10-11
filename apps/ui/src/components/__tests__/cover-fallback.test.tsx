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
