// Tests the Ubisoft Connect plugin (issues #192/#193/#194) through
// Platypus's real plugin path (manager, JSON-RPC). The installer fetch drives
// the real download code against a loopback server with a patched pin
// (testenv allows loopback); the client's caches, install registry and
// executable are synthetic fixtures in a temp prefix, encoded here by hand
// from docs/research/ubisoft-library-format.md. No network, no Wine, no
// Ubisoft account, no real Ubisoft file.
package ubisoft_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/plugtest"
)

const id = "ubisoft-connect"

// pinServer serves bytes as the installer download and counts its hits.
func pinServer(t *testing.T, data []byte) (*httptest.Server, *int64) {
	t.Helper()
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func pinEnv(srv *httptest.Server, data []byte) map[string]string {
	sum := sha256.Sum256(data)
	return map[string]string{
		"PLATYPUS_UBISOFT_INSTALLER_URL":    srv.URL + "/UbisoftConnectInstaller.exe",
		"PLATYPUS_UBISOFT_INSTALLER_SHA256": hex.EncodeToString(sum[:]),
		"PLATYPUS_UBISOFT_INSTALLER_SIZE":   fmt.Sprint(len(data)),
	}
}

func suite(extra map[string]string) plugtest.Suite {
	return plugtest.Suite{ID: id, CopyDir: ".", Env: extra}
}

// fakeInstaller is the loopback "Connect installer": 64 KiB of pattern bytes.
func fakeInstaller() []byte {
	data := make([]byte, 64<<10)
	for i := range data {
		data[i] = byte(i * 31)
	}
	return data
}

func call(t *testing.T, s *plugtest.Session, method string, params, result any) {
	t.Helper()
	if err := s.Call(method, params, result); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

func wantCode(t *testing.T, s *plugtest.Session, method string, params any, code int) {
	t.Helper()
	var raw json.RawMessage
	err := s.Call(method, params, &raw)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("%s: got %s (%v), want RPC error %d", method, raw, err, code)
	}
	if rpcErr.Code != code {
		t.Fatalf("%s: error code %d (%s), want %d", method, rpcErr.Code, rpcErr.Message, code)
	}
}

// The client's caches, encoded by hand: protobuf varint and
// length-delimited fields only, as docs/research/ubisoft-library-format.md
// describes them (ConfigurationCache/Configuration in §2, OwnershipCache in
// §3, the install registry text in §4).

func pbVarint(value uint64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value == 0 {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func pbInt(field int, value uint64) []byte {
	return append(pbVarint(uint64(field)<<3), pbVarint(value)...)
}

func pbBytes(field int, payload []byte) []byte {
	out := append(pbVarint(uint64(field)<<3|2), pbVarint(uint64(len(payload)))...)
	return append(out, payload...)
}

// configRecord is one Configuration message: product id 1, uplay id 2, the
// product's YAML blob 3.
func configRecord(product, uplay uint64, yaml string) []byte {
	body := pbInt(1, product)
	if uplay != 0 {
		body = append(body, pbInt(2, uplay)...)
	}
	return pbBytes(1, append(body, pbBytes(3, []byte(yaml))...))
}

// ownershipFile is an OwnershipCache: field 1 per owned game (product id 1)
// plus field 2, the same ids as packed uint32.
func ownershipFile(ids ...uint64) []byte {
	var out []byte
	packed := []byte{}
	for _, id := range ids {
		out = append(out, pbBytes(1, pbInt(1, id))...)
		packed = append(packed, pbVarint(id)...)
	}
	return append(out, pbBytes(2, packed)...)
}

// gameYAML is a launchable product's v2.0 YAML blob.
func gameYAML(name, exe string) string {
	return "version: 2.0\nroot:\n  name: \"" + name + "\"\n" +
		"  start_game:\n    online:\n      executables:\n" +
		"        - path:\n            relative: \"" + exe + "\"\n" +
		"    offline:\n      executables:\n" +
		"        - path:\n            relative: \"" + exe + "\"\n" +
		"  installer:\n    game_identifier: " + name + "\n"
}

// dlcYAML is an add-on product: is_ulc, no start_game.
func dlcYAML(name string) string {
	return "version: 2.0\nroot:\n  name: \"" + name + "\"\n" +
		"  is_visible: no\n  is_ulc: yes\n"
}

// thirdPartyYAML is a product sold on another store: no Ubisoft library entry.
func thirdPartyYAML(name string) string {
	return "version: 2.0\nroot:\n  name: \"" + name + "\"\n" +
		"  start_game:\n    online:\n      executables:\n" +
		"        - path:\n            relative: \"Game.exe\"\n" +
		"  third_party_platform:\n    name: Steam\n"
}

// cacheDir is the client's per-user data folder in the prefix.
func cacheDir(prefix string) string {
	return filepath.Join(prefix, "drive_c", "users", "steamuser", "AppData",
		"Local", "Ubisoft Game Launcher")
}

// plantCaches writes the plaintext catalogue and ownership cache. Passing
// owned also writes the ownership file (an empty list writes an empty one).
func plantCaches(t *testing.T, prefix string, records [][]byte, owned ...uint64) {
	t.Helper()
	configDir := filepath.Join(cacheDir(prefix), "cache", "configuration")
	ownDir := filepath.Join(cacheDir(prefix), "cache", "ownership")
	for _, dir := range []string{configDir, ownDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	catalogue := []byte{}
	for _, record := range records {
		catalogue = append(catalogue, record...)
	}
	if err := os.WriteFile(filepath.Join(configDir, "configurations"), catalogue, 0o644); err != nil {
		t.Fatal(err)
	}
	if owned != nil {
		if err := os.WriteFile(filepath.Join(ownDir, "1234"), ownershipFile(owned...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// plantEncryptedCache writes the 2026 .vdat container instead of the
// plaintext files: a versioned_storage FileRoot with version 1, an
// AES-256-GCM config and ciphertext-shaped bytes.
func plantEncryptedCache(t *testing.T, prefix string) {
	t.Helper()
	configDir := filepath.Join(cacheDir(prefix), "cache", "configuration")
	ownDir := filepath.Join(cacheDir(prefix), "cache", "ownership")
	for _, dir := range []string{configDir, ownDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	crypto := append(pbInt(1, 1), pbBytes(2, []byte("0640adc5590d47b6a556f0d2e5dcd4e0"))...)
	crypto = append(crypto, pbBytes(3, bytes.Repeat([]byte{1}, 12))...)
	crypto = append(crypto, pbBytes(4, bytes.Repeat([]byte{2}, 16))...)
	header := append(pbInt(1, 1), pbBytes(2, crypto)...)
	body := append(pbBytes(1, header), pbBytes(2, bytes.Repeat([]byte{0xaa}, 64))...)
	data := pbBytes(1, body)
	if err := os.WriteFile(filepath.Join(configDir, "configurations.vdat"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ownDir, "1234.vdat"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// plantInstalls writes the client's install registry into Wine's text
// system.reg (HKLM), both of which the reader must tolerate.
func plantInstalls(t *testing.T, prefix string, installs map[string]string) {
	t.Helper()
	ids := make([]string, 0, len(installs))
	for id := range installs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lines := []string{"WINE REGISTRY Version 2", "#arch=win64"}
	for _, id := range ids {
		escaped := strings.ReplaceAll(installs[id], `\`, `\\`)
		lines = append(lines, "",
			`[Software\\Wow6432Node\\Ubisoft\\Launcher\\Installs\\`+id+`] 1700000000`,
			`"InstallDir"="`+escaped+`"`)
	}
	if err := os.WriteFile(filepath.Join(prefix, "system.reg"),
		[]byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// libraryError is library.list's RPC error, for message checks.
func libraryError(t *testing.T, s *plugtest.Session) *jsonrpc.Error {
	t.Helper()
	var raw json.RawMessage
	err := s.Call("library.list", nil, &raw)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("library.list: got %s (%v), want the unreadable-cache status", raw, err)
	}
	return rpcErr
}

// plantClient writes a client executable into a drive layout.
func plantClient(t *testing.T, prefix, layout, name string) {
	t.Helper()
	exe := filepath.Join(prefix, layout, "Program Files (x86)", "Ubisoft",
		"Ubisoft Game Launcher", name)
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fake "+name), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUbisoftHandshake(t *testing.T) {
	s := suite(nil).Handshake(t)
	// The manifest speaks 1.20 (setup + launch) and 1.23 (plus the
	// core-run client commands); the core offering up to 1.23 negotiates
	// exactly that.
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var man struct {
		Protocol struct {
			Versions []string `json:"versions"`
		} `json:"protocol"`
		Capabilities []string `json:"capabilities"`
		Version      string   `json:"version"`
	}
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(man.Protocol.Versions, []string{"1.20", "1.23"}) {
		t.Fatalf("manifest protocol versions = %v, want [1.20 1.23]", man.Protocol.Versions)
	}
	if man.Version != "0.2.0" {
		t.Fatalf("manifest version = %q, want 0.2.0", man.Version)
	}
	if !slices.Equal(man.Capabilities, []string{"library", "install", "launch"}) {
		t.Fatalf("manifest capabilities = %v, want [library install launch]", man.Capabilities)
	}
	var init struct {
		Protocol     string   `json:"protocol"`
		Capabilities []string `json:"capabilities"`
	}
	call(t, s, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.20", "1.23"}}, &init)
	if init.Protocol != "1.23" {
		t.Fatalf("protocol = %q, want the 1.23 pick", init.Protocol)
	}
	if !slices.Equal(init.Capabilities, []string{"library", "install", "launch"}) {
		t.Fatalf("capabilities = %v, want [library install launch] over 1.23", init.Capabilities)
	}
	call(t, s, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.20"}}, &init)
	if init.Protocol != "1.20" {
		t.Fatalf("protocol = %q, want the 1.20 fallback", init.Protocol)
	}
	if !slices.Equal(init.Capabilities, []string{"library", "launch"}) {
		t.Fatalf("capabilities = %v, want [library launch] over 1.20 (no install)", init.Capabilities)
	}
	// Over 1.20 the install methods read as unimplemented; job.cancel stays
	// harmless (stopping nothing).
	wantCode(t, s, "game.install", map[string]any{}, jsonrpc.CodeMethodNotFound)
	wantCode(t, s, "game.installSize", map[string]any{}, jsonrpc.CodeMethodNotFound)
	var discarded json.RawMessage
	call(t, s, "job.cancel", map[string]any{"jobId": "nothing-running"}, &discarded)
}

func TestUbisoftClientSetupFetch(t *testing.T) {
	data := fakeInstaller()
	srv, hits := pinServer(t, data)
	sess := suite(pinEnv(srv, data)).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")

	var raw json.RawMessage
	call(t, sess, "client.setup",
		map[string]any{"jobId": "setup-1", "prefix": prefix}, &raw)
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("client.setup isn't an object: %s", raw)
	}
	for k := range spec {
		if !slices.Contains(plugtest.InstallerKeys, k) {
			t.Errorf("client.setup has the key %q, which the protocol doesn't define", k)
		}
	}
	var decoded struct {
		Exe     string   `json:"exe"`
		Args    []string `json:"args"`
		Size    int64    `json:"size"`
		Confirm string   `json:"confirm"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("client.setup isn't an installer spec: %s", raw)
	}
	wantExe := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "UbisoftConnectInstaller.exe")
	if decoded.Exe != wantExe {
		t.Errorf("exe = %q, want the prefix copy %q", decoded.Exe, wantExe)
	}
	if got, err := os.ReadFile(decoded.Exe); err != nil || string(got) != string(data) {
		t.Errorf("prefix copy isn't the downloaded bytes: %v", err)
	}
	if !slices.Equal(decoded.Args, []string{"/S"}) {
		t.Errorf("args = %v, want the silent-install [/S]", decoded.Args)
	}
	if decoded.Size != int64(len(data)) {
		t.Errorf("size = %d, want %d", decoded.Size, len(data))
	}
	if !strings.Contains(decoded.Confirm, "downloads") || !strings.Contains(decoded.Confirm, "127.0.0.1") {
		t.Errorf("confirm = %q, want the download approval text", decoded.Confirm)
	}
	// The empty-jobId inspection answers the same spec, reusing the cached
	// download instead of fetching again.
	var again json.RawMessage
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, &again)
	if string(again) != string(raw) {
		t.Errorf("inspection spec = %s, want %s", again, raw)
	}
	if n := atomic.LoadInt64(hits); n != 1 {
		t.Errorf("server saw %d downloads, want 1 (cache reuse)", n)
	}
	// The Proton-only layout takes the copy under pfx/drive_c.
	proton := filepath.Join(t.TempDir(), "proton")
	os.MkdirAll(filepath.Join(proton, "pfx", "drive_c"), 0o755)
	call(t, sess, "client.setup", map[string]any{"jobId": "setup-pfx", "prefix": proton}, &raw)
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(proton, "pfx", "drive_c", "platypus-ubisoft",
		"UbisoftConnectInstaller.exe"); decoded.Exe != want {
		t.Errorf("exe = %q, want the Proton-layout copy %q", decoded.Exe, want)
	}
	wantCode(t, sess, "client.setup", map[string]any{"jobId": "x"}, jsonrpc.CodeInvalidParams)
}

func TestUbisoftClientSetupPinMismatch(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)

	t.Run("sha", func(t *testing.T) {
		env := pinEnv(srv, data)
		env["PLATYPUS_UBISOFT_INSTALLER_SHA256"] = strings.Repeat("0", 64)
		sess := suite(env).Start(t)
		wantCode(t, sess, "client.setup",
			map[string]any{"jobId": "s", "prefix": t.TempDir()}, -32000)
	})
	t.Run("size", func(t *testing.T) {
		env := pinEnv(srv, data)
		env["PLATYPUS_UBISOFT_INSTALLER_SIZE"] = fmt.Sprint(len(data) + 1)
		sess := suite(env).Start(t)
		wantCode(t, sess, "client.setup",
			map[string]any{"jobId": "s", "prefix": t.TempDir()}, -32000)
	})
}

// startWithPin starts a session whose installer pin is a loopback server, so
// client.setup (which every library test needs to remember the prefix) never
// touches the network.
func startWithPin(t *testing.T) *plugtest.Session {
	t.Helper()
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	return suite(pinEnv(srv, data)).Start(t)
}

func rememberPrefix(t *testing.T, sess *plugtest.Session, prefix string) {
	t.Helper()
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, nil)
}

// statusMessage is the plugin's single "can't read the library yet" status.
func statusMessage(t *testing.T, sess *plugtest.Session) string {
	t.Helper()
	err := libraryError(t, sess)
	if err.Code != -32002 {
		t.Fatalf("library.list error code = %d (%s), want the -32002 status",
			err.Code, err.Message)
	}
	return err.Message
}

func TestUbisoftLibraryNeedsClient(t *testing.T) {
	// No client.setup ran: no prefix is known.
	sess := suite(nil).Start(t)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Start Ubisoft Connect once") {
		t.Fatalf("status = %q, want the start-the-client hint", msg)
	}
	wantCode(t, sess, "game.details", map[string]any{"id": "635"}, -32002)

	// The locale from initialize is the language of the status (1.14).
	call(t, sess, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.23"}, "locale": "fr-FR"}, nil)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Lancez Ubisoft Connect une fois") {
		t.Fatalf("status = %q, want the French start-the-client hint", msg)
	}
	// A bare "fr" tag and a switch notification work the same way.
	call(t, sess, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.23"}, "locale": "fr"}, nil)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Lancez Ubisoft Connect une fois") {
		t.Fatalf("status under fr = %q, want French", msg)
	}
	call(t, sess, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.23"}, "locale": "en-GB"}, nil)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Start Ubisoft Connect once") {
		t.Fatalf("status under en-GB = %q, want English", msg)
	}
}

func TestUbisoftLibraryLocalCache(t *testing.T) {
	sess := startWithPin(t)
	prefix := filepath.Join(t.TempDir(), "client")
	rememberPrefix(t, sess, prefix)
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Splinter Cell Conviction", `src\system\Conviction_game.exe`)),
		configRecord(636, 0, gameYAML("Owned Not Installed", "Game2.exe")),
		configRecord(96, 0, dlcYAML("Conviction DLC")),
		configRecord(97, 0, thirdPartyYAML("Steam Copy")),
		configRecord(900, 0, gameYAML("Not Owned", "Other.exe")),
	}, 635, 636, 96, 97)
	plantInstalls(t, prefix, map[string]string{
		"635": `C:\Program Files (x86)\Ubisoft\Game`,
	})

	var lib struct {
		Games []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Installed   bool   `json:"installed"`
			InstallPath string `json:"installPath"`
			Installable bool   `json:"installable"`
		} `json:"games"`
	}
	call(t, sess, "library.list", nil, &lib)
	if len(lib.Games) != 2 {
		t.Fatalf("library.list = %+v, want the two owned launchable games", lib.Games)
	}
	byID := map[string]struct {
		Title       string `json:"title"`
		Installed   bool   `json:"installed"`
		InstallPath string `json:"installPath"`
		Installable bool   `json:"installable"`
	}{}
	for _, game := range lib.Games {
		byID[game.ID] = struct {
			Title       string `json:"title"`
			Installed   bool   `json:"installed"`
			InstallPath string `json:"installPath"`
			Installable bool   `json:"installable"`
		}{game.Title, game.Installed, game.InstallPath, game.Installable}
	}
	if installed := byID["635"]; installed.Title != "Splinter Cell Conviction" ||
		!installed.Installed || !installed.Installable {
		t.Errorf("game = %+v, want 635 installed and installable", installed)
	}
	if want := filepath.Join(prefix, "drive_c", "Program Files (x86)", "Ubisoft", "Game"); byID["635"].InstallPath != want {
		t.Errorf("installPath = %q, want the registry path mapped into the prefix %q",
			byID["635"].InstallPath, want)
	}
	if owned := byID["636"]; owned.Title != "Owned Not Installed" ||
		owned.Installed || owned.InstallPath != "" || !owned.Installable {
		t.Errorf("game = %+v, want 636 owned and not installed", owned)
	}

	// Only protocol keys go on the wire (the DLC/third-party/not-owned
	// products are dropped, not flagged).
	var raw struct {
		Games []json.RawMessage `json:"games"`
	}
	call(t, sess, "library.list", nil, &raw)
	for _, entry := range raw.Games {
		var fields map[string]any
		if err := json.Unmarshal(entry, &fields); err != nil {
			t.Fatal(err)
		}
		for key := range fields {
			if !slices.Contains(plugtest.LibraryKeys, key) {
				t.Errorf("library.list game has the key %q, which the protocol doesn't define", key)
			}
		}
	}

	// The catalogue's executable is the launch find, not the install path.
	var launch struct {
		Find struct {
			Cmdline []string `json:"cmdline"`
		} `json:"find"`
	}
	call(t, sess, "game.launchSpec",
		map[string]any{"id": "635", "prefix": prefix, "preview": true}, &launch)
	if !slices.Equal(launch.Find.Cmdline, []string{"Conviction_game.exe"}) {
		t.Errorf("find = %v, want the catalogue's game exe", launch.Find.Cmdline)
	}
	// A known game has no metadata source: it answers {}; an unknown id errors.
	var details json.RawMessage
	call(t, sess, "game.details", map[string]any{"id": "635"}, &details)
	if string(details) != "{}" {
		t.Errorf("game.details = %s, want {}", details)
	}
	wantCode(t, sess, "game.details", map[string]any{"id": "999"}, -32000)

	// An account with nothing owned is a valid, empty library (not a status).
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Splinter Cell Conviction", "Game.exe")),
	}, []uint64{}...)
	call(t, sess, "library.list", nil, &lib)
	if len(lib.Games) != 0 {
		t.Fatalf("library.list = %+v, want an empty library for no owned games", lib.Games)
	}
}

func TestUbisoftLibraryUnreadable(t *testing.T) {
	sess := startWithPin(t)
	prefix := filepath.Join(t.TempDir(), "client")
	rememberPrefix(t, sess, prefix)
	game := [][]byte{configRecord(635, 0, gameYAML("Game", "Game.exe"))}

	// The prefix exists but the client never wrote its caches.
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Start Ubisoft Connect once") {
		t.Fatalf("status = %q, want the start-the-client hint", msg)
	}

	// A truncated catalogue: the first record declares more bytes than the
	// file holds.
	plantCaches(t, prefix, [][]byte{{0x0a, 0xff}}, 635)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Start Ubisoft Connect once") {
		t.Fatalf("truncated catalogue status = %q", msg)
	}

	// A garbage ownership file (declared length past the end).
	plantCaches(t, prefix, game, 635)
	own := filepath.Join(cacheDir(prefix), "cache", "ownership", "1234")
	if err := os.WriteFile(own, []byte{0x0a, 0x05, 'a'}, 0o644); err != nil {
		t.Fatal(err)
	}
	statusMessage(t, sess)

	// The 2026 encrypted container: detected, never guessed.
	plantEncryptedCache(t, prefix)
	if msg := statusMessage(t, sess); !strings.Contains(msg, "Start Ubisoft Connect once") {
		t.Fatalf("encrypted cache status = %q, want the start-the-client hint", msg)
	}

	// A plaintext file that is actually a .vdat-shaped envelope is garbage
	// to the plaintext reader: still the same status, no crash.
	data, err := os.ReadFile(filepath.Join(cacheDir(prefix), "cache", "configuration", "configurations.vdat"))
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(cacheDir(prefix), "cache", "ownership"))
	os.Remove(filepath.Join(cacheDir(prefix), "cache", "configuration", "configurations.vdat"))
	if err := os.WriteFile(filepath.Join(cacheDir(prefix), "cache", "configuration", "configurations"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	statusMessage(t, sess)
}

func TestUbisoftLaunchSpec(t *testing.T) {
	sess := suite(nil).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	// No client executable planted: the spec is still answered (Windows
	// paths), so the core reports client.notSetUp and offers the setup.
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Game", `bin\Game.exe`)),
		configRecord(637, 0, gameYAML("Client-named", "upc.exe")),
	}, 635, 637)
	plantInstalls(t, prefix, map[string]string{
		"635": `C:\Ubisoft\Game`,
		"637": `C:\Ubisoft\Client`,
	})

	var raw json.RawMessage
	call(t, sess, "game.launchSpec",
		map[string]any{"id": "635", "prefix": prefix, "preview": true}, &raw)
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("launchSpec isn't an object: %s", raw)
	}
	for k := range spec {
		if !slices.Contains(plugtest.LaunchKeys, k) {
			t.Errorf("launchSpec has the key %q, which the protocol doesn't define", k)
		}
	}
	var launch struct {
		Delegate string   `json:"delegate"`
		Client   string   `json:"client"`
		Exe      string   `json:"exe"`
		Args     []string `json:"args"`
		Prefix   string   `json:"prefix"`
		Find     struct {
			Cmdline []string `json:"cmdline"`
		} `json:"find"`
		StartTimeout int `json:"startTimeout"`
	}
	if err := json.Unmarshal(raw, &launch); err != nil {
		t.Fatalf("launchSpec isn't a wine-client spec: %s", raw)
	}
	if launch.Delegate != "wine-client" || launch.Client == "" {
		t.Errorf("launch = %+v, want the wine-client delegate", launch)
	}
	if want := `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\UbisoftConnect.exe`; launch.Exe != want {
		t.Errorf("exe = %q, want the preferred client path %q", launch.Exe, want)
	}
	if !slices.Equal(launch.Args, []string{"uplay://launch/635/0"}) {
		t.Errorf("args = %v, want the launch URL", launch.Args)
	}
	if launch.Prefix != "client" {
		t.Errorf("prefix = %q, want client", launch.Prefix)
	}
	if !slices.Equal(launch.Find.Cmdline, []string{"Game.exe"}) {
		t.Errorf("find = %v, want the catalogue's game exe", launch.Find.Cmdline)
	}
	if launch.StartTimeout != 900 {
		t.Errorf("startTimeout = %d, want the 900 s client window", launch.StartTimeout)
	}
	// No install entry, no spec: the game isn't installed.
	wantCode(t, sess, "game.launchSpec",
		map[string]any{"id": "636", "prefix": prefix}, -32000)
	// A catalogue naming the client itself is refused (find must track the
	// game).
	wantCode(t, sess, "game.launchSpec",
		map[string]any{"id": "637", "prefix": prefix}, -32000)
}

func TestUbisoftClientStatus(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	sess := suite(pinEnv(srv, data)).Start(t)

	// No setup ran in this session: the client isn't set up.
	var status struct {
		State string `json:"state"`
		Label string `json:"label"`
	}
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "idle" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the idle not-set-up state", status)
	}

	prefix := filepath.Join(t.TempDir(), "client")
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, nil)
	// Client present but the game unknown: waiting (sign in if it asks).
	plantClient(t, prefix, "drive_c", "UbisoftConnect.exe")
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "updating" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the waiting state", status)
	}
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Game", "Game.exe")),
	}, 635)
	plantInstalls(t, prefix, map[string]string{"635": `C:\Ubisoft\Game`})
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the launching state", status)
	}

	// Proton layout plus the older exe name: still found (S2, S3).
	legacy := filepath.Join(t.TempDir(), "legacy")
	call(t, sess, "client.setup", map[string]any{"jobId": "l", "prefix": legacy}, nil)
	plantClient(t, legacy, filepath.Join("pfx", "drive_c"), "upc.exe")
	legacyDir := filepath.Join(legacy, "pfx", "drive_c", "users", "steamuser",
		"AppData", "Local", "Ubisoft Game Launcher", "cache")
	if err := os.MkdirAll(filepath.Join(legacyDir, "configuration"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(legacyDir, "ownership"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "configuration", "configurations"),
		configRecord(635, 0, gameYAML("Game", "Game.exe")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "ownership", "1234"),
		ownershipFile(635), 0o644); err != nil {
		t.Fatal(err)
	}
	plantInstalls(t, legacy, map[string]string{"635": `C:\Ubisoft\Game`})
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" {
		t.Errorf("clientStatus = %+v, want launching via the Proton/upc.exe fallback", status)
	}
}

// waitJobEvent polls the plugin's job notifications for one of the named
// methods on a job, failing when it doesn't arrive in time.
func waitJobEvent(t *testing.T, s *plugtest.Session, jobID string, methods []string, within time.Duration) (string, plugins.JobUpdate) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			for _, m := range methods {
				if e.Method == m {
					return e.Method, e.Update
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	var got []string
	for _, e := range s.Events(jobID) {
		got = append(got, e.Method+":"+e.Update.JobID)
	}
	t.Fatalf("no %v for job %s within %v (saw %v)", methods, jobID, within, got)
	return "", plugins.JobUpdate{}
}

// noJobEvent fails when one of the named methods arrives for a job within
// the window (a canceled watch must stay silent).
func noJobEvent(t *testing.T, s *plugtest.Session, jobID string, methods []string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			for _, m := range methods {
				if e.Method == m {
					t.Fatalf("unexpected %s for job %s", m, jobID)
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type clientRunAnswer struct {
	ClientRun struct {
		Exe  string   `json:"exe"`
		Args []string `json:"args"`
	} `json:"clientRun"`
	ClientOwnsLocation bool `json:"clientOwnsLocation"`
}

func setupPrefix(t *testing.T, sess *plugtest.Session, prefix string) {
	t.Helper()
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, nil)
	plantClient(t, prefix, "drive_c", "UbisoftConnect.exe")
}

func TestUbisoftInstallAnswersClientRun(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_WATCH_TIMEOUT"] = "10"
	env["PLATYPUS_UBISOFT_WATCH_POLL"] = "0.05"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var ans clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "install-1"}, &ans)
	if ans.ClientRun.Exe != `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\UbisoftConnect.exe` {
		t.Errorf("clientRun exe = %q, want the client path", ans.ClientRun.Exe)
	}
	if !slices.Equal(ans.ClientRun.Args, []string{"uplay://install/635"}) {
		t.Errorf("clientRun args = %v, want the install URL", ans.ClientRun.Args)
	}
	if !ans.ClientOwnsLocation {
		t.Error("clientOwnsLocation = false, want true (the client owns installs)")
	}
	// The plugin ends the job from the client's state: the install registry
	// gaining the product's InstallDir completes the install.
	plantInstalls(t, prefix, map[string]string{"635": `C:\Ubisoft\Game`})
	waitJobEvent(t, sess, "install-1", []string{"job.done"}, 5*time.Second)

	var upd clientRunAnswer
	call(t, sess, "game.update",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "update-1"}, &upd)
	if !slices.Equal(upd.ClientRun.Args, []string{"uplay://install/635"}) {
		t.Errorf("update args = %v, want the install URL", upd.ClientRun.Args)
	}
	// A pre-existing install is not completion: the update must not finish
	// until the client rewrites its catalogue cache.
	noJobEvent(t, sess, "update-1", []string{"job.done", "job.failed"}, time.Second)
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Game", "Game.exe")),
	}, 635)
	waitJobEvent(t, sess, "update-1", []string{"job.done"}, 5*time.Second)

	var un clientRunAnswer
	call(t, sess, "game.uninstall",
		map[string]any{"id": "635", "jobId": "uninstall-1"}, &un)
	if !slices.Equal(un.ClientRun.Args, []string{"uplay://uninstall/635"}) {
		t.Errorf("uninstall args = %v, want the uninstall URL", un.ClientRun.Args)
	}
	plantInstalls(t, prefix, nil)
	waitJobEvent(t, sess, "uninstall-1", []string{"job.done"}, 5*time.Second)

	// An uninstall for a game the client doesn't have fails readably.
	wantCode(t, sess, "game.uninstall",
		map[string]any{"id": "636", "jobId": "uninstall-x"}, -32000)

	// A canceled watch stays silent: cancel, then complete the install.
	var cancelAns clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "636", "installDir": t.TempDir(), "jobId": "install-cancel"}, &cancelAns)
	call(t, sess, "job.cancel", map[string]any{"jobId": "install-cancel"}, nil)
	plantInstalls(t, prefix, map[string]string{"636": `C:\Ubisoft\Other`})
	noJobEvent(t, sess, "install-cancel", []string{"job.done", "job.failed"}, time.Second)
}

func TestUbisoftInstallWatchTimeout(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_WATCH_TIMEOUT"] = "0.3"
	env["PLATYPUS_UBISOFT_WATCH_POLL"] = "0.05"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var ans clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "install-timeout"}, &ans)
	if ans.ClientRun.Exe == "" {
		t.Fatal("install didn't answer a clientRun command")
	}
	// No record ever appears: the watch fails with a readable message that
	// promises nothing (no library reader picks the install up, #192).
	method, update := waitJobEvent(t, sess, "install-timeout", []string{"job.failed"}, 5*time.Second)
	if method != "job.failed" || update.Error == "" ||
		!strings.Contains(update.Error, "didn't report the install of 635 in time") {
		t.Fatalf("job end = %s %+v, want the honest timeout failure", method, update)
	}
}

func TestUbisoftClientStatusSignin(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_SIGNIN_GRACE"] = "0"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var status struct {
		State string `json:"state"`
		Label string `json:"label"`
	}
	// No install entry: not launchable (updating), even with no grace.
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "updating" {
		t.Fatalf("clientStatus = %+v, want updating with no record", status)
	}
	plantCaches(t, prefix, [][]byte{
		configRecord(635, 0, gameYAML("Game", "Game.exe")),
	}, 635)
	plantInstalls(t, prefix, map[string]string{"635": `C:\Ubisoft\Game`})
	// Installed but nothing running: the client starts it (launching).
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" {
		t.Fatalf("clientStatus = %+v, want launching with nothing running", status)
	}
	// A client process in this prefix without the game: waiting for the
	// user in the client. (bash's exec -a sets argv[0], which the /proc
	// scan reads; dash's exec has no -a. WINEPREFIX scopes the match to
	// this prefix.)
	spawn := func(name string) {
		t.Helper()
		cmd := exec.Command("bash", "-c", "exec -a "+name+" sleep 30")
		cmd.Env = append(os.Environ(), "WINEPREFIX="+prefix)
		if err := cmd.Start(); err != nil {
			t.Skipf("can't spawn the fake %s process: %v", name, err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	}
	spawn("UbisoftConnect.exe")
	waitStatus := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			call(t, sess, "game.clientStatus",
				map[string]any{"id": "635", "since": 0}, &status)
			if status.State == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("clientStatus = %+v, want %s", status, want)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitStatus("signin")
	if status.Label == "" {
		t.Error("signin has no label for the UI")
	}
	// The game itself running reads as launching, as before.
	spawn("Game.exe")
	waitStatus("launching")
}
