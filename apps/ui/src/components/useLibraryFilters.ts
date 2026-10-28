// i18n: converted
// One library filter (issue #157): the ownership/store/collection/
// extra-filter/query predicate lived in three copies (LibraryView,
// LibraryListDetail, AtlasLibraryViews) that already drifted. Every
// presentation filters through filterLibrary; sorting stays per view.
// The hook owns the filter state App used to hold inline, with one search
// state and one reset shared by the list, grid and table.
import { useCallback, useState } from 'react';
import type { StoreType } from '../types';
import type { TierFilter } from '../lib/compat';
import type { StatusFilter } from './LibraryView';

export { filterLibrary, type LibraryFilterOptions } from '../lib/libraryFilter';

export type AtlasPageView = 'list' | 'grid' | 'table';

export function useLibraryFilters() {
  const [ownership, setOwnership] = useState<'installed' | 'owned'>('installed');
  const [source, setSource] = useState<StoreType | 'all'>('all');
  // The sourcebar's multi-store selection (D21 empty = every store): kept in
  // sync with the single source the sidebar and topbar use.
  const [selectedStores, setSelectedStores] = useState<StoreType[]>([]);
  // The /22 library bar shares its filter state with the list: one state
  // drives both, like the classic library mode keeps its view.
  const [tier, setTier] = useState<TierFilter>('all');
  const [status, setStatus] = useState<StatusFilter>('all');
  const [lending, setLending] = useState<'all' | 'with-me'>('all');
  const [query, setQuery] = useState('');
  // Which library view the /22 switch shows: the list-detail stage, or the
  // Atlas grid/table. Persisted like the classic library mode, so the choice
  // survives a relaunch.
  const [atlasView, setAtlasViewState] = useState<'list' | 'grid' | 'table'>(() => {
    try {
      const saved = window.localStorage.getItem('platypus.atlas.view');
      return saved === 'grid' || saved === 'table' ? saved : 'list';
    } catch {
      return 'list';
    }
  });
  const setAtlasView = useCallback((v: 'list' | 'grid' | 'table') => {
    setAtlasViewState(v);
    try {
      window.localStorage.setItem('platypus.atlas.view', v);
    } catch {
      // A private window or denied storage keeps the in-memory choice.
    }
  }, []);

  const resetLibraryFilters = useCallback(() => {
    setOwnership('owned');
    setSelectedStores([]);
    setSource('all');
    setQuery('');
    setTier('all');
    setStatus('all');
    setLending('all');
  }, []);

  return {
    ownership, setOwnership,
    source, setSource,
    selectedStores, setSelectedStores,
    tier, setTier,
    status, setStatus,
    lending, setLending,
    query, setQuery,
    atlasView, setAtlasView,
    resetLibraryFilters,
  };
}
