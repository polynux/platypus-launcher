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
	got, ok := PickAuto(rs)
	if !ok || got.ID != "compat:GE-Proton11-3" {
		t.Fatalf("got %q", got.ID)
	}
	got, _ = PickAuto(rs[:2])
	if got.ID != "steam:Proton - Experimental" {
		t.Fatalf("got %q", got.ID)
	}
	if _, ok := Resolve("auto", nil); ok {
		t.Fatal("auto with no runners must fail")
	}
	if r, ok := Resolve("", rs); !ok || r.ID != "compat:GE-Proton11-3" {
		t.Fatalf("empty id should resolve to auto, got %q", r.ID)
	}
}
