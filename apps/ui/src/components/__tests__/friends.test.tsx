import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { Effect } from 'effect';
import { FriendsModal } from '../FriendsModal';
import { FriendsSummary } from '../FriendsSummary';
import type { FamilyMember, SocialFriend, SocialView, StorePlugin } from '../../types';

const api = vi.hoisted(() => ({ getFamilyMembers: vi.fn() }));

vi.mock('../../effect/runtime', () => ({
  LauncherApi: {
    getFamilyMembers: (source: string) => Effect.promise(() => api.getFamilyMembers(source)),
  },
  onJobs: () => () => {},
  onActivity: () => () => {},
  onSession: () => () => {},
}));

const friend = (id: string, name: string, state: SocialFriend['state'], game?: SocialFriend['game'], lastSeen?: number): SocialFriend =>
  ({ id, name, state, game, lastSeen });

const echo: SocialView = {
  seq: 1,
  plugin: 'echo',
  enabled: true,
  showPlaying: true,
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
  seq: 1,
  plugin: 'gog',
  enabled: false,
  showPlaying: true,
  running: false,
  connection: { state: 'offline' },
  friends: [],
};

const labels = { echo: 'Echo', gog: 'GOG' };

const plugin = (id: string, capabilities: string[]): StorePlugin =>
  ({ id, name: labels[id as keyof typeof labels], status: 'ready', capabilities, declared: capabilities }) as unknown as StorePlugin;

function modal(over: Partial<React.ComponentProps<typeof FriendsModal>> = {}) {
  const props = {
    views: [echo, other],
    labels,
    plugins: [] as StorePlugin[],
    inLibrary: (id: string) => id === 'p.echo.tiny',
    onClose: vi.fn(),
    onSetEnabled: vi.fn(),
    onSetStatus: vi.fn(),
    onSetPlaying: vi.fn(),
    onReconnect: vi.fn(),
    onOpenGame: vi.fn(),
    ...over,
  };
  return { props, ...render(<FriendsModal {...props} />) };
}

describe('FriendsModal', () => {
  beforeEach(() => {
    api.getFamilyMembers.mockReset();
    api.getFamilyMembers.mockResolvedValue([]);
  });

  it('shows the family inside the section of a store that offers it, and not elsewhere', async () => {
    const members: FamilyMember[] = [{ id: 'ana', name: 'Ana', role: 'adult', state: 'online' } as FamilyMember];
    api.getFamilyMembers.mockResolvedValue(members);
    modal({ plugins: [plugin('echo', ['social', 'family']), plugin('gog', ['social'])] });
    const echoSection = screen.getByRole('region', { name: 'Echo friends' });
    expect(await within(echoSection).findByRole('list', { name: 'Family members' })).toHaveTextContent('Ana');
    const gogSection = screen.getByRole('region', { name: 'GOG friends' });
    expect(within(gogSection).queryByRole('list', { name: 'Family members' })).toBeNull();
    expect(api.getFamilyMembers).toHaveBeenCalledTimes(1);
    expect(api.getFamilyMembers).toHaveBeenCalledWith('echo');
  });

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

  it('lets the user hide what they play, per store', () => {
    const onSetPlaying = vi.fn();
    modal({ onSetPlaying });
    const sw = screen.getByRole('switch', { name: "Show what I'm playing to friends on Echo" });
    expect(sw).toBeChecked();
    fireEvent.click(sw);
    expect(onSetPlaying).toHaveBeenCalledWith('echo', false);
  });

  it('offers to reconnect after the session was replaced, and not while online', () => {
    const onReconnect = vi.fn();
    const { rerender, props } = modal({ onReconnect });
    expect(screen.queryByRole('button', { name: 'Reconnect' })).toBeNull();

    rerender(<FriendsModal {...props} views={[{ ...echo, connection: { state: 'replaced' } }, other]} />);
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }));
    expect(onReconnect).toHaveBeenCalledWith('echo');
  });

  it('shows the first 200 friends of a group and more on request', () => {
    const many = Array.from({ length: 250 }, (_, i) => friend(`n${i}`, `Friend ${String(i).padStart(3, '0')}`, 'online'));
    modal({ views: [{ ...echo, friends: many }, other] });
    const section = screen.getByRole('region', { name: 'Echo friends' });
    expect(within(section).getByText('Online · 250')).toBeInTheDocument();
    expect(within(section).getByText('Friend 199')).toBeInTheDocument();
    expect(within(section).queryByText('Friend 200')).toBeNull();
    fireEvent.click(within(section).getByRole('button', { name: 'Show 50 more' }));
    expect(within(section).getByText('Friend 249')).toBeInTheDocument();
    expect(within(section).queryByRole('button', { name: /Show .* more/ })).toBeNull();
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
