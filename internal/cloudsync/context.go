package cloudsync

import (
	"os"
	"os/user"
	"path/filepath"

	"platypus/internal/types"
)

// Context builds the CloudContext for a launch. windows says the game runs
// through Wine or Proton, with prefixDir the folder that holds drive_c and
// winUser the Windows user name inside it ("" takes the current user's name).
func Context(windows bool, installDir, prefixDir, winUser string) types.CloudContext {
	home, _ := os.UserHomeDir()
	cc := types.CloudContext{
		OS:         "linux",
		Home:       home,
		XDGData:    xdgDir("XDG_DATA_HOME", home, ".local/share"),
		XDGConfig:  xdgDir("XDG_CONFIG_HOME", home, ".config"),
		InstallDir: installDir,
	}
	if windows {
		cc.OS = "windows"
		cc.PrefixDir = prefixDir
		cc.WinUser = winUser
		if cc.WinUser == "" {
			if u, err := user.Current(); err == nil {
				cc.WinUser = u.Username
			}
		}
	}
	return cc
}

func xdgDir(env, home, fallback string) string {
	if d := os.Getenv(env); filepath.IsAbs(d) {
		return d
	}
	return filepath.Join(home, fallback)
}
