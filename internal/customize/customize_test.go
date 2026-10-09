package customize

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pngFile(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func open(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "data")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestTitleAndSortTitlePersist(t *testing.T) {
	s, dir := open(t)
	if err := s.SetTitle("g1", "  My Game  ", "game, my"); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if e := again.Get("g1"); e.Title != "My Game" || e.SortTitle != "game, my" {
		t.Fatalf("reopened entry = %+v", e)
	}
	// Clearing both removes the game from the index.
	if err := s.SetTitle("g1", " ", ""); err != nil {
		t.Fatal(err)
	}
	if e := s.Get("g1"); !e.empty() {
		t.Fatalf("entry after clearing = %+v", e)
	}
}

func TestTitleValidation(t *testing.T) {
	s, _ := open(t)
	if err := s.SetTitle("g1", strings.Repeat("é", MaxTitleLen+1), ""); err == nil {
		t.Fatal("a title over the cap was accepted")
	}
	if err := s.SetTitle("g1", "bad\x00title", ""); err == nil {
		t.Fatal("a control character was accepted")
	}
	if err := s.SetTitle("g1", "ok", "bad\nsort"); err == nil {
		t.Fatal("a sort title with a newline was accepted")
	}
	if err := s.SetTitle("", "x", ""); err == nil {
		t.Fatal("an empty game id was accepted")
	}
	if e := s.Get("g1"); !e.empty() {
		t.Fatalf("a refused title was kept: %+v", e)
	}
	if err := s.SetTitle("g1", strings.Repeat("é", MaxTitleLen), ""); err != nil {
		t.Fatalf("a title at the cap was refused: %v", err)
	}
}

func TestArtIsCopiedIntoTheStoreAndSurvivesTheSource(t *testing.T) {
	s, dir := open(t)
	src := pngFile(t, t.TempDir(), "art.png", 8, 12)
	if err := s.SetArt("g1", Cover, src); err != nil {
		t.Fatal(err)
	}
	stored := s.Path("g1", Cover)
	if stored == "" || !strings.HasPrefix(stored, dir) || strings.HasPrefix(stored, filepath.Dir(src)) {
		t.Fatalf("stored path = %q, want a copy under %s", stored, dir)
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("the copy went with the source: %v", err)
	}
	u := s.URL("g1", Cover)
	if !strings.HasPrefix(u, URLPrefix) || !strings.Contains(u, "/cover.png?v=") {
		t.Fatalf("url = %q", u)
	}
	if s.URL("g1", Banner) != "" || s.URL("g2", Cover) != "" {
		t.Fatal("a kind or game without art has a url")
	}
	// It is still there after a reopen.
	again, _ := Open(dir)
	if again.Path("g1", Cover) != stored {
		t.Fatalf("reopened path = %q", again.Path("g1", Cover))
	}
}

func TestReplacingArtChangesTheVersionAndDropsTheOldExtension(t *testing.T) {
	s, _ := open(t)
	src := pngFile(t, t.TempDir(), "a.png", 4, 4)
	if err := s.SetArt("g1", Banner, src); err != nil {
		t.Fatal(err)
	}
	first := s.URL("g1", Banner)
	jpg := filepath.Join(t.TempDir(), "b.jpeg") // the content decides, not the name
	if err := os.WriteFile(jpg, append([]byte("\xff\xd8\xff\xe0"), make([]byte, 64)...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.SetArt("g1", Banner, jpg); err == nil {
		t.Fatal("a truncated JPEG was accepted")
	}
	if s.URL("g1", Banner) != first {
		t.Fatal("a refused replacement changed the image")
	}
	if err := s.SetArt("g1", Banner, pngFile(t, t.TempDir(), "c.png", 5, 5)); err != nil {
		t.Fatal(err)
	}
	if s.URL("g1", Banner) == first {
		t.Fatal("the version did not change on replacement")
	}
}

func TestArtValidation(t *testing.T) {
	s, _ := open(t)
	tmp := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(tmp, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	svg := write("a.png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	text := write("b.jpg", []byte("hello, I am not an image"))
	empty := write("c.png", nil)
	huge := pngFile(t, tmp, "d.png", MaxImageSide+1, 1)
	big := write("e.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxImageBytes)...))
	cases := map[string]string{"svg named .png": svg, "text named .jpg": text, "empty": empty, "too many pixels": huge, "too many bytes": big, "a folder": tmp, "missing": filepath.Join(tmp, "nope.png")}
	for name, p := range cases {
		if err := s.SetArt("g1", Cover, p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := s.SetArt("g1", Kind("wallpaper"), pngFile(t, tmp, "ok.png", 2, 2)); err == nil {
		t.Error("an unknown kind was accepted")
	}
	if e := s.Get("g1"); !e.empty() {
		t.Fatalf("refused art left an entry: %+v", e)
	}
	// WebP is taken by its header (the standard library has no decoder).
	webp := write("w.bin", append([]byte("RIFF\x10\x00\x00\x00WEBPVP8 "), make([]byte, 16)...))
	if err := s.SetArt("g1", Icon, webp); err != nil {
		t.Fatalf("webp refused: %v", err)
	}
	if !strings.Contains(s.URL("g1", Icon), "/icon.webp?v=") {
		t.Fatalf("webp url = %q", s.URL("g1", Icon))
	}
}

func TestResetArtAndResetAll(t *testing.T) {
	s, dir := open(t)
	src := pngFile(t, t.TempDir(), "a.png", 3, 3)
	for _, k := range []Kind{Cover, Logo} {
		if err := s.SetArt("g1", k, src); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetTitle("g1", "Mine", ""); err != nil {
		t.Fatal(err)
	}
	coverPath := s.Path("g1", Cover)
	if err := s.ResetArt("g1", Cover); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(coverPath); !os.IsNotExist(err) {
		t.Fatalf("the reset image is still on disk: %v", err)
	}
	if s.URL("g1", Cover) != "" || s.URL("g1", Logo) == "" || s.Get("g1").Title != "Mine" {
		t.Fatalf("reset touched more than the cover: %+v", s.Get("g1"))
	}
	if err := s.ResetArt("g1", Cover); err != nil { // nothing to reset is not an error
		t.Fatal(err)
	}
	if err := s.Reset("g1"); err != nil {
		t.Fatal(err)
	}
	if e := s.Get("g1"); !e.empty() {
		t.Fatalf("entry after Reset = %+v", e)
	}
	if left, _ := os.ReadDir(filepath.Join(dir, "art")); len(left) != 0 {
		t.Fatalf("files left after Reset: %v", left)
	}
	if err := s.ResetArt("g1", Kind("nope")); err == nil {
		t.Fatal("an unknown kind was accepted by ResetArt")
	}
}

func TestSnapshotDecoratesAWholeLibrary(t *testing.T) {
	s, _ := open(t)
	if err := s.SetArt("g1", Icon, pngFile(t, t.TempDir(), "a.png", 2, 2)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTitle("g1", "Mine", "sorted"); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if e := snap.Entry("g1"); e.Title != "Mine" || e.SortTitle != "sorted" {
		t.Fatalf("entry = %+v", e)
	}
	if snap.URL("g1", Icon) != s.URL("g1", Icon) || snap.URL("g2", Icon) != "" {
		t.Fatalf("urls = %q", snap.URL("g1", Icon))
	}
	// Changing the store later doesn't change the snapshot.
	_ = s.Reset("g1")
	if snap.Entry("g1").Title != "Mine" {
		t.Fatal("snapshot followed the store")
	}
}

func TestCorruptIndexIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "customization.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err == nil || s == nil {
		t.Fatalf("Open = %v, %v; want a usable store and an error", s, err)
	}
	if err := s.SetTitle("g1", "Works", ""); err != nil {
		t.Fatalf("the store is not usable: %v", err)
	}
	moved, _ := filepath.Glob(filepath.Join(dir, "customization.json.bad-*"))
	if len(moved) != 1 {
		t.Fatalf("bad files = %v", moved)
	}
}

func TestServeOnlyTheNamesTheStoreWrites(t *testing.T) {
	s, _ := open(t)
	if err := s.SetArt("g1", Cover, pngFile(t, t.TempDir(), "a.png", 2, 2)); err != nil {
		t.Fatal(err)
	}
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handled := s.Serve(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if !handled && strings.HasPrefix(path, URLPrefix) {
			t.Fatalf("%s not handled", path)
		}
		return rec
	}
	u := s.URL("g1", Cover)
	rec := get(strings.SplitN(u, "?", 2)[0])
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("serving the cover: %d %v", rec.Code, rec.Header())
	}
	k := key("g1")
	for _, bad := range []string{URLPrefix + k + "/../customization.json", URLPrefix + k + "/cover.svg", URLPrefix + "../x/cover.png", URLPrefix + k, URLPrefix + "ZZZZ/cover.png"} {
		if rec := get(bad); rec.Code != http.StatusNotFound && rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s answered %d", bad, rec.Code)
		}
	}
	if s.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)) {
		t.Fatal("handled a request that is not its own")
	}
}
