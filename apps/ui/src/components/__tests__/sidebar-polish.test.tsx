import React from 'react';
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import { Sidebar } from '../Sidebar';
import type { SocialView } from '../../types';

const base = {
  view: { name: 'library' as const },
  onNavigate: () => {},
  search: '',
  onSearch: () => {},
  storeCounts: { steam: 3 },
  onPickSource: () => {},
  gameCount: 3,
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
  canRedeem: true,
  onRedeem: () => {},
};

const longStore = 'Some Very Long Storefront Name That Cannot Fit';
const view: SocialView = {
  seq: 1,
  plugin: 'echo',
  enabled: true,
  showPlaying: true,
  running: true,
  connection: { state: 'online' },
  friends: [],
  self: { id: 'me', name: 'A Long Display Name For The Account', state: 'online' },
} as SocialView;

const side = (extra: Partial<React.ComponentProps<typeof Sidebar>> = {}) =>
  render(
    <TooltipProvider>
      <Sidebar {...base} friends={[view]} friendLabels={{ echo: longStore }} {...extra} />
    </TooltipProvider>,
  );

describe('sidebar footer', () => {
  it('keeps the long store name truncated, with its full text as a tooltip', () => {
    side();
    const status = screen.getByTitle(new RegExp(longStore));
    expect(status.className).toContain('truncate');
    expect(status.className).toContain('min-w-0');
    expect(status.className).not.toContain('flex-none');
  });

  it('keeps the friends block and redeem entry out of the scrolling nav', () => {
    side();
    const nav = screen.getByRole('navigation', { name: 'Main' });
    const scroller = nav.parentElement as HTMLElement;
    expect(scroller.className).toContain('overflow-y-auto');
    expect(scroller.className).toContain('min-h-0');
    expect(screen.getByRole('button', { name: /Redeem a code/ }).closest('.overflow-y-auto')).toBeNull();
    expect(screen.getByRole('button', { name: /Friends:/ }).closest('.overflow-y-auto')).toBeNull();
  });
});
