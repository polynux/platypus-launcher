// Game invites (protocol 1.24): splitting a join fragment into argv and
// deciding what Join does: launch with the join args, open the game page
// when it isn't installed, or stay disabled with a reason.
import type { Game, SocialFriend } from '../types';

// joinArgCap mirrors the core's cap: a longer fragment is truncated, never
// shelled, and the core validates again before anything runs.
const joinArgCap = 16;

/** Split a join fragment into argv: whitespace-separated, double quotes
 *  group, backslash escapes the next character. Null when a quote never
 *  closes: Join stays disabled rather than guessing argv.
 */
export function splitConnectArgs(fragment: string): string[] | null {
  const out: string[] = [];
  let cur = '';
  let inToken = false;
  let quote = false;
  for (let i = 0; i < fragment.length; i++) {
    const c = fragment[i];
    if (quote) {
      if (c === '"') quote = false;
      else if (c === '\\' && i + 1 < fragment.length) cur += fragment[++i];
      else cur += c;
      continue;
    }
    if (c === '"') {
      quote = true;
      inToken = true;
      continue;
    }
    if (c === '\\' && i + 1 < fragment.length) {
      cur += fragment[++i];
      inToken = true;
      continue;
    }
    if (c === ' ' || c === '\t' || c === '\n' || c === '\r') {
      if (inToken) {
        out.push(cur);
        cur = '';
        inToken = false;
      }
      continue;
    }
    cur += c;
    inToken = true;
  }
  if (quote) return null;
  if (inToken) out.push(cur);
  return out.slice(0, joinArgCap);
}

export interface JoinFragment {
  kind: string;
  connect?: string | null;
  lobbyId?: string | null;
}

/** The invite a joinable friend row carries: its presence fragment when it
 *  has one (no client needed), else its lobby id. Null when the row isn't
 *  joinable.
 */
export function friendInvite(friend: SocialFriend): JoinFragment | null {
  const g = friend.game;
  if (!g?.joinable) return null;
  if (g.connect) return { kind: 'connect', connect: g.connect, lobbyId: g.lobbyId };
  if (g.lobbyId) return { kind: 'lobby', lobbyId: g.lobbyId };
  return null;
}

export type JoinDecision =
  | { kind: 'launch'; game: Game; args: string[] }
  | { kind: 'page'; gameId: string }
  | { kind: 'disabled'; reason: 'client' | 'fragment' | 'missing' };

/** What Join does for an invite: launch the installed game with the join
 *  args, open the game page when it isn't installed, or stay disabled.
 *  A lobby join needs the store client (the game's launch mode); without
 *  it there is nothing to join through.
 */
export function joinDecision(frag: JoinFragment, gameId: string | undefined, library: Game[]): JoinDecision {
  const game = gameId ? library.find((g) => g.id === gameId) : undefined;
  if (!game) {
    if (gameId) return { kind: 'page', gameId };
    return { kind: 'disabled', reason: 'missing' };
  }
  if (!game.is_installed) return { kind: 'page', gameId: game.id };
  if (frag.kind === 'lobby') {
    if (!game.client_launch) return { kind: 'disabled', reason: 'client' };
    if (!frag.lobbyId) return { kind: 'disabled', reason: 'fragment' };
    return { kind: 'launch', game, args: ['+connect_lobby', frag.lobbyId] };
  }
  const args = frag.connect ? splitConnectArgs(frag.connect) : null;
  if (!args || args.length === 0) return { kind: 'disabled', reason: 'fragment' };
  return { kind: 'launch', game, args };
}

/** The dedupe key of a toasted invite: one toast per friend, kind and time. */
export function inviteKey(plugin: string, fromId: string, kind: string, at: number): string {
  return `${plugin}:${fromId}:${kind}:${at}`;
}
