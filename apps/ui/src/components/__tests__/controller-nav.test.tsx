// Controller navigation behaviour (issue #53): arrows move focus through
// the one shared handler, layers trap focus, inputs and local key handlers
// keep their keys, A activates and B backs out. Rects are stubbed because
// jsdom lays out at 0x0.
import React, { useEffect, useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import {
  activateFocused,
  closeTopmostLayer,
  handleBackAction,
  installControllerKeyboard,
} from '../../lib/spatialNav';
import { dispatchPadArrow } from '../../lib/useGamepad';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../ds/dropdown-menu';

function rect(el: HTMLElement, x: number, y: number, w = 40, h = 24): void {
  el.getBoundingClientRect = () =>
    ({ left: x, top: y, width: w, height: h, right: x + w, bottom: y + h, x, y, toJSON: () => ({}) }) as DOMRect;
}

function layOut(grid: Record<string, [number, number]>): void {
  for (const [testid, [x, y]] of Object.entries(grid)) {
    rect(screen.getByTestId(testid), x, y);
  }
}

let uninstall: (() => void) | null = null;

afterEach(() => {
  cleanup();
  uninstall?.();
  uninstall = null;
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

function Grid() {
  return (
    <div>
      <button type="button" data-testid="up">up</button>
      <button type="button" data-testid="left">left</button>
      <button type="button" data-testid="center">center</button>
      <button type="button" data-testid="right">right</button>
      <button type="button" data-testid="down">down</button>
    </div>
  );
}

function gridPositions(): void {
  layOut({ up: [100, 0], left: [0, 100], center: [100, 100], right: [200, 100], down: [100, 200] });
}

describe('arrow keys move focus in every direction', () => {
  it.each([
    ['ArrowRight', 'right'],
    ['ArrowLeft', 'left'],
    ['ArrowUp', 'up'],
    ['ArrowDown', 'down'],
  ])('%s moves from the center', (key, target) => {
    uninstall = installControllerKeyboard();
    render(<Grid />);
    gridPositions();
    screen.getByTestId('center').focus();
    fireEvent.keyDown(screen.getByTestId('center'), { key });
    expect(document.activeElement).toBe(screen.getByTestId(target));
  });

  it('a pad direction moves through the same installed handler', () => {
    uninstall = installControllerKeyboard();
    render(<Grid />);
    gridPositions();
    const center = screen.getByTestId('center');
    center.focus();
    // Exactly what the App's gamepad callback dispatches for D-pad/stick.
    dispatchPadArrow('right');
    expect(document.activeElement).toBe(screen.getByTestId('right'));
  });
});

describe('layers trap focus', () => {
  it('arrows stay inside an open dialog', () => {
    uninstall = installControllerKeyboard();
    render(
      <div>
        <button type="button" data-testid="outside">outside</button>
        <div role="dialog" aria-label="settings">
          <button type="button" data-testid="first">first</button>
          <button type="button" data-testid="second">second</button>
        </div>
      </div>,
    );
    layOut({ outside: [0, 0], first: [100, 0], second: [200, 0] });
    screen.getByTestId('first').focus();
    fireEvent.keyDown(screen.getByTestId('first'), { key: 'ArrowRight' });
    expect(document.activeElement).toBe(screen.getByTestId('second'));
    // Past the last dialog control there is nowhere to go inside: focus
    // stays put instead of leaking behind the dialog.
    fireEvent.keyDown(screen.getByTestId('second'), { key: 'ArrowRight' });
    expect(document.activeElement).toBe(screen.getByTestId('second'));
  });

  it('focus outside an open dialog is pulled inside', () => {
    uninstall = installControllerKeyboard();
    render(
      <div>
        <button type="button" data-testid="outside">outside</button>
        <div role="dialog" aria-label="settings">
          <button type="button" data-testid="first">first</button>
        </div>
      </div>,
    );
    layOut({ outside: [0, 0], first: [100, 0] });
    screen.getByTestId('outside').focus();
    fireEvent.keyDown(screen.getByTestId('outside'), { key: 'ArrowDown' });
    expect(document.activeElement).toBe(screen.getByTestId('first'));
  });
});

describe('inputs and local handlers keep their keys', () => {
  it('arrows are ignored in a text input', () => {
    uninstall = installControllerKeyboard();
    render(
      <div>
        <input data-testid="field" defaultValue="abc" />
        <button type="button" data-testid="next">next</button>
      </div>,
    );
    layOut({ field: [0, 0], next: [100, 0] });
    const field = screen.getByTestId('field');
    field.focus();
    fireEvent.keyDown(field, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(field);
  });

  it('a local key zone that consumes the event wins', () => {
    uninstall = installControllerKeyboard();
    const onKeyDown = (e: React.KeyboardEvent): void => {
      if (e.key === 'ArrowDown') e.preventDefault();
    };
    // The zone marks itself so the shared capture handler yields to it.
    render(
      <div data-keys-local onKeyDown={onKeyDown}>
        <button type="button" data-testid="row">row</button>
        <button type="button" data-testid="below">below</button>
      </div>,
    );
    layOut({ row: [0, 0], below: [0, 100] });
    screen.getByTestId('row').focus();
    fireEvent.keyDown(screen.getByTestId('row'), { key: 'ArrowDown' });
    expect(document.activeElement).toBe(screen.getByTestId('row'));
  });
});

describe('A activates', () => {
  it('clicks the focused element', () => {
    const onClick = vi.fn();
    render(<button type="button" data-testid="play" onClick={onClick}>Play</button>);
    rect(screen.getByTestId('play'), 0, 0);
    screen.getByTestId('play').focus();
    expect(activateFocused()).toBe(true);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});

describe('B backs out', () => {
  function LayerHarness({ onBack }: { onBack: () => void }) {
    const [open, setOpen] = useState(true);
    useEffect(() => {
      if (!open) return;
      const key = (e: KeyboardEvent): void => {
        if (e.key !== 'Escape') return;
        setOpen(false);
      };
      window.addEventListener('keydown', key);
      return () => window.removeEventListener('keydown', key);
    }, [open ]);
    const back = (): void => {
      if (!closeTopmostLayer()) onBack();
    };
    return (
      <div>
        <button type="button" data-testid="back" onClick={back}>back</button>
        {open && (
          <div role="dialog" aria-label="layer">
            <button type="button" data-testid="inside">inside</button>
          </div>
        )}
      </div>
    );
  }

  it('closes the topmost layer first, then goes back through the real path', async () => {
    const onBack = vi.fn();
    render(<LayerHarness onBack={onBack} />);
    rect(screen.getByTestId('inside'), 0, 0);
    screen.getByTestId('inside').focus();
    // B with the layer open: the layer closes, no back navigation.
    expect(closeTopmostLayer()).toBe(true);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(onBack).not.toHaveBeenCalled();
    // B with nothing open: the App's back branch runs.
    handleBackAction(onBack);
    expect(onBack).toHaveBeenCalledTimes(1);
  });
});

describe('composite widgets', () => {
  function Tabs() {
    const tabs = ['a', 'b'];
    const [tab, setTab] = React.useState('a');
    return (
      <div>
        <div role="tablist" aria-label="sections" aria-orientation="vertical">
          {tabs.map((name) => (
            <button
              key={name}
              type="button"
              role="tab"
              id={`t-${name}`}
              data-testid={`tab-${name}`}
              aria-selected={tab === name}
              tabIndex={tab === name ? 0 : -1}
              onClick={() => setTab(name)}
              onKeyDown={(e) => {
                const i = tabs.indexOf(name);
                const next =
                  e.key === 'ArrowDown' || e.key === 'ArrowRight' ? tabs[(i + 1) % tabs.length]
                  : e.key === 'ArrowUp' || e.key === 'ArrowLeft' ? tabs[(i - 1 + tabs.length) % tabs.length]
                  : null;
                if (!next) return;
                e.preventDefault();
                setTab(next);
                document.getElementById(`t-${next}`)?.focus();
              }}
            >
              {name}
            </button>
          ))}
        </div>
        <div data-testid="panel">
          <button type="button" data-testid="content">content</button>
        </div>
        <button type="button" data-testid="below">below</button>
      </div>
    );
  }

  function tabLayout(): void {
    layOut({ 'tab-a': [0, 0], 'tab-b': [0, 100], content: [200, 0], below: [0, 300] });
  }

  it('along-axis arrows stay in the tablist, across-axis leave it', () => {
    uninstall = installControllerKeyboard();
    render(<Tabs />);
    tabLayout();
    screen.getByTestId('tab-a').focus();
    fireEvent.keyDown(screen.getByTestId('tab-a'), { key: 'ArrowDown' });
    // The tablist's own handler moves within: selection follows, focus too.
    expect(screen.getByTestId('tab-b')).toHaveAttribute('aria-selected', 'true');
    expect(document.activeElement).toBe(screen.getByTestId('tab-b'));
    // Across the axis the widget must not pull focus back.
    fireEvent.keyDown(screen.getByTestId('tab-b'), { key: 'ArrowRight' });
    expect(document.activeElement).toBe(screen.getByTestId('content'));
    expect(screen.getByTestId('tab-b')).toHaveAttribute('aria-selected', 'true');
    // Back across lands on the active tab, not the nearest one.
    fireEvent.keyDown(screen.getByTestId('content'), { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(screen.getByTestId('tab-b'));
  });

  it('along-axis at the last item leaves instead of wrapping', () => {
    uninstall = installControllerKeyboard();
    render(<Tabs />);
    tabLayout();
    screen.getByTestId('tab-b').focus();
    fireEvent.keyDown(screen.getByTestId('tab-b'), { key: 'ArrowDown' });
    expect(document.activeElement).toBe(screen.getByTestId('below'));
    // No wrap: the selection stays where it was.
    expect(screen.getByTestId('tab-a')).toHaveAttribute('aria-selected', 'true');
  });
});

describe('A opens menus like Enter', () => {
  it('opens a DropdownMenu trigger', async () => {
    uninstall = installControllerKeyboard();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" data-testid="more">More places</button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onSelect={() => {}}>Settings</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>,
    );
    screen.getByTestId('more').focus();
    activateFocused();
    expect(await screen.findByRole('menu')).not.toBeNull();
  });
});

describe('segmented controls (SegRow radiogroup)', () => {
  it('keyboard moves between segments, leaves at the ends and Up/Down', async () => {
    const { SegRow } = await import('../ui');
    uninstall = installControllerKeyboard();
    const onChange = vi.fn();
    render(
      <div>
        <button type="button" data-testid="above">above</button>
        <div style={{ display: 'flex' }}>
          <SegRow
            label="Scheme"
            value="dark"
            onChange={onChange}
            options={[
              { value: 'dark', label: 'Dark' },
              { value: 'light', label: 'Light' },
              { value: 'black', label: 'Black' },
            ]}
          />
        </div>
        <button type="button" data-testid="outside">outside</button>
      </div>,
    );
    const segs = screen.getAllByRole('radio');
    expect(segs).toHaveLength(3);
    layOut({ above: [100, 0] });
    segs.forEach((s, i) => rect(s, 100 + i * 100, 100));
    rect(screen.getByTestId('outside'), 0, 100);
    // Entering from above lands on the selected segment.
    screen.getByTestId('above').focus();
    fireEvent.keyDown(screen.getByTestId('above'), { key: 'ArrowDown' });
    expect(document.activeElement).toBe(segs[0]);
    // Along the axis the roving group owns the key: middle moves to first
    // (Radix focuses async, like real roving widgets).
    segs[1].focus();
    fireEvent.keyDown(segs[1], { key: 'ArrowLeft' });
    await waitFor(() => expect(document.activeElement).toBe(segs[0]));
    // At the first item Left leaves instead of wrapping or sticking.
    fireEvent.keyDown(segs[0], { key: 'ArrowLeft' });
    expect(document.activeElement).toBe(screen.getByTestId('outside'));
    expect(onChange).not.toHaveBeenCalled();
    // Across the axis leaves upward.
    segs[1].focus();
    fireEvent.keyDown(segs[1], { key: 'ArrowUp' });
    expect(document.activeElement).toBe(screen.getByTestId('above'));
  });
});
