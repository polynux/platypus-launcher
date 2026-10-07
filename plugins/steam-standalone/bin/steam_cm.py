"""Steam CM session for hydra-steam: exchange the stored refresh token for an
access token, over an authenticated CM session.

Why: since 2025-04-30, SteamClient refresh tokens fail with AccessDenied when
GenerateAccessTokenForApp is called over plain REST, and the tokens are
rejected by the Web API directly — they only work from an authenticated
CM logon session (node-steam-session's README; SteamKit2 does the same
SteamUser.LogOn dance). So the plugin logs on to a CM server itself, using
only the Python stdlib: a TLS WebSocket to wss://<host>/cmsocket/ with
handshake and framing written by hand, and protobuf messages assembled
field-wise. Message layouts follow SteamKit2's CMsgProtoBufHeader /
CMsgClientLogon and node-steam-session's WebSocketCMTransport:

  - every message: <uint32 EMsg|0x80000000 (LE)><uint32 header length (LE)>
    <CMsgProtoBufHeader><body bytes>
  - CMsgClientLogon puts the refresh token in access_token (field 108) and
    the account's steamid (the JWT's "sub") in the header steamid field,
    matching what SteamKit2's SteamUser.LogOn sends for token logons
  - the CM batches some messages into CMsgMulti (gzip) envelopes; the logon
    response, heartbeats and service responses are what we care about
  - Authentication.GenerateAccessTokenForApp#1 goes through
    ServiceMethodCallFromClient (151) with target_job_name in the header;
    the reply (147) is matched by jobid_target

Renewal is deliberately NOT requested (renewal_type None): renewing would
invalidate the refresh token DepotDownloader keeps in its login key, and the
plugin doesn't persist tokens anywhere else.

Test hook: when HYDRA_STEAM_CM_SERVER is set ("host:port"), the module uses
ws:// to that server instead of fetching Valve's CM list and using wss:// —
the fake CM in testdata/fake_cm.py implements the same protocol so tests
drive this code with no network.
"""

import base64
import gzip
import hashlib
import json
import os
import secrets
import socket
import ssl
import struct
import time

# EMsg values (steam/enums_clientserver.proto).
EMSG_MULTI = 1
EMSG_SERVICE_METHOD_RESPONSE = 147
EMSG_SERVICE_METHOD_CALL_FROM_CLIENT = 151
EMSG_CLIENT_HEARTBEAT = 703
EMSG_CLIENT_LOGOFF = 706
EMSG_CLIENT_LOGON_RESPONSE = 751
EMSG_CLIENT_LOGGED_OFF = 757
EMSG_CLIENT_LOGON = 5514
EMSG_CLIENT_HELLO = 9805
PROTO_MASK = 0x80000000

# Logon constants, matching SteamKit2's SteamUser (PROTOCOL_VERSION 65580)
# and node-steam-user (client_package_version 1771, os type LinuxUnknown).
PROTOCOL_VERSION = 65580
CLIENT_PACKAGE_VERSION = 1771
CLIENT_OS_LINUX = (-203) & 0xFFFFFFFF   # EOSType.LinuxUnknown as uint32
machine_name = "Hydra"

# The obfuscated private ip doubles as the login id: a fixed value keeps a
# logon from kicking an existing session — same as hydra-steam's -loginid.
LOGIN_ID = 2718281

# EResult names worth putting in front of users (steam/steammessages...,
# SteamKit2's EResult); anything unmapped shows the number.
ERESULT_NAMES = {
    1: "OK", 2: "Fail", 3: "NoConnection", 5: "InvalidPassword", 8: "InvalidParam",
    15: "AccessDenied", 20: "ServiceUnavailable", 26: "Revoked", 27: "Expired",
    48: "TryAnotherCM", 84: "RateLimitExceeded", 85: "AccountLoginDeniedNeedTwoFactor",
    87: "AccountLoginDeniedThrottle",
}

# EResults that mean "Steam says this login/token is no good": the caller
# (hydra-steam) maps these to its auth-required error, everything else is a
# plain failure with the real message.
ERESULTS_TOKEN_REJECTED = frozenset((5, 26, 27))


class CMError(Exception):
    """A CM exchange that failed. `eresult` is the server's EResult (or None
    for connection-level failures); `auth` marks token-rejected results."""

    def __init__(self, message, eresult=None, auth=False):
        super().__init__(message)
        self.eresult = eresult
        self.auth = auth


def eresult_name(eresult):
    return ERESULT_NAMES.get(eresult, f"EResult {eresult}")


# --- minimal protobuf --------------------------------------------------------
# Steam's messages only use varints, fixed64/32 and length-delimited fields;
# both directions stay in that subset.

def _varint(value):
    out = bytearray()
    value &= (1 << 64) - 1
    while True:
        byte = value & 0x7F
        value >>= 7
        if value:
            out.append(byte | 0x80)
        else:
            out.append(byte)
            return bytes(out)


def pb_int(field, value):
    """A varint (int32/int64/uint32/bool/enum) field."""
    return _varint((field << 3)) + _varint(value)


def pb_fixed64(field, value):
    return _varint((field << 3) | 1) + struct.pack("<Q", value)


def pb_fixed32(field, value):
    return _varint((field << 3) | 5) + struct.pack("<I", value)


def pb_str(field, value):
    if isinstance(value, str):
        value = value.encode()
    return _varint((field << 3) | 2) + _varint(len(value)) + value


def pb_msg(field, payload):
    return pb_str(field, payload)


def pb_parse(data):
    """field number -> list of values (int for varint/fixed, bytes otherwise).
    Parsing stops at the first unknown wire type; Steam's messages don't use
    any beyond these four, and every field we read is present since we sent
    the request."""
    out = {}
    i = 0
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


def jwt_payload(token):
    """The decoded payload segment of a Steam JWT ({" sub": ...} with spaces —
    standard base64url padding rules still apply)."""
    seg = token.split(".")[1]
    seg += "=" * (-len(seg) % 4)
    return json.loads(base64.urlsafe_b64decode(seg))


# --- websocket by hand -------------------------------------------------------
# One binary frame per Steam message; Steam pings are answered inline and
# close frames end the session.

class WebSocket:
    def __init__(self, host, port, tls, timeout=15):
        self.sock = socket.create_connection((host, port), timeout=timeout)
        try:
            if tls:
                self.sock = ssl.create_default_context().wrap_socket(
                    self.sock, server_hostname=host)
            self.sock.settimeout(timeout)  # the message loop applies its own deadlines
            key = base64.b64encode(secrets.token_bytes(16)).decode()
            self.sock.sendall(
                "GET /cmsocket/ HTTP/1.1\r\n"
                f"Host: {host}\r\n"
                "Upgrade: websocket\r\nConnection: Upgrade\r\n"
                f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n".encode())
            buf = b""
            while b"\r\n\r\n" not in buf:
                chunk = self.sock.recv(4096)
                if not chunk:
                    raise OSError("handshake: connection closed")
                buf += chunk
            head, _, rest = buf.partition(b"\r\n\r\n")
            status = head.split(b"\r\n")[0].decode("ascii", "replace")
            if " 101 " not in status + " ":
                raise OSError(f"handshake refused: {status}")
            self.buf = rest
            self.frag = b""
        except BaseException:
            self.sock.close()
            raise

    def _read(self, n):
        while len(self.buf) < n:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise OSError("connection closed")
            self.buf += chunk
        out, self.buf = self.buf[:n], self.buf[n:]
        return out

    def _frame(self, opcode, payload):
        mask = secrets.token_bytes(4)
        n = len(payload)
        head = bytes((0x80 | opcode,))
        if n < 126:
            head += bytes((0x80 | n,))
        elif n < 65536:
            head += bytes((0x80 | 126,)) + struct.pack(">H", n)
        else:
            head += bytes((0x80 | 127,)) + struct.pack(">Q", n)
        return head + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(payload))

    def send_binary(self, data):
        self.sock.sendall(self._frame(2, data))

    def close(self):
        try:
            self.sock.sendall(self._frame(8, struct.pack(">H", 1000)))
        except OSError:
            pass
        try:
            self.sock.close()
        except OSError:
            pass

    def recv_message(self):
        """One complete binary message, de-fragmented; answers pings; None on
        close. Text frames ( Valve only sends them on errors) are skipped."""
        while True:
            first = self._read(2)
            fin, op = first[0] & 0x80, first[0] & 0x0F
            n = first[1] & 0x7F
            if n == 126:
                n = struct.unpack(">H", self._read(2))[0]
            elif n == 127:
                n = struct.unpack(">Q", self._read(8))[0]
            payload = self._read(n)
            if op == 8:
                self.close()
                return None
            if op == 9:  # ping -> pong
                self.sock.sendall(self._frame(10, payload))
                continue
            if op in (1, 10):  # text, pong: nothing Steam sends here concerns us
                continue
            self.frag += payload
            if not fin:
                continue  # continuation frame follows
            out, self.frag = self.frag, b""
            return out


# --- CM message framing -------------------------------------------------------

def pack_emsg(emsg, header, body):
    return struct.pack("<II", emsg | PROTO_MASK, len(header)) + header + body


def unpack_emsg(data):
    emsg, blen = struct.unpack_from("<II", data, 0)
    return emsg & ~PROTO_MASK, pb_parse(data[8:8 + blen]), data[8 + blen:]


def header(steamid=0, sessionid=0, jobid_source=0, target_job_name=""):
    h = b""
    if steamid:
        h += pb_fixed64(1, steamid)                 # steamid
    if sessionid:
        h += pb_int(2, sessionid)                   # client_sessionid
    if jobid_source:
        h += pb_fixed64(10, jobid_source)           # jobid_source
    if target_job_name:
        h += pb_str(12, target_job_name)            # target_job_name
    return h


def machine_id():
    """A binary-KeyValues MessageObject with SHA-1 hashes like SteamKit's
    machine_id (it must be well-formed but the values just identify this
    machine; random once per session is what node-steam-user does).
    Layout: type 0 "MessageObject", then type 1 "BB3"/"FF2"/"3B3" strings,
    each null-terminated, closed by two end markers (0x08 0x08)."""
    out = b"\x00MessageObject\x00"
    for name in ("BB3", "FF2", "3B3"):
        digest = hashlib.sha1(secrets.token_bytes(16)).hexdigest()
        out += b"\x01" + name.encode() + b"\x00" + digest.encode() + b"\x00"
    return out + b"\x08\x08"


def cm_list():
    """WebSocket CM endpoints from ISteamDirectory/GetCMListForConnect, best
    load first (the API already sorts most-used-last by wtd_load)."""
    import urllib.request
    hook = os.environ.get("HYDRA_STEAM_CM_SERVER")
    if hook:
        return [hook], False  # fake CM: plain ws://, no TLS
    url = ("https://api.steampowered.com/ISteamDirectory/GetCMListForConnect/v1/"
           "?cellid=0&maxcount=20&cmtype=websockets")
    with urllib.request.urlopen(url, timeout=30) as r:
        data = json.loads(r.read().decode())
    if not data.get("response", {}).get("success"):
        raise CMError(f"Steam's CM list request failed: {data.get('response', {}).get('message') or 'unknown error'}")
    servers = [s["endpoint"] for s in data["response"].get("serverlist", [])
               if s.get("type") == "websockets" and s.get("realm") == "steamglobal"]
    if not servers:
        raise CMError("Steam returned no WebSocket CM servers to connect to")
    return servers, True


class CMSession:
    """A logon session to one CM: connects, sends ClientHello+ClientLogon,
    then serves call() requests until logoff(). Responses can arrive wrapped
    in Multi (gzip) envelopes; incoming heartbeats are answered inline."""

    def __init__(self, steamid, refresh_token):
        self.steamid = steamid
        self.refresh_token = refresh_token
        self.sessionid = 0
        self.jobid = secrets.randbelow(1 << 30) + 1
        self.last_heartbeat = 0
        self._connect()

    def _connect(self):
        errors = []
        for endpoint, tls in self._candidates():
            host, _, port = endpoint.rpartition(":")
            ws = None
            try:
                ws = WebSocket(host, int(port), tls)
                self.ws = ws
                self._logon()
                return
            except CMError:
                raise  # the CM answered: its result is the answer, don't shop around
            except (OSError, ValueError) as e:
                errors.append(f"{endpoint}: {e}")
                if ws:
                    ws.close()
        raise CMError("couldn't reach any Steam CM server (" + "; ".join(errors[:3]) + ")")

    def _candidates(self):
        servers, tls = cm_list()
        return [(s, tls) for s in servers]

    def _logon(self):
        self.ws.send_binary(pack_emsg(EMSG_CLIENT_HELLO, header(),
                                      pb_int(1, PROTOCOL_VERSION)))
        self.jobid += 1
        body = b"".join([
            pb_int(1, PROTOCOL_VERSION),                    # protocol_version
            pb_msg(11, pb_fixed32(1, LOGIN_ID)),            # obfuscated_private_ip {v4: loginid}
            pb_int(2, LOGIN_ID),                            # deprecated_obfustucated_private_ip
            pb_int(5, CLIENT_PACKAGE_VERSION),              # client_package_version
            pb_str(6, "english"),                           # client_language
            pb_int(7, CLIENT_OS_LINUX),                     # client_os_type
            pb_int(8, 1),                                   # should_remember_password
            pb_str(30, machine_id()),                       # machine_id
            pb_int(33, 2),                                  # chat_mode
            pb_str(96, machine_name),                       # machine_name
            pb_int(102, 1),                                 # supports_rate_limit_response
            pb_str(108, self.refresh_token),                # access_token = the refresh token
        ])
        self.ws.send_binary(pack_emsg(EMSG_CLIENT_LOGON,
                                      header(steamid=self.steamid), body))
        for emsg, h, b in self._messages(deadline=time.time() + 30):
            if emsg == EMSG_CLIENT_LOGON_RESPONSE:
                eresult = pb_parse(b).get(1, [2])[0]
                if eresult == 2 and h.get(13):
                    eresult = h[13][0]  # some CMs only set the header eresult
                self.sessionid = h.get(2, [0])[0]
                if eresult != 1:
                    self.ws.close()
                    auth = eresult in ERESULTS_TOKEN_REJECTED
                    raise CMError(
                        f"Steam rejected the login with {eresult_name(eresult)}"
                        + ("" if auth else " while logging on to its CM server"),
                        eresult=eresult, auth=auth)
                return
            self._side_message(emsg)
        raise CMError("Steam didn't answer the CM logon within 30 seconds")

    def _side_message(self, emsg):
        """Messages the CM sends unasked; only heartbeats need an answer."""
        if emsg == EMSG_CLIENT_HEARTBEAT:
            self.ws.send_binary(pack_emsg(EMSG_CLIENT_HEARTBEAT,
                                          header(steamid=self.steamid,
                                                 sessionid=self.sessionid), b""))

    def _messages(self, deadline):
        """Yields (emsg, header-dict, body) until the deadline, unwrapping
        Multi envelopes (gzip when size_unzipped is set)."""
        while True:
            if time.time() > deadline:
                raise CMError("timed out waiting for Steam's CM response")
            try:
                data = self.ws.recv_message()
            except OSError as e:
                raise CMError(f"Steam CM connection dropped: {e}")
            if data is None:
                raise CMError("Steam CM closed the connection before answering")
            pending = [data]
            while pending:
                emsg, h, body = unpack_emsg(pending.pop(0))
                if emsg == EMSG_MULTI:
                    multi = pb_parse(body)
                    payload = multi.get(2, [b""])[0]
                    if multi.get(1):
                        payload = gzip.decompress(payload)
                    pos = 0
                    while pos + 4 <= len(payload):
                        length = struct.unpack_from("<I", payload, pos)[0]
                        pending.append(payload[pos + 4:pos + 4 + length])
                        pos += 4 + length
                    continue
                yield emsg, h, body

    def call(self, target_job_name, body, timeout=30):
        """One ServiceMethodCallFromClient request; returns the response body.
        The reply (147, or the CM's equivalent) is matched by jobid_target —
        node-steam-session does the same, since the CM picks its own EMsg."""
        self.jobid += 1
        jobid = self.jobid
        hdr = header(steamid=self.steamid, sessionid=self.sessionid,
                     jobid_source=jobid, target_job_name=target_job_name)
        self.ws.send_binary(pack_emsg(EMSG_SERVICE_METHOD_CALL_FROM_CLIENT, hdr, body))
        for emsg, h, b in self._messages(deadline=time.time() + timeout):
            if emsg == EMSG_CLIENT_HEARTBEAT:
                self._side_message(emsg)
                continue
            if h.get(11, [None])[0] == jobid:  # jobid_target
                eresult = h.get(13, [2])[0]
                if eresult != 1:
                    error = h.get(14, [b""])[0]
                    auth = eresult in ERESULTS_TOKEN_REJECTED
                    raise CMError(
                        f"Steam rejected the request with {eresult_name(eresult)}"
                        + (f": {error.decode(errors='replace')}" if error else ""),
                        eresult=eresult, auth=auth)
                return b
            self._side_message(emsg)
        raise CMError(f"Steam didn't answer {target_job_name} within {timeout} seconds")

    def logoff(self):
        """ClientLogOff and a clean close; Steam answers with ClientLoggedOff
        and closes on its own, and a dropped connection here is fine."""
        try:
            self.ws.send_binary(pack_emsg(EMSG_CLIENT_LOGOFF,
                                          header(steamid=self.steamid,
                                                 sessionid=self.sessionid), b""))
            self.ws.close()
        except OSError:
            pass


def exchange(refresh_token, timeout=30):
    """The refresh token -> access token, via an authenticated CM session.
    Returns the access token string (a JWT whose payload carries exp).
    Raises CMError; `auth` is set only when Steam says the token itself is
    bad — hydra-steam maps that to its auth-required error."""
    try:
        steamid = int(jwt_payload(refresh_token)["sub"])
    except (KeyError, ValueError) as e:
        raise CMError(f"the stored login key isn't a usable Steam token ({e})", auth=True)
    session = CMSession(steamid, refresh_token)
    try:
        body = session.call("Authentication.GenerateAccessTokenForApp#1",
                            pb_str(1, session.refresh_token)    # refresh_token
                            + pb_fixed64(2, steamid))           # steamid
        response = pb_parse(body)
        if not response.get(1):
            raise CMError("Steam's token response carried no access token")
        return response[1][0].decode()
    finally:
        session.logoff()