package platform

import "strings"

// Windows path rules (issue #83), kept as pure functions so the Linux test
// run covers them: a Windows path is case-insensitive, accepts either
// separator, and has a volume root of its own.

// windowsVolumeRoot is the root of the volume holding a Windows path: "C:\"
// for a drive, `\\server\share\` for a network share, "" for a path with no
// volume.
func windowsVolumeRoot(path string) string {
	p := strings.ReplaceAll(path, "/", `\`)
	if strings.HasPrefix(p, `\\`) {
		parts := strings.SplitN(strings.TrimPrefix(p, `\\`), `\`, 3)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return ""
		}
		return `\\` + parts[0] + `\` + parts[1] + `\`
	}
	if len(p) >= 2 && p[1] == ':' && windowsDriveLetter(p[0]) {
		return strings.ToUpper(p[:1]) + `:\`
	}
	return ""
}

func windowsDriveLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// windowsIsRoot says whether path names a volume root: a drive like "C:\"
// or a share like `\\server\share\`.
func windowsIsRoot(path string) bool {
	root := windowsVolumeRoot(path)
	return root != "" && windowsKey(path) == windowsKey(root)
}

// windowsProtectedDirs lists the folders Windows owns, from its environment:
// the Windows folder, both Program Files, ProgramData, and those defaults on
// the system drive when the environment doesn't say.
func windowsProtectedDirs(getenv func(string) string) []string {
	systemDrive := getenv("SystemDrive")
	if systemDrive == "" {
		systemDrive = "C:"
	}
	dirs := []string{
		getenv("SystemRoot"),
		getenv("windir"),
		getenv("ProgramFiles"),
		getenv("ProgramFiles(x86)"),
		getenv("ProgramData"),
		systemDrive + `\Windows`,
		systemDrive + `\Program Files`,
		systemDrive + `\Program Files (x86)`,
		systemDrive + `\ProgramData`,
	}
	out := make([]string, 0, len(dirs))
	seen := map[string]bool{}
	for _, dir := range dirs {
		key := windowsKey(dir)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, dir)
	}
	return out
}

// windowsIsProtected says whether path is one of dirs or inside one. Only
// folder boundaries match: "C:\WindowsApps" is not inside "C:\Windows".
func windowsIsProtected(path string, dirs []string) bool {
	key := windowsKey(path)
	for _, dir := range dirs {
		d := windowsKey(dir)
		if d == "" {
			continue
		}
		if key == d || strings.HasPrefix(key, d+`\`) {
			return true
		}
	}
	return false
}

// windowsKey normalises a Windows path for comparison: one separator, no
// case, no trailing one.
func windowsKey(path string) string {
	p := strings.ReplaceAll(path, "/", `\`)
	return strings.ToLower(strings.TrimSuffix(p, `\`))
}
