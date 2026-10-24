package gents_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"platypus/internal/app"
	"platypus/internal/corerpc"
	"platypus/internal/gents"
	"platypus/internal/themes"
)

// repoRoot is the repository root, anchored at this test's source file so the
// test runs with any working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// TestGeneratedUpToDate fails when apps/ui/src/gen/ is stale: regenerate with
// `go run ./cmd/gen-ts` from the repository root and commit the result.
func TestGeneratedUpToDate(t *testing.T) {
	root := repoRoot(t)
	models, api, err := gents.Generate(&app.App{}, filepath.Join(root, "internal", "app"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for name, want := range map[string]string{"models.ts": models, "api.ts": api, "catalog.ts": themes.CatalogTS(), "builtinLayouts.ts": themes.BuiltinLayoutsTS()} {
		got, err := os.ReadFile(filepath.Join(root, "apps", "ui", "src", "gen", name))
		if err != nil {
			t.Fatalf("%s: %v (run `go run ./cmd/gen-ts` from the repository root)", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s is stale: run `go run ./cmd/gen-ts` from the repository root and commit the result", name)
		}
	}
}

// TestMethodsMatchCore locks the generated method list to the RPC surface:
// every method the core serves must be generated, and nothing else.
func TestMethodsMatchCore(t *testing.T) {
	methods, err := gents.Methods(&app.App{})
	if err != nil {
		t.Fatalf("Methods: %v", err)
	}
	got := map[string]bool{}
	for _, m := range methods {
		got[m.Name] = true
	}
	for _, name := range corerpc.New(&app.App{}).Methods() {
		if !got[name] {
			t.Errorf("core serves %s, missing from the generated bridge", name)
		}
		delete(got, name)
	}
	for name := range got {
		t.Errorf("generated %s, not served by the core", name)
	}
}
