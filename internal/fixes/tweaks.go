package fixes

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hydra-launcher/internal/types"
)

// System is what Hydra knows about the machine when suggesting tweaks.
type System struct {
	GPUVendors []string // "nvidia", "amd", "intel"
	Wayland    bool
	NTSync     bool
	GameMode   bool
	MangoHud   bool
}

var pciVendors = map[string]string{"0x10de": "nvidia", "0x1002": "amd", "0x8086": "intel"}

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

// SuggestTweaks returns tweaks worth offering for every game, even platinum
// ones: a platinum rating says the game works for most people, not that it
// works on this GPU or session. "recommended" tweaks are safe to enable by
// default; "troubleshoot" ones fix specific failures and cost something.
func SuggestTweaks(s System) []types.Tweak {
	var out []types.Tweak
	if s.has("nvidia") {
		out = append(out,
			types.Tweak{
				ID: "nvidia-nvapi", Kind: "recommended",
				Title:  "Enable NVAPI",
				Reason: "NVIDIA GPU detected. Exposes NVAPI so DLSS, Reflex and some vendor checks work.",
				Env:    map[string]string{"PROTON_ENABLE_NVAPI": "1"},
			},
			types.Tweak{
				ID: "nvidia-shader-cache", Kind: "recommended",
				Title:  "Keep the NVIDIA shader cache",
				Reason: "Stops the driver from pruning its cache, which causes stutter on every launch of large games.",
				Env:    map[string]string{"__GL_SHADER_DISK_CACHE_SKIP_CLEANUP": "1"},
			},
			types.Tweak{
				ID: "nvidia-hide", Kind: "troubleshoot",
				Title:  "Report the GPU as AMD",
				Reason: "For games that refuse to start or crash on NVIDIA-specific code paths.",
				Env:    map[string]string{"PROTON_HIDE_NVIDIA_GPU": "1"},
			},
		)
	}
	if s.has("amd") {
		out = append(out, types.Tweak{
			ID: "amd-fsr", Kind: "troubleshoot",
			Title:  "FSR 1 upscaling",
			Reason: "AMD GPU detected. Upscales from a lower fullscreen resolution in older games (GE-Proton only).",
			Env:    map[string]string{"WINE_FULLSCREEN_FSR": "1"},
		})
	}
	if s.NTSync {
		out = append(out, types.Tweak{
			ID: "ntsync", Kind: "recommended",
			Title:  "Use ntsync",
			Reason: "/dev/ntsync is available. Kernel-level Windows sync primitives, faster and more correct than fsync.",
			Env:    map[string]string{"PROTON_USE_NTSYNC": "1"},
		})
	}
	if s.GameMode {
		out = append(out, types.Tweak{
			ID: "gamemode", Kind: "recommended",
			Title:    "Run with GameMode",
			Reason:   "Feral GameMode is installed. Sets the CPU governor to performance while playing.",
			Wrappers: []string{"gamemode"},
		})
	}
	if s.Wayland {
		out = append(out, types.Tweak{
			ID: "wayland", Kind: "troubleshoot",
			Title:  "Native Wayland",
			Reason: "Wayland session detected. Bypasses XWayland: better frame pacing and HDR, but some games misbehave (Proton 10+/GE).",
			Env:    map[string]string{"PROTON_ENABLE_WAYLAND": "1"},
		})
	}
	out = append(out,
		types.Tweak{
			ID: "no-d3d12", Kind: "troubleshoot",
			Title:  "Force D3D11",
			Reason: "For crashes at startup in games with a D3D11 fallback.",
			Env:    map[string]string{"PROTON_NO_D3D12": "1"},
		},
		types.Tweak{
			ID: "wined3d", Kind: "troubleshoot",
			Title:  "Use WineD3D instead of DXVK",
			Reason: "For old D3D8/9 games with rendering glitches under DXVK. Slower.",
			Env:    map[string]string{"PROTON_USE_WINED3D": "1"},
		},
		types.Tweak{
			ID: "log", Kind: "troubleshoot",
			Title:  "Write a Proton log",
			Reason: "Saves $HOME/steam-<appid>.log, needed for any bug report.",
			Env:    map[string]string{"PROTON_LOG": "1"},
		},
	)
	return out
}
