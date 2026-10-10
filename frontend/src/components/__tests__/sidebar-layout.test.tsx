import React from 'react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/react';
import { TooltipProvider } from '../ds/tooltip';
import { Sidebar } from '../Sidebar';
import type { SocialView } from '../../types';

const base = {
  view: { name: 'library' as const },
  onNavigate: vi.fn(),
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
  canRedeem: false,
  onRedeem: () => {},
};

const side = (extra: Partial<React.ComponentProps<typeof Sidebar>> = {}) =>
  render(
    <TooltipProvider>
      <Sidebar {...base} {...extra} />
    </TooltipProvider>,
  );

const nav = () => screen.getByRole('navigation', { name: 'Main' });

describe('sidebar grouping', () => {
  beforeEach(() => {
    localStorage.clear();
    base.onNavigate.mockClear();
  });

  it('keeps the four main destinations at the top and the tools below the Tools group', () => {
    side();
    const labels = within(nav()).getAllByRole('button').map((b) => b.textContent?.replace(/\d+$/, '').trim());
    expect(labels.slice(0, 4)).toEqual(['Dashboard', 'Library', 'Downloads', 'Activity']);
    expect(within(nav()).queryByRole('button', { name: 'Settings' })).toBeNull();
  });

  it('puts Settings in the footer, outside the nav', () => {
    side();
    const settings = screen.getByRole('button', { name: 'Settings' });
    expect(nav().contains(settings)).toBe(false);
    fireEvent.click(settings);
    expect(base.onNavigate).toHaveBeenCalledWith({ name: 'settings' });
  });

  it('starts Tools closed, opens it on click, and remembers the choice', () => {
    const first = side();
    const toggle = screen.getByRole('button', { name: /Tools/ });
    expect(toggle.getAttribute('aria-expanded')).toBe('false');
    fireEvent.click(toggle);
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
    expect(localStorage.getItem('platypus.sidebar.toolsOpen')).toBe('true');
    first.unmount();

    side();
    expect(screen.getByRole('button', { name: /Tools/ }).getAttribute('aria-expanded')).toBe('true');
    expect(screen.getByRole('button', { name: 'Fixes' })).toBeInTheDocument();
  });

  it('opens Tools when a tool is the current view, without storing that', () => {
    side({ view: { name: 'plugins' } as never });
    expect(screen.getByRole('button', { name: /Tools/ }).getAttribute('aria-expanded')).toBe('true');
    expect(localStorage.getItem('platypus.sidebar.toolsOpen')).toBeNull();
  });

  it('shows the tool count on the closed Tools header', () => {
    side({ fixCount: 2, authNeeded: true });
    const toggle = screen.getByRole('button', { name: /Tools/ });
    expect(within(toggle).getByText('2')).toBeInTheDocument();
    expect(within(toggle).getByLabelText('needs attention')).toBeInTheDocument();
  });

  it('marks the active tool with aria-current inside the group', () => {
    side({ view: { name: 'runners' } as never });
    expect(screen.getByRole('button', { name: 'Runners' }).getAttribute('aria-current')).toBe('page');
  });
});

describe('sidebar account rows', () => {
  const account = {
    seq: 1,
    plugin: 'echo',
    enabled: true,
    showPlaying: true,
    running: true,
    connection: { state: 'online' },
    friends: [],
    self: { id: 'me', name: 'Echo Account With A Long Name', state: 'online' },
  } as unknown as SocialView;

  it('gives the account name its own line, with the full name as its title', () => {
    side({ friends: [account], friendLabels: { echo: 'Echo Store' } });
    const name = screen.getByTitle('Echo Account With A Long Name');
    expect(name.className).toContain('truncate');
    expect(name.className).toContain('min-w-0');
    expect(name.textContent).toBe('Echo Account With A Long Name');
  });
});
