package main

import (
	"os"
	"path/filepath"
	"testing"
)

// installEchoB puts echo-b next to echo: its folder holds the manifest and the
// wrapper from plugins/echo-b, and the wrapper runs echo's program, which
// Platypus installs as echo/bin/platypus-echo (the name the wrapper expects).
func installEchoB(t *testing.T, pluginsDir string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, "echo-b")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		from, to string
		mode     os.FileMode
	}{
		{"plugins/echo-b/plugin.json", "plugin.json", 0o644},
		{"plugins/echo-b/bin/platypus-echo-b", "bin/platypus-echo-b", 0o755},
	} {
		if err := os.WriteFile(filepath.Join(dir, f.to), mustRead(t, f.from), f.mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "bin", "platypus-echo"), mustRead(t, filepath.Join(pluginsDir, "echo", "bin", "platypus-echo-app")), 0o755); err != nil {
		t.Fatal(err)
	}
}

// Two stores at once: each has its own friends, its own user and its own
// switch, and switching one off leaves the other connected.
func TestTwoStoresHaveTheirOwnFriends(t *testing.T) {
	a, _ := socialEnvWith(t, true)
	a.syncSocial()

	waitSocial(t, "both friends lists connected", func() bool {
		e, b := a.social.View("echo"), a.social.View("echo-b")
		return e.Connection.State == "online" && len(e.Friends) == 12 && b.Connection.State == "online" && len(b.Friends) == 4
	})
	views := a.SocialViews()
	if len(views) != 2 {
		t.Fatalf("SocialViews: %d stores, want 2", len(views))
	}
	byID := map[string]string{}
	for _, v := range views {
		if !v.Enabled || v.Self == nil {
			t.Fatalf("%s: enabled %v, self %v", v.Plugin, v.Enabled, v.Self)
		}
		byID[v.Plugin] = v.Self.Name
	}
	if byID["echo"] == byID["echo-b"] {
		t.Fatalf("both stores show the same user %q", byID["echo"])
	}

	// A status goes to the store it was set on, and only there.
	if err := a.SetSocialStatus("echo-b", "busy"); err != nil {
		t.Fatalf("SetSocialStatus on echo-b: %v", err)
	}
	if statuses, _ := echoRecorded(t, a); len(statuses) != 0 {
		t.Fatalf("echo got the status meant for echo-b: %v", statuses)
	}

	// Switching echo-b off disconnects it and leaves echo connected.
	if err := a.SetSocialEnabled("echo-b", false); err != nil {
		t.Fatal(err)
	}
	if a.social.Running("echo-b") || !a.social.Running("echo") {
		t.Fatal("switching echo-b off must stop only echo-b")
	}
	if err := a.SetSocialEnabled("echo-b", true); err != nil {
		t.Fatal(err)
	}
	waitSocial(t, "echo-b reconnected", func() bool { return a.social.View("echo-b").Connection.State == "online" })
}
