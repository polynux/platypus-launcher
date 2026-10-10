// Keyboard shortcuts the production window must not answer. The app has no
// menu (Menu.setApplicationMenu(null)), so menu accelerators are already
// gone; but Chromium still reloads on Ctrl+R/F5 and opens DevTools on
// F12/Ctrl+Shift+I. Development keeps them (Vite hot reload, F12 with
// PLATYPUS_DEVTOOLS); production blocks them in before-input-event.

export interface KeyInput {
  key: string;
  control: boolean;
  meta: boolean;
  shift: boolean;
}

/** Whether a key press reloads the page or reaches DevTools. */
export function isReloadOrInspect(input: KeyInput): boolean {
  const k = input.key.toLowerCase();
  if (input.key === 'F5' || input.key === 'F12') return true;
  if (!(input.control || input.meta)) return false;
  if (!input.shift && (k === 'r')) return true; // Ctrl+R
  if (input.shift && (k === 'r' || k === 'i' || k === 'j' || k === 'c')) return true; // Ctrl+Shift+R/I/J/C
  return false;
}
