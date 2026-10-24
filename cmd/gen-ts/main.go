// Command gen-ts regenerates the UI's typed core bridge from the Go
// core: struct interfaces into apps/ui/src/gen/models.ts, the typed
// method list into apps/ui/src/gen/api.ts, the theme catalog's variant
// axes into apps/ui/src/gen/catalog.ts, and the built-in view layouts into
// apps/ui/src/gen/builtinLayouts.ts.
//
// Run it from the repository root: `go run ./cmd/gen-ts` (an optional first
// argument overrides the root). The generated files are checked in;
// internal/gents's test fails when they are stale.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"platypus/internal/app"
	"platypus/internal/gents"
	"platypus/internal/themes"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	models, api, err := gents.Generate(&app.App{}, filepath.Join(root, "internal", "app"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-ts:", err)
		os.Exit(1)
	}
	dir := filepath.Join(root, "apps", "ui", "src", "gen")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "gen-ts:", err)
		os.Exit(1)
	}
	for name, src := range map[string]string{"models.ts": models, "api.ts": api, "catalog.ts": themes.CatalogTS(), "builtinLayouts.ts": themes.BuiltinLayoutsTS()} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "gen-ts:", err)
			os.Exit(1)
		}
	}
	fmt.Println("wrote apps/ui/src/gen/models.ts, api.ts, catalog.ts and builtinLayouts.ts")
}
