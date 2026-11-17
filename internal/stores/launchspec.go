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

	"platypus/internal/apperr"
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
//     (the plugin's install folder or folders the user allowed), after
//     resolving symlinks, and no env key may be on the denylist;
//   - store-client: the command must be one of the tools the manifest
//     requires, and the game must be identifiable by its command line;
//   - wine-client (protocol 1.20): the client's executable must be an
//     absolute path (or a C:\ path, resolved against the client's prefix)
//     inside one of folders (its prefix rides in the folders), with the same
//     env denylist, and the game must be identifiable by its command line.
//     A missing executable inside the prefix reads as "not set up yet"
//     (client.notSetUp), not a broken spec. clientDir is the client's prefix,
//     for resolving Windows paths; "" means no client prefix is known.
func ValidateSpec(spec plugins.LaunchSpec, man plugins.Manifest, folders []string, clientDir string) (*Launch, error) {
	if spec.Delegate != "" {
		return validateDelegate(spec, man, folders, clientDir)
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

func validateDelegate(spec plugins.LaunchSpec, man plugins.Manifest, folders []string, clientDir string) (*Launch, error) {
	if spec.Delegate == "wine-client" {
		return validateWineClient(spec, man, folders, clientDir)
	}
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
	find, err := validateFind(spec, man)
	if err != nil {
		return nil, err
	}
	client := spec.Client
	if client == "" {
		client = man.Name
	}
	return &Launch{Client: &ClientLaunch{
		Client:       client,
		Command:      spec.Command,
		Find:         find,
		StartTimeout: validateTimeout(spec.StartTimeout),
	}}, nil
}

// resolveWineExe maps a wine-client executable to a Linux path. Absolute
// paths pass through; Windows paths (C:\... or C:/...) resolve against the
// client's prefix, whose Windows side lives at drive_c (plain Wine) or
// pfx/drive_c (Proton), whichever exists. Only drive C is supported.
func resolveWineExe(exe, clientDir string) (string, error) {
	if len(exe) >= 4 && exe[1] == ':' && (exe[2] == '\\' || exe[2] == '/') &&
		((exe[0] >= 'A' && exe[0] <= 'Z') || (exe[0] >= 'a' && exe[0] <= 'z')) {
		if exe[0] != 'C' && exe[0] != 'c' {
			return "", fmt.Errorf("wine path %q must be on drive C:", exe)
		}
		rest := filepath.FromSlash(strings.ReplaceAll(exe[3:], "\\", "/"))
		if rest == "" || !filepath.IsLocal(rest) {
			return "", fmt.Errorf("wine path %q escapes the prefix", exe)
		}
		for _, base := range []string{filepath.Join(clientDir, "drive_c"), filepath.Join(clientDir, "pfx", "drive_c")} {
			if st, err := os.Stat(filepath.Join(base, rest)); err == nil && st.Mode().IsRegular() {
				return filepath.Join(base, rest), nil
			}
		}
		return filepath.Join(clientDir, "drive_c", rest), nil
	}
	if !filepath.IsAbs(exe) {
		return "", fmt.Errorf("executable %q must be an absolute path or a C:\\ path", exe)
	}
	return exe, nil
}

// validateWineClient checks a wine-client launch spec (protocol 1.20): the
// store client Platypus runs under the runner in its own prefix. The folders
// the App passes already include the client's prefix, so the executable
// containment is the same check direct launches get. A missing executable
// inside the client's prefix means the client isn't set up yet, which reads
// as client.notSetUp rather than a broken spec.
func validateWineClient(spec plugins.LaunchSpec, man plugins.Manifest, folders []string, clientDir string) (*Launch, error) {
	if spec.Prefix != ClientPrefixKey {
		return nil, fmt.Errorf("%s: unknown wine-client prefix %q (want %q)", man.ID, spec.Prefix, ClientPrefixKey)
	}
	if spec.Exe == "" {
		return nil, fmt.Errorf("%s returned no client executable", man.ID)
	}
	exe, err := resolveWineExe(spec.Exe, clientDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", man.ID, err)
	}
	resolved := false
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe, resolved = real, true
	}
	if resolved {
		if !insideAny(exe, folders) {
			return nil, fmt.Errorf("%s: client executable %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
		}
		if st, err := os.Stat(exe); err != nil {
			return nil, err
		} else if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: %s is not a file", man.ID, spec.Exe)
		}
	} else {
		if !insideLexical(exe, folders) {
			return nil, fmt.Errorf("%s: client executable %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
		}
		client := spec.Client
		if client == "" {
			client = man.Name
		}
		return nil, apperr.New("client.notSetUp",
			fmt.Sprintf("%s isn't set up yet; set it up first, then launch the game", client),
			map[string]string{"client": client, "source": man.ID})
	}
	env, err := validateEnvMap(spec.Env, man, "wine-client launch")
	if err != nil {
		return nil, err
	}
	find, err := validateFind(spec, man)
	if err != nil {
		return nil, err
	}
	client := spec.Client
	if client == "" {
		client = man.Name
	}
	return &Launch{Client: &ClientLaunch{
		Client:       client,
		Command:      append([]string{exe}, spec.Args...),
		Find:         find,
		StartTimeout: validateTimeout(spec.StartTimeout),
		Wine: &WineClientLaunch{
			Exe:    exe,
			Args:   slices.Clone(spec.Args),
			Env:    env,
			Redact: validateRedact(spec.Redact),
		},
	}}, nil
}

// ValidateClientSetup checks a client.setup answer (protocol 1.20): the
// installer the core runs under the runner in the client's prefix. Unlike a
// launch spec, the installer must exist now — the core runs it as soon as
// the job starts.
func ValidateClientSetup(spec plugins.ClientSetupSpec, man plugins.Manifest, folders []string, clientDir string) (*ClientSetup, error) {
	if spec.Exe == "" {
		return nil, fmt.Errorf("%s returned no installer executable", man.ID)
	}
	exe, err := resolveWineExe(spec.Exe, clientDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", man.ID, err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("%s: installer %s: %w", man.ID, spec.Exe, err)
	}
	if !insideAny(real, folders) {
		return nil, fmt.Errorf("%s: installer %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
	}
	if st, err := os.Stat(real); err != nil {
		return nil, err
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %s is not a file", man.ID, spec.Exe)
	}
	env, err := validateEnvMap(spec.Env, man, "client.setup")
	if err != nil {
		return nil, err
	}
	if spec.Size < 0 {
		return nil, fmt.Errorf("%s: installer size %d must not be negative", man.ID, spec.Size)
	}
	return &ClientSetup{
		Exe:     real,
		Args:    slices.Clone(spec.Args),
		Env:     env,
		Redact:  validateRedact(spec.Redact),
		Size:    spec.Size,
		Confirm: spec.Confirm,
	}, nil
}

// ValidateClientRun checks a clientRun answer (protocol 1.23): the client
// command the core runs under the runner in the pinned client prefix. Like
// a client.setup exe it must exist now, inside the prefix or an allowed
// folder, with the same env denylist.
func ValidateClientRun(spec plugins.ClientRunSpec, man plugins.Manifest, folders []string, clientDir string) (*ClientRun, error) {
	if spec.Exe == "" {
		return nil, fmt.Errorf("%s returned no client executable", man.ID)
	}
	exe, err := resolveWineExe(spec.Exe, clientDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", man.ID, err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("%s: client command %s: %w", man.ID, spec.Exe, err)
	}
	if !insideAny(real, folders) {
		return nil, fmt.Errorf("%s: client command %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
	}
	if st, err := os.Stat(real); err != nil {
		return nil, err
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %s is not a file", man.ID, spec.Exe)
	}
	env, err := validateEnvMap(spec.Env, man, "clientRun")
	if err != nil {
		return nil, err
	}
	return &ClientRun{
		Exe:    real,
		Args:   slices.Clone(spec.Args),
		Env:    env,
		Redact: validateRedact(spec.Redact),
	}, nil
}

// ClientOpen is a validated client.openSpec answer (protocol 1.25): the store
// client Platypus runs under the runner in its own prefix with no game.
type ClientOpen struct {
	Exe    string
	Args   []string
	Dir    string
	Redact []string
}

// ValidateClientOpen checks a client.openSpec answer (protocol 1.25): the
// client executable Platypus runs under the runner in the pinned client
// prefix with no game. Like a clientRun exe it must exist now, inside the
// prefix or an allowed folder, with no env of its own. A missing executable
// inside the client's prefix means the client isn't set up yet, which reads
// as client.notSetUp rather than a broken spec.
func ValidateClientOpen(spec plugins.ClientOpenSpec, man plugins.Manifest, folders []string, clientDir string) (*ClientOpen, error) {
	if spec.Exe == "" {
		return nil, fmt.Errorf("%s returned no client executable", man.ID)
	}
	exe, err := resolveWineExe(spec.Exe, clientDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", man.ID, err)
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		if insideLexical(exe, folders) {
			client := man.Name
			if client == "" {
				client = man.ID
			}
			return nil, apperr.New("client.notSetUp",
				fmt.Sprintf("%s isn't set up yet; set it up first, then open it", client),
				map[string]string{"client": client, "source": man.ID})
		}
		return nil, fmt.Errorf("%s: client executable %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
	}
	if !insideAny(real, folders) {
		return nil, fmt.Errorf("%s: client executable %s is outside its prefix (%s)", man.ID, spec.Exe, strings.Join(folders, ", "))
	}
	if st, err := os.Stat(real); err != nil {
		return nil, err
	} else if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %s is not a file", man.ID, spec.Exe)
	}
	dir := filepath.Dir(real)
	if spec.Cwd != "" {
		wd, err := resolveWineExe(spec.Cwd, clientDir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", man.ID, err)
		}
		realWd, err := filepath.EvalSymlinks(wd)
		if err != nil {
			return nil, fmt.Errorf("%s: working dir %s: %w", man.ID, spec.Cwd, err)
		}
		if !insideAny(realWd, folders) {
			return nil, fmt.Errorf("%s: working dir %s is outside its prefix (%s)", man.ID, spec.Cwd, strings.Join(folders, ", "))
		}
		if st, err := os.Stat(realWd); err != nil {
			return nil, err
		} else if !st.IsDir() {
			return nil, fmt.Errorf("%s: %s is not a directory", man.ID, spec.Cwd)
		}
		dir = realWd
	}
	return &ClientOpen{
		Exe:    real,
		Args:   slices.Clone(spec.Args),
		Dir:    dir,
		Redact: validateRedact(spec.Redact),
	}, nil
}

// validateEnvMap checks plugin env against the denylist. Wine-client launches
// and client setups share the direct launches' denylist: prefix and DLL
// overrides are Platypus's.
func validateEnvMap(env map[string]string, man plugins.Manifest, what string) (map[string]string, error) {
	out := make(map[string]string, len(env))
	for k, v := range env {
		if !envAllowed(k) {
			return nil, fmt.Errorf("%s: %s sets %s, which plugins may not set", man.ID, what, k)
		}
		out[k] = v
	}
	return out, nil
}

// validateRedact drops the empty redact prefixes; the rest hide argument
// values in previews and logs.
func validateRedact(redact []string) []string {
	var out []string
	for _, r := range redact {
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func validateFind(spec plugins.LaunchSpec, man plugins.Manifest) (func() int, error) {
	if spec.Find == nil || len(spec.Find.Cmdline) == 0 {
		return nil, fmt.Errorf("%s: %s launch must say how to find the game (find.cmdline)", man.ID, spec.Delegate)
	}
	parts := slices.Clone(spec.Find.Cmdline)
	for _, p := range parts {
		if len(p) < 3 {
			return nil, fmt.Errorf("%s: find.cmdline entry %q is too short to identify a game", man.ID, p)
		}
	}
	return func() int { return session.FindByCmdline(parts...) }, nil
}

func validateTimeout(seconds int) time.Duration {
	timeout := time.Duration(seconds) * time.Second
	if timeout < 0 || timeout > 30*time.Minute {
		return 0
	}
	return timeout
}

// insideLexical reports whether path is one of the folders or below one,
// without resolving symlinks: for paths that don't exist yet (a client that
// isn't set up), where EvalSymlinks has nothing to resolve.
func insideLexical(path string, folders []string) bool {
	for _, f := range folders {
		if f == "" {
			continue
		}
		rel, err := filepath.Rel(f, path)
		if err == nil && (rel == "." || filepath.IsLocal(rel)) {
			return true
		}
	}
	return false
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
