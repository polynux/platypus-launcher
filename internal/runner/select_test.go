package runner

import (
	"testing"

	"platypus/internal/types"
)

func TestPickAuto(t *testing.T) {
	rs := []types.ProtonVersion{
		{ID: "steam:Proton - Experimental", Name: "Proton - Experimental", Path: "/x/proton"},
		{ID: "steam:Proton 11.0", Name: "Proton 11.0", Path: "/x/proton"},
		{ID: "compat:GE-Proton10-32", Name: "GE-Proton10-32 (Custom/GE)", Path: "/x/proton"},
		{ID: "compat:GE-Proton10-33-rtsp23-4", Name: "GE-Proton10-33-rtsp23-4 (Custom/GE)", Path: "/x/proton"},
		{ID: "compat:GE-Proton11-3", Name: "GE-Proton11-3 (Custom/GE)", Path: "/x/proton"},
		{ID: "compat:GE-Proton10-34", Name: "GE-Proton10-34 (Custom/GE)", Path: "/x/proton"},
		{ID: "system:wine", Name: "System Wine", Path: "/usr/bin/wine"},
	}
	got, ok := PickAuto(rs, "")
	if !ok || got.ID != "compat:GE-Proton11-3" {
		t.Fatalf("got %q", got.ID)
	}
	got, _ = PickAuto(rs[:2], "")
	if got.ID != "steam:Proton - Experimental" {
		t.Fatalf("got %q", got.ID)
	}
	if _, ok := Resolve("auto", nil, ""); ok {
		t.Fatal("auto with no runners must fail")
	}
	if r, ok := Resolve("", rs, ""); !ok || r.ID != "compat:GE-Proton11-3" {
		t.Fatalf("empty id should resolve to auto, got %q", r.ID)
	}
}

// The user's default runner wins the automatic pick while it is installed
// (issue #146); a default that is gone, or the umu entry, leaves the pick as
// it was.
func TestPickAutoFollowsTheDefaultRunner(t *testing.T) {
	rs := []types.ProtonVersion{
		{ID: "compat:GE-Proton11-3", Name: "GE-Proton11-3 (Custom/GE)", Path: "/x/proton"},
		{ID: "steam:Proton 9.0", Name: "Proton 9.0", Path: "/y/proton"},
		{ID: "system:wine", Name: "System Wine", Path: "/usr/bin/wine"},
	}
	if got, ok := PickAuto(rs, "steam:Proton 9.0"); !ok || got.ID != "steam:Proton 9.0" {
		t.Fatalf("PickAuto with a default = %q, want the default", got.ID)
	}
	if r, ok := Resolve("", rs, "steam:Proton 9.0"); !ok || r.ID != "steam:Proton 9.0" {
		t.Fatalf("Resolve(auto) = %q, want the default", r.ID)
	}
	// An uninstalled default falls back to the automatic pick.
	if got, _ := PickAuto(rs, "steam:Proton 4.11"); got.ID != "compat:GE-Proton11-3" {
		t.Fatalf("uninstalled default = %q, want the automatic pick", got.ID)
	}
	umu := UmuEntry("/usr/bin/umu-run")
	if got, _ := PickAuto(append(rs, umu), umu.ID); got.ID != "compat:GE-Proton11-3" {
		t.Fatalf("default of umu = %q, want the automatic pick (umu runs what auto runs)", got.ID)
	}
	if got, _ := PickAuto(append(rs, umu), "auto"); got.ID != "compat:GE-Proton11-3" {
		t.Fatalf("default of auto = %q, want the automatic pick", got.ID)
	}
}

// The runner the list marks default follows the setting, the umu entry
// included: it is what an automatic game runs when it is installed
// (issue #146). Without a usable default the automatic pick is marked.
func TestMarkDefaultFollowsTheSetting(t *testing.T) {
	proton := types.ProtonVersion{ID: "compat:GE-Proton11-3", Name: "GE-Proton11-3 (Custom/GE)", Path: "/x/proton"}
	umu := UmuEntry("/usr/bin/umu-run")
	marked := func(versions []types.ProtonVersion) string {
		found := ""
		for _, r := range versions {
			if r.IsDefault {
				if found != "" {
					t.Fatalf("more than one runner marked default: %q and %q", found, r.ID)
				}
				found = r.ID
			}
		}
		return found
	}

	versions := []types.ProtonVersion{proton, umu}
	MarkDefault(versions, umu.ID)
	if got := marked(versions); got != umu.ID {
		t.Fatalf("default umu marks %q, want the umu entry", got)
	}
	// Switching the pick clears the old mark.
	MarkDefault(versions, proton.ID)
	if got := marked(versions); got != proton.ID {
		t.Fatalf("default Proton marks %q", got)
	}
	// An uninstalled default, and the automatic pick, mark the auto pick.
	MarkDefault(versions, "steam:Proton 4.11")
	if got := marked(versions); got != proton.ID {
		t.Fatalf("uninstalled default marks %q, want the automatic pick", got)
	}
	MarkDefault(versions, "")
	if got := marked(versions); got != proton.ID {
		t.Fatalf("automatic marks %q, want the automatic pick", got)
	}
}
