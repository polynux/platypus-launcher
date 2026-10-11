package main

import (
	_ "embed"
	"fmt"
	"net/http"
)

// Media facts (protocol 1.12, docs/plugins/protocol.md, "Media"):
// game.details carries the game page's richer data and trailers. "tiny" has
// the full set by default (deterministic, served locally, no network); any
// other game has none. The description HTML deliberately carries hostile
// markup (a script tag) so the core sanitizer tests prove it never reaches
// the UI.

//go:embed testdata/trailer.mp4
var trailerMP4 []byte

func mediaFor(id string) map[string]any {
	if id != "tiny" {
		return nil
	}
	bulkOnce.Do(startCoverServer)
	if bulkBase == "" {
		return nil
	}
	return map[string]any{
		"descriptionHtml": `<p>A tiny fake game from the Echo test store, shown on its game page.</p><ul><li>Installs in a second</li><li>Runs natively</li></ul><script>alert("echo")</script><a href="https://example.com/echo-tiny">Echo Tiny site</a>`,
		"website":         "https://example.com/echo-tiny",
		"storeUrl":        "https://example.com/store/echo-tiny",
		"languages":       []string{"English", "French"},
		"controller":      "full",
		"requirements": map[string]any{
			"minimum":     "<p><strong>Minimum:</strong> 1 GHz CPU, 512 MB RAM</p>",
			"recommended": "<p><strong>Recommended:</strong> 2 GHz CPU, 1 GB RAM</p>",
		},
		"ageRating": "12+",
		"tags":      []string{"Test", "Single-player"},
		"videos": []map[string]any{
			{"url": fmt.Sprintf("%s/trailer.mp4", bulkBase),
				"thumbnail": fmt.Sprintf("%s/shot/1.jpg", bulkBase),
				"title":     "Echo Tiny trailer", "mime": "video/mp4"},
		},
	}
}

func serveTrailer(mux *http.ServeMux) {
	mux.HandleFunc("/trailer.mp4", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(trailerMP4)
	})
}
