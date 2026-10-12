package customize

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func artPNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "art.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Art source attribution (issue #36): remembered on an online apply,
// cleared when the looked-up art is gone, ended by a hand-picked image.
func TestArtSourceAttribution(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "custom-art"))
	if err != nil {
		t.Fatal(err)
	}
	id := "custom-1"
	src := artPNG(t)

	if err := s.SetArtSource(id, "steam:123", "Hollow Knight"); err == nil {
		t.Fatal("SetArtSource with no art = nil, want a refusal")
	}
	if err := s.SetArt(id, Cover, src); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArt(id, Banner, src); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArtSource(id, "steam:123", "Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	e := s.Get(id)
	if e.ArtSource != "steam:123" || e.ArtSourceName != "Hollow Knight" {
		t.Fatalf("source = %q/%q", e.ArtSource, e.ArtSourceName)
	}
	// Resetting one kind keeps the credit while looked-up art remains.
	if err := s.ResetArt(id, Cover); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(id).ArtSource; got == "" {
		t.Fatal("resetting the cover dropped the credit for the banner")
	}
	// Resetting the last of it clears the credit.
	if err := s.ResetArt(id, Banner); err != nil {
		t.Fatal(err)
	}
	if e := s.Get(id); e.ArtSource != "" || e.ArtSourceName != "" {
		t.Fatalf("source after reset = %q/%q, want cleared", e.ArtSource, e.ArtSourceName)
	}
	// A hand-picked cover ends the credit.
	if err := s.SetArt(id, Cover, src); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArt(id, Banner, src); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArtSource(id, "steam:123", "Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArt(id, Cover, src); err != nil {
		t.Fatal(err)
	}
	if e := s.Get(id); e.ArtSource != "" {
		t.Fatalf("source after a hand pick = %q, want cleared", e.ArtSource)
	}
	// Resetting everything clears it too.
	if err := s.SetArtSource(id, "steam:123", "Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(id); err != nil {
		t.Fatal(err)
	}
	if e := s.Get(id); e.ArtSource != "" || len(e.Art) != 0 {
		t.Fatalf("entry after reset = %+v, want empty", e)
	}
}
