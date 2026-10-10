/** Developer text that never belongs on screen by default: JSON-RPC codes and
 *  parameter checks, decoder and runtime errors, stack positions. */
const DEVELOPER = /invalid params|-3\d{4}|jsonrpc|\bjson:|unmarshal|unexpected end|\bEOF\b|\bnil\b|\bpanic\b|\bundefined\b|TypeError|ReferenceError|NetworkError|Failed to fetch|\bctx\b|\.go:\d+/i;

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
