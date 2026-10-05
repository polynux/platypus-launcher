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
