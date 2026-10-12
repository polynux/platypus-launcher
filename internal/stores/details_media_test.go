package stores

import (
	"strings"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/sanitize"
)

func TestMediaVersionKnown(t *testing.T) {
	if last := plugins.SupportedProtocols[len(plugins.SupportedProtocols)-1]; last != plugins.UnreadCountsVersion {
		t.Fatalf("SupportedProtocols %v does not end at %v", plugins.SupportedProtocols, plugins.UnreadCountsVersion)
	}
	if (plugins.Runtime{Protocol: "1.12"}).Speaks(plugins.MediaVersion) != true {
		t.Error("1.12 should speak MediaVersion")
	}
	if (plugins.Runtime{Protocol: "1.11"}).Speaks(plugins.MediaVersion) {
		t.Error("1.11 should not speak MediaVersion")
	}
}

func TestVideoMIME(t *testing.T) {
	if got := videoMIME("video/mp4", "https://x/v"); got != "video/mp4" {
		t.Errorf("mp4: %q", got)
	}
	if got := videoMIME("", "https://cdn.example/t.webm?x=1"); got != "video/webm" {
		t.Errorf("webm guess: %q", got)
	}
	if got := videoMIME("video/ogg", "https://x/v.ogg"); got != "" {
		t.Errorf("ogg kept: %q", got)
	}
	if got := videoMIME("", "https://x/stream.m3u8"); got != "" {
		t.Errorf("hls guessed: %q", got)
	}
}

func TestMediaValidation(t *testing.T) {
	// Hostile HTML never survives the sanitizer used for description/requirements.
	out := sanitize.HTML(`<script>alert(1)</script><p>ok <a href="javascript:x">y</a></p><img src=x>`)
	if strings.Contains(out, "alert") || strings.Contains(out, "<script") || strings.Contains(out, "<img") {
		t.Errorf("hostile HTML survived: %q", out)
	}
	// URLs: https or loopback only.
	if sanitize.MediaURL("https://cdn.example/v.mp4") == "" {
		t.Error("https video dropped")
	}
	if sanitize.MediaURL("http://cdn.example/v.mp4") != "" {
		t.Error("plain http video kept")
	}
	if sanitize.HTTPSURL("http://example.com/") != "" {
		t.Error("http website kept")
	}
}
