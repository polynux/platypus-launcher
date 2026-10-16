// Package crashhints matches a game's Wine/Proton session log against known
// failure patterns and proposes a fix for each (issue #46). The rule table is
// data: one row is a pattern and what it suggests; the explanations are in the
// UI's catalogue under the rule id. The log is read here and never leaves the
// machine.
package crashhints

import (
	"regexp"
	"strings"

	"platypus/internal/types"
	"platypus/internal/winetricks"
)

// rule is one row of the table. A log line belongs to the first rule whose re
// matches it. Group 1 of re, when present, is the Param (a file name). verb is
// a winetricks verb, where "$1" expands to group 1 and a ".dll" suffix is
// dropped. action is "runner" or "anticheat", or "" for an explanation only.
type rule struct {
	id     string
	re     *regexp.Regexp
	verb   string
	action string
}

var rules = []rule{
	{"vcredist", regexp.MustCompile(`(?i)err:.*\b((?:vcruntime|msvcp)1[34]0\w*\.dll)`), "vcrun2022", ""},
	{"directx", regexp.MustCompile(`(?i)err:.*\b((?:d3dx9(?:_\d+)?|d3dcompiler_\d+|d3dx1[01]_\d+)\.dll)`), "$1", ""},
	{"media", regexp.MustCompile(`(?i)err:.*\b(?:mfplat|mfreadwrite|mfmediaengine)\b`), "", "runner"},
	{"dotnet", regexp.MustCompile(`(?i)err:.*\b(?:mscoree|mscoreei|mscorlib)\b`), "dotnet48", ""},
	{"vulkan", regexp.MustCompile(`(?i)\b(?:dxvk|vkd3d)\b.*(?:no (?:compatible )?vulkan|failed to (?:create|find|enumerate)\b.*vulkan|vulkan.*(?:not found|unavailable|no devices))`), "", ""},
	{"anticheat", regexp.MustCompile(`(?i)easyanticheat|eac_launcher|battleye|beservice`), "", "anticheat"},
	// Last: the generic case names the DLL and nothing more.
	{"missingDll", regexp.MustCompile(`err:module:import_dll Library (\S+)`), "", ""},
}

// maxHints keeps the panel short: a log can name many missing DLLs.
const maxHints = 8

// Match returns the hints for a session log, one per distinct finding, in the
// order the log names them. Lines that fit no rule are ignored. Most rules
// need an err: line; the Vulkan and anti-cheat rules match any line.
func Match(log string) []types.CrashHint {
	var out []types.CrashHint
	seen := map[types.CrashHint]bool{}
	for _, line := range strings.Split(log, "\n") {
		h, ok := matchLine(strings.TrimRight(line, "\r"))
		if !ok || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
		if len(out) == maxHints {
			break
		}
	}
	return out
}

func matchLine(line string) (types.CrashHint, bool) {
	for _, r := range rules {
		m := r.re.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		h := types.CrashHint{Rule: r.id, Action: r.action}
		if len(m) >= 4 && m[2] >= 0 {
			h.Param = line[m[2]:m[3]]
		}
		if r.verb != "" {
			v := strings.TrimSuffix(strings.ToLower(string(r.re.ExpandString(nil, r.verb, line, m))), ".dll")
			if winetricks.KnownVerb(v) {
				h.Verb = v
			}
		}
		return h, true
	}
	return types.CrashHint{}, false
}
