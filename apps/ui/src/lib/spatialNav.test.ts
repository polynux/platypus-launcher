// Geometry and scope of the spatial navigation module (issue #53). jsdom
// lays out at 0x0, so every button here gets a stubbed rect; an unstubbed
// button is hidden on purpose (the module skips 0x0 rects).
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  activateFocused,
  closeTopmostLayer,
  controllerKeyHandler,
  getFocusableElements,
  getScopeRoot,
  moveFocus,
  nearestInDirection,
  nextSection,
  sectionOf,
  shouldIgnoreArrows,
} from './spatialNav';

function rect(el: HTMLElement, x: number, y: number, w = 40, h = 24): void {
  el.getBoundingClientRect = () =>
    ({ left: x, top: y, width: w, height: h, right: x + w, bottom: y + h, x, y, toJSON: () => ({}) }) as DOMRect;
}

function button(label: string, x: number, y: number): HTMLButtonElement {
  const b = document.createElement('button');
  b.type = 'button';
  b.textContent = label;
  document.body.append(b);
  rect(b, x, y);
  return b;
}

afterEach(() => {
  document.body.innerHTML = '';
  vi.restoreAllMocks();
});

describe('nearestInDirection', () => {
  it('moves right to the nearest element in that direction', () => {
    const from = button('from', 0, 0);
    const near = button('near', 100, 0);
    button('far', 300, 0);
    button('left', -100, 0);
    expect(nearestInDirection(from, getFocusableElements(document), 'right')).toBe(near);
  });

  it('moves left, up and down', () => {
    const from = button('from', 100, 100);
    const left = button('left', 0, 100);
    const right = button('right', 200, 100);
    const up = button('up', 100, 0);
    const down = button('down', 100, 200);
    const els = getFocusableElements(document);
    expect(nearestInDirection(from, els, 'left')).toBe(left);
    expect(nearestInDirection(from, els, 'right')).toBe(right);
    expect(nearestInDirection(from, els, 'up')).toBe(up);
    expect(nearestInDirection(from, els, 'down')).toBe(down);
  });

  it('prefers the primary axis over diagonals', () => {
    const from = button('from', 0, 0);
    const straight = button('straight', 200, 0);
    button('diagonal', 120, 90);
    expect(nearestInDirection(from, getFocusableElements(document), 'right')).toBe(straight);
  });

  it('returns null with nothing in that direction', () => {
    const from = button('from', 0, 0);
    button('left', -100, 0);
    expect(nearestInDirection(from, getFocusableElements(document), 'right')).toBeNull();
  });
});

describe('scope', () => {
  it('scopes to the topmost dialog while one is open', () => {
    const outside = button('outside', 0, 0);
    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    const inside = document.createElement('button');
    inside.type = 'button';
    inside.textContent = 'inside';
    dialog.append(inside);
    document.body.append(dialog);
    rect(inside, 100, 0);
    expect(getScopeRoot()).toBe(dialog);
    outside.focus();
    expect(moveFocus('right')).toBe(true);
    expect(document.activeElement).toBe(inside);
    dialog.remove();
  });

  it('scopes to the friends panel when no dialog is open', () => {
    const panel = document.createElement('aside');
    panel.setAttribute('data-controller-layer', 'friends');
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.textContent = 'friend';
    panel.append(btn);
    document.body.append(panel);
    rect(btn, 50, 0);
    expect(getScopeRoot()).toBe(panel);
  });

  it('falls back to the document with no layer open', () => {
    button('a', 0, 0);
    expect(getScopeRoot()).toBe(document);
  });
});

describe('guards', () => {
  it('ignores arrows from inputs, selects, sliders and menus', () => {
    const input = document.createElement('input');
    const select = document.createElement('select');
    const slider = document.createElement('div');
    slider.setAttribute('role', 'slider');
    slider.tabIndex = 0;
    const menu = document.createElement('div');
    menu.setAttribute('role', 'menu');
    const listbox = document.createElement('ul');
    listbox.setAttribute('role', 'listbox');
    const combo = document.createElement('div');
    combo.setAttribute('role', 'combobox');
    document.body.append(input, select, slider, menu, listbox, combo);
    for (const el of [input, select, slider, menu, listbox, combo]) {
      expect(shouldIgnoreArrows(el)).toBe(true);
    }
    const btn = button('b', 0, 0);
    expect(shouldIgnoreArrows(btn)).toBe(false);
  });

  it('leaves events local handlers already consumed', () => {
    const a = button('a', 0, 0);
    const b = button('b', 100, 0);
    a.focus();
    const e = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true });
    e.preventDefault();
    expect(controllerKeyHandler(e)).toBe(false);
    expect(document.activeElement).toBe(a);
    expect(b).toBeDefined();
  });

  it('ignores modified arrows', () => {
    const a = button('a', 0, 0);
    button('b', 100, 0);
    a.focus();
    const e = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true, ctrlKey: true });
    expect(controllerKeyHandler(e)).toBe(false);
    expect(document.activeElement).toBe(a);
  });
});

describe('moveFocus', () => {
  it('focuses the first element when nothing is focused', () => {
    const a = button('a', 0, 0);
    button('b', 100, 0);
    (document.activeElement as HTMLElement | null)?.blur?.();
    expect(moveFocus('right')).toBe(true);
    expect(document.activeElement).toBe(a);
  });

  it('marks the target and scrolls it into view', () => {
    const a = button('a', 0, 0);
    const b = button('b', 100, 0);
    const spy = vi.spyOn(b, 'scrollIntoView');
    a.focus();
    expect(moveFocus('right')).toBe(true);
    expect(document.activeElement).toBe(b);
    expect(b.getAttribute('data-controller-focus')).toBe('true');
    expect(a.getAttribute('data-controller-focus')).toBeNull();
    expect(spy).toHaveBeenCalled();
  });
});

describe('activateFocused', () => {
  it('clicks the focused element', () => {
    const onClick = vi.fn();
    const b = button('b', 0, 0);
    b.addEventListener('click', onClick);
    b.focus();
    expect(activateFocused()).toBe(true);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does nothing on the bare body', () => {
    document.body.focus?.();
    expect(activateFocused()).toBe(false);
  });
});

describe('closeTopmostLayer', () => {
  it('dispatches Escape to the layer and reports it', () => {
    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    document.body.append(dialog);
    const seen: string[] = [];
    // Layers listen globally (the flyout, the friends panel and Radix all
    // close from window/document key handlers), not on their own node.
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') seen.push(e.key);
    };
    window.addEventListener('keydown', onKey);
    try {
      expect(closeTopmostLayer()).toBe(true);
      expect(seen).toEqual(['Escape']);
    } finally {
      window.removeEventListener('keydown', onKey);
      dialog.remove();
    }
  });

  it('reports false with no layer open', () => {
    button('a', 0, 0);
    expect(closeTopmostLayer()).toBe(false);
  });
});

describe('sections', () => {
  it('a game page counts as where it was opened from', () => {
    expect(sectionOf('game', 'library')).toBe('library');
    expect(sectionOf('game', 'fixes')).toBe('fixes');
    expect(sectionOf('game', undefined)).toBe('dashboard');
    expect(sectionOf('store')).toBe('plugins');
    expect(sectionOf('library')).toBe('library');
  });

  it('steps through the main sections and wraps', () => {
    expect(nextSection('dashboard', 1)).toBe('library');
    expect(nextSection('library', -1)).toBe('dashboard');
    expect(nextSection('storage', 1)).toBe('dashboard');
    expect(nextSection('dashboard', -1)).toBe('storage');
  });
});
