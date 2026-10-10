import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

/** The address, as written, when it is a web link the browser may open ('' otherwise). */
export function webURL(raw: string | undefined): string {
  const s = (raw ?? '').trim();
  try {
    const u = new URL(s);
    return u.protocol === 'https:' || u.protocol === 'http:' ? s : '';
  } catch {
    return '';
  }
}

/** Opens a web link in the user's browser. The Wails webview ignores
 *  target="_blank" anchors, so every external link goes through here. Returns
 *  false, and opens nothing, for anything that is not http(s). */
export function openExternal(raw: string | undefined): boolean {
  const url = webURL(raw);
  if (!url) return false;
  BrowserOpenURL(url);
  return true;
}
