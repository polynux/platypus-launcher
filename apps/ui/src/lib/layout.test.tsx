// ThemeLayout empty regions: a region with no nodes renders nothing, so it
// takes no space in the view's grid.
import React from 'react';
import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { ThemeLayout } from './layout';

describe('ThemeLayout empty regions', () => {
  it('skips a region with nodes: []', () => {
    render(
      <ThemeLayout
        layout={{
          view: 'settings',
          regions: [
            { name: 'main', nodes: [] },
            { name: 'side', nodes: [{ block: 'waiting' }] },
          ],
        }}
        blocks={{ waiting: <span>Waiting</span> }}
      />
    );
    expect(document.querySelector('[data-region="main"]')).toBeNull();
    expect(document.querySelector('[data-region="side"]')?.textContent).toBe('Waiting');
  });
});
