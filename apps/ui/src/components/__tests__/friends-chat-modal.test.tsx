import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Effect } from 'effect';
import { FriendsModal } from '../FriendsModal';
import type { SocialFriend, SocialView, StorePlugin } from '../../types';

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getFamilyMembers: () => Effect.succeed([] as never),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
}));

const ana: SocialFriend = { id: '1', name: 'Ana Ruiz', state: 'online' };

const echo: SocialView = {
  seq: 1,
  plugin: 'echo',
  enabled: true,
  showPlaying: true,
  running: true,
  connection: { state: 'online' },
  friends: [ana],
};

const plugins = [{ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['chat'], declared: ['chat'] }] as unknown as StorePlugin[];

function modal(over: Partial<React.ComponentProps<typeof FriendsModal>> = {}) {
  const props = {
    views: [echo],
    labels: { echo: 'Echo' },
    plugins,
    inLibrary: () => false,
    onClose: vi.fn(),
    onSetEnabled: vi.fn(),
    onSetStatus: vi.fn(),
    onSetPlaying: vi.fn(),
    onReconnect: vi.fn(),
    onOpenGame: vi.fn(),
    onOpenChat: vi.fn(),
    onBackFromChat: vi.fn(),
    ...over,
  };
  render(<FriendsModal {...props} />);
  return props;
}

const chatNode = (onBack: () => void, onClose: () => void) => (
  <div>
    <button type="button" onClick={onBack}>Back</button>
    <button type="button" onClick={onClose}>Close</button>
    <p>Conversation with Ana</p>
  </div>
);

describe('friends modal chat view', () => {
  it('shows the chat in place of the friends list', () => {
    modal({ chat: chatNode(vi.fn(), vi.fn()) });
    expect(screen.getByText('Conversation with Ana')).toBeTruthy();
    expect(screen.queryByPlaceholderText('Search friends…')).toBeNull();
  });

  it('Back returns to the list through the chat\'s own button', () => {
    const onBack = vi.fn();
    const onClose = vi.fn();
    modal({ chat: chatNode(onBack, onClose) });
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('Escape goes back to the list first, and does not close the modal', () => {
    const props = modal({ chat: chatNode(vi.fn(), vi.fn()) });
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' });
    expect(props.onBackFromChat).toHaveBeenCalledTimes(1);
    expect(props.onClose).not.toHaveBeenCalled();
  });

  it('Escape closes the modal from the list', () => {
    const props = modal();
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' });
    expect(props.onClose).toHaveBeenCalledTimes(1);
    expect(props.onBackFromChat).not.toHaveBeenCalled();
  });
});

describe('atlas reachability (issue #132)', () => {
  it('reaches the store family group from the friends modal', async () => {
    modal({ plugins: [{ id: 'echo', name: 'Echo', status: 'ready', capabilities: ['family'], declared: ['family'] }] as unknown as StorePlugin[] });
    // FamiliesPanel renders its labelled section once the store offers family.
    expect(await screen.findByRole('region', { name: 'Families' })).toBeInTheDocument();
  });
});
