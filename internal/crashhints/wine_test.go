package crashhints

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"platypus/internal/testenv"
	"platypus/internal/types"
)

// Real Wine crash logs (issue #70): tiny Windows executables that fail the
// way real games do, run under the system Wine in a temp prefix. The log the
// test matches against is genuine Wine stderr, not a hand-written fixture.
// The normalized captures live in testdata/wine-*.log so the fast suite
// covers real output too (TestMatchFixtures below).
//
// The executables are minimal 64-bit PEs built below in pure Go: one import
// descriptor naming a DLL Wine cannot load, entry point never reached. No
// mingw, no new system packages. Per-case WINEDLLOVERRIDES reproduce the
// broken-prefix state: vcruntime140/d3dx9_43 forced to native-only (modern
// Wine ships builtins for both, so an undecorated run would succeed), and
// mscoree disabled (no Mono/Gecko installs, no dialogs; WINEDEBUG=err+all
// keeps just the loader errors).
func TestRealWineMissingImports(t *testing.T) {
	testenv.RequireSlow(t)
	wine, err := exec.LookPath("wine")
	if err != nil {
		t.Skip("wine not installed: rerun where wine is available")
	}

	cases := []struct {
		exe       string
		dll       string
		fn        string
		overrides string
		want      []types.CrashHint
	}{
		{"vcrun.exe", "vcruntime140.dll", "memcpy", "vcruntime140=n;mscoree,mshtml=",
			[]types.CrashHint{{Rule: "vcredist", Param: "vcruntime140.dll", Verb: "vcrun2022"}}},
		{"d3dx.exe", "d3dx9_43.dll", "D3DXCompileShader", "d3dx9_43=n;mscoree,mshtml=",
			[]types.CrashHint{{Rule: "directx", Param: "d3dx9_43.dll", Verb: "d3dx9_43"}}},
		{"dotnet.exe", "mscoree.dll", "_CorExeMain", "mscoree,mshtml=",
			[]types.CrashHint{{Rule: "dotnet", Verb: "dotnet48"}}},
	}

	prefix := t.TempDir()
	env := append(os.Environ(),
		"WINEPREFIX="+prefix,
		"WINEDEBUG=err+all",
	)
	run := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Env = env
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err := cmd.Run()
		return buf.String(), err
	}

	// Slows the suite by one prefix creation; afterwards wineserver is warm.
	if out, err := run(wine, "wineboot", "--init"); err != nil {
		t.Fatalf("wineboot --init: %v\n%s", err, out)
	}
	defer func() {
		cmd := exec.Command("wineserver", "-k")
		cmd.Env = env
		_ = cmd.Run()
	}()

	for _, c := range cases {
		t.Run(c.exe, func(t *testing.T) {
			exePath := filepath.Join(prefix, "drive_c", c.exe)
			if err := writeMissingImportEXE(exePath, c.dll, c.fn); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, wine, exePath)
			cmd.Env = append(env, "WINEDLLOVERRIDES="+c.overrides)
			var buf bytes.Buffer
			cmd.Stdout = &buf
			cmd.Stderr = &buf
			_ = cmd.Run() // fails with status 53: the missing import
			out := buf.String()
			if got := Match(out); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("Match(real wine output):\n got %+v\nwant %+v\noutput:\n%s", got, c.want, out)
			}
		})
	}
}

// writeMissingImportEXE writes a minimal 64-bit PE that imports fn from dll
// and does nothing else: the loader fails before the entry point (a lone
// ret) ever runs, printing the classic "Library X not found" error.
func writeMissingImportEXE(path, dll, fn string) error {
	const (
		fileAlign = 0x200
		sectAlign = 0x1000
		textRVA   = 0x1000
		idataRVA  = 0x2000
		imageBase = 0x140000000
	)
	fnBytes := append([]byte(fn), 0)
	dllBytes := append([]byte(dll), 0)

	offILT := uint32(40)
	offIAT := uint32(56)
	offHint := uint32(72)
	offDLL := offHint + (2+uint32(len(fnBytes))+1)/2*2
	idataUsed := offDLL + uint32(len(dllBytes))
	sizeOfImage := (idataRVA + idataUsed + sectAlign - 1) &^ (sectAlign - 1)

	var hdr []byte
	u16 := func(v uint16) { hdr = append(hdr, byte(v), byte(v>>8)) }
	u32 := func(v uint32) { hdr = append(hdr, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }
	u64 := func(v uint64) {
		hdr = append(hdr, byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
			byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
	}

	dos := make([]byte, 64)
	dos[0], dos[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(dos[60:], 64)
	hdr = append(hdr, dos...)
	hdr = append(hdr, 'P', 'E', 0, 0)
	u16(0x8664) // amd64
	u16(2)      // two sections
	u32(0)
	u32(0)
	u32(0)
	u16(240) // PE32+ optional header size
	u16(0x012f)
	u16(0x20b)
	hdr = append(hdr, 0, 0)
	u32(fileAlign)
	u32(fileAlign)
	u32(0)
	u32(textRVA) // entry point
	u32(textRVA) // base of code
	u64(imageBase)
	u32(sectAlign)
	u32(fileAlign)
	u16(6)
	u16(0)
	u16(0)
	u16(0)
	u16(6)
	u16(0)
	u32(0)
	u32(sizeOfImage)
	u32(fileAlign) // size of headers
	u32(0)
	u16(3) // console
	u16(0)
	u64(0x100000)
	u64(0x1000)
	u64(0x100000)
	u64(0x1000)
	u32(0)
	u32(16)
	u32(0) // export dir: none
	u32(0)
	u32(idataRVA) // import dir
	u32(20)
	for i := 0; i < 14; i++ {
		u32(0)
		u32(0)
	}
	section := func(name string, vsz, rva, rawptr, chars uint32) {
		var n [8]byte
		copy(n[:], name)
		hdr = append(hdr, n[:]...)
		u32(vsz)
		u32(rva)
		u32(fileAlign)
		u32(rawptr)
		u32(0)
		u32(0)
		u16(0)
		u16(0)
		u32(chars)
	}
	section(".text", 1, textRVA, fileAlign, 0x60000020)
	section(".idata", idataUsed, idataRVA, 2*fileAlign, 0xC0000040)
	hdr = append(hdr, make([]byte, fileAlign-uint32(len(hdr)))...)

	idata := make([]byte, (idataUsed+fileAlign-1)&^(fileAlign-1))
	put32 := func(off, v uint32) { binary.LittleEndian.PutUint32(idata[off:], v) }
	put64 := func(off uint32, v uint64) { binary.LittleEndian.PutUint64(idata[off:], v) }
	hintRVA := idataRVA + offHint
	put32(0, idataRVA+offILT) // OriginalFirstThunk
	put32(12, idataRVA+offDLL)
	put32(16, idataRVA+offIAT) // FirstThunk
	put64(offILT, uint64(hintRVA))
	put64(offIAT, uint64(hintRVA))
	copy(idata[offHint+2:], fnBytes)
	copy(idata[offDLL:], dllBytes)

	text := make([]byte, fileAlign)
	text[0] = 0xC3 // ret: never reached, the loader fails first

	return os.WriteFile(path, append(append(hdr, text...), idata...), 0o644)
}
