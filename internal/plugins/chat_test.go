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
	if !slices.Equal(c.Methods, []string{"chat.history", "chat.send", "chat.markRead"}) || !slices.Equal(c.OptionalMethods, []string{"chat.unreadCounts"}) {
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
	// The unread snapshot is the optional method of chat, added in 1.13: an
	// older plugin answers -32601, which Platypus remembers for the run.
	if owner, optional, ok := CapabilityForMethod("chat.unreadCounts"); !ok || owner != "chat" || !optional {
		t.Fatalf("chat.unreadCounts belongs to %q (optional %v, known %v)", owner, optional, ok)
	}
	if UnreadCountsVersion != (Version{1, 13}) {
		t.Fatalf("UnreadCountsVersion = %v, want 1.13", UnreadCountsVersion)
	}
	if !slices.Contains(SupportedProtocols, UnreadCountsVersion) {
		t.Fatalf("SupportedProtocols %v has no 1.13", SupportedProtocols)
	}
	v113, _ := ParseVersion("1.13")
	if kept, later := WithinProtocol([]string{"social", "chat"}, v113); len(later) != 0 || len(kept) != 2 {
		t.Fatalf("over 1.13: kept %v, later %v", kept, later)
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
