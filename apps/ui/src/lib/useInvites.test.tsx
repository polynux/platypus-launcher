import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import i18n from './i18n';
import { useInviteToasts } from './useInvites';
import type { NotifyOptions } from './notify';
import type { Game, SocialView } from '../types';

const game = (over: Partial<Game> = {}): Game =>
  ({
    id: 'p.steam-standalone.620',
    title: 'Portal',
    source: 'steam-standalone',
    is_installed: true,
    client_launch: false,
    ...over,
  }) as Game;

const inviteView = (invite: Record<string, unknown>): SocialView[] => [
  {
    seq: 1,
    plugin: 'steam-standalone',
    enabled: true,
    showPlaying: true,
    running: true,
    connection: { state: 'online' },
    friends: [{ id: '1', name: 'Ana Ruiz', state: 'online' }],
    invites: [
      {
        from: { id: '1', name: 'Ana Ruiz' },
        game: { id: 'p.steam-standalone.620', title: 'Portal', storeId: '620' },
        kind: 'connect',
        connect: '+connect 1.2.3.4:27015',
        at: 1791400000,
        ...invite,
      },
    ],
  } as SocialView,
];

const setup = (views: SocialView[], games: Game[]) => {
  const launchWithArgs = vi.fn();
  const openGame = vi.fn();
  const calls: { msg: string; kind?: string; options?: NotifyOptions }[] = [];
  const notify = vi.fn((msg: string, kind?: 'ok' | 'err' | 'busy', options?: NotifyOptions) => {
    calls.push({ msg, kind, options });
  });
  const hook = renderHook(({ v }) => useInviteToasts(v, games, launchWithArgs, openGame, notify), {
    initialProps: { v: views },
  });
  return { hook, launchWithArgs, openGame, notify, calls };
};

afterEach(async () => {
  await act(async () => {
    await i18n.changeLanguage('en');
  });
});

describe('useInviteToasts', () => {
  it('toasts a new invite with a Join action that launches with the join args', async () => {
    await act(async () => {
      await i18n.changeLanguage('en');
    });
    const games = [game()];
    const { hook, launchWithArgs, calls } = setup([], games);
    hook.rerender({ v: inviteView({}) });
    expect(calls).toHaveLength(1);
    expect(calls[0].msg).toBe('Ana Ruiz invited you to play Portal');
    expect(calls[0].options?.action?.label).toBe('Join');
    act(() => {
      calls[0].options?.action?.onClick({} as React.MouseEvent<HTMLButtonElement>);
    });
    expect(launchWithArgs).toHaveBeenCalledWith(game(), ['+connect', '1.2.3.4:27015']);
  });

  it('does not re-toast invites that were already here, and toasts in French', async () => {
    await act(async () => {
      await i18n.changeLanguage('fr');
    });
    try {
      const games = [game()];
      const { hook, openGame, calls } = setup(inviteView({}), games);
      expect(calls).toHaveLength(0);
      hook.rerender({ v: inviteView({ lobbyId: '109775241000000001', connect: undefined, kind: 'lobby' }) });
      // A lobby join on a game without a client launch explains itself, with no Join action.
      expect(calls).toHaveLength(1);
      expect(calls[0].msg).toBe('Ana Ruiz vous invite à jouer à Portal');
      expect(calls[0].options?.action).toBeUndefined();
      expect(calls[0].options?.description).toBe("Rejoindre nécessite le client Steam, et Portal ne l'utilise pas.");
      expect(openGame).not.toHaveBeenCalled();
    } finally {
      await act(async () => {
        await i18n.changeLanguage('en');
      });
    }
  });

  it('opens the game page when the game is not installed', async () => {
    await act(async () => {
      await i18n.changeLanguage('en');
    });
    const games = [game({ is_installed: false })];
    const { hook, launchWithArgs, openGame, calls } = setup([], games);
    hook.rerender({ v: inviteView({}) });
    expect(calls).toHaveLength(1);
    act(() => {
      calls[0].options?.action?.onClick({} as React.MouseEvent<HTMLButtonElement>);
    });
    expect(openGame).toHaveBeenCalledWith('p.steam-standalone.620');
    expect(launchWithArgs).not.toHaveBeenCalled();
  });
});
