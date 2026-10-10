import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import { FriendsModal } from '../FriendsModal';
import { FriendsSummary } from '../FriendsSummary';
import type { SocialFriend, SocialView, StorePlugin } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getFamilyMembers: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onSession: () => () => {},
}));

const ana: SocialFriend = { id: '1', name: 'Ana Ruiz', state: 'online' };
const bo: SocialFriend = { id: '2', name: 'Bo Lindqvist', state: 'away' };

const echo: SocialView = {
  seq: 1,
  plugin: 'echo',
  enabled: true,
  showPlaying: true,
  running: true,
  connection: { state: 'online' },
  friends: [ana, bo],
};

const labels = { echo: 'Echo' };
const plugins = [{ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['chat'], declared: ['chat'] }] as unknown as StorePlugin[];

function modal(over: Partial<React.ComponentProps<typeof FriendsModal>> = {}) {
  const props = {
    views: [echo],
    labels,
    plugins,
    inLibrary: () => false,
    onClose: vi.fn(),
    onSetEnabled: vi.fn(),
    onSetStatus: vi.fn(),
    onSetPlaying: vi.fn(),
    onReconnect: vi.fn(),
    onOpenGame: vi.fn(),
    ...over,
  };
  render(<FriendsModal {...props} />);
  return props;
}

describe('friends modal chat controls', () => {
  it('shows the unread count on the friend it belongs to, and nothing on the others', () => {
    modal({ unreadOf: (plugin, id) => (plugin === 'echo' && id === '1' ? 3 : 0) });
    expect(screen.getByLabelText('3 unread messages').textContent).toBe('3');
    const anaRow = screen.getByText('Ana Ruiz').closest('li') as HTMLElement;
    expect(within(anaRow).getByLabelText('3 unread messages')).toBeTruthy();
    const boRow = screen.getByText('Bo Lindqvist').closest('li') as HTMLElement;
    expect(within(boRow).queryByLabelText(/unread/)).toBeNull();
  });

  it('opens the conversation with the friend when their row is clicked', () => {
    const props = modal({ onOpenChat: vi.fn() });
    fireEvent.click(screen.getByRole('button', { name: /Bo Lindqvist/ }));
    expect(props.onOpenChat).toHaveBeenCalledWith('echo', expect.objectContaining({ id: '2', name: 'Bo Lindqvist' }));
  });

  it('has no Message action, and no row button, when the modal has no chat', () => {
    modal();
    expect(screen.queryByRole('button', { name: /Message/ })).toBeNull();
    expect(screen.queryByRole('button', { name: /Ana Ruiz/ })).toBeNull();
  });

  it('switches a store\'s message alerts, for a store that offers chat only', () => {
    const props = modal({ chatAlerts: { echo: true }, onSetChatAlerts: vi.fn() });
    const sw = screen.getByRole('switch', { name: 'Message alerts on Echo' });
    fireEvent.click(sw);
    expect(props.onSetChatAlerts).toHaveBeenCalledWith('echo', false);
  });

  it('has no alerts switch for a store without chat', () => {
    modal({ views: [{ ...echo, plugin: 'gog', enabled: true }], labels: { gog: 'GOG' }, chatAlerts: { echo: true }, onSetChatAlerts: vi.fn() });
    expect(screen.queryByRole('switch', { name: /Message alerts/ })).toBeNull();
  });
});

describe('friends summary unread badge', () => {
  const summary = (unread?: number) =>
    render(<FriendsSummary views={[echo]} labels={labels} onOpen={vi.fn()} unread={unread} />);

  it('shows the total of unread messages when there are some', () => {
    summary(5);
    expect(screen.getByLabelText('5 unread messages').textContent).toBe('5 new');
  });

  it('shows no badge when nothing is unread', () => {
    summary(0);
    expect(screen.queryByLabelText(/unread messages/)).toBeNull();
  });
});
