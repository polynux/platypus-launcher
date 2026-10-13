// One place turning a core error into display text (issue #39, phase 3).
//
// Coded core errors travel with a translatable payload on the thrown error's
// `data` (internal/apperr through corerpc and the shell bridge): when
// `errors:<code>` is in the catalog it renders translated with its params,
// otherwise the English message shows as before. Plain errors are unchanged.
import i18n from './i18n';

/** The translatable payload of a core error. */
export interface CoreErrorData {
  code: string;
  params?: Record<string, string>;
}

/** The translatable payload riding on an error, if it is a well-formed one. */
export function coreErrorDataOf(e: unknown): CoreErrorData | null {
  if (typeof e !== 'object' || e === null) return null;
  const data = (e as { data?: unknown }).data;
  if (typeof data !== 'object' || data === null) return null;
  const code = (data as { code?: unknown }).code;
  if (typeof code !== 'string' || code === '') return null;
  const raw = (data as { params?: unknown }).params;
  let params: Record<string, string> | undefined;
  if (raw !== undefined) {
    if (typeof raw !== 'object' || raw === null || Array.isArray(raw)) return null;
    params = {};
    for (const [k, v] of Object.entries(raw as Record<string, unknown>)) {
      if (typeof v !== 'string') return null;
      params[k] = v;
    }
  }
  return params === undefined ? { code } : { code, params };
}

/** A core error as display text: the catalog entry for a coded error, the
 *  English message for anything else (plain errors, unknown codes). */
export function coreErrMsg(e: unknown): string {
  const message = e instanceof Error ? e.message : String(e);
  const data = coreErrorDataOf(e);
  if (data && i18n.exists(`errors:${data.code}`)) {
    return i18n.t(`errors:${data.code}`, data.params ?? {});
  }
  return message;
}
