// Game invites (protocol 1.24): splitting a join fragment into argv and
// deciding what Join does: launch the installed game, open the game page
// when it isn't installed, or stay disabled with a reason. The shapes mirror
// the core's strict allowlist (social.ParseJoinArgs); the core parses the
// raw fragment again before anything runs, so this only decides the button.
import type { Game, SocialFriend } from '../types';

/** Split a join fragment on whitespace. Quotes and backslashes never appear
 *  in a valid fragment, so their presence refuses the split: Join stays
 *  disabled rather than guessing argv.
 */
export function splitConnectArgs(fragment: string): string[] | null {
  if (fragment.includes('"') || fragment.includes('\\')) return null;
  return fragment.split(/[\t\n\r ]+/).filter((t) => t.length > 0);
}

const uintRe = /^\d{1,20}$/;
const hostRe = /^(?:[A-Za-z0-9.-]+|\[[0-9A-Fa-f:.]+\])$/;
const portRe = /^\d{1,5}$/;
const passwordRe = /^[A-Za-z0-9._~!@#$%^&*-]{1,64}$/;

function parseUintId(s: string): string | null {
  if (!uintRe.test(s)) return null;
  try {
    const n = BigInt(s);
    if (n === 0n) return null;
    return String(n);
  } catch {
    return null;
  }
}

function validJoinPassword(pw: string): boolean {
  return passwordRe.test(pw) && !pw.startsWith('-');
}

function validJoinAddr(addr: string): boolean {
  const slash = addr.indexOf('/');
  let hostport = addr;
  if (slash >= 0) {
    if (!validJoinPassword(addr.slice(slash + 1))) return false;
    hostport = addr.slice(0, slash);
  }
  let host = hostport;
  let port: string | null = null;
  if (hostport.startsWith('[')) {
    const end = hostport.indexOf(']');
    if (end < 0) return false;
    const rest = hostport.slice(end + 1);
    if (rest !== '' && !rest.startsWith(':')) return false;
    if (rest !== '') port = rest.slice(1);
    host = hostport.slice(0, end + 1);
  } else {
    if (hostport.split(':').length > 2) return false;
    const cut = hostport.indexOf(':');
    if (cut >= 0) {
      host = hostport.slice(0, cut);
      port = hostport.slice(cut + 1);
    }
  }
  if (!hostRe.test(host)) return false;
  if (port === null) return true;
  if (!portRe.test(port)) return false;
  const n = Number(port);
  return n >= 1 && n <= 65535;
}

/** Parse a raw join fragment against the allowlist; null refuses it, the
 *  same shapes the core enforces before anything runs.
 */
export function parseJoinFragment(kind: string, connect?: string | null, lobbyId?: string | null): string[] | null {
  if (kind === 'lobby') {
    const id = lobbyId ? parseUintId(lobbyId) : null;
    return id ? ['+connect_lobby', id] : null;
  }
  if (kind !== 'connect' || !connect || connect.length > 256) return null;
  const toks = splitConnectArgs(connect);
  if (!toks) return null;
  if (toks.length === 2 && toks[0] === '+connect' && validJoinAddr(toks[1])) return ['+connect', toks[1]];
  if (toks.length === 4 && toks[0] === '+connect' && toks[2] === '+password' && validJoinAddr(toks[1]) && validJoinPassword(toks[3]))
    return toks;
  if (toks.length === 2 && toks[0] === '+invite') {
    const id = parseUintId(toks[1]);
    return id ? ['+invite', id] : null;
  }
  if (toks.length === 3 && toks[0] === '+invite' && toks[2] === '-launchsource2') {
    const id = parseUintId(toks[1]);
    return id ? ['+invite', id, '-launchsource2'] : null;
  }
  return null;
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
  if (frag.kind === 'lobby' && !game.client_launch) return { kind: 'disabled', reason: 'client' };
  const args = parseJoinFragment(frag.kind, frag.connect, frag.lobbyId);
  if (!args) return { kind: 'disabled', reason: 'fragment' };
  return { kind: 'launch', game, args };
}

/** The dedupe key of a toasted invite: one toast per friend and app. */
export function inviteKey(plugin: string, fromId: string, storeId: string): string {
  return `${plugin}:${fromId}:${storeId}`;
}
