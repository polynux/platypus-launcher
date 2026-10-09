package plugins

import (
	"regexp"
	"slices"
)

// Capability is one row of the capability registry: what a name in a
// manifest's `capabilities` means to core. Adding a capability is adding a
// row here (plus its orchestrator); manifest validation, the conformance
// suite and the gate in Manager.Call all read this table.
type Capability struct {
	Name string `json:"name"`
	// Since is the protocol revision that introduced the capability. All
	// revisions are additive to api 1.
	Since string `json:"since"`
	// Methods are the requests Hydra sends once the plugin offers the
	// capability; the plugin must implement them.
	Methods []string `json:"methods"`
	// OptionalMethods may be missing: the plugin answers -32601, Hydra
	// notes it for the rest of that plugin run and stops asking.
	OptionalMethods []string `json:"optionalMethods,omitempty"`
	// Surfaces are the places in the UI that appear only when the plugin
	// (and the game) offers the capability.
	Surfaces []string `json:"surfaces"`
	// Orchestrated is true when core has code that uses the capability.
	// False entries are recognised, so manifests naming them are valid and a
	// plugin's `needs` can name them, but nothing consumes them yet.
	Orchestrated bool `json:"orchestrated"`
}

const (
	rev10 = "1.0" // capabilities of the first plugin releases
	rev11 = "1.1" // parity roadmap (docs/plans/launcher-parity.md §2)
	rev12 = "1.2" // cloud saves (docs/plans/steam-cloud-social.md)
	rev13 = "1.3" // versions (docs/plans/launcher-parity.md §2.2)
	rev14 = "1.4" // verify and move (docs/plans/launcher-parity.md §2.3)
)

var registry = []Capability{
	{Name: "library", Since: rev10, Orchestrated: true,
		Methods: []string{"library.list"}, OptionalMethods: []string{"game.details"},
		Surfaces: []string{"library", "game-page"}},
	{Name: "install", Since: rev10, Orchestrated: true,
		Methods:  []string{"game.installSize", "game.install", "game.update", "game.uninstall", "job.cancel"},
		Surfaces: []string{"install-button", "update-button", "uninstall-button", "install-folder"}},
	{Name: "launch", Since: rev10, Orchestrated: true,
		Methods: []string{"game.launchSpec"}, Surfaces: []string{"play-button"}},
	{Name: "auth", Since: rev10, Orchestrated: true,
		Methods:  []string{"auth.status", "auth.begin", "auth.complete", "auth.logout"},
		Surfaces: []string{"login", "logout"}},
	{Name: "discard", Since: rev10, Orchestrated: true,
		Methods: []string{"job.discard"}, Surfaces: []string{"downloads-discard"}},
	{Name: "cloud-saves", Since: rev12, Orchestrated: true,
		Methods:  []string{"cloud.status", "cloud.pull", "cloud.push", "cloud.resolve"},
		Surfaces: []string{"cloud-row", "cloud-conflict"}},

	{Name: "install-options", Since: rev11,
		Methods: []string{"game.options"}, Surfaces: []string{"install-dialog"}},
	{Name: "versions", Since: rev13, Orchestrated: true,
		Methods: []string{"versions.list"}, OptionalMethods: []string{"versions.unlock"},
		Surfaces: []string{"version-picker", "version-pin"}},
	{Name: "verify", Since: rev14, Orchestrated: true,
		Methods: []string{"game.verify"}, Surfaces: []string{"verify-button"}},
	{Name: "move", Since: rev14, Orchestrated: true,
		Methods: []string{"game.move"}, Surfaces: []string{"move-dialog"}},
	{Name: "social", Since: rev11,
		Methods:         []string{"social.start", "social.stop", "social.state", "social.self", "social.friends"},
		OptionalMethods: []string{"social.setStatus", "social.setPlaying"},
		Surfaces:        []string{"friends-summary", "friends-modal"}},
	{Name: "achievements", Since: rev11,
		Methods: []string{"achievements.list"}, Surfaces: []string{"achievement-list"}},
	{Name: "playtime", Since: rev11,
		Methods: []string{"playtime.list"}, Surfaces: []string{"playtime"}},
	{Name: "redeem", Since: rev11,
		Methods: []string{"redeem.submit"}, Surfaces: []string{"redeem-entry"}},
	{Name: "family", Since: rev11,
		Methods: []string{"family.members"}, Surfaces: []string{"shared-badge", "shared-filter", "families-section"}},
}

// capabilityFreeOptional are optional methods that belong to no capability:
// Hydra sends them to any plugin that speaks the revision that defines them,
// and remembers a -32601 answer like any optional method. game.relocated
// (protocol 1.4) tells a plugin that core moved its game folder.
var capabilityFreeOptional = []string{"game.relocated"}

// OptionalWithoutCapability reports whether method is an optional method that
// no capability owns (see capabilityFreeOptional).
func OptionalWithoutCapability(method string) bool {
	return slices.Contains(capabilityFreeOptional, method)
}

// sharedMethods are requests that more than one capability uses besides the
// one that owns them: job.cancel stops an install, and also a cloud sync.
var sharedMethods = map[string][]string{"job.cancel": {"cloud-saves"}}

// MethodOffered reports whether a plugin offering caps serves method, which
// belongs to capability (see CapabilityForMethod).
func MethodOffered(method, capability string, caps []string) bool {
	if slices.Contains(caps, capability) {
		return true
	}
	for _, c := range sharedMethods[method] {
		if slices.Contains(caps, c) {
			return true
		}
	}
	return false
}

// WithinProtocol splits caps into the capabilities a plugin that settled on
// protocol v can offer and the ones introduced by a later revision, which
// stay off for it. Names this Hydra doesn't know pass (they do nothing).
func WithinProtocol(caps []string, v Version) (kept, later []string) {
	kept = []string{}
	for _, name := range caps {
		if c, ok := LookupCapability(name); ok {
			if since, err := ParseVersion(c.Since); err == nil && v.Less(since) {
				later = append(later, name)
				continue
			}
		}
		kept = append(kept, name)
	}
	return kept, later
}

type methodOwner struct {
	capability string
	optional   bool
}

var (
	capByName   = map[string]int{}
	capByMethod = map[string]methodOwner{}
)

func init() {
	for i, c := range registry {
		capByName[c.Name] = i
		for _, m := range c.Methods {
			capByMethod[m] = methodOwner{c.Name, false}
		}
		for _, m := range c.OptionalMethods {
			capByMethod[m] = methodOwner{c.Name, true}
		}
	}
}

func (c Capability) clone() Capability {
	c.Methods = slices.Clone(c.Methods)
	c.OptionalMethods = slices.Clone(c.OptionalMethods)
	c.Surfaces = slices.Clone(c.Surfaces)
	return c
}

// Registry returns every capability this Hydra knows, in registration order.
func Registry() []Capability {
	out := make([]Capability, len(registry))
	for i, c := range registry {
		out[i] = c.clone()
	}
	return out
}

// LookupCapability finds a registered capability by name.
func LookupCapability(name string) (Capability, bool) {
	i, ok := capByName[name]
	if !ok {
		return Capability{}, false
	}
	return registry[i].clone(), true
}

// CapabilityForMethod says which capability a protocol method belongs to,
// and whether the method is optional within it. Methods outside every
// capability (initialize, ...) are not gated.
func CapabilityForMethod(method string) (capability string, optional, ok bool) {
	e, ok := capByMethod[method]
	return e.capability, e.optional, ok
}

// capNameRe is the shape of a capability name: lowercase words joined by '-'.
// A name that is well formed but not registered is merely unknown.
var capNameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// ValidCapabilityName reports whether s is a well-formed capability name.
func ValidCapabilityName(s string) bool { return len(s) <= 40 && capNameRe.MatchString(s) }
