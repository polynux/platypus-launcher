import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import { Sidebar } from '../Sidebar';

const base = {
  view: { name: 'library' as const },
  onNavigate: () => {},
  search: '',
  onSearch: () => {},
  storeCounts: {},
  onPickSource: () => {},
  gameCount: 0,
  downloadCount: 0,
  authNeeded: false,
  fixCount: 0,
  runnerCount: 0,
  lastScan: null,
  sessions: [],
  onStop: () => {},
  friends: [],
  friendLabels: {},
  onOpenFriends: () => {},
};

const side = (extra: Partial<React.ComponentProps<typeof Sidebar>>) =>
  render(<TooltipProvider><Sidebar {...base} canRedeem={false} onRedeem={() => {}} {...extra} /></TooltipProvider>);

describe('one redeem entry in the sidebar', () => {
  it('shows when some ready store offers redeem, and opens the dialog', () => {
    const onRedeem = vi.fn();
    const { getByRole } = side({ canRedeem: true, onRedeem });
    fireEvent.click(getByRole('button', { name: /Redeem a code/ }));
    expect(onRedeem).toHaveBeenCalledTimes(1);
  });

  it('is hidden when no store offers redeem', () => {
    const { queryByRole } = side({ canRedeem: false });
    expect(queryByRole('button', { name: /Redeem a code/ })).toBeNull();
  });
});
