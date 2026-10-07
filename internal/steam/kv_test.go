package steam

import "testing"

func TestParseKV(t *testing.T) {
	src := `// comment
"UserLocalConfigStore"
{
	"Software" { "Valve" { "Steam" { "apps"
	{
		"480"
		{
			"LastPlayed"		"1790282473"
			"Playtime"		"21566"
		}
		"70" { "Playtime" "29" }
	}
	} } }
	"escaped"	"say \"hi\"\n"
}`
	kv, err := ParseKV(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := kv.String("UserLocalConfigStore", "software", "valve", "steam", "apps", "480", "Playtime"); got != "21566" {
		t.Fatalf("case-insensitive lookup: got %q", got)
	}
	if got := kv.String("UserLocalConfigStore", "escaped"); got != "say \"hi\"\n" {
		t.Fatalf("escapes: got %q", got)
	}
	if kv.Get("UserLocalConfigStore", "missing", "x") != nil {
		t.Fatal("missing path should be nil")
	}
	if _, err := ParseKV(`"a" { "b" "c"`); err == nil {
		t.Fatal("unterminated block should fail")
	}
}

func TestParseKVLiveShape(t *testing.T) {
	// Live PICS buffers are text KV wrapped in "appinfo" and
	// NUL-terminated; the parser must tolerate the terminator.
	src := "\"appinfo\"\n{\n\"depots\"\n{\n\"11\"\n{\n\"manifests\"\n{\n\"public\"\n{\n\"gid\"\t\t\"999\"\n}\n}\n}\n}\n}\n\x00"
	kv, err := ParseKV(src)
	if err != nil {
		t.Fatal(err)
	}
	app := kv.Get("appinfo")
	if app == nil || app.Get("depots") == nil {
		t.Fatal("no appinfo/depots section")
	}
	if got := app.String("depots", "11", "manifests", "public", "gid"); got != "999" {
		t.Fatalf("gid: %q", got)
	}
}

func TestParseBinaryKV(t *testing.T) {
	// Hand-built binary KV: None{depots: None{11: None{config:
	// None{oslist: "linux"}, manifests: None{public: None{gid:
	// "999"}}}}}. Validates the binary reader against the same
	// section layout selectDepots walks.
	var blob []byte
	str := func(s string) { blob = append(blob, s...); blob = append(blob, 0) }
	node := func(typ byte, key string) { blob = append(blob, typ); str(key) }
	node(0, "depots")
	node(0, "11")
	node(0, "config")
	node(1, "oslist")
	str("linux")
	blob = append(blob, 8)
	node(0, "manifests")
	node(0, "public")
	node(1, "gid")
	str("999")
	blob = append(blob, 8, 8, 8, 8)
	kv, err := ParseBinaryKV(blob)
	if err != nil {
		t.Fatal(err)
	}
	if got := kv.String("depots", "11", "config", "oslist"); got != "linux" {
		t.Fatalf("oslist: %q", got)
	}
	if got := kv.String("depots", "11", "manifests", "public", "gid"); got != "999" {
		t.Fatalf("gid: %q", got)
	}
	if _, err := ParseBinaryKV([]byte{1, '"', 'x'}); err == nil {
		t.Fatal("unterminated key accepted")
	}
	if _, err := ParseBinaryKV([]byte{34}); err == nil {
		t.Fatal("unknown node type accepted")
	}
}
