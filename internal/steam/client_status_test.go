package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientStatusReadsAppActionAndManifestProgress(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local/share/Steam")
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("testdata/console-launch.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs/console_log.txt"), fixture, 0600); err != nil {
		t.Fatal(err)
	}
	status := ClientStatus("3599690")
	if status == nil || status.State != "idle" {
		t.Fatalf("status = %#v, want completed idle", status)
	}
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0755); err != nil {
		t.Fatal(err)
	}
	acf := `"AppState" { "StateFlags" "1026" "BytesToDownload" "1000" "BytesDownloaded" "450" }`
	if err := os.WriteFile(filepath.Join(root, "steamapps/appmanifest_3599690.acf"), []byte(acf), 0600); err != nil {
		t.Fatal(err)
	}
	status = ClientStatus("3599690")
	if status.State != "updating" || status.Bytes != 450 || status.Total != 1000 || status.Progress == nil || *status.Progress != .45 {
		t.Fatalf("status = %#v, want 45%% updating", status)
	}
}
