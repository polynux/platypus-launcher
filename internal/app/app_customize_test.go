package app

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"platypus/internal/customize"
	"platypus/internal/types"
)

func customApp(t *testing.T) *App {
	t.Helper()
	a := &App{}
	if err := a.openCustomization(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	return a
}

func writePNG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 6, 9))); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "mine.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCustomizationReachesTheLibraryAndResets(t *testing.T) {
	t.Parallel()
	a := customApp(t)
	src := writePNG(t)
	if _, err := a.SetCustomTitle("g1", "My Title", "title, my"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"cover", "banner", "logo", "icon", "background"} {
		if _, err := a.SetCustomArt("g1", k, src); err != nil {
			t.Fatalf("%s: %v", k, err)
		}
	}
	games := []types.Game{
		{ID: "g1", Title: "Store Title", CoverURL: "https://cdn/c.jpg", BannerURL: "https://cdn/b.jpg", HeroURL: "https://cdn/h.jpg"},
		{ID: "g2", Title: "Untouched", CoverURL: "https://cdn/x.jpg"},
	}
	a.applyCustomization(games)
	g := games[0]
	if g.Title != "My Title" || g.StoreTitle != "Store Title" || g.OriginalTitle() != "Store Title" || g.SortTitle != "title, my" {
		t.Fatalf("titles = %q / %q / %q", g.Title, g.StoreTitle, g.SortTitle)
	}
	for name, u := range map[string]string{"cover": g.CoverURL, "banner": g.BannerURL, "hero": g.HeroURL, "logo": g.LogoURL, "icon": g.IconURL, "background": g.BackgroundURL} {
		if !strings.HasPrefix(u, customize.URLPrefix) {
			t.Errorf("%s = %q, want a Platypus art url", name, u)
		}
	}
	if want := []string{"title", "sort_title", "cover", "banner", "logo", "icon", "background"}; !slices.Equal(g.Customized, want) {
		t.Fatalf("customized = %v", g.Customized)
	}
	if o := games[1]; o.Title != "Untouched" || o.CoverURL != "https://cdn/x.jpg" || o.Customized != nil || o.StoreTitle != "" || o.OriginalTitle() != "Untouched" {
		t.Fatalf("an uncustomized game changed: %+v", o)
	}

	// Reset one kind: the store's art is back for it only.
	if _, err := a.ResetCustomArt("g1", "cover"); err != nil {
		t.Fatal(err)
	}
	games = []types.Game{{ID: "g1", Title: "Store Title", CoverURL: "https://cdn/c.jpg"}}
	a.applyCustomization(games)
	if games[0].CoverURL != "https://cdn/c.jpg" || games[0].LogoURL == "" {
		t.Fatalf("after resetting the cover: %+v", games[0])
	}
	// Reset everything.
	c, err := a.ResetCustomization("g1")
	if err != nil || c.Title != "" || len(c.Art) != 0 {
		t.Fatalf("ResetCustomization = %+v, %v", c, err)
	}
	games = []types.Game{{ID: "g1", Title: "Store Title"}}
	a.applyCustomization(games)
	if games[0].Title != "Store Title" || games[0].Customized != nil {
		t.Fatalf("after reset: %+v", games[0])
	}
}

func TestCustomizationRefusesBadInput(t *testing.T) {
	t.Parallel()
	a := customApp(t)
	text := filepath.Join(t.TempDir(), "notes.png")
	if err := os.WriteFile(text, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetCustomArt("g1", "cover", text); err == nil {
		t.Error("a text file was taken as art")
	}
	if _, err := a.SetCustomArt("g1", "wallpaper", writePNG(t)); err == nil {
		t.Error("an unknown kind was taken")
	}
	if _, err := a.SetCustomTitle("g1", "x\ty\x07", ""); err == nil {
		t.Error("a title with a control character was taken")
	}
	if c, err := a.GetCustomization("g1"); err != nil || c.Title != "" || len(c.Art) != 0 {
		t.Fatalf("a refusal left something: %+v %v", c, err)
	}
	if _, err := (&App{}).GetCustomization("g1"); err == nil {
		t.Error("no store: want an error, not a panic")
	}
}

func TestCustomArtIsServedAndOthersPassThrough(t *testing.T) {
	t.Parallel()
	a := customApp(t)
	c, err := a.SetCustomArt("g1", "cover", writePNG(t))
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Next", "1") })
	h := a.customArt(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, strings.SplitN(c.Art["cover"], "?", 2)[0], nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Next") != "" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("art request: %d %v", rec.Code, rec.Header())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if rec.Header().Get("X-Next") != "1" {
		t.Fatal("a request that is not art did not reach the next handler")
	}
	// Before startup there is no store: everything passes through.
	rec = httptest.NewRecorder()
	(&App{}).customArt(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, customize.URLPrefix+"x", nil))
	if rec.Header().Get("X-Next") != "1" {
		t.Fatal("no store: the request should pass through")
	}
}

// A plugin only ever hears the store's title: a custom one must not reach the
// job (the plugin keys records on it) or the cloud context.
func TestPluginsAreSentTheStoreTitle(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, allVersions, echoCaps)
	e.game.StoreTitle, e.game.Title = "Echo Tiny", "My Tiny"
	j, err := e.a.InstallGame(e.game)
	if err != nil {
		t.Fatal(err)
	}
	if j.Title != "Echo Tiny" {
		t.Fatalf("job title = %q, want the store's", j.Title)
	}
	if g, ok := e.a.cloudGame(e.game); ok && g.Title != "Echo Tiny" {
		t.Fatalf("cloud title = %q, want the store's", g.Title)
	}
}
