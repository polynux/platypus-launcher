package jobs

import (
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

func TestWindowOpen(t *testing.T) {
	at := func(hh, mm int) int { return hh*60 + mm }
	cases := []struct {
		start, end, minute int
		want               bool
	}{
		// Same day, inclusive start, exclusive end.
		{at(3, 0), at(6, 0), at(2, 59), false},
		{at(3, 0), at(6, 0), at(3, 0), true},
		{at(3, 0), at(6, 0), at(5, 59), true},
		{at(3, 0), at(6, 0), at(6, 0), false},
		// Overnight.
		{at(22, 0), at(6, 0), at(21, 59), false},
		{at(22, 0), at(6, 0), at(22, 0), true},
		{at(22, 0), at(6, 0), at(0, 0), true},
		{at(22, 0), at(6, 0), at(5, 59), true},
		{at(22, 0), at(6, 0), at(6, 0), false},
		{at(22, 0), at(6, 0), at(12, 0), false},
		// Equal times are all day.
		{at(3, 0), at(3, 0), at(0, 0), true},
		{at(3, 0), at(3, 0), at(15, 0), true},
	}
	for _, c := range cases {
		if got := WindowOpen(c.start, c.end, c.minute); got != c.want {
			t.Errorf("WindowOpen(%d, %d, %d) = %v, want %v", c.start, c.end, c.minute, got, c.want)
		}
	}
}

func TestNextWindowOpen(t *testing.T) {
	loc := time.Local
	at := func(hh, mm int) time.Time { return time.Date(2026, 11, 17, hh, mm, 0, 0, loc) }
	// Closed before today's start: it opens today.
	if got, want := NextWindowOpen(3*60, 6*60, at(1, 0)), at(3, 0); !got.Equal(want) {
		t.Errorf("before window: %v, want %v", got, want)
	}
	// Closed after today's end: tomorrow.
	if got, want := NextWindowOpen(3*60, 6*60, at(10, 0)), at(3, 0).AddDate(0, 0, 1); !got.Equal(want) {
		t.Errorf("after window: %v, want %v", got, want)
	}
	// Overnight, closed in the morning: it opens tonight.
	if got, want := NextWindowOpen(22*60, 6*60, at(10, 0)), at(22, 0); !got.Equal(want) {
		t.Errorf("overnight closed: %v, want %v", got, want)
	}
}

// An update queued outside the window waits; an install behind it starts.
func TestUpdateWaitsForWindow(t *testing.T) {
	b := &fakeBackend{size: plugins.InstallSize{DownloadBytes: 100, DiskBytes: 200}}
	q, _ := newQueue(t, b)
	now := time.Date(2026, 11, 17, 10, 0, 0, 0, time.Local) // outside 03:00–06:00
	q.SetClock(func() time.Time { return now })
	q.SetUpdateWindow(settings.UpdateWindow{Enabled: true, Start: "03:00", End: "06:00"})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	up, err := q.Add(KindUpdate, "echo", "p.echo.tiny", "tiny", "Tiny", dir)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := q.Add(KindInstall, "echo", "p.echo.other", "other", "Other", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitState(t, q, inst.ID, StateRunning); got.Kind != KindInstall {
		t.Fatalf("started job = %+v", got)
	}
	// The update stayed queued while the install ran.
	for _, j := range q.List() {
		if j.ID == up.ID && j.State != StateQueued {
			t.Fatalf("update state = %s, want queued", j.State)
		}
	}
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: inst.ID})
	waitState(t, q, inst.ID, StateDone)
	// Inside the window it starts.
	now = time.Date(2026, 11, 17, 4, 0, 0, 0, time.Local)
	q.SetUpdateWindow(settings.UpdateWindow{Enabled: true, Start: "03:00", End: "06:00"})
	if got := waitState(t, q, up.ID, StateRunning); got.Kind != KindUpdate {
		t.Fatalf("window job = %+v", got)
	}
}

// With the window off, an update starts right away.
func TestUpdateStartsWithWindowOff(t *testing.T) {
	b := &fakeBackend{size: plugins.InstallSize{DownloadBytes: 100, DiskBytes: 200}}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	up, err := q.Add(KindUpdate, "echo", "p.echo.tiny", "tiny", "Tiny", dir)
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, up.ID, StateRunning)
}
