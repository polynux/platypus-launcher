import { useEffect, useRef } from 'react';
import i18n from './i18n';
import { inviteKey, joinDecision } from './invites';
import type { NotifyOptions } from './notify';
import type { Game, SocialView } from '../types';

/** Toast each new game invite (protocol 1.24) with a Join action. Join
 *  launches the installed game with its join args, opens the game page when
 *  it isn't installed, or explains why it can't join. Invites already here
 *  when the window mounts don't toast; the views stay the pending list.
 */
export function useInviteToasts(
  social: SocialView[],
  games: Game[],
  launchWithArgs: (game: Game, args: string[]) => void,
  openGame: (gameId: string) => void,
  notify: (msg: string, kind?: 'ok' | 'err' | 'busy', options?: NotifyOptions) => void,
): void {
  const seen = useRef(new Set<string>());
  const primed = useRef(false);
  const live = useRef({ games, launchWithArgs, openGame, notify });
  live.current = { games, launchWithArgs, openGame, notify };
  useEffect(() => {
    const current = live.current;
    if (!primed.current) {
      for (const v of social) for (const iv of v.invites ?? []) seen.current.add(inviteKey(v.plugin, iv.from.id, iv.kind, iv.at));
      primed.current = true;
      return;
    }
    for (const v of social) {
      for (const iv of v.invites ?? []) {
        const key = inviteKey(v.plugin, iv.from.id, iv.kind, iv.at);
        if (seen.current.has(key)) continue;
        seen.current.add(key);
        const title = iv.game?.title ?? '';
        const message = i18n.t('chat:invite.toast', { name: iv.from.name, title });
        const decision = joinDecision({ kind: iv.kind, connect: iv.connect, lobbyId: iv.lobbyId }, iv.game?.id, current.games);
        if (decision.kind === 'launch') {
          const { game, args } = decision;
          current.notify(message, 'busy', {
            action: { label: i18n.t('chat:invite.join'), onClick: () => current.launchWithArgs(game, args) },
          });
        } else if (decision.kind === 'page') {
          const { gameId } = decision;
          current.notify(message, 'busy', {
            action: { label: i18n.t('chat:invite.join'), onClick: () => current.openGame(gameId) },
          });
        } else {
          current.notify(message, 'busy', { description: i18n.t(`chat:invite.${decision.reason}`, { title }) });
        }
      }
    }
  }, [social]);
}
