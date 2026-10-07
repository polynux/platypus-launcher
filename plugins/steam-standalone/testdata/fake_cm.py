#!/usr/bin/env python3
"""A fake Steam CM server for the plugin's tests. Speaks the same wire
protocol as the real thing — WebSocket framing, the
<emsg><hdrlen><protobuf header><body> message format, Multi (gzip)
envelopes, heartbeats — over plain ws:// on localhost, and hands out an
access-token JWT for Authentication.GenerateAccessTokenForApp#1.

State lives in $FAKE_CM_STATE (JSON, re-read for every connection):
  logon_eresult     EResult for the logon response (default 1 = OK)
  service_eresult   EResult in the GenerateAccessTokenForApp response header
                    (default 1 = OK)
  wrap_logon_multi  when true, the logon response arrives inside a Multi
                    (gzip) envelope, like the real CM sometimes sends
  token_ttl         access-token lifetime in seconds (default 3600)
  persona           persona name Player.GetPlayerLinkDetails answers with
  send_heartbeat    when true, one ClientHeartBeat arrives between the logon
                    response and the first service response
Every connection appends a line to $FAKE_CM_STATE.log: "connect",
"logon <steamid>", "heartbeat_reply", "service <jobid>", "logoff" — the Go
tests read it to assert what the plugin actually did on the wire.

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
import time

STATE = os.environ["FAKE_CM_STATE"]
GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

PROTO_MASK = 0x80000000
EMSG_MULTI = 1
EMSG_SERVICE_METHOD_RESPONSE = 147
EMSG_CLIENT_HEARTBEAT = 703
EMSG_CLIENT_LOGOFF = 706
EMSG_CLIENT_LOGON_RESPONSE = 751
EMSG_CLIENT_LOGGED_OFF = 757
EMSG_CLIENT_GET_DEPOT_KEY = 5438
EMSG_CLIENT_CHECK_BETA_PASSWORD = 5450
EMSG_CLIENT_LOGON = 5514
EMSG_CLIENT_PICS_PRODUCT_INFO = 8903
EMSG_CLIENT_PICS_ACCESS_TOKEN = 8905
EMSG_CLIENT_HELLO = 9805


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
        if emsg == EMSG_CLIENT_LOGON:
            serve_logged_in(conn, hdr, body, st)
            return
        record(f"unexpected {emsg}")


def serve_logged_in(conn, hdr, body, st):
    steamid = hdr.get(1, [0])[0]
    record(f"logon {steamid}")
    steamid_req = steamid  # default until the service call names one
    eres = st.get("logon_eresult", 1)
    h = pb_int(1, steamid) + pb_int(2, 4711)
    resp = pb_int(1, eres) + pb_int(3, 5)  # eresult, heartbeat_seconds
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
            record("heartbeat_reply")
            continue
        if emsg == EMSG_CLIENT_LOGOFF:
            record("logoff")
            conn.send_frame(2, pack(EMSG_CLIENT_LOGGED_OFF,
                                    pb_int(1, steamid) + pb_int(2, sessionid), pb_int(1, 1)))
            conn.close_frame()
            return
        if emsg in (EMSG_CLIENT_GET_DEPOT_KEY, EMSG_CLIENT_PICS_ACCESS_TOKEN,
                    EMSG_CLIENT_PICS_PRODUCT_INFO, EMSG_CLIENT_CHECK_BETA_PASSWORD):
            serve_content(conn, emsg, hdr, body, st, steamid, sessionid)
            continue
        target = (hdr.get(12, [b""])[0]).decode() if hdr.get(12) else ""
        jobid = hdr.get(10, [0])[0]
        record(f"service {target}")
        if target == "ContentServerDirectory.GetServersForSteamPipe#1":
            serve_servers(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "ContentServerDirectory.GetManifestRequestCode#1":
            serve_request_code(conn, hdr, body, st, steamid, sessionid)
            continue
        if target == "ContentServerDirectory.GetCDNAuthToken#1":
            serve_cdn_token(conn, hdr, body, st, steamid, sessionid)
            continue
        if target.startswith("Player.GetPlayerLinkDetails"):
            # AccountPublicData: steamid, …, persona_name (field 10).
            resp_hdr = (pb_int(1, steamid) + pb_int(2, sessionid)
                        + pb_fixed64(11, jobid) + pb_int(13, 1))
            pub = pb_fixed64(1, steamid_req) + pb_str(10, st.get("persona", "TestPersona"))
            conn.send_frame(2, pack(EMSG_SERVICE_METHOD_RESPONSE,
                                    resp_hdr, pb_msg(1, pb_msg(1, pub))))
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


# --- content RPCs for hydra-steam-dl -------------------------------------
# Driven by the state file so the Go tests control depot keys, PICS, the
# server list, request codes and CDN tokens without touching Valve:
#   depot_key    hex of the 32-byte depot key the fake CDN encrypted with
#   app_token    PICS access token handed out for the app
#   depot_id     depot the PICS reply describes (default 11)
#   manifest_gid manifest gid the PICS reply and fake CDN agree on
#   oslist       oslist the PICS depot carries ("linux", "windows", ...)
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
        key = bytes.fromhex(st.get("depot_key", "00" * 32))
        record(f"depot_key {depot_id}")
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
        entry = (pb_int(1, app) + pb_int(2, 42)
                 + pb_str(5, product_kv(st).encode()))
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid),
                                pb_msg(1, entry)))
    elif emsg == EMSG_CLIENT_CHECK_BETA_PASSWORD:
        record("beta_password")
        conn.send_frame(2, pack(emsg + 1, content_hdr(steamid, sessionid, jobid),
                                pb_int(1, 1)))


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
    record(f"request_code depot={req.get(2, [0])[0]}")
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


def main():
    srv = socket.socket()
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", 0))
    srv.listen(4)
    print(srv.getsockname()[1], flush=True)
    while True:
        if not os.path.exists(STATE):
            return  # state file removed: test is done
        try:
            srv.settimeout(2)
            sock, _addr = srv.accept()
        except socket.timeout:
            continue
        srv.settimeout(None)
        try:
            serve(Conn(sock))
        except OSError:
            pass
        finally:
            sock.close()


if __name__ == "__main__":
    main()