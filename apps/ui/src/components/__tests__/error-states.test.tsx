import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import { ErrorNotice } from '../ErrorNotice';
import { LibrarySkeleton } from '../LibrarySkeleton';
import { Sidebar } from '../Sidebar';

const FALLBACK = 'The store plugin didn\'t answer. Try again.';

describe('ErrorNotice', () => {
  it('shows a sentence core wrote and no Details for it', () => {
    const { getByRole, queryByText } = render(<ErrorNotice message="Echo Tiny is not installed" fallback={FALLBACK} />);
    expect(getByRole('alert').textContent).toContain('Echo Tiny is not installed');
    expect(queryByText('Details')).toBeNull();
  });

  it('shows a plain sentence for developer text and keeps the raw message behind Details', () => {
    const raw = 'invalid params: ctx needs os (linux or windows), an absolute installDir';
    const { getByRole, getByText, queryByText } = render(<ErrorNotice message={raw} fallback={FALLBACK} />);
    const alert = getByRole('alert');
    expect(alert.querySelector('p')?.textContent).toBe(FALLBACK);
    expect(getByText('Details')).toBeTruthy();
    expect(queryByText(raw)).toBeTruthy(); // in the disclosure's body
  });

  it('offers Try again only where it was given', () => {
    const onRetry = vi.fn();
    const { getByRole, rerender, queryByRole } = render(<ErrorNotice message="Boom" fallback={FALLBACK} onRetry={onRetry} />);
    fireEvent.click(getByRole('button', { name: /Try again/ }));
    expect(onRetry).toHaveBeenCalledTimes(1);
    rerender(<ErrorNotice message="Boom" fallback={FALLBACK} />);
    expect(queryByRole('button', { name: /Try again/ })).toBeNull();
  });
});

describe('scan state', () => {
  it('shows placeholder tiles, not an empty library', () => {
    const { container, queryByText } = render(<LibrarySkeleton />);
    expect(container.querySelector('[aria-busy="true"]')).toBeTruthy();
    expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0);
    expect(queryByText(/no games/i)).toBeNull();
  });

  it('shows the library count as pending while the first scan runs, not 0 and not a dash', () => {
    const base = {
      view: { name: 'library' as const }, onNavigate: () => {}, search: '', onSearch: () => {}, storeCounts: {},
      onPickSource: () => {}, gameCount: 0, downloadCount: 0, authNeeded: false, fixCount: 0, runnerCount: 0,
      lastScan: null, sessions: [], onStop: () => {}, friends: [], friendLabels: {}, onOpenFriends: () => {},
      canRedeem: false, onRedeem: () => {},
    };
    const { getByRole, getAllByLabelText, container } = render(
      <TooltipProvider><Sidebar {...base} loading /></TooltipProvider>,
    );
    const library = getByRole('button', { name: /Library/ });
    expect(library.textContent).not.toMatch(/\b0\b/);
    expect(getAllByLabelText('counting games').length).toBeGreaterThan(0);
    expect(container.textContent).not.toContain('—');
  });
});
