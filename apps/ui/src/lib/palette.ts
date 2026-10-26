/** The command palette's rows and its filter (issue #124). Pure, so the
 *  filtering rules are unit-tested without rendering the dialog. */

/** One row: a place to go, a game to open, or an action on the current game. */
export interface PaletteItem {
  id: string;
  /** The heading the row sits under, in the list's own order. */
  group: string;
  label: string;
  /** A short right-hand note (hours played, "not installed", …). */
  hint?: string;
  kind: 'action' | 'place' | 'game';
  /** The store the row's glyph comes from (a storefront id or a game's source). */
  store?: string;
  /** The store name for the glyph's monogram fallback. */
  storeName?: string;
  /** The glyph art for `store`, resolved by the caller. */
  logoSrc?: string | null;
  /** The row is the current choice (the collection in the sub-mode). */
  active?: boolean;
  /** The sub-mode the row opens instead of running. */
  next?: 'collection';
  /** A row that warns: its label reads in danger. */
  tone?: 'bad';
  /** The key shown at the row's right (the lab's Play row). */
  kbd?: string;
  run: () => void;
}

/** Without a query the palette lists the places and the current game's
 *  actions, then only the first few games: typing is how you reach the rest. */
export const UNFILTERED_GAME_LIMIT = 5;

/**
 * How well `text` matches `query`: 0 for no match, then a subsequence match,
 * a substring match, and a prefix match (the highest). Case-insensitive, the
 * same rule the /21 reference design uses.
 */
export function paletteScore(query: string, text: string): number {
  const q = query.toLowerCase();
  if (q === '') return 1;
  const s = text.toLowerCase();
  if (s.includes(q)) return 2 + (s.startsWith(q) ? 1 : 0);
  let i = 0;
  for (const c of s) if (c === q[i]) i++;
  return i === q.length ? 1 : 0;
}

/** The rows the palette shows for a query: everything when it's empty (with
 *  the games capped), else the matches, best first and in built order within
 *  a score. */
export function filterPaletteItems(items: readonly PaletteItem[], query: string): PaletteItem[] {
  if (query.trim() === '') {
    let games = 0;
    return items.filter((item) => item.kind !== 'game' || games++ < UNFILTERED_GAME_LIMIT);
  }
  return items
    // The +0.5 clusters game rows with the other matches (R4-4), like the
    // lab: it applies only to rows that match at all. (The lab adds it before
    // filtering, which would also list every non-matching game.)
    .map((item, index) => ({ item, index, base: paletteScore(query, item.label) }))
    .filter((scored) => scored.base > 0)
    .map((scored) => ({ ...scored, score: scored.base + (scored.item.kind === 'game' ? 0.5 : 0) }))
    .sort((a, b) => b.score - a.score || a.index - b.index)
    .map((scored) => scored.item)
    .slice(0, 40);
}
