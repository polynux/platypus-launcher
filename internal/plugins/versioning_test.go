package plugins

import (
	"strings"
	"testing"
)

// useProtocol rewrites echo's manifest to the new fields (protocol, transport,
// manifestVersion) instead of the legacy "api".
func useProtocol(versions any, transport string) func(m map[string]any) {
	return func(m map[string]any) {
		delete(m, "api")
		m["manifestVersion"] = 1
		proto := map[string]any{"versions": versions}
		if transport != "" {
			proto["transport"] = transport
		}
		m["protocol"] = proto
	}
}

func approvedEnv(t *testing.T, adjust func(*Options), edit func(m map[string]any)) (*env, Plugin) {
	t.Helper()
	e := newEnvWith(t, adjust)
	install(t, e.plugins, edit)
	p := e.discover(t)["echo"]
	if p.Status == StatusUnapproved {
		if err := e.m.Approve("echo"); err != nil {
			t.Fatal(err)
		}
		p = e.discover(t)["echo"]
	}
	return e, p
}

func platypusSpeaks(vs ...Version) func(*Options) {
	return func(o *Options) { o.Protocols = vs }
}

func TestLegacyAPIManifestStillWorks(t *testing.T) {
	// A plugin from before negotiation: "api": 1 in the manifest, no
	// `protocol` in the initialize answer.
	t.Setenv("ECHO_NO_PROTOCOL", "1")
	e, p := approvedEnv(t, nil, func(m map[string]any) {
		delete(m, "protocol")
		delete(m, "manifestVersion")
		m["api"] = 1
	})
	if p.Status != StatusReady {
		t.Fatalf("legacy manifest: %+v", p)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatalf("legacy plugin didn't start: %v", err)
	}
	if got := e.m.Runtime("echo").Protocol; got != "1.0" {
		t.Fatalf("legacy plugin should be taken to speak 1.0, got %q", got)
	}
}

func TestPluginOffering1To5NegotiatesWhatPlatypusSpeaks(t *testing.T) {
	t.Setenv("ECHO_FUTURE", "1") // its answer carries things only newer protocols define
	e, p := approvedEnv(t, platypusSpeaks(Version{1, 0}), useProtocol(map[string]any{"min": "1.0", "max": "1.5"}, "jsonrpc-stdio"))
	if p.Status != StatusReady {
		t.Fatalf("%+v", p)
	}
	info, err := e.m.Initialize(ctx(t), "echo")
	if err != nil {
		t.Fatal(err)
	}
	if info.Protocol != "1.0" || e.m.Runtime("echo").Protocol != "1.0" {
		t.Fatalf("negotiated %q, want 1.0 against a Platypus that speaks only 1.0", info.Protocol)
	}
	// The unknown field and notification were ignored, and the plugin works.
	if err := e.m.Call(ctx(t), "echo", "echo", map[string]any{"x": 1}, nil); err != nil {
		t.Fatalf("call after negotiating: %v", err)
	}
}

func TestNegotiationPicksTheHighestCommon(t *testing.T) {
	e, _ := approvedEnv(t, nil, useProtocol([]string{"1.0", "1.1", "1.4", "1.5"}, ""))
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if got := e.m.Runtime("echo").Protocol; got != "1.5" {
		t.Fatalf("negotiated %q, want 1.5 (the highest Platypus speaks that the plugin offers)", got)
	}
}

func TestPluginNeedingOnlyANewerProtocolNeedsANewerPlatypus(t *testing.T) {
	e, p := approvedEnv(t, nil, useProtocol([]string{"2.0"}, ""))
	if p.Status != StatusNeedsNewer || !strings.Contains(p.Problem, "newer Platypus") || !strings.Contains(p.Problem, "2.0") {
		t.Fatalf("status %q problem %q", p.Status, p.Problem)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil {
		t.Fatal("started a plugin whose protocol Platypus doesn't speak")
	}
	if e.m.Running("echo") != 0 {
		t.Fatal("process running")
	}
	if err := e.m.Approve("echo"); err == nil {
		t.Fatal("approving a plugin that can never run")
	}
}

func TestPluginWithTooNewMinorOfTheSameMajor(t *testing.T) {
	_, p := approvedEnv(t, nil, useProtocol(map[string]any{"min": "1.18"}, ""))
	if p.Status != StatusNeedsNewer {
		t.Fatalf("%+v", p)
	}
}

func TestPluginTooOldForThisPlatypus(t *testing.T) {
	_, p := approvedEnv(t, platypusSpeaks(Version{2, 0}, Version{2, 1}), useProtocol([]string{"1.0"}, ""))
	if p.Status != StatusTooOld || !strings.Contains(p.Problem, "too old") {
		t.Fatalf("status %q problem %q", p.Status, p.Problem)
	}
}

func TestUnknownTransportIsRefusedCleanly(t *testing.T) {
	e, p := approvedEnv(t, nil, useProtocol([]string{"1.0"}, "carrier-pigeon"))
	if p.Status != StatusNeedsNewer || !strings.Contains(p.Problem, "carrier-pigeon") {
		t.Fatalf("status %q problem %q", p.Status, p.Problem)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil || e.m.Running("echo") != 0 {
		t.Fatalf("an unknown transport must not start: %v", err)
	}
}

func TestMalformedTransportIsInvalid(t *testing.T) {
	_, p := approvedEnv(t, nil, useProtocol([]string{"1.0"}, "Not A Name"))
	if p.Status != StatusInvalid {
		t.Fatalf("%+v", p)
	}
}

func TestNewerManifestVersionIsListedNotParsed(t *testing.T) {
	_, p := approvedEnv(t, nil, func(m map[string]any) {
		m["manifestVersion"] = 2
		// A newer schema may change any field's shape; Platypus must not call
		// that "invalid".
		m["capabilities"] = map[string]any{"library": map[string]any{"since": "2.0"}}
		m["permissions"] = []string{"a list now"}
	})
	if p.Status != StatusNeedsNewer || !strings.Contains(p.Problem, "manifest") {
		t.Fatalf("status %q problem %q", p.Status, p.Problem)
	}
	if p.Manifest.ID != "echo" {
		t.Fatalf("a listed-but-unreadable plugin still needs its id to show: %+v", p.Manifest)
	}
}

func TestUnknownManifestFieldsAreIgnored(t *testing.T) {
	e, p := approvedEnv(t, nil, func(m map[string]any) {
		m["telemetry"] = map[string]any{"endpoint": "https://example.invalid"}
		m["protocol"] = map[string]any{"versions": []string{"1.0", "1.1"}, "compression": "zstd"}
		m["manifestVersion"] = 1
	})
	if p.Status != StatusReady {
		t.Fatalf("%+v", p)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolDeclarationErrorsAreInvalid(t *testing.T) {
	cases := map[string]func(m map[string]any){
		"nothing declared":    func(m map[string]any) { delete(m, "api"); delete(m, "protocol") },
		"bad version":         func(m map[string]any) { useProtocol([]string{"one"}, "")(m) },
		"empty list":          func(m map[string]any) { useProtocol([]string{}, "")(m) },
		"inverted range":      func(m map[string]any) { useProtocol(map[string]any{"min": "1.3", "max": "1.0"}, "")(m) },
		"negative manifest v": func(m map[string]any) { m["manifestVersion"] = -1 },
		"api zero":            func(m map[string]any) { delete(m, "protocol"); m["api"] = 0 },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			_, p := approvedEnv(t, nil, edit)
			if p.Status != StatusInvalid {
				t.Fatalf("want invalid, got %+v", p)
			}
		})
	}
}

func TestLegacyAPIOfANewerMajorNeedsANewerPlatypus(t *testing.T) {
	_, p := approvedEnv(t, nil, func(m map[string]any) { delete(m, "protocol"); m["api"] = 2 })
	if p.Status != StatusNeedsNewer {
		t.Fatalf("api 2: %+v", p)
	}
}

func TestPluginAnsweringAProtocolItWasNotOfferedFails(t *testing.T) {
	t.Setenv("ECHO_PROTOCOL", "1.1")
	e, _ := approvedEnv(t, platypusSpeaks(Version{1, 0}), useProtocol([]string{"1.0", "1.1"}, ""))
	_, err := e.m.Initialize(ctx(t), "echo")
	if err == nil || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("initialize with a protocol Platypus didn't offer: %v", err)
	}
	t.Setenv("ECHO_PROTOCOL", "1.5") // the manifest doesn't offer it either
	e2, _ := approvedEnv(t, nil, useProtocol([]string{"1.0"}, ""))
	if _, err := e2.m.Initialize(ctx(t), "echo"); err == nil {
		t.Fatal("initialize with a protocol the manifest doesn't offer succeeded")
	}
}

func TestInitializeTellsThePluginWhatPlatypusSpeaks(t *testing.T) {
	// echo refuses to initialize when it has no version in common, so a
	// successful start proves the list reached it.
	t.Setenv("ECHO_PROTOCOLS", "1.1")
	e, _ := approvedEnv(t, nil, useProtocol([]string{"1.0", "1.1"}, ""))
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if got := e.m.Runtime("echo").Protocol; got != "1.1" {
		t.Fatalf("protocol %q", got)
	}
}

func TestTransportRegistry(t *testing.T) {
	tr, ok := LookupTransport("jsonrpc-stdio")
	if !ok || tr.Name() != "jsonrpc-stdio" {
		t.Fatalf("the default transport is missing: %v %v", tr, ok)
	}
	if _, ok := LookupTransport("carrier-pigeon"); ok {
		t.Fatal("unknown transport found")
	}
}
