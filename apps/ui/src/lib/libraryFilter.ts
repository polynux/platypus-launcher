// One library filter (issue #157): the ownership/store/collection/
// extra-filter/query predicate lived in three copies (LibraryView,
// LibraryListDetail, AtlasLibraryViews) that already drifted. Every
// presentation filters through filterLibrary; sorting stays per view.
import type { Collection, Game, StoreType } from '../types';
import type { TierFilter } from './compat';
import { tierFilterValue } from './compat';
import { collectionPredicate } from './collections';
import type { StatusFilter } from '../components/LibraryView';

export interface LibraryFilterOptions {
  ownership: 'installed' | 'owned';
  source: StoreType | 'all';
  storeSelection?: StoreType[];
  collection?: string;
  collections?: Collection[];
  tier: TierFilter;
  status: StatusFilter;
  lending: 'all' | 'with-me';
  query: string;
}

/** The shared library predicate: ownership, store(s), collection,
// extra filters (tier/status/lending) and the title query. Sorting stays
// with the view (sortGames with its own key), so this never sorts. */
export function filterLibrary(games: Game[], o: LibraryFilterOptions): Game[] {
  const hasBorrowed = games.some((g) => g.shared_by != null);
  const lendingFilter = hasBorrowed ? o.lending : 'all';
  const inCollection = collectionPredicate(o.collections ?? [], o.collection ?? '', Math.floor(Date.now() / 1000));
  const q = o.query.trim().toLowerCase();
  const multi = o.storeSelection;
  return games.filter(
    (g) =>
      (o.ownership === 'owned' || g.is_installed) &&
      (multi !== undefined ? multi.length === 0 || multi.includes(g.store) : o.source === 'all' || g.store === o.source) &&
      (!inCollection || inCollection(g)) &&
      matchesExtraFilters(g, o.tier, o.status, lendingFilter) &&
      (q === '' || g.title.toLowerCase().includes(q)),
  );
}

/** The tier, status and lent-to-me filters as one predicate: both library
 *  presentations filter through it, so the logic can't drift. Moved here
 *  from LibraryView (issue #157) so filterLibrary shares it without a
 *  component import cycle; LibraryView re-exports it. */
export function matchesExtraFilters(g: Game, tier: TierFilter, status: StatusFilter, lending: 'all' | 'with-me'): boolean {
  return (
    (tier === 'all' || tierFilterValue(g) === tier) &&
    (status === 'all' || (status === 'none' ? !g.status : g.status === status)) &&
    (lending === 'all' || g.shared_by != null)
  );
}
