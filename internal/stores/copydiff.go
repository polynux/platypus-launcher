package stores

import (
	"slices"

	"platypus/internal/types"
)

// compareCopies sets the copies of a borrowed game against a baseline: the
// preferred copy, or the copy with the most add-ons when none is preferred
// (ties go to the lower owner id). It needs two copies; with fewer there is
// nothing to compare, and nil comes back. The baseline itself has no entry.
func compareCopies(copies []types.GameCopy, preferred string) *types.CopyComparison {
	if len(copies) < 2 {
		return nil
	}
	base := slices.IndexFunc(copies, func(c types.GameCopy) bool { return c.OwnerID == preferred })
	if base < 0 {
		base = 0
		for i, c := range copies {
			b := copies[base]
			if len(c.DLC) > len(b.DLC) || (len(c.DLC) == len(b.DLC) && c.OwnerID < b.OwnerID) {
				base = i
			}
		}
	}
	out := &types.CopyComparison{Baseline: copies[base].OwnerID, Diffs: []types.CopyDiff{}}
	for i, c := range copies {
		if i == base {
			continue
		}
		out.Diffs = append(out.Diffs, diffCopy(c, copies[base]))
	}
	return out
}

// diffCopy counts the add-ons of copy c against the baseline b, by id. Each
// add-on is counted once: on one side only, or on both.
func diffCopy(c, b types.GameCopy) types.CopyDiff {
	d := types.CopyDiff{OwnerID: c.OwnerID}
	for _, x := range c.DLC {
		soundtrack := x.Kind == "soundtrack"
		switch {
		case hasDLC(b.DLC, x.ID) && soundtrack:
			d.SoundtracksShared++
		case hasDLC(b.DLC, x.ID):
			d.Shared++
		case soundtrack:
			d.SoundtracksOnlyHere++
		default:
			d.OnlyHere++
		}
	}
	for _, x := range b.DLC {
		if hasDLC(c.DLC, x.ID) {
			continue
		}
		if x.Kind == "soundtrack" {
			d.SoundtracksOnlyBaseline++
		} else {
			d.OnlyBaseline++
		}
	}
	return d
}

func hasDLC(list []types.GameDLC, id string) bool {
	return slices.ContainsFunc(list, func(d types.GameDLC) bool { return d.ID == id })
}
