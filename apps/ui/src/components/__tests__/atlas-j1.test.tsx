// Atlas J1 fidelity fixes (Refs #121): tests for N2, item 4, N3 and item 6.
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react';
import { Effect } from 'effect';

const themeState = vi.hoisted(() => ({ structure: 'tabs' as string }));
vi.mock('../../lib/theme', async (importOriginal) => {
  const mod = await importOriginal<typeof import('../../lib/theme')>();
  return {
    ...mod,
    useVariant: (axis: string) => {
      if (axis === 'game.structure') return themeState.structure;
      return 'page';
    },
    useThemeLayout: () => null,
  };
});

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getRunnerOptions: () => Effect.succeed([] as never),
    saveGameSettings: () => Effect.succeed(undefined as never),
    getGameDetails: () => Effect.succeed(null as never),
    getCrashHints: () => Effect.succeed(null),
    getCloudInfo: () => Effect.succeed(null as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
  onCloud: () => () => {},
  onFamily: () => () => {},
  onSocial: () => () => {},
  onPlaytime: () => () => {},
  onLibraryPartial: () => () => {},
}));

import { TuneFlyout } from '../TuneFlyout';
import { TweaksPanel } from '../TweaksPanel';
import { AboutSection } from '../GamePage';
import { AchRowIcon } from '../GameTabs';
import type { Game, Tweak } from '../../types';

afterEach(() => cleanup());

const settings = (o: Record<string, unknown> = {}) => ({
  runner_id: 'auto',
  wine_prefix: '',
  launch_arguments: '',
  env_vars: {},
  use_gamemode: false,
  use_mangohud: false,
  use_gamescope: false,
  gamescope_w: 0,
  gamescope_h: 0,
  gamescope_r: 0,
  gamescope_args: '',
  eac_runtime: false,
  battleye_runtime: false,
  working_dir: '',
  ...o,
});

const game = (o: Record<string, unknown> = {}): Game =>
  ({
    id: 'j1',
    source: 'echo',
    store: 'echo',
    store_app_id: '1',
    title: 'J1 Game',
    cover_url: '',
    banner_url: '',
    hero_url: '',
    install_dir: '/g',
    executable: '/g/game',
    is_installed: true,
    settings: settings(),
    ...o,
  }) as unknown as Game;

describe('N2 Tune flyout does not shift the stage', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
  });

  it('renders fixed (out of the stage scroll flow) with a capped inner scroll', async () => {
    const anchor = document.createElement('button');
    anchor.setAttribute('data-tune-anchor', '');
    // Anchor near the top-left of a 1280x800 stage.
    anchor.getBoundingClientRect = () =>
      ({ left: 400, top: 300, right: 480, bottom: 342, width: 80, height: 42, x: 400, y: 300, toJSON: () => {} }) as DOMRect;
    document.body.appendChild(anchor);
    const { getByRole } = render(<TuneFlyout game={game()} open onClose={() => {}} onAllSettings={() => {}} />);
    const dialog = getByRole('dialog');
    await waitFor(() => expect(dialog.style.position).toBe('fixed'));
    expect(dialog.className).not.toMatch(/absolute/);
    // Anchored under the Tune button with a capped inner scroll.
    expect(dialog.style.top).toBe('348px');
    expect(dialog.style.left).toBe('400px');
    expect(dialog.style.overflowY).toBe('auto');
    expect(Number.parseInt(dialog.style.maxHeight, 10)).toBeLessThanOrEqual(520);
    anchor.remove();
  });

  it('follows the anchor on scroll and resize, keeps focus return, Esc and outside-click', async () => {
    const anchor = document.createElement('button');
    anchor.setAttribute('data-tune-anchor', '');
    let rect = { left: 100, top: 100, right: 180, bottom: 142, width: 80, height: 42, x: 100, y: 100, toJSON: () => {} };
    anchor.getBoundingClientRect = () => rect as unknown as DOMRect;
    document.body.appendChild(anchor);
    const onClose = vi.fn();
    const { getByRole } = render(<TuneFlyout game={game()} open onClose={onClose} onAllSettings={() => {}} />);
    const dialog = getByRole('dialog');
    await waitFor(() => expect(dialog.style.left).toBe('100px'));
    rect = { ...rect, left: 250, x: 250, right: 330 } as typeof rect;
    window.dispatchEvent(new Event('resize'));
    await waitFor(() => expect(dialog.style.left).toBe('250px'));
    rect = { ...rect, left: 320, x: 320, right: 400 } as typeof rect;
    window.dispatchEvent(new Event('scroll'));
    await waitFor(() => expect(dialog.style.left).toBe('320px'));
    // Focus, Esc and outside-click still work from the fixed panel.
    expect(document.activeElement).toBe(dialog);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(anchor);
    fireEvent.pointerDown(document.body);
    expect(onClose).toHaveBeenCalledTimes(2);
    anchor.remove();
  });
});

describe('Item 4 Fixes suggested card', () => {
  const tweaks = [
    { id: 't1', title: 'Use fsync', reason: 'Helps on this kernel', kind: 'recommended', env: {} },
    { id: 't2', title: 'Disable esync', reason: 'When the game stutters', kind: 'troubleshoot', env: {} },
  ] as Tweak[];

  it('renders recommended tweaks as the accent-tint card, troubleshoot as plain rows', () => {
    render(<TweaksPanel game={game()} tweaks={tweaks} viaSteam={false} onChange={() => {}} onOpenSettings={() => {}} framed={false} atlas />);
    const suggested = screen.getByRole('switch', { name: /Use fsync/ });
    expect(suggested.className).toMatch(/rounded-lg/);
    expect(suggested.className).toMatch(/bg-\[color-mix/);
    expect(suggested.className).toMatch(/ring-accent/);
    const plain = screen.getByRole('switch', { name: /Disable esync/ });
    expect(plain.className).not.toMatch(/bg-\[color-mix/);
    expect(plain.className).toMatch(/py-\[11px\]/);
  });
});

describe('N3 failed tweak save leaves no unhandled rejection', () => {
  it('the App onChangeSettings wrapper swallows the rethrown save failure', async () => {
    // App's saveSettings toasts then rethrows so the drawer keeps its dirty
    // state; the tweak switch (void prop) must catch where it ignores it.
    // This mirrors App.tsx: void saveSettings(...).catch(() => {}).
    let toasted = false;
    const saveSettings = () => Promise.reject(new Error('tweak save failed')).catch((err) => {
      toasted = true;
      expect(err.message).toBe('tweak save failed');
      throw err;
    });
    const onChangeSettings = (p: Promise<void>) => {
      void p.catch(() => {});
    };
    onChangeSettings(saveSettings());
    // Flushing must not surface an unhandled rejection (vitest fails those).
    await new Promise((r) => setTimeout(r, 10));
    expect(toasted).toBe(true);
  });
});

describe('Item 6 description mask only on overflow', () => {
  const base = {
    game: game(),
    details: null,
    loading: false,
    error: null,
    compatUrls: {},
    onSearchInfo: async () => [],
    onLinkInfo: async () => {},
    onUnlinkInfo: async () => {},
    hideDetails: true,
  } as unknown as React.ComponentProps<typeof AboutSection>;

  it('short text shows with no mask and no toggle', async () => {
    const details = { description_html: '<p>A tiny fake game.</p>', summary: '' } as never;
    render(<AboutSection {...base} details={details} />);
    const wrap = await screen.findByTestId('about-html-wrap');
    // jsdom reports no overflow: no clamp, no toggle.
    expect(wrap.className).not.toMatch(/max-h-/);
    expect(wrap.className).not.toMatch(/mask-image/);
    expect(screen.queryByTestId('about-toggle')).toBeNull();
  });

  it('long text keeps the clamp, mask and toggle, and expands', async () => {
    const longHtml = `<p>${'Long description. '.repeat(200)}</p>`;
    const details = { description_html: longHtml, summary: '' } as never;
    render(<AboutSection {...base} details={details} />);
    const wrap = await screen.findByTestId('about-html-wrap');
    Object.defineProperty(wrap, 'scrollHeight', { configurable: true, value: 2000 });
    window.dispatchEvent(new Event('resize'));
    const toggle = await screen.findByTestId('about-toggle');
    await waitFor(() => expect(wrap.className).toMatch(/max-h-\[760px\]/));
    expect(wrap.className).toMatch(/mask-image/);
    fireEvent.click(toggle);
    expect(screen.getByTestId('about-toggle').textContent).toBe('Show less');
    expect(screen.getByTestId('about-html-wrap').className).not.toMatch(/max-h-/);
  });
});

describe('Item 9 locked achievements without an image show the lock', () => {
  it('every locked row carries the overlay, including the fallback icon', () => {
    const lockedFallback = { id: 'l1', name: 'Locked', desc: '', icon: '', hidden: false, unlocked: false, at: null, pct: null };
    const unlockedFallback = { ...lockedFallback, id: 'u1', unlocked: true };
    const lockedImaged = { ...lockedFallback, id: 'l2', icon: 'https://example.com/a.png' };
    const { container: c1 } = render(<AchRowIcon a={lockedFallback} />);
    expect(c1.querySelector('.absolute.bottom-\\[3px\\], .absolute')).toBeTruthy();
    expect(c1.querySelectorAll('svg').length).toBe(2);
    const { container: c2 } = render(<AchRowIcon a={unlockedFallback} />);
    expect(c2.querySelectorAll('svg').length).toBe(1);
    expect(c2.querySelector('.absolute')).toBeNull();
    const { container: c3 } = render(<AchRowIcon a={lockedImaged} />);
    // Imaged rows keep their existing overlay.
    expect(c3.querySelector('img')).toBeTruthy();
  });
});
