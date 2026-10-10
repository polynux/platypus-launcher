import { browserOpen } from '../effect/bridge';

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

/** Opens a web link in the user's browser. Neither shell opens
 *  target="_blank" anchors in the user's browser, so every external link goes through here. Returns
 *  false, and opens nothing, for anything that is not http(s). */
export function openExternal(raw: string | undefined): boolean {
  const url = webURL(raw);
  if (!url) return false;
  browserOpen(url);
  return true;
}
