package settings

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/types"
	"platypus/internal/winetricks"
)

// Wine prefix presets (issue #52): named templates the user saves. The
// built-in presets live in the backend, not here; ids starting with
// "builtin:" are reserved for them.
const presetBuiltinPrefix = "builtin:"

// Windows versions a preset may record. Empty means win10.
var presetWindowsVersions = []string{"win7", "win8", "win81", "win10", "win11"}

// The verb check shares dataplug's winetricks rule (a known verb from the
// pinned list), and the env key rule mirrors dataplug's and the launch
// spec's: a variable name, nothing more.
var presetEnvKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidatePrefixPreset checks a user preset the way SavePrefixPreset does:
// a non-empty name, known winetricks verbs, variable-name env keys,
// non-empty DLL overrides and a known Windows version (or none, meaning
// win10).
func ValidatePrefixPreset(p types.PrefixPreset) error {
	if strings.TrimSpace(p.Name) == "" {
		return apperr.New("settings.preset.nameMissing", "the preset needs a name", nil)
	}
	for _, w := range p.Winetricks {
		if strings.TrimSpace(w) == "" || strings.ContainsAny(w, " \t") || !winetricks.KnownVerb(w) {
			return apperr.New("settings.preset.badVerb",
				fmt.Sprintf("preset verb %q is not a verb", w),
				map[string]string{"verb": w})
		}
	}
	for k := range p.Env {
		if !presetEnvKeyRe.MatchString(k) {
			return apperr.New("settings.preset.badEnvKey",
				fmt.Sprintf("preset env key %q is not a variable name", k),
				map[string]string{"key": k})
		}
	}
	for k, v := range p.DLLOverrides {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			return apperr.New("settings.preset.badOverride",
				fmt.Sprintf("preset DLL override %q: %q is not a DLL override", k, v),
				map[string]string{"dll": k, "mode": v})
		}
	}
	if p.WindowsVersion != "" && !slices.Contains(presetWindowsVersions, p.WindowsVersion) {
		return apperr.New("settings.preset.badWindowsVersion",
			fmt.Sprintf("Windows version must be %s, got %q", strings.Join(presetWindowsVersions, ", "), p.WindowsVersion),
			map[string]string{"value": p.WindowsVersion})
	}
	return nil
}

// PrefixPresets returns the user's presets by id.
func (s *Store) PrefixPresets() map[string]types.PrefixPreset {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]types.PrefixPreset, len(s.data.PrefixPresets))
	for id, p := range s.data.PrefixPresets {
		out[id] = p
	}
	return out
}

// SavePrefixPreset creates or replaces a user preset, assigning it an id
// when it has none. Built-in ids can't be saved over.
func (s *Store) SavePrefixPreset(p types.PrefixPreset) (types.PrefixPreset, error) {
	if strings.HasPrefix(p.ID, presetBuiltinPrefix) {
		return types.PrefixPreset{}, apperr.New("settings.preset.builtin",
			fmt.Sprintf("the built-in preset %s is read-only", p.ID),
			map[string]string{"id": p.ID})
	}
	if err := ValidatePrefixPreset(p); err != nil {
		return types.PrefixPreset{}, err
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.ID == "" {
		p.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.PrefixPresets == nil {
		s.data.PrefixPresets = map[string]types.PrefixPreset{}
	}
	prev, had := s.data.PrefixPresets[p.ID]
	s.data.PrefixPresets[p.ID] = p
	if err := s.save(); err != nil {
		if had {
			s.data.PrefixPresets[p.ID] = prev
		} else {
			delete(s.data.PrefixPresets, p.ID)
		}
		return types.PrefixPreset{}, err
	}
	return p, nil
}

// DeletePrefixPreset forgets a user preset. Built-ins aren't stored here and
// can't be deleted; deleting what isn't there is an error.
func (s *Store) DeletePrefixPreset(id string) error {
	if strings.HasPrefix(id, presetBuiltinPrefix) {
		return apperr.New("settings.preset.builtin",
			fmt.Sprintf("the built-in preset %s is read-only", id),
			map[string]string{"id": id})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.PrefixPresets[id]
	if !had {
		return apperr.New("settings.preset.notFound",
			fmt.Sprintf("unknown preset %q", id),
			map[string]string{"id": id})
	}
	delete(s.data.PrefixPresets, id)
	if err := s.save(); err != nil {
		s.data.PrefixPresets[id] = prev
		return err
	}
	return nil
}
