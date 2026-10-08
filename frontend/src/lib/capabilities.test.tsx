import React from 'react';
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { availability, offers, CapabilitySurface } from './capabilities';
import type { Game, StorePlugin } from '../types';

const plugin = (o: Partial<StorePlugin> = {}): StorePlugin =>
  ({ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['library', 'install', 'versions'], declared: ['library', 'install', 'versions', 'verify'], ...o }) as StorePlugin;
const game = (o: Partial<Game> = {}): Game => ({ id: 'p.echo.tiny', source: 'echo', ...o }) as Game;

describe('availability', () => {
  it('is on when the plugin offers the capability', () => {
    expect(availability(plugin(), 'install')).toEqual({ available: true });
    expect(offers(plugin(), 'versions')).toBe(true);
  });

  it('is off without a plugin (Steam, sideloaded games)', () => {
    expect(availability(undefined, 'install')).toEqual({ available: false, why: 'no-plugin' });
    expect(offers(undefined, 'install')).toBe(false);
  });

  it('tells a capability the plugin never declared from one it switched off', () => {
    expect(availability(plugin(), 'redeem')).toMatchObject({ available: false, why: 'not-offered' });
    expect(availability(plugin(), 'verify')).toMatchObject({ available: false, why: 'switched-off' });
  });

  it('follows the runtime set, not the declared one', () => {
    const p = plugin({ capabilities: ['library'] });
    expect(offers(p, 'install')).toBe(false);
  });

  it('applies a game that lists its features', () => {
    const p = plugin();
    expect(availability(p, 'versions', { game: game({ features: ['install'] }) })).toMatchObject({ available: false, why: 'game' });
    expect(offers(p, 'versions', { game: game({ features: ['versions'] }) })).toBe(true);
  });

  it('treats a game without features as having all of them', () => {
    expect(offers(plugin(), 'versions', { game: game() })).toBe(true);
    expect(offers(plugin(), 'versions', { game: game({ features: null }) })).toBe(true);
  });

  it('an empty feature list means none', () => {
    expect(offers(plugin(), 'install', { game: game({ features: [] }) })).toBe(false);
  });

  it('carries the reason the plugin wrote when unavailable', () => {
    const a = availability(plugin(), 'versions', { game: game({ features: [] }), reason: 'Only one build exists.' });
    expect(a).toEqual({ available: false, why: 'game', reason: 'Only one build exists.' });
    expect(availability(plugin(), 'install', { reason: 'ignored while available' })).toEqual({ available: true });
  });
});

describe('CapabilitySurface', () => {
  it('renders its children when offered', () => {
    const { container } = render(<CapabilitySurface plugin={plugin()} capability="install"><button>Uninstall</button></CapabilitySurface>);
    expect(container.textContent).toBe('Uninstall');
  });

  it('renders nothing when not offered and no reason is given', () => {
    const { container } = render(<CapabilitySurface plugin={plugin()} capability="redeem"><button>Redeem</button></CapabilitySurface>);
    expect(container.textContent).toBe('');
  });

  it('shows only the plugin-written reason in place of an unavailable surface', () => {
    const { container, getByRole } = render(
      <CapabilitySurface plugin={plugin()} capability="versions" game={game({ features: [] })} reason="Steam Cloud needs a login">
        <button>Versions</button>
      </CapabilitySurface>,
    );
    expect(container.querySelector('button')).toBeNull();
    expect(getByRole('note').textContent).toBe('Steam Cloud needs a login');
  });

  it('shows no reason for a capability the plugin does not have at all', () => {
    const { container } = render(
      <CapabilitySurface plugin={plugin()} capability="redeem" reason="whatever"><button>Redeem</button></CapabilitySurface>,
    );
    expect(container.textContent).toBe('');
  });
});
