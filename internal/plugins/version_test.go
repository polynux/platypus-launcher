package plugins

import (
	"encoding/json"
	"testing"
)

func v(t *testing.T, s string) Version {
	t.Helper()
	x, err := ParseVersion(s)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestParseVersion(t *testing.T) {
	for in, want := range map[string]Version{"1.0": {1, 0}, "1.12": {1, 12}, "2.3": {2, 3}} {
		got, err := ParseVersion(in)
		if err != nil || got != want || got.String() != in {
			t.Errorf("ParseVersion(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "1", "1.", ".1", "1.0.0", "a.b", "-1.0", "1.-1", " 1.0", "01.0x"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("ParseVersion(%q) should fail", bad)
		}
	}
}

func TestVersionOrder(t *testing.T) {
	if !v(t, "1.2").Less(v(t, "1.10")) || !v(t, "1.9").Less(v(t, "2.0")) || v(t, "1.0").Less(v(t, "1.0")) {
		t.Fatal("versions compare numerically, major first")
	}
}

func TestVersionSetJSON(t *testing.T) {
	cases := map[string]struct {
		in       string
		contains map[string]bool
		bad      bool
	}{
		"list":      {in: `["1.0","1.2"]`, contains: map[string]bool{"1.0": true, "1.1": false, "1.2": true}},
		"range":     {in: `{"min":"1.0","max":"1.3"}`, contains: map[string]bool{"0.9": false, "1.0": true, "1.3": true, "1.4": false}},
		"open":      {in: `{"min":"1.2"}`, contains: map[string]bool{"1.1": false, "1.2": true, "1.9": true, "2.0": false}},
		"empty":     {in: `[]`, bad: true},
		"inverted":  {in: `{"min":"1.3","max":"1.0"}`, bad: true},
		"no bounds": {in: `{}`, bad: true},
		"string":    {in: `"1.0"`, bad: true},
		"malformed": {in: `["1.x"]`, bad: true},
	}
	for name, c := range cases {
		var s VersionSet
		err := json.Unmarshal([]byte(c.in), &s)
		if c.bad {
			if err == nil {
				t.Errorf("%s: accepted %s", name, c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for ver, want := range c.contains {
			if s.Contains(v(t, ver)) != want {
				t.Errorf("%s: Contains(%s) = %v, want %v", name, ver, !want, want)
			}
		}
		// Round trip keeps the meaning.
		out, _ := json.Marshal(s)
		var back VersionSet
		if err := json.Unmarshal(out, &back); err != nil {
			t.Errorf("%s: re-reading %s: %v", name, out, err)
		}
		for ver, want := range c.contains {
			if back.Contains(v(t, ver)) != want {
				t.Errorf("%s: after a round trip Contains(%s) = %v", name, ver, !want)
			}
		}
	}
}

func TestNegotiateTakesTheHighestCommonVersion(t *testing.T) {
	supported := []Version{{1, 0}, {1, 1}}
	cases := []struct {
		offered string
		want    string
		ok      bool
	}{
		{`["1.0","1.1"]`, "1.1", true},
		{`{"min":"1.0","max":"1.5"}`, "1.1", true},
		{`["1.0"]`, "1.0", true},
		{`["1.1","1.5"]`, "1.1", true},
		{`["1.2","1.5"]`, "", false},
		{`["2.0"]`, "", false},
		{`{"min":"2.0"}`, "", false},
		{`{"min":"0.1","max":"0.9"}`, "", false},
	}
	for _, c := range cases {
		var s VersionSet
		if err := json.Unmarshal([]byte(c.offered), &s); err != nil {
			t.Fatal(err)
		}
		got, ok := Negotiate(s, supported)
		if ok != c.ok || (ok && got.String() != c.want) {
			t.Errorf("Negotiate(%s) = %v, %v; want %s, %v", c.offered, got, ok, c.want, c.ok)
		}
	}
	// Hydra's own list order doesn't matter.
	var s VersionSet
	_ = json.Unmarshal([]byte(`["1.0","1.1"]`), &s)
	if got, _ := Negotiate(s, []Version{{1, 1}, {1, 0}}); got != (Version{1, 1}) {
		t.Fatalf("got %v", got)
	}
}

func TestSupportedProtocolsAreSortedAndSameMajorPerRevision(t *testing.T) {
	if len(SupportedProtocols) == 0 {
		t.Fatal("no supported protocol")
	}
	for i := 1; i < len(SupportedProtocols); i++ {
		if !SupportedProtocols[i-1].Less(SupportedProtocols[i]) {
			t.Fatalf("SupportedProtocols not ascending: %v", SupportedProtocols)
		}
	}
	if SupportedProtocols[0].Major != APIVersion {
		t.Fatalf("the oldest supported protocol %v is not major %d", SupportedProtocols[0], APIVersion)
	}
}

// An open-ended range stays inside its minimum's major: a plugin that says
// {"min":"1.0"} was written for 1.x and must never negotiate a breaking 2.0.
func TestOpenRangeStaysInItsMajor(t *testing.T) {
	var s VersionSet
	if err := json.Unmarshal([]byte(`{"min":"1.0"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Contains(Version{2, 0}) {
		t.Fatal(`{"min":"1.0"} must not contain 2.0`)
	}
	if !s.Contains(Version{1, 7}) {
		t.Fatal(`{"min":"1.0"} must contain 1.7`)
	}
	got, ok := Negotiate(s, []Version{{1, 0}, {1, 1}, {2, 0}})
	if !ok || got != (Version{1, 1}) {
		t.Fatalf("Negotiate = %v, %v; want 1.1", got, ok)
	}
}
