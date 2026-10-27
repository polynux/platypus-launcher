// Navigation router tests (issue #140): every View round-trips through
// viewToLocation/useView, a reload restores the route from the initial hash,
// and back returns to the previous view.
import React from 'react';
import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { RouterProvider } from '@tanstack/react-router';
import { createAppRouter, createTestRouter, locationToView, useView, viewToLocation } from './router';
import type { View } from './types';

function Probe() {
  const view = useView();
  return <div data-testid="view">{JSON.stringify(view)}</div>;
}

function renderProbe(initialPath: string) {
  const router = createTestRouter(Probe, initialPath);
  render(<RouterProvider router={router} />);
  return router;
}

const VIEWS: View[] = [
  { name: 'dashboard' },
  { name: 'library' },
  { name: 'library', collection: 'c1' },
  { name: 'game', id: 'g1', from: 'dashboard' },
  { name: 'game', id: 'g1', from: 'library' },
  { name: 'game', id: 'g1', from: 'fixes' },
  { name: 'activity' },
  { name: 'fixes' },
  { name: 'runners' },
  { name: 'storage' },
  { name: 'plugins' },
  { name: 'downloads' },
  { name: 'settings' },
  { name: 'store', plugin: 'p1' },
  { name: 'store', plugin: 'p1', url: 'https://example.com/game' },
];

describe('router navigation (issue #140)', () => {
  afterEach(() => {
    cleanup();
    window.location.hash = '';
  });

  it.each(VIEWS.map((v) => [JSON.stringify(v), v] as const))('round-trips %s', async (_label, view) => {
    const router = renderProbe('/');
    await screen.findByTestId('view');
    await act(async () => {
      await router.navigate(viewToLocation(view) as never);
    });
    expect(screen.getByTestId('view')).toHaveTextContent(JSON.stringify(view));
  });

  it('restores the route from the initial hash (reload)', async () => {
    window.location.hash = '#/library?collection=c1';
    const router = createAppRouter(Probe);
    await router.load();
    render(<RouterProvider router={router} />);
    expect(await screen.findByTestId('view')).toHaveTextContent(
      JSON.stringify({ name: 'library', collection: 'c1' }),
    );
  });

  it('back returns to the previous view', async () => {
    const router = renderProbe('/');
    await screen.findByText('{"name":"dashboard"}');
    await act(async () => {
      await router.navigate({ to: '/library' } as never);
    });
    await screen.findByText('{"name":"library"}');
    await act(async () => {
      await router.navigate({ to: '/game/$id', params: { id: 'g1' }, search: { from: 'library' } } as never);
    });
    expect(screen.getByTestId('view')).toHaveTextContent('{"name":"game","id":"g1","from":"library"}');
    await act(async () => {
      router.history.back();
    });
    await waitFor(() => expect(screen.getByTestId('view')).toHaveTextContent('{"name":"library"}'));
  });

  it('defaults an invalid game from to dashboard', () => {
    expect(locationToView('/game/g1', { from: 'nope' }, { id: 'g1' })).toEqual({
      name: 'game',
      id: 'g1',
      from: 'dashboard',
    });
  });
});
