// CM wire framing: every message is <uint32 EMsg|0x80000000 (LE)>
// <uint32 header length (LE)><CMsgProtoBufHeader><body bytes>.
// CMsgClientLogon puts the refresh token in access_token (field 108) and
// the account's steamid (the JWT's "sub") in the header steamid field.
// Service calls go through ServiceMethodCallFromClient (151) with
// target_job_name in the header; replies (147, or the CM's equivalent)
// match by jobid_target. Some replies arrive batched in CMsgMulti (gzip).
package steamcm

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hydra-launcher/internal/depot"
)

const protoMask = 0x80000000

// PackEMsg and UnpackEMsg expose the wire framing to the importable
// fake CM in internal/steamcm/steamcmtest; everything else keeps using
// the unexported forms.
func PackEMsg(emsg uint32, header, body []byte) []byte { return packEMsg(emsg, header, body) }

func UnpackEMsg(data []byte) (emsg uint32, hdr depot.Msg, body []byte, err error) {
	return unpackEMsg(data)
}

// Header builds a CMsgProtoBufHeader (see header); HeaderReply adds the
// jobid_target/eresult fields replies carry.
func Header(steamid uint64, sessionid int32, jobSource uint64, targetJobName string) []byte {
	return header(steamid, sessionid, jobSource, targetJobName)
}

func HeaderReply(steamid uint64, sessionid int32, jobTarget uint64, eresult int) []byte {
	base := header(steamid, sessionid, 0, "")
	out := append([]byte{}, base...)
	e := &depot.Enc{}
	e.Fixed64(11, jobTarget)
	e.Varint(13, uint64(eresult))
	return append(out, e.Bytes()...)
}

func MultiWrap(msgs ...[]byte) []byte { return multiWrap(msgs...) }

func packEMsg(emsg uint32, header, body []byte) []byte {
	out := make([]byte, 8, 8+len(header)+len(body))
	binary.LittleEndian.PutUint32(out, emsg|protoMask)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(header)))
	out = append(out, header...)
	return append(out, body...)
}

func unpackEMsg(data []byte) (uint32, depot.Msg, []byte, error) {
	if len(data) < 8 {
		return 0, nil, nil, fmt.Errorf("message too short (%d bytes)", len(data))
	}
	raw := binary.LittleEndian.Uint32(data)
	emsg := raw &^ protoMask
	if raw&protoMask == 0 {
		// Old-format (non-protobuf) message: Steam still mixes some
		// into Multi envelopes (seen live: 798, 782). The header is a
		// fixed struct, not a protobuf, so there is nothing to match
		// service calls by; callers ignore these via sideMessage.
		return emsg, depot.Msg{}, data[8:], nil
	}
	hlen := binary.LittleEndian.Uint32(data[4:])
	if len(data) < 8+int(hlen) {
		return 0, nil, nil, fmt.Errorf("header runs past the end")
	}
	hdr, err := depot.Decode(data[8 : 8+hlen])
	if err != nil {
		return 0, nil, nil, err
	}
	return emsg, hdr, data[8+hlen:], nil
}

// header builds a CMsgProtoBufHeader (steammessages_base.proto):
// steamid = 1 (fixed64), client_sessionid = 2, jobid_source = 10
// (fixed64), jobid_target = 11 (fixed64), target_job_name = 12.
func header(steamid uint64, sessionid int32, jobSource uint64, targetJobName string) []byte {
	e := &depot.Enc{}
	if steamid != 0 {
		e.Fixed64(1, steamid)
	}
	if sessionid != 0 {
		e.Varint(2, uint64(uint32(sessionid)))
	}
	if jobSource != 0 {
		e.Fixed64(10, jobSource)
	}
	if targetJobName != "" {
		e.Str(12, targetJobName)
	}
	return e.Bytes()
}

// headerWithTarget appends jobid_target (field 11) for matching replies.
func headerWithTarget(base []byte, jobTarget uint64) []byte {
	// The base is already a header; appending a field keeps it a valid
	// protobuf (fields may repeat; the CM reads the last one).
	out := append([]byte{}, base...)
	extra := &depot.Enc{}
	extra.Fixed64(11, jobTarget)
	return append(out, extra.Bytes()...)
}

// multiWrap packs messages into a CMsgMulti gzip envelope, like the real
// CM sometimes sends. Each inner message is length-prefixed ([u32 len][msg]).
func multiWrap(msgs ...[]byte) []byte {
	var chunks bytes.Buffer
	for _, m := range msgs {
		var h [4]byte
		binary.LittleEndian.PutUint32(h[:], uint32(len(m)))
		chunks.Write(h[:])
		chunks.Write(m)
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write(chunks.Bytes())
	w.Close()
	comp := gz.Bytes()
	e := &depot.Enc{}
	e.Varint(1, uint64(len(comp)))
	e.BytesField(2, comp)
	return packEMsg(EMsgMulti, nil, e.Bytes())
}

// multiUnwrap expands a CMsgMulti body into inner messages.
func multiUnwrap(body []byte) ([][]byte, error) {
	msg, err := depot.Decode(body)
	if err != nil {
		return nil, err
	}
	payload := msg.Bytes(2)
	if sizes := msg.AllInts(1); len(sizes) > 0 && len(payload) > 0 {
		if r, err := gzip.NewReader(bytes.NewReader(payload)); err == nil {
			if full, err := io.ReadAll(r); err == nil {
				payload = full
			}
			r.Close()
		}
	}
	var out [][]byte
	pos := 0
	for pos+4 <= len(payload) {
		n := int(binary.LittleEndian.Uint32(payload[pos:]))
		pos += 4
		if pos+n > len(payload) {
			return nil, fmt.Errorf("multi chunk runs past the end")
		}
		out = append(out, payload[pos:pos+n])
		pos += n
	}
	return out, nil
}

// machineIDBody builds a binary-KeyValues MessageObject with SHA-1
// hashes like SteamKit's machine_id (well-formed; the values just
// identify this machine). The seed is one fixed nonce: the same seed
// gives the same machine id, so a persistent session looks like the
// same device every time (research R2).
func machineIDBody(seed []byte) []byte {
	out := []byte("\x00MessageObject\x00")
	for _, name := range []string{"BB3", "FF2", "3B3"} {
		// One digest per key, derived from the same seed so the blob
		// is deterministic for the install but its keys differ.
		digest := sha1.Sum(append(append([]byte{}, seed...), name...))
		out = append(out, 0x01)
		out = append(out, name...)
		out = append(out, 0x00)
		hex := fmt.Sprintf("%x", digest)
		out = append(out, hex...)
		out = append(out, 0x00)
	}
	return append(out, 0x08, 0x08)
}

// machineSeed returns the stored per-install nonce (32 random bytes,
// created on first use) or nil when there is nowhere to store it. The
// file sits under HYDRA_PLUGIN_DATA with 0600 permissions, next to the
// refresh token.
func machineSeed(dir string) []byte {
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, "machine.id")
	if seed, err := os.ReadFile(path); err == nil && len(seed) >= 32 {
		return seed
	}
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	if err := os.WriteFile(path, seed, 0o600); err != nil {
		return nil
	}
	return seed
}

// StableMachineID returns the machine_id blob for the install whose
// plugin data lives in dataDir: stable across restarts, random on
// first use. dataDir "" or an unusable directory falls back to a
// random per-call id (the old behaviour).
func StableMachineID(dataDir string) []byte {
	if seed := machineSeed(dataDir); seed != nil {
		return machineIDBody(seed)
	}
	return randomMachineID()
}

// randomMachineID builds a one-off machine id, like the old per-logon
// behaviour.
func randomMachineID() []byte {
	seed := make([]byte, 32)
	rand.Read(seed)
	return machineIDBody(seed)
}

// jwtPayload decodes the payload segment of a Steam JWT ({" sub": ...}
// with spaces — standard base64url padding rules still apply).
func jwtPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT")
	}
	seg := parts[1]
	if m := len(seg) % 4; m != 0 {
		seg += strings.Repeat("=", 4-m)
	}
	raw, err := base64.URLEncoding.DecodeString(seg)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// SteamIDFromToken returns the JWT's "sub" (the account's steamid).
func SteamIDFromToken(refreshToken string) (uint64, error) {
	m, err := jwtPayload(refreshToken)
	if err != nil {
		return 0, &CMError{Msg: fmt.Sprintf("the stored login key isn't a usable Steam token (%v)", err), Auth: true}
	}
	var id uint64
	switch v := m["sub"].(type) {
	case string:
		fmt.Sscanf(v, "%d", &id)
	case float64:
		id = uint64(v)
	default:
		return 0, &CMError{Msg: "the stored login key isn't a usable Steam token (no sub)", Auth: true}
	}
	if id == 0 {
		return 0, &CMError{Msg: "the stored login key isn't a usable Steam token (no sub)", Auth: true}
	}
	return id, nil
}

// TokenExpiry returns the JWT's "exp" as a time, or the zero time.
func TokenExpiry(token string) time.Time {
	m, err := jwtPayload(token)
	if err != nil {
		return time.Time{}
	}
	var exp int64
	switch v := m["exp"].(type) {
	case float64:
		exp = int64(v)
	case json.Number:
		exp, _ = v.Int64()
	default:
		return time.Time{}
	}
	if exp == 0 {
		return time.Time{}
	}
	return time.Unix(exp, 0)
}

// logonBody builds CMsgClientLogon (steammessages_clientserver_login.proto):
// protocol_version = 1, obfuscated_private_ip = 11, deprecated ip = 2,
// client_package_version = 5, client_language = 6, client_os_type = 7,
// should_remember_password = 8, machine_id = 30, chat_mode = 33,
// machine_name = 96, supports_rate_limit_response = 102,
// access_token = 108 (the refresh token).
func logonBody(refreshToken string, loginID uint32, machineIDBlob []byte) []byte {
	obf := &depot.Enc{}
	obf.Fixed32(1, loginID) // v4
	e := &depot.Enc{}
	e.Varint(1, ProtocolVersion)
	e.MsgField(11, obf)
	e.Varint(2, uint64(loginID))
	e.Varint(5, ClientPackageVersion)
	e.Str(6, "english")
	e.Varint(7, uint64(ClientOSLinux))
	e.Varint(8, 1)
	e.BytesField(30, machineIDBlob)
	e.Varint(33, 2)
	e.Str(96, MachineName)
	e.Varint(102, 1)
	e.Str(108, refreshToken)
	return e.Bytes()
}
