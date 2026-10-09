import React from 'react';
import type { Game, StorePlugin } from '../types';

/** The capability names Hydra knows (internal/plugins/capabilities.go). A UI
 *  surface names the one it depends on; the compiler catches a typo. */
export type Capability =
  | 'library' | 'install' | 'launch' | 'auth' | 'discard' | 'cloud-saves'
  | 'install-options' | 'versions' | 'verify' | 'move' | 'social' | 'achievements' | 'playtime' | 'redeem' | 'family' | 'settings'
  | 'chat' | 'terms';

/** Why a surface is unavailable:
 *  - no-plugin: the game doesn't come from a store plugin
 *  - not-offered: the plugin never declared the capability
 *  - switched-off: declared, but off right now (its initialize said so)
 *  - game: on for the plugin, not for this game (its `features` list) */
export type Why = 'no-plugin' | 'not-offered' | 'switched-off' | 'game';

export type Availability =
  | { available: true; why?: undefined; reason?: undefined }
  | { available: false; why: Why; reason?: string };

export interface AvailabilityOptions {
  game?: Game;
  /** What the plugin wrote about this state (e.g. a cloud status message).
   *  Hydra never makes one up; it is kept only where the capability exists. */
  reason?: string;
}

/** Whether a UI surface that depends on `capability` should render. It uses
 *  the runtime set (what the plugin offers right now), then the game's own
 *  `features` when it lists some. A game without `features` (null/absent) has
 *  all of the plugin's; an empty list has none. */
export function availability(plugin: StorePlugin | undefined, capability: Capability, opts: AvailabilityOptions = {}): Availability {
  if (!plugin) return { available: false, why: 'no-plugin' };
  if (!plugin.capabilities.includes(capability)) {
    return plugin.declared.includes(capability)
      ? { available: false, why: 'switched-off', ...(opts.reason ? { reason: opts.reason } : {}) }
      : { available: false, why: 'not-offered' };
  }
  const features = opts.game?.features;
  if (features != null && !features.includes(capability)) {
    return { available: false, why: 'game', ...(opts.reason ? { reason: opts.reason } : {}) };
  }
  return { available: true };
}

export const offers = (plugin: StorePlugin | undefined, capability: Capability, opts?: AvailabilityOptions): boolean =>
  availability(plugin, capability, opts).available;

/** Renders its children only when the plugin (and game) offers the capability.
 *  When the capability exists but is unavailable and the plugin wrote a reason,
 *  that text stands in; otherwise nothing renders. */
export function CapabilitySurface({
  plugin, capability, game, reason, children,
}: { plugin: StorePlugin | undefined; capability: Capability; game?: Game; reason?: string; children: React.ReactNode }) {
  const a = availability(plugin, capability, { game, reason });
  if (a.available) return <>{children}</>;
  if (a.reason) return <p role="note" className="font-mono text-[11.5px] text-white/50">{a.reason}</p>;
  return null;
}
