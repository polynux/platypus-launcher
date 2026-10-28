// Geometry and scope of the spatial navigation module (issue #53). jsdom
// lays out at 0x0, so every button here gets a stubbed rect; an unstubbed
// button is hidden on purpose (the module skips 0x0 rects).
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  activateFocused,
  closeTopmostLayer,
  controllerKeyHandler,
  focusFirstInView,
  getFocusableElements,
  getScopeRoot,
  handleBackAction,
  installControllerKeyboard,
  moveFocus,
  nearestInDirection,
  nextSection,
  sectionOf,
  shouldIgnoreArrows,
} from './spatialNav';
import { dispatchPadArrow } from './useGamepad';

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
  it('ignores arrows where the element owns them', () => {
    // Single-line inputs and closed triggers are escapable (finding 1): the
    // handler decides per key, so the guard does not claim them.
    const input = document.createElement('input');
    const combo = document.createElement('div');
    combo.setAttribute('role', 'combobox');
    document.body.append(input, combo);
    expect(shouldIgnoreArrows(input)).toBe(false);
    expect(shouldIgnoreArrows(combo)).toBe(false);
    input.remove();
    combo.remove();
    const select = document.createElement('select');
    const slider = document.createElement('div');
    slider.setAttribute('role', 'slider');
    slider.tabIndex = 0;
    const menu = document.createElement('div');
    menu.setAttribute('role', 'menu');
    const listbox = document.createElement('ul');
    listbox.setAttribute('role', 'listbox');
    document.body.append(select, slider, menu, listbox);
    for (const el of [select, slider, menu, listbox]) {
      expect(shouldIgnoreArrows(el)).toBe(true);
    }
    // With a popup open even a trigger yields to it.
    document.body.append(combo);
    expect(shouldIgnoreArrows(combo)).toBe(true);
    const btn = button('b', 0, 0);
    expect(shouldIgnoreArrows(btn)).toBe(false);
  });

  it('yields to a local key zone even when it also handles the key', () => {
    const a = button('a', 0, 0);
    const b = button('b', 100, 0);
    // A local zone (the palette, the library list) owns every key: the
    // shared capture handler yields, and the zone consumes the press.
    const zone = document.createElement('div');
    zone.setAttribute('data-keys-local', '');
    zone.append(a, b);
    document.body.append(zone);
    (zone as HTMLElement).addEventListener('keydown', (ev) => {
      if ((ev as KeyboardEvent).key === 'ArrowRight') ev.preventDefault();
    });
    a.focus();
    const e = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true });
    a.dispatchEvent(e);
    expect(controllerKeyHandler(e)).toBe(false);
    expect(document.activeElement).toBe(a);
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

describe('direction gating (finding 2)', () => {
  it('Right from a row goes to the same-row control, not above', () => {
    // Settings > Appearance: Home sits in the top bar above, the Theme
    // trigger on the same row. Distance-only scoring picked Home.
    const home = button('home', 0, 0);
    const appearance = button('appearance', 0, 100);
    const theme = document.createElement('button');
    theme.type = 'button';
    theme.textContent = 'theme';
    theme.setAttribute('role', 'combobox');
    document.body.append(theme);
    rect(theme, 200, 100);
    const els = getFocusableElements(document);
    expect(nearestInDirection(appearance, els, 'right')).toBe(theme);
    expect(home).toBeDefined();
  });

  it('Right skips a nearer-by-center control on another row', () => {
    // "Add store" with Collections on its row and search above-right and
    // nearer by center: distance-only scoring jumped to search.
    const add = button('add', 0, 200);
    const collections = button('collections', 120, 200);
    button('search', 30, 160);
    const els = getFocusableElements(document);
    expect(nearestInDirection(add, els, 'right')).toBe(collections);
  });

  it('Right prefers the same row over a nearer off-row control', () => {
    const lib = button('library', 0, 0);
    const row = button('row', 400, 0);
    button('down', 60, 120);
    const els = getFocusableElements(document);
    expect(nearestInDirection(lib, els, 'right')).toBe(row);
  });

  it('Down from a row reaches the wide control below, not the neighbour', () => {
    const a = button('a', 0, 0);
    button('b', 200, 0);
    const wide = document.createElement('textarea');
    document.body.append(wide);
    rect(wide, 0, 100, 400, 80);
    const els = getFocusableElements(document);
    expect(nearestInDirection(a, els, 'down')).toBe(wide);
  });

  it('Up and Down skip wide offsets: a far-below row is not "right of"', () => {
    const lib = button('library', 0, 0);
    button('far', -425, 500);
    const below = button('below', 0, 200);
    const els = getFocusableElements(document);
    expect(nearestInDirection(lib, els, 'right')).toBeNull();
    expect(nearestInDirection(lib, els, 'down')).toBe(below);
  });

  it('treats a card as one stop: hover-only buttons are skipped', () => {
    const prev = button('prev', -120, 0);
    const card = button('card', 0, 0);
    rect(card, 0, 0, 100, 100);
    const quick = document.createElement('button');
    quick.type = 'button';
    quick.textContent = 'quick';
    quick.style.opacity = '0';
    document.body.append(quick);
    rect(quick, 10, 10, 30, 30);
    const els = getFocusableElements(document);
    expect(els).not.toContain(quick);
    expect(nearestInDirection(prev, els, 'right')).toBe(card);
  });
});

describe('fields and triggers (finding 1)', () => {
  it('keyboard Up leaves a single-line input, Left/Right keep the caret', () => {
    const uninstall = installControllerKeyboard();
    try {
      const above = button('above', 0, 0);
      const input = document.createElement('input');
      input.type = 'search';
      document.body.append(input);
      rect(input, 0, 100);
      input.focus();
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(above);
      const again = document.createElement('input');
      document.body.append(again);
      rect(again, 0, 200);
      again.focus();
      const e = new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true });
      again.dispatchEvent(e);
      expect(e.defaultPrevented).toBe(false);
      expect(document.activeElement).toBe(again);
    } finally {
      uninstall();
    }
  });

  it('pad directions leave a text input and a textarea', () => {
    const uninstall = installControllerKeyboard();
    try {
      const left = button('left', 0, 100);
      const input = document.createElement('input');
      input.type = 'text';
      document.body.append(input);
      rect(input, 100, 100);
      input.focus();
      dispatchPadArrow('left');
      expect(document.activeElement).toBe(left);
      const area = document.createElement('textarea');
      document.body.append(area);
      rect(area, 100, 200);
      const below = button('below', 100, 400);
      area.focus();
      dispatchPadArrow('down');
      expect(document.activeElement).toBe(below);
    } finally {
      uninstall();
    }
  });

  it('all arrows move off a closed Select trigger without opening it', () => {
    const uninstall = installControllerKeyboard();
    try {
      const trigger = document.createElement('button');
      trigger.type = 'button';
      trigger.textContent = 'recent';
      trigger.setAttribute('role', 'combobox');
      trigger.setAttribute('aria-haspopup', 'listbox');
      document.body.append(trigger);
      rect(trigger, 0, 0);
      const next = button('next', 100, 0);
      const seen: string[] = [];
      // A bubble listener (where Radix would open the select) never fires:
      // the capture handler stops the descent.
      document.body.addEventListener('keydown', (ev) => seen.push((ev as KeyboardEvent).key));
      trigger.focus();
      trigger.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true }));
      expect(document.activeElement).toBe(next);
      expect(seen).toEqual([]);
    } finally {
      uninstall();
    }
  });
});

describe('lost focus (finding 3)', () => {
  it('starts from main content with focus on body', () => {
    const main = document.createElement('main');
    const first = document.createElement('button');
    first.type = 'button';
    first.textContent = 'first';
    main.append(first);
    document.body.append(main);
    button('outside', 0, 0);
    rect(first, 0, 200);
    (document.activeElement as HTMLElement | null)?.blur?.();
    expect(document.activeElement === document.body || document.activeElement === null).toBe(true);
    expect(moveFocus('down')).toBe(true);
    expect(document.activeElement).toBe(first);
  });

  it('treats a removed focused element as lost', () => {
    const a = button('a', 0, 0);
    const b = button('b', 100, 0);
    a.focus();
    a.remove();
    expect(moveFocus('right')).toBe(true);
    expect(document.activeElement).toBe(b);
  });
});

describe('menus as the topmost layer (finding 5)', () => {
  it('B targets an open menu and does not go back', () => {
    const popper = document.createElement('div');
    popper.setAttribute('data-radix-popper-content-wrapper', '');
    document.body.append(popper);
    const item = document.createElement('button');
    item.type = 'button';
    item.textContent = 'item';
    popper.append(item);
    rect(item, 0, 0);
    expect(getScopeRoot()).toBe(popper);
    const seen: string[] = [];
    // Radix closes from a document-level Escape listener, like the real one.
    const onKey = (e: KeyboardEvent): void => {
      if (e.key === 'Escape') seen.push('Escape');
    };
    window.addEventListener('keydown', onKey);
    try {
      item.focus();
      expect(closeTopmostLayer()).toBe(true);
      expect(seen).toEqual(['Escape']);
    } finally {
      window.removeEventListener('keydown', onKey);
      popper.remove();
    }
  });

  it('handleBackAction goes back only with no layer open', () => {
    button('a', 0, 0);
    const goBack = vi.fn();
    handleBackAction(goBack);
    expect(goBack).toHaveBeenCalledTimes(1);
    const dialog = document.createElement('div');
    dialog.setAttribute('role', 'dialog');
    document.body.append(dialog);
    const seen: string[] = [];
    window.addEventListener('keydown', (ev) => {
      if ((ev as KeyboardEvent).key === 'Escape') seen.push('Escape');
    });
    handleBackAction(goBack);
    expect(goBack).toHaveBeenCalledTimes(1);
    expect(seen).toEqual(['Escape']);
    dialog.remove();
  });
});

describe('scroll instead of sticking (finding 6)', () => {
  it('does not preventDefault when nothing lies in that direction', () => {
    const last = button('last', 0, 0);
    last.focus();
    const e = new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true });
    expect(controllerKeyHandler(e)).toBe(false);
    expect(e.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(last);
  });

  it('the pad scrolls the scroll root by hand', () => {
    const uninstall = installControllerKeyboard();
    try {
      const scroller = document.createElement('div');
      Object.defineProperty(scroller, 'scrollHeight', { value: 1000, configurable: true });
      Object.defineProperty(scroller, 'clientHeight', { value: 100, configurable: true });
      const scrollBy = vi.fn();
      scroller.scrollBy = scrollBy as unknown as typeof scroller.scrollBy;
      const last = document.createElement('button');
      last.type = 'button';
      last.textContent = 'last';
      scroller.append(last);
      document.body.append(scroller);
      rect(last, 0, 0);
      last.focus();
      dispatchPadArrow('down');
      expect(scrollBy).toHaveBeenCalledWith({ top: 48 });
    } finally {
      uninstall();
    }
  });
});

describe('stale ring (finding 7)', () => {
  it('clears the ring when another element takes focus', () => {
    const uninstall = installControllerKeyboard();
    try {
      const a = button('a', 0, 0);
      const b = button('b', 100, 0);
      a.setAttribute('data-controller-focus', 'true');
      a.focus();
      expect(a.getAttribute('data-controller-focus')).toBe('true');
      b.focus();
      expect(a.getAttribute('data-controller-focus')).toBeNull();
    } finally {
      uninstall();
    }
  });
});
