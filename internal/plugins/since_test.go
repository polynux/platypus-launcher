package plugins

import (
	"errors"
	"slices"
	"testing"

	"hydra-launcher/internal/jsonrpc"
)

// A capability is only on when the protocol the plugin settled on includes the
// revision that introduced it (Capability.Since).

func TestCapabilityNewerThanTheNegotiatedProtocolIsOff(t *testing.T) {
	e, _ := approvedEnv(t, nil, func(m map[string]any) {
		useProtocol([]string{"1.0", "1.1"}, "")(m)
		m["capabilities"] = []string{"library", "install", "versions", "verify"}
	})
	info, err := e.m.Initialize(ctx(t), "echo")
	if err != nil {
		t.Fatal(err)
	}
	if info.Protocol != "1.1" {
		t.Fatalf("protocol = %s", info.Protocol)
	}
	rt := e.m.Runtime("echo")
	if !slices.Equal(rt.Capabilities, []string{"library", "install"}) {
		t.Fatalf("runtime capabilities = %v: versions (since 1.3) and verify (since 1.4) must be off over 1.1", rt.Capabilities)
	}
	err = e.m.Call(ctx(t), "echo", "versions.list", map[string]any{"id": "tiny"}, nil)
	var re *jsonrpc.Error
	if !errors.As(err, &re) || re.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("versions.list over 1.1: %v, want -32601 from Hydra's side", err)
	}
	if n := echoCalls(t, e)["versions.list"]; n != 0 {
		t.Fatalf("versions.list reached the plugin %d times", n)
	}
}

func TestCapabilityIsOnOverAProtocolThatIncludesIt(t *testing.T) {
	e, _ := approvedEnv(t, nil, func(m map[string]any) {
		useProtocol([]string{"1.0", "1.1", "1.3"}, "")(m)
		m["capabilities"] = []string{"library", "install", "versions"}
	})
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if got := e.m.Runtime("echo"); got.Protocol != "1.3" || !slices.Contains(got.Capabilities, "versions") {
		t.Fatalf("runtime = %+v", got)
	}
}

func TestHydraSpeaksVersionsRevision(t *testing.T) {
	if !slices.Contains(SupportedProtocols, Version{1, 3}) {
		t.Fatalf("SupportedProtocols = %v, want 1.3 in it", SupportedProtocols)
	}
	if c, _ := LookupCapability("versions"); c.Since != "1.3" || !c.Orchestrated {
		t.Fatalf("versions: %+v", c)
	}
}

func TestVerifyAndMoveNeedProtocol14(t *testing.T) {
	e, _ := approvedEnv(t, nil, func(m map[string]any) {
		useProtocol([]string{"1.0", "1.3"}, "")(m)
		m["capabilities"] = []string{"library", "install", "verify", "move"}
	})
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if rt := e.m.Runtime("echo"); slices.Contains(rt.Capabilities, "verify") || slices.Contains(rt.Capabilities, "move") {
		t.Fatalf("verify/move on over 1.3: %v", rt.Capabilities)
	}
}

func TestRelocatedIsOptionalWithoutACapability(t *testing.T) {
	if OptionalWithoutCapability("game.move") || !OptionalWithoutCapability("game.relocated") {
		t.Fatal("game.relocated must be an optional method outside every capability")
	}
	if _, _, ok := CapabilityForMethod("game.relocated"); ok {
		t.Fatal("game.relocated belongs to no capability")
	}
	if !(Runtime{Protocol: "1.4"}).Speaks(Version{1, 4}) || (Runtime{Protocol: "1.3"}).Speaks(Version{1, 4}) || (Runtime{}).Speaks(Version{1, 0}) {
		t.Fatal("Runtime.Speaks compares the negotiated protocol")
	}
}
