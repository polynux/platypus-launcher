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
 *  plugin messages before. */
export function friendlyCoreError(
  raw: string | null | undefined,
  t: (key: string, opts?: Record<string, unknown>) => string,
): string | null {
  const text = (raw ?? '').trim();
  if (!text) return null;
  const system = text.match(/refusing to allow system folder\s+(\S+)/i);
  if (system) {
    return t('downloads:atlas.failedSystemFolder', { path: system[1].replace(/[.,;:'"]+$/, '') });
  }
  const space = text.match(/not enough space in\s+(\S+)/i);
  if (space) {
    return t('downloads:atlas.failedNoSpace', { dir: space[1].replace(/[.,;:'"]+$/, '') });
  }
  if (/refusing tar (entry|archive|symlink)/i.test(text)) {
    return t('downloads:atlas.failedSafetyCheck');
  }
  return null;
}
