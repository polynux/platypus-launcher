package steam

import (
	"reflect"
	"testing"

	"platypus/internal/types"
)

// CORE-7: the xdg-open fallback must not receive launch arguments (the
// steam:// handler can't forward them; xdg-open would treat them as open
// targets).
func TestXdgOpenFallbackDropsLaunchArguments(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no steam, no flatpak
	game := types.Game{
		Store:      types.StoreSteam,
		StoreAppID: "620",
		Settings:   types.GameSettings{LaunchArguments: "-novid -fullscreen"},
	}
	want := []string{"xdg-open", "steam://rungameid/620"}
	if got := Command(game); !reflect.DeepEqual(got, want) {
		t.Fatalf("Command with fallback = %q, want %q", got, want)
	}
}
