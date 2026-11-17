#!/usr/bin/env python3
"""A fake Steam CM server for the plugin's tests. Speaks the same wire
protocol as the real thing — WebSocket framing, the
<emsg><hdrlen><protobuf header><body> message format, Multi (gzip)
envelopes, heartbeats — over plain ws:// on localhost, and hands out an
access-token JWT for Authentication.GenerateAccessTokenForApp#1.

State lives in $FAKE_CM_STATE (JSON, re-read for every connection, and
re-read on every auth poll so tests can approve mid-login):
  logon_eresult     EResult for the logon response (default 1 = OK)
  service_eresult   EResult in the GenerateAccessTokenForApp response header
                    (default 1 = OK)
  wrap_logon_multi  when true, the logon response arrives inside a Multi
                    (gzip) envelope, like the real CM sometimes sends
  token_ttl         access-token lifetime in seconds (default 3600)
  persona           persona name Player.GetPlayerLinkDetails answers with
  personas          {steamid: name} per-id override for the same call
  send_heartbeat    when true, one ClientHeartBeat arrives between the logon
                    response and the first service response
  heartbeat_seconds heartbeat_seconds of the logon response (default 5);
                    a persistent client sends its own ClientHeartBeat at
                    this cadence, recorded like any other heartbeat
  push_after        {"<n>": ["friends", "persona", "playing", "cloud"]}
                    unsolicited pushes sent before the n-th service reply
                    (0 = right after logon), so they land between the
                    client's request and the reply it waits for:
                    friends  -> ClientFriendsList (767)
                    persona  -> ClientPersonaState (766)
                    playing  -> ClientPlayingSessionState (9600)
                    cloud    -> CloudClient.NotifyAppStateChange (146)
  kick              ClientLoggedOff EResult sent after kick_after service
                    calls, then the socket closes: 34 = the session was
                    replaced, 6 = logged in elsewhere (0 = never)
  kick_after        service calls before the kick (default 0 = after logon)
  friends           [{"id", "rel"}] scripted ClientFriendsList (rel 3 =
                    friend; 1 = blocked, 2 = pending request)
  friends_incr      the same, sent as an incremental list
  persona_states    {steamid: {name, state, app, title, ava, seen}} scripted
                    ClientPersonaState (ava = 40 hex chars of avatar_hash)
  push_body_after   {"<n>": ["friends", "friends_incr", "personas",
                    "incoming"]} scripted pushes with bodies, before the
                    n-th service reply ("0" = right after logon)
  chat_incoming     {"friend", "text", "ts", "ord", "entry", "local_echo"}
                    (or a list) incoming chat notification(s); entry 2 is
                    typing, local_echo marks our own send coming back
  chat_history      {friendid: [{ts, ord, self, text}]} seeded conversations
  chat_unread       {friendid: n} unread counts for GetActiveMessageSessions
  The log records ids, states and counts only, never message text.
  drop_after_logon  when true, the first successful logon is followed by an
                    abrupt close (no close frame), like a dead socket; the
                    drop fires once, the next logon stays up
  Auth knobs (phase 6 logins, no Valve network):
    auth_account        account name Poll reports (default "PlatypusUser" for
                        QR, else the Begin account)
    auth_steamid        steamid for auth tokens (default 76561198)
    auth_password       password Begin accepts (default "correct-horse")
    auth_guard          "none", "email", "totp" or "confirm" (default "none")
    auth_expected_code  code Update accepts (default "ABC12" for email,
                        "123456" for totp)
    auth_challenge_url  QR challenge URL handed out
    auth_new_challenge_url refreshed QR challenge URL
    unauth_close_after  seconds after an unauthenticated connection opens
                        before the fake closes it, normally (1000), once per
                        server process: the live CM closes every pre-logon
                        websocket after about a minute whatever it sends
                        (issue #182). 0 = never.
    link_details_decoys steamids the fake prepends to every
                        Player.GetPlayerLinkDetails reply, so the client
                        must match by steamid instead of accounts[0]
  Cloud saves (Auto-Cloud listings + downloads, issue #6; uploads, P3):
    cloud_changelist  {"current_change_number": N, "is_only_delta": bool,
                       "path_prefixes": [...], "machine_names": [...],
                       "files": [{"name", "sha" (hex), "time", "size",
                                  "persist" (0/1/2), "prefix", "machine"}]}
    cloud_files       {"%Root%rel": {"body": b64} | {"zip": {entry: b64}} |
                       {"deleted": true} | {"encrypted": true, ...},
                       "sha" (hex, default sha of served bytes),
                       "time", "raw_size" (default len), "wire_size"}
    cloud_quota       {"used_files", "used_bytes", "max_files", "max_bytes"}
                       (absent = unlimited)
    cloud_split_blocks  true: every uploaded file gets two blocks, so the
                       client must honor block_offset/block_length
    cloud_post_blocks   true: block drops say POST (http_method 3, else 4)
    cloud_shuffle_blocks  true: block drops come in reverse offset order;
                       the store assembles by offset, not arrival order
    cloud_explicit      {"<name>": {"body": b64, "post": bool}}: the block
                       carries its own explicit_body_data (POST by default)
    cloud_fail_files    ["<name>", ...]: begin-file and commit refuse these
                       files (mid-batch failure)
    cloud_encrypt_files  ["<name>", ...]: begin-file wants the file
                       encrypted (client must refuse and report failure)
    cloud_no_blocks     true: begin-file answers no block requests; the
                       file commits directly
    cloud_bad_method    true: block drops say HTTP method 1 (refused)
    cloud_begin_bump    true: another batch lands between the listing and
                       begin, so begin answers listing + 2 (S3 race)
    cloud_complete_eresult  eresult for the complete reply (default 1);
                       2 fails the push at completion
# Downloads ride plain HTTP on localhost: the fake prints "http <port>" as
# its second stdout line and serves /cloud/<appid>/<name>; url_host in the
# ClientFileDownload reply points at it. Uploaded blocks ride the same
# server at /upload/<appid>/<batch>/<offset>/<name>. No Valve network.
The server prints its port on stdout, serves connections one after another
and exits when its state file disappears.
"""

import base64
import gzip
import hashlib
import json
import os
import socket
import struct
import threading
import time
import urllib.parse
from http.server import BaseHTTPRequestHandler, HTTPServer

STATE = os.environ["FAKE_CM_STATE"]
GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

PROTO_MASK = 0x80000000
EMSG_MULTI = 1
EMSG_SERVICE_METHOD_RESPONSE = 147
EMSG_CLIENT_HEARTBEAT = 703
EMSG_CLIENT_LOGOFF = 706
EMSG_CLIENT_LOGON_RESPONSE = 751
EMSG_CLIENT_LOGGED_OFF = 757
EMSG_CLIENT_LICENSE_LIST = 780
EMSG_CLIENT_GET_DEPOT_KEY = 5438
EMSG_CLIENT_CHECK_BETA_PASSWORD = 5450
EMSG_CLIENT_LOGON = 5514
EMSG_CLIENT_PICS_PRODUCT_INFO = 8903
EMSG_CLIENT_PICS_ACCESS_TOKEN = 8905
EMSG_CLIENT_PICS_PRIVATE_BETA = 8907
EMSG_CLIENT_GET_USER_STATS = 818
EMSG_CLIENT_GET_USER_STATS_RESPONSE = 819
EMSG_CLIENT_HELLO = 9805
EMSG_UCM_ENUMERATE_SUBSCRIBED = 7378
EMSG_UCM_ENUMERATE_SUBSCRIBED_RESPONSE = 7379
# Push messages phase 0 fakes (enums_clientserver.proto).
EMSG_CLOUD_NOTIFY_APP_STATE_CHANGE = 146
EMSG_CLIENT_PERSONA_STATE = 766
EMSG_CLIENT_FRIENDS_LIST = 767
EMSG_CLIENT_PLAYING_SESSION_STATE = 9600
EMSG_CLIENT_CHANGE_STATUS = 716
EMSG_CLIENT_GAMES_PLAYED = 742
EMSG_CLIENT_REQUEST_FRIEND_DATA = 815
# drop_after_logon fires once per server process.
drop_fired = {}


def load():
    try:
        with open(STATE) as f:
            return json.load(f)
    except (FileNotFoundError, ValueError):
        return {}


def record(text):
    with open(STATE + ".log", "a") as f:
        f.write(text + "\n")


# --- minimal protobuf, mirroring the plugin's -------------------------------

def varint(n):
    out = bytearray()
    n &= (1 << 64) - 1
    while True:
        b = n & 0x7F
        n >>= 7
        if n:
            out.append(b | 0x80)
        else:
            out.append(b)
            return bytes(out)


def pb_int(field, n):
    return varint((field << 3)) + varint(n)


def pb_fixed64(field, n):
    return varint((field << 3) | 1) + struct.pack("<Q", n)


def pb_fixed32(field, n):
    return varint((field << 3) | 5) + struct.pack("<I", n)


def pb_str(field, s):
    if isinstance(s, str):
        s = s.encode()
    return varint((field << 3) | 2) + varint(len(s)) + s


def pb_msg(field, payload):
    return pb_str(field, payload)


def pb_parse(data):
    out, i = {}, 0
    while i < len(data):
        key, shift = 0, 0
        while True:
            b = data[i]
            i += 1
            key |= (b & 0x7F) << shift
            if not b & 0x80:
                break
            shift += 7
        field, wt = key >> 3, key & 7
        if wt == 0:
            value, shift = 0, 0
            while True:
                b = data[i]
                i += 1
                value |= (b & 0x7F) << shift
                if not b & 0x80:
                    break
                shift += 7
        elif wt == 1:
            value = struct.unpack_from("<Q", data, i)[0]
            i += 8
        elif wt == 5:
            value = struct.unpack_from("<I", data, i)[0]
            i += 4
        elif wt == 2:
            length, shift = 0, 0
            while True:
                b = data[i]
                i += 1
                length |= (b & 0x7F) << shift
                if not b & 0x80:
                    break
                shift += 7
            value = bytes(data[i:i + length])
            i += length
        else:
            break
        out.setdefault(field, []).append(value)
    return out

# wire types, for the upload tests' assertions that the client sends the
# proto's varint fields (client_id, batch_id, upload_batch_id) as varints.
_WT = {0: "varint", 1: "fixed64", 2: "bytes", 5: "fixed32"}


def pb_wire(data):
    """First wire type seen per field number."""
    out, i = {}, 0
    while i < len(data):
        key, shift = 0, 0
        while True:
            b = data[i]
            i += 1
            key |= (b & 0x7F) << shift
            if not b & 0x80:
                break
            shift += 7
        field, wt = key >> 3, key & 7
        out.setdefault(field, wt)
        if wt == 0:
            while data[i] & 0x80:
                i += 1
            i += 1
        elif wt == 1:
            i += 8
        elif wt == 5:
            i += 4
        elif wt == 2:
            length, shift = 0, 0
            while True:
                b = data[i]
                i += 1
                length |= (b & 0x7F) << shift
                if not b & 0x80:
                    break
                shift += 7
            i += length
        else:
            break
    return out


def wire_line(names, data):
    wt = pb_wire(data)
    return ",".join(f"{names.get(f, f)}={_WT.get(w, w)}" for f, w in sorted(wt.items()))

# --- websocket framing (server side: frames to the client are unmasked) -----
# All reads go through one buffer per connection: the plugin can pipeline its
# frames with the handshake request in one TCP segment, so whatever follows
# \r\n\r\n must not be dropped.

class Conn:
    def __init__(self, sock):
        self.sock = sock
        self.pend = b""

    def read(self, n):
        while len(self.pend) < n:
            c = self.sock.recv(65536)
            if not c:
                raise OSError("EOF")
            self.pend += c
        out, self.pend = self.pend[:n], self.pend[n:]
        return out

    def read_frame(self):
        first = self.read(2)
        opcode = first[0] & 0x0F
        n = first[1] & 0x7F
        masked = first[1] & 0x80
        if n == 126:
            n = struct.unpack(">H", self.read(2))[0]
        elif n == 127:
            n = struct.unpack(">Q", self.read(8))[0]
        payload = self.read(n + (4 if masked else 0))
        if masked:
            mask = payload[:4]
            payload = bytes(b ^ mask[i % 4] for i, b in enumerate(payload[4:]))
        return opcode, payload

    def send_frame(self, opcode, payload):
        head = bytes((0x80 | opcode,))
        n = len(payload)
        if n < 126:
            head += bytes((n,))
        elif n < 65536:
            head += bytes((126,)) + struct.pack(">H", n)
        else:
            head += bytes((127,)) + struct.pack(">Q", n)
        self.sock.sendall(head + payload)

    def close_frame(self):
        try:
            self.send_frame(8, struct.pack(">H", 1000))
        except OSError:
            pass


def make_token(steamid, ttl):
    """A real-shaped access-token JWT ({" alg ": "EdDSA"} header with spaces,
    like Steam's), aud "web" without "derive" — that would make it a refresh
    token."""
    now = int(time.time())

    def seg(obj):
        return base64.urlsafe_b64encode(json.dumps(obj).encode()).rstrip(b"=").decode()

    return f"{seg({'typ': 'JWT', 'alg': 'EdDSA'})}.{seg({'iss': 'steam', 'sub': str(steamid), 'aud': ['web'], 'exp': now + ttl, 'iat': now, 'nbf': now})}.{seg({'sig': True})}"


def pack(emsg, header, body):
    return struct.pack("<II", emsg | PROTO_MASK, len(header)) + header + body


def serve(conn):
    """One session: handshake, ClientHello, ClientLogon, service calls."""
    request = b""
    while b"\r\n\r\n" not in request:
        c = conn.sock.recv(4096)
        if not c:
            return
        request += c
    head, _, conn.pend = request.partition(b"\r\n\r\n")
    key = ""
    for line in head.decode("latin1").split("\r\n"):
        if line.lower().startswith("sec-websocket-key:"):
            key = line.split(":", 1)[1].strip()
    accept = base64.b64encode(hashlib.sha1((key + GUID).encode()).digest()).decode()
    conn.sock.sendall(("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\n"
                       f"Connection: Upgrade\r\nSec-WebSocket-Accept: {accept}\r\n\r\n").encode())
    record("connect")

    def send(emsg, header, body):
        conn.send_frame(2, pack(emsg, header, body))

    # Pre-logon socket cap (issue #182): the live CM closes an
    # unauthenticated websocket after about a minute whatever traffic it
    # carries, and the login session lives on the server. unauth_close_after
    # reproduces the drop once per server process; close normally (1000), as
    # the CM does. Sends from here race the read loop's replies only when
    # the connection is being closed anyway.
    started = time.monotonic()
    stop_watch = threading.Event()

    def shut():
        conn.close_frame()
        try:
            conn.sock.shutdown(socket.SHUT_RDWR)
        except OSError:
            pass

    def watchdog():
        while not stop_watch.wait(0.1):
            drop = load().get("unauth_close_after") or 0
            if drop and not drop_fired.get("unauth") and time.monotonic() - started >= drop:
                drop_fired["unauth"] = True
                record("unauth_drop")
                shut()
                return

    threading.Thread(target=watchdog, daemon=True).start()
    try:
        st = load()
        while True:
            try:
                opcode, payload = conn.read_frame()
            except OSError:
                return
            if opcode == 8:  # close
                conn.close_frame()
                return
            if opcode == 9:  # ping -> pong
                conn.send_frame(10, payload)
                continue
            if opcode != 2 or not payload:
                continue
            emsg = struct.unpack_from("<I", payload, 0)[0] & ~PROTO_MASK
            hlen = struct.unpack_from("<I", payload, 4)[0]
            hdr = pb_parse(payload[8:8 + hlen])
            body = payload[8 + hlen:]
            if emsg == EMSG_CLIENT_HELLO:
                continue
            # Phase 6 logins run before any token exists, so Authentication
            # service calls arrive on an unauthenticated connection (no logon)
            # as 9804 NonAuthed — never 151. Like the real CM, a 151 here gets
            # the socket closed normally (1000).
            if emsg == 9804:  # ServiceMethodCallFromClientNonAuthed
                target = (hdr.get(12, [b""])[0]).decode() if hdr.get(12) else ""
                if target.startswith("Authentication."):
                    serve_auth(conn, target, hdr, body, hdr.get(1, [0])[0])
                    continue
            if emsg == 151:  # ServiceMethodCallFromClient
                record("rejected 151 before logon")
                conn.close_frame()
                return
            if emsg == EMSG_CLIENT_LOGOFF:
                record("logoff")
                conn.close_frame()
                return
            if emsg == EMSG_CLIENT_LOGON:
                serve_logged_in(conn, hdr, body, st)
                return
            record(f"unexpected {emsg}")
    finally:
        stop_watch.set()


def serve_logged_in(conn, hdr, body, st):
    steamid = hdr.get(1, [0])[0]
    logon_body = pb_parse(body)
    loginid = logon_body.get(2, [0])[0]
    if not loginid and logon_body.get(11):
        loginid = pb_parse(logon_body[11][0]).get(1, [0])[0]
    record(f"logon {steamid} loginid={loginid}")
    steamid_req = steamid  # default until the service call names one
    eres = st.get("logon_eresult", 1)
    h = pb_int(1, steamid) + pb_int(2, 4711)
    resp = pb_int(1, eres) + pb_int(3, st.get("heartbeat_seconds", 5))  # eresult, heartbeat_seconds
    if st.get("wrap_logon_multi"):
        # EMsg 1 envelope carrying the gzip'd logon response; the real CM
        # packs message_body as length-prefixed chunks ([u32 len][msg]).
        envelope = pack(EMSG_CLIENT_LOGON_RESPONSE, h, resp)
        chunks = struct.pack("<I", len(envelope)) + envelope
        conn.send_frame(2, pack(EMSG_MULTI, b"",
                                pb_int(1, len(gzip.compress(chunks))) + pb_str(2, gzip.compress(chunks))))
    else:
        conn.send_frame(2, pack(EMSG_CLIENT_LOGON_RESPONSE, h, resp))
    if eres != 1:
        # The real CM also drops the connection on a rejected logon.
        conn.close_frame()
        return
    sessionid = 4711
    replied_heartbeat = False
    service_calls = 0

    def push_frame(emsg, body, target=""):
        hdr = pb_int(1, steamid) + pb_int(2, sessionid)
        if target:
            hdr += pb_str(12, target)
        conn.send_frame(2, pack(emsg, hdr, body))

    def send_pushes(count):
        for kind in (st.get("push_after") or {}).get(str(count), []):
            emsg = {"friends": 767, "persona": 766, "playing": 9600, "cloud": 146}.get(kind)
            if emsg is None:
                continue
            record(f"push {emsg} {kind}")
            conn.send_frame(2, pack(emsg, pb_int(1, steamid) + pb_int(2, sessionid), b""))
        for kind in (st.get("push_body_after") or {}).get(str(count), []):
            if kind == "friends":
                record("push 767 friends")
                push_frame(767, friends_list_body(st.get("friends") or []))
            elif kind == "friends_incr":
                record("push 767 friends_incr")
                push_frame(767, friends_list_body(st.get("friends_incr") or [], incr=True))
            elif kind == "personas":
                record("push 766 personas")
                push_frame(766, persona_body(st.get("persona_states") or {}))
            elif kind == "incoming":
                msgs = st.get("chat_incoming")
                if isinstance(msgs, dict):
                    msgs = [msgs]
                for msg in msgs or []:
                    record(f"push incoming {msg.get('friend')}")
                    push_frame(147, incoming_body(msg),
                               "FriendMessagesClient.IncomingMessage#1")

    # The licence list arrives after logon, like the real CM: package ids
    # from the "licenses" state, so licence-based tests can own DLC.
    if st.get("licenses"):
        lic = b"".join(pb_msg(2, pb_int(1, p)) for p in st["licenses"])
        conn.send_frame(2, pack(EMSG_CLIENT_LICENSE_LIST,
                                pb_int(1, steamid) + pb_int(2, sessionid), lic))
    seed_chat(st, steamid)

    send_pushes(0)
    if st.get("drop_after_logon") and not drop_fired.get("x"):
        drop_fired["x"] = True
        return
    while True:
        try:
            opcode, payload = conn.read_frame()
        except OSError:
            return
        if opcode == 8:
            conn.close_frame()
            return
        if opcode == 9:
            conn.send_frame(10, payload)
            continue
        if opcode != 2 or not payload:
            continue
        emsg = struct.unpack_from("<I", payload, 0)[0] & ~PROTO_MASK
        hlen = struct.unpack_from("<I", payload, 4)[0]
        hdr = pb_parse(payload[8:8 + hlen])
        body = payload[8 + hlen:]
        if emsg == EMSG_CLIENT_HEARTBEAT:
            # A client heartbeat: either the plugin's reply to a
            # server-initiated one (send_heartbeat) or the persistent
            # client's own cadence. Recorded as heartbeat_reply, the
            # string the plugin tests have always asserted on.
            record("heartbeat_reply")
            continue
        if emsg == EMSG_CLIENT_LOGOFF:
            record("logoff")
            conn.send_frame(2, pack(EMSG_CLIENT_LOGGED_OFF,
                                    pb_int(1, steamid) + pb_int(2, sessionid), pb_int(1, 1)))
            conn.close_frame()
            return
        if emsg in (EMSG_CLIENT_GET_DEPOT_KEY, EMSG_CLIENT_PICS_ACCESS_TOKEN,
                    EMSG_CLIENT_PICS_PRODUCT_INFO, EMSG_CLIENT_CHECK_BETA_PASSWORD,
                    EMSG_CLIENT_PICS_PRIVATE_BETA):
            serve_content(conn, emsg, hdr, body, st, steamid, sessionid)
            continue
        if emsg == EMSG_UCM_ENUMERATE_SUBSCRIBED:
            serve_subscribed(conn, hdr, body, st, steamid, sessionid)
            continue
        if emsg == EMSG_CLIENT_GET_USER_STATS:
            serve_user_stats(conn, hdr, body, st, steamid, sessionid)
            continue
        if emsg in (EMSG_CLIENT_CHANGE_STATUS, EMSG_CLIENT_GAMES_PLAYED,
                    EMSG_CLIENT_REQUEST_FRIEND_DATA):
            # A raw client message: recorded, never answered.
            serve_social_raw(emsg, body)
            continue
        target = (hdr.get(12, [b""])[0]).decode() if hdr.get(12) else ""
        jobid = hdr.get(10, [0])[0]
        if target.startswith("Authentication."):
            if serve_auth(conn, target, hdr, body, steamid):
                continue
        record(f"service {target}")
        if target == "Cloud.GetAppFileChangelist#1":
            serve_cloud_changelist(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.ClientFileDownload#1":
            serve_cloud_download(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.ClientGetAppQuotaUsage#1":
            serve_upload_quota(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.BeginAppUploadBatch#1":
            serve_upload_begin(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.ClientBeginFileUpload#1":
            serve_upload_begin_file(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.ClientCommitFileUpload#1":
            serve_upload_commit(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.CompleteAppUploadBatchBlocking#1":
            serve_upload_complete(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "Cloud.ClientConflictResolution#1":
            serve_upload_conflict(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "PublishedFile.GetDetails#1":
            serve_published_details(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "ContentServerDirectory.GetServersForSteamPipe#1":
            serve_servers(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "ContentServerDirectory.GetManifestRequestCode#1":
            serve_request_code(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "ContentServerDirectory.GetCDNAuthToken#1":
            serve_cdn_token(conn, hdr, body, st, steamid, sessionid)
            continue
        # Pushes scheduled for this call go out BEFORE the reply, so
        # they land between the client's request and the reply it waits
        # for (jobid_target matching must not lose them).
        service_calls += 1
        send_pushes(service_calls)
        if target.startswith("FriendMessages."):
            if serve_friend_messages(conn, target, hdr, body, st, steamid, sessionid):
                kick = st.get("kick", 0)
                if kick and service_calls >= st.get("kick_after", 0):
                    record(f"kick {kick} loginid={loginid}")
                    conn.send_frame(2, pack(EMSG_CLIENT_LOGGED_OFF,
                                            pb_int(1, steamid) + pb_int(2, sessionid), pb_int(1, kick)))
                    conn.close_frame()
                    return
                continue
        if target.startswith("FamilyGroups."):
            if serve_family_groups(conn, target, hdr, body, st, steamid, sessionid):
                continue
        if target.startswith("Player.GetPlayerLinkDetails") and st.get("hang_link_details"):
            # Test hook: accept the names lookup and never answer, so the
            # plugin's family fetch must ride out a silent CM. Longer than
            # any test budget; the harness kills the server on cleanup.
            time.sleep(60)
            continue
        if target.startswith("Player.GetPlayerLinkDetails"):
            # AccountPublicData: steamid, …, persona_name (field 10). The
            # real request is `repeated uint64 steamids = 1` and the reply
            # carries one account per id; the legacy self-lookup shape (a
            # nested message in field 1) parses as bytes, not ints, and
            # falls back to the session user.
            resp_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                        + pb_fixed64(11, jobid) + pb_int(13, 1))
            personas = st.get("personas") or {}
            ids = [v for v in pb_parse(body).get(1, []) if isinstance(v, int)]
            if not ids:
                ids = [steamid]
            accounts = b""
            # Decoys come first, so a client that takes accounts[0] (or
            # whose request Steam parses differently) resolves the wrong
            # name.
            decoys = [int(x) for x in (st.get("link_details_decoys") or [])]
            for want in decoys + ids:
                name = personas.get(str(want)) or st.get("persona", "TestPersona")
                if isinstance(name, dict):
                    name = name.get("name") or st.get("persona", "TestPersona")
                pub = pb_fixed64(1, want) + pb_str(10, name)
                accounts += pb_msg(1, pb_msg(1, pub))
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                    resp_hdr, accounts))
            continue
        if st.get("send_heartbeat") and not replied_heartbeat:
            replied_heartbeat = True
            conn.send_frame(2, pack(EMSG_CLIENT_HEARTBEAT,
                                    pb_int(1, steamid) + pb_int(2, sessionid), b""))
        eres = st.get("service_eresult", 1)
        steamid_req = pb_parse(body).get(2, [0])[0] or steamid
        resp_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                    + pb_fixed64(11, jobid) + pb_int(13, eres))
        resp_body = pb_str(1, make_token(steamid_req, st.get("token_ttl", 3600))) \
            if eres == 1 else b""
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, resp_hdr, resp_body))
        if eres != 1:
            conn.close_frame()
            return
        kick = st.get("kick", 0)
        if kick and service_calls >= st.get("kick_after", 0):
            record(f"kick {kick} loginid={loginid}")
            conn.send_frame(2, pack(EMSG_CLIENT_LOGGED_OFF,
                                    pb_int(1, steamid) + pb_int(2, sessionid), pb_int(1, kick)))
            conn.close_frame()
            return


# --- workshop subscriptions (protocol 1.22) ----------------------------------
# Knobs (state file):
#   workshop_items:   [{"published_file_id", "appid", "hcontent", "size",
#                       "updated", "subscribed", "depot_content"}], the
#                       account's subscriptions as UCM enumerates them.
#   workshop_details: [{"id", "title", "preview_url", "file_url",
#                       "filename", "hcontent", "size", "updated",
#                       "file_type", "children": ["id", ...]}], what
#                       PublishedFile.GetDetails answers.
def serve_subscribed(conn, hdr, body, st, steamid, sessionid):
    """CMsgClientUCMEnumerateUserSubscribedFilesWithUpdates (7378/7379):
    app_id = 1, and the answer carries eresult = 1, subscribed_files = 2
    (published_file_id = 1, rtime32_subscribed = 2, appid = 3,
    file_hcontent = 4, file_size = 5, rtime32_last_updated = 6,
    is_depot_content = 7) and total_results = 3."""
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    want = req.get(1, [0])[0]
    start = req.get(2, [0])[0]
    record(f"workshop subscribed app={want} start={start}")
    items = [it for it in st.get("workshop_items") or []
             if int(it.get("appid") or want) == want]
    total = len(items)
    page = int(st.get("workshop_page") or 0)
    items = items[start:]
    if page > 0:
        items = items[:page]
    out = pb_int(1, 1)
    for it in items:
        entry = pb_fixed64(1, int(it.get("published_file_id") or 0))
        if it.get("subscribed"):
            entry += pb_fixed32(2, int(it["subscribed"]))
        entry += pb_int(3, int(it.get("appid") or want))
        entry += pb_fixed64(4, int(it.get("hcontent") or 0))
        entry += pb_int(5, int(it.get("size") or 0))
        entry += pb_fixed32(6, int(it.get("updated") or 0))
        entry += pb_int(7, 1 if it.get("depot_content") else 0)
        out += pb_msg(2, entry)
    out += pb_int(3, total)
    resp_hdr = pb_fixed64(11, jobid)
    conn.send_frame(2, pack(EMSG_UCM_ENUMERATE_SUBSCRIBED_RESPONSE, resp_hdr, out))


def serve_published_details(conn, hdr, body, st, steamid, sessionid):
    """PublishedFile.GetDetails#1: publishedfileids = 1 (repeated fixed64),
    appid = 14; the answer is repeated publishedfiledetails = 1."""
    jobid = hdr.get(10, [0])[0]
    want = set(pb_parse(body).get(1) or [])
    record(f"workshop details {len(want)} ids")
    out = b""
    for d in st.get("workshop_details") or []:
        did = int(d.get("id") or 0)
        if did not in want:
            continue
        item = pb_int(1, 1)
        item += pb_fixed64(2, did)
        item += pb_int(4, int(d.get("creator_appid") or 0))
        item += pb_int(5, int(d.get("appid") or 0))
        if d.get("filename"):
            item += pb_str(7, d["filename"])
        item += pb_int(8, int(d.get("size") or 0))
        if d.get("file_url"):
            item += pb_str(10, d["file_url"])
        if d.get("preview_url"):
            item += pb_str(11, d["preview_url"])
        if d.get("hcontent"):
            item += pb_fixed64(14, int(d["hcontent"]))
        item += pb_str(16, d.get("title") or "")
        item += pb_int(20, int(d.get("updated") or 0))
        item += pb_int(34, int(d.get("file_type") or 0))
        for child in d.get("children") or []:
            item += pb_msg(53, pb_fixed64(1, int(child)))
        out += pb_msg(1, item)
    resp_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                + pb_fixed64(11, jobid) + pb_int(13, 1))
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, resp_hdr, out))

# --- family running apps (protocol 1.8, issue #178) ---------------------------
# Driven by the state file so the plugin tests script what the helper's
# family-running read sees, without touching Valve:
#   family_group_id: this session's family group (unset = no group: the
#            ForUser reply carries no group id and the helper stops there)
#   family_running: [{"appid", "player", "owner"}] pushed as one
#            FamilyGroupsClient.NotifyRunningApps#1 after a GetFamilyGroup
#            with send_running_apps (unset = no push: Steam's silence when
#            nothing is running). Entries for one app ride in one field-2
#            entry with one field-3 pair per player. Steamids are strings,
#            like the Web API's.
#   family_push_after_reply: push after the GetFamilyGroup reply (the real
#            ordering) instead of before it (the default, which also
#            exercises the reader's pre-reply path).
def serve_family_groups(conn, target, hdr, body, st, steamid, sessionid):
    """FamilyGroups.GetFamilyGroupForUser#1 and GetFamilyGroup#1. Returns
    True when handled (the reply — and the running-apps push — went out)."""
    jobid = hdr.get(10, [0])[0]
    if target == "FamilyGroups.GetFamilyGroupForUser#1":
        # The helper sends steamid as a uint64 varint (field 1), like the
        # group id: assert the wire shape the helper must use live. A bad
        # shape answers no group, so a wrong request can't subscribe.
        wtype = pb_wire(body).get(1)
        ok = wtype == 0 and pb_parse(body).get(1, [0])[0] == steamid
        record("family foruser ok" if ok else f"family foruser BAD wire={wtype}")
        group_id = int(st.get("family_group_id") or 0) if ok else 0
        resp_body = pb_int(1, group_id) if group_id else b""
        push = None
    elif target == "FamilyGroups.GetFamilyGroup#1":
        resp_body = b""
        push = None
        if pb_parse(body).get(2, [0])[0] and st.get("family_running") is not None:
            # One field-2 entry per app, one field-3 pair per player: two
            # pairs on one app ride in one entry, like the live push.
            groups, order = {}, []
            for e in st.get("family_running") or []:
                try:
                    appid = int(e.get("appid"))
                    player = int(e.get("player"))
                    owner = int(e.get("owner"))
                except (TypeError, ValueError, AttributeError):
                    continue
                if not appid or not player or not owner:
                    continue
                if appid not in groups:
                    groups[appid] = []
                    order.append(appid)
                groups[appid].append((player, owner))
            entries = b""
            for appid in order:
                pairs = b"".join(pb_msg(3, pb_fixed64(1, player) + pb_fixed64(2, owner))
                                 for player, owner in groups[appid])
                entries += pb_msg(2, pb_int(1, appid) + pairs)
            group_id = int(st.get("family_group_id") or 0)
            push = ((pb_int(1, group_id) if group_id else b"") + entries)
    else:
        return False
    eres = st.get("service_eresult", 1)
    resp_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                + pb_fixed64(11, jobid) + pb_int(13, eres))
    if eres != 1:
        # A rejected read: no reply body, no push — the helper degrades to
        # an empty running list.
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, resp_hdr, b""))
        return True
    if push is not None and not st.get("family_push_after_reply"):
        send_family_push(conn, st, steamid, sessionid, push)
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, resp_hdr, resp_body))
    if push is not None and st.get("family_push_after_reply"):
        # The real ordering: the push lands after the reply, so the reader
        # must drain for it (issue #178).
        time.sleep(0.2)
        send_family_push(conn, st, steamid, sessionid, push)
    return True


def send_family_push(conn, st, steamid, sessionid, push_body):
    """One FamilyGroupsClient.NotifyRunningApps#1 push: EMsg 146 with a
    target_job_name, like the live push."""
    push_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                + pb_str(12, "FamilyGroupsClient.NotifyRunningApps#1"))
    record("push 146 FamilyGroupsClient.NotifyRunningApps#1")
    conn.send_frame(2, pack(146, push_hdr, push_body))

# --- friends, persona and chat (protocol 1.6/1.9) ------------------------------
# Driven by the state file so the plugin tests script lists, pushes and
# conversations without touching Valve:
#   friends: [{"id": "7656...", "rel": 3}]      full ClientFriendsList (rel 3
#            = friend; 1 = blocked, 2 = pending; clan ids filtered by client)
#   friends_incr: [...]                         the incremental list instead
#   persona_states: {"<id>": {"name", "state", "app", "title", "ava", "seen"}}
#            state is the EPersonaState number, app the played appid, ava
#            40 hex chars of avatar_hash, seen a unix last-seen time
#   push_body_after: {"<n>": ["friends", "friends_incr", "personas", "incoming"]}
#            pushes that ride out before the nth service call's reply
#            (they also work at "0", right after logon)
#   chat_incoming: {"friend", "text", "ts", "ord", "entry", "local_echo"}
#            (or a list of those); entry 2 is typing, local_echo is our
#            own send coming back
#   chat_history: {"<friendid>": [{"ts", "ord", "self", "text"}]}
#   chat_unread: {"<friendid>": n}
# The log records ids, states and counts only, never message text.
_chat = {}  # (steamid, friend) -> [{"ts", "ord", "self", "text"}]
_chat_ts = [1791400100]
_chat_lock = threading.Lock()


def friends_list_body(entries, incr=False):
    out = b""
    if incr:
        out += pb_int(1, 1)
    for fr in entries:
        out += pb_msg(2, pb_fixed64(1, int(fr["id"])) + pb_int(2, fr.get("rel", 3)))
    return out


def persona_body(personas):
    out = b""
    for fid, p in personas.items():
        if not isinstance(p, dict):
            continue
        f = pb_fixed64(1, int(fid)) + pb_int(2, p.get("state", 1))
        if p.get("app"):
            f += pb_int(3, p["app"])
        if p.get("name"):
            f += pb_str(15, p["name"])
        if p.get("ava"):
            f += pb_str(31, bytes.fromhex(p["ava"]))
        if p.get("seen"):
            f += pb_int(47, p["seen"])
        if p.get("title"):
            f += pb_str(55, p["title"])
        out += pb_msg(2, f)
    return out


def incoming_body(msg):
    e = pb_fixed64(1, int(msg["friend"])) + pb_int(2, msg.get("entry", 1))
    if msg.get("text"):
        e += pb_str(4, msg["text"])
    e += pb_fixed32(5, msg.get("ts", 0)) + pb_int(6, msg.get("ord", 0))
    if msg.get("local_echo"):
        e += pb_int(7, 1)
    return e


def seed_chat(st, steamid):
    with _chat_lock:
        for fid, conv in (st.get("chat_history") or {}).items():
            key = (steamid, int(fid))
            if key not in _chat:
                _chat[key] = [{"ts": m["ts"], "ord": m["ord"],
                               "self": bool(m.get("self")), "text": m["text"]}
                              for m in conv]


def serve_social_raw(emsg, body):
    """A raw client message (716/742/815): recorded, never answered."""
    if emsg == EMSG_CLIENT_CHANGE_STATUS:
        record(f"status {pb_parse(body).get(1, [0])[0]}")
    elif emsg == EMSG_CLIENT_GAMES_PLAYED:
        apps = []
        for raw in pb_parse(body).get(1, []):
            apps.append(str(pb_parse(raw).get(2, [0])[0]))
        record("games_played " + (",".join(apps) if apps else "none"))
    elif emsg == EMSG_CLIENT_REQUEST_FRIEND_DATA:
        record(f"friend_data {len(pb_parse(body).get(2, []))}")


def serve_friend_messages(conn, target, hdr, body, st, steamid, sessionid):
    """FriendMessages service calls. Returns True when handled."""
    jobid = hdr.get(10, [0])[0]

    def reply(resp_body, eresult=1):
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                pb_int(1, steamid) + pb_int(2, sessionid)
                                + pb_fixed64(11, jobid) + pb_int(13, eresult),
                                resp_body))

    if target == "FriendMessages.SendMessage#1":
        m = pb_parse(body)
        friend = m.get(1, [0])[0]
        record(f"send {friend}")
        with _chat_lock:
            _chat_ts[0] += 1
            key = (steamid, friend)
            conv = _chat.setdefault(key, [])
            ord = (conv[-1]["ord"] + 1) if conv else 1
            conv.append({"ts": _chat_ts[0], "ord": ord, "self": True,
                         "text": m.get(3, [b""])[0].decode("utf-8", "replace")})
            ts = _chat_ts[0]
        reply(pb_int(2, ts) + pb_int(3, ord))
        return True
    if target == "FriendMessages.GetRecentMessages#1":
        m = pb_parse(body)
        friend = m.get(2, [0])[0]
        limit = m.get(3, [50])[0] or 50
        recent = bool(m.get(4, [0])[0])
        tl, ol = m.get(8, [0])[0], m.get(9, [0])[0]
        with _chat_lock:
            conv = [mm for mm in _chat.get((steamid, friend), [])]
        if not recent:
            conv = [mm for mm in conv if mm["ts"] < tl or (mm["ts"] == tl and mm["ord"] < ol)]
        more = len(conv) > limit
        out = b""
        for mm in conv[-limit:]:
            acct = (steamid if mm["self"] else friend) & 0xFFFFFFFF
            out += pb_msg(1, pb_int(1, acct) + pb_int(2, mm["ts"])
                          + pb_str(3, mm["text"]) + pb_int(4, mm["ord"]))
        if more:
            out += pb_int(4, 1)
        reply(out)
        return True
    if target == "FriendMessages.GetActiveMessageSessions#1":
        out = b""
        for fid, n in (st.get("chat_unread") or {}).items():
            if n:
                out += pb_msg(1, pb_int(1, int(fid) & 0xFFFFFFFF) + pb_int(4, n))
        reply(out)
        return True
    if target == "FriendMessages.AckMessage#1":
        m = pb_parse(body)
        record(f"ack {m.get(1, [0])[0]} {m.get(2, [0])[0]}")
        reply(b"")
        return True
    return False

# --- user stats (achievements) ------------------------------------------------
# Driven by the state file's "userstats" map so the plugin tests control the
# schema and unlock state without touching Valve:
#   userstats: {"<appid>": {"achievements": [{"api", "name", "desc",
#             "hidden", "icon", "unlocked"}]}}
# An app with no entry answers eresult 2 (no stats), like a game without
# achievements; "userstats": {"eresult": 2} forces that for every app.
def vdf_write(out, node):
    for k, v in node.items():
        if isinstance(v, dict):
            out.append(0)
            out += k.encode() + b"\x00"
            vdf_write(out, v)
        elif isinstance(v, str):
            out.append(1)
            out += k.encode() + b"\x00" + v.encode() + b"\x00"
        else:
            out.append(2)
            out += k.encode() + b"\x00" + struct.pack("<I", int(v))
    out.append(8)


def serve_user_stats(conn, hdr, body, st, steamid, sessionid):
    game_id = pb_parse(body).get(1, [0])[0]
    app = str(game_id & 0xFFFFFFFF)
    cfg = st.get("userstats") or {}
    h = pb_int(1, steamid) + pb_int(2, sessionid)
    if cfg.get("eresult", 1) != 1 or app not in cfg:
        record(f"userstats {app} eresult 2")
        conn.send_frame(2, pack(EMSG_CLIENT_GET_USER_STATS_RESPONSE, h,
                                pb_fixed64(1, game_id) + pb_int(2, 2)))
        return
    spec = cfg[app]
    node = {"gamename": spec.get("name", "Fake Game"), "version": 1, "stats": {}}
    bits, times = {}, []
    for i, a in enumerate(spec.get("achievements") or []):
        bits[str(i)] = {
            "name": a.get("api", f"ACH_{i}"),
            "display": {
                "name": {"english": a.get("name", f"Achievement {i}")},
                "desc": {"english": a.get("desc", "")},
                "hidden": a.get("hidden", 0),
                "icon": a.get("icon", ""),
                "icon_gray": "",
            },
        }
        times.append(1700000000 + i if a.get("unlocked") else 0)
    node["stats"]["0"] = {"type": "ACHIEVEMENTS", "bits": bits}
    out = bytearray()
    out.append(0)
    out += app.encode() + b"\x00"
    vdf_write(out, node)
    out.append(8)
    resp = pb_fixed64(1, game_id) + pb_int(2, 1) + pb_str(4, bytes(out))
    resp += pb_msg(6, pb_int(1, 0) + b"".join(pb_fixed32(2, t) for t in times))
    record(f"userstats {app} achievements {len(times)}")
    conn.send_frame(2, pack(EMSG_CLIENT_GET_USER_STATS_RESPONSE, h, resp))


# --- content RPCs for platypus-steam-dl -------------------------------------
# Driven by the state file so the Go tests control depot keys, PICS, the
# server list, request codes and CDN tokens without touching Valve:
#   depot_key    hex of the 32-byte depot key the fake CDN encrypted with
#   app_token    PICS access token handed out for the app
#   depot_id     depot the PICS reply describes (default 11)
#   manifest_gid manifest gid the PICS reply and fake CDN agree on
#   oslist       oslist the PICS depot carries ("linux", "windows", ...)
#   appinfo      full text KeyValues served instead of the depot-only buffer
#                (an excerpt of a real app info, e.g. with common/eulas)
#   request_code manifest request code handed out
#   cdn_token    CDN auth token handed out
#   cdn_hosts    ["127.0.0.1:port", ...] served as SteamCache hosts

def content_hdr(steamid, sessionid, jobid, eresult=1):
    return (pb_int(1, steamid) + pb_int(2, sessionid)
            + pb_fixed64(11, jobid) + pb_int(13, eresult))


def product_kv(st):
    depot = str(st.get("depot_id", 11))
    gid = str(st.get("manifest_gid", 999))
    oslist = st.get("oslist", "linux")
    return ('"depots"\n{\n"%s"\n{\n"config"\n{\n"oslist"\t\t"%s"\n}\n'
            '"manifests"\n{\n"public"\n{\n"gid"\t\t"%s"\n}\n}\n}\n}\n'
            % (depot, oslist, gid))


def serve_content(conn, emsg, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    if emsg == EMSG_CLIENT_GET_DEPOT_KEY:
        req = pb_parse(body)
        depot_id = req.get(1, [st.get("depot_id", 11)])[0]
        record(f"depot_key {depot_id}")
        if depot_id in (st.get("depot_key_deny") or []):
            # A refunded or unshared licence: AccessDenied, like the real
            # CM answers for a depot key the account may not fetch.
            conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid, 15),
                                    pb_int(1, 15)))
            return
        key = bytes.fromhex(st.get("depot_key", "00" * 32))
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid),
                                pb_int(1, 1) + pb_int(2, depot_id) + pb_str(3, key)))
    elif emsg == EMSG_CLIENT_PICS_ACCESS_TOKEN:
        req = pb_parse(body)
        apps = req.get(2, [])
        record(f"pics_token {apps}")
        resp = b""
        for app in apps:
            tok = pb_int(1, app) + pb_int(2, st.get("app_token", 12345))
            resp += pb_msg(3, tok)
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid), resp))
    elif emsg == EMSG_CLIENT_PICS_PRODUCT_INFO:
        req = pb_parse(body)
        app = 0
        for raw in req.get(2, []):
            app = pb_parse(raw).get(1, [0])[0]
        record(f"pics_info {app}")
        # Per-app appinfo for shared-depot fixtures ("appinfo_<id>"),
        # else the global one.
        kv = st.get("appinfo_%d" % app) or st.get("appinfo") or product_kv(st)
        entry = (pb_int(1, app) + pb_int(2, 42)
                 + pb_str(5, kv.encode()))
        resp = pb_msg(1, entry)
        # PICS package info ("package_apps": {pkgid: [appids]}) answers
        # the packages = 1 side of the request as packages = 2, so
        # licence-based tests can own DLC.
        for raw in req.get(1, []):
            pkg = pb_parse(raw).get(1, [0])[0]
            record(f"pics_package {pkg}")
            apps = (st.get("package_apps") or {}).get(str(pkg), [])
            kv = '"packageinfo"\n{\n"appids"\n{\n' + "".join(
                '"%d"\t\t"%d"\n' % (i, a) for i, a in enumerate(apps)) + '}\n}\n'
            resp += pb_msg(2, pb_int(1, pkg) + pb_int(2, 42) + pb_str(5, kv.encode()))
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid), resp))
    elif emsg == EMSG_CLIENT_CHECK_BETA_PASSWORD:
        # beta_password_eresult refuses the password (a wrong-password live
        # shape the unlock tests drive); the default 1 accepts it. Each
        # entry carries the branch key (betapassword = 2, hex) the
        # private-beta call needs, like SteamKit's FromHexString decode.
        # drop_beta hangs up instead: a transport failure, never a refusal.
        if st.get("drop_beta"):
            record(f"beta_password app={pb_parse(body).get(1, [0])[0]} dropped")
            conn.close_frame()
            return
        record(f"beta_password app={pb_parse(body).get(1, [0])[0]}")
        resp = pb_int(1, st.get("beta_password_eresult", 1))
        if st.get("beta_password_eresult", 1) == 1:
            # (betapasswords = 4, betaname = 1): every pwdrequired branch
            # in the served appinfo, like a live accept, plus beta_branches
            # (branches live hides from PICS but the password still opens).
            # beta_key_<branch> overrides the announced key; the true key
            # stays the branch hash, so a wrong override fails the 8907
            # check below.
            names = locked_branches(st.get("appinfo") or "")
            for extra in st.get("beta_branches") or []:
                if extra not in names:
                    names.append(extra)
            for name in names:
                key = st.get("beta_key_" + name) or hashlib.sha1(name.encode()).hexdigest()
                resp += pb_msg(4, pb_str(1, name) + pb_str(2, key))
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid), resp))
    elif emsg == EMSG_CLIENT_PICS_PRIVATE_BETA:
        # The private-beta depot section (PICSGetPrivateBeta): text
        # KeyValues with the branch's manifest gids, served from the
        # private_beta state (or private_beta_<app> per-app). The branch
        # key (password_hash = 4) must match the hex key CheckBetaPassword
        # returned; a refused password fails the install distinctly.
        req = pb_parse(body)
        app = req.get(1, [0])[0]
        branch = req.get(3, [b""])[0]
        if isinstance(branch, bytes):
            branch = branch.decode("utf-8", "replace")
        key = req.get(4, [b""])[0]
        record(f"private_beta app={app} branch={branch} key={len(key) if isinstance(key, bytes) else 0}B")
        section = st.get("private_beta_%d" % app, st.get("private_beta", ""))
        # The true key is the branch hash; a beta_key_ override only
        # changes what CheckBetaPassword announced, so it fails here.
        want = hashlib.sha1(branch.encode()).digest()
        if st.get("beta_password_eresult", 1) != 1 or not section or key != want:
            conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid), pb_int(1, 2)))
        else:
            conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid),
                                    pb_int(1, 1) + pb_str(2, section)))


def locked_branches(appinfo):
    """Branch names with pwdrequired in a text KeyValues appinfo buffer."""
    import re
    names = []
    for m in re.finditer(r'"([^"]+)"\s*\{([^{}]*)\}', appinfo):
        if re.search(r'"pwdrequired"\s+"1"', m.group(2)):
            names.append(m.group(1))
    return names


def serve_servers(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    cell = pb_parse(body).get(1, [0])[0]
    record(f"servers cell={cell}")
    resp = b""
    for i, host in enumerate(st.get("cdn_hosts", [])):
        # Plain http like the httptest servers; the Go client only uses
        # https for hosts that say so (https/all/valid).
        entry = (pb_str(1, "SteamCache") + pb_int(3, 0) + pb_int(4, 10)
                 + pb_fixed32(5, 1) + pb_int(6, 1)
                 + pb_str(8, host) + pb_str(9, host) + pb_str(12, "http"))
        resp += pb_msg(1, entry)
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))


def serve_request_code(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    # Branch and hash ride fields 4 and 5; the tests assert the
    # password-derived hash traveled for locked branches.
    branch = req.get(4, [""])[0]
    if isinstance(branch, bytes):
        branch = branch.decode("utf-8", "replace")
    bhash = req.get(5, [""])[0]
    if isinstance(bhash, bytes):
        bhash = bhash.decode("utf-8", "replace")
    record(f"request_code depot={req.get(2, [0])[0]} branch={branch} hash={bhash}")
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid),
                            pb_int(1, st.get("request_code", 4242))))


def serve_cdn_token(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    record("cdn_token")
    exp = int(time.time()) + 3600
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid),
                            pb_str(1, st.get("cdn_token", "cdn-token")) + pb_int(2, exp)))


# --- Cloud saves (Auto-Cloud listings + downloads, issue #6) -----------------
# Knobs are documented at the top (cloud_changelist, cloud_files). Downloads
# ride plain HTTP on localhost; HTTP_PORT is set in main().

HTTP_PORT = 0


def serve_cloud_changelist(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    appid = (pb_parse(body).get(1, [0])[0] if body else 0)
    cl = st.get("cloud_changelist") or {}
    prefixes = list(cl.get("path_prefixes") or [])
    machines = list(cl.get("machine_names") or [])
    files = list(cl.get("files") or [])
    with _cloud_lock:
        uploads = [(n, u) for (a, n), u in _cloud["uploads"].items() if a == appid]
        change = _cloud["change"]
    current = cl.get("current_change_number", 0)
    if change is not None:
        current = max(current, change)
    # Committed uploads read back like the real cloud: same name, new
    # bytes, moved change number.
    for name, u in sorted(uploads):
        idx, rest = None, name
        for i, p in enumerate(prefixes):
            if p and name.startswith(p):
                idx, rest = i, name[len(p):]
                break
        if idx is None:
            if name.startswith("%") and name.find("%", 1) > 0:
                prefixes.append(name[:name.find("%", 1) + 1])
                rest = name[name.find("%", 1) + 1:]
            else:
                prefixes.append("")
            idx = len(prefixes) - 1
        machine = u.get("machine") or "platypus"
        if machine in machines:
            midx = machines.index(machine)
        else:
            machines.append(machine)
            midx = len(machines) - 1
        files = [f for f in files
                 if not (f.get("name", "") == rest and f.get("prefix", 0) == idx)]
        files.append({"name": rest, "sha": u["sha"].hex(), "time": u["time"],
                      "size": u["size"], "persist": 0, "prefix": idx,
                      "machine": midx, "platforms": u.get("platforms")})
    resp = pb_int(1, current)
    for f in files:
        entry = pb_str(1, f.get("name", ""))
        entry += pb_str(2, bytes.fromhex(f.get("sha", "00" * 20)))
        entry += pb_int(3, f.get("time", 0)) + pb_int(4, f.get("size", 0))
        entry += pb_int(5, f.get("persist", 0))
        # platforms_to_sync (field 6): emitted when the entry names one, like
        # the real cloud; a missing field reads as "all platforms".
        if f.get("platforms") is not None:
            entry += pb_int(6, f["platforms"])
        entry += pb_int(7, f.get("prefix", 0)) + pb_int(8, f.get("machine", 0))
        resp += pb_msg(2, entry)
    if cl.get("is_only_delta"):
        resp += pb_int(3, 1)
    for p in prefixes:
        resp += pb_str(4, p)
    for m in machines:
        resp += pb_str(5, m)
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))


def cloud_served_bytes(spec):
    """The (wire, raw) byte pair a cloud_files entry serves: a zip dict
    packs one entry (stored or deflate), anything else serves as is."""
    import io
    import zipfile
    if "zip" in spec:
        buf = io.BytesIO()
        with zipfile.ZipFile(buf, "w", zipfile.ZIP_DEFLATED) as zf:
            for name, b64 in spec["zip"].items():
                zf.writestr(name, base64.b64decode(b64))
        first = next(iter(spec["zip"].values()), "")
        return buf.getvalue(), base64.b64decode(first)
    return base64.b64decode(spec.get("body", "")), base64.b64decode(spec.get("body", ""))


def serve_cloud_download(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    name = ((req.get(2, [b""])[0]).decode() if req.get(2) else "")
    spec = None
    # A file this fake took through an upload reads back the same way: the
    # commit is the current cloud state, newer than the knob entry.
    with _cloud_lock:
        up = _cloud["uploads"].get((req.get(1, [0])[0], name))
    if up is not None:
        spec = {"body": base64.b64encode(up["body"]).decode(),
                "sha": up["sha"].hex(), "time": up["time"]}
    if spec is None:
        spec = (st.get("cloud_files") or {}).get(name)
    if spec is None:
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                content_hdr(steamid, sessionid, jobid, 2), b""))
        return
    wire, raw = cloud_served_bytes(spec)
    sha = bytes.fromhex(spec["sha"]) if spec.get("sha") else hashlib.sha1(raw).digest()
    import urllib.parse
    resp = pb_int(2, spec.get("wire_size", len(wire)))
    resp += pb_int(3, spec.get("raw_size", len(raw)))
    resp += pb_str(4, sha) + pb_int(5, spec.get("time", 0))
    if spec.get("deleted"):
        resp += pb_int(6, 1)
    resp += pb_str(7, "127.0.0.1:%d" % HTTP_PORT)
    resp += pb_str(8, "/cloud/%d/%s" % (req.get(1, [0])[0], urllib.parse.quote(name, safe="")))
    resp += pb_int(9, 0)
    if spec.get("encrypted"):
        resp += pb_int(11, 1)
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))


# --- Cloud uploads (issue #6 P3) ------------------------------------------------
# The batch flow mirrors the real Cloud service: BeginAppUploadBatch names
# the set, ClientBeginFileUpload returns block drops, the client PUTs each
# block to /upload/<appid>/<batch>/<name>, ClientCommitFileUpload moves the
# staged bytes into the store, CompleteAppUploadBatchBlocking ends the batch
# with the client's eresult. Committed files merge into the changelist and
# move its change number, so status -> push -> status reads back the upload.
_cloud = {"batch": 0, "batches": {}, "chunks": {}, "uploads": {}, "change": None}
_cloud_lock = threading.Lock()


def serve_upload_quota(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    q = st.get("cloud_quota") or {}
    resp = (pb_int(1, q.get("used_files", 0)) + pb_int(2, q.get("used_bytes", 0))
            + pb_int(3, q.get("max_files", 0)) + pb_int(4, q.get("max_bytes", 0)))
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))


_BEGIN_WIRE = {1: "appid", 2: "machine", 3: "files", 5: "client_id", 6: "build_id"}
_BEGIN_FILE_WIRE = {1: "appid", 2: "size", 3: "raw", 4: "sha", 5: "time",
                     6: "name", 7: "platforms", 9: "cell", 13: "upload_batch_id"}
_COMMIT_WIRE = {1: "ok", 2: "appid", 3: "sha", 4: "name"}
_COMPLETE_WIRE = {1: "appid", 2: "batch_id", 3: "eresult"}


def _fail_names(st):
    return st.get("cloud_fail_files") or []


def serve_upload_begin(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    record("wire begin " + wire_line(_BEGIN_WIRE, body))
    names = [n.decode() for n in req.get(3, [])]
    machine = (req.get(2, [b""])[0].decode() if req.get(2) else "")
    base = (st.get("cloud_changelist") or {}).get("current_change_number", 0)
    with _cloud_lock:
        _cloud["batch"] += 1
        batch = _cloud["batch"]
        _cloud["batches"][batch] = {"appid": req.get(1, [0])[0],
                                    "names": names, "machine": machine, "done": None}
        cur = _cloud["change"]
        if cur is None or base > cur:
            cur = base
        # Like real Steam (live check, 2026-11-16): beginning a batch
        # moves the app to the next change number and answers it, so a
        # quiet cloud answers listing + 1. The bump knob plays another
        # device's batch landing first.
        cur += 2 if st.get("cloud_begin_bump") else 1
        _cloud["change"] = cur
    record(f"upload begin {batch} {len(names)} files")
    resp = pb_int(1, batch) + pb_int(4, _cloud["change"])
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))
def _upload_spans(st, size, name):
    if st.get("cloud_no_blocks"):
        return []
    meth = 1 if st.get("cloud_bad_method") else (3 if st.get("cloud_post_blocks") else 4)
    explicit = (st.get("cloud_explicit") or {}).get(name)
    if explicit is not None:
        spans = [(0, size, base64.b64decode(explicit.get("body", "")),
                  3 if explicit.get("post", True) else 4)]
    elif st.get("cloud_split_blocks") and size > 1:
        half = size // 2
        spans = [(0, half, None, meth), (half, size - half, None, meth)]
    else:
        spans = [(0, size, None, meth)]
    if st.get("cloud_shuffle_blocks"):
        spans = spans[::-1]
    return spans


def serve_upload_begin_file(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    record("wire begin-file " + wire_line(_BEGIN_FILE_WIRE, body))
    size = req.get(2, [0])[0]
    name = (req.get(6, [b""])[0].decode() if req.get(6) else "")
    batch = req.get(13, [0])[0]
    eres = st.get("cloud_upload_eresult", 1)
    if name in _fail_names(st):
        eres = 2
    if name in (st.get("cloud_encrypt_files") or []):
        resp = pb_int(1, 1)
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                content_hdr(steamid, sessionid, jobid), resp))
        return
    with _cloud_lock:
        known = batch in _cloud["batches"]
        in_batch = known and name in _cloud["batches"][batch]["names"]
        if known and in_batch and eres == 1:
            _cloud["chunks"][(batch, name)] = {
                "appid": req.get(1, [0])[0], "size": size,
                "sha": req.get(4, [b""])[0], "time": req.get(5, [0])[0],
                "platforms": req.get(7, [0])[0],
                "spans": [(off, ln) for off, ln, _body, _m in _upload_spans(st, size, name)],
                "parts": {}}
    if not known or not in_batch:
        eres = 2
    if eres != 1:
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                content_hdr(steamid, sessionid, jobid, eres), b""))
        return
    import urllib.parse
    host = "127.0.0.1:%d" % HTTP_PORT
    resp = b""
    for offset, length, explicit, method in _upload_spans(st, size, name):
        # Each block gets its own path carrying its offset: identity, not
        # arrival order, decides where the bytes land.
        path = "/upload/%d/%d/%d/%s" % (req.get(1, [0])[0], batch, offset,
                                        urllib.parse.quote(name, safe=""))
        blk = pb_str(1, host) + pb_str(2, path) + pb_int(3, 0) + pb_int(4, method)
        blk += pb_msg(5, pb_str(1, "X-Fake-Block") + pb_str(2, "1"))
        blk += pb_int(6, offset) + pb_int(7, length)
        if explicit is not None:
            blk += pb_str(8, explicit)
        resp += pb_msg(2, blk)
    spans = _upload_spans(st, size, name)
    record(f"upload begin-file {batch} {name} {len(spans)} blocks platforms={req.get(7, [0])[0]}")
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), resp))


def serve_upload_commit(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    record("wire commit " + wire_line(_COMMIT_WIRE, body))
    ok = req.get(1, [0])[0]
    name = (req.get(4, [b""])[0].decode() if req.get(4) else "")
    batch = None
    with _cloud_lock:
        for (b, n), staged in list(_cloud["chunks"].items()):
            if n == name and staged["appid"] == req.get(2, [0])[0]:
                batch = b
                break
    eres = st.get("cloud_upload_eresult", 1)
    if name in _fail_names(st):
        eres = 2
    if not ok or batch is None:
        eres = 2
    assembled = None
    if eres == 1:
        with _cloud_lock:
            staged = _cloud["chunks"].pop((batch, name))
            machine = _cloud["batches"][batch]["machine"]
        # Offset-honoring assembly: parts land at the spans' offsets, so
        # shuffled blocks still reassemble. Anything off fails the commit.
        if len(staged["parts"]) == len(staged["spans"]):
            buf = bytearray(staged["size"])
            ok_parts = True
            for off, ln in staged["spans"]:
                part = staged["parts"].get(off)
                if part is None or len(part) != ln or off + ln > staged["size"]:
                    ok_parts = False
                    break
                buf[off:off + ln] = part
            if ok_parts and hashlib.sha1(bytes(buf)).digest() == staged["sha"]:
                assembled = bytes(buf)
        if assembled is None:
            eres = 2
            record(f"upload commit {name} sha mismatch")
    if eres == 1:
        with _cloud_lock:
            base = (st.get("cloud_changelist") or {}).get("current_change_number", 0)
            if _cloud["change"] is None:
                _cloud["change"] = base
            _cloud["uploads"][(staged["appid"], name)] = {
                "body": assembled, "sha": staged["sha"],
                "time": staged["time"], "size": staged["size"],
                "machine": machine, "platforms": staged.get("platforms")}
        record(f"upload commit {name} ok=1 {len(assembled)} bytes")
    else:
        record(f"upload commit {name} ok={ok} refused")
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid, eres),
                            pb_int(1, 1) if eres == 1 else b""))


def serve_upload_complete(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    record("wire complete " + wire_line(_COMPLETE_WIRE, body))
    batch = req.get(2, [0])[0]
    eres = req.get(3, [0])[0]
    with _cloud_lock:
        if batch in _cloud["batches"]:
            _cloud["batches"][batch]["done"] = eres
    record(f"upload complete {batch} {eres}")
    reply_eres = st.get("cloud_complete_eresult", 1)
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid, reply_eres), b""))


def serve_upload_conflict(conn, hdr, body, st, steamid, sessionid):
    jobid = hdr.get(10, [0])[0]
    req = pb_parse(body)
    record("conflict-resolution %d %d" % (req.get(1, [0])[0], req.get(2, [0])[0]))
    conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                            content_hdr(steamid, sessionid, jobid), b""))


class CloudHTTPHandler(BaseHTTPRequestHandler):
    """Plain-HTTP file drops for ClientFileDownload tests (no Valve
    network): GET /cloud/<appid>/<quoted cloud name> serves the entry's
    wire bytes from a fresh state read, like the real CDN would."""

    def do_GET(self):
        import urllib.parse
        parts = urllib.parse.urlparse(self.path).path.split("/", 3)
        if len(parts) != 4 or parts[1] != "cloud":
            self.send_response(404)
            self.end_headers()
            return
        name = urllib.parse.unquote(parts[3])
        spec = None
        # A commit is the current cloud state, newer than the knob entry.
        try:
            upappid = int(parts[2])
        except ValueError:
            upappid = None
        if upappid is not None:
            up = _cloud["uploads"].get((upappid, name))
            if up is not None:
                spec = {"body": base64.b64encode(up["body"]).decode()}
        if spec is None:
            spec = (load().get("cloud_files") or {}).get(name)
        if spec is None:
            self.send_response(404)
            self.end_headers()
            return
        if spec.get("deleted"):
            self.send_response(404)
            self.end_headers()
            return
        wire, _raw = cloud_served_bytes(spec)
        record("http %s" % name)
        self.send_response(200)
        self.send_header("Content-Length", str(len(wire)))
        self.end_headers()
        self.wfile.write(wire)

    def do_PUT(self):
        self._upload()

    def do_POST(self):
        self._upload()

    def _upload(self):
        """One upload block: stored under its path offset, so identity —
        not arrival order — decides where the bytes land. A hang knob reads
        the bytes first so the batch stays begun for the cancel test."""
        import urllib.parse
        parts = urllib.parse.urlparse(self.path).path.split("/", 5)
        if len(parts) != 6 or parts[1] != "upload":
            self.send_response(404)
            self.end_headers()
            return
        try:
            appid, batch, offset = int(parts[2]), int(parts[3]), int(parts[4])
        except ValueError:
            self.send_response(404)
            self.end_headers()
            return
        name = urllib.parse.unquote(parts[5])
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length) if length else b""
        with _cloud_lock:
            staged = _cloud["chunks"].get((batch, name))
            same_app = staged is not None and staged["appid"] == appid
        if not same_app:
            self.send_response(404)
            self.end_headers()
            return
        record("upload http %s %s %d" % (self.command, name, len(body)))
        if load().get("cloud_upload_hang"):
            time.sleep(60)
            self.send_response(200)
            self.end_headers()
            return
        with _cloud_lock:
            staged = _cloud["chunks"].get((batch, name))
            if staged is not None:
                staged["parts"][offset] = body
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def log_message(self, *args):
        pass


# --- authentication RPCs (phase 6 logins, no Valve network) ---------------
# Field numbers from steammessages_auth.steamclient.proto; EResults ride
# the service header like other calls. The RSA key below is a throwaway
# test key (1024-bit, generated for the fake) — it encrypts only the fake
# password "correct-horse" in tests, never real credentials.
FAKE_RSA_MOD = "f0e1a9393eb896a7728260802446c4e5a8b258050758d97cdd3c025bb37ce7f160ec3a4ba173498d368d5f38c70d1b9e157ae237a8e02bcf54c40cce73e0f78b43ea92fdc6e4b712168e44a56574715cc390dd609ebfb6ae86977332f62a23460bcf4e805cf9bf404e8069a2579ff96a83810074d095dbf8efc5f9395c2a8321"
FAKE_RSA_EXP = "10001"
FAKE_RSA_D = "17b31c2335b8939a05fa2de46dad48d20fb6fc5202570b8eed756a05fbb2de5f315a0ca24450437c5151b77cae98eb8eb22e54f384fa09556cd19d30f2d5bf851359e987b283c121288a8ef6f4377b903bc71949e543776640505f76710e786447a7c1c2ca1725d2090d00a6455e905cf298a1895aab2e535fda216b35fb0001"
FAKE_RSA_TS = 1234567890

# client_id -> {"request_id", "account", "steamid", "mode", "guard",
# "expected_code", "challenge", "refreshed", "approved", "polls"}
_auth_sessions = {}
_auth_next_id = [1000]


def pb_float(field, value):
    return pb_fixed32(field, struct.unpack("<I", struct.pack("<f", value))[0])


def rsa_decrypt_password(b64):
    """PKCS#1 v1.5-decrypt the base64 password with the fake key, or None."""
    try:
        blob = base64.b64decode(b64)
        n = int(FAKE_RSA_MOD, 16)
        d = int(FAKE_RSA_D, 16)
        m = pow(int.from_bytes(blob, "big"), d, n)
        k = (n.bit_length() + 7) // 8
        em = m.to_bytes(k, "big")
        # 0x00 0x02 PS 0x00 M
        if len(em) < 11 or em[0] != 0 or em[1] != 2:
            return None
        sep = em.find(b"\x00", 2)
        if sep < 10:
            return None
        return em[sep + 1:].decode()
    except Exception:
        return None


def serve_auth(conn, target, hdr, body, steamid):
    """One Authentication.* service call on an unauthenticated connection
    (logins run before any token exists). Returns True when handled."""
    jobid = hdr.get(10, [0])[0]
    st = load()  # re-read so tests can approve mid-login
    if target == "Authentication.GetPasswordRSAPublicKey#1":
        record("service Authentication.GetPasswordRSAPublicKey#1")
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                content_hdr(steamid, 0, jobid),
                                pb_str(1, FAKE_RSA_MOD) + pb_str(2, FAKE_RSA_EXP) + pb_int(3, FAKE_RSA_TS)))
        return True
    if target == "Authentication.BeginAuthSessionViaQR#1":
        record("service Authentication.BeginAuthSessionViaQR#1")
        eres = st.get("auth_begin_eresult", 1)
        if eres != 1:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, eres), b""))
            return True
        _auth_next_id[0] += 1
        cid = _auth_next_id[0]
        req_id = os.urandom(16)
        challenge = st.get("auth_challenge_url", "https://steamcommunity.com/qr/TESTCHALLENGE")
        _auth_sessions[cid] = {"request_id": req_id, "account": st.get("auth_account", "PlatypusUser"),
                               "steamid": st.get("auth_steamid", 76561198), "mode": "qr",
                               "challenge": challenge, "refreshed": False, "polls": 0}
        allow = pb_int(1, 4) + pb_str(2, "Confirm via the Steam mobile app")
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid),
                                pb_int(1, cid) + pb_str(2, challenge) + pb_str(3, req_id)
                                + pb_float(4, 0.2) + pb_msg(5, allow) + pb_int(6, 2)))
        return True
    if target == "Authentication.BeginAuthSessionViaCredentials#1":
        record("service Authentication.BeginAuthSessionViaCredentials#1")
        eres = st.get("auth_begin_eresult", 1)
        if eres != 1:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, eres), b""))
            return True
        req = pb_parse(body)
        account = (req.get(2, [b""])[0]).decode() if req.get(2) else ""
        enc = (req.get(3, [b""])[0]).decode() if req.get(3) else ""
        want = st.get("auth_password", "correct-horse")
        if rsa_decrypt_password(enc) != want:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, 5), b""))
            return True
        if not account:
            account = st.get("auth_account", "PlatypusUser")
        guard = st.get("auth_guard", "none")
        exp_code = st.get("auth_expected_code", "123456" if guard == "totp" else "ABC12")
        _auth_next_id[0] += 1
        cid = _auth_next_id[0]
        req_id = os.urandom(16)
        sess = {"request_id": req_id, "account": account,
                "steamid": st.get("auth_steamid", 76561198), "mode": "password",
                "guard": guard, "expected_code": exp_code, "approved": guard == "none", "polls": 0}
        _auth_sessions[cid] = sess
        resp = pb_int(1, cid) + pb_str(2, req_id) + pb_float(3, 0.2)
        if guard == "email":
            resp += pb_msg(4, pb_int(1, 2) + pb_str(2, "Enter the code emailed to you"))
        elif guard == "totp":
            resp += pb_msg(4, pb_int(1, 3) + pb_str(2, "Enter the authenticator code"))
        elif guard == "confirm":
            resp += pb_msg(4, pb_int(1, 4) + pb_str(2, "Confirm in the mobile app"))
        resp += pb_int(5, sess["steamid"])
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid), resp))
        return True
    if target == "Authentication.PollAuthSessionStatus#1":
        req = pb_parse(body)
        cid = req.get(1, [0])[0]
        record(f"service Authentication.PollAuthSessionStatus#1 client={cid}")
        eres = st.get("auth_poll_eresult", 1)
        if eres != 1:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, eres), b""))
            return True
        sess = _auth_sessions.get(cid)
        if sess is None:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, 27), b""))
            return True
        sess["polls"] += 1
        if sess["mode"] == "qr" and st.get("auth_qr_refresh") and not sess["refreshed"]:
            sess["refreshed"] = True
            new_url = st.get("auth_new_challenge_url", "https://steamcommunity.com/qr/REFRESHED")
            sess["challenge"] = new_url
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid), pb_str(2, new_url)))
            return True
        approved = False
        if sess["mode"] == "qr" and st.get("auth_qr_approved"):
            approved = True
        elif sess["mode"] == "password" and (sess.get("approved") or
                                             (sess.get("guard") == "confirm" and st.get("auth_mobile_approved"))):
            approved = True
        if not approved:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid), b""))
            return True
        ttl = st.get("token_ttl", 3600)
        token = make_token(sess["steamid"], ttl)
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid),
                                pb_str(3, token) + pb_str(6, sess["account"])))
        return True
    if target == "Authentication.UpdateAuthSessionWithSteamGuardCode#1":
        req = pb_parse(body)
        cid = req.get(1, [0])[0]
        code = (req.get(3, [b""])[0]).decode() if req.get(3) else ""
        record(f"service Authentication.UpdateAuthSessionWithSteamGuardCode#1 client={cid}")
        sess = _auth_sessions.get(cid)
        if sess is None:
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, 27), b""))
            return True
        if code != sess.get("expected_code"):
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid, 5), b""))
            return True
        sess["approved"] = True
        conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE, content_hdr(steamid, 0, jobid), b""))
        return True
    return False


def serve_conn(sock):
    try:
        serve(Conn(sock))
    except OSError:
        pass
    finally:
        try:
            sock.close()
        except OSError:
            pass


def main():
    global HTTP_PORT
    httpd = HTTPServer(("127.0.0.1", 0), CloudHTTPHandler)
    HTTP_PORT = httpd.server_address[1]
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    srv = socket.socket()
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", 0))
    srv.listen(4)
    print(srv.getsockname()[1], flush=True)
    print("http %d" % HTTP_PORT, flush=True)
    while True:
        if not os.path.exists(STATE):
            return  # state file removed: test is done
        try:
            srv.settimeout(2)
            sock, _addr = srv.accept()
        except socket.timeout:
            continue
        srv.settimeout(None)
        # Connections run in parallel: a persistent social session holds
        # its socket while one-shot calls (auth, stats, downloads) come
        # and go on theirs, the way the real CM serves them.
        threading.Thread(target=serve_conn, args=(sock,), daemon=True).start()


if __name__ == "__main__":
    main()