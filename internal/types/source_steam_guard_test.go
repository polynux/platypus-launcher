package types_test

// Guard for issue #54: the built-in Steam source is gone, so `SourceSteam`
// outside tests may only appear where the value is genuinely generic: the
// type definition itself, the steam domain's own scan (now the bundled
// plugin's data source), and store-based rules that treat any game from the
// Steam store the same (ratings, umu, save resolution, legacy inference).
// Playtime, cloud probing and art serving must never special-case it again:
// the steam plugin provides that data.
import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allowedSourceSteam names the non-test files that may mention SourceSteam,
// with why each one is generic.
var allowedSourceSteam = map[string]string{
	"internal/types/models.go":    "the type definition",
	"internal/steam/vdf.go":       "the Steam domain scan (the bundled plugin's data source)",
	"internal/app/app.go":         "legacy inference for games from before Game.Source existed",
	"internal/app/app_compat.go":  "store-based ratings: Steam games use their own AppID, like any store game",
	"internal/runner/umu.go":      "store-based umu rule for Steam-store games",
	"internal/savebackup/resolve.go": "store-based save resolution for Steam-store games",
	"internal/savebackup/service.go": "store-based save resolution for Steam-store games",
}

func TestSourceSteamOnlyWhereGeneric(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// The test walks the repo from internal/types; anchor on the module root.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root, _ = filepath.Abs(".")
	}
	var bad []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "dist" || name == ".build" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if !strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "cmd/") && !strings.HasPrefix(rel, "plugins/") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil // vet covers syntax; the guard only looks for the identifier
		}
		found := false
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "SourceSteam" {
				found = true
			}
			return !found
		})
		if found {
			if _, ok := allowedSourceSteam[filepath.ToSlash(rel)]; !ok {
				bad = append(bad, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("SourceSteam in non-generic places (see allowedSourceSteam):\n%s", strings.Join(bad, "\n"))
	}
}
