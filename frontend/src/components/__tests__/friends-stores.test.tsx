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

const friend = (id: string, name: string, state: SocialFriend['state']): SocialFriend => ({ id, name, state });

// Two stores with their own user, friends and connection: echo is online,
// echo-b is reconnecting after a dropped session.
const echo: SocialView = {
  seq: 1, plugin: 'echo', enabled: true, showPlaying: true, running: true,
  connection: { state: 'online' },
  self: { name: 'Echo Player', state: 'online' },
  friends: [friend('1', 'Ana Ruiz', 'online'), friend('2', 'Bo Lindqvist', 'away')],
};
const echoB: SocialView = {
  seq: 1, plugin: 'echo-b', enabled: true, showPlaying: true, running: true,
  connection: { state: 'reconnecting', message: 'the store plugin stopped' },
  self: { name: 'Echo B Player', state: 'busy' },
  friends: [friend('8', 'Mira Vale', 'online')],
};
const labels = { echo: 'Echo', 'echo-b': 'Echo B' };
const plugins = [
  { id: 'echo', name: 'Echo', status: 'ready', capabilities: ['social'], declared: ['social'] },
  { id: 'echo-b', name: 'Echo B', status: 'ready', capabilities: ['social'], declared: ['social'] },
] as unknown as StorePlugin[];

function modal(views: SocialView[]) {
  return render(
    <FriendsModal
      views={views}
      labels={labels}
      plugins={plugins}
      inLibrary={() => false}
      onClose={() => {}}
      onSetEnabled={() => {}}
      onSetStatus={() => {}}
      onSetPlaying={() => {}}
      onReconnect={() => {}}
      onOpenGame={() => {}}
    />,
  );
}

describe('two stores in the friends modal', () => {
  it('shows one section per store, each with its own user and status selector', () => {
    modal([echo, echoB]);
    const first = screen.getByRole('region', { name: 'Echo friends' });
    const second = screen.getByRole('region', { name: 'Echo B friends' });
    expect(within(first).getByRole('combobox', { name: 'Your status on Echo' })).not.toBeDisabled();
    expect(within(second).getByRole('combobox', { name: 'Your status on Echo B' })).toBeDisabled();
    expect(within(first).getByText('Ana Ruiz')).toBeInTheDocument();
    expect(within(first).queryByText('Mira Vale')).toBeNull();
    expect(within(second).getByText('Mira Vale')).toBeInTheDocument();
    expect(within(second).queryByText('Ana Ruiz')).toBeNull();
  });

  it('explains a connection state in its own store only', () => {
    modal([echo, echoB]);
    const second = screen.getByRole('region', { name: 'Echo B friends' });
    expect(within(second).getByRole('status')).toHaveTextContent('Reconnecting… the store plugin stopped');
    expect(within(screen.getByRole('region', { name: 'Echo friends' })).queryByRole('status')).toBeNull();
  });

  it('turns one store off without touching the other', () => {
    const onSetEnabled = vi.fn();
    render(
      <FriendsModal
        views={[echo, echoB]}
        labels={labels}
        plugins={plugins}
        inLibrary={() => false}
        onClose={() => {}}
        onSetEnabled={onSetEnabled}
        onSetStatus={() => {}}
        onSetPlaying={() => {}}
        onReconnect={() => {}}
        onOpenGame={() => {}}
      />,
    );
    fireEvent.click(within(screen.getByRole('region', { name: 'Echo B friends' })).getByRole('button', { name: 'Turn off' }));
    expect(onSetEnabled).toHaveBeenCalledWith('echo-b', false);
  });
});

describe('the sidebar summary with several stores', () => {
  const four = [
    echo,
    echoB,
    { ...echo, seq: 1, plugin: 'gog', enabled: false, self: undefined, friends: [], connection: { state: 'offline' } } as SocialView,
    { ...echo, seq: 1, plugin: 'epic', enabled: true, self: { name: 'Epic Player', state: 'away' }, friends: [friend('9', 'Lou Petit', 'online')], connection: { state: 'online' } } as SocialView,
  ];
  const fourLabels = { echo: 'Echo', 'echo-b': 'Echo B', gog: 'GOG', epic: 'Epic' };

  it('lists one compact line per store, up to four, and the combined count', () => {
    render(<FriendsSummary views={four} labels={fourLabels} onOpen={() => {}} />);
    const block = screen.getByRole('button', { name: /Friends: / });
    for (const name of ['Echo Player', 'Echo B Player', 'Epic Player']) {
      expect(within(block).getByText(name)).toBeInTheDocument();
    }
    expect(within(block).getByText('friends off')).toBeInTheDocument();
    expect(within(block).getByText('Echo B · Busy')).toBeInTheDocument();
    // Ana, Bo (away still counts as online), Mira and Lou.
    expect(within(block).getByText('4 online')).toBeInTheDocument();
  });
});

describe('the status dot is not clipped', () => {
  it('keeps every dot outside an overflow-hidden picture, in the modal and in the summary', () => {
    modal([echo, echoB]);
    // The modal is portalled: look in the document. Dots are the avatars' corner marks.
    const dots = [...document.querySelectorAll('.rounded-full.absolute')].filter((d) => d.className.includes('bottom-0'));
    expect(dots.length).toBe(5);
    for (const dot of dots) {
      expect(dot.parentElement?.className).not.toMatch(/overflow-hidden/);
    }
  });

  it('keeps the summary dots outside the clipped picture too', () => {
    const { container } = render(<FriendsSummary views={[echo, echoB]} labels={labels} onOpen={() => {}} />);
    const dots = [...container.querySelectorAll('.rounded-full.absolute')];
    expect(dots.length).toBe(2);
    for (const dot of dots) {
      expect(dot.parentElement?.className).not.toMatch(/overflow-hidden/);
    }
  });
});
