// TanStack Router for navigation (issue #140): code-based route tree in one
// file. No file-based routing, no Vite plugin, no devtools. Hash history so
// the Electron app:// page and the Vite dev server both keep the route on
// reload. Routes mirror View; App stays the one place that renders views via
// useView()/useSetView() so existing onNavigate/setView props keep their types.
import { useCallback } from 'react';
import {
  createHashHistory,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  useRouter,
  useRouterState,
  type RouterHistory,
} from '@tanstack/react-router';
import type { View } from './types';

export type GameFrom = 'dashboard' | 'library' | 'fixes';

function parseGameFrom(value: unknown): GameFrom {
  return value === 'library' || value === 'fixes' || value === 'dashboard' ? value : 'dashboard';
}

export interface ViewLocation {
  to: string;
  params?: Record<string, string>;
  search?: Record<string, unknown>;
}

// View → router location. Game `from: dashboard` and absent library
// collection / store url stay out of the URL; the view mapper defaults them
// back so the round-trip still holds and URLs stay clean.
export function viewToLocation(view: View): ViewLocation {
  switch (view.name) {
    case 'dashboard':
      return { to: '/' };
    case 'library':
      return view.collection === undefined
        ? { to: '/library', search: {} }
        : { to: '/library', search: { collection: view.collection } };
    case 'game':
      return view.from === 'dashboard'
        ? { to: '/game/$id', params: { id: view.id }, search: {} }
        : { to: '/game/$id', params: { id: view.id }, search: { from: view.from } };
    case 'activity':
      return { to: '/activity' };
    case 'fixes':
      return { to: '/fixes' };
    case 'runners':
      return { to: '/runners' };
    case 'storage':
      return { to: '/storage' };
    case 'plugins':
      return { to: '/plugins' };
    case 'downloads':
      return { to: '/downloads' };
    case 'settings':
      return { to: '/settings' };
    case 'store':
      return view.url === undefined
        ? { to: '/store/$plugin', params: { plugin: view.plugin }, search: {} }
        : { to: '/store/$plugin', params: { plugin: view.plugin }, search: { url: view.url } };
  }
}

// Pathname + validated search/params → View. Unknown paths fall back to the
// dashboard; an invalid game `from` defaults to dashboard like openGame does.
export function locationToView(
  pathname: string,
  search: Record<string, unknown> = {},
  params: Record<string, string> = {},
): View {
  if (pathname === '/') return { name: 'dashboard' };
  if (pathname === '/library') {
    const collection = typeof search.collection === 'string' ? search.collection : undefined;
    return collection === undefined ? { name: 'library' } : { name: 'library', collection };
  }
  if (pathname === '/activity') return { name: 'activity' };
  if (pathname === '/fixes') return { name: 'fixes' };
  if (pathname === '/runners') return { name: 'runners' };
  if (pathname === '/storage') return { name: 'storage' };
  if (pathname === '/plugins') return { name: 'plugins' };
  if (pathname === '/downloads') return { name: 'downloads' };
  if (pathname === '/settings') return { name: 'settings' };
  if (pathname.startsWith('/game/')) {
    const id = params.id ?? decodeURIComponent(pathname.slice('/game/'.length).split('/')[0] ?? '');
    return { name: 'game', id, from: parseGameFrom(search.from) };
  }
  if (pathname.startsWith('/store/')) {
    const plugin = params.plugin ?? decodeURIComponent(pathname.slice('/store/'.length).split('/')[0] ?? '');
    const url = typeof search.url === 'string' ? search.url : undefined;
    return url === undefined ? { name: 'store', plugin } : { name: 'store', plugin, url };
  }
  return { name: 'dashboard' };
}

// Matched route → View (what useView maps through).
export function matchToView(
  routeId: string,
  params: Record<string, string> = {},
  search: Record<string, unknown> = {},
): View {
  switch (routeId) {
    case '/':
      return { name: 'dashboard' };
    case '/library': {
      const collection = typeof search.collection === 'string' ? search.collection : undefined;
      return collection === undefined ? { name: 'library' } : { name: 'library', collection };
    }
    case '/game/$id':
      return { name: 'game', id: params.id ?? '', from: parseGameFrom(search.from) };
    case '/activity':
      return { name: 'activity' };
    case '/fixes':
      return { name: 'fixes' };
    case '/runners':
      return { name: 'runners' };
    case '/storage':
      return { name: 'storage' };
    case '/plugins':
      return { name: 'plugins' };
    case '/downloads':
      return { name: 'downloads' };
    case '/settings':
      return { name: 'settings' };
    case '/store/$plugin': {
      const url = typeof search.url === 'string' ? search.url : undefined;
      return url === undefined
        ? { name: 'store', plugin: params.plugin ?? '' }
        : { name: 'store', plugin: params.plugin ?? '', url };
    }
    default:
      return { name: 'dashboard' };
  }
}

// The current view, derived from the matched route.
export function useView(): View {
  const matches = useRouterState({ select: (s) => s.matches });
  const match = matches[matches.length - 1] as
    | { routeId?: string; params?: Record<string, string>; search?: Record<string, unknown> }
    | undefined;
  if (!match || match.routeId === '__root__') return { name: 'dashboard' };
  return matchToView(match.routeId ?? '', (match.params ?? {}) as Record<string, string>, (match.search ?? {}) as Record<string, unknown>);
}

// setView with the same type as before, implemented as navigate(). Functional
// updaters compute from the current view so existing callers keep working.
// Same-view navigations are skipped so re-clicks don't pile history entries.
export function useSetView(): (v: View | ((prev: View) => View)) => void {
  const router = useRouter();
  return useCallback(
    (v: View | ((prev: View) => View)) => {
      const st = router.state;
      const match = st.matches[st.matches.length - 1] as
        | { routeId?: string; params?: Record<string, string>; search?: Record<string, unknown> }
        | undefined;
      const prev: View =
        !match || match.routeId === '__root__'
          ? { name: 'dashboard' }
          : matchToView(match.routeId ?? '', (match.params ?? {}) as Record<string, string>, (match.search ?? {}) as Record<string, unknown>);
      const next = typeof v === 'function' ? (v as (p: View) => View)(prev) : v;
      if (JSON.stringify(next) === JSON.stringify(prev)) return;
      void router.navigate(viewToLocation(next) as never);
    },
    [router],
  );
}

export function buildRouteTree(AppComponent: React.ComponentType) {
  const rootRoute = createRootRoute({ component: AppComponent as never });
  const dashboardRoute = createRoute({ getParentRoute: () => rootRoute, path: '/' });
  const libraryRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/library',
    validateSearch: (search: Record<string, unknown>) => ({
      collection: typeof search.collection === 'string' ? search.collection : undefined,
    }),
  });
  const gameRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/game/$id',
    validateSearch: (search: Record<string, unknown>) => ({ from: parseGameFrom(search.from) }),
  });
  const activityRoute = createRoute({ getParentRoute: () => rootRoute, path: '/activity' });
  const fixesRoute = createRoute({ getParentRoute: () => rootRoute, path: '/fixes' });
  const runnersRoute = createRoute({ getParentRoute: () => rootRoute, path: '/runners' });
  const storageRoute = createRoute({ getParentRoute: () => rootRoute, path: '/storage' });
  const pluginsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/plugins' });
  const downloadsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/downloads' });
  const settingsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/settings' });
  const storeRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/store/$plugin',
    validateSearch: (search: Record<string, unknown>) => ({
      url: typeof search.url === 'string' ? search.url : undefined,
    }),
  });
  const routeTree = rootRoute.addChildren([
    dashboardRoute,
    libraryRoute,
    gameRoute,
    activityRoute,
    fixesRoute,
    runnersRoute,
    storageRoute,
    pluginsRoute,
    downloadsRoute,
    settingsRoute,
    storeRoute,
  ]);
  return routeTree;
}

export function createAppRouter(AppComponent: React.ComponentType, history?: RouterHistory): ReturnType<typeof createRouter> {
  // The project's tsconfig runs without strictNullChecks; the router's types
  // ask for it, so the options go in as never (the tree is still validated
  // at runtime by the router itself).
  return createRouter({ routeTree: buildRouteTree(AppComponent), history: history ?? createHashHistory() } as never);
}

export type AppRouter = ReturnType<typeof createAppRouter>;

declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter;
  }
}

// Memory-history router for tests: the same tree, a controllable history.
export function createTestRouter(AppComponent: React.ComponentType, initialPath: string) {
  return createAppRouter(AppComponent, createMemoryHistory({ initialEntries: [initialPath] }));
}

export { createMemoryHistory, createHashHistory };
