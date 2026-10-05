package dataplug

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// System is what Hydra knows about the machine when applying data plugins:
// tweaks declare which machines they apply to, fixes match by game.
type System struct {
	GPUVendors []string // "nvidia", "amd", "intel"
	Wayland    bool
	NTSync     bool
	GameMode   bool
	MangoHud   bool
}

var pciVendors = map[string]string{"0x10de": "nvidia", "0x1002": "amd", "0x8086": "intel"}

// DetectSystem reads the machine once: GPU vendors from DRM, the session type
// from the environment, and the presence of ntsync, gamemoderun and mangohud.
func DetectSystem() System {
	var s System
	seen := map[string]bool{}
	cards, _ := filepath.Glob("/sys/class/drm/card*/device/vendor")
	for _, c := range cards {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		if v, ok := pciVendors[strings.TrimSpace(string(data))]; ok && !seen[v] {
			seen[v] = true
			s.GPUVendors = append(s.GPUVendors, v)
		}
	}
	s.Wayland = os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != ""
	_, err := os.Stat("/dev/ntsync")
	s.NTSync = err == nil
	_, err = exec.LookPath("gamemoderun")
	s.GameMode = err == nil
	_, err = exec.LookPath("mangohud")
	s.MangoHud = err == nil
	return s
}

func (s System) has(vendor string) bool {
	for _, v := range s.GPUVendors {
		if v == vendor {
			return true
		}
	}
	return false
}
