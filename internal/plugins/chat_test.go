package plugins

import (
	"slices"
	"testing"
)

// Chat is protocol 1.9: a plugin that settled on 1.8 never gets its methods,
// whatever its manifest lists, and it is one of the rows of the registry.
func TestChatIsProtocol19(t *testing.T) {
	c, ok := LookupCapability("chat")
	if !ok || c.Since != "1.9" || !c.Orchestrated {
		t.Fatalf("chat: %+v, known %v", c, ok)
	}
	if !slices.Equal(c.Methods, []string{"chat.history", "chat.send", "chat.markRead"}) || len(c.OptionalMethods) != 0 {
		t.Fatalf("chat methods %v, optional %v", c.Methods, c.OptionalMethods)
	}
	v18, _ := ParseVersion("1.8")
	kept, later := WithinProtocol([]string{"social", "chat"}, v18)
	if !slices.Equal(kept, []string{"social"}) || !slices.Equal(later, []string{"chat"}) {
		t.Fatalf("over 1.8: kept %v, later %v", kept, later)
	}
	v19, _ := ParseVersion("1.9")
	if kept, later := WithinProtocol([]string{"social", "chat"}, v19); len(later) != 0 || len(kept) != 2 {
		t.Fatalf("over 1.9: kept %v, later %v", kept, later)
	}
	if owner, optional, ok := CapabilityForMethod("chat.send"); !ok || owner != "chat" || optional {
		t.Fatalf("chat.send belongs to %q (optional %v, known %v)", owner, optional, ok)
	}
	// Notifications are not methods: the registry has no row for them.
	for _, n := range []string{"chat.message", "chat.typing", "chat.unread"} {
		if _, _, ok := CapabilityForMethod(n); ok {
			t.Fatalf("%s is listed as a method", n)
		}
	}
	if !slices.Contains(SupportedProtocols, Version{1, 9}) {
		t.Fatal("1.9 is not in SupportedProtocols")
	}
}
