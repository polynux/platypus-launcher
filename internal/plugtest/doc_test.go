package plugtest_test

// docs/plugins/protocol.md is what plugin authors read, and the bundled
// plugins are what they copy. This test keeps the two in step: every method a
// plugin dispatches, and every key the conformance suite lets a payload carry,
// must appear in the doc.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/plugtest"
)

func TestProtocolDocMentionsEveryMethodAndKey(t *testing.T) {
	doc, err := os.ReadFile("../../docs/plugins/protocol.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)

	pyMethod := regexp.MustCompile(`(?m)^\s+"([a-z]+\.[A-Za-z]+)":\s`)
	methods := map[string]string{}
	scripts, _ := filepath.Glob("../../plugins/*/bin/platypus-*")
	for _, p := range scripts {
		data, err := os.ReadFile(p)
		if err != nil || !strings.HasPrefix(string(data), "#!/usr/bin/env python3") {
			continue // the built platypus-steam-dl binary
		}
		_, after, ok := strings.Cut(string(data), "\nMETHODS = {")
		if !ok {
			t.Errorf("%s has no METHODS table", p)
			continue
		}
		table, _, _ := strings.Cut(after, "\n}")
		for _, m := range pyMethod.FindAllStringSubmatch(table, -1) {
			methods[m[1]] = p
		}
	}
	echo, err := os.ReadFile("../../plugins/echo/main.go")
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`"([a-z]+\.[A-Za-z]+)"`)
	for _, line := range regexp.MustCompile(`(?m)^\s*case ".*:$`).FindAllString(string(echo), -1) {
		for _, m := range quoted.FindAllStringSubmatch(line, -1) {
			methods[m[1]] = "plugins/echo/main.go"
		}
	}
	if len(methods) < 10 {
		t.Fatalf("found only %d methods; the extraction is broken: %v", len(methods), methods)
	}
	for m, src := range methods {
		if !strings.Contains(text, "`"+m+"`") {
			t.Errorf("%s dispatches %q, which protocol.md doesn't document", src, m)
		}
	}

	// Every registered capability, its methods and its protocol version
	// are in the Capabilities table.
	for _, c := range plugins.Registry() {
		if !strings.Contains(text, "| `"+c.Name+"` |") {
			t.Errorf("protocol.md's capability table has no row for %q", c.Name)
		}
		for _, m := range append(append([]string{}, c.Methods...), c.OptionalMethods...) {
			if !strings.Contains(text, "`"+m+"`") {
				t.Errorf("protocol.md doesn't document %s (capability %s)", m, c.Name)
			}
		}
	}
	newest := plugins.SupportedProtocols[len(plugins.SupportedProtocols)-1]
	if !strings.HasPrefix(text, "# Store plugin protocol ("+newest.String()+",") {
		t.Errorf("protocol.md's title doesn't name the newest supported protocol %s", newest)
	}
	for _, code := range []string{"-32001", "-32002", "-32601"} {
		if !strings.Contains(text, "`"+code+"`") {
			t.Errorf("protocol.md doesn't document the error code %s", code)
		}
	}

	for _, set := range [][]string{plugtest.LibraryKeys, plugtest.DetailsKeys, plugtest.DRMKeys, plugtest.RequirementsKeys, plugtest.VideoKeys, plugtest.LaunchKeys,
		plugtest.CloudStatusKeys, plugtest.CloudCountsKeys, plugtest.CloudConflictKeys, plugtest.CloudFileKeys, plugtest.CloudClientKeys, plugtest.CloudProgressKeys,
		plugtest.VersionListKeys, plugtest.VersionChannelKeys, plugtest.VersionBuildKeys,
		plugtest.VersionManualKeys, plugtest.VersionLinkKeys,
		plugtest.FamilyMemberKeys, plugtest.FamilyCopyKeys, plugtest.FamilyCopyDLCKeys, plugtest.SettingsKeys,
		plugtest.SettingGroupKeys, plugtest.SettingFieldKeys, plugtest.SettingOptionKeys,
		plugtest.ChatHistoryKeys, plugtest.ChatMessageKeys, plugtest.ChatSendKeys} {
		for _, k := range set {
			if !strings.Contains(text, "`"+k+"`") && !strings.Contains(text, `"`+k+`"`) {
				t.Errorf("protocol.md doesn't mention the key %q", k)
			}
		}
	}
}
