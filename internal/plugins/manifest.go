package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
}

var (
	idRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	storeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
)

// Manifest is a plugin's plugin.json.
type Manifest struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	API          int           `json:"api"`
	Store        string        `json:"store"`
	Exec         string        `json:"exec"`
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
// (milestone 8). Until then they are shown to the user before approval.
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
	if _, err := m.ExecPath(dir); err != nil {
		return m, err
	}
	return m, nil
}

func (m Manifest) validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if !idRe.MatchString(m.ID) {
		bad("id %q must be lowercase letters, digits, '.', '_' or '-'", m.ID)
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
	if !storeRe.MatchString(m.Store) {
		bad("store %q must be lowercase letters, digits or '-'", m.Store)
	}
	// Data plugins without an executable arrive in milestone 2.
	if m.Exec == "" {
		bad("exec is required")
	} else if !filepath.IsLocal(m.Exec) {
		bad("exec %q must be a path inside the plugin folder", m.Exec)
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

// hashFile returns the hex SHA-256 of a file, used to pin plugin executables.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
