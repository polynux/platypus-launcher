// useThemeLayout live-switch (issue #154): a theme with no layout falls back
// to built-in instead of keeping the previous theme's layout, and overlapping
// reads resolve in order.
import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import { Effect } from 'effect';
import type { ThemeLayout } from '../types';

const layoutState = vi.hoisted(() => ({ impl: null as null | ((view: string) => unknown) }));
vi.mock('../effect/runtime', () => ({
  LauncherApi: { getThemeLayout: (view: string) => layoutState.impl?.(view) ?? Effect.succeed(null) },
}));

import { applyTheme } from './theme';
import { useThemeLayout } from './theme';
import { BUILTIN_LAYOUTS } from '../gen/builtinLayouts';

function Probe({ view }: { view: string }) {
  const layout = useThemeLayout(view);
  return <div data-testid="regions">{(layout?.regions ?? []).map((r) => r.name).join(',')}</div>;
}

const coolLayout: ThemeLayout = {
  view: 'dashboard',
  regions: [{ name: 'cool-main', nodes: [{ block: 'continue' }] }],
};

const warmTokens = (variants?: Record<string, string>) => ({
  id: 'warm',
  tokens: {},
  wash: [],
  variants: variants ?? {},
}) as never;

const coolTokens = () =>
  ({
    id: 'cool',
    tokens: {},
    wash: [],
    variants: {},
  }) as never;

beforeEach(() => {
  layoutState.impl = null;
  window.localStorage.clear();
});

afterEach(() => cleanup());

describe('useThemeLayout live switch', () => {
  it('falls back to built-in when the new theme ships no layout', async () => {
    layoutState.impl = () => Effect.succeed(coolLayout);
    render(<Probe view="dashboard" />);
    await waitFor(() => expect(screen.getByTestId('regions').textContent).toBe('cool-main'));

    // Warm ships no dashboard layout: the core answers null.
    layoutState.impl = () => Effect.succeed(null);
    applyTheme(warmTokens());
    await waitFor(() =>
      expect(screen.getByTestId('regions').textContent).toBe(
        (BUILTIN_LAYOUTS['dashboard'].regions ?? []).map((r) => r.name).join(','),
      ),
    );
  });

  it('ignores a slow stale read that resolves after the latest theme', async () => {
    let resolveSlow!: (v: ThemeLayout | null) => void;
    const slow = new Promise<ThemeLayout | null>((r) => { resolveSlow = r; });
    // First fetch hangs; the theme switch triggers a fast null (built-in).
    layoutState.impl = () => Effect.tryPromise({ try: () => slow, catch: () => new Error('no') });
    render(<Probe view="dashboard" />);
    // Let the slow fetch start.
    await new Promise((r) => setTimeout(r, 10));

    layoutState.impl = () => Effect.succeed(null);
    applyTheme(coolTokens());
    await waitFor(() =>
      expect(screen.getByTestId('regions').textContent).toBe(
        (BUILTIN_LAYOUTS['dashboard'].regions ?? []).map((r) => r.name).join(','),
      ),
    );

    // The stale cool layout resolves last and must not win.
    resolveSlow(coolLayout);
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.getByTestId('regions').textContent).toBe(
      (BUILTIN_LAYOUTS['dashboard'].regions ?? []).map((r) => r.name).join(','),
    );
  });
});
