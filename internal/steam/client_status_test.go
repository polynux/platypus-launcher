package steam

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
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
	if status == nil || status.State != "launching" {
		t.Fatalf("status = %#v, want completed launch handoff", status)
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

func TestClientStatusReportsIdleAfterDownloadCompletes(t *testing.T) {
	status := statusFromFixture(t, "GameAction [AppID 3599690, ActionID 4] : LaunchApp changed task to CheckShaderDepotManifest with \"\"\nGameAction [AppID 3599690, ActionID 4] : LaunchApp changed task to DownloadingDepots with \"\"\n",
		`"AppState" { "StateFlags" "4" "BytesToDownload" "1000" "BytesDownloaded" "1000" }`)
	if status.State != "idle" {
		t.Fatalf("status = %#v, want idle after work completes", status)
	}
}

func TestClientStatusDownloadThenNewLaunchTask(t *testing.T) {
	status := statusFromFixture(t, "GameAction [AppID 3599690, ActionID 4] : LaunchApp changed task to DownloadingDepots with \"\"\nGameAction [AppID 3599690, ActionID 5] : LaunchApp changed task to CreatingProcess with \"\"\n", "")
	if status.State != "launching" {
		t.Fatalf("status = %#v, want launching", status)
	}
}

func TestClientStatusGameProcessAddedIsLaunchHandoff(t *testing.T) {
	status := statusFromFixture(t, "[2026-10-23 20:04:10] Game process added : AppID 3599690 (pid 1234)\n", "")
	if status.State != "launching" {
		t.Fatalf("status = %#v, want launching", status)
	}
}

func statusFromFixture(t *testing.T, log, manifest string) *types.ClientStatus {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local/share/Steam")
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "logs/console_log.txt"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "steamapps/appmanifest_3599690.acf"), []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return ClientStatus("3599690")
}
