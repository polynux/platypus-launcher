import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import i18n from './i18n';
import { useInviteToasts } from './useInvites';
import type { NotifyOptions } from './notify';
import type { Game, SocialView } from '../types';

const nowSec = () => Math.floor(Date.now() / 1000);

const game = (over: Partial<Game> = {}): Game =>
  ({
    id: 'p.steam-standalone.620',
    title: 'Portal',
    source: 'steam-standalone',
    is_installed: true,
    client_launch: false,
    ...over,
  }) as Game;

const inviteView = (invite: Record<string, unknown>, from = { id: '1', name: 'Ana Ruiz' }): SocialView[] => [
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
        from,
        game: { id: 'p.steam-standalone.620', title: 'Portal', storeId: '620' },
        kind: 'connect',
        connect: '+connect 1.2.3.4:27015',
        at: nowSec(),
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
  it('toasts a new invite with a Join action that launches with the raw fragment', async () => {
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
    expect(launchWithArgs).toHaveBeenCalledWith(game(), 'connect', '+connect 1.2.3.4:27015', '');
  });

  it('never shows a Steam id in a toast', async () => {
    await act(async () => {
      await i18n.changeLanguage('en');
    });
    const games = [game()];
    const { hook, calls } = setup([], games);
    hook.rerender({ v: inviteView({}, { id: '76561198000000002', name: '76561198000000002' }) });
    expect(calls).toHaveLength(1);
    expect(calls[0].msg).toBe('a friend invited you to play Portal');
  });

  it('skips stale invites and throttles to one toast per friend per 120 s, in French', async () => {
    await act(async () => {
      await i18n.changeLanguage('fr');
    });
    try {
      const games = [game()];
      const { hook, openGame, calls } = setup([], games);
      // A 16-minute-old invite never toasts.
      hook.rerender({ v: inviteView({ at: nowSec() - 16 * 60 }) });
      expect(calls).toHaveLength(0);
      // A lobby join on a game without a client launch explains itself, with no Join action.
      hook.rerender({ v: inviteView({ lobbyId: '109775241000000001', connect: undefined, kind: 'lobby' }) });
      expect(calls).toHaveLength(1);
      expect(calls[0].msg).toBe('Ana Ruiz vous invite à jouer à Portal');
      expect(calls[0].options?.action).toBeUndefined();
      expect(calls[0].options?.description).toBe("Rejoindre nécessite le client Steam, et Portal ne l'utilise pas.");
      expect(openGame).not.toHaveBeenCalled();
      // A second invite from the same friend inside 120 s stays quiet,
      // even for another app.
      hook.rerender({
        v: inviteView({
          game: { title: 'Other', storeId: '47700' },
          kind: 'connect',
          connect: '+connect 9.9.9.9:27015',
          at: nowSec() + 2,
        }),
      });
      expect(calls).toHaveLength(1);
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
