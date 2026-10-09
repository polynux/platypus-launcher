import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { FriendsModal } from '../FriendsModal';
import { FriendsSummary } from '../FriendsSummary';
import type { SocialFriend, SocialView } from '../../types';

const friend = (id: string, name: string, state: SocialFriend['state'], game?: SocialFriend['game'], lastSeen?: number): SocialFriend =>
  ({ id, name, state, game, lastSeen });

const echo: SocialView = {
  plugin: 'echo',
  enabled: true,
  running: true,
  connection: { state: 'online' },
  self: { name: 'Echo Player', state: 'online' },
  friends: [
    friend('1', 'Ana Ruiz', 'online', { id: 'p.echo.tiny', title: 'Echo Tiny' }),
    friend('2', 'Bo Lindqvist', 'away'),
    friend('3', 'Cy Okafor', 'busy', { title: 'Some Other Game' }),
    friend('4', 'Eli Strand', 'offline', undefined, 1791398361),
  ],
};

const other: SocialView = {
  plugin: 'gog',
  enabled: false,
  running: false,
  connection: { state: 'offline' },
  friends: [],
};

const labels = { echo: 'Echo', gog: 'GOG' };

function modal(over: Partial<React.ComponentProps<typeof FriendsModal>> = {}) {
  const props = {
    views: [echo, other],
    labels,
    inLibrary: (id: string) => id === 'p.echo.tiny',
    onClose: vi.fn(),
    onSetEnabled: vi.fn(),
    onSetStatus: vi.fn(),
    onOpenGame: vi.fn(),
    ...over,
  };
  return { props, ...render(<FriendsModal {...props} />) };
}

describe('FriendsModal', () => {
  it('shows one section per store with the groups In game, Online and Offline', () => {
    modal();
    const section = screen.getByRole('region', { name: 'Echo friends' });
    expect(within(section).getByText('In game · 2')).toBeInTheDocument();
    expect(within(section).getByText('Online · 1')).toBeInTheDocument();
    expect(within(section).getByText('Offline · 1')).toBeInTheDocument();
    expect(within(section).getByText('Ana Ruiz')).toBeInTheDocument();
    expect(within(section).getByText('Cy Okafor')).toBeInTheDocument();
    expect(within(section).getByText('Bo Lindqvist')).toBeInTheDocument();
    expect(within(section).getByText('Eli Strand')).toBeInTheDocument();
  });

  it('links a game that is in the library, and not one that is not', () => {
    const onOpenGame = vi.fn();
    modal({ onOpenGame });
    const section = screen.getByRole('region', { name: 'Echo friends' });
    expect(within(section).getByText(/Playing Echo Tiny/)).toBeInTheDocument();
    expect(within(section).getByText(/Playing Some Other Game/)).toBeInTheDocument();
    fireEvent.click(within(section).getByRole('button', { name: 'open' }));
    expect(onOpenGame).toHaveBeenCalledWith('p.echo.tiny');
    expect(within(section).getAllByRole('button', { name: 'open' })).toHaveLength(1);
  });

  it('searches every store at once', () => {
    modal();
    fireEvent.change(screen.getByLabelText('Search friends'), { target: { value: 'eli' } });
    const section = screen.getByRole('region', { name: 'Echo friends' });
    expect(within(section).getByText('Eli Strand')).toBeInTheDocument();
    expect(within(section).queryByText('Ana Ruiz')).toBeNull();
    fireEvent.change(screen.getByLabelText('Search friends'), { target: { value: 'nobody' } });
    expect(within(section).getByText('No friends match “nobody”.')).toBeInTheDocument();
  });

  it('says when friends are off for a store and offers to turn them on', () => {
    const onSetEnabled = vi.fn();
    modal({ onSetEnabled });
    const section = screen.getByRole('region', { name: 'GOG friends' });
    expect(within(section).getByText('Friends are off for GOG. Turn them on to see who is online.')).toBeInTheDocument();
    fireEvent.click(within(section).getByRole('button', { name: 'Turn on' }));
    expect(onSetEnabled).toHaveBeenCalledWith('gog', true);
  });

  it('turns a store off from its section', () => {
    const onSetEnabled = vi.fn();
    modal({ onSetEnabled });
    fireEvent.click(within(screen.getByRole('region', { name: 'Echo friends' })).getByRole('button', { name: 'Turn off' }));
    expect(onSetEnabled).toHaveBeenCalledWith('echo', false);
  });

  it('lets the user change their status only while the store is online', () => {
    const { rerender, props } = modal();
    expect(screen.getByRole('combobox', { name: 'Your status on Echo' })).not.toBeDisabled();

    rerender(<FriendsModal {...props} views={[{ ...echo, connection: { state: 'reconnecting' } }, other]} />);
    expect(screen.getByRole('combobox', { name: 'Your status on Echo' })).toBeDisabled();
    expect(screen.getByText('Reconnecting…')).toBeInTheDocument();
  });

  it('explains a replaced session in the store section', () => {
    modal({ views: [{ ...echo, connection: { state: 'replaced' } }, other] });
    expect(screen.getByText('Another session replaced this one.')).toBeInTheDocument();
  });

  it('says so when a store has no friends yet', () => {
    modal({ views: [{ ...echo, friends: [] }] });
    expect(screen.getByText('No friends on Echo yet.')).toBeInTheDocument();
  });
});

describe('FriendsSummary', () => {
  it('shows each store and its counts, and opens the modal on click', () => {
    const onOpen = vi.fn();
    render(<FriendsSummary views={[echo, other]} labels={labels} onOpen={onOpen} />);
    const block = screen.getByRole('button', { name: /Friends: 3 online · 2 in game/ });
    expect(within(block).getByText('3 online · 2 in game')).toBeInTheDocument();
    expect(within(block).getByText('Echo Player')).toBeInTheDocument();
    expect(within(block).getByText('friends off')).toBeInTheDocument();
    fireEvent.click(block);
    expect(onOpen).toHaveBeenCalledTimes(1);
  });

  it('renders nothing when no store offers friends', () => {
    const { container } = render(<FriendsSummary views={[]} labels={{}} onOpen={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });
});
