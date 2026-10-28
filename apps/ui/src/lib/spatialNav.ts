// Spatial navigation for controller and keyboard use (issue #53). Arrow keys
// (and the gamepad D-pad / left stick through the same code path, see
// useGamepad.ts) move focus to the nearest focusable element in that
// direction, using DOM geometry. The scope is the topmost open layer: when a
// dialog, flyout, the command palette or the friends panel is open, focus
// stays inside it.
export type NavDir = 'up' | 'down' | 'left' | 'right';

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

/** Text fields, selects, sliders and the places that already handle arrows
 *  themselves keep them: the palette listbox, Radix menus and poppers, and
 *  combobox triggers (the closed select opens on ArrowDown). */
export function shouldIgnoreArrows(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false;
  if (
    target.closest(
      'input, textarea, select, [contenteditable="true"], [contenteditable=""]',
    )
  ) return true;
  if (target.closest('[role="slider"]')) return true;
  if (
    target.closest(
      '[role="listbox"], [role="menu"], [role="combobox"], [data-radix-popper-content-wrapper]',
    )
  ) return true;
  return false;
}

/** The layer arrows are scoped to: the last open dialog (Modals, the command
 *  palette, the Tune flyout, all render role=dialog) wins over the friends
 *  side panel; with nothing open the whole document is the scope. */
export function getScopeRoot(): Element | Document {
  const dialogs = document.querySelectorAll('[role="dialog"]');
  if (dialogs.length > 0) return dialogs[dialogs.length - 1];
  const layer = document.querySelector('[data-controller-layer]');
  if (layer) return layer;
  return document;
}

function isHidden(el: HTMLElement): boolean {
  if (el.closest('[hidden]')) return true;
  const labelled = el.closest('[aria-hidden="true"]');
  if (labelled && labelled !== el) return true;
  return el.getAttribute('aria-hidden') === 'true';
}

/** Focusable elements in scope, in DOM order. Visibility is read off the
 *  rect: hidden elements lay out at 0x0 (tests stub the rects they need). */
export function getFocusableElements(scope: Element | Document): HTMLElement[] {
  const root: ParentNode = scope instanceof Document ? scope.documentElement ?? scope : scope;
  const out: HTMLElement[] = [];
  for (const el of root.querySelectorAll(FOCUSABLE_SELECTOR)) {
    if (!(el instanceof HTMLElement)) continue;
    if (el.tabIndex < 0) continue;
    if (isHidden(el)) continue;
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

interface Point { x: number; y: number; }

function center(r: DOMRect): Point {
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
}

/** The nearest candidate strictly in `dir` of `from`, weighting the primary
 *  axis over the orthogonal one so rows and columns win over diagonals. */
export function nearestInDirection(
  from: HTMLElement,
  els: HTMLElement[],
  dir: NavDir,
): HTMLElement | null {
  const fc = center(from.getBoundingClientRect());
  let best: HTMLElement | null = null;
  let bestScore = Infinity;
  for (const el of els) {
    if (el === from) continue;
    const c = center(el.getBoundingClientRect());
    const dx = c.x - fc.x;
    const dy = c.y - fc.y;
    let primary = 0;
    let secondary = 0;
    if (dir === 'right') {
      if (dx <= 0) continue;
      primary = dx;
      secondary = Math.abs(dy);
    } else if (dir === 'left') {
      if (dx >= 0) continue;
      primary = -dx;
      secondary = Math.abs(dy);
    } else if (dir === 'down') {
      if (dy <= 0) continue;
      primary = dy;
      secondary = Math.abs(dx);
    } else {
      if (dy >= 0) continue;
      primary = -dy;
      secondary = Math.abs(dx);
    }
    const score = primary + secondary * 2.2;
    if (score < bestScore) {
      bestScore = score;
      best = el;
    }
  }
  return best;
}

/** Mark the controller's target: programmatic focus() does not match
 *  :focus-visible, so the module marks its target and the stylesheet paints
 *  the same token ring. Cleared on pointer input. */
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

/** Move focus one step in `dir` inside the topmost layer. With focus outside
 *  the layer (or nowhere), the first element of the layer takes focus so a
 *  layer never leaks focus behind itself. */
export function moveFocus(dir: NavDir): boolean {
  const scope = getScopeRoot();
  const els = getFocusableElements(scope);
  if (els.length === 0) return false;
  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const inScope = active !== null && (scope instanceof Document ? true : scope.contains(active));
  if (!active || !inScope) {
    focusController(els[0]);
    return true;
  }
  const next = nearestInDirection(active, els, dir);
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
 *  (the dialog, flyout, palette and friends panel all close on Escape; the
 *  maintenance confirm row stops the event so the settings stay open).
 *  Returns whether a layer was present; the caller goes back otherwise. */
export function closeTopmostLayer(): boolean {
  if (getScopeRoot() instanceof Document) return false;
  const target =
    document.activeElement instanceof HTMLElement ? document.activeElement : document.body;
  target.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
  return true;
}

/** One arrow-key handler for keyboard and pad (the pad dispatches real
 *  ArrowLeft/Right/Up/Down events; see useGamepad). Local handlers run first
 *  in the bubble path: a preventDefaulted event, an editable target or a
 *  menu/listbox target is left alone. Returns whether focus moved. */
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
  if (shouldIgnoreArrows(e.target)) return false;
  e.preventDefault();
  return moveFocus(dir);
}

/** Install the shared arrow handler (and clear the controller ring on mouse
 *  input). The App owns the one installation; tests use the same one. */
export function installControllerKeyboard(): () => void {
  window.addEventListener('keydown', controllerKeyHandler);
  window.addEventListener('pointerdown', clearControllerFocus);
  return () => {
    window.removeEventListener('keydown', controllerKeyHandler);
    window.removeEventListener('pointerdown', clearControllerFocus);
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
