package steamcm

import "testing"

// The base follows the install's machine id: two installs on one account get
// different role logon ids, and one install keeps them across restarts
// (issue #186).
func TestLoginIDBaseFollowsTheMachineID(t *testing.T) {
	one, two := t.TempDir(), t.TempDir()
	a := loginIDBaseFor(StableMachineID(one))
	b := loginIDBaseFor(StableMachineID(two))
	if a == b {
		t.Fatalf("two installs share the logon id base %d", a)
	}
	if again := loginIDBaseFor(StableMachineID(one)); again != a {
		t.Fatalf("one install changed its base: %d then %d", a, again)
	}
	// Every role shifts by the same offset, so a different base means
	// different ids for every role.
	if a+0x01000000 == b+0x01000000 || a+0x03000000 == b+0x03000000 {
		t.Fatalf("bases %d and %d collide on a role id", a, b)
	}
}

// The base stays clear of the role range and of the one-shot fallback: role
// ids never wrap a uint32, and base-1 is never a role id.
func TestLoginIDRangesAreDisjoint(t *testing.T) {
	for _, base := range []uint32{
		loginIDBaseFor(StableMachineID(t.TempDir())),
		loginIDBaseFor(StableMachineID(t.TempDir())),
		1,
		0x3F000000,
	} {
		if base == 0 || base > 0x3F000000 {
			t.Fatalf("base %d is out of range", base)
		}
		lo, hi := base+0x01000000, base+0x03000000
		if lo <= base || hi < lo {
			t.Fatalf("base %d: role range %#x..%#x wraps or touches the base", base, lo, hi)
		}
		if base-1 >= lo && base-1 <= hi {
			t.Fatalf("base %d: the one-shot fallback %d lands in the role range", base, base-1)
		}
	}
}
