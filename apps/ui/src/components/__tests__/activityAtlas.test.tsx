import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';

vi.mock('../../lib/theme', async () => {
  const actual = await vi.importActual<typeof import('../../lib/theme')>('../../lib/theme');
  return { ...actual, useVariant: (axis: string) => axis === 'shell.navigation' ? 'sourcebar' : (axis === 'dashboard.presentation' ? 'home' : axis === 'fixes.presentation' ? 'cards' : axis === 'activity.presentation' ? 'days' : axis === 'runners.presentation' ? 'table' : axis === 'downloads.presentation' ? 'cards' : axis === 'storage.presentation' ? 'cards' : axis === 'plugins.presentation' ? 'directory' : axis === 'friends.presentation' ? 'panel' : axis === 'store.presentation' ? 'storefronts' : axis === 'settings-row.style' ? 'srow' : axis === 'toast.style' ? 'pill' : actual.useVariant(axis as never)) };
});

import { ActivityView } from '../ActivityView';

describe('Atlas activity filters', () => {
  it('keeps scan events out of the Updates filter', async () => {
    const event = { id: 1, ts: Date.now(), type: 'scan' as const, detail: 'Library scan · 2 installed', game: '', meta: '' };
    render(<ActivityView events={[event]} />);
    expect(screen.getByText(event.detail)).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Updates' }));
    await waitFor(() => expect(screen.queryByText(event.detail)).toBeNull());
  });
});
