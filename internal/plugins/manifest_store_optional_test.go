package plugins

import "testing"

func TestStoreCanBeOmittedWithoutStoreCapabilities(t *testing.T) {
	m := Manifest{
		ID: "facts", Name: "Facts", Version: "1.0.0", Exec: "bin/facts",
		Protocol:     &ProtocolSpec{Versions: VersionSet{List: []Version{{1, 17}}}},
		Capabilities: []string{"compat-facts"},
	}
	if err := m.validate(); err != nil {
		t.Fatalf("store-independent plugin needs no store id: %v", err)
	}
	m.Capabilities = []string{"library"}
	if err := m.validate(); err == nil {
		t.Fatal("store plugin without a store id was accepted")
	}
}
