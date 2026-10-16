package crashhints

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"platypus/internal/types"
)

// Each fixture is a real-shaped Wine stderr log. The expected hints pin what
// the table says about it, so changing a pattern shows up here.
func TestMatchFixtures(t *testing.T) {
	cases := []struct {
		file string
		want []types.CrashHint
	}{
		{"vcruntime.log", []types.CrashHint{
			{Rule: "vcredist", Param: "vcruntime140_1.dll", Verb: "vcrun2022"},
			{Rule: "vcredist", Param: "MSVCP140.dll", Verb: "vcrun2022"},
		}},
		{"directx.log", []types.CrashHint{
			{Rule: "directx", Param: "d3dx9_43.dll", Verb: "d3dx9_43"},
			{Rule: "directx", Param: "D3DCOMPILER_47.dll", Verb: "d3dcompiler_47"},
		}},
		{"media.log", []types.CrashHint{
			{Rule: "media", Action: "runner"},
		}},
		{"dotnet.log", []types.CrashHint{
			{Rule: "dotnet", Verb: "dotnet48"},
		}},
		{"vulkan.log", []types.CrashHint{
			{Rule: "vulkan"},
		}},
		{"anticheat.log", []types.CrashHint{
			{Rule: "anticheat", Action: "anticheat"},
		}},
		{"generic.log", []types.CrashHint{
			{Rule: "missingDll", Param: "libfoo-7.dll"},
			{Rule: "missingDll", Param: "bar.dll"},
		}},
		{"clean.log", nil},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			got := Match(string(raw))
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Match(%s):\n got %+v\nwant %+v", c.file, got, c.want)
			}
		})
	}
}

// The first rule that fits a line owns it: a missing vcruntime is the
// vcredist finding, not the generic missing DLL.
func TestSpecificRuleBeatsGenericForTheSameLine(t *testing.T) {
	got := Match(`err:module:import_dll Library msvcp140.dll (which is needed by L"x.exe") not found`)
	if len(got) != 1 || got[0].Rule != "vcredist" {
		t.Fatalf("got %+v, want one vcredist hint", got)
	}
}

// A verb that isn't in the pinned winetricks list is never suggested, but the
// explanation still is.
func TestUnknownVerbIsExplainedNotSuggested(t *testing.T) {
	got := Match(`err:module:import_dll Library d3dx9_99.dll (which is needed by L"x.exe") not found`)
	if len(got) != 1 || got[0].Rule != "directx" || got[0].Verb != "" {
		t.Fatalf("got %+v, want a directx hint with no verb", got)
	}
}

func TestMatchDedupesAndCapsTheHints(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("err:module:import_dll Library lib%d.dll not found", i))
	}
	lines = append(lines, "err:module:import_dll Library lib0.dll not found")
	got := Match(strings.Join(lines, "\r\n"))
	if len(got) != maxHints {
		t.Fatalf("got %d hints, want the cap %d", len(got), maxHints)
	}
	if got[0].Param != "lib0.dll" || got[1].Param != "lib1.dll" {
		t.Fatalf("hints not in log order: %+v", got[:2])
	}
}
