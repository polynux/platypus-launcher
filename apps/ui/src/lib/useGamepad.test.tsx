// The gamepad hook (issue #53): standard mapping, dead zone, repeat, and
// polling only while a pad is connected. The navigator stub below is the
// point: without the hook nothing observes it.
import { afterEach, describe, expect, it, vi } from 'vitest';
import React, { useState } from 'react';
import { cleanup, render, waitFor } from '@testing-library/react';
import { padDirectionHolds, readPadSnapshot, useGamepad, type PadAction } from './useGamepad';

function pad(over: { buttons?: boolean[]; axes?: number[] } = {}): {
  connected: boolean;
  buttons: { pressed: boolean }[];
  axes: number[];
} {
  const pressed = over.buttons ?? [];
  const buttons = Array.from({ length: 17 }, (_, i) => ({ pressed: !!pressed[i] }));
  return { connected: true, buttons, axes: over.axes ?? [0, 0] };
}

function stubPads(first: ReturnType<typeof pad> | null): void {
  Object.defineProperty(window.navigator, 'getGamepads', {
    value: () => (first ? [first, null, null, null] : [null, null, null, null]),
    configurable: true,
  });
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  if ('getGamepads' in window.navigator) {
    delete (window.navigator as unknown as Record<string, unknown>).getGamepads;
  }
});

describe('padDirectionHolds', () => {
  it('reads the D-pad', () => {
    expect(padDirectionHolds({ buttons: [], axes: [] })).toEqual([]);
    const up = Array(16).fill(false);
    up[12] = true;
    expect(padDirectionHolds({ buttons: up, axes: [0, 0] })).toEqual(['up']);
    const diag = Array(16).fill(false);
    diag[13] = true;
    diag[14] = true;
    expect(padDirectionHolds({ buttons: diag, axes: [0, 0] })).toEqual(['down', 'left']);
  });

  it('reads the left stick past the dead zone only', () => {
    expect(padDirectionHolds({ buttons: [], axes: [0.2, 0] })).toEqual([]);
    expect(padDirectionHolds({ buttons: [], axes: [0.8, 0] })).toEqual(['right']);
    expect(padDirectionHolds({ buttons: [], axes: [-0.9, 0] })).toEqual(['left']);
    expect(padDirectionHolds({ buttons: [], axes: [0, 0.9] })).toEqual(['down']);
    expect(padDirectionHolds({ buttons: [], axes: [0, -0.9] })).toEqual(['up']);
  });
});

describe('readPadSnapshot', () => {
  it('is null without a pad', () => {
    expect(readPadSnapshot(null)).toBeNull();
    expect(readPadSnapshot(undefined)).toBeNull();
    expect(readPadSnapshot({ connected: false, buttons: [], axes: [] })).toBeNull();
  });
});

describe('useGamepad', () => {
  function Harness({ onAction }: { onAction: (a: PadAction) => void }) {
    const [tick] = useState(0);
    useGamepad(onAction);
    return <div data-testid="harness">{tick}</div>;
  }

  it('fires A (activate) on the stubbed pad', async () => {
    const seen: PadAction[] = [];
    stubPads(pad({ buttons: [true] }));
    render(<Harness onAction={(a) => seen.push(a)} />);
    window.dispatchEvent(new Event('gamepadconnected'));
    await waitFor(() => expect(seen).toContain('activate'));
  });

  it('fires B, LB and RB as edges', async () => {
    const seen: PadAction[] = [];
    stubPads(pad({ buttons: [false, true, false, false, true, true] }));
    render(<Harness onAction={(a) => seen.push(a)} />);
    window.dispatchEvent(new Event('gamepadconnected'));
    await waitFor(() => {
      expect(seen).toContain('back');
      expect(seen).toContain('prevSection');
      expect(seen).toContain('nextSection');
    });
  });

  it('fires a held D-pad direction', async () => {
    const seen: PadAction[] = [];
    const down = Array(16).fill(false);
    down[13] = true;
    stubPads(pad({ buttons: down }));
    render(<Harness onAction={(a) => seen.push(a)} />);
    window.dispatchEvent(new Event('gamepadconnected'));
    await waitFor(() => expect(seen).toContain('down'));
  });

  it('stays quiet with no pad connected', async () => {
    const seen: PadAction[] = [];
    stubPads(null);
    render(<Harness onAction={(a) => seen.push(a)} />);
    window.dispatchEvent(new Event('gamepadconnected'));
    await new Promise((r) => setTimeout(r, 120));
    expect(seen).toEqual([]);
  });
});
