package settings

import (
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

func TestPinsPersistAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if _, ok := s.Pin("g"); ok {
		t.Fatal("a game starts unpinned")
	}
	if err := s.SetPin("g", types.VersionPin{Kind: types.PinBuild, ID: "b200"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPin("h", types.VersionPin{Kind: types.PinChannel, ID: "legacy"}); err != nil {
		t.Fatal(err)
	}
	again, _ := Open(path)
	if p, ok := again.Pin("g"); !ok || p.Kind != types.PinBuild || p.ID != "b200" {
		t.Fatalf("pin after reload = %+v, %v", p, ok)
	}
	if len(again.Pins()) != 2 {
		t.Fatalf("pins = %v", again.Pins())
	}
	if err := again.ClearPin("g"); err != nil {
		t.Fatal(err)
	}
	if err := again.ClearPin("g"); err != nil {
		t.Fatalf("clearing an unset pin is not an error: %v", err)
	}
	third, _ := Open(path)
	if _, ok := third.Pin("g"); ok {
		t.Fatal("pin survived the clear")
	}
	if _, ok := third.Pin("h"); !ok {
		t.Fatal("clearing one game dropped another's pin")
	}
}

func TestSetPinRefusesNonsense(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	bad := []struct {
		game string
		pin  types.VersionPin
	}{
		{"", types.VersionPin{Kind: "build", ID: "b1"}},
		{"g", types.VersionPin{Kind: "latest", ID: "b1"}},
		{"g", types.VersionPin{Kind: "build"}},
		{"g", types.VersionPin{Kind: "build", ID: strings.Repeat("x", maxPinID+1)}},
	}
	for _, c := range bad {
		if err := s.SetPin(c.game, c.pin); err == nil {
			t.Errorf("SetPin(%q, %+v) succeeded", c.game, c.pin)
		}
	}
	if len(s.Pins()) != 0 {
		t.Fatalf("refused pins were stored: %v", s.Pins())
	}
}

func TestPinSavedAlongsideOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if err := s.SetGameSettings("g", map[string]any{"runner_id": "auto"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPin("g", types.VersionPin{Kind: types.PinBuild, ID: "b1"}); err != nil {
		t.Fatal(err)
	}
	again, _ := Open(path)
	if again.GameSettings()["g"]["runner_id"] != "auto" {
		t.Fatal("saving a pin lost the game's settings")
	}
}
