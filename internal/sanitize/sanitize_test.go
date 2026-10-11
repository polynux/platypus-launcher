package sanitize

import (
	"strings"
	"testing"
)

func TestHostileInputs(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		absent []string
	}{
		{"script", `<p>hi</p><script>alert(1)</script>`, []string{"<script", "alert(1)"}},
		{"iframe", `watch <iframe src="https://evil/x"></iframe> now`, []string{"<iframe"}},
		{"img onerror", `<img src=x onerror=alert(1)>hi`, []string{"<img", "alert"}},
		{"style", `<style>p{color:red}</style><p>ok</p>`, []string{"<style"}},
		{"js link", `<a href="javascript:alert(1)">click</a>`, []string{"javascript", "href"}},
		{"http link stripped", `<a href="http://example.com">click</a>`, []string{"href"}},
		{"onclick", `<p onclick="alert(1)">hi</p>`, []string{"onclick"}},
		{"comment", `a<!-- secret -->b`, []string{"secret", "<!--"}},
	}
	for _, c := range cases {
		out := HTML(c.in)
		for _, bad := range c.absent {
			if strings.Contains(out, bad) {
				t.Errorf("%s: output %q contains %q", c.name, out, bad)
			}
		}
	}
}

func TestAllowlistKept(t *testing.T) {
	out := HTML(`<p>Hello <b>world</b></p><ul><li>one</li><li>two</li></ul><h2>Title</h2><a href="https://example.com/x">link</a><br>`)
	for _, want := range []string{"<p>", "<b>", "</b>", "<ul>", "<li>", "<h2>", `<a href="https://example.com/x">`, "<br>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestTextEscaped(t *testing.T) {
	out := HTML(`5 < 6 & "quoted"`)
	if strings.Contains(out, "5 < 6") || !strings.Contains(out, "5 &lt; 6") {
		t.Errorf("text not escaped: %q", out)
	}
}

func TestMediaURL(t *testing.T) {
	if MediaURL("https://cdn.example/v.mp4") == "" {
		t.Error("https dropped")
	}
	if MediaURL("http://example.com/v.mp4") != "" {
		t.Error("plain http kept")
	}
	if MediaURL("http://127.0.0.1:1234/shot/1.jpg") == "" {
		t.Error("loopback dropped")
	}
	if MediaURL("file:///etc/passwd") != "" || MediaURL("javascript:alert(1)") != "" {
		t.Error("dangerous scheme kept")
	}
	if HTTPSURL("http://example.com/") != "" || HTTPSURL("https://example.com/") == "" {
		t.Error("HTTPSURL wrong")
	}
}
