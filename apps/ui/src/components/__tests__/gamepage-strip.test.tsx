import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Effect } from 'effect';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getCrashHints: () => Effect.succeed(null),
    getGameDetails: () =>
      Effect.succeed({
        summary: 's', screenshots: ['a.png', 'b.png', 'c.png'], genres: [], developers: [], publishers: [], release_date: '', achievements: null,
      } as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { GamePage } from '../GamePage';
import type { Game } from '../../types';

const game = { id: 'strip-1', source: 'steam', store: 'steam', store_app_id: '1', title: 'Strip', cover_url: '', banner_url: '', install_dir: '/g', executable: '', is_installed: true, settings: { runner_id: 'auto', wine_prefix: '', launch_arguments: '', env_vars: {}, use_gamemode: false, use_mangohud: false, use_gamescope: false, gamescope_w: 0, gamescope_h: 0, gamescope_r: 0, gamescope_args: '', eac_runtime: false, battleye_runtime: false, working_dir: '' } } as unknown as Game;
const props = {
  runners: [], activity: [], backLabel: 'Library', onBack() {}, onPlay() {}, onStop() {}, onForceStop() {}, launching: false, tweaks: [],
  onChangeSettings() {}, onSaveSettings() { return Promise.resolve(); }, onJob() {}, onJobAction() {}, onAllowFolder() {}, onNavigate() {},
};

describe('screenshot strip', () => {
  it('renders in a padded ScrollArea and scrolls the active thumbnail into view', async () => {
    const calls: Element[] = [];
    Element.prototype.scrollIntoView = function (this: Element) { calls.push(this); };
    render(<GamePage game={game} {...props} />);
    const strip = await screen.findByTestId('shot-strip');
    expect(strip.getAttribute('data-slot')).toBe('scroll-area');
    const inner = screen.getByTestId('shot-strip-inner');
    expect(strip.contains(inner)).toBe(true);
    expect(inner.className).toMatch(/\bpx-1\.5\b/);
    expect(inner.className).toMatch(/\bpt-1\.5\b/);
    expect(strip.className).not.toMatch(/mask/);
    calls.length = 0;
    fireEvent.click(screen.getByLabelText('Show screenshot 3'));
    expect(calls).toContain(screen.getByLabelText('Show screenshot 3'));
  });

  it('is always scrollable with its bar showing, and a wheel turn scrolls it sideways', async () => {
    render(<GamePage game={game} {...props} />);
    const strip = await screen.findByTestId('shot-strip');
    const viewport = strip.querySelector<HTMLElement>('[data-radix-scroll-area-viewport]')!;
    Object.defineProperty(viewport, 'scrollWidth', { configurable: true, value: 1000 });
    Object.defineProperty(viewport, 'clientWidth', { configurable: true, value: 300 });
    viewport.scrollLeft = 0;
    const wheel = new WheelEvent('wheel', { deltaY: 120, cancelable: true, bubbles: true });
    viewport.dispatchEvent(wheel);
    expect(wheel.defaultPrevented).toBe(true);
    expect(viewport.scrollLeft).toBe(120);
    // A sideways wheel is left to the browser.
    const sideways = new WheelEvent('wheel', { deltaX: 40, cancelable: true, bubbles: true });
    viewport.dispatchEvent(sideways);
    expect(sideways.defaultPrevented).toBe(false);
  });
});
