package gog_test

// Library update detection (issue #209): library.list says an installed game
// has an update from its recorded build and its channel's tip, cached so a
// listing doesn't re-ask the store for every game.

import "testing"

func TestGogLibraryReportsUpdateAvailable(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	var out struct {
		Update  any  `json:"update"`
		Update2 any  `json:"update2"`
		Calls   int  `json:"calls"`
		Calls2  int  `json:"calls2"`
		NoFlag  bool `json:"noFlag"`
	}
	pyJSON(t, `
import json
calls = {"info": 0}
installed = {"1234": {"path": "/games/x", "title": "Super Game", "build": "7777",
                      "version": "0.9", "platform": "windows"}}
def fake_info(app, platform, build=None, branch=None, timeout=600):
    calls["info"] += 1
    return {"buildId": "8888", "builds": {"items": [
        {"build_id": "8888", "version_name": "1.0", "branch": None}]}}
hg.credentials = lambda: {"access_token": "tok"}
hg.account_products = lambda token: [{"id": 1234, "title": "Super Game", "isGame": True}]
hg.platform_of = lambda app: "windows"
hg.installed_games = lambda: installed
hg.gog_info = fake_info
first = hg.library_list({})
one = first["games"][0]
calls_one = calls["info"]
second = hg.library_list({})
two = second["games"][0]
out = {"update": one.get("updateAvailable"), "update2": two.get("updateAvailable"),
       "calls": calls_one, "calls2": calls["info"]}
# Same build as the tip: no update, and the cached tip costs no call.
installed["1234"]["build"] = "8888"
three = hg.library_list({})
out["noFlag"] = not three["games"][0].get("updateAvailable")
print(json.dumps(out))
`, &out)
	if out.Update != true || out.Update2 != true || out.Calls != 1 || out.Calls2 != 1 {
		t.Fatalf("update flag = %v / %v, info calls %d / %d", out.Update, out.Update2, out.Calls, out.Calls2)
	}
	if !out.NoFlag {
		t.Fatal("a game at the channel tip still reports an update")
	}
}

func TestGogLinuxInstallReportsNoUpdate(t *testing.T) {
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	var out struct {
		Flag  bool `json:"flag"`
		Calls int  `json:"calls"`
	}
	pyJSON(t, `
import json
calls = {"info": 0}
def fake_info(*a, **k):
    calls["info"] += 1
    return {"buildId": "8888"}
hg.credentials = lambda: {"access_token": "tok"}
hg.account_products = lambda token: [{"id": 1234, "title": "Super Game", "isGame": True}]
hg.platform_of = lambda app: "linux"
hg.installed_games = lambda: {"1234": {"path": "/games/x", "build": "1",
                                       "platform": "linux", "native": True}}
hg.gog_info = fake_info
lib = hg.library_list({})
print(json.dumps({"flag": "updateAvailable" in lib["games"][0], "calls": calls["info"]}))
`, &out)
	if out.Flag || out.Calls != 0 {
		t.Fatalf("Linux install: flag=%v info calls=%d", out.Flag, out.Calls)
	}
}
