package runner

import (
	"regexp"
	"strconv"
	"strings"

	"platypus/internal/types"
)

// IsAuto reports whether a runner ID means "let Platypus pick".
func IsAuto(id string) bool {
	return id == "" || strings.EqualFold(id, "auto")
}

var numRe = regexp.MustCompile(`\d+`)

// versionKey extracts the numeric parts of a runner name: "GE-Proton10-34" → [10 34].
func versionKey(name string) []int {
	var out []int
	for _, m := range numRe.FindAllString(name, -1) {
		n, _ := strconv.Atoi(m)
		out = append(out, n)
	}
	return out
}

func lessVersion(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// rank orders runner families for automatic selection: GE-Proton carries the
// most game fixes, then Valve's Experimental, then numbered Proton releases.
// Wine builds come last since they lack Proton's Steam integration.
func rank(r types.ProtonVersion) int {
	n := strings.ToLower(r.Name)
	switch {
	case strings.HasPrefix(n, "ge-proton") || strings.Contains(n, "proton-ge"):
		return 4
	case strings.Contains(n, "experimental"):
		return 3
	case strings.HasPrefix(n, "proton") && !strings.Contains(n, "hotfix") && !strings.Contains(n, "beta"):
		return 2
	case strings.HasSuffix(r.Path, "/proton"):
		return 1
	}
	return 0
}

// PickAuto returns the runner "auto" resolves to: defaultID when that runner is
// still installed (issue #146), else the best one. Within a family the highest
// version wins, so GE-Proton10-34 beats GE-Proton10-32. Builds with a suffix
// after the version (e.g. "-rtsp23-4" test builds) are skipped when a plain
// release of the same family exists. The umu launcher entry is skipped: it is
// not a Proton/Wine build (auto prefers it separately, see SelectUmu), so a
// default of umu runs what auto runs.
func PickAuto(runners []types.ProtonVersion, defaultID string) (types.ProtonVersion, bool) {
	if !IsAuto(defaultID) {
		for _, r := range runners {
			if (r.ID == defaultID || r.Name == defaultID) && !IsUmu(r) {
				return r, true
			}
		}
	}
	best := -1
	for i, r := range runners {
		if IsUmu(r) {
			continue
		}
		if best < 0 || better(r, runners[best]) {
			best = i
		}
	}
	if best < 0 {
		return types.ProtonVersion{}, false
	}
	return runners[best], true
}

var plainGE = regexp.MustCompile(`(?i)^ge-proton\d+-\d+( \(|$)`)

func better(a, b types.ProtonVersion) bool {
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra > rb
	}
	if ra == 4 {
		pa, pb := plainGE.MatchString(a.Name), plainGE.MatchString(b.Name)
		if pa != pb {
			return pa
		}
	}
	return lessVersion(versionKey(b.Name), versionKey(a.Name))
}

// Resolve maps a game's runner setting to an installed runner. "auto" (or
// none) resolves to defaultID when installed, else the automatic pick.
func Resolve(runnerID string, runners []types.ProtonVersion, defaultID string) (types.ProtonVersion, bool) {
	if IsAuto(runnerID) {
		return PickAuto(runners, defaultID)
	}
	for _, r := range runners {
		if r.ID == runnerID || r.Name == runnerID {
			return r, true
		}
	}
	return types.ProtonVersion{}, false
}

// MarkDefault sets IsDefault on the runner "auto" resolves to (issue #146):
// defaultID when it is installed, the umu entry included (auto runs through it
// when it is installed, see SelectUmu), else the automatic pick. It clears any
// earlier mark.
func MarkDefault(versions []types.ProtonVersion, defaultID string) {
	for i := range versions {
		versions[i].IsDefault = false
	}
	if !IsAuto(defaultID) {
		for i := range versions {
			if versions[i].ID == defaultID || versions[i].Name == defaultID {
				versions[i].IsDefault = true
				return
			}
		}
	}
	if pick, ok := PickAuto(versions, ""); ok {
		for i := range versions {
			versions[i].IsDefault = versions[i].ID == pick.ID
		}
	}
}
