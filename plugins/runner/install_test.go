package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Issue #142: available lists releases through the downloader; remove only
// deletes user-installed builds; install verifies the checksum and unpacks
// safely.
func TestAvailableInstallRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var tarBytes []byte
	{
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		body := []byte("#!/bin/sh\n")
		_ = tw.WriteHeader(&tar.Header{Name: "GE-Proton10-99/proton", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write(body)
		_ = tw.Close()
		_ = gz.Close()
		tarBytes = buf.Bytes()
	}
	sum := sha512.Sum512(tarBytes)
	hexSum := hex.EncodeToString(sum[:])
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases":
			fmt.Fprintf(w, `[{"tag_name":"GE-Proton10-99","name":"GE-Proton10-99","assets":[{"name":"GE-Proton10-99.tar.gz","browser_download_url":"%s/g.tar.gz"},{"name":"GE-Proton10-99.sha512sum","browser_download_url":"%s/g.sha512sum"}]}]`, srvURL, srvURL)
		case "/g.tar.gz":
			w.Write(tarBytes)
		case "/g.sha512sum":
			fmt.Fprintf(w, "%s  GE-Proton10-99.tar.gz\n", hexSum)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	got := handle(request("runner.available", map[string]any{"releasesUrl": srv.URL + "/releases"}))
	if got.Error != nil {
		t.Fatalf("available: %+v", got.Error)
	}

	got = handle(request("runner.install", map[string]any{
		"tag":         "GE-Proton10-99",
		"url":         srv.URL + "/g.tar.gz",
		"checksumUrl": srv.URL + "/g.sha512sum",
	}))
	if got.Error != nil {
		t.Fatalf("install: %+v", got.Error)
	}
	proton := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-99/proton")
	if _, err := os.Stat(proton); err != nil {
		t.Fatalf("installed proton missing: %v", err)
	}

	got = handle(request("runner.remove", map[string]any{"id": "system:wine"}))
	if got.Error == nil {
		t.Fatal("removing system wine must fail")
	}
	got = handle(request("runner.remove", map[string]any{"id": "compat:GE-Proton10-99"}))
	if got.Error != nil {
		t.Fatalf("remove: %+v", got.Error)
	}
	if _, err := os.Stat(proton); !os.IsNotExist(err) {
		t.Fatalf("build still exists: %v", err)
	}
}
