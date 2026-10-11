package steam

import (
	"strings"
	"testing"
)

func TestParseLanguages(t *testing.T) {
	got := parseLanguages(`English<strong>*</strong>, French, German<br>Italian`)
	for _, want := range []string{"English", "French", "German", "Italian"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("languages %v missing %q", got, want)
		}
	}
}

func TestRequirementsHostileSanitized(t *testing.T) {
	req := requirementsOf(
		map[string]any{"minimum": `<p>2 GB RAM</p><script>alert(1)</script>`, "recommended": `<b>4 GB</b><img src=x>`},
		nil,
	)
	if req == nil || !strings.Contains(req.Minimum, "2 GB RAM") {
		t.Fatalf("requirements: %+v", req)
	}
	if strings.Contains(req.Minimum, "script") || strings.Contains(req.Recommended, "<img") {
		t.Errorf("hostile survived: %+v", req)
	}
}

func TestPickMovieSkipsHLS(t *testing.T) {
	u, mime := pickMovie(map[string]string{"max": "https://cdn/x/trailer.mp4"}, nil)
	if u == "" || mime != "video/mp4" {
		t.Errorf("mp4: %q %q", u, mime)
	}
	u, mime = pickMovie(nil, map[string]string{"480": "https://cdn/x/t.webm"})
	if mime != "video/webm" {
		t.Errorf("webm: %q %q", u, mime)
	}
	if u, _ := pickMovie(nil, nil); u != "" {
		t.Errorf("empty picked: %q", u)
	}
}

func TestRequiredAge(t *testing.T) {
	if requiredAge(float64(0)) != "" || requiredAge("0") != "" {
		t.Error("zero age shown")
	}
	if requiredAge(float64(17)) != "17+" {
		t.Error("17 not rendered")
	}
}
