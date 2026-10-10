import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/react';
import type { StorePlugin } from '../../types';
import { PluginCapabilities } from '../PluginCapabilities';

const cap = (name: string) => ({ name, on: true, known: true, orchestrated: true, features: {} });
const echo = {
  id: 'echo', name: 'Echo', status: 'ready', initialized: true, protocol: '1.8', warnings: [],
  offers: [cap('library'), cap('install'), cap('redeem')],
} as unknown as StorePlugin;

describe('plugin capabilities fold away', () => {
  it('starts collapsed, with the count in the header', () => {
    render(<PluginCapabilities plugin={echo} />);
    const toggle = screen.getByRole('button', { name: /capabilities · 3/ });
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    expect(screen.queryByRole('list', { name: 'Capabilities' })).toBeNull();
  });

  it('opens the list on click and closes it again', () => {
    render(<PluginCapabilities plugin={echo} />);
    const toggle = screen.getByRole('button', { name: /capabilities · 3/ });
    fireEvent.click(toggle);
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('list', { name: 'Capabilities' }).querySelectorAll('li')).toHaveLength(3);
    fireEvent.click(toggle);
    expect(screen.queryByRole('list', { name: 'Capabilities' })).toBeNull();
  });

  it('shows nothing for a plugin that offers nothing and says nothing', () => {
    const { container } = render(<PluginCapabilities plugin={{ ...echo, offers: [], warnings: [] } as unknown as StorePlugin} />);
    expect(container.textContent).toBe('');
  });
});
