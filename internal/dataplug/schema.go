// Package dataplug loads data plugins: fix recipes and tweak packs with no
// executable. They contain no code, so Hydra applies them without asking the
// user to approve anything, through the same paths as its built-in fixes and
// tweaks. See docs/plugins/data-plugins.md.
package dataplug

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

// DefaultFiles are the data files a plugin uses when its manifest lists none.
var DefaultFiles = []string{"fixes.json", "tweaks.json"}

// Fix is one curated fix recipe. It matches a game by title or by store app
// id; the first match in load order wins.
type Fix struct {
	Title        string            `json:"title,omitempty"`
	AppID        string            `json:"appId,omitempty"`
	Source       string            `json:"source"`
	Tier         string            `json:"tier"`
	Description  string            `json:"description"`
	Winetricks   []string          `json:"winetricks,omitempty"`
	DLLOverrides map[string]string `json:"dllOverrides,omitempty"`
	LaunchArgs   string            `json:"launchArgs,omitempty"`
}

// Tweak is a launch setting offered when the machine matches When.
type Tweak struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"` // "recommended" | "troubleshoot"
	Title    string            `json:"title"`
	Reason   string            `json:"reason"`
	When     When              `json:"when"`
	Env      map[string]string `json:"env,omitempty"`
	Wrappers []string          `json:"wrappers,omitempty"` // "gamemode", "mangohud"
}

// When says which machines a tweak applies to. Every field that is set must
// hold; an empty When applies to every machine.
type When struct {
	GPU      []string `json:"gpu,omitempty"` // any of these vendors is present
	NTSync   *bool    `json:"ntsync,omitempty"`
	GameMode *bool    `json:"gamemode,omitempty"`
	MangoHud *bool    `json:"mangohud,omitempty"`
	Wayland  *bool    `json:"wayland,omitempty"`
}

// Data is one data plugin's content: its fixes and its tweaks, in file order.
type Data struct {
	Fixes  []Fix
	Tweaks []Tweak
}

// file is one JSON data file. A file may hold fixes, tweaks or both.
type file struct {
	Fixes  []Fix   `json:"fixes,omitempty"`
	Tweaks []Tweak `json:"tweaks,omitempty"`
}

var (
	idRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	envKeyRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	fixTiers   = []string{"platinum", "gold", "silver", "bronze", "borked"}
	tweakKinds = []string{"recommended", "troubleshoot"}
	gpuVendors = []string{"nvidia", "amd", "intel"}
	wrappers   = []string{"gamemode", "mangohud"}
)

// Load reads a data plugin's files from fsys. files comes from the manifest;
// an empty list means the default file names, and missing ones are skipped.
// A listed file that is missing is an error, so a typo doesn't load nothing.
func Load(fsys fs.FS, files []string) (*Data, error) {
	explicit := len(files) > 0
	if !explicit {
		files = DefaultFiles
	}
	out := &Data{}
	loaded := 0
	for _, name := range files {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			if !explicit && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		var f file
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for i, fix := range f.Fixes {
			if err := fix.validate(); err != nil {
				return nil, fmt.Errorf("%s: fixes[%d]: %w", name, i, err)
			}
		}
		for i, tw := range f.Tweaks {
			if err := tw.validate(); err != nil {
				return nil, fmt.Errorf("%s: tweaks[%d]: %w", name, i, err)
			}
			dup := slices.ContainsFunc(out.Tweaks, func(x Tweak) bool { return x.ID == tw.ID }) ||
				slices.ContainsFunc(f.Tweaks[:i], func(x Tweak) bool { return x.ID == tw.ID })
			if dup {
				return nil, fmt.Errorf("%s: tweaks[%d]: duplicate tweak id %q", name, i, tw.ID)
			}
		}
		out.Fixes = append(out.Fixes, f.Fixes...)
		out.Tweaks = append(out.Tweaks, f.Tweaks...)
		loaded++
	}
	if loaded == 0 {
		return nil, fmt.Errorf("no data files found (looked for %s)", strings.Join(files, ", "))
	}
	return out, nil
}

func (f Fix) validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if strings.TrimSpace(f.Title) == "" && strings.TrimSpace(f.AppID) == "" {
		bad("a fix needs a title or an appId to match a game")
	}
	if strings.TrimSpace(f.Source) == "" {
		bad("source is required (e.g. \"clarity/lutris\")")
	}
	if !slices.Contains(fixTiers, f.Tier) {
		bad("tier %q must be one of %s", f.Tier, strings.Join(fixTiers, ", "))
	}
	if strings.TrimSpace(f.Description) == "" {
		bad("description is required")
	}
	for i, w := range f.Winetricks {
		if strings.TrimSpace(w) == "" || strings.ContainsAny(w, " \t") {
			bad("winetricks[%d]: %q is not a verb", i, w)
		}
	}
	for k, v := range f.DLLOverrides {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			bad("dllOverrides: %q: %q is not a DLL override", k, v)
		}
	}
	return errors.Join(errs...)
}

func (t Tweak) validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	if !idRe.MatchString(t.ID) {
		bad("id %q must be lowercase letters, digits, '.', '_' or '-'", t.ID)
	}
	if !slices.Contains(tweakKinds, t.Kind) {
		bad("kind %q must be %s", t.Kind, strings.Join(tweakKinds, " or "))
	}
	if strings.TrimSpace(t.Title) == "" {
		bad("title is required")
	}
	if strings.TrimSpace(t.Reason) == "" {
		bad("reason is required; it is what the user reads")
	}
	if len(t.Env) == 0 && len(t.Wrappers) == 0 {
		bad("a tweak needs env vars, wrappers or both")
	}
	for k := range t.Env {
		if !envKeyRe.MatchString(k) {
			bad("env key %q is not a variable name", k)
		}
	}
	for _, w := range t.Wrappers {
		if !slices.Contains(wrappers, w) {
			bad("wrapper %q must be %s", w, strings.Join(wrappers, " or "))
		}
	}
	for _, v := range t.When.GPU {
		if !slices.Contains(gpuVendors, v) {
			bad("when.gpu %q must be %s", v, strings.Join(gpuVendors, ", "))
		}
	}
	return errors.Join(errs...)
}

// matches reports whether the tweak applies to this machine.
func (t Tweak) matches(s System) bool {
	w := t.When
	if len(w.GPU) > 0 && !slices.ContainsFunc(w.GPU, s.has) {
		return false
	}
	if w.NTSync != nil && s.NTSync != *w.NTSync {
		return false
	}
	if w.GameMode != nil && s.GameMode != *w.GameMode {
		return false
	}
	if w.MangoHud != nil && s.MangoHud != *w.MangoHud {
		return false
	}
	if w.Wayland != nil && s.Wayland != *w.Wayland {
		return false
	}
	return true
}
