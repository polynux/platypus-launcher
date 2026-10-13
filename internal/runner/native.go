package runner

import (
	"os"
	"path/filepath"
	"strings"
)

// Native sideloaded games (issue #47): detection is by content, not
// extension. ELF magic, a shebang or AppImage magic means native; MZ means
// Windows. A few Windows-only types have no reliable magic, so their
// extension decides (.msi, .bat/.cmd, .lnk). The .AppImage name also decides:
// an AppImage is an ELF anyway, but the name catches truncated files.
//
// Flatpak bundles and .desktop entries are out of scope: Platypus launches
// executables, not sandboxed apps.

// ExecKind is what DetectExecutable found.
type ExecKind string

const (
	ExecELF              ExecKind = "elf"
	ExecAppImage         ExecKind = "appimage"
	ExecScript           ExecKind = "script"
	ExecWindowsExe       ExecKind = "windows-exe"
	ExecWindowsInstaller ExecKind = "windows-installer"
	ExecWindowsScript    ExecKind = "windows-script"
	ExecWindowsShortcut  ExecKind = "windows-shortcut"
	ExecUnknown          ExecKind = "unknown"
)

// Detected is one executable's kind: Native runs without Wine/Proton.
type Detected struct {
	Native bool
	Kind   ExecKind
}

// DetectExecutable inspects the file's content (up to a small header), not
// its extension, except for the types with no magic: .msi/.bat/.cmd/.lnk are
// Windows by extension, .AppImage (and .sh/.x86_64/.x86/.bin as a fallback)
// are native by extension. An empty file is unknown. An unreadable file
// returns the error, so callers can default to Wine and surface it.
func DetectExecutable(path string) (Detected, error) {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return Detected{Kind: ExecUnknown}, err
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, rerr := f.Read(buf)
	if rerr != nil && n == 0 {
		// An empty file reads 0, EOF: that is unknown, not an error.
		if rerr.Error() != "EOF" {
			// Fall through: treat other read errors as unreadable.
			return Detected{Kind: ExecUnknown}, rerr
		}
	}
	buf = buf[:n]
	if len(buf) >= 4 && buf[0] == 0x7f && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'F' {
		if len(buf) >= 11 && buf[8] == 'A' && buf[9] == 'I' && buf[10] == 0x02 {
			return Detected{Native: true, Kind: ExecAppImage}, nil
		}
		return Detected{Native: true, Kind: ExecELF}, nil
	}
	if len(buf) >= 2 && buf[0] == 'M' && buf[1] == 'Z' {
		return Detected{Native: false, Kind: ExecWindowsExe}, nil
	}
	if len(buf) >= 2 && buf[0] == '#' && buf[1] == '!' {
		return Detected{Native: true, Kind: ExecScript}, nil
	}
	switch ext {
	case ".appimage":
		return Detected{Native: true, Kind: ExecAppImage}, nil
	case ".msi":
		return Detected{Native: false, Kind: ExecWindowsInstaller}, nil
	case ".bat", ".cmd":
		return Detected{Native: false, Kind: ExecWindowsScript}, nil
	case ".lnk":
		return Detected{Native: false, Kind: ExecWindowsShortcut}, nil
	case ".sh":
		return Detected{Native: true, Kind: ExecScript}, nil
	case ".x86_64", ".x86", ".bin":
		return Detected{Native: true, Kind: ExecELF}, nil
	}
	return Detected{Native: false, Kind: ExecUnknown}, nil
}

// Run modes for a sideloaded game (GameSettings.RunMode): "" and "auto" mean
// the detected value, "native" forces a Linux run, "wine" forces Wine/Proton.
const (
	RunAuto   = "auto"
	RunNative = "native"
	RunWine   = "wine"
)

// ResolveNative applies the user's override to the detected value.
func ResolveNative(detected bool, runMode string) bool {
	switch strings.ToLower(strings.TrimSpace(runMode)) {
	case RunNative:
		return true
	case RunWine:
		return false
	default:
		return detected
	}
}

// ValidRunMode says whether runMode is a storable override.
func ValidRunMode(runMode string) bool {
	switch strings.ToLower(strings.TrimSpace(runMode)) {
	case "", RunAuto, RunNative, RunWine:
		return true
	}
	return false
}

// IsExecutable says whether path has any execute bit.
func IsExecutable(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return st.Mode().Perm()&0o111 != 0, nil
}

// MakeExecutable sets the owner's execute bit on that one file (chmod u+x),
// after the UI confirmed it. It touches nothing else.
func MakeExecutable(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0o100 != 0 {
		return nil
	}
	return os.Chmod(path, st.Mode().Perm()|0o100)
}

// NativeTarget builds the runner target for a native game. A .sh script
// without the execute bit runs through /bin/sh; everything else runs
// directly and needs the bit (the launch then fails with a clear error when
// it is missing, instead of a raw exec failure).
func NativeTarget(exe string, kind ExecKind, executable bool) Target {
	if !executable && (kind == ExecScript || strings.EqualFold(filepath.Ext(exe), ".sh")) {
		return Target{Exe: "/bin/sh", Args: []string{exe}, Native: true}
	}
	return Target{Exe: exe, Native: true}
}
