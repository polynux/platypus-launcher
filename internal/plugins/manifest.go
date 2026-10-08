package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// APIVersion is the protocol version Hydra speaks (docs/plans/launcher-plugins.md).
const APIVersion = 1

// ManifestFile is the manifest's name inside a plugin folder.
const ManifestFile = "plugin.json"

// Capabilities a store plugin may declare.
var knownCapabilities = map[string]bool{
	"library":     true,
	"install":     true,
	"launch":      true,
	"auth":        true,
	"cloud-saves": true,
	// discard: the plugin implements job.discard (see docs/plugins/protocol.md).
	"discard": true,
}

// Plugin kinds. "store" is the default: an executable Hydra talks to over
// JSON-RPC. "data" is JSON only — fix recipes and tweak packs — and needs no
// approval because it can't run anything.
const (
	KindStore = "store"
	KindData  = "data"
)

// Game sources that aren't plugins.
var reservedIDs = map[string]bool{"steam": true, "custom": true}

var (
	idRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	storeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
)

// Manifest is a plugin's plugin.json.
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description"`
	API         int    `json:"api"`
	Store       string `json:"store"`
	Exec        string `json:"exec"`
	// Kind is "store" (default) or "data". A data plugin has no exec and
	// lists its JSON files in Data.
	Kind         string        `json:"kind"`
	Data         []string      `json:"data"`
	Capabilities []string      `json:"capabilities"`
	Requires     []Requirement `json:"requires"`
	// InstallDir is the plugin's subfolder of the Games folder; empty means
	// the plugin id.
	InstallDir  string      `json:"install_dir"`
	Permissions Permissions `json:"permissions"`
}

// Requirement is a command-line tool the plugin wraps. Hydra never installs
// it; Hint tells the user how.
type Requirement struct {
	Bin  string `json:"bin"`
	Hint string `json:"hint"`
}

// Permissions are declared now and enforced by the bubblewrap sandbox later
// (plan milestone 8). Until then they are shown to the user before approval.
type Permissions struct {
	Network []string `json:"network"`
	Read    []string `json:"read"`
	Write   []string `json:"write"`
}

// LoadManifest reads and validates dir/plugin.json. The plugin id must match
// the folder name, so two folders can't claim the same plugin.
func LoadManifest(dir string) (Manifest, error) {
	var m Manifest
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("%s: %w", ManifestFile, err)
	}
	if err := m.validate(); err != nil {
		return m, err
	}
	if m.ID != filepath.Base(dir) {
		return m, fmt.Errorf("id %q doesn't match folder name %q", m.ID, filepath.Base(dir))
	}
	if m.Kind == KindData {
		return m, nil // the data files are read by Discover
	}
	if _, err := m.ExecPath(dir); err != nil {
		return m, err
	}
	return m, nil
}

// IsData reports whether the manifest describes a data plugin.
func (m Manifest) IsData() bool { return m.Kind == KindData }

func (m Manifest) validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if !idRe.MatchString(m.ID) {
		bad("id %q must be lowercase letters, digits, '.', '_' or '-'", m.ID)
	}
	if reservedIDs[m.ID] {
		bad("id %q is reserved for a built-in source", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		bad("name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		bad("version is required")
	}
	if m.API != APIVersion {
		bad("api %d is not supported (Hydra speaks api %d)", m.API, APIVersion)
	}
	if !m.IsData() && !storeRe.MatchString(m.Store) {
		bad("store %q must be lowercase letters, digits or '-'", m.Store)
	}
	kind := m.Kind
	if kind == "" {
		kind = KindStore
	}
	switch kind {
	case KindStore:
		if m.Exec == "" {
			bad("exec is required")
		} else if !filepath.IsLocal(m.Exec) {
			bad("exec %q must be a path inside the plugin folder", m.Exec)
		}
		if len(m.Data) > 0 {
			bad("data files belong to a data plugin (kind \"data\")")
		}
	case KindData:
		if m.Exec != "" {
			bad("a data plugin has no exec")
		}
		if m.Store != "" {
			bad("a data plugin has no store")
		}
		if len(m.Capabilities) > 0 {
			bad("a data plugin has no capabilities; it can't run anything")
		}
		if len(m.Requires) > 0 {
			bad("a data plugin needs no tools; it can't run anything")
		}
		for _, f := range m.Data {
			if !filepath.IsLocal(f) {
				bad("data file %q must be a path inside the plugin folder", f)
			}
		}
	default:
		bad("kind %q must be %q or %q", m.Kind, KindStore, KindData)
	}
	for _, c := range m.Capabilities {
		if !knownCapabilities[c] {
			bad("unknown capability %q", c)
		}
	}
	for _, r := range m.Requires {
		if r.Bin == "" || strings.ContainsRune(r.Bin, '/') {
			bad("requires: bin %q must be a command name", r.Bin)
		}
	}
	if m.InstallDir != "" && !filepath.IsLocal(m.InstallDir) {
		bad("install_dir %q must be a relative folder name", m.InstallDir)
	}
	for _, p := range append(append([]string{}, m.Permissions.Read...), m.Permissions.Write...) {
		if !filepath.IsAbs(p) && p != "~" && !strings.HasPrefix(p, "~/") {
			bad("permission path %q must be absolute or start with ~/", p)
		}
	}
	for _, h := range m.Permissions.Network {
		if strings.TrimSpace(h) == "" {
			bad("network permission entries can't be empty")
		}
	}
	return errors.Join(errs...)
}

// ExecPath resolves the executable, following symlinks, and checks it stays
// inside the plugin folder and is an executable regular file.
func (m Manifest) ExecPath(dir string) (string, error) {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(realDir, m.Exec))
	if err != nil {
		return "", fmt.Errorf("exec: %w", err)
	}
	if rel, err := filepath.Rel(realDir, path); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("exec %q resolves outside the plugin folder", m.Exec)
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("exec: %w", err)
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("exec %q is not an executable file", m.Exec)
	}
	return path, nil
}

// InstallPath is where the plugin installs games: its subfolder of the
// Games folder.
func (m Manifest) InstallPath(gamesFolder string) string {
	sub := m.InstallDir
	if sub == "" {
		sub = m.ID
	}
	return filepath.Join(gamesFolder, sub)
}

// MissingRequirements returns the required tools not found on PATH.
func (m Manifest) MissingRequirements(lookPath func(string) (string, error)) []Requirement {
	var missing []Requirement
	for _, r := range m.Requires {
		if _, err := lookPath(r.Bin); err != nil {
			missing = append(missing, r)
		}
	}
	return missing
}

// hashPlugin returns the hex SHA-256 of a plugin folder, used to pin what the
// user approved: plugin.json and every regular file under it, in sorted path
// order. Bytecode caches (__pycache__/, *.pyc) are skipped, since an
// interpreter may write them next to the sources it already ran. Symlinks
// count by their target text, so repointing one changes the hash.
func hashPlugin(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".pyc") {
			return nil
		}
		if d.Type().IsRegular() || d.Type()&fs.ModeSymlink != 0 {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths) // WalkDir is lexical per folder, not across the whole path
	h := sha256.New()
	for _, path := range paths {
		rel, _ := filepath.Rel(dir, path)
		fi, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		var body []byte
		if fi.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			body = []byte(target)
		} else if body, err = os.ReadFile(path); err != nil {
			return "", err
		}
		sum := sha256.Sum256(body)
		fmt.Fprintf(h, "%s\x00%c\x00%x\n", filepath.ToSlash(rel), fi.Mode().String()[0], sum)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
