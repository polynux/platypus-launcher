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
	"slices"
	"sort"
	"strings"
)

// APIVersion is the major protocol version of the legacy "api" field and of
// the PLATYPUS_PLUGIN_API variable. The versions Platypus speaks, with minors, are
// SupportedProtocols.
const APIVersion = 1

// ManifestVersion is the manifest schema this Platypus reads. A manifest naming
// a newer one is listed ("needs a newer Platypus") but never interpreted.
const ManifestVersion = 1

// ManifestFile is the manifest's name inside a plugin folder.
const ManifestFile = "plugin.json"

// Plugin kinds. "store" is the default: an executable Platypus talks to over
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
	// ManifestVersion is the schema of this file; absent means 1.
	ManifestVersion int `json:"manifestVersion"`
	// API is the legacy way to name the protocol: "api": N means protocol
	// N.0 over jsonrpc-stdio. Protocol replaces it.
	API int `json:"api"`
	// Protocol is what the plugin speaks and over which transport.
	Protocol *ProtocolSpec `json:"protocol"`
	Store    string        `json:"store"`
	Exec     string        `json:"exec"`
	// Icon is the plugin's logo inside its folder (e.g. "icon.svg"): SVG
	// preferred, PNG accepted. It is served locally to the UI; a missing or
	// invalid file only means no logo (see IconPath), never a broken plugin.
	Icon string `json:"icon,omitempty"`
	// Kind is "store" (default) or "data". A data plugin has no exec and
	// lists its JSON files in Data.
	Kind         string   `json:"kind"`
	Data         []string `json:"data"`
	Capabilities []string `json:"capabilities"`
	// Needs lists capabilities this Platypus must know for the plugin to be
	// worth running: if one is not in the registry the plugin is listed and
	// can be approved, but doesn't start ("needs a newer Platypus"). Every
	// entry must also be in Capabilities.
	Needs    []string      `json:"needs"`
	Requires []Requirement `json:"requires"`
	// InstallDir is the plugin's subfolder of the Games folder; empty means
	// the plugin id.
	InstallDir  string      `json:"install_dir"`
	Permissions Permissions `json:"permissions"`
}

// ProtocolSpec is the manifest's protocol declaration.
type ProtocolSpec struct {
	// Versions the plugin speaks: a list or a {min, max} range.
	Versions VersionSet `json:"versions"`
	// Transport names how to talk to it; empty means DefaultTransport.
	Transport string `json:"transport"`
}

// IncompatibleError says a plugin can't run on this Platypus, and why. Status
// is StatusNeedsNewer or StatusTooOld; Reason is shown to the user.
type IncompatibleError struct {
	Status Status
	Reason string
}

func (e *IncompatibleError) Error() string { return e.Reason }

// Offered is the protocol versions and transport the manifest declares, with
// the legacy api read as "N.0 over jsonrpc-stdio".
func (m Manifest) Offered() (VersionSet, string) {
	if m.Protocol != nil {
		transport := m.Protocol.Transport
		if transport == "" {
			transport = DefaultTransport
		}
		return m.Protocol.Versions, transport
	}
	return VersionSet{List: []Version{{m.API, 0}}}, DefaultTransport
}

// Compatible says whether this Platypus can run the plugin: it knows the
// transport and shares a protocol version with it. supported is the versions
// Platypus speaks (SupportedProtocols). nil means yes.
func (m Manifest) Compatible(supported []Version) *IncompatibleError {
	if m.IsData() {
		if m.API > APIVersion {
			return &IncompatibleError{StatusNeedsNewer, fmt.Sprintf("needs a newer Platypus (data plugin format %d)", m.API)}
		}
		return nil
	}
	offered, transport := m.Offered()
	if _, ok := LookupTransport(transport); !ok {
		return &IncompatibleError{StatusNeedsNewer, fmt.Sprintf("needs a newer Platypus (transport %q)", transport)}
	}
	if _, ok := Negotiate(offered, supported); ok {
		return nil
	}
	list := make([]string, len(supported))
	oldest := supported[0]
	for i, v := range supported {
		list[i] = v.String()
		if v.Less(oldest) {
			oldest = v
		}
	}
	if top, bounded := offered.Highest(); bounded && top.Less(oldest) {
		return &IncompatibleError{StatusTooOld, fmt.Sprintf("too old for this Platypus (speaks protocol %s; Platypus speaks %s)", offered, strings.Join(list, ", "))}
	}
	return &IncompatibleError{StatusNeedsNewer, fmt.Sprintf("needs a newer Platypus (speaks protocol %s; Platypus speaks %s)", offered, strings.Join(list, ", "))}
}

// Requirement is a command-line tool the plugin wraps. Platypus never installs
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
	// A manifest written for a newer schema is not interpreted: it may change
	// any field's shape, which would read as "invalid" instead of "newer".
	var head struct {
		ManifestVersion int `json:"manifestVersion"`
	}
	if json.Unmarshal(data, &head) == nil && head.ManifestVersion > ManifestVersion {
		_ = json.Unmarshal(data, &m) // whatever still reads (the id, the name) is for display
		return m, &IncompatibleError{StatusNeedsNewer, fmt.Sprintf("needs a newer Platypus (manifest format %d)", head.ManifestVersion)}
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

// UnknownCapabilities returns the declared capabilities missing from the
// registry. Platypus ignores them.
func (m Manifest) UnknownCapabilities() []string {
	var out []string
	for _, c := range m.Capabilities {
		if _, ok := LookupCapability(c); !ok {
			out = append(out, c)
		}
	}
	return out
}

// UnmetNeeds returns the needed capabilities missing from the registry.
func (m Manifest) UnmetNeeds() []string {
	var out []string
	for _, c := range m.Needs {
		if _, ok := LookupCapability(c); !ok {
			out = append(out, c)
		}
	}
	return out
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
	if m.ManifestVersion < 0 {
		bad("manifestVersion %d is not a version", m.ManifestVersion)
	}
	if m.API < 0 {
		bad("api %d is not a version", m.API)
	}
	if !m.IsData() && !storeRe.MatchString(m.Store) {
		bad("store %q must be lowercase letters, digits or '-'", m.Store)
	}
	kind := m.Kind
	if kind == "" {
		kind = KindStore
	}
	if kind == KindStore {
		switch {
		case m.Protocol != nil:
			if m.Protocol.Versions.empty() {
				bad("protocol.versions is required")
			}
			if t := m.Protocol.Transport; t != "" && !ValidCapabilityName(t) {
				bad("protocol.transport %q must be lowercase words joined by '-'", t)
			}
		case m.API == 0:
			bad(`protocol is required (or the legacy "api": 1)`)
		}
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
	// A well-formed name Platypus doesn't know is not an error (UnknownCapabilities
	// reports it as a warning); a malformed one is.
	for i, c := range m.Capabilities {
		switch {
		case !ValidCapabilityName(c):
			bad("capability %q is malformed (lowercase words joined by '-')", c)
		case slices.Contains(m.Capabilities[:i], c):
			bad("capability %q is listed twice", c)
		}
	}
	for _, c := range m.Needs {
		switch {
		case !ValidCapabilityName(c):
			bad("needs: %q is malformed (lowercase words joined by '-')", c)
		case !slices.Contains(m.Capabilities, c):
			bad("needs: %q is not in capabilities", c)
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
	if m.Icon != "" {
		if !filepath.IsLocal(m.Icon) {
			bad("icon %q must be a path inside the plugin folder", m.Icon)
		} else {
			switch strings.ToLower(filepath.Ext(m.Icon)) {
			case ".svg", ".png":
			default:
				bad("icon %q must be an SVG or PNG file", m.Icon)
			}
		}
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
