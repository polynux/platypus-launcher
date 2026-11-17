# Research: Steam friend game invites and join (#206)

Status: research only, written 2026-11-17 on `research/steam-invites-206`. Nothing
here changes code. One read-only live probe ran against the logged-in account on
msi (CM logon, friends list and persona state; no status change, no message, no
invite). No token, account name, steamid, friend name or address is printed
below; probe output is aggregate only.

Legend: **[src]** read in a primary source (linked). **[live]** confirmed by a
read-only probe during this research. **[inferred]** reasoned from sources, not
confirmed. **[LIVE]** needs a live check before building.

## 0. What already exists (don't re-propose)

- `social` (1.6) friends/status and `chat` (1.9) 1:1 chat are built:
  `plugins/steam-standalone/bin/platypus-steam` (`social_*`, `chat_*`) drives the
  persistent CM session in `plugins/steam-standalone/cmd/platypus-steam-dl/social.go`
  (`runSocial`, `social.go:161`), and core keeps the merged view in
  `internal/social` (`Friend`, `Game` at `social.go:68-81`).
- That session already decodes `ClientFriendsList` (767), `ClientPersonaState`
  (766) and `FriendMessagesClient.IncomingMessage#1` (`social.go:208-216`,
  `social.go:741-786`). It reads persona name/avatar/state/app/game name/last-seen
  (`social.go:576-670`) and renders friends with `{id,name,state,game:{appid,title},lastSeen}`
  (`social.go:542-566`, mapped to the protocol in `bin/platypus-steam:4660-4676`).
- It does **not** read rich presence (persona field 71), `game_lobby_id` (field 73),
  `ClientChatInvite` (800) or `ClientInviteToGame` (7005), and it drops any chat
  entry that is not type 1 or 2 (`social.go:753-761`).
- `game.launchSpec` exists (`internal/stores/plugin.go:331`, `docs/plugins/protocol.md:477`),
  including the store-client spec `["steam","-silent","-applaunch",<appid>]`
  (`bin/platypus-steam:4236-4244`) and the standalone spec that exports
  `SteamAppId`/`SteamGameId` for "optional" games (`:4221-4235`). There is no way
  to pass extra arguments today.
- `docs/research/steam-parity-gaps.md:191-211` is the earlier provisional note
  (§7); this document replaces its "[LIVE] identify the EMsg" gap.

## 1. How an incoming game invite reaches a client

Steam sends the **same** invitation over up to three channels; a client that
handles any one of them has the invitation, and the copies arrive together. The
best-documented open-source reader of exactly this traffic is Obelisk's Steam kit
(GPL-3.0, read as a reference, no code copied):
`crates/storefronts/steam-kit/src/invites.rs:1-6` names the three channels and
`:235-270` decodes them.

| # | Channel | EMsg / method | Payload | Meaning |
|---|---|---|---|---|
| 1 | `ClientInviteToGame` | **EMsg 7005** ([src](https://github.com/SteamDatabase/Protobufs/blob/master/steam/enums_clientserver.proto), `enums_clientserver.proto:1163`) | `CMsgClientInviteToGame` `{steam_id_dest=1, steam_id_src=2, connect_string=3, remote_play=4}` (`steammessages_clientserver.proto:222-227`) | "join me" invitation carrying a **command-line fragment**; the game is the CM header's `routing_appid` (field 3, `steammessages_base.proto:97`) |
| 2 | `ClientChatInvite` | **EMsg 800** (`enums_clientserver.proto:277`) | `CMsgClientChatInvite` `{steam_id_invited=1, steam_id_chat=2, steam_id_patron=3, chatroom_type=4, steam_id_friend_chat=5, chat_name=6, game_id=7}` (`steammessages_clientserver.proto:229-237`) | **lobby** invitation: `steam_id_chat` is the lobby SteamID, `steam_id_patron` the inviter, `game_id` a CGameID whose low 24 bits are the app id, `chatroom_type == 3` (`EChatRoomType::Lobby`, `SteamKit/Resources/SteamLanguage/enums.steamd:606-611`) |
| 3 | `FriendMessagesClient.IncomingMessage#1` | service method (site `SteamClient`; `steammessages_friendmessages.steamclient.proto:192-199`) | `CFriendMessages_IncomingMessage_Notification` `{steamid_friend=1, chat_entry_type=2, from_limited_account=3, message=4, rtime32_server_timestamp=5, ordinal=6, local_echo=7, message_no_bbcode=8, low_priority=9}` (`:151-162`) | a 1:1 chat message whose BBCode is the invite card; `chat_entry_type` is `k_EChatEntryTypeInviteGame = 3` (`EChatEntryType`, `SteamKit2/SteamKit2/Base/Generated/SteamLanguage.cs:1886-1901`; same value in `ValvePython/steam` `steam/enums/common.py:545`) |

The BBCode bodies, as they appear in real history and in tests:

```
[lobbyinvite appid="477160" lobbyid="109775241000000001"][/lobbyinvite]
[gameinvite appid="620" connect="+connect 1.2.3.4:27015"][/gameinvite]
[gameinvite  appid="570" connect="+invite 30095787509687845 -launchsource2"][/gameinvite]
```

- Reader rules that matter (Obelisk `invites.rs:44-72`, tests `:400-460`;
  SteamInRust `crates/model/src/lib.rs:1440-1470`): take the **first**
  `[lobbyinvite]` or `[gameinvite]` opening tag; attribute values may be quoted
  and a quoted value may contain `]`; a backslash escapes the next character;
  `appid` must be a non-zero number; `lobbyinvite` requires `lobbyid`;
  `gameinvite` requires `connect`, and an invite whose `remoteplay` attribute is
  set is a **Remote Play Together** invitation, not a server/lobby join.
- `message_no_bbcode` (field 8) is the display text and may have the invite
  markup stripped: detect invites on field 4 (`message`) and only then choose
  the plain text. The plugin currently picks field 8 first (`social.go:762-765`),
  so invite detection must be moved before that.
- One conflicting observation: the gamescope-ritz research states an incoming
  invite "arrives as a chat entry of type `k_EChatEntryTypeInviteGame`"
  (`superdoc/planning/steam-invite-and-vtable-layout.md:361`), while Obelisk
  only accepts `chat_entry_type == 1` and ignores everything else
  (`invites.rs:227-232`). Both are third-party readings of Steam's traffic. The
  safe rule: accept entry types **1 and 3**, then require the BBCode tag — the
  tag is what identifies the invite, and the duplicate copies are deduped.
- Redundancy: an invite typically arrives once per channel, seconds apart.
  Obelisk keeps **one invitation per friend, newest first**, and treats the same
  `(friend, appid, invite)` within 120 s as a repeat (`invites.rs:17-21`,
  `:274-300`). Core should apply the same rule.
- Nothing about an invite changes the sender's persona state: gamescope's 300 s
  callback pump saw `FriendRichPresenceUpdate_t` (336) arrive but no change on
  invite (`superdoc/features/steam-friends.md:418-428` and `superdoc/planning/steam-invite-and-vtable-layout.md:355-360`). There is **no server-side
  queryable "pending invites" list** (`superdoc/features/steam-friends.md:384-476`);
  an invite is only observable as it is pushed.

Related but not invites: `ClientChatInvite` to a **chat room** (`chatroom_type`
1 or 2) and the legacy `ClientMMSInviteToLobby` (EMsg 6621,
`steammessages_clientserver_mms.proto:232-236`) — the latter is the client→Steam
request old matchmaking sent; SteamKit sends it (`SteamMatchmaking.cs:292-300`)
and `steam-client-rs` still decodes it as an incoming event with no inviter in
the body (`src/client/events.rs:1556-1565`). Lobby invites that reach a modern
client arrive as channel 2 above.

## 2. Join: what the connect string is, and who can consume it

`connect_string` (7005), the chat `connect=` attribute and the rich presence
`connect` key are the **same** thing: a command-line fragment. Valve's docs are
explicit for `ISteamFriends::InviteUserToGame`: "If the target user accepts the
invite then the `pchConnectString` gets added to the command-line when launching
the game", and a running game receives `GameRichPresenceJoinRequested_t` with
"the value associated with the `connect` Rich Presence key"
([ISteamFriends#InviteUserToGame](https://partner.steamgames.com/doc/api/ISteamFriends#InviteUserToGame);
max 256 bytes, `k_cchMaxRichPresenceValueLength`).

So the launcher must **not** prepend `+connect`: split the string as a command
line and append the tokens. Observed shapes:

| Shape | Used by | Source |
|---|---|---|
| `+connect <ip>[:port][/password]` | Source 1 and GoldSrc dedicated servers, id Tech; the engine runs `+commands` before the first map (VDC `Command_Line_Options`: `+connect` "Immediately connects to a server address") | [VDC snapshot](https://web.archive.org/web/2024/https://developer.valvesoftware.com/wiki/Command_Line_Options); `steam://connect/<ip>[:port][/password]` in [Steam browser protocol](https://web.archive.org/web/2024/https://developer.valvesoftware.com/wiki/Steam_browser_protocol) |
| `+connect_lobby <64-bit lobby SteamID>` | Steamworks lobbies (CS2, PEAK, Rust…): Steam's own launch argument for a lobby join | [ISteamMatchmaking#InviteUserToLobby](https://partner.steamgames.com/doc/api/ISteamMatchmaking#InviteUserToLobby), [#LobbyInvite_t](https://partner.steamgames.com/doc/api/ISteamMatchmaking#LobbyInvite_t), [ISteamFriends#GameLobbyJoinRequested_t](https://partner.steamgames.com/doc/api/ISteamFriends#GameLobbyJoinRequested_t) |
| `+invite <id> -launchsource2` | Source 2 games that invite through their GC (Dota 2 fixture) | `SteamInRust` `crates/model/src/lib.rs:1443-1447` |
| arbitrary `+cvar value`, `-switch`, `%xx`-encoded | per game; `steam-client-rs` examples set `connect` to `+connect localhost:27015` (`src/services/rich_presence.rs:41`) | [src] |

Steam's own URL route is `steam://joinlobby/<appid>/<lobbyid>/<steamid>`, where
the third component is the friend's (inviter's) SteamID64
([StackOverflow 36179809](https://stackoverflow.com/questions/36179809); the
Steam client's URL table registers `joinlobby` and the literal `+connect_lobby %llu`
sits next to it in `steamui.so`, with no `steam://…invite…` route at all —
`superdoc/planning/steam-friends-join.md:283-304`,
`superdoc/planning/steam-invite-and-vtable-layout.md:320-330`). If the game is
already running, the client delivers the join to it
(`GameLobbyJoinRequested_t`/`GameRichPresenceJoinRequested_t`) instead of
relaunching.

How the *game* sees the arguments, which decides whether a non-Steam launcher can
join at all:

1. **OS argv.** Steam puts the connect string on the process command line unless
   the app opted into "Use launch command line". Any game that parses its own
   argv (Source 1 `+connect`, id Tech, many custom parsers) works launched by
   Platypus, no Steam client involved.
2. `ISteamApps::GetLaunchCommandLine` — only returns a value for
   `steam://run/<appid>//<command line>/` launches, and only when "Use launch
   command line" is enabled for the app; otherwise the string is on the OS
   command line instead
   ([ISteamApps#GetLaunchCommandLine](https://partner.steamgames.com/doc/api/ISteamApps#GetLaunchCommandLine)).
   A game that reads *only* this gets nothing from a Platypus launch: **cannot
   work without the Steam client**.
3. `ISteamApps::GetLaunchQueryParam` reads `steam://run/<appid>/?k=v;k2=v2` style
   parameters ([#GetLaunchQueryParam](https://partner.steamgames.com/doc/api/ISteamApps#GetLaunchQueryParam));
   same dependency.
4. **Steamworks state itself.** A lobby id is meaningless without
   `ISteamMatchmaking::JoinLobby`, and rich-presence/`friendGameInfo.m_steamIDLobby`
   data comes from the client's backend connection. Lobby joins therefore need
   the Steam client running (the game may attach to it even when started by
   Platypus, provided `SteamAppId` is exported — which the plugin already does
   for `optional` games, `bin/platypus-steam:4229-4234`).

Family matrix **[inferred]**:

| Games | Join string | Works launched by Platypus alone? | Notes |
|---|---|---|---|
| Source 1 / GoldSrc / id Tech (TF2, L4D2, CS:S, Garry's Mod, Quake) | `+connect host:port` | **yes** — engine parses `+connect` from argv | server browser/address join; no Steam client needed |
| Steamworks lobby games (CS2, Dota 2, PEAK, Rust, Deadlock) | `+connect_lobby <id>` / `+invite <id> …` | **no** — the game must call into Steamworks for lobby membership; `+connect_lobby` on argv is only useful if the game both parses argv and has a live Steamworks session | launch through the Steam client (`via: client`), or offer the `steam://joinlobby` URL to a running client |
| Games using `GetLaunchCommandLine` only | whatever the fragment is | **no** — needs the client's launch plumbing | `[LIVE]` per title |
| Remote Play Together (`remote_play` non-empty) | stream invite, no address | n/a | out of scope |
| Games with their own networking (Minecraft, most UE/Unity peer-to-peer) | no Steam invite at all | n/a | no presence/`connect` to join |

Playtime/ownership side effects: joining a game the account does not own means
no `game.launchSpec` (not installed), so the toast can only offer the store page.
Joining counts as playtime when Platypus launches the game — normal session
tracking, no extra work.

## 3. Sending an invite, and a friend's joinable state

**Sending.** The wire path is `CMsgClientInviteToGame` (EMsg 7005) with
`steam_id_dest` and `connect_string`; a working CM client sends exactly that
(`steam-client-rs` 0.2.0, MIT, `src/services/friends.rs:636-647`). The old
lobby route is `CMsgClientMMSInviteToLobby` (6621) `{app_id, steam_id_lobby,
steam_id_user_invited}` (SteamKit `SteamMatchmaking.InviteToLobby`,
`SteamMatchmaking.cs:292-300`). Forging the chat markup does **not** work:
node-steam-user users report `sendFriendMessage(..., '[lobbyinvite …]',
{chatEntryType: 3})` produces no invite
([issue #259](https://github.com/DoctorMcKay/node-steam-user/issues/259)), and
SteamInRust notes Steam strips `[gameinvite]` markup that did not come from its
own client (`crates/model/src/lib.rs:1315-1321`).

What sending needs is the **inviter's own** connect string, which only the game
publishes (rich presence `connect` / `steam_player_group`, or its lobby id). A
CM session can read that from its own persona state entry (the same 766 push;
the helper currently discards game data for `self`, `social.go:611-619`) if the
game runs under Steamworks. A standalone (SteamAPI-less) game publishes nothing,
so "Invite friend" can only name the game, not a join address. Scope it as a
second step after receiving/joining.

**Joinable state on a friend** (the data behind Steam's "Join game"):

| Source | Field | Use |
|---|---|---|
| persona state 766 | `rich_presence` (field 71) KV pairs (`steammessages_clientserver_friends.proto:151`) | `connect` = join command line → enables "Join game" ([ISteamFriends#SetRichPresence](https://partner.steamgames.com/doc/api/ISteamFriends#SetRichPresence)); `status` = the "view game info" line; `steam_display` = localized line; `steam_player_group` + `steam_player_group_size` = party grouping |
| persona state 766 | `game_lobby_id` (field 73, `steammessages_clientserver_friends.proto:153`) | the friend's lobby SteamID — the same value Steamworks exposes as `FriendGameInfo_t.m_steamIDLobby` ([ISteamFriends](https://partner.steamgames.com/doc/api/ISteamFriends), `FriendGameInfo_t`) → `steam://joinlobby/<appid>/<lobbyid>/<friendid>` **[inferred]** mapping of 73 to `m_steamIDLobby` |
| persona state 766 | `gameid` (56) / `game_played_app_id` (3), `game_name` (55) | the app the invite/join is for |
| `ClientRichPresenceRequest` (7502) → `ClientRichPresenceInfo` (7503) | binary KV blob `rich_presence_kv` (`steammessages_clientserver_2.proto:14-35`) | rich presence of **non-friends** (lobby peers, players from a server; [ISteamFriends#RequestFriendRichPresence](https://partner.steamgames.com/doc/api/ISteamFriends#RequestFriendRichPresence)). Friends' data already rides in 766; node-steam-user's `uploadRichPresence` shows the blob format (`components/friends.js:743-770`) |

Steam shows "Join game" when either `connect` or a lobby id is present **[inferred]** from the two docs above. The Web
API alternative (`ISteamUser/GetPlayerSummaries` exposes `lobbysteamid`,
`gameextrainfo` — the StackOverflow answer above) needs an API key and is not
acceptable here.

## 4. Implementation proposal for this repo

### 4a. Plugin and helper (no protocol change to launch)

1. `internal/steamcm/steamcm.go`: add `EMsgClientChatInvite = 800`,
   `EMsgClientInviteToGame = 7005`. Both arrive as ordinary protobuf EMsgs, so
   `hdr.Int(3)` is the routing app id (`internal/depot.Msg`).
2. `cmd/platypus-steam-dl/social.go`:
   - register `onChatInvite` (800) and `onInviteToGame` (7005) in `OnPush`
     (`social.go:208-216`). 7005: skip `remote_play != ""`, take
     `steam_id_src` as inviter, `connect_string` as the join fragment, and the
     header's `routing_appid` as the game. 800: require `chatroom_type == 3`,
     `game_id>>24 == 0`, inviter = `steam_id_patron`, lobby = `steam_id_chat`,
     app = `game_id & 0xFFFFFF`.
   - extend `onService` (`:741`): for `IncomingMessage#1`, accept entry type 3
     (and 1) and parse the **BBCode field 4** with a small `parseChatInvite`
     (first `[lobbyinvite]`/`[gameinvite]` tag, `appid`/`lobbyid`/`connect`/
     `remoteplay` attributes, quotes and backslash escapes, `appid != 0`).
     Do it before the `message_no_bbcode` substitution (`:762-765`).
   - dedupe: keep `{friend, app, kind, connect|lobby}` with a 120 s repeat
     window and one invite per friend; never emit for `local_echo`, for a
     non-individual `steam_id_src`, or for a `0` app id. Never log the connect
     string (it carries server addresses).
   - `onPersonaState` (`:576`): decode field 71 KVs and field 73 into
     `friendInfo` (`:112-121`) and `renderFriend` (`:542`), exposing
     `game.connect`, `game.lobbyId`, `game.joinable` (either present) — the
     same mapping for `self` so a later send feature can read its own state.
   - emit `{"event":"invite","params":{friendId, appId, kind, connect|lobbyId, at}}`
     and include the current invites in the `state` reply so a UI restart does
     not lose them.
3. `bin/platypus-steam`: `social_notify_event` (`:4755`) maps the helper's
   `invite` event to `notify("social.invite", …)`, resolving the friend through
   the existing friends map and the game through `social_game` (`:4644`,
   `:4660-4676`). `social_state` returns the helper's invites mapped the same
   way. Version `0.29.0` → `0.30.0`; manifest `protocol.versions` += `1.24`.

### 4b. Protocol 1.24 (small; 1.23 is taken)

New optional notifications/keys only, so an older core simply ignores them
(`internal/app/app.go:541-558` routes `social.*` to the manager, which drops
unknown names at `internal/social/social.go:271-334`):

```
social.invite { from: { id, name }, game: { id?, title?, storeId },
                kind: "connect" | "lobby", connect?, lobbyId?, at }
Friend.game += { storeId, joinable, connect?, lobbyId? }      (all optional)
social.state += { invites: [Invite] }                          (optional)
```

`storeId` is the store's raw game id (`620`) and is what the core needs even
when the game is not in the library; `game.id` stays the Platypus key and is
only present when the game is in the library (protocol convention,
`docs/plugins/protocol.md:1064-1104`). Core clamps the new strings like the rest
(`internal/social/limits.go`: 256 for `connect`, 64 for `storeId`/`lobbyId`,
`at` is unix seconds) and keeps the newest invite per friend.

The alternative (reuse `chat.message` carrying the invite) is rejected: chat
messages already have a defined shape and a dedupe/read path, and an invite is
not a conversation message.

### 4c. Core and UI

- `internal/social`: `Friend.Game` gains the four fields above; `View` gains
  `Invites`; `Notify` handles `NoteInvite = "social.invite"` with the caps and
  one-per-friend rule; the existing `Emit` of the view (`app_social.go:36-40`)
  is what the UI redraws from.
- `internal/app`: a new `LaunchWithArgs(gameID string, extraArgs []string)` next
  to `launchGame` (`app.go:1305`), threading `extraArgs` into
  `prepareLaunch` (`:1107`). `stores.Plugin.Launch` (`internal/stores/plugin.go:331`)
  appends them **after** the plugin answered `game.launchSpec`: direct spec →
  `spec.Args`; store-client spec → `spec.Command` after the app id (`steam
  -silent -applaunch <appid> +connect_lobby <id>`, exactly what Steam's own
  `steam://run/<appid>//<args>` does). This is core-owned argv, not a plugin
  capability: validate `len(args) <= 16`, each `<= 256` bytes, no NUL/CR/LF,
  and show them in the command preview. No protocol bump, no plugin change for
  launching.
- UI: the toast lib already carries an action (`NotifyOptions.action`,
  `apps/ui/src/lib/notify.ts:23-31`), so "**Ana invited you to play PEAK** — Join"
  is one option on the existing `notify` call (`apps/ui/src/App.tsx:298-313`,
  which needs to pass `action` through); "Dismiss" is the toast's own close. A
  "Join game" verb also goes on any friend row where `game.joinable` in
  `apps/ui/src/components/AtlasFriendsPanel.tsx` (its rows already show
  `f.game.title`, `:107`). Join calls `LaunchWithArgs`; if the game is not
  installed the toast says so and opens the game page.
- Join routing (core):

  | Invite `kind` | Game launch mode | Action |
  |---|---|---|
  | `connect` | direct (native/wine) | `LaunchWithArgs(game, split(connect))` |
  | `connect` | client (`mode == "client"`) | `LaunchWithArgs` too: args land after `-applaunch` |
  | `lobby` | client-capable (`CloudStatus.client.canLaunch`) | `LaunchWithArgs(game, ["+connect_lobby", lobbyId])` in client mode |
  | `lobby` | client not available | disable Join, explain: the lobby needs the Steam client; offer "Open in Steam" (`steam://joinlobby/<appid>/<lobby>/<friend>`) only when the client is running |
  | any | not installed | "Install to join" |
  | `remote_play` | — | not surfaced as a join |

- Splitting the connect fragment: whitespace split honouring double quotes and
  backslashes (the Obelisk/SteamInRust rule above), never through a shell, cap
  at 16 tokens; a fragment that fails to parse disables Join (no guessed argv).

### 4d. Verification

- Helper: table tests for `parseChatInvite` (quoted `]`, escapes, missing
  `appid`, `remoteplay`, case-insensitive tag); fake-CM tests pushing 7005 (with
  and without `routing_appid`), 800 (lobby, non-lobby `chatroom_type`, mod
  `game_id`), and `IncomingMessage#1` entry 3 / entry 1 with the tag, plus the
  120 s dedupe and the `local_echo` drop — `internal/steamcm/steamcmtest`
  already supports arbitrary pushes (`internal/steamcm/steamcmtest/fake.go:119`).
- Plugin: `social.invite` mapping and the friends mapping with `game.connect`
  (the Python suite under `plugins/steam-standalone/tests`).
- Core: `internal/social` caps/dedupe/one-per-friend, `internal/app` launch-args
  threading for both spec kinds, and a UI test for the toast → Join.
- Live (needs the user): one friend sends an invite to a game we own. That is
  the only way to confirm 7005/800 reach a social-role CM session at all
  (see §5) and that the chat entry type is 3 or 1 in practice.

## 5. Read-only live probe (msi, 2026-11-17)

Method: a throwaway Go program in the worktree importing `internal/steamcm`,
logged on with the **social** logon id and status `"none"` (never sends
`ClientChangeStatus`, `social.go:11-15`), token read from
`$PLATYPUS_PLUGIN_DATA/refresh.token` without printing it, then
`ClientRequestFriendData` for every friend and 20 s of persona pushes. The
program was deleted; it printed only counts, key names, and value *shapes*.

Results (three runs, ~24 s each):

```
friends=63 personaPushes=64 personasSeen=64 inGame=0 gameNameSet=0
states=[0:64] withRichPresence=0 gameLobbyIdSet=0 gameLobbyIdType4=0
connect: values=0 +connect=0 +connect_lobby=0 bare=0 otherPlus=[]
keys:
```

- **[live]** the friends list and persona pushes arrive and decode in a plain CM
  session (63 friends, 64 entries including self); the existing code decodes
  name/avatar/last-seen (fields 15/31/47), and the probe decoded id, state, app
  and lobby id from the same pushes.
- **[live]** at probe time every friend was offline (`persona_state = 0`), so
  fields 71 (rich presence) and 73 (`game_lobby_id`) were absent everywhere:
  the read path is wired, but the **key names and value shapes were not
  observed live**. They are [src] only (three independent readers:
  `steam-client-rs` `events.rs:1051-1067`, Obelisk, CS2's own binary strings).
- Not testable without an invite: whether 7005/800 are delivered to a
  social-role CM session (Steam may route them to the friends service) and
  whether the chat entry type is 3 or 1. **[LIVE]**
- To close both, re-run the same probe while a friend is in-game (keys appear
  in field 71) and once with a real invitation from an account we control.

## 6. Open questions / risks

1. **Delivery target for 7005/800.** Only third-party clients show these being
   received; Steam's own client is closed. If Steam only pushes them to a
   "friends/Steam client" service registration our session does not have, the
   fallback is the chat channel (type 3/1 with BBCode), which we already
   receive. The implementation should handle all three and not assume.
2. **Chat entry type** (1 vs 3) for the invite card — accept both by tag, dedupe.
3. **argv acceptance per game**: `+connect_lobby` on the OS command line only
   helps a game that parses it and can reach a running Steam client; games that
   read only `GetLaunchCommandLine` cannot be joined this way. Needs per-title
   [LIVE] checks, starting with Source 1 games (expected to work) and CS2
   (expected to need `via: client`).
4. **Two accounts needed** for an end-to-end test (send + receive); everything
   else can be tested against the fake CM.
5. **Privacy**: invites and connect strings are other users' data; keep them in
   memory, out of logs, and out of `social.state` persistence (the existing
   plugin persists chat ids only, never message text).

## Sources

Repo (this tree, line numbers on `b305edb`):
`plugins/steam-standalone/cmd/platypus-steam-dl/social.go`,
`plugins/steam-standalone/bin/platypus-steam`,
`internal/social/{social.go,limits.go}`, `internal/app/{app.go,app_social.go}`,
`internal/stores/plugin.go`, `internal/steamcm/{steamcm.go,dial*,client.go}`,
`internal/depot/pb.go`, `docs/plugins/protocol.md`,
`docs/research/steam-parity-gaps.md`, `docs/research/steam-cloud-social.md`.

Protocol definitions (fetched 2026-11-17):
[SteamDatabase/Protobufs](https://github.com/SteamDatabase/Protobufs) —
`steam/enums_clientserver.proto` (EMsg numbers), `steam/steammessages_clientserver.proto`
(`CMsgClientInviteToGame`, `CMsgClientChatInvite`), `steam/steammessages_clientserver_friends.proto`
(`CMsgClientPersonaState.Friend`, rich presence 71, `game_lobby_id` 73),
`steam/steammessages_clientserver_2.proto` (rich presence 7501-7503),
`steam/steammessages_clientserver_mms.proto` (6621),
`steam/steammessages_friendmessages.steamclient.proto` (`IncomingMessage`),
`steam/steammessages_base.proto` (`routing_appid`).

Reference clients (read, no code copied):
[SteamKit2](https://github.com/SteamRE/SteamKit) (LGPL-2.1, `EChatEntryType`,
`SteamMatchmaking.InviteToLobby`),
[ValvePython/steam](https://github.com/ValvePython/steam) (MIT, `EChatEntryType`,
`ClientInviteToGame`),
[node-steam-user](https://github.com/DoctorMcKay/node-steam-user) (MIT,
`uploadRichPresence`, chat entry switch, issue #259),
[steam-client-rs 0.2.0](https://crates.io/crates/steam-client-rs) (MIT,
`invite_to_game`, persona rich presence),
[Obelisk](https://github.com/Zagrios/Obelisk) (GPL-3.0, `steam-kit/src/invites.rs`,
the invite reader),
[SteamInRust](https://github.com/madonchik123/SteamInRust) (`[gameinvite]` parser
and `+invite … -launchsource2` fixture),
[gamescope-ritz](https://github.com/AlessandroTelesca/gamescope-ritz)
(`superdoc/features/steam-friends.md`, `superdoc/planning/steam-friends-join.md`,
`superdoc/planning/steam-invite-and-vtable-layout.md`).

Valve docs: [ISteamFriends](https://partner.steamgames.com/doc/api/ISteamFriends),
[ISteamApps](https://partner.steamgames.com/doc/api/ISteamApps),
[ISteamMatchmaking](https://partner.steamgames.com/doc/api/ISteamMatchmaking),
[Steam Matchmaking & Lobbies](https://partner.steamgames.com/doc/features/multiplayer/matchmaking),
[Enhanced Rich Presence](https://partner.steamgames.com/doc/features/enhancedrichpresence).
Valve Developer Community (via Wayback snapshots, fetched 2026-11-17):
`Command_Line_Options` (`+connect`), `Steam_browser_protocol`
(`steam://connect`, `steam://rungameid`).
