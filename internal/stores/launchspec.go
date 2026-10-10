package stores

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/session"
)

// Env keys a plugin may not set: they change which code runs (loader, wine
// DLL overrides, Vulkan layers, interpreters) or where Platypus's own files are.
// A denylist rather than an allowlist because games need arbitrary env.
var (
	deniedEnv = map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
		"IFS": true, "ENV": true, "BASH_ENV": true, "TMPDIR": true, "GCONV_PATH": true,
		"MANGOHUD_CONFIGFILE": true, "MANGOHUD_CONFIG": true,
	}
	deniedEnvPrefixes = []string{
		"LD_", "DYLD_", "WINE", "PROTON_", "STEAM_COMPAT_", "VK_", "__EGL_", "__GLX_",
		"LIBGL_", "GST_", "GTK_", "GIO_", "QT_", "XDG_", "PYTHON", "PERL", "RUBY", "NODE_",
	}
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func envAllowed(key string) bool {
	if !envKeyRe.MatchString(key) || deniedEnv[strings.ToUpper(key)] {
		return false
	}
	upper := strings.ToUpper(key)
	for _, p := range deniedEnvPrefixes {
		if strings.HasPrefix(upper, p) {
			return false
		}
	}
	return true
}

// ValidateSpec checks a plugin's launch spec before Platypus runs anything:
//   - direct: the executable and working dir must be inside one of folders
//     (the plugin's install folder, or folders the user allowed), after
//     resolving symlinks, and no env key may be on the denylist;
//   - store-client: the command must be one of the tools the manifest
//     requires, and the game must be identifiable by its command line.
func ValidateSpec(spec plugins.LaunchSpec, man plugins.Manifest, folders []string) (*Launch, error) {
	if spec.Delegate != "" {
		return validateDelegate(spec, man)
	}
	if spec.Exe == "" {
		return nil, fmt.Errorf("%s returned no executable", man.ID)
	}
	if !filepath.IsAbs(spec.Exe) {
		return nil, fmt.Errorf("%s: executable %q must be an absolute path", man.ID, spec.Exe)
	}
	exe, err := filepath.EvalSymlinks(spec.Exe)
	if err != nil {
		return nil, fmt.Errorf("%s: executable: %w", man.ID, err)
	}
	if !insideAny(exe, folders) {
		return nil, fmt.Errorf("%s: executable %s is outside its install folder (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
	}
	st, err := os.Stat(exe)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %s is not a file", man.ID, spec.Exe)
	}
	if !spec.NeedsWine && st.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("%s: %s is not executable", man.ID, spec.Exe)
	}

	dir := spec.Cwd
	if dir == "" {
		dir = filepath.Dir(spec.Exe)
	}
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("%s: working directory %q must be an absolute path", man.ID, dir)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: working directory: %w", man.ID, err)
	}
	if !insideAny(realDir, folders) {
		return nil, fmt.Errorf("%s: working directory %s is outside its install folder", man.ID, dir)
	}

	env := make(map[string]string, len(spec.Env))
	for k, v := range spec.Env {
		if !envAllowed(k) {
			return nil, fmt.Errorf("%s: launch spec sets %s, which plugins may not set", man.ID, k)
		}
		env[k] = v
	}
	var redact []string
	for _, r := range spec.Redact {
		if r != "" {
			redact = append(redact, r)
		}
	}
	return &Launch{Direct: &DirectLaunch{
		Exe:       exe,
		Args:      spec.Args,
		Dir:       realDir,
		Env:       env,
		NeedsWine: spec.NeedsWine,
		Redact:    redact,
	}}, nil
}

func validateDelegate(spec plugins.LaunchSpec, man plugins.Manifest) (*Launch, error) {
	if spec.Delegate != "store-client" {
		return nil, fmt.Errorf("%s: unknown delegate %q", man.ID, spec.Delegate)
	}
	if len(spec.Command) == 0 {
		return nil, fmt.Errorf("%s: store-client launch without a command", man.ID)
	}
	bin := spec.Command[0]
	if !slices.ContainsFunc(man.Requires, func(r plugins.Requirement) bool { return r.Bin == bin }) {
		return nil, fmt.Errorf("%s: store-client command %q is not one of the tools its manifest requires", man.ID, bin)
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("%s: %s is not installed", man.ID, bin)
	}
	if spec.Find == nil || len(spec.Find.Cmdline) == 0 {
		return nil, fmt.Errorf("%s: store-client launch must say how to find the game (find.cmdline)", man.ID)
	}
	parts := slices.Clone(spec.Find.Cmdline)
	for _, p := range parts {
		if len(p) < 3 {
			return nil, fmt.Errorf("%s: find.cmdline entry %q is too short to identify a game", man.ID, p)
		}
	}
	client := spec.Client
	if client == "" {
		client = man.Name
	}
	timeout := time.Duration(spec.StartTimeout) * time.Second
	if timeout < 0 || timeout > 30*time.Minute {
		timeout = 0
	}
	return &Launch{Client: &ClientLaunch{
		Client:       client,
		Command:      spec.Command,
		Find:         func() int { return session.FindByCmdline(parts...) },
		StartTimeout: timeout,
	}}, nil
}

// insideAny reports whether path is one of the folders or below one. Folders
// are resolved through symlinks first, like path should already be.
func insideAny(path string, folders []string) bool {
	for _, f := range folders {
		if f == "" {
			continue
		}
		if real, err := filepath.EvalSymlinks(f); err == nil {
			f = real
		}
		rel, err := filepath.Rel(f, path)
		if err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return true
		}
	}
	return false
}

// RedactArgs hides the values of arguments that start with one of prefixes.
func RedactArgs(args, prefixes []string) []string {
	if len(prefixes) == 0 {
		return args
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		for _, p := range prefixes {
			if strings.HasPrefix(a, p) && len(a) > len(p) {
				out[i] = p + "***"
				break
			}
		}
	}
	return out
}
