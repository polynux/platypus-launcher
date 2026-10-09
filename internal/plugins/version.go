package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version is a protocol version, major.minor. A minor bump is additive only
// (new methods, optional fields, capabilities, error codes); a major bump may
// break. See "Versioning and compatibility" in docs/plugins/protocol.md.
type Version struct{ Major, Minor int }

// ParseVersion reads "major.minor".
func ParseVersion(s string) (Version, error) {
	maj, min, ok := strings.Cut(s, ".")
	a, errA := strconv.ParseUint(maj, 10, 16)
	b, errB := strconv.ParseUint(min, 10, 16)
	if !ok || errA != nil || errB != nil || strconv.FormatUint(a, 10) != maj || strconv.FormatUint(b, 10) != min {
		return Version{}, fmt.Errorf("version %q must look like 1.0 (major.minor)", s)
	}
	return Version{int(a), int(b)}, nil
}

func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// Less orders by major, then minor.
func (v Version) Less(o Version) bool {
	return v.Major < o.Major || (v.Major == o.Major && v.Minor < o.Minor)
}

// VersionSet is the protocol versions a plugin speaks: an explicit list
// (["1.0", "1.2"]) or an inclusive range ({"min": "1.0", "max": "1.3"}; a
// missing max is open-ended).
type VersionSet struct {
	List     []Version
	Min, Max *Version
}

// Contains reports whether the plugin speaks v.
func (s VersionSet) Contains(v Version) bool {
	if s.Min == nil && s.Max == nil {
		for _, x := range s.List {
			if x == v {
				return true
			}
		}
		return false
	}
	// A half-open range stays inside its bound's major: {"min":"1.0"} means
	// "1.0 and later 1.x", never a breaking 2.0.
	if s.Max == nil && v.Major != s.Min.Major {
		return false
	}
	if s.Min == nil && v.Major != s.Max.Major {
		return false
	}
	return (s.Min == nil || !v.Less(*s.Min)) && (s.Max == nil || !s.Max.Less(v))
}

func (s VersionSet) empty() bool { return s.Min == nil && s.Max == nil && len(s.List) == 0 }

// Highest is the newest version the set names, and false when a range has no
// upper bound.
func (s VersionSet) Highest() (Version, bool) {
	if s.Min != nil || s.Max != nil {
		if s.Max == nil {
			return Version{}, false
		}
		return *s.Max, true
	}
	var top Version
	for _, x := range s.List {
		if top.Less(x) {
			top = x
		}
	}
	return top, len(s.List) > 0
}

func (s VersionSet) String() string {
	switch {
	case s.Min != nil || s.Max != nil:
		lo, hi := "any", "any"
		if s.Min != nil {
			lo = s.Min.String()
		}
		if s.Max != nil {
			hi = s.Max.String()
		}
		return lo + "–" + hi
	default:
		parts := make([]string, len(s.List))
		for i, x := range s.List {
			parts[i] = x.String()
		}
		return strings.Join(parts, ", ")
	}
}

func (s *VersionSet) UnmarshalJSON(data []byte) error {
	*s = VersionSet{}
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		if len(list) == 0 {
			return errors.New("versions is empty")
		}
		for _, str := range list {
			v, err := ParseVersion(str)
			if err != nil {
				return err
			}
			s.List = append(s.List, v)
		}
		return nil
	}
	var rng struct {
		Min string `json:"min"`
		Max string `json:"max"`
	}
	if err := json.Unmarshal(data, &rng); err != nil {
		return errors.New(`versions must be a list like ["1.0", "1.1"] or a range like {"min": "1.0", "max": "1.3"}`)
	}
	if rng.Min == "" && rng.Max == "" {
		return errors.New("a versions range needs min, max or both")
	}
	if rng.Min != "" {
		v, err := ParseVersion(rng.Min)
		if err != nil {
			return err
		}
		s.Min = &v
	}
	if rng.Max != "" {
		v, err := ParseVersion(rng.Max)
		if err != nil {
			return err
		}
		s.Max = &v
	}
	if s.Min != nil && s.Max != nil && s.Max.Less(*s.Min) {
		return fmt.Errorf("versions range %s–%s is empty", s.Min, s.Max)
	}
	return nil
}

func (s VersionSet) MarshalJSON() ([]byte, error) {
	if s.Min != nil || s.Max != nil {
		rng := map[string]string{}
		if s.Min != nil {
			rng["min"] = s.Min.String()
		}
		if s.Max != nil {
			rng["max"] = s.Max.String()
		}
		return json.Marshal(rng)
	}
	list := make([]string, len(s.List))
	for i, x := range s.List {
		list[i] = x.String()
	}
	return json.Marshal(list)
}

// SupportedProtocols are the protocol versions this Hydra speaks, oldest
// first. The first entry is the oldest major kept alive; Hydra keeps
// supporting the previous major for a deprecation period after a new one.
//
// 1.2 adds cloud saves; 1.3 adds versions; 1.4 adds verify and move (and
// game.relocated); 1.5 adds install options and DLC.
var SupportedProtocols = []Version{{1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4}, {1, 5}}

// Negotiate picks the highest version both sides speak.
func Negotiate(offered VersionSet, supported []Version) (Version, bool) {
	var best Version
	found := false
	for _, v := range supported {
		if offered.Contains(v) && (!found || best.Less(v)) {
			best, found = v, true
		}
	}
	return best, found
}
