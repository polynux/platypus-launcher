// Upload block and naming unit tests (issue #6 P3). No CM: blocks ride
// httptest (loopback only, like the repo's fast set demands), names come
// from synthetic rules.
package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A block PUT lands with the verbatim path, headers and file slice; a
// http_method 3 block goes out as POST with its explicit body.
func TestPutUploadBlockMethodsAndBodies(t *testing.T) {
	var got []struct {
		method, path, header, body string
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		got = append(got, struct {
			method, path, header, body string
		}{r.Method, r.URL.Path, r.Header.Get("X-Fake-Block"), string(buf)})
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	data := []byte("0123456789abcdef")

	if err := putUploadBlock(context.Background(), uploadBlock{
		host: host, path: "/up/a", method: "PUT",
		headers: map[string]string{"X-Fake-Block": "1"},
		offset:  2, length: 4,
	}, data); err != nil {
		t.Fatal(err)
	}
	if err := putUploadBlock(context.Background(), uploadBlock{
		host: host, path: "/up/a", method: "POST", headers: map[string]string{},
		explicit: []byte("carried"),
	}, data); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("requests = %d, want 2", len(got))
	}
	if got[0].method != "PUT" || got[0].path != "/up/a" || got[0].header != "1" || got[0].body != "2345" {
		t.Fatalf("PUT = %+v, want slice [2:6] with the header", got[0])
	}
	if got[1].method != "POST" || got[1].body != "carried" {
		t.Fatalf("POST = %+v, want the explicit body", got[1])
	}
}

// A refused block fails before anything is mistaken for success.
func TestPutUploadBlockHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	err := putUploadBlock(context.Background(), uploadBlock{
		host: host, path: "/up/a", method: "PUT", headers: map[string]string{},
	}, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("err = %v, want the HTTP status", err)
	}
}

// The public branch build id rides BeginAppUploadBatch; absent means 0.
func TestCloudAppBuildID(t *testing.T) {
	kv := mustParseKV(t, `"depots" { "branches" { "public" { "buildid" "1234" } } }`)
	if got := cloudAppBuildID(kv); got != 1234 {
		t.Fatalf("build = %d", got)
	}
	kv = mustParseKV(t, `"ufs" { "quota" "1" }`)
	if got := cloudAppBuildID(kv); got != 0 {
		t.Fatalf("build without depots = %d", got)
	}
}

// A local-only save names the %rule-root% cloud file a later listing
// carries, so push and status agree on it (real path: scan + compare).
func TestUploadCloudNameLocalOnly(t *testing.T) {
	home := t.TempDir()
	install := filepath.Join(home, "install")
	ufs := parseCloudRules(mustParseKV(t, gameInstallUFS()))
	ctx := cloudContext{OS: "linux", Home: home, InstallDir: install}
	writeLocal(t, filepath.Join(install, "saves", "new.sav"), "fresh")
	local := scanLocal(ufs, ctx, 0)
	base := &cloudBaseline{Files: map[string]cloudBaselineFile{}}
	files, _ := compareCloud(nil, local, ufs, ctx, 0, base)
	if len(files) != 1 {
		t.Fatalf("files = %+v", files)
	}
	if files[0].Want != "upload" {
		t.Fatalf("want = %q", files[0].Want)
	}
	if files[0].CloudName != "%GameInstall%saves/new.sav" {
		t.Fatalf("cloud name = %q", files[0].CloudName)
	}
	if back, _, ok := mapCloudName(files[0].CloudName, ufs, ctx, 0); !ok || back != files[0].Local {
		t.Fatalf("round trip = %q, %v", back, ok)
	}
}

// With a root override (SUPERHOT on native Linux), the upload name keeps
// the rule's own root: scan + compare must file it under the platform-
// independent name, and mapping that name back lands on the same file.
func TestUploadCloudNameRoundTripsOverride(t *testing.T) {
	home := t.TempDir()
	ufs := parseCloudRules(mustParseKV(t, superhotUFS))
	ctx := cloudContext{OS: "linux", Home: home, InstallDir: filepath.Join(home, "install")}
	local := filepath.Join(home, ".config", "unity3d", "SUPERHOT_Team", "SUPERHOT", "super.hot")
	writeLocal(t, local, "fresh")
	const name = "%WinAppDataLocalLow%SUPERHOT_Team/SUPERHOT/super.hot"
	files, _ := compareCloud(nil, scanLocal(ufs, ctx, 0), ufs, ctx, 0,
		&cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if len(files) != 1 {
		t.Fatalf("files = %+v", files)
	}
	up := &files[0]
	if up.Want != "upload" || up.CloudName != name || up.Rel != "SUPERHOT_Team/SUPERHOT/super.hot" {
		t.Fatalf("upload = %+v, want %q", up, name)
	}
	if back, _, ok := mapCloudName(name, ufs, ctx, 0); !ok || back != local {
		t.Fatalf("round trip = %q, %v", back, ok)
	}
}

// Two rules covering one directory with different roots, matching different
// patterns: the file uploads under the root of the rule whose pattern
// matched, not the first rule covering the directory.
func TestUploadCloudNameOverlappingRoots(t *testing.T) {
	home := t.TempDir()
	ufs := parseCloudRules(mustParseKV(t, `"ufs" { "savefiles" {
		"0" { "root" "GameInstall" "path" "saves" "pattern" "*.cfg" }
		"1" { "root" "LinuxHome" "path" "install/saves" "pattern" "*.sav" } } }`))
	ctx := cloudContext{OS: "linux", Home: home, InstallDir: filepath.Join(home, "install")}
	local := filepath.Join(home, "install", "saves", "game.sav")
	writeLocal(t, local, "sav")
	got := scanLocal(ufs, ctx, 0)
	if len(got) != 1 || got[local].Root != "LinuxHome" {
		t.Fatalf("scan = %+v, want the *.sav rule's root", got)
	}
	files, _ := compareCloud(nil, got, ufs, ctx, 0, &cloudBaseline{Files: map[string]cloudBaselineFile{}})
	if len(files) != 1 || files[0].CloudName != "%LinuxHome%install/saves/game.sav" {
		t.Fatalf("files = %+v", files)
	}
	if back, _, ok := mapCloudName(files[0].CloudName, ufs, ctx, 0); !ok || back != local {
		t.Fatalf("round trip = %q, %v", back, ok)
	}
}

// The staged read refuses a save that changed after the compare, instead
// of uploading half-written bytes (S5). The message names the cloud-rel
// path, never the local one.
func TestReadUploadFileRefusesChangedSave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.sav")
	writeLocal(t, p, "compared")
	f := cloudPlanFile{Rel: "saves/a.sav", Local: p, LocalInfo: statLocal(p)}
	if _, err := readUploadFile(&f); err != nil {
		t.Fatalf("unchanged file must read: %v", err)
	}
	if err := os.WriteFile(p, []byte("changed!"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := readUploadFile(&f)
	if err == nil || !strings.Contains(err.Error(), "changed while syncing") {
		t.Fatalf("data = %q, err = %v, want the changed-save refusal", data, err)
	}
	if data != nil {
		t.Fatalf("a refused read must return no bytes")
	}
}

// The client id is stable across runs of one install (S4).
func TestCloudClientIDStable(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	a, b := cloudClientID(), cloudClientID()
	if a == 0 || a != b {
		t.Fatalf("client ids = %d, %d, want one stable nonzero id", a, b)
	}
}

// The status message names the pending direction instead of the old
// not-enabled note.
func TestCloudMessageUploadWording(t *testing.T) {
	up := []cloudPlanFile{{Rel: "a", Want: "upload"}}
	if msg := cloudMessage(up, 0); !strings.Contains(msg, "waiting to upload") {
		t.Fatalf("message = %q", msg)
	}
	if strings.Contains(cloudMessage(up, 0), "isn't enabled") {
		t.Fatalf("message still disclaims uploads: %q", cloudMessage(up, 0))
	}
}
