import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { PluginLogo, monogramFor, tintFor } from '../PluginLogo';

const LOGO = '/platypus-art/plugin/echo?v=abc123';

const imgOf = (c: HTMLElement) => c.querySelector('img');

describe('PluginLogo fallback (issue #21)', () => {
  it('shows the logo when there is one', () => {
    const { container } = render(<PluginLogo src={LOGO} name="Echo" />);
    expect(imgOf(container)).toHaveAttribute('src', LOGO);
  });

  it('shows a monogram tile when there is none', () => {
    const { container } = render(<PluginLogo src="" name="Echo" />);
    expect(imgOf(container)).toBeNull();
    expect(screen.getByText('E')).toBeInTheDocument();
  });

  it('falls back to the monogram when the logo fails to load', () => {
    const { container } = render(<PluginLogo src={LOGO} name="Echo" />);
    const first = imgOf(container);
    expect(first).toHaveAttribute('src', LOGO);
    fireEvent.error(first!);
    expect(imgOf(container)).toBeNull();
    expect(screen.getByText('E')).toBeInTheDocument();
  });

  it('tries again when the source changes after a failure', () => {
    const { container, rerender } = render(<PluginLogo src={LOGO} name="Echo" />);
    fireEvent.error(imgOf(container)!);
    expect(imgOf(container)).toBeNull();
    rerender(<PluginLogo src={`${LOGO}x`} name="Echo" />);
    expect(imgOf(container)).toHaveAttribute('src', `${LOGO}x`);
  });

  it('keeps a consistent tint per name', () => {
    const a = render(<PluginLogo src="" name="Echo" />);
    const b = render(<PluginLogo src="" name="echo  " />);
    expect((a.container.firstChild as HTMLElement)?.className).toBe((b.container.firstChild as HTMLElement)?.className);
    expect(tintFor('Echo')).toBe(tintFor('echo'));
    expect(monogramFor('  epic games ')).toBe('E');
    expect(monogramFor('')).toBe('?');
  });
});
