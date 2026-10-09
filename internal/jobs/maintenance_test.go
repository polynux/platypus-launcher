package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// A verify or move job holds its game like any other, runs through the same
// queue, and never asks the plugin to clean up data it doesn't have.

func TestVerifyJobKeepsItsReport(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != KindVerify || j.Repair {
		t.Fatalf("queued %+v", j)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, j.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID, Verify: &types.VerifyReport{Checked: 12, Damaged: 3}})
	got := waitState(t, q, j.ID, StateDone)
	if got.Verify == nil || got.Verify.Damaged != 3 || got.Verify.Checked != 12 {
		t.Fatalf("report = %+v", got.Verify)
	}
}

func TestVerifyReportFromAnotherPluginIsIgnored(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, j.ID, StateRunning)
	q.Notify("steam", "job.done", plugins.JobUpdate{JobID: j.ID, Verify: &types.VerifyReport{Damaged: 9}})
	if got := waitState(t, q, j.ID, StateRunning); got.Verify != nil {
		t.Fatalf("another plugin set the report: %+v", got.Verify)
	}
}

func TestOneJobPerGameCoversVerifyAndMove(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	if _, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", dir, filepath.Join(t.TempDir(), "Tiny")); err == nil {
		t.Fatal("a move was queued next to a verify of the same game")
	}
}

func TestMoveNeedsAbsoluteFolders(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", "relative", "/tmp/x"); err == nil {
		t.Fatal("relative source accepted")
	}
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/x", "relative"); err == nil {
		t.Fatal("relative destination accepted")
	}
	if _, err := q.AddVerify("echo", "g", "tiny", "Tiny", "relative", false); err == nil {
		t.Fatal("relative verify folder accepted")
	}
}

func TestCanceledMoveHasNoDataForThePluginToDiscard(t *testing.T) {
	clock := newClock()
	d := &discardBackend{fakeBackend: &fakeBackend{}}
	q := newCleanupQueue(t, d, clock)
	from := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddMove("echo", "g", "tiny", "Tiny", from, filepath.Join(t.TempDir(), "Tiny"))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	clock.advance(30 * 24 * time.Hour)
	q.Cleanup(context.Background())
	if n := len(d.discards()); n != 0 {
		t.Fatalf("a canceled move asked the plugin to discard %d times", n)
	}
	if got := q.List(); len(got) != 1 || got[0].CleanedAt != 0 {
		t.Fatalf("canceled move changed: %+v", got)
	}
}

func TestSetLeftoverKeepsTheDoneJob(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	j, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/a", "/tmp/b")
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	q.SetLeftover(j.ID, "/tmp/a")
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID})
	got := waitState(t, q, j.ID, StateDone)
	if got.Leftover != "/tmp/a" {
		t.Fatalf("leftover = %q", got.Leftover)
	}
}
