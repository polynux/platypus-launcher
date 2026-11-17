import React from 'react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import i18n from '../../lib/i18n';
import { AtlasFriendsPanel } from '../AtlasFriendsPanel';
import type { SocialFriend, SocialView } from '../../types';

const friend = (over: Partial<SocialFriend> = {}): SocialFriend => ({
  id: '1',
  name: 'Ana Ruiz',
  state: 'online',
  game: { title: 'Portal', storeId: '620', joinable: true, connect: '+connect 1.2.3.4:27015' },
  ...over,
});

const view = (friends: SocialFriend[]): SocialView =>
  ({
    seq: 1,
    plugin: 'steam-standalone',
    enabled: true,
    showPlaying: true,
    running: true,
    connection: { state: 'online' },
    friends,
  }) as SocialView;

function panel(friends: SocialFriend[], join: { onJoin?: (plugin: string, f: SocialFriend) => void; joinReason?: (plugin: string, f: SocialFriend) => string | null } = {}) {
  const onJoin = join.onJoin ?? vi.fn();
  const props = {
    views: [view(friends)],
    labels: { 'steam-standalone': 'Steam' },
    plugins: [],
    inLibrary: () => true,
    onClose: vi.fn(),
    onSetEnabled: vi.fn(),
    onSetStatus: vi.fn(),
    onSetPlaying: vi.fn(),
    onReconnect: vi.fn(),
    onOpenGame: vi.fn(),
    onJoin,
    joinReason: join.joinReason ?? (() => null),
  };
  render(<AtlasFriendsPanel {...props} />);
  return { onJoin, props };
}

afterEach(async () => {
  await act(async () => {
    await i18n.changeLanguage('en');
  });
});

describe('AtlasFriendsPanel Join', () => {
  it('shows a Join button on a joinable in-game row that joins on click', async () => {
    await act(async () => {
      await i18n.changeLanguage('en');
    });
    const { onJoin } = panel([friend()]);
    const join = screen.getByRole('button', { name: 'Join' });
    expect(join.getAttribute('title')).toBe('Join');
    fireEvent.click(join);
    expect(onJoin).toHaveBeenCalledWith('steam-standalone', expect.objectContaining({ id: '1' }));
  });

  it('shows no Join button when the friend is not joinable', async () => {
    await act(async () => {
      await i18n.changeLanguage('en');
    });
    panel([friend({ game: { title: 'Portal' } })]);
    expect(screen.queryByRole('button', { name: 'Join' })).toBeNull();
  });

  it('disables Join with the reason, in French', async () => {
    await act(async () => {
      await i18n.changeLanguage('fr');
    });
    try {
      const { onJoin } = panel([friend()], {
        joinReason: () => "Rejoindre nécessite le client Steam, et Portal ne l'utilise pas.",
      });
      const join = screen.getByRole('button', { name: 'Rejoindre' });
      expect((join as HTMLButtonElement).disabled).toBe(true);
      expect(join.getAttribute('title')).toBe("Rejoindre nécessite le client Steam, et Portal ne l'utilise pas.");
      fireEvent.click(join);
      expect(onJoin).not.toHaveBeenCalled();
    } finally {
      await act(async () => {
        await i18n.changeLanguage('en');
      });
    }
  });
});
