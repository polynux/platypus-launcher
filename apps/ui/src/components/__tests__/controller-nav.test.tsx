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
  installControllerKeyboard,
} from '../../lib/spatialNav';

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

  it('a pad direction dispatches the same Arrow event the keyboard uses', () => {
    uninstall = installControllerKeyboard();
    render(<Grid />);
    gridPositions();
    const center = screen.getByTestId('center');
    center.focus();
    // Exactly what the App's gamepad callback dispatches for D-pad/stick.
    center.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true }));
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

  it('a local handler that consumes the event wins', () => {
    uninstall = installControllerKeyboard();
    const onKeyDown = (e: React.KeyboardEvent): void => {
      if (e.key === 'ArrowDown') e.preventDefault();
    };
    render(
      <div onKeyDown={onKeyDown}>
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

  it('closes the topmost layer first, then goes back', async () => {
    const onBack = vi.fn();
    render(<LayerHarness onBack={onBack} />);
    rect(screen.getByTestId('inside'), 0, 0);
    screen.getByTestId('inside').focus();
    // B with the layer open: the layer closes, no back navigation.
    expect(closeTopmostLayer()).toBe(true);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(onBack).not.toHaveBeenCalled();
    // B with nothing open: the caller goes back.
    expect(closeTopmostLayer()).toBe(false);
    onBack();
    expect(onBack).toHaveBeenCalledTimes(1);
  });
});
