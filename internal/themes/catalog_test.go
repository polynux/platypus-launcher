package themes

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"platypus/internal/types"
)

var updateCatalogSnapshot = flag.Bool("update", false, "update the catalog snapshot")

type catalogSnapshot struct {
	Version string                    `json:"version"`
	Entries []types.ThemeCatalogEntry `json:"entries"`
}

// A major bump permits removals; within a major, catalog additions need an in-range since version.
func TestCatalogGuard(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate catalog test")
	}
	path := filepath.Join(filepath.Dir(source), "testdata", "catalog-1.json")
	if *updateCatalogSnapshot {
		if err := writeCatalogSnapshot(path); err != nil {
			t.Fatal(err)
		}
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read catalog snapshot: %v (refresh with go test ./internal/themes -run TestCatalogGuard -update)", err)
	}
	var snapshot catalogSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("decode catalog snapshot: %v", err)
	}
	snapshotMajor, snapshotMinor, ok := catalogVersion(snapshot.Version)
	if !ok {
		t.Fatalf("invalid snapshot catalog version %q", snapshot.Version)
	}
	currentMajor, currentMinor, ok := catalogVersion(CatalogVersion)
	if !ok {
		t.Fatalf("invalid CatalogVersion %q", CatalogVersion)
	}
	if currentMajor < snapshotMajor || (currentMajor == snapshotMajor && currentMinor < snapshotMinor) {
		t.Fatalf("CatalogVersion %s is older than snapshot %s", CatalogVersion, snapshot.Version)
	}
	for _, entry := range Catalog {
		if _, exists := findSnapshotEntry(snapshot.Entries, entry.Entry); exists {
			continue
		}
		checkSince(t, "entry "+entry.Entry, entry.Since, snapshot.Version, CatalogVersion)
		for _, axis := range entry.Axes {
			checkSince(t, "axis "+entry.Entry+"."+axis.Name, axis.Since, snapshot.Version, CatalogVersion)
		}
	}
	for _, entry := range Catalog {
		old, exists := findSnapshotEntry(snapshot.Entries, entry.Entry)
		if !exists {
			continue
		}
		for _, axis := range entry.Axes {
			oldAxis, exists := findSnapshotAxis(old.Axes, axis.Name)
			if !exists {
				checkSince(t, "axis "+entry.Entry+"."+axis.Name, axis.Since, snapshot.Version, CatalogVersion)
				continue
			}
			if currentMajor <= snapshotMajor {
				for _, value := range oldAxis.Values {
					if !slices.Contains(axis.Values, value) {
						t.Errorf("catalog removed value %s.%s=%s without a major version bump", entry.Entry, axis.Name, value)
					}
				}
			}
		}
	}
	if currentMajor <= snapshotMajor {
		for _, old := range snapshot.Entries {
			entry, exists := findSnapshotEntry(Catalog, old.Entry)
			if !exists {
				t.Errorf("catalog removed entry %q without a major version bump", old.Entry)
				continue
			}
			for _, oldAxis := range old.Axes {
				if _, exists := findSnapshotAxis(entry.Axes, oldAxis.Name); !exists {
					t.Errorf("catalog removed axis %s.%s without a major version bump", old.Entry, oldAxis.Name)
				}
			}
		}
	}
	if !reflect.DeepEqual(snapshot.Entries, Catalog) || snapshot.Version != CatalogVersion {
		t.Log("catalog snapshot is out of date; refresh with go test ./internal/themes -run TestCatalogGuard -update")
	}
}

func writeCatalogSnapshot(path string) error {
	data, err := json.MarshalIndent(catalogSnapshot{Version: CatalogVersion, Entries: Catalog}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog snapshot: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write catalog snapshot: %w", err)
	}
	return nil
}

func findSnapshotEntry(entries []types.ThemeCatalogEntry, name string) (types.ThemeCatalogEntry, bool) {
	for _, entry := range entries {
		if entry.Entry == name {
			return entry, true
		}
	}
	return types.ThemeCatalogEntry{}, false
}

func findSnapshotAxis(axes []types.ThemeCatalogAxis, name string) (types.ThemeCatalogAxis, bool) {
	for _, axis := range axes {
		if axis.Name == name {
			return axis, true
		}
	}
	return types.ThemeCatalogAxis{}, false
}

func checkSince(t *testing.T, name, since, snapshotVersion, currentVersion string) {
	t.Helper()
	sinceMajor, sinceMinor, ok := catalogVersion(since)
	if !ok {
		t.Errorf("new %s has invalid since version %q", name, since)
		return
	}
	snapshotMajor, snapshotMinor, _ := catalogVersion(snapshotVersion)
	currentMajor, currentMinor, _ := catalogVersion(currentVersion)
	if sinceMajor < snapshotMajor || (sinceMajor == snapshotMajor && sinceMinor < snapshotMinor) {
		t.Errorf("new %s since %s predates snapshot %s", name, since, snapshotVersion)
	}
	if sinceMajor > currentMajor || (sinceMajor == currentMajor && sinceMinor > currentMinor) {
		t.Errorf("new %s since %s is newer than CatalogVersion %s", name, since, currentVersion)
	}
}
