package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
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

// BundledPluginIDs are the first-party plugin ids allowed to declare a
// manifest gameIdPrefix (protocol 1.15, issue #54 groundwork): a
// third-party plugin can't take over other ids.
var BundledPluginIDs = map[string]bool{"steam": true}

// gameIDPrefixRe is the shape of a manifest gameIdPrefix: 1-32 word chars.
var gameIDPrefixRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

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
	// GameIDPrefix is the prefix of this plugin's game ids (protocol 1.15,
	// issue #54 groundwork): optional, default "p.<plugin id>.". Only
	// bundled/first-party plugins (see BundledPluginIDs) may set it, so a
	// third-party plugin can't take over other ids.
	GameIDPrefix string `json:"gameIdPrefix,omitempty"`
	// InstallDir is the plugin's subfolder of the Games folder; empty means
	// the plugin id.
	InstallDir  string      `json:"install_dir"`
	Permissions Permissions `json:"permissions"`
	// Storefront is the store's official website, embedded in the launcher
	// (issue #17). Nil when the plugin declares none (no store view).
	Storefront *Storefront `json:"storefront,omitempty"`
}

// Storefront is the store's official website as a plugin declares it: the
// page the launcher embeds, and the domains that page belongs to (checkout
// may wander to payment providers outside them; the UI warns about that).
type Storefront struct {
	// URL is the store's front page. Production plugins use https; the test
	// plugin may use an http loopback page (see allowHTTPStorefront).
	URL string `json:"url"`
	// Domains are the plain hostnames the storefront belongs to (no
	// wildcards, paths, ports or IPs — except loopback for the test plugin).
	Domains []string `json:"domains"`
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
// it; Hint tells the user how. Optional (protocol 1.15, issue #44) marks a
// launch helper (steam, flatpak, xdg-open) that isn't reported as missing:
// the plugin still starts without it. validateDelegate still requires the
// helper a spec actually uses to be declared and on PATH.
type Requirement struct {
	Bin      string `json:"bin"`
	Hint     string `json:"hint"`
	Optional bool   `json:"optional,omitempty"`
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

// ValidateForTest exposes manifest validation to other packages' tests
// (protocol 1.15 gameIdPrefix rules).
func (m Manifest) ValidateForTest() error { return m.validate() }

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
	if m.GameIDPrefix != "" {
		switch {
		case !gameIDPrefixRe.MatchString(m.GameIDPrefix):
			bad("gameIdPrefix %q must match ^[A-Za-z0-9._-]{1,32}$", m.GameIDPrefix)
		case strings.HasPrefix(m.GameIDPrefix, "p."):
			bad("gameIdPrefix %q must not start with %q", m.GameIDPrefix, "p.")
		case !BundledPluginIDs[m.ID]:
			bad("gameIdPrefix is only for bundled plugins")
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
	if err := m.validateStorefront(); err != nil {
		bad("%s", err.Error())
	}
	return errors.Join(errs...)
}

// allowHTTPStorefront says an http loopback storefront page is acceptable:
// only for the test plugin (echo, echo-b), or when the operator explicitly
// opts in. Production stores are https-only; the exception exists so the test
// plugin can serve a local page with no network (issue #17).
func allowHTTPStorefront(m Manifest) bool {
	if os.Getenv("PLATYPUS_ALLOW_HTTP_STOREFRONT") != "" {
		return true
	}
	return m.ID == "echo" || m.ID == "echo-b"
}

// isLoopbackHost reports whether host is a loopback address for storefront
// purposes: 127.0.0.0/8, ::1, or localhost.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$`)

// validateStorefront checks the manifest's storefront declaration (issue #17):
// an https URL whose host belongs to the declared domains, which are plain
// hostnames (no wildcards, paths, ports or IPs). The test exception (see
// allowHTTPStorefront) additionally permits an http loopback URL with a
// loopback domain, so the test plugin's local page validates.
func (m Manifest) validateStorefront() error {
	sf := m.Storefront
	if sf == nil {
		return nil
	}
	if m.IsData() {
		return errors.New("a data plugin has no storefront")
	}
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	test := allowHTTPStorefront(m)

	u, err := url.Parse(sf.URL)
	if err != nil || u.Host == "" {
		bad("storefront.url %q is not a URL", sf.URL)
		return errors.Join(errs...)
	}
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "https":
	case "http":
		if !(test && isLoopbackHost(host)) {
			bad("storefront.url %q must be https (http is only for the test plugin's local page)", sf.URL)
		}
	default:
		bad("storefront.url %q must be https (http is only for the test plugin's local page)", sf.URL)
	}
	if len(sf.Domains) == 0 {
		bad("storefront.domains is required when a storefront url is declared")
	}
	if len(sf.Domains) > 16 {
		bad("storefront.domains lists more than 16 domains")
	}
	seen := map[string]bool{}
	for _, d := range sf.Domains {
		dl := strings.ToLower(d)
		switch {
		case seen[dl]:
			bad("storefront.domains lists %q twice", d)
		case test && isLoopbackHost(dl):
			// The test plugin's local page.
		case net.ParseIP(dl) != nil:
			bad("storefront.domains entry %q must be a hostname, not an IP", d)
		case !domainRe.MatchString(dl):
			bad("storefront.domains entry %q must be a plain hostname (no wildcards, paths or ports)", d)
		}
		seen[dl] = true
	}
	if err == nil && u.Host != "" && len(errs) == 0 {
		inside := false
		for d := range seen {
			if host == d || strings.HasSuffix(host, "."+d) {
				inside = true
				break
			}
		}
		if !inside {
			bad("storefront.url host %q is not inside storefront.domains", host)
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
// Optional requirements (protocol 1.15) are never reported as missing:
// the plugin still starts without them.
func (m Manifest) MissingRequirements(lookPath func(string) (string, error)) []Requirement {
	var missing []Requirement
	for _, r := range m.Requires {
		if r.Optional {
			continue
		}
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
