package plugins

import (
	"context"
	"strings"
	"testing"
	"time"
)

// initialize carries the effective locale (protocol 1.14): a plugin started
// after SetLocale answers localized text from its first request.
func TestInitializeCarriesLocale(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, nil)
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	e.m.SetLocale("fr-FR")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := e.m.Call(ctx(t), "echo", "game.details", map[string]any{"id": "tiny"}, &details); err != nil {
		t.Fatal(err)
	}
	summary, _ := details["summary"].(string)
	if !strings.Contains(summary, "faux jeu") {
		t.Fatalf("game.details summary = %q, want French under fr-FR", summary)
	}
	var begin map[string]any
	if err := e.m.Call(ctx(t), "echo", "auth.begin", map[string]any{}, &begin); err != nil {
		t.Fatal(err)
	}
	instr, _ := begin["instructions"].(string)
	if !strings.Contains(instr, "connexion") {
		t.Fatalf("auth.begin instructions = %q, want French under fr-FR", instr)
	}
}

func detailsSummary(t *testing.T, e *env, id string) string {
	t.Helper()
	var details map[string]any
	if err := e.m.Call(ctx(t), id, "game.details", map[string]any{"id": "tiny"}, &details); err != nil {
		t.Fatal(err)
	}
	s, _ := details["summary"].(string)
	return s
}

// locale.changed reaches a running plugin that negotiated 1.14: its texts
// switch without a restart.
func TestLocaleChangedReaches14(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, nil)
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	e.m.SetLocale("en")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if got := detailsSummary(t, e, "echo"); !strings.Contains(got, "tiny fake game") {
		t.Fatalf("summary before switch = %q, want English", got)
	}
	e.m.SetLocale("fr")
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := detailsSummary(t, e, "echo")
		if strings.Contains(got, "faux jeu") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("summary after locale.changed = %q, want French", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A plugin that settled on 1.13 never gets locale.changed: it keeps
// answering in the locale its initialize carried.
func TestLocaleChangedSkips13(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["protocol"] = map[string]any{"versions": []string{"1.0", "1.13"}}
	})
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	e.m.SetLocale("en")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if rt := e.m.Runtime("echo"); rt.Protocol != "1.13" {
		t.Fatalf("negotiated = %q, want 1.13", rt.Protocol)
	}
	if got := detailsSummary(t, e, "echo"); !strings.Contains(got, "tiny fake game") {
		t.Fatalf("summary before switch = %q, want English", got)
	}
	e.m.SetLocale("fr")
	time.Sleep(300 * time.Millisecond) // a notification would have landed by now
	if got := detailsSummary(t, e, "echo"); !strings.Contains(got, "tiny fake game") {
		t.Fatalf("summary after SetLocale = %q, want still English (no locale.changed on 1.13)", got)
	}
	_ = context.Background
}
