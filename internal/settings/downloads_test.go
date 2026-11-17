package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DownloadLimit(); got != 0 {
		t.Fatalf("default limit = %d, want unlimited (0)", got)
	}
	if err := s.SetDownloadLimit(1 << 20); err != nil {
		t.Fatal(err)
	}
	if got := s.DownloadLimit(); got != 1<<20 {
		t.Fatalf("limit = %d", got)
	}
	// It survives a restart.
	s2, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.DownloadLimit(); got != 1<<20 {
		t.Fatalf("reloaded limit = %d", got)
	}
	for _, bad := range []int64{-1, MaxDownloadLimit + 1} {
		if err := s.SetDownloadLimit(bad); err == nil {
			t.Errorf("limit %d accepted", bad)
		}
	}
	if got := s.DownloadLimit(); got != 1<<20 {
		t.Fatalf("invalid limit changed the stored one: %d", got)
	}
	// 0 is unlimited, a choice worth storing rather than dropping.
	if err := s.SetDownloadLimit(0); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f map[string]any
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if _, has := f["download_limit"]; has {
		t.Fatalf("unlimited stored an explicit 0: %s", raw)
	}
}

func TestUpdateWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := s.UpdateWindow()
	if w.Enabled || w.Start != DefaultUpdateWindowStart || w.End != DefaultUpdateWindowEnd {
		t.Fatalf("default window = %+v", w)
	}
	want := UpdateWindow{Enabled: true, Start: "22:30", End: "06:15"}
	if err := s.SetUpdateWindow(want); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.UpdateWindow(); got != want {
		t.Fatalf("reloaded window = %+v, want %+v", got, want)
	}
	for _, bad := range []UpdateWindow{
		{Start: "3:00", End: "06:00"},
		{Start: "03:00", End: "24:00"},
		{Start: "03:60", End: "06:00"},
		{Start: "", End: "06:00"},
		{Start: "aa:bb", End: "06:00"},
	} {
		if err := s.SetUpdateWindow(bad); err == nil {
			t.Errorf("window %+v accepted", bad)
		}
	}
	if got := s.UpdateWindow(); got != want {
		t.Fatalf("invalid window changed the stored one: %+v", got)
	}
}

func TestParseClock(t *testing.T) {
	for in, want := range map[string]int{"00:00": 0, "03:05": 185, "23:59": 1439} {
		if got, err := ParseClock(in); err != nil || got != want {
			t.Errorf("ParseClock(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "24:00", "3:05", "03:5", "03-05", " 03:05"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("ParseClock(%q) accepted", bad)
		}
	}
}
