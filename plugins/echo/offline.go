package main

// Offline-launch fixtures (protocol 1.15, issue #44): per-game offline states,
// the store-synced saves flag, the local-art file, and the executable-override
// param of game.launchSpec.
//
//	tiny     offline "ok" (launches without a network)
//	huge     offline "network" (needs a connection to Echo)
//	slow     no offline key (unknown)
//	client   offline "network" (its store client needs the network)
//	licensed cloudSync (the store itself syncs this game's saves)
//	tiny     file:// cover under /tmp/platypus-echo-art (the local-art
//	         capability; plugin.json declares that folder in permissions.read)

import (
	"encoding/base64"
	"os"
	"path/filepath"
)

// echoArtDir is the folder the local-art fixture lives in. plugin.json
// declares it in permissions.read, so the core serves files under it.
const echoArtDir = "/tmp/platypus-echo-art"

// offlineFor is the offline value library.list reports for a game: "ok",
// "network", or "" (absent, meaning unknown).
func offlineFor(id string) string {
	switch id {
	case "tiny":
		return "ok"
	case "huge", "client":
		return "network"
	default:
		return ""
	}
}

// cloudSyncFor reports whether the store itself syncs the game's saves.
func cloudSyncFor(id string) bool {
	return id == "licensed" || id == "client"
}

// echoArtFile ensures the local-art fixture exists and returns its path: a
// 1x1 PNG under echoArtDir. It is written lazily so a read-only checkout
// still runs echo without it until library.list is called.
func echoArtFile() string {
	path := filepath.Join(echoArtDir, "cover.png")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if err := os.MkdirAll(echoArtDir, 0o755); err != nil {
		return ""
	}
	// A 1x1 transparent PNG.
	const png1x1 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	raw, err := base64.StdEncoding.DecodeString(png1x1)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return ""
	}
	return path
}

// echoArtURL is the file:// art URL library.list reports for a game, or ""
// for games without local art. Only tiny carries one, so the conformance
// suite sees both a rewritten URL and plain https/absent art.
func echoArtURL(id string) string {
	if id != "tiny" {
		return ""
	}
	if path := echoArtFile(); path != "" {
		return "file://" + path
	}
	return ""
}
