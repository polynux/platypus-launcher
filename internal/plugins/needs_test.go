package plugins

import (
	"strings"
	"testing"
)

func TestUnknownCapabilityIsIgnoredWithAWarning(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["capabilities"] = []string{"library", "install", "launch", "auth", "hologram"}
	})
	p := e.discover(t)["echo"]
	if p.Status != StatusUnapproved {
		t.Fatalf("a manifest with an unknown capability should still list and wait for approval: %+v", p)
	}
	if len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "hologram") {
		t.Fatalf("warnings = %q, want one naming the unknown capability", p.Warnings)
	}
	if err := e.m.Approve("echo"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	p = e.discover(t)["echo"]
	if p.Status != StatusReady {
		t.Fatalf("the unknown capability must not stop the plugin: %+v", p)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatalf("plugin with an unknown capability didn't start: %v", err)
	}
}

func TestKnownCapabilitiesGiveNoWarning(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["capabilities"] = []string{"library", "install", "versions", "cloud-saves"}
	})
	if p := e.discover(t)["echo"]; len(p.Warnings) != 0 {
		t.Fatalf("registered capabilities warned: %q", p.Warnings)
	}
}

func TestUnmetNeedsKeepThePluginFromStarting(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["capabilities"] = []string{"library", "install", "launch", "auth", "hologram"}
		m["needs"] = []string{"hologram"}
	})
	p := e.discover(t)["echo"]
	// Still listable and approvable by this Platypus, with the reason shown.
	if p.Status != StatusUnapproved {
		t.Fatalf("unapproved plugin with unmet needs: %+v", p)
	}
	if len(p.Warnings) == 0 || !strings.Contains(strings.Join(p.Warnings, "\n"), "newer Platypus") {
		t.Fatalf("warnings should say it needs a newer Platypus: %q", p.Warnings)
	}
	if err := e.m.Approve("echo"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	p = e.discover(t)["echo"]
	if p.Status != StatusNeedsNewer || !strings.Contains(p.Problem, "newer Platypus") || !strings.Contains(p.Problem, "hologram") {
		t.Fatalf("approved plugin with unmet needs: status %q problem %q", p.Status, p.Problem)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil {
		t.Fatal("a plugin that needs a newer Platypus was started")
	}
	if e.m.Running("echo") != 0 {
		t.Fatal("the plugin process is running")
	}
}

func TestRegisteredNeedsAreMet(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["capabilities"] = []string{"library", "install", "launch", "auth", "versions"}
		m["needs"] = []string{"versions"}
	})
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if p := e.discover(t)["echo"]; p.Status != StatusReady || len(p.Warnings) != 0 {
		t.Fatalf("needs a registered capability: %+v", p)
	}
}
