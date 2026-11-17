import { useEffect, useRef } from 'react';
import i18n from './i18n';
import { inviteKey, joinDecision } from './invites';
import type { NotifyOptions } from './notify';
import type { Game, SocialView } from '../types';

// A pending invite lives 15 minutes, agreed with the core (the authority on
// expiry) and the helper. At most one toast goes out per friend per 2
// minutes; the seen sets are pruned with the TTL so they stay small.
const inviteTTLSeconds = 15 * 60;
const toastGapSeconds = 120;

/** Toast each new game invite (protocol 1.24) with a Join action. Join
 *  launches the installed game with its join args (the core parses the raw
 *  fragment again before anything runs), opens the game page when it isn't
 *  installed, or explains why it can't join. A numeric sender name never
 *  reaches a toast: a Steam id is not a name. Invites already here when the
 *  window mounts don't toast.
 */
export function useInviteToasts(
  social: SocialView[],
  games: Game[],
  launchWithArgs: (game: Game, kind: string, connect: string, lobbyId: string) => void,
  openGame: (gameId: string) => void,
  notify: (msg: string, kind?: 'ok' | 'err' | 'busy', options?: NotifyOptions) => void,
): void {
  const seen = useRef(new Map<string, number>());
  const lastToast = useRef(new Map<string, number>());
  const primed = useRef(false);
  const live = useRef({ games, launchWithArgs, openGame, notify });
  live.current = { games, launchWithArgs, openGame, notify };
  useEffect(() => {
    const current = live.current;
    const now = Math.floor(Date.now() / 1000);
    for (const map of [seen.current, lastToast.current]) {
      for (const [key, at] of map) {
        if (now - at >= inviteTTLSeconds) map.delete(key);
      }
    }
    if (!primed.current) {
      for (const v of social) for (const iv of v.invites ?? []) seen.current.set(inviteToastKey(v.plugin, iv), now);
      primed.current = true;
      return;
    }
    for (const v of social) {
      for (const iv of v.invites ?? []) {
        // Stale invites never toast: the view may hold one past its TTL
        // until the next prune.
        if (!iv.at || now - iv.at >= inviteTTLSeconds) continue;
        const key = inviteToastKey(v.plugin, iv);
        if (seen.current.has(key)) continue;
        const friendKey = `${v.plugin}:${iv.from.id}`;
        if (now - (lastToast.current.get(friendKey) ?? -toastGapSeconds) < toastGapSeconds) continue;
        seen.current.set(key, now);
        lastToast.current.set(friendKey, now);
        const title = iv.game?.title ?? '';
        const sender = /^\d+$/.test(iv.from.name) ? i18n.t('chat:unknownFriend') : iv.from.name;
        const message = i18n.t('chat:invite.toast', { name: sender, title });
        const decision = joinDecision({ kind: iv.kind, connect: iv.connect, lobbyId: iv.lobbyId }, iv.game?.id, current.games);
        if (decision.kind === 'launch') {
          const { game } = decision;
          current.notify(message, 'busy', {
            action: {
              label: i18n.t('chat:invite.join'),
              onClick: () => current.launchWithArgs(game, iv.kind, iv.connect ?? '', iv.lobbyId ?? ''),
            },
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

function inviteToastKey(plugin: string, iv: { from: { id: string }; game?: { storeId?: string } | null }): string {
  return inviteKey(plugin, iv.from.id, iv.game?.storeId ?? '');
}
