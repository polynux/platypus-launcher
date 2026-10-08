package dataplug

import (
	"embed"
	"slices"
	"strings"

	"hydra-launcher/internal/types"
)

//go:embed builtin
var builtinFS embed.FS

// Builtin is Hydra's own fixes and tweaks, in the data plugin format and read
// through the same loader as any plugin's data. It can't fail: the embedded
// files are validated by tests.
func Builtin() *Data {
	d, err := Load(builtinFS, []string{"builtin/fixes.json", "builtin/tweaks.json"})
	if err != nil {
		panic("dataplug: built-in data is invalid: " + err.Error())
	}
	return d
}

// Registry is the fixes and tweaks available right now: the built-in data
// first, then every valid data plugin, in load order. The first fix that
// matches a game wins; a tweak id already known is ignored, so a plugin can't
// change what a built-in tweak does.
type Registry struct {
	fixes  []Fix
	tweaks []Tweak
}

func NewRegistry() *Registry {
	return &Registry{}
}

// Add appends one data plugin's fixes and tweaks.
func (r *Registry) Add(d *Data) {
	if d == nil {
		return
	}
	r.fixes = append(r.fixes, d.Fixes...)
	for _, t := range d.Tweaks {
		if slices.ContainsFunc(r.tweaks, func(x Tweak) bool { return x.ID == t.ID }) {
			continue
		}
		r.tweaks = append(r.tweaks, t)
	}
}

// Reset drops every fix and tweak so the registry can be rebuilt from the
// current data plugins plus the built-in data (CORE-5: rescan sync).
func (r *Registry) Reset() {
	r.fixes = nil
	r.tweaks = nil
}

// Fix returns the curated fix for a game, or nil. Titles match exactly; the
// app id matches Steam's and plugin games' store ids.
func (r *Registry) Fix(title, appID string) *types.GameFixInfo {
	title = strings.TrimSpace(title)
	for _, f := range r.fixes {
		matched := (f.Title != "" && f.Title == title) || (f.AppID != "" && f.AppID == appID)
		if !matched {
			continue
		}
		return &types.GameFixInfo{
			Source:       f.Source,
			Tier:         f.Tier,
			Description:  f.Description,
			Winetricks:   f.Winetricks,
			DLLOverrides: f.DLLOverrides,
			LaunchArgs:   f.LaunchArgs,
		}
	}
	return nil
}

// Tweaks returns the tweaks that apply to this machine, in load order. They
// are offered for every game, platinum ratings included: a rating says the
// game works for most people, not that it works on this GPU or session.
func (r *Registry) Tweaks(s System) []types.Tweak {
	var out []types.Tweak
	for _, t := range r.tweaks {
		if !t.matches(s) {
			continue
		}
		out = append(out, types.Tweak{
			ID:       t.ID,
			Title:    t.Title,
			Reason:   t.Reason,
			Kind:     t.Kind,
			Env:      t.Env,
			Wrappers: t.Wrappers,
		})
	}
	return out
}
