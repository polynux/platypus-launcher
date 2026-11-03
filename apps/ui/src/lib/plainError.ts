/** Developer text that never belongs on screen by default: JSON-RPC codes and
 *  parameter checks, decoder and runtime errors, stack positions, and the
 *  core's engine strings ("size check: unknown game …"). */
const DEVELOPER = /invalid params|-3\d{4}|jsonrpc|\bjson:|unmarshal|unexpected end|\bEOF\b|\bnil\b|\bpanic\b|\bundefined\b|TypeError|ReferenceError|NetworkError|Failed to fetch|\bctx\b|\.go:\d+|size check: unknown game/i;

/** The sentence to show for an error message: the message itself when it reads
 *  as a sentence for a person (core's "X is not installed"), otherwise the
 *  fallback. The raw text stays available behind a Details disclosure. */
export function plainError(raw: string | null | undefined, fallback: string): string {
  const text = (raw ?? '').trim();
  if (!text) return fallback;
  return DEVELOPER.test(text) ? fallback : text;
}

/** Whether a raw message differs from what is shown, so Details has something to add. */
export function hasDetails(raw: string | null | undefined, fallback: string): boolean {
  const text = (raw ?? '').trim();
  return text !== '' && plainError(text, fallback) !== text;
}

/** The core's known failure kinds as app copy (audit F4): lowercase engine
 *  sentences become translated sentences, with the raw text kept behind
 *  Details/title by the caller. Plugin-written sentences pass through (null),
 *  so they stay readable. Not a wider `plainError`: that filter hid useful
 *  plugin messages before.
 *
 *  Returns the sentence with the match index, so a "Title: engine error"
 *  toast keeps its title (`prefix: sentence`, raw as the description). Paths
 *  may contain spaces, so they capture up to the next ": " or the end. Only
 *  lowercase engine wording maps: the catalog's own sentences (which already
 *  read well, e.g. English `move.noSpace`) start uppercase and pass through.
 */
export interface FriendlyMatch {
  text: string;
  /** Offset of the engine wording inside the trimmed input. */
  at: number;
}

export function friendlyCoreError(
  raw: string | null | undefined,
  t: (key: string, opts?: Record<string, unknown>) => string,
): FriendlyMatch | null {
  const text = (raw ?? '').trim();
  if (!text) return null;
  const lower = (s: string): boolean => /^[a-z]/.test(s);
  let m = text.match(/refusing to allow system folder (.+?)(?::\s|$)/i);
  if (m && m.index !== undefined && lower(m[0])) {
    return { text: t('downloads:atlas.failedSystemFolder', { path: m[1].replace(/[.,;:'"]+$/, '') }), at: m.index };
  }
  m = text.match(/not enough space in (.+?)(?::\s|$)/i);
  if (m && m.index !== undefined && lower(m[0])) {
    return { text: t('downloads:atlas.failedNoSpace', { dir: m[1].replace(/[.,;:'"]+$/, '') }), at: m.index };
  }
  m = text.match(/refusing tar (?:entry|archive|symlink)\b/i);
  if (m && m.index !== undefined && lower(m[0])) {
    return { text: t('downloads:atlas.failedSafetyCheck'), at: m.index };
  }
  return null;
}

/** An error toast's title and optional raw description (audit F4, pure for
 *  tests): a "Title: engine error" keeps its title and swaps only the error
 *  part for the friendly sentence; a bare engine string swaps wholly.
 *  Plugin-written sentences pass through `plainError` as before. */
export function errorToastParts(
  msg: string,
  t: (key: string, opts?: Record<string, unknown>) => string,
  fallback: string,
): { title: string; description?: string } {
  const m = msg.trim();
  const friendly = friendlyCoreError(m, t);
  if (friendly) {
    const prefix = m.slice(0, friendly.at).replace(/:\s*$/, '');
    const title = prefix ? `${prefix}: ${friendly.text}` : friendly.text;
    return { title, description: msg };
  }
  const raw = hasDetails(msg, fallback) ? msg : undefined;
  return { title: plainError(msg, fallback), description: raw };
}
