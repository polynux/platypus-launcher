// Spatial navigation for controller and keyboard use (issue #53). Arrow keys
// (and the gamepad D-pad / left stick through the same code path, see
// useGamepad.ts) move focus to the nearest focusable element in that
// direction, using DOM geometry. The scope is the topmost open layer: an open
// dialog (Modals, the command palette, the Tune flyout) wins over the friends
// side panel, which wins over an open Radix menu/popper; with nothing open
// the whole document is the scope.
export type NavDir = 'up' | 'down' | 'left' | 'right';

/** Set by the gamepad path (see dispatchPadArrow): a pad has no caret, so a
 *  pad direction leaves text fields and closed triggers that keyboard arrows
 *  must keep working in. */
export const PAD_FLAG = 'controllerPadMove';

/** What can take focus. Kept to natively focusable elements so a move always
 *  lands somewhere a mouse or Tab user could reach too. */
export const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not(:disabled)',
  'input:not(:disabled)',
  'select:not(:disabled)',
  'textarea:not(:disabled)',
  'audio[controls]',
  'video[controls]',
  'summary',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

/** Subtrees whose own key handling owns every key (the palette, the library
 *  list): the shared handler yields without consuming. */
const LOCAL_KEYS = '[data-keys-local]';
/** Open popups the shared handler never steers inside (Radix owns their
 *  keys; B closes them through Escape). */
const POPUP = '[role="listbox"], [role="menu"], [data-radix-popper-content-wrapper]';
/** Closed select/menu triggers: arrows move focus away, Enter/A opens. */
const TRIGGER = '[role="combobox"], [aria-haspopup="menu"], [aria-haspopup="listbox"]';
/** Text editing targets. */
const TEXT = 'input, textarea, select, [contenteditable="true"], [contenteditable=""]';
/** Single-line text entry: Up/Down are free, Left/Right keep the caret. */
const TEXT_INPUT_TYPES = new Set(['', 'text', 'search', 'url', 'email', 'password', 'tel']);

function menuOpen(): boolean {
  return document.querySelector(POPUP) !== null;
}

function isPadEvent(e: KeyboardEvent): boolean {
  return (e as KeyboardEvent & { [PAD_FLAG]?: boolean })[PAD_FLAG] === true;
}

/** Whether this arrow must keep the element's own behaviour. */
export function shouldIgnoreArrows(target: EventTarget | null, fromPad = false): boolean {
  if (!(target instanceof Element)) return false;
  if (target.closest(LOCAL_KEYS)) return true;
  if (target.closest(POPUP)) return true;
  const trigger = target.closest(TRIGGER);
  // Closed triggers never consume arrows (Enter/A opens them); with a menu
  // open the popup owns the keys instead.
  if (trigger) return menuOpen();
  const text = target.closest(TEXT);
  if (text) {
    if (text.tagName === 'SELECT') return true;
    if (text.getAttribute('role') === 'slider') return true;
    if (text.tagName === 'INPUT') {
      const type = (text.getAttribute('type') ?? '').toLowerCase();
      if (!TEXT_INPUT_TYPES.has(type)) return true;
      // Single-line entry: the caret owns Left/Right for the keyboard; a
      // pad has no caret, so every direction may leave.
      return false;
    }
    // Textareas and contenteditables: the keyboard keeps caret keys, the pad
    // always leaves (it cannot address a line or column).
    return !fromPad;
  }
  if (target.closest('[role="slider"]')) return true;
  return false;
}

/** Left/Right keep the caret in single-line inputs; every other unconsumed
 *  arrow moves focus. The pad path (no caret) moves on all four. */
function caretKept(target: Element, dir: NavDir, fromPad: boolean): boolean {
  if (fromPad) return false;
  const text = target.closest(TEXT);
  if (!text || text.tagName !== 'INPUT') return false;
  const type = (text.getAttribute('type') ?? '').toLowerCase();
  if (!TEXT_INPUT_TYPES.has(type)) return false;
  return dir === 'left' || dir === 'right';
}

/** The layer arrows are scoped to: the last open dialog wins over the
 *  friends side panel, which wins over an open Radix menu/popper; with
 *  nothing open the whole document is the scope. */
export function getScopeRoot(): Element | Document {
  const dialogs = document.querySelectorAll('[role="dialog"]');
  if (dialogs.length > 0) return dialogs[dialogs.length - 1];
  const layer = document.querySelector('[data-controller-layer]');
  if (layer) return layer;
  const popup =
    document.querySelector('[data-radix-popper-content-wrapper]') ??
    [...document.querySelectorAll('[role="menu"], [role="listbox"]')].pop() ??
    null;
  if (popup) return popup;
  return document;
}

function isHidden(el: HTMLElement): boolean {
  if (el.closest('[hidden]')) return true;
  const labelled = el.closest('[aria-hidden="true"]');
  if (labelled && labelled !== el) return true;
  return el.getAttribute('aria-hidden') === 'true';
}

/** Hover-only controls (the cards' quick actions) are not stops: keyboard
 *  and mouse users reach them through the card, and the pad treats a card as
 *  one stop. Focused controls are never skipped. */
function isHoverOnly(el: HTMLElement): boolean {
  if (el.contains(document.activeElement) || el === document.activeElement) return false;
  try {
    return getComputedStyle(el).opacity === '0';
  } catch {
    return false;
  }
}

function isEffectivelyHidden(el: HTMLElement): boolean {
  try {
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden') return true;
  } catch {
    // jsdom: fall through to the rect check below.
  }
  return false;
}

/** Focusable elements in scope, in DOM order. Visibility is read off the
 *  rect: hidden elements lay out at 0x0 (tests stub the rects they need). */
export function getFocusableElements(scope: Element | Document): HTMLElement[] {
  const root: ParentNode = scope instanceof Document ? scope.documentElement ?? scope : scope;
  const out: HTMLElement[] = [];
  for (const el of root.querySelectorAll(FOCUSABLE_SELECTOR)) {
    if (!(el instanceof HTMLElement)) continue;
    if (el.tabIndex < 0) continue;
    if (el.hasAttribute('disabled')) continue;
    if (isHidden(el)) continue;
    if (isEffectivelyHidden(el)) continue;
    if (isHoverOnly(el)) continue;
    const r = el.getBoundingClientRect();
    if (r.width <= 0 && r.height <= 0) continue;
    out.push(el);
  }
  if (scope instanceof HTMLElement && scope.matches(FOCUSABLE_SELECTOR)) {
    if (scope.tabIndex >= 0 && !isHidden(scope)) {
      const r = scope.getBoundingClientRect();
      if ((r.width > 0 || r.height > 0) && !out.includes(scope)) out.unshift(scope);
    }
  }
  return out;
}

/** A candidate must lie in the pressed direction: its near edge past the
 *  current element's far edge, with a small tolerance for shared borders. */
const EDGE_TOL = 8;

function intervalGap(a1: number, a2: number, b1: number, b2: number): number {
  if (b1 <= a2 && a1 <= b2) return 0;
  return Math.min(Math.abs(b1 - a2), Math.abs(a1 - b2));
}

/** The nearest candidate strictly in `dir` of `from`: edge-gated, preferring
 *  the ones overlapping the current row/column, then the nearest gap. */
export function nearestInDirection(
  from: HTMLElement,
  els: HTMLElement[],
  dir: NavDir,
): HTMLElement | null {
  const f = from.getBoundingClientRect();
  const fl = f.left;
  const fr = f.left + f.width;
  const ft = f.top;
  const fb = f.top + f.height;
  let best: HTMLElement | null = null;
  let bestScore = Infinity;
  let bestRow = false;
  for (const el of els) {
    if (el === from) continue;
    const r = el.getBoundingClientRect();
    const l = r.left;
    const rr = r.left + r.width;
    const t = r.top;
    const b = r.top + r.height;
    let gap = 0;
    let orth = 0;
    if (dir === 'right') {
      if (l < fr - EDGE_TOL) continue;
      gap = l - fr;
      orth = intervalGap(ft, fb, t, b);
    } else if (dir === 'left') {
      if (rr > fl + EDGE_TOL) continue;
      gap = fl - rr;
      orth = intervalGap(ft, fb, t, b);
    } else if (dir === 'down') {
      if (t < fb - EDGE_TOL) continue;
      gap = t - fb;
      orth = intervalGap(fl, fr, l, rr);
    } else {
      if (b > ft + EDGE_TOL) continue;
      gap = ft - b;
      orth = intervalGap(fl, fr, l, rr);
    }
    // Row/column neighbours win over nearer off-axis controls: only when
    // nothing overlaps the current row/column do diagonal candidates count.
    const inRow = orth === 0;
    if (bestRow && !inRow) continue;
    const score = Math.max(0, gap) + orth * 3;
    if (!bestRow && inRow) {
      best = el;
      bestScore = score;
      bestRow = true;
      continue;
    }
    if (score < bestScore) {
      bestScore = score;
      best = el;
    }
  }
  return best;
}

/** Mark the controller's target: programmatic focus() does not match
 *  :focus-visible, so the module marks its target and the stylesheet paints
 *  the same token ring. Cleared on pointer input and on any other focus. */
export function focusController(el: HTMLElement): void {
  clearControllerFocus();
  el.setAttribute('data-controller-focus', 'true');
  el.focus({ preventScroll: true });
  try {
    el.scrollIntoView({ block: 'nearest' });
  } catch {
    // jsdom and hidden containers: focus still landed.
  }
}

export function clearControllerFocus(): void {
  for (const el of document.querySelectorAll('[data-controller-focus]')) {
    el.removeAttribute('data-controller-focus');
  }
}

/** The first sensible element of the current view: main content first, then
 *  the active nav item, then whatever is first. */
export function focusFirstInView(): boolean {
  const scope = getScopeRoot();
  const els = getFocusableElements(scope);
  if (els.length === 0) return false;
  if (!(scope instanceof Document)) {
    focusController(els[0]);
    return true;
  }
  const main = document.querySelector('main');
  if (main instanceof HTMLElement) {
    const inMain = els.filter((el) => main.contains(el));
    if (inMain.length > 0) {
      focusController(inMain[0]);
      return true;
    }
  }
  const current = document.querySelector('[aria-current="true"], [aria-current="page"]');
  if (current instanceof HTMLElement && els.includes(current)) {
    focusController(current);
    return true;
  }
  focusController(els[0]);
  return true;
}

/** Move focus one step in `dir` inside the topmost layer. With focus lost
 *  (body, a removed element, or outside the layer) the first sensible
 *  element of the view takes focus. */
export function moveFocus(dir: NavDir): boolean {
  const scope = getScopeRoot();
  const ae = document.activeElement;
  const lost =
    !(ae instanceof HTMLElement) ||
    ae === document.body ||
    ae === document.documentElement ||
    !document.contains(ae) ||
    (!(scope instanceof Document) && !scope.contains(ae));
  if (lost) return focusFirstInView();
  const els = getFocusableElements(scope);
  if (els.length === 0) return false;
  const next = nearestInDirection(ae, els, dir);
  if (!next) return false;
  focusController(next);
  return true;
}

/** A (gamepad) / Enter-equivalent: click the focused element (open a game,
 *  press Play). Synthetic key events never run default button activation,
 *  so this clicks directly. */
export function activateFocused(): boolean {
  const el = document.activeElement;
  if (!(el instanceof HTMLElement)) return false;
  if (el === document.body || el === document.documentElement) return false;
  el.click();
  return true;
}

/** B (gamepad): back out of the topmost layer through its own Escape path
 *  (a dialog, the friends panel, or an open menu/popper all close on
 *  Escape; the maintenance confirm row stops the event so the settings stay
 *  open). Returns whether a layer was present; the caller goes back
 *  otherwise. */
export function closeTopmostLayer(): boolean {
  if (getScopeRoot() instanceof Document) return false;
  const target =
    document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  target.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  return true;
}

/** B through the real path: close the topmost layer, else the fallback
 *  (the app's back navigation). The App passes its history-back here. */
export function handleBackAction(goBack: () => void): void {
  if (!closeTopmostLayer()) goBack();
}

/** Scroll one step when nothing lies in `dir`: keyboard arrows keep their
 *  native scroll, and the pad (whose synthetic events never scroll) moves
 *  the nearest scrollable ancestor by hand. */
function scrollOneStep(dir: NavDir): void {
  const step = 48;
  let el: Element | null =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;
  while (el && el !== document.documentElement) {
    if (el instanceof HTMLElement && (el.scrollHeight > el.clientHeight + 1 || el.scrollWidth > el.clientWidth + 1)) break;
    el = el.parentElement;
  }
  const target =
    el instanceof HTMLElement ? el : (document.scrollingElement as HTMLElement | null);
  if (!target || typeof target.scrollBy !== 'function') return;
  try {
    if (dir === 'up') target.scrollBy({ top: -step });
    else if (dir === 'down') target.scrollBy({ top: step });
    else if (dir === 'left') target.scrollBy({ left: -step });
    else target.scrollBy({ left: step });
  } catch {
    // jsdom: no layout, nothing to scroll.
  }
}

/** One arrow-key handler for keyboard and pad (the pad dispatches flagged
 *  ArrowLeft/Right/Up/Down events; see dispatchPadArrow in useGamepad.ts).
 *  Capture-first so a closed select/menu trigger never opens under the
 *  handler: arrows move away, Enter/A opens. Local handlers run in the
 *  bubble path for everything the handler yields to; a preventDefaulted
 *  event is left alone. Returns whether focus moved. */
export function controllerKeyHandler(e: KeyboardEvent): boolean {
  if (e.defaultPrevented) return false;
  const dir =
    e.key === 'ArrowUp' ? 'up'
    : e.key === 'ArrowDown' ? 'down'
    : e.key === 'ArrowLeft' ? 'left'
    : e.key === 'ArrowRight' ? 'right'
    : null;
  if (dir === null) return false;
  if (e.ctrlKey || e.metaKey || e.altKey) return false;
  const fromPad = isPadEvent(e);
  const target = e.target instanceof Element ? e.target : null;
  if (target && shouldIgnoreArrows(target, fromPad)) return false;
  if (target && caretKept(target, dir, fromPad)) return false;
  // Closed select/menu triggers: arrows move, never open (Enter/A opens).
  // Stop the descent so Radix never sees the key.
  if (target && target.closest(TRIGGER) && !menuOpen()) {
    e.preventDefault();
    e.stopPropagation();
    return moveFocus(dir);
  }
  const moved = moveFocus(dir);
  if (moved) {
    e.preventDefault();
    return true;
  }
  // Nothing in that direction: let the browser scroll (keyboard), or scroll
  // the scroll root by hand (the pad's synthetic events never scroll).
  if (fromPad) scrollOneStep(dir);
  return false;
}

/** Install the shared arrow handler (and clear the controller ring on mouse
 *  input or any other focus). The App owns the one installation; tests use
 *  the same one. */
export function installControllerKeyboard(): () => void {
  const onFocusIn = (e: FocusEvent): void => {
    const t = e.target instanceof Element ? e.target : null;
    if (!t || !t.hasAttribute('data-controller-focus')) clearControllerFocus();
  };
  window.addEventListener('keydown', controllerKeyHandler, true);
  window.addEventListener('pointerdown', clearControllerFocus);
  window.addEventListener('focusin', onFocusIn);
  return () => {
    window.removeEventListener('keydown', controllerKeyHandler, true);
    window.removeEventListener('pointerdown', clearControllerFocus);
    window.removeEventListener('focusin', onFocusIn);
  };
}

/** The main navigation sections LB/RB cycle (the sidebar in classic themes,
 *  the sourcebar in Atlas; both navigate the same views). The store view
 *  needs a plugin id, so it is not a section: from there RB/LB lands on
 *  Plugins. */
export const NAV_SECTIONS = [
  'dashboard',
  'library',
  'downloads',
  'activity',
  'fixes',
  'runners',
  'plugins',
  'storage',
] as const;

export type NavSection = (typeof NAV_SECTIONS)[number];

/** The section a view counts as (a game page counts as where it was opened
 *  from, like the shell's active item). */
export function sectionOf(viewName: string, from?: string): string {
  if (viewName === 'game') return from === 'fixes' || from === 'library' ? from : 'dashboard';
  if (viewName === 'store') return 'plugins';
  return viewName;
}

/** The section `delta` steps from `current` (unknown views wrap to the
 *  nearer end, like the list selection does). */
export function nextSection(current: string, delta: 1 | -1): NavSection {
  const i = (NAV_SECTIONS as readonly string[]).indexOf(current);
  const at = i < 0 ? (delta === 1 ? -1 : 0) : i;
  return NAV_SECTIONS[(at + delta + NAV_SECTIONS.length) % NAV_SECTIONS.length];
}
